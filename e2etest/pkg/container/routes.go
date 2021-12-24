// SPDX-License-Identifier:Apache-2.0

package container

import (
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
