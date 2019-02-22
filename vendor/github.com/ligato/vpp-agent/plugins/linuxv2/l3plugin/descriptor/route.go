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
	"net"
	"strings"

	prototypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/addrs"

	ifmodel "github.com/ligato/vpp-agent/api/models/linux/interfaces"
	"github.com/ligato/vpp-agent/api/models/linux/l3"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin"
	ifdescriptor "github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/linuxv2/l3plugin/descriptor/adapter"
	l3linuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/l3plugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin"
	nslinuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin/linuxcalls"
)

const (
	// RouteDescriptorName is the name of the descriptor for Linux routes.
	RouteDescriptorName = "linux-route"

	// IP addresses matching any destination.
	ipv4AddrAny = "0.0.0.0"
	ipv6AddrAny = "::"

	// dependency labels
	routeOutInterfaceDep   = "outgoing-interface-exists"
	routeGwReachabilityDep = "gw-reachable"
)

// A list of non-retriable errors:
var (
	// ErrRouteWithoutInterface is returned when Linux Route configuration is missing
	// outgoing interface reference.
	ErrRouteWithoutInterface = errors.New("Linux Route defined without outgoing interface reference")

	// ErrRouteWithoutDestination is returned when Linux Route configuration is missing destination network.
	ErrRouteWithoutDestination = errors.New("Linux Route defined without destination network")

	// ErrRouteWithUndefinedScope is returned when Linux Route is configured without scope.
	ErrRouteWithUndefinedScope = errors.New("Linux Route defined without scope")

	// ErrRouteWithInvalidDst is returned when Linux Route configuration contains destination
	// network that cannot be parsed.
	ErrRouteWithInvalidDst = errors.New("Linux Route defined with invalid destination network")

	// ErrRouteWithInvalidGW is returned when Linux Route configuration contains gateway
	// address that cannot be parsed.
	ErrRouteWithInvalidGw = errors.New("Linux Route defined with invalid GW address")

	// ErrRouteLinkWithGw is returned when link-local Linux route has gateway address
	// specified - it shouldn't be since destination is already neighbour by definition.
	ErrRouteLinkWithGw = errors.New("Link-local Linux Route was defined with non-empty GW address")
)

// RouteDescriptor teaches KVScheduler how to configure Linux routes.
type RouteDescriptor struct {
	log       logging.Logger
	l3Handler l3linuxcalls.NetlinkAPI
	ifPlugin  ifplugin.API
	nsPlugin  nsplugin.API
	scheduler kvs.KVScheduler

	// parallelization of the Dump operation
	dumpGoRoutinesCnt int
}

