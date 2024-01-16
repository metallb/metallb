// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"sort"

	"go.universe.tf/metallb/internal/config"
)

func toConfig(fromK8s config.ClusterResources, validate config.Validate) (*config.Config, error) {
	resources := config.ClusterResources{
		Pools:           sortedCopy(fromK8s.Pools),
		Peers:           sortedCopy(fromK8s.Peers),
		BFDProfiles:     sortedCopy(fromK8s.BFDProfiles),
		L2Advs:          sortedCopy(fromK8s.L2Advs),
		BGPAdvs:         sortedCopy(fromK8s.BGPAdvs),
		Communities:     sortedCopy(fromK8s.Communities),
		PasswordSecrets: fromK8s.PasswordSecrets,
		Nodes:           sortedCopy(fromK8s.Nodes),
		Namespaces:      sortedCopy(fromK8s.Namespaces),
		BGPExtras:       fromK8s.BGPExtras,
	}

	cfg, err := config.For(resources, validate)
	return cfg, err
}

// We need to do this ballet because we need to leverage the GetName() function
// of the objects, but the interface is implemented by the pointer, not the object,
// whereas what we are given with .Items is the slice of objects.
func sortedCopy[T any, PT interface {
	GetName() string
	*T
}](toSort []T) []T {
	res := make([]T, len(toSort))
	copy(res, toSort)
	sort.Slice(res, func(i, j int) bool {
		first := PT(&toSort[i])
		second := PT(&toSort[j])
		return first.GetName() < second.GetName()
	})
	return res
}
