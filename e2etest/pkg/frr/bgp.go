// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"

	"github.com/pkg/errors"
	"go.universe.tf/metallb/e2etest/pkg/executor"
)

// TODO: Leaving this package "test unaware" on purpose, since we may find it
// useful for fetching informations from FRR (such as metrics) and we may need to move it
// to metallb.

// NeighborForContainer returns informations for the given neighbor in the given
// executor.
func NeighborInfo(neighborName, exec executor.Executor) (*Neighbor, error) {
	res, err := exec.Exec("vtysh", "-c", fmt.Sprintf("show bgp neighbor %s json", neighborName))

	if err != nil {
		return nil, errors.Wrapf(err, "Failed to query neighbour %s", neighborName)
	}
	neighbor, err := parseNeighbour(res)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse neighbour %s", neighborName)
	}
	return neighbor, nil
}

// NeighborsForContainer returns informations for the all the neighbors in the given
// executor.
func NeighborsInfo(exec executor.Executor) ([]*Neighbor, error) {
	res, err := exec.Exec("vtysh", "-c", "show bgp neighbor json")
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to query neighbours")
	}
	neighbors, err := parseNeighbours(res)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse neighbours %s", res)
	}
	return neighbors, nil
}

// Routes returns informations about routes in the given executor
// first for ipv4 routes and then for ipv6 routes.
func Routes(exec executor.Executor) (map[string]Route, map[string]Route, error) {
	res, err := exec.Exec("vtysh", "-c", "show bgp ipv4 json")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to query routes")
	}
	v4Routes, err := parseRoutes(res)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to parse routes %s", res)
	}
	res, err = exec.Exec("vtysh", "-c", "show bgp ipv6 json")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to query routes")
	}
	v6Routes, err := parseRoutes(res)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to parse routes %s", res)
	}
	return v4Routes, v6Routes, nil
}
