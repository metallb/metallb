// SPDX-License-Identifier:Apache-2.0

package mac

import (
	"encoding/json"
	"fmt"
	"net"
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
	macRe := regexp.MustCompile("([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})")

	res, err := exec.Exec("ip", []string{"neigh", "show", ip}...)
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

	return nil, fmt.Errorf("could not resolve MAC for ip %s", ip)
}

// MatchNode returns the node with the given MAC address.
func MatchNode(nodes []corev1.Node, mac net.HardwareAddr, exec executor.Executor) (*corev1.Node, error) {
	res, err := exec.Exec("ip", []string{"neigh", "show"}...)
	if err != nil {
		return nil, err
	}

	nodesIPs := map[string]corev1.Node{}
	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == corev1.NodeInternalIP {
				nodesIPs[a.Address] = n
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

		ip := routes.Ipv4Re.FindString(r)
		if ip == "" {
			ip = routes.Ipv6Re.FindString(r)
		}

		if ip == "" {
			continue
		}

		netIP := net.ParseIP(ip)
		if netIP == nil {
			return nil, fmt.Errorf("failed to convert %s to net.IP", ip)
		}

		if n, ok := nodesIPs[netIP.String()]; ok {
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
				_, err := exec.Exec("ping", []string{a.Address, "-c", "1"}...)
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
	netIP := net.ParseIP(ip)
	if netIP == nil {
		return fmt.Errorf("failed to convert %s to net.IP", ip)
	}

	iface, err := ifaceForIPNetwork(netIP, exc)
	if err != nil {
		return err
	}

	return RequestAddressResolutionFromIface(ip, iface, exc)
}

// RequestAddressResolutionFromIface does an ARP/NS request for the given IP from the given interface.
func RequestAddressResolutionFromIface(ip string, iface string, exec executor.Executor) error {
	netIP := net.ParseIP(ip)
	if netIP == nil {
		return fmt.Errorf("failed to convert %s to net.IP", ip)
	}
	var cmd string
	var args []string
	if netIP.To4() != nil {
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
	out, err := exec.Exec("ip", "neigh", "flush", "to", ip)
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
func ifaceForIPNetwork(ip net.IP, exec executor.Executor) (string, error) {
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
			_, ipnet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", addr.Local, addr.Prefixlen))
			if err != nil {
				return "", errors.Join(err, fmt.Errorf("failed to parse CIDR from interface %s %s", intf.Ifname, fmt.Sprintf("%s/%d", addr.Local, addr.Prefixlen)))
			}
			if ipnet.Contains(ip) {
				return intf.Ifname, nil
			}
		}
	}

	return "", fmt.Errorf("no interface found in %s network", ip)
}
