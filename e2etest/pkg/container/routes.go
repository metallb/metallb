// SPDX-License-Identifier:Apache-2.0

package container

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"

	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/netdev"
	"go.universe.tf/e2etest/pkg/routes"
)

// Adds the routes that enable communication between execnet and tonet using the ref routes.
// The ref routes should come from the container that is connected to both execnet and tonet.
func AddMultiHop(exec executor.Executor, execnet, tonet, routingTable string, ref map[string]NetworkSettings) error {
	localNetGW, ok := ref[execnet]
	if !ok {
		return fmt.Errorf("network %s not found in %v", execnet, ref)
	}
	externalNet, ok := ref[tonet]
	if !ok {
		return fmt.Errorf("network %s not found in %v", tonet, ref)
	}

	err := routes.Add(exec, fmt.Sprintf("%s/%d", externalNet.IPAddress, externalNet.IPPrefixLen), localNetGW.IPAddress, routingTable)
	if err != nil {
		return err
	}

	err = routes.Add(exec, fmt.Sprintf("%s/%d", externalNet.GlobalIPv6Address, externalNet.GlobalIPv6PrefixLen), localNetGW.GlobalIPv6Address, routingTable)
	if err != nil {
		return err
	}

	return nil
}

// Deletes the routes that enable communication between execnet and tonet using the ref routes.
func DeleteMultiHop(exec executor.Executor, execnet, tonet, routingTable string, ref map[string]NetworkSettings) error {
	localNetGW, ok := ref[execnet]
	if !ok {
		return fmt.Errorf("network %s not found in %v", execnet, ref)
	}

	externalNet, ok := ref[tonet]
	if !ok {
		return fmt.Errorf("network %s not found in %v", tonet, ref)
	}

	err := routes.Delete(exec, fmt.Sprintf("%s/%d", externalNet.IPAddress, externalNet.IPPrefixLen), localNetGW.IPAddress, routingTable)
	if err != nil {
		return err
	}

	err = routes.Delete(exec, fmt.Sprintf("%s/%d", externalNet.GlobalIPv6Address, externalNet.GlobalIPv6PrefixLen), localNetGW.GlobalIPv6Address, routingTable)
	if err != nil {
		return err
	}

	return nil
}

// SetupVRFForNetwork takes the name of a container, a docker network and the name of a VRF
// and:
// - finds the interface corresponding to the docker network inside the container
// - creates a vrf named after vrfName if it does not exist
// - associates the interface listed above to the vrf.
func SetupVRFForNetwork(containerName, vrfNetwork, vrfName, vrfRoutingTable string) error {
	containerNetworks, err := Networks(containerName)
	if err != nil {
		return err
	}
	r, ok := containerNetworks[vrfNetwork]
	if !ok {
		return fmt.Errorf("network %s not found in container %s", vrfNetwork, containerName)
	}
	exec := executor.ForContainer(containerName)
	// Get the interface beloning to the given network
	interfaceInVRFNetwork, err := netdev.ForAddress(exec, r.IPAddress, r.GlobalIPv6Address)
	if err != nil {
		return fmt.Errorf("interface with IPs %s , %s belonging to network %s not found in container %s: %w", r.IPAddress, r.GlobalIPv6Address, vrfNetwork, containerName, err)
	}

	err = netdev.CreateVRF(exec, vrfName, vrfRoutingTable)
	if err != nil {
		return err
	}

	err = netdev.AddToVRF(exec, interfaceInVRFNetwork, vrfName, r.GlobalIPv6Address)
	if err != nil {
		return fmt.Errorf("failed to add %s to vrf %s in container %s, %w", interfaceInVRFNetwork, vrfName, containerName, err)
	}

	return nil
}

