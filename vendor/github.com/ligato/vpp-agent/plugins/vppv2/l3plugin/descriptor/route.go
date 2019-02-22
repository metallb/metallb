// Copyright (c) 2018 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package descriptor

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	"github.com/pkg/errors"

	"github.com/ligato/cn-infra/logging"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	ifdescriptor "github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/vppcalls"
)

const (
	// RouteDescriptorName is the name of the descriptor for static routes.
	RouteDescriptorName = "vpp-route"

	// dependency labels
	routeOutInterfaceDep = "interface-exists"

	// static route weight by default
	defaultWeight = 1
)

// RouteDescriptor teaches KVScheduler how to configure VPP routes.
type RouteDescriptor struct {
	log          logging.Logger
	routeHandler vppcalls.RouteVppAPI
}

// NewRouteDescriptor creates a new instance of the Route descriptor.
func NewRouteDescriptor(
	routeHandler vppcalls.RouteVppAPI, log logging.PluginLogger) *RouteDescriptor {

	return &RouteDescriptor{
		routeHandler: routeHandler,
		log:          log.NewLogger("static-route-descriptor"),
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *RouteDescriptor) GetDescriptor() *adapter.RouteDescriptor {
	return &adapter.RouteDescriptor{
		Name:            RouteDescriptorName,
		NBKeyPrefix:     l3.ModelRoute.KeyPrefix(),
		ValueTypeName:   l3.ModelRoute.ProtoName(),
		KeySelector:     l3.ModelRoute.IsKeyValid,
		ValueComparator: d.EquivalentRoutes,
		Add:             d.Add,
		Delete:          d.Delete,
		ModifyWithRecreate: func(key string, oldValue, newValue *l3.Route, metadata interface{}) bool {
			return true
		},
		Dependencies:     d.Dependencies,
		Dump:             d.Dump,
		DumpDependencies: []string{ifdescriptor.InterfaceDescriptorName},
	}
}

// EquivalentRoutes is case-insensitive comparison function for l3.Route.
func (d *RouteDescriptor) EquivalentRoutes(key string, oldRoute, newRoute *l3.Route) bool {
	if oldRoute.GetType() != newRoute.GetType() ||
		oldRoute.GetVrfId() != newRoute.GetVrfId() ||
		oldRoute.GetViaVrfId() != newRoute.GetViaVrfId() ||
		oldRoute.GetOutgoingInterface() != newRoute.GetOutgoingInterface() ||
		getWeight(oldRoute) != getWeight(newRoute) ||
		oldRoute.GetPreference() != newRoute.GetPreference() {
		return false
	}

	// compare dst networks
	if !equalNetworks(oldRoute.DstNetwork, newRoute.DstNetwork) {
		return false
	}

	// compare gw addresses (next hop)
	if !equalAddrs(getGwAddr(oldRoute), getGwAddr(newRoute)) {
		return false
	}

	return true
}

// Add adds VPP static route.
func (d *RouteDescriptor) Add(key string, route *l3.Route) (metadata interface{}, err error) {
	if err = validateRoute(route); err != nil {
		return nil, err
	}

	err = d.routeHandler.VppAddRoute(route)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func validateRoute(route *l3.Route) error {
	_, ipNet, err := net.ParseCIDR(route.DstNetwork)
	if err != nil {
		return err
	}
	if ipNet.String() != route.DstNetwork {
		return fmt.Errorf("DstNetwork must represent IP network")
	}
	return nil
}

// Delete removes VPP static route.
func (d *RouteDescriptor) Delete(key string, route *l3.Route, metadata interface{}) error {
	err := d.routeHandler.VppDelRoute(route)
	if err != nil {
		return err
	}

	return nil
}

// Dependencies lists dependencies for a VPP route.
func (d *RouteDescriptor) Dependencies(key string, route *l3.Route) []kvs.Dependency {
	var dependencies []kvs.Dependency
	// the outgoing interface must exist and be UP
	if route.OutgoingInterface != "" {
		dependencies = append(dependencies, kvs.Dependency{
			Label: routeOutInterfaceDep,
			Key:   interfaces.InterfaceKey(route.OutgoingInterface),
		})
	}
	// TODO: perhaps check GW routability
	return dependencies
}

// Dump returns all routes associated with interfaces managed by this agent.
func (d *RouteDescriptor) Dump(correlate []adapter.RouteKVWithMetadata) (
	dump []adapter.RouteKVWithMetadata, err error,
) {
	// Retrieve VPP route configuration
	Routes, err := d.routeHandler.DumpRoutes()
	if err != nil {
		return nil, errors.Errorf("failed to dump VPP routes: %v", err)
	}

	for _, Route := range Routes {
		dump = append(dump, adapter.RouteKVWithMetadata{
			Key:    l3.RouteKey(Route.Route.VrfId, Route.Route.DstNetwork, Route.Route.NextHopAddr),
			Value:  Route.Route,
			Origin: kvs.UnknownOrigin,
		})
	}

	return dump, nil
}

// equalAddrs compares two IP addresses for equality.
func equalAddrs(addr1, addr2 string) bool {
	a1 := net.ParseIP(addr1)
	a2 := net.ParseIP(addr2)
	if a1 == nil || a2 == nil {
		// if parsing fails, compare as strings
		return strings.ToLower(addr1) == strings.ToLower(addr2)
	}
	return a1.Equal(a2)
}

// getGwAddr returns the GW address chosen in the given route, handling the cases
// when it is left undefined.
func getGwAddr(route *l3.Route) string {
	if route.GetNextHopAddr() != "" {
		return route.GetNextHopAddr()
	}
	// return zero address
	_, dstIPNet, err := net.ParseCIDR(route.GetDstNetwork())
	if err != nil {
		return ""
	}
	if dstIPNet.IP.To4() == nil {
		return net.IPv6zero.String()
	}
	return net.IPv4zero.String()
}

// getWeight returns static route weight, handling the cases when it is left undefined.
func getWeight(route *l3.Route) uint32 {
	if route.Weight == 0 {
		return defaultWeight
	}
	return route.Weight
}

// equalNetworks compares two IP networks for equality.
func equalNetworks(net1, net2 string) bool {
	_, n1, err1 := net.ParseCIDR(net1)
	_, n2, err2 := net.ParseCIDR(net2)
	if err1 != nil || err2 != nil {
		// if parsing fails, compare as strings
		return strings.ToLower(net1) == strings.ToLower(net2)
	}
	return n1.IP.Equal(n2.IP) && bytes.Equal(n1.Mask, n2.Mask)
}
