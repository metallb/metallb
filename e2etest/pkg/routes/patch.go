// SPDX-License-Identifier:Apache-2.0

package routes

import (
	"fmt"
	"net"

	"errors"

	"go.universe.tf/e2etest/pkg/executor"
)

// Add route to target via for the given Executor.
func Add(exec executor.Executor, target, via, routingTable string) error {
	_, dst, err := net.ParseCIDR(target)
	if err != nil {
		return err
	}

	gw := net.ParseIP(via)

	cmd := "ip"
	args := []string{"route", "add"}
	if dst.IP.To4() == nil {
		args = []string{"-6", "route", "add"}
	}
	if routingTable != "" {
		args = append(args, "table", routingTable)
	}
	args = append(args, dst.String(), "via", gw.String())
	out, err := exec.Exec(cmd, args...)
	if err != nil {
		return errors.Join(err, fmt.Errorf("Failed to add route %s %s %s", cmd, args, out))
	}

	return nil
}

// Delete route to target via for the given Executor.
func Delete(exec executor.Executor, target, via, routingTable string) error {
	_, dst, err := net.ParseCIDR(target)
	if err != nil {
		return err
	}

	gw := net.ParseIP(via)

	cmd := "ip"
	args := []string{"route", "del"}
	if dst.IP.To4() == nil {
		args = []string{"-6", "route", "del"}
	}
	if routingTable != "" {
		args = append(args, "table", routingTable)
	}
	args = append(args, dst.String(), "via", gw.String())
	out, err := exec.Exec(cmd, args...)
	if err != nil {
		return errors.Join(err, fmt.Errorf("failed to delete route %s", out))
	}

	return nil
}
