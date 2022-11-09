// SPDX-License-Identifier:Apache-2.0

package container

import (
	"errors"
	"fmt"

	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/routes"
)

// Adds the routes that enable communication between execnet and tonet using the ref routes.
// The ref routes should come from the container that is connected to both execnet and tonet.
func AddMultiHop(exec executor.Executor, execnet, tonet string, ref map[string]NetworkSettings) error {
	localNetGW, ok := ref[execnet]
	if !ok {
		return fmt.Errorf("network %s not found in %v", execnet, ref)
	}

	externalNet, ok := ref[tonet]
	if !ok {
		return fmt.Errorf("network %s not found in %v", tonet, ref)
	}

	err := routes.Add(exec, fmt.Sprintf("%s/%d", externalNet.IPAddress, externalNet.IPPrefixLen), localNetGW.IPAddress)
	if err != nil {
		return err
	}

	err = routes.Add(exec, fmt.Sprintf("%s/%d", externalNet.GlobalIPv6Address, externalNet.GlobalIPv6PrefixLen), localNetGW.GlobalIPv6Address)
	if err != nil {
		return err
	}

	return nil
}

// Deletes the routes that enable communication between execnet and tonet using the ref routes.
func DeleteMultiHop(exec executor.Executor, execnet, tonet string, ref map[string]NetworkSettings) error {
	localNetGW, ok := ref[execnet]
	if !ok {
		return fmt.Errorf("network %s not found in %v", execnet, ref)
	}

	externalNet, ok := ref[tonet]
	if !ok {
		return fmt.Errorf("network %s not found in %v", tonet, ref)
	}

	err := routes.Delete(exec, fmt.Sprintf("%s/%d", externalNet.IPAddress, externalNet.IPPrefixLen), localNetGW.IPAddress)
	if err != nil {
		return err
	}

	err = routes.Delete(exec, fmt.Sprintf("%s/%d", externalNet.GlobalIPv6Address, externalNet.GlobalIPv6PrefixLen), localNetGW.GlobalIPv6Address)
	if err != nil {
		return err
	}

	return nil
}

func AddNetworkToVRF(containerName, vrfNetwork, vrfName string) error {
	containerRoutes, err := Networks(containerName)
	if err != nil {
		return err
	}
	r, ok := containerRoutes[vrfNetwork]
	if !ok {
		return fmt.Errorf("Network %s not found in container %s", vrfNetwork, containerName)
	}
	exec := executor.ForContainer(containerName)
	// Get the interface beloning to the given network
	interfaceInVRFNetwork, err := routes.InterfaceForAddress(exec, r.IPAddress, r.GlobalIPv6Address)
	if err != nil {
		return fmt.Errorf("interface with IPs %s , %s belonging to network %s not found in container %s", r.IPAddress, r.GlobalIPv6Address, vrfNetwork, containerName)
	}
	err = routes.InterfaceExists(exec, vrfName)
	var notFound *routes.InterfaceNotFoundErr
	if err != nil && !errors.As(err, &notFound) {
		return err
	}
	if errors.As(err, &notFound) {
		err := routes.CreateVRF(exec, vrfName)
		if err != nil {
			return err
		}
	}

	err = routes.AddInterfaceToVRF(exec, interfaceInVRFNetwork, vrfName, r.GlobalIPv6Address)
	if err != nil {
		return fmt.Errorf("failed to add %s to vrf %s in container %s, %w", interfaceInVRFNetwork, vrfName, containerName, err)
	}

	return nil
}
