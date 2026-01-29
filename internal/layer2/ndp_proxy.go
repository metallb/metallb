// SPDX-License-Identifier:Apache-2.0

package layer2

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/vishvananda/netlink"
)

const (
	sysctlBasePathEnv     = "METALLB_SYSCTL_BASE_PATH"
	defaultSysctlBasePath = "/proc/sys"
)

// NDPProxyManager manages kernel-level NDP proxying for IPv6 addresses.
// It handles the proxy_ndp sysctl setting and manages proxy neighbor entries.
type NDPProxyManager struct {
	logger    log.Logger
	iface     string
	link      netlink.Link
	vips      map[string]net.IP
	vipsMutex sync.Mutex
	// Track whether proxy_ndp is enabled for this interface
	proxyNDPEnabled bool
}

func sysctlBasePath() string {
	if base := os.Getenv(sysctlBasePathEnv); base != "" {
		return base
	}
	return defaultSysctlBasePath
}

// NewNDPProxyManager creates a new NDPProxyManager for the given interface.
func NewNDPProxyManager(logger log.Logger, iface string) (*NDPProxyManager, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to get link for interface %s: %w", iface, err)
	}

	return &NDPProxyManager{
		logger: logger,
		iface:  iface,
		link:   link,
		vips:   make(map[string]net.IP),
	}, nil
}

// Enable adds a VIP to the NDP proxy table and ensures proxy_ndp is enabled.
func (n *NDPProxyManager) Enable(vip net.IP) error {
	if vip.To4() != nil {
		// IPv4 addresses don't need NDP proxy
		return nil
	}

	n.vipsMutex.Lock()
	defer n.vipsMutex.Unlock()

	vipStr := vip.String()

	// Check if already enabled
	if _, exists := n.vips[vipStr]; exists {
		return nil
	}

	// Enable proxy_ndp sysctl if not already enabled
	if !n.proxyNDPEnabled {
		if err := n.enableProxyNDP(); err != nil {
			return fmt.Errorf("failed to enable proxy_ndp for %s: %w", n.iface, err)
		}
		n.proxyNDPEnabled = true
		level.Info(n.logger).Log("event", "proxyNDPEnabled", "interface", n.iface)
	}

	// Add proxy entry
	if err := n.addProxyEntry(vip); err != nil {
		return fmt.Errorf("failed to add proxy entry for %s on %s: %w", vipStr, n.iface, err)
	}

	n.vips[vipStr] = vip
	level.Info(n.logger).Log("event", "ndpProxyEntryAdded", "ip", vipStr, "interface", n.iface)
	return nil
}

// Disable removes a VIP from the NDP proxy table.
// If no VIPs remain, proxy_ndp is disabled.
func (n *NDPProxyManager) Disable(vip net.IP) error {
	if vip.To4() != nil {
		// IPv4 addresses don't need NDP proxy
		return nil
	}

	n.vipsMutex.Lock()
	defer n.vipsMutex.Unlock()

	vipStr := vip.String()

	// Check if this VIP is being managed
	if _, exists := n.vips[vipStr]; !exists {
		return nil
	}

	// Delete proxy entry (ignore errors, entry might not exist)
	_ = n.delProxyEntry(vip)

	delete(n.vips, vipStr)
	level.Info(n.logger).Log("event", "ndpProxyEntryRemoved", "ip", vipStr, "interface", n.iface)

	// If no VIPs left, disable proxy_ndp
	if len(n.vips) == 0 && n.proxyNDPEnabled {
		if err := n.disableProxyNDP(); err != nil {
			level.Warn(n.logger).Log("event", "disableProxyNDP", "interface", n.iface, "error", err)
		} else {
			n.proxyNDPEnabled = false
			level.Info(n.logger).Log("event", "proxyNDPDisabled", "interface", n.iface)
		}
	}

	return nil
}

// enableProxyNDP enables the proxy_ndp sysctl for the interface.
func (n *NDPProxyManager) enableProxyNDP() error {
	path := n.proxyNDPPath()
	if data, err := os.ReadFile(path); err == nil {
		if strings.TrimSpace(string(data)) == "1" {
			return nil
		}
	}
	return os.WriteFile(path, []byte("1"), 0644)
}

// disableProxyNDP disables the proxy_ndp sysctl for the interface.
func (n *NDPProxyManager) disableProxyNDP() error {
	path := n.proxyNDPPath()
	return os.WriteFile(path, []byte("0"), 0644)
}

// proxyNDPPath returns the sysctl path for proxy_ndp on this interface.
func (n *NDPProxyManager) proxyNDPPath() string {
	// Sanitize interface name to prevent directory traversal
	cleanIface := filepath.Base(n.iface)
	return filepath.Join(sysctlBasePath(), "net", "ipv6", "conf", cleanIface, "proxy_ndp")
}

// addProxyEntry adds a proxy neighbor entry for the VIP using netlink.
func (n *NDPProxyManager) addProxyEntry(vip net.IP) error {
	// Create a proxy neighbor entry
	// NUD_PROXY (0x08) is the kernel flag for proxy neighbor entries
	// See: https://www.kernel.org/doc/Documentation/networking/operstates.txt
	neigh := &netlink.Neigh{
		LinkIndex: n.link.Attrs().Index,
		IP:        vip,
		State:     0x08, // NUD_PROXY
	}

	if err := netlink.NeighAdd(neigh); err != nil {
		return fmt.Errorf("failed to add proxy neighbor entry: %w", err)
	}
	return nil
}

// delProxyEntry removes a proxy neighbor entry for the VIP using netlink.
func (n *NDPProxyManager) delProxyEntry(vip net.IP) error {
	// Delete the proxy neighbor entry
	// NUD_PROXY (0x08) is the kernel flag for proxy neighbor entries
	neigh := &netlink.Neigh{
		LinkIndex: n.link.Attrs().Index,
		IP:        vip,
		State:     0x08, // NUD_PROXY
	}

	if err := netlink.NeighDel(neigh); err != nil {
		// Ignore "no such file" errors
		if !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "message not found") {
			return fmt.Errorf("failed to delete proxy neighbor entry: %w", err)
		}
	}
	return nil
}

// IsEnabled returns true if NDP proxy is enabled for the given VIP.
func (n *NDPProxyManager) IsEnabled(vip net.IP) bool {
	if vip.To4() != nil {
		return false
	}
	n.vipsMutex.Lock()
	defer n.vipsMutex.Unlock()
	_, exists := n.vips[vip.String()]
	return exists
}
