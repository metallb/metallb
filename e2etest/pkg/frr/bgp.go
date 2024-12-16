// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"
	"strings"

	"errors"

	"go.universe.tf/e2etest/pkg/executor"

	"go.universe.tf/e2etest/pkg/ipfamily"
)

// TODO: Leaving this package "test unaware" on purpose, since we may find it
// useful for fetching informations from FRR (such as metrics) and we may need to move it
// to metallb.

// NeighborInfo returns informations for the given neighbor in the given
// executor.
func NeighborInfo(neighborName string, exec executor.Executor) (*Neighbor, error) {
	res, err := exec.Exec("vtysh", "-c", fmt.Sprintf("show bgp neighbor %s json", neighborName))

	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("Failed to query neighbour %s", neighborName))
	}
	neighbor, err := ParseNeighbour(res)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("Failed to parse neighbour %s", neighborName))
	}
	return neighbor, nil
}

type NeighborsMap map[string]*Neighbor

// NeighborsInfo returns informations for the all the neighbors in the given
// executor.
func NeighborsInfo(exec executor.Executor) (NeighborsMap, error) {
	jsonRes, err := exec.Exec("vtysh", "-c", "show bgp neighbor json")
	if err != nil {
		return nil, errors.Join(err, errors.New("Failed to query neighbours"))
	}
	neighbors, err := ParseNeighbours(jsonRes)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("Failed to parse neighbours %s", jsonRes))
	}
	res := map[string]*Neighbor{}
	for _, n := range neighbors {
		res[n.ID] = n
	}
	return res, nil
}

// Routes returns informations about routes in the given executor
// first for ipv4 routes and then for ipv6 routes.
func Routes(exec executor.Executor) (map[string]Route, map[string]Route, error) {
	return RoutesForVRF("", exec)
}

// RoutesForVRF returns informations about routes in the given executor
// first for ipv4 routes and then for ipv6 routes for the given vrf.
func RoutesForVRF(vrf string, exec executor.Executor) (map[string]Route, map[string]Route, error) {
	cmd := "show bgp ipv4 json"
	if vrf != "" {
		cmd = fmt.Sprintf("show bgp vrf %s ipv4  json", vrf)
	}
	res, err := exec.Exec("vtysh", "-c", cmd)
	if err != nil {
		return nil, nil, errors.Join(err, errors.New("Failed to query routes"))
	}
	v4Routes, err := ParseRoutes(res)
	if err != nil {
		return nil, nil, errors.Join(err, fmt.Errorf("Failed to parse routes %s", res))
	}
	cmd = "show bgp ipv6 json"
	if vrf != "" {
		cmd = fmt.Sprintf("show bgp vrf %s ipv6 json", vrf)
	}
	res, err = exec.Exec("vtysh", "-c", cmd)
	if err != nil {
		return nil, nil, errors.Join(err, errors.New("Failed to query routes"))
	}
	v6Routes, err := ParseRoutes(res)
	if err != nil {
		return nil, nil, errors.Join(err, fmt.Errorf("Failed to parse routes %s", res))
	}
	return v4Routes, v6Routes, nil
}

func RoutesForFamily(exec executor.Executor, family ipfamily.Family) (map[string]Route, error) {
	v4, v6, err := Routes(exec)
	if err != nil {
		return nil, err
	}
	switch family {
	case ipfamily.IPv4:
		return v4, nil
	case ipfamily.IPv6:
		return v6, nil
	}
	return nil, fmt.Errorf("unsupported ipfamily %v", family)
}

// RoutesForCommunity returns informations about routes in the given executor related to the given community.
func RoutesForCommunity(exec executor.Executor, communityString string, family ipfamily.Family) (map[string]Route, error) {
	// here we assume the community is formatted properly, and we just count the number
	// of elements to understand if it's large or not.
	parts := strings.Split(communityString, ":")
	communityType := "community"
	if len(parts) == 4 {
		communityType = "large-community"
		communityString = strings.Join(parts[1:], ":")
	}

	families := []string{family.String()}
	if family == ipfamily.DualStack {
		families = []string{ipfamily.IPv4.String(), ipfamily.IPv6.String()}
	}

	routes := map[string]Route{}
	for _, f := range families {
		res, err := exec.Exec("vtysh", "-c", fmt.Sprintf("show bgp %s %s %s json", f, communityType, communityString))
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("Failed to query routes for family %s %s %s", f, communityType, communityString))
		}

		r, err := ParseRoutes(res)
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("Failed to parse routes %s", res))
		}

		for k, v := range r {
			routes[k] = v
		}
	}

	return routes, nil
}