// NewRouteDescriptor creates a new instance of the Route descriptor.
func NewRouteDescriptor(
	scheduler kvs.KVScheduler, ifPlugin ifplugin.API, nsPlugin nsplugin.API,
	l3Handler l3linuxcalls.NetlinkAPI, log logging.PluginLogger, dumpGoRoutinesCnt int) *RouteDescriptor {

	return &RouteDescriptor{
		scheduler:         scheduler,
		l3Handler:         l3Handler,
		ifPlugin:          ifPlugin,
		nsPlugin:          nsPlugin,
		dumpGoRoutinesCnt: dumpGoRoutinesCnt,
		log:               log.NewLogger("route-descriptor"),
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *RouteDescriptor) GetDescriptor() *adapter.RouteDescriptor {
	return &adapter.RouteDescriptor{
		Name:               RouteDescriptorName,
		NBKeyPrefix:        linux_l3.ModelRoute.KeyPrefix(),
		ValueTypeName:      linux_l3.ModelRoute.ProtoName(),
		KeySelector:        linux_l3.ModelRoute.IsKeyValid,
		KeyLabel:           linux_l3.ModelRoute.StripKeyPrefix,
		ValueComparator:    d.EquivalentRoutes,
		Validate:           d.Validate,
		Add:                d.Add,
		Delete:             d.Delete,
		Modify:             d.Modify,
		Dependencies:       d.Dependencies,
		DerivedValues:      d.DerivedValues,
		Dump:               d.Dump,
		DumpDependencies:   []string{ifdescriptor.InterfaceDescriptorName},
	}
}

// EquivalentRoutes is case-insensitive comparison function for l3.LinuxRoute.
func (d *RouteDescriptor) EquivalentRoutes(key string, oldRoute, newRoute *linux_l3.Route) bool {
	// attributes compared as usually:
	if oldRoute.OutgoingInterface != newRoute.OutgoingInterface ||
		oldRoute.Scope != newRoute.Scope ||
		oldRoute.Metric != newRoute.Metric {
		return false
	}

	// compare IP addresses converted to net.IP(Net)
	if !equalNetworks(oldRoute.DstNetwork, newRoute.DstNetwork) {
		return false
	}
	return equalAddrs(getGwAddr(oldRoute), getGwAddr(newRoute))
}

// Validate validates static route configuration.
func (d *RouteDescriptor) Validate(key string, route *linux_l3.Route) (err error) {
	if route.OutgoingInterface == "" {
		return kvs.NewInvalidValueError(ErrRouteWithoutInterface, "outgoing_interface")
	}
	if route.DstNetwork == "" {
		return kvs.NewInvalidValueError(ErrRouteWithoutDestination, "dst_network")
	}
	if route.Scope == linux_l3.Route_LINK && route.GwAddr != "" {
		return kvs.NewInvalidValueError(ErrRouteLinkWithGw, "scope", "gw_addr")
	}
	return nil
}

// Add adds Linux route.
func (d *RouteDescriptor) Add(key string, route *linux_l3.Route) (metadata interface{}, err error) {
	err = d.updateRoute(route, "add", d.l3Handler.AddRoute)
	return nil, err
}

// Delete removes Linux route.
func (d *RouteDescriptor) Delete(key string, route *linux_l3.Route, metadata interface{}) error {
	return d.updateRoute(route, "delete", d.l3Handler.DelRoute)
}

// Modify is able to change route scope, metric and GW address.
func (d *RouteDescriptor) Modify(key string, oldRoute, newRoute *linux_l3.Route, oldMetadata interface{}) (newMetadata interface{}, err error) {
	err = d.updateRoute(newRoute, "modify", d.l3Handler.ReplaceRoute)
	return nil, err
}

// updateRoute adds, modifies or deletes a Linux route.
func (d *RouteDescriptor) updateRoute(route *linux_l3.Route, actionName string, actionClb func(route *netlink.Route) error) error {
	var err error

	// Prepare Netlink Route object
	netlinkRoute := &netlink.Route{}

	// Get interface metadata
	ifMeta, found := d.ifPlugin.GetInterfaceIndex().LookupByName(route.OutgoingInterface)
	if !found || ifMeta == nil {
		err = errors.Errorf("failed to obtain metadata for interface %s", route.OutgoingInterface)
		d.log.Error(err)
		return err
	}

	// set link index
	netlinkRoute.LinkIndex = ifMeta.LinuxIfIndex

	// set destination network
	_, dstNet, err := net.ParseCIDR(route.DstNetwork)
	if err != nil {
		err = ErrRouteWithInvalidDst
		d.log.Error(err)
		return err
	}
	netlinkRoute.Dst = dstNet

	// set gateway address
	if route.GwAddr != "" {
		gwAddr := net.ParseIP(route.GwAddr)
		if gwAddr == nil {
			err = ErrRouteWithInvalidGw
			d.log.Error(err)
			return err
		}
		netlinkRoute.Gw = gwAddr
	}

	// set route scope
	scope, err := rtScopeFromNBToNetlink(route.Scope)
	if err != nil {
		d.log.Error(err)
		return err
	}
	netlinkRoute.Scope = scope

	// set route metric
	netlinkRoute.Priority = int(route.Metric)

	// move to the namespace of the associated interface
	nsCtx := nslinuxcalls.NewNamespaceMgmtCtx()
	revertNs, err := d.nsPlugin.SwitchToNamespace(nsCtx, ifMeta.Namespace)
	if err != nil {
		err = errors.Errorf("failed to switch namespace: %v", err)
		d.log.Error(err)
		return err
	}
	defer revertNs()

	// update route in the interface namespace
	err = actionClb(netlinkRoute)
	if err != nil {
		err = errors.Errorf("failed to %s linux route: %v", actionName, err)
		d.log.Error(err)
		return err
	}

	return nil
}

// Dependencies lists dependencies for a Linux route.
func (d *RouteDescriptor) Dependencies(key string, route *linux_l3.Route) []kvs.Dependency {
	var dependencies []kvs.Dependency
	// the outgoing interface must exist and be UP
	if route.OutgoingInterface != "" {
		dependencies = append(dependencies, kvs.Dependency{
			Label: routeOutInterfaceDep,
			Key:   ifmodel.InterfaceStateKey(route.OutgoingInterface, true),
		})
	}
	// GW must be routable
	gwAddr := net.ParseIP(getGwAddr(route))
	if gwAddr != nil && !gwAddr.IsUnspecified() {
		dependencies = append(dependencies, kvs.Dependency{
			Label: routeGwReachabilityDep,
			AnyOf: func(key string) bool {
				dstAddr, ifName, isRouteKey := linux_l3.ParseStaticLinkLocalRouteKey(key)
				if isRouteKey && ifName == route.OutgoingInterface && dstAddr.Contains(gwAddr) {
					// GW address is neighbour as told by another link-local route
					return true
				}
				ifName, addr, isAddrKey := ifmodel.ParseInterfaceAddressKey(key)
				if isAddrKey && ifName == route.OutgoingInterface && addr.Contains(gwAddr) {
					// GW address is inside the local network of the outgoing interface
					// as given by the assigned IP address
					return true
				}
				return false
			},
		})
	}
	return dependencies
}

// DerivedValues derives empty value under StaticLinkLocalRouteKey if route is link-local.
// It is used in dependencies for network reachability of a route gateway (see above).
func (d *RouteDescriptor) DerivedValues(key string, route *linux_l3.Route) (derValues []kvs.KeyValuePair) {
	if route.Scope == linux_l3.Route_LINK {
		derValues = append(derValues, kvs.KeyValuePair{
			Key:   linux_l3.StaticLinkLocalRouteKey(route.DstNetwork, route.OutgoingInterface),
			Value: &prototypes.Empty{},
		})
	}
	return derValues
}

// routeDump is used as the return value sent via channel by dumpRoutes().
type routeDump struct {
	routes []adapter.RouteKVWithMetadata
	err    error
}

// Dump returns all routes associated with interfaces managed by this agent.
func (d *RouteDescriptor) Dump(correlate []adapter.RouteKVWithMetadata) ([]adapter.RouteKVWithMetadata, error) {
	var dump []adapter.RouteKVWithMetadata
	interfaces := d.ifPlugin.GetInterfaceIndex().ListAllInterfaces()
	goRoutinesCnt := len(interfaces) / minWorkForGoRoutine
	if goRoutinesCnt == 0 {
		goRoutinesCnt = 1
	}
	if goRoutinesCnt > d.dumpGoRoutinesCnt {
		goRoutinesCnt = d.dumpGoRoutinesCnt
	}
	dumpCh := make(chan routeDump, goRoutinesCnt)

	// invoke multiple go routines for more efficient parallel dumping
	for idx := 0; idx < goRoutinesCnt; idx++ {
		if goRoutinesCnt > 1 {
			go d.dumpRoutes(interfaces, idx, goRoutinesCnt, dumpCh)
		} else {
			d.dumpRoutes(interfaces, idx, goRoutinesCnt, dumpCh)
		}
	}

	// collect results from the go routines
	for idx := 0; idx < goRoutinesCnt; idx++ {
		routeDump := <-dumpCh
		if routeDump.err != nil {
			return dump, routeDump.err
		}
		dump = append(dump, routeDump.routes...)
	}

	return dump, nil
}

// dumpRoutes is run by a separate go routine to dump all routes entries associated
// with every <goRoutineIdx>-th interface.
func (d *RouteDescriptor) dumpRoutes(interfaces []string, goRoutineIdx, goRoutinesCnt int, dumpCh chan<- routeDump) {
	var dump routeDump
	ifMetaIdx := d.ifPlugin.GetInterfaceIndex()
	nsCtx := nslinuxcalls.NewNamespaceMgmtCtx()

	for i := goRoutineIdx; i < len(interfaces); i += goRoutinesCnt {
		ifName := interfaces[i]
		// get interface metadata
		ifMeta, found := ifMetaIdx.LookupByName(ifName)
		if !found || ifMeta == nil {
			dump.err = errors.Errorf("failed to obtain metadata for interface %s", ifName)
			d.log.Error(dump.err)
			break
		}

		// switch to the namespace of the interface
		revertNs, err := d.nsPlugin.SwitchToNamespace(nsCtx, ifMeta.Namespace)
		if err != nil {
			// namespace and all the routes it had contained no longer exist
			d.log.WithFields(logging.Fields{
				"err":       err,
				"namespace": ifMeta.Namespace,
			}).Warn("Failed to dump namespace")
			continue
		}

		// get routes assigned to this interface
		v4Routes, v6Routes, err := d.l3Handler.GetRoutes(ifMeta.LinuxIfIndex)
		revertNs()
		if err != nil {
			dump.err = err
			d.log.Error(dump.err)
			break
		}

		// convert each route from Netlink representation to the NB representation
		for idx, route := range append(v4Routes, v6Routes...) {
			var dstNet, gwAddr string
			if route.Dst == nil {
				if idx < len(v4Routes) {
					dstNet = ipv4AddrAny + "/0"
				} else {
					dstNet = ipv6AddrAny + "/0"
				}
			} else {
				if route.Dst.IP.To4() == nil && route.Dst.IP.IsLinkLocalUnicast() {
					// skip link-local IPv6 destinations until there is a requirement to support them
					continue
				}
				dstNet = route.Dst.String()
			}
			if len(route.Gw) != 0 {
				gwAddr = route.Gw.String()
			}
			scope, err := rtScopeFromNetlinkToNB(route.Scope)
			if err != nil {
				// route not configured by the agent
				continue
			}
			dump.routes = append(dump.routes, adapter.RouteKVWithMetadata{
				Key: linux_l3.RouteKey(dstNet, ifName),
				Value: &linux_l3.Route{
					OutgoingInterface: ifName,
					Scope:             scope,
					DstNetwork:        dstNet,
					GwAddr:            gwAddr,
					Metric:            uint32(route.Priority),
				},
				Origin: kvs.UnknownOrigin, // let the scheduler to determine the origin
			})
		}
	}

	dumpCh <- dump
}

// rtScopeFromNBToNetlink convert Route scope from NB configuration
// to the corresponding Netlink constant.
func rtScopeFromNBToNetlink(scope linux_l3.Route_Scope) (netlink.Scope, error) {
	switch scope {
	case linux_l3.Route_GLOBAL:
		return netlink.SCOPE_UNIVERSE, nil
	case linux_l3.Route_HOST:
		return netlink.SCOPE_HOST, nil
	case linux_l3.Route_LINK:
		return netlink.SCOPE_LINK, nil
	case linux_l3.Route_SITE:
		return netlink.SCOPE_SITE, nil
	}
	return 0, ErrRouteWithUndefinedScope
}

// rtScopeFromNetlinkToNB converts Route scope from Netlink constant
// to the corresponding NB constant.
func rtScopeFromNetlinkToNB(scope netlink.Scope) (linux_l3.Route_Scope, error) {
	switch scope {
	case netlink.SCOPE_UNIVERSE:
		return linux_l3.Route_GLOBAL, nil
	case netlink.SCOPE_HOST:
		return linux_l3.Route_HOST, nil
	case netlink.SCOPE_LINK:
		return linux_l3.Route_LINK, nil
	case netlink.SCOPE_SITE:
		return linux_l3.Route_SITE, nil
	}
	return 0, ErrRouteWithUndefinedScope
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

// getGwAddr returns the GW address chosen in the given route, handling the cases
// when it is left undefined.
func getGwAddr(route *linux_l3.Route) string {
	if route.GwAddr == "" {
		if ipv6, _ := addrs.IsIPv6(route.DstNetwork); ipv6 {
			return ipv6AddrAny
		}
		return ipv4AddrAny
	}
	return route.GwAddr
}
