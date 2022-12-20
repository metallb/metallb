// SPDX-License-Identifier:Apache-2.0

package container

import (
	"fmt"

	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/netdev"
	"go.universe.tf/metallb/e2etest/pkg/routes"
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
		return fmt.Errorf("Network %s not found in container %s", vrfNetwork, containerName)
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
