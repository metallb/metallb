// SPDX-License-Identifier:Apache-2.0

package mac

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strings"

	"errors"

	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/routes"

	corev1 "k8s.io/api/core/v1"
)

type ipIface struct {
	Ifname   string `json:"ifname"`
	Address  string `json:"address"`
	AddrInfo []struct {
		Local     string `json:"local"`
		Prefixlen int    `json:"prefixlen"`
	} `json:"addr_info"`
}

// ForIP returns the MAC address of a given IP.
func ForIP(ip string, exec executor.Executor) (net.HardwareAddr, error) {
	address, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, fmt.Errorf("failed to parse address %q: %w", ip, err)
	}
	address = address.Unmap()
	macRe := regexp.MustCompile("([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})")

	res, err := exec.Exec("ip", []string{"neigh", "show", address.String()}...)
	if err != nil {
		return nil, err
	}

	rows := strings.Split(res, "\n")
	for _, r := range rows {
		m := macRe.FindString(r)
		if m == "" {
			continue
		}
		mac, err := net.ParseMAC(m)
		if err != nil {
			return nil, err
		}
		return mac, nil
	}

	return nil, fmt.Errorf("could not resolve MAC for ip %s", address)
}

// MatchNode returns the node with the given MAC address.
func MatchNode(nodes []corev1.Node, mac net.HardwareAddr, exec executor.Executor) (*corev1.Node, error) {
	res, err := exec.Exec("ip", []string{"neigh", "show"}...)
	if err != nil {
		return nil, err
	}

	nodesIPs := map[netip.Addr]corev1.Node{}
	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == corev1.NodeInternalIP {
				ip, err := netip.ParseAddr(a.Address)
				if err != nil {
					return nil, fmt.Errorf("invalid node internal IP %q on node %s: %w", a.Address, n.Name, err)
				}
				nodesIPs[ip.Unmap()] = n
			}
		}
	}

	rows := strings.Split(res, "\n")
	// The output of ip neigh show looks like:
	/*
		...
		172.18.0.4 dev br-97bb56038aab lladdr 02:42:ac:12:00:04 REACHABLE
		fe80::42:acff:fe12:3 dev br-97bb56038aab lladdr 02:42:ac:12:00:03 router REACHABLE
		...
	*/
	for _, r := range rows {
		if !strings.Contains(r, mac.String()) {
			continue
		}

		netIP := routes.ParseIPToken(r)
		if !netIP.IsValid() {
			continue
		}

		if n, ok := nodesIPs[netIP]; ok {
			return &n, nil
		}
	}

	return nil, fmt.Errorf("no node found for MAC: %s", mac)
}

// UpdateNodeCache pings all the nodes to update the address resolution cache.
func UpdateNodeCache(nodes []corev1.Node, exec executor.Executor) error {
	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == corev1.NodeInternalIP {
				ip, err := netip.ParseAddr(a.Address)
				if err != nil {
					return fmt.Errorf("invalid node internal IP %q on node %s: %w", a.Address, n.Name, err)
				}
				_, err = exec.Exec("ping", []string{ip.Unmap().String(), "-c", "1"}...)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// RequestAddressResolution does an ARP/NS request for the given IP.
func RequestAddressResolution(ip string, exc executor.Executor) error {
	netIP, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("failed to parse address %q: %w", ip, err)
	}
	netIP = netIP.Unmap()

	iface, err := ifaceForIPNetwork(netIP, exc)
	if err != nil {
		return err
	}

	return RequestAddressResolutionFromIface(ip, iface, exc)
}

// RequestAddressResolutionFromIface does an ARP/NS request for the given IP from the given interface.
func RequestAddressResolutionFromIface(ip string, iface string, exec executor.Executor) error {
	netIP, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("failed to parse address %q: %w", ip, err)
	}
	netIP = netIP.Unmap()
	var cmd string
	var args []string
	if netIP.Is4() {
		cmd = "arping"
		args = []string{"-c", "1", "-I", iface, netIP.String()}
	} else {
		cmd = "ndisc6"
		args = []string{netIP.String(), iface}
	}

	out, err := exec.Exec(cmd, args...)
	if err != nil {
		return errors.Join(err, errors.New(out))
	}

	return nil
}

// FlushIPNeigh flush the ip from ip neighbor table.
func FlushIPNeigh(ip string, exec executor.Executor) error {
	address, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("failed to parse address %q: %w", ip, err)
	}
	out, err := exec.Exec("ip", "neigh", "flush", "to", address.Unmap().String())
	if err != nil {
		return errors.Join(err, errors.New(out))
	}
	return nil
}

// GetIfaceMac returns the mac of the iface.
func GetIfaceMac(iface string, exec executor.Executor) (net.HardwareAddr, error) {
	out, err := exec.Exec("ifconfig")
	if err != nil {
		return nil, errors.Join(err, errors.New(out))
	}

	macRe := regexp.MustCompile("([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})")

	res := strings.Split(out, "\n")
	for _, line := range res {
		cur := strings.Split(line, " ")
		if cur[0] == iface {
			m := macRe.FindString(line)
			if m != "" {
				mac, err := net.ParseMAC(m)
				return mac, err
			}
		}
	}

	return nil, fmt.Errorf("failed to find the mac of interface %s", iface)
}

// ifaceForIPNetwork returns the interface name that is in the same network as the IP.
func ifaceForIPNetwork(ip netip.Addr, exec executor.Executor) (string, error) {
	ifaces := []ipIface{}
	res, err := exec.Exec("ip", "--json", "address", "show")
	if err != nil {
		return "", errors.Join(err, errors.New("Failed to list interfaces"))
	}

	err = json.Unmarshal([]byte(res), &ifaces)
	if err != nil {
		return "", errors.Join(err, errors.New("failed to parse interfaces"))
	}

	for _, intf := range ifaces {
		for _, addr := range intf.AddrInfo {
			local, err := netip.ParseAddr(addr.Local)
			if err != nil {
				return "", errors.Join(err, fmt.Errorf("failed to parse interface address %s %s", intf.Ifname, addr.Local))
			}
			mapped := local.Is4In6()
			local = local.Unmap()
			bits := addr.Prefixlen
			if mapped {
				bits -= 96
			}
			prefix := netip.PrefixFrom(local, bits)
			if !prefix.IsValid() {
				return "", fmt.Errorf("invalid interface prefix %s/%d on %s", addr.Local, addr.Prefixlen, intf.Ifname)
			}
			if prefix.Masked().Contains(ip.Unmap()) {
				return intf.Ifname, nil
			}
		}
	}

	return "", fmt.Errorf("no interface found in %s network", ip)
}