// WireContainers creates a point-to-point link between two containers.
// The second argument will be the name of the interface which will be
// common to both containers e.g. net0.
func WireContainers(containerA, containerB, dev string) error {
	netNSCntA, err := executor.Host.Exec(executor.ContainerRuntime,
		"inspect", "-f", "{{ .NetworkSettings.SandboxKey }}", containerA)
	if err != nil {
		return fmt.Errorf("%s - %w", netNSCntA, err)
	}
	netNSCntA = strings.TrimSpace(netNSCntA)

	netNSCntB, err := executor.Host.Exec(executor.ContainerRuntime,
		"inspect", "-f", "{{ .NetworkSettings.SandboxKey }}", containerB)
	if err != nil {
		return fmt.Errorf("%s - %w", netNSCntB, err)
	}
	netNSCntB = strings.TrimSpace(netNSCntB)

	if out, err := executor.Host.Exec("sudo", "ip", "link", "add", dev, "netns",
		netNSCntA, "type", "veth", "peer", "name", dev); err != nil {
		return fmt.Errorf("%s - %w", out, err)
	}

	if out, err := executor.Host.Exec("sudo", "ip", "link", "set", "dev", dev,
		"netns", netNSCntB); err != nil {
		return fmt.Errorf("%s - %w", out, err)
	}

	for _, c := range []executor.Executor{executor.ForContainer(containerA), executor.ForContainer(containerB)} {
		if out, err := c.Exec("ip", "link", "set", "dev", dev, "up"); err != nil {
			return fmt.Errorf("%s - %w", out, err)
		}
	}

	return nil
}

// BGPRoutes executes `ip route show proto bgp` in the executor and returns all
// routes filtered by device name e.g. net0. If device name is the empty string
// no filtering takes place. The return is map[destination CIDR]-> set[nextHops].
func BGPRoutes(exc executor.Executor, dev string) (map[netip.Prefix]map[netip.Addr]struct{}, error) {
	ret := make(map[netip.Prefix]map[netip.Addr]struct{})

	type Nexthop struct {
		Gateway string   `json:"gateway"`
		Dev     string   `json:"dev"`
		Weight  int      `json:"weight"`
		Flags   []string `json:"flags"`
	}

	type IPRoute struct {
		Dst      string    `json:"dst"`
		Nexthops []Nexthop `json:"nexthops,omitempty"`
		Gateway  string    `json:"gateway,omitempty"`
		Via      *struct {
			Family string `json:"family"`
			Host   string `json:"host"`
		} `json:"via,omitempty"`
		Dev string `json:"dev"`
	}

	for _, proto := range []string{"-4", "-6"} {
		out, err := exc.Exec("ip", proto, "--json", "route", "show", "proto", "bgp")
		if err != nil {
			return nil, fmt.Errorf("%s - %w", out, err)
		}

		var routes []IPRoute
		err = json.Unmarshal([]byte(out), &routes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON output: %w", err)
		}

		for _, r := range routes {
			dst, err := netip.ParsePrefix(r.Dst)
			if err != nil {
				return nil, fmt.Errorf("invalid prefix %s: %w", r.Dst, err)
			}

			nextHops := make(map[netip.Addr]struct{})
			// this is for ipv4 single next-hop
			if r.Via != nil {
				addr, err := netip.ParseAddr(r.Via.Host)
				if err != nil {
					return nil, fmt.Errorf("invalid next-hop %s: %w", r.Via.Host, err)
				}
				if dev == "" || r.Dev == dev {
					nextHops[addr] = struct{}{}
				}
			}

			// this is for ipv4 multiple next-hops
			for _, nh := range r.Nexthops {
				addr, err := netip.ParseAddr(nh.Gateway)
				if err != nil {
					return nil, fmt.Errorf("invalid next-hop %s: %w", nh.Gateway, err)
				}

				if dev == "" || nh.Dev == dev {
					nextHops[addr] = struct{}{}
				}
			}

			// this is for ipv6
			if r.Gateway != "" {
				addr, err := netip.ParseAddr(r.Gateway)
				if err != nil {
					return nil, fmt.Errorf("invalid next-hop %s: %w", r.Gateway, err)
				}
				if dev == "" || r.Dev == dev {
					nextHops[addr] = struct{}{}
				}
			}

			if len(nextHops) == 0 {
				continue
			}
			ret[dst] = nextHops
		}
	}

	return ret, nil
}