// NeighborConnected tells if the neighbor in the given
// json format is connected.
func NeighborConnected(neighborJSON string) (bool, error) {
	n, err := ParseNeighbour(neighborJSON)
	if err != nil {
		return false, err
	}
	return n.Connected, nil
}

// RawDump dumps all the low level info as a single string.
// To be used for debugging in order to print the status of the frr instance.
func RawDump(exec executor.Executor, filesToDump ...string) (string, error) {
	allerrs := errors.New("")
	res := "####### Show version\n"
	out, err := exec.Exec("vtysh", "-c", "show version")
	if err != nil {
		allerrs = errors.Join(allerrs, fmt.Errorf("\nFailed exec show version: %v", err))
	}
	res += out

	res += "####### Show running config\n"
	out, err = exec.Exec("vtysh", "-c", "show running-config")
	if err != nil {
		allerrs = errors.Join(allerrs, fmt.Errorf("\nFailed exec show bgp neighbor: %v", err))
	}
	res += out

	for _, file := range filesToDump {
		res += fmt.Sprintf("####### Dumping file %s\n", file)
		// limiting the output to 500 lines:
		out, err = exec.Exec("bash", "-c", fmt.Sprintf("cat %s | tail -n 500", file))
		if err != nil {
			allerrs = errors.Join(allerrs, fmt.Errorf("\nFailed to cat file %s: %v", file, err))
		}
		res += out
	}

	res += "####### BGP Neighbors\n"
	out, err = exec.Exec("vtysh", "-c", "show bgp neighbor")
	if err != nil {
		allerrs = errors.Join(allerrs, fmt.Errorf("\nFailed exec show bgp neighbor: %v", err))
	}
	res += out

	res += "####### Routes\n"
	out, err = exec.Exec("vtysh", "-c", "show bgp ipv4 json")
	if err != nil {
		allerrs = errors.Join(allerrs, fmt.Errorf("\nFailed exec show bgp ipv4: %v", err))
	}
	res += out

	res += "####### Routes\n"
	out, err = exec.Exec("vtysh", "-c", "show bgp ipv6 json")
	if err != nil {
		allerrs = errors.Join(allerrs, fmt.Errorf("\nFailed exec show bgp ipv6: %v", err))
	}
	res += out

	res += "####### BFD Peers\n"
	out, err = exec.Exec("vtysh", "-c", "show bfd peer")
	if err != nil {
		allerrs = errors.Join(allerrs, fmt.Errorf("\nFailed exec show bfd peer: %v", err))
	}
	res += out

	res += "####### Check for any crashinfo files\n"
	if crashInfo, err := exec.Exec("bash", "-c", "ls /var/tmp/frr/bgpd.*/crashlog"); err == nil {
		crashInfo = strings.TrimSuffix(crashInfo, "\n")
		files := strings.Split(crashInfo, "\n")
		for _, file := range files {
			res += fmt.Sprintf("####### Dumping crash file %s\n", file)
			out, err = exec.Exec("bash", "-c", fmt.Sprintf("cat %s", file))
			if err != nil {
				allerrs = errors.Join(allerrs, fmt.Errorf("\nFailed to cat bgpd crashinfo file %s: %v", file, err))
			}
			res += out
		}
	}

	res += "####### Network setup for host\n"
	out, err = exec.Exec("bash", "-c", "ip -6 route; ip -4 route")
	if err != nil {
		allerrs = errors.Join(allerrs, fmt.Errorf("\nFailed exec to print network setup: %v", err))
	}
	res += out

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
		return errors.Join(err, fmt.Errorf("show community %s doesn't include %s", res, community))
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
