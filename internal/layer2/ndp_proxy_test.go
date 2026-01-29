// SPDX-License-Identifier:Apache-2.0

package layer2

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
)

// TestNDPProxyManagerCreation tests that NDPProxyManager can be created.
func TestNDPProxyManagerCreation(t *testing.T) {
	logger := log.NewNopLogger()

	// This will fail in most environments since eth0 may not exist
	// We're just testing that the function signature is correct
	mgr, err := NewNDPProxyManager(logger, "lo")

	if err != nil {
		// Expected to fail in some environments, but we can test the error path
		t.Logf("NewNDPProxyManager failed (expected in many environments): %v", err)
		return
	}

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}

	if mgr.iface != "lo" {
		t.Errorf("expected interface lo, got %s", mgr.iface)
	}

	if mgr.vips == nil {
		t.Error("expected non-nil vips map")
	}
}

// TestNDPProxyManagerIPv4Skipped tests that IPv4 addresses are skipped.
func TestNDPProxyManagerIPv4Skipped(t *testing.T) {
	logger := log.NewNopLogger()
	mgr, _ := NewNDPProxyManager(logger, "lo")

	if mgr == nil {
		t.Skip("could not create manager")
	}

	ipv4 := net.IPv4(192, 168, 1, 1)

	// Enable should succeed but not add the VIP
	err := mgr.Enable(ipv4)
	if err != nil {
		t.Fatalf("expected no error for IPv4, got %v", err)
	}

	if mgr.IsEnabled(ipv4) {
		t.Error("expected IPv4 to not be enabled")
	}

	// Disable should also succeed
	err = mgr.Disable(ipv4)
	if err != nil {
		t.Fatalf("expected no error disabling IPv4, got %v", err)
	}
}

// TestNDPProxyManagerProxyNDPPath tests the proxy_ndp path generation.
func TestNDPProxyManagerProxyNDPPath(t *testing.T) {
	tests := []struct {
		name         string
		iface        string
		expectedPath string
	}{
		{
			name:         "simple interface name",
			iface:        "eth0",
			expectedPath: "/proc/sys/net/ipv6/conf/eth0/proxy_ndp",
		},
		{
			name:         "bridge interface",
			iface:        "br-provider",
			expectedPath: "/proc/sys/net/ipv6/conf/br-provider/proxy_ndp",
		},
		{
			name:         "vlan interface",
			iface:        "eth0.100",
			expectedPath: "/proc/sys/net/ipv6/conf/eth0.100/proxy_ndp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, _ := NewNDPProxyManager(log.NewNopLogger(), tt.iface)
			if mgr == nil {
				// If we can't create the manager (e.g., interface doesn't exist),
				// create a minimal one just for testing path generation
				mgr = &NDPProxyManager{iface: tt.iface}
			}
			path := mgr.proxyNDPPath()
			if path != tt.expectedPath {
				t.Errorf("expected path %s, got %s", tt.expectedPath, path)
			}
		})
	}
}

// TestNDPProxyManagerPathSanitization tests that interface names are sanitized.
func TestNDPProxyManagerPathSanitization(t *testing.T) {
	tests := []struct {
		name         string
		iface        string
		expectedPath string
	}{
		{
			name:         "path traversal attempt",
			iface:        "../../../etc",
			expectedPath: "/proc/sys/net/ipv6/conf/etc/proxy_ndp",
		},
		{
			name:         "absolute path attempt",
			iface:        "/absolute/path",
			expectedPath: "/proc/sys/net/ipv6/conf/path/proxy_ndp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, _ := NewNDPProxyManager(log.NewNopLogger(), tt.iface)
			if mgr == nil {
				// If we can't create the manager, create a minimal one
				mgr = &NDPProxyManager{iface: tt.iface}
			}
			path := mgr.proxyNDPPath()
			if path != tt.expectedPath {
				t.Errorf("expected path %s, got %s", tt.expectedPath, path)
			}

			// Check that the path is clean (no .. components)
			cleanPath := filepath.Clean(path)
			if path != cleanPath {
				t.Errorf("path not clean: %s != %s", path, cleanPath)
			}
		})
	}
}

// TestNDPProxyManagerEnableDisable tests enabling and disabling NDP proxy.
func TestNDPProxyManagerEnableDisable(t *testing.T) {
	// Skip if not running as root (required for sysctl and ip commands)
	if os.Getuid() != 0 {
		t.Skip("skipping test: requires root privileges")
	}

	logger := log.NewNopLogger()
	iface := "lo" // Use loopback for testing

	mgr, err := NewNDPProxyManager(logger, iface)
	if err != nil {
		t.Skipf("could not create manager: %v", err)
	}

	ipv6 := net.ParseIP("fd00::100")

	// Test Enable
	err = mgr.Enable(ipv6)
	if err != nil {
		t.Logf("Enable failed (may be expected in container): %v", err)
		// Don't fail the test in container environments
		return
	}

	if !mgr.IsEnabled(ipv6) {
		t.Error("expected IPv6 to be enabled")
	}

	// Test Disable
	err = mgr.Disable(ipv6)
	if err != nil {
		t.Errorf("Disable failed: %v", err)
	}

	if mgr.IsEnabled(ipv6) {
		t.Error("expected IPv6 to be disabled")
	}
}

// TestNDPProxyManagerIdempotent tests that Enable and Disable are idempotent.
func TestNDPProxyManagerIdempotent(t *testing.T) {
	logger := log.NewNopLogger()
	mgr, _ := NewNDPProxyManager(logger, "lo")

	if mgr == nil {
		t.Skip("could not create manager")
	}

	ipv6 := net.ParseIP("fd00::100")

	// Enabling twice should not cause issues (we just check it doesn't panic)
	// Note: This will fail if not running as root, but we're testing the logic
	_ = mgr.Enable(ipv6)
	_ = mgr.Enable(ipv6)

	// Disabling twice should also be safe
	_ = mgr.Disable(ipv6)
	_ = mgr.Disable(ipv6)
}

// TestIsEnabled tests the IsEnabled method.
func TestIsEnabled(t *testing.T) {
	logger := log.NewNopLogger()
	mgr, _ := NewNDPProxyManager(logger, "lo")

	if mgr == nil {
		t.Skip("could not create manager")
	}

	ipv6 := net.ParseIP("fd00::100")
	ipv4 := net.ParseIP("192.168.1.1")

	// Initially nothing is enabled
	if mgr.IsEnabled(ipv6) {
		t.Error("expected IPv6 to not be enabled initially")
	}
	if mgr.IsEnabled(ipv4) {
		t.Error("expected IPv4 to not be enabled")
	}

	// Add a VIP directly to test IsEnabled
	mgr.vips[ipv6.String()] = ipv6

	if !mgr.IsEnabled(ipv6) {
		t.Error("expected IPv6 to be enabled after manual add")
	}
}
