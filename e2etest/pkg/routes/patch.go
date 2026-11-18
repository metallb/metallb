// SPDX-License-Identifier:Apache-2.0

package routes

import (
	"fmt"
	"net/netip"

	"errors"

	"go.universe.tf/e2etest/pkg/executor"
)

// Add route to target via for the given Executor.
func Add(exec executor.Executor, target, via, routingTable string) error {
	dst, err := netip.ParsePrefix(target)
	if err != nil {
		return err
	}
	dst = dst.Masked()
	dst = normalizePrefix(dst)

	gw, err := netip.ParseAddr(via)
	if err != nil {
		return fmt.Errorf("invalid gateway address %q: %w", via, err)
	}
	gw = gw.Unmap()

	cmd := "ip"
	args := []string{"route", "add"}
	if !dst.Addr().Is4() {
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
	dst, err := netip.ParsePrefix(target)
	if err != nil {
		return err
	}
	dst = dst.Masked()
	dst = normalizePrefix(dst)

	gw, err := netip.ParseAddr(via)
	if err != nil {
		return fmt.Errorf("invalid gateway address %q: %w", via, err)
	}
	gw = gw.Unmap()

	cmd := "ip"
	args := []string{"route", "del"}
	if !dst.Addr().Is4() {
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

func normalizePrefix(prefix netip.Prefix) netip.Prefix {
	if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
		return netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
	}
	return prefix
}
