// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"go.universe.tf/metallb/e2etest/pkg/executor"

	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
	"go.universe.tf/metallb/internal/ipfamily"
)

// TODO: Leaving this package "test unaware" on purpose, since we may find it
// useful for fetching informations from FRR (such as metrics) and we may need to move it
// to metallb.

// NeighborForContainer returns informations for the given neighbor in the given
// executor.
func NeighborInfo(neighborName, exec executor.Executor) (*bgpfrr.Neighbor, error) {
	res, err := exec.Exec("vtysh", "-c", fmt.Sprintf("show bgp neighbor %s json", neighborName))

	if err != nil {
		return nil, errors.Wrapf(err, "Failed to query neighbour %s", neighborName)
	}
	neighbor, err := bgpfrr.ParseNeighbour(res)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse neighbour %s", neighborName)
	}
	return neighbor, nil
}

// NeighborsForContainer returns informations for the all the neighbors in the given
// executor.
func NeighborsInfo(exec executor.Executor) ([]*bgpfrr.Neighbor, error) {
	res, err := exec.Exec("vtysh", "-c", "show bgp neighbor json")
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to query neighbours")
	}
	neighbors, err := bgpfrr.ParseNeighbours(res)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse neighbours %s", res)
	}
	return neighbors, nil
}

// Routes returns informations about routes in the given executor
// first for ipv4 routes and then for ipv6 routes.
func Routes(exec executor.Executor) (map[string]bgpfrr.Route, map[string]bgpfrr.Route, error) {
	res, err := exec.Exec("vtysh", "-c", "show bgp ipv4 json")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to query routes")
	}
	v4Routes, err := bgpfrr.ParseRoutes(res)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to parse routes %s", res)
	}
	res, err = exec.Exec("vtysh", "-c", "show bgp ipv6 json")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to query routes")
	}
	v6Routes, err := bgpfrr.ParseRoutes(res)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to parse routes %s", res)
	}
	return v4Routes, v6Routes, nil
}

// RoutesForCommunity returns informations about routes in the given executor related to the given community.
func RoutesForCommunity(exec executor.Executor, community, family ipfamily.Family) (map[string]bgpfrr.Route, error) {
	res, err := exec.Exec("vtysh", "-c", fmt.Sprintf("show bgp %s community %s json", family, community))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to query routes")
	}
	routes, err := bgpfrr.ParseRoutes(res)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse routes %s", res)
	}
	return routes, nil
}

// NeighborConnected tells if the neighbor in the given
// json format is connected.
func NeighborConnected(neighborJson string) (bool, error) {
	n, err := bgpfrr.ParseNeighbour(neighborJson)
	if err != nil {
		return false, err
	}
	return n.Connected, nil
}

// RawDump dumps all the low level info as a single string.
// To be used for debugging in order to print the status of the frr instance.
func RawDump(exec executor.Executor, filesToDump ...string) (string, error) {
	allerrs := errors.New("")

	res := "####### Show running config\n"
	out, err := exec.Exec("vtysh", "-c", "show running-config")
	if err != nil {
		allerrs = errors.Wrapf(allerrs, "\nFailed exec show bgp neighbor: %v", err)
	}
	res = res + out

	for _, file := range filesToDump {
		res = res + fmt.Sprintf("####### Dumping file %s\n", file)
		// limiting the output to 500 lines:
		out, err = exec.Exec("bash", "-c", fmt.Sprintf("cat %s | tail -n 500", file))
		if err != nil {
			allerrs = errors.Wrapf(allerrs, "\nFailed to cat file %s: %v", file, err)
		}
		res = res + out
	}

	res = res + "####### BGP Neighbors\n"
	out, err = exec.Exec("vtysh", "-c", "show bgp neighbor")
	if err != nil {
		allerrs = errors.Wrapf(allerrs, "\nFailed exec show bgp neighbor: %v", err)
	}
	res = res + out

	res = res + "####### BFD Peers\n"
	out, err = exec.Exec("vtysh", "-c", "show bfd peer")
	if err != nil {
		allerrs = errors.Wrapf(allerrs, "\nFailed exec show bfd peer: %v", err)
	}
	res = res + out

	res = res + "####### Check for any crashinfo files\n"
	if crashInfo, err := exec.Exec("bash", "-c", "ls /var/tmp/frr/bgpd.*/crashlog"); err == nil {
		crashInfo = strings.TrimSuffix(crashInfo, "\n")
		files := strings.Split(crashInfo, "\n")
		for _, file := range files {
			res = res + fmt.Sprintf("####### Dumping crash file %s\n", file)
			out, err = exec.Exec("bash", "-c", fmt.Sprintf("cat %s", file))
			if err != nil {
				allerrs = errors.Wrapf(allerrs, "\nFailed to cat bgpd crashinfo file %s: %v", file, err)
			}
			res = res + out
		}
	}

	if allerrs.Error() == "" {
		allerrs = nil
	}

	return res, allerrs
}

// ContainsCommunity check if the passed in community string exists in show bgp community.
func ContainsCommunity(exec executor.Executor, community string) error {
	res, err := exec.Exec("vtysh", "-c", "show bgp community-info")
	if err != nil {
		return err
	}
	if !strings.Contains(res, community) {
		return errors.Wrapf(err, "show community %s doesn't include %s", res, community)
	}
	return nil
}

// LocalPrefForPrefix returns the localPref value for the given prefix.
func LocalPrefForPrefix(exec executor.Executor, prefix string, family ipfamily.Family) (uint32, error) {
	routes, v6Routes, err := Routes(exec)
	if err != nil {
		return 0, err
	}
	if family == ipfamily.IPv6 {
		routes = v6Routes
	}
	route, ok := routes[prefix]
	if !ok {
		return 0, fmt.Errorf("prefix %s not found in routes", prefix)
	}
	return route.LocalPref, nil
}
