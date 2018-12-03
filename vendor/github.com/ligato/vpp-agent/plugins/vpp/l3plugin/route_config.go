// Copyright (c) 2017 Cisco and/or its affiliates.
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

// Package l3plugin implements the L3 plugin that handles L3 FIBs.
package l3plugin

import (
	"fmt"
	"strconv"

	"strings"

	"sort"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	ifvppcalls "github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin/l3idx"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// SortedRoutes type is used to implement sort interface for slice of Route.
type SortedRoutes []*l3.StaticRoutes_Route

// Return length of slice.
// Implements sort.Interface
func (arr SortedRoutes) Len() int {
	return len(arr)
}

// Swap swaps two items in slice identified by indices.
// Implements sort.Interface
func (arr SortedRoutes) Swap(i, j int) {
	arr[i], arr[j] = arr[j], arr[i]
}

// Less returns true if the item at index i in slice
// should be sorted before the element with index j.
// Implements sort.Interface
func (arr SortedRoutes) Less(i, j int) bool {
	return lessRoute(arr[i], arr[j])
}

// RouteConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of L3 routes as modelled by the proto file "../model/l3/l3.proto" and stored
// in ETCD under the key "/vnf-agent/{vnf-agent}/vpp/config/v1routes". Updates received from the northbound API
// are compared with the VPP run-time configuration and differences are applied through the VPP binary API.
type RouteConfigurator struct {
	log logging.Logger

	// In-memory mappings
	ifIndexes       ifaceidx.SwIfIndex
	rtIndexes       l3idx.RouteIndexRW
	rtCachedIndexes l3idx.RouteIndexRW
	rtIndexSeq      uint32

	// VPP channels
	vppChan govppapi.Channel
	// VPP API handlers
	ifHandler ifvppcalls.IfVppWrite
	rtHandler vppcalls.RouteVppAPI
}

// Init members (channels...) and start go routines.
func (c *RouteConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex) (err error) {
	// Logger
	c.log = logger.NewLogger("l3-route-conf")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.rtIndexes = l3idx.NewRouteIndex(nametoidx.NewNameToIdx(c.log, "route_indexes", nil))
	c.rtCachedIndexes = l3idx.NewRouteIndex(nametoidx.NewNameToIdx(c.log, "route_cached_indexes", nil))
	c.rtIndexSeq = 1

	// VPP channel
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// VPP API handlers
	c.ifHandler = ifvppcalls.NewIfVppHandler(c.vppChan, c.log)
	c.rtHandler = vppcalls.NewRouteVppHandler(c.vppChan, c.ifIndexes, c.log)

	c.log.Debug("L3 Route configurator initialized")

	return nil
}

// GetRouteIndexes exposes rtIndexes mapping
func (c *RouteConfigurator) GetRouteIndexes() l3idx.RouteIndex {
	return c.rtIndexes
}

// GetCachedRouteIndexes exposes rtCachedIndexes mapping
func (c *RouteConfigurator) GetCachedRouteIndexes() l3idx.RouteIndex {
	return c.rtCachedIndexes
}

// Close GOVPP channel.
func (c *RouteConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose VPP route configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *RouteConfigurator) clearMapping() {
	c.rtIndexes.Clear()
	c.rtCachedIndexes.Clear()
	c.log.Debugf("VPP ARP configurator mapping cleared")
}

// Create unique identifier which serves as a name in name-to-index mapping.
func routeIdentifier(vrf uint32, destination string, nextHop string) string {
	if nextHop == "<nil>" {
		nextHop = ""
	}
	return fmt.Sprintf("vrf%v-%v-%v", vrf, destination, nextHop)
}

// ConfigureRoute processes the NB config and propagates it to bin api calls.
func (c *RouteConfigurator) ConfigureRoute(route *l3.StaticRoutes_Route, vrfFromKey string) error {
	// Validate VRF index from key and it's value in data.
	if err := c.validateVrfFromKey(route, vrfFromKey); err != nil {
		return err
	}

	routeID := routeIdentifier(route.VrfId, route.DstIpAddr, route.NextHopAddr)

	swIdx, err := resolveInterfaceSwIndex(route.OutgoingInterface, c.ifIndexes)
	if err != nil {
		c.rtCachedIndexes.RegisterName(routeID, c.rtIndexSeq, route)
		c.rtIndexSeq++
		c.log.Debugf("Route %v registered to cache", routeID)
		return nil
	}

	// Check mandatory destination address
	if route.DstIpAddr == "" {
		return errors.Errorf("route %s does not contain destination address", routeID)
	}

	// Create new route.
	err = c.rtHandler.VppAddRoute(c.ifHandler, route, swIdx)
	if err != nil {
		return errors.Errorf("failed to add VPP route %s: %v", routeID, err)
	}

	// Register configured route
	_, _, routeExists := c.rtIndexes.LookupIdx(routeID)
	if !routeExists {
		c.rtIndexes.RegisterName(routeID, c.rtIndexSeq, route)
		c.rtIndexSeq++
		c.log.Infof("Route %v registered", routeID)
	}

	c.log.Infof("Route %v -> %v configured", route.DstIpAddr, route.NextHopAddr)
	return nil
}

// ModifyRoute processes the NB config and propagates it to bin api calls.
func (c *RouteConfigurator) ModifyRoute(newConfig *l3.StaticRoutes_Route, oldConfig *l3.StaticRoutes_Route, vrfFromKey string) error {
	routeID := routeIdentifier(oldConfig.VrfId, oldConfig.DstIpAddr, oldConfig.NextHopAddr)
	if newConfig.OutgoingInterface != "" {
		_, _, existsNewOutgoing := c.ifIndexes.LookupIdx(newConfig.OutgoingInterface)
		newrouteID := routeIdentifier(newConfig.VrfId, newConfig.DstIpAddr, newConfig.NextHopAddr)
		if existsNewOutgoing {
			c.rtCachedIndexes.UnregisterName(newrouteID)
			c.log.Debugf("Route %s unregistered from cache", newrouteID)
		} else {
			if routeIdx, _, isCached := c.rtCachedIndexes.LookupIdx(routeID); isCached {
				c.rtCachedIndexes.RegisterName(newrouteID, routeIdx, newConfig)
				c.log.Debugf("Route %s registered to cache", newrouteID)
			} else {
				c.rtCachedIndexes.RegisterName(newrouteID, c.rtIndexSeq, newConfig)
				c.rtIndexSeq++
				c.log.Debugf("Route %s registered to cache", newrouteID)
			}
		}
	}

	if err := c.deleteOldRoute(oldConfig, vrfFromKey); err != nil {
		return err
	}

	if err := c.addNewRoute(newConfig, vrfFromKey); err != nil {
		return err
	}

	c.log.Infof("Route %s -> %s modified", oldConfig.DstIpAddr, oldConfig.NextHopAddr)
	return nil
}

func (c *RouteConfigurator) deleteOldRoute(route *l3.StaticRoutes_Route, vrfFromKey string) error {
	// Check if route entry is not just cached
	routeID := routeIdentifier(route.VrfId, route.DstIpAddr, route.NextHopAddr)
	_, _, found := c.rtCachedIndexes.LookupIdx(routeID)
	if found {
		c.rtCachedIndexes.UnregisterName(routeID)
		c.log.Debugf("Route entry %v found in cache, removed", routeID)
		// Cached route is not configured on the VPP, return
		return nil
	}

	swIdx, err := resolveInterfaceSwIndex(route.OutgoingInterface, c.ifIndexes)
	if err != nil {
		return err
	}

	// Validate old cachedRoute data Vrf.
	if err := c.validateVrfFromKey(route, vrfFromKey); err != nil {
		return err
	}
	// Remove and unregister old route.
	if err := c.rtHandler.VppDelRoute(route, swIdx); err != nil {
		return errors.Errorf("failed to delete VPP route %s: %v", routeID, err)
	}
	_, _, found = c.rtIndexes.UnregisterName(routeID)
	if found {
		c.log.Infof("Old route %s unregistered", routeID)
	}

	return nil
}

func (c *RouteConfigurator) addNewRoute(route *l3.StaticRoutes_Route, vrfFromKey string) error {
	// Validate new route data Vrf.
	if err := c.validateVrfFromKey(route, vrfFromKey); err != nil {
		return err
	}

	swIdx, err := resolveInterfaceSwIndex(route.OutgoingInterface, c.ifIndexes)
	if err != nil {
		return err
	}

	routeID := routeIdentifier(route.VrfId, route.DstIpAddr, route.NextHopAddr)

	// Create and register new route.
	if err = c.rtHandler.VppAddRoute(c.ifHandler, route, swIdx); err != nil {
		return errors.Errorf("failed to add VPP route %s: %v", routeID, err)
	}
	c.rtIndexes.RegisterName(routeID, c.rtIndexSeq, route)
	c.rtIndexSeq++
	c.log.Debugf("New route %v registered", routeID)

	return nil
}

// DeleteRoute processes the NB config and propagates it to bin api calls.
func (c *RouteConfigurator) DeleteRoute(route *l3.StaticRoutes_Route, vrfFromKey string) (wasError error) {
	// Validate VRF index from key and it's value in data.
	if err := c.validateVrfFromKey(route, vrfFromKey); err != nil {
		return err
	}

	// Check if route entry is not just cached
	routeID := routeIdentifier(route.VrfId, route.DstIpAddr, route.NextHopAddr)
	_, _, found := c.rtCachedIndexes.LookupIdx(routeID)
	if found {
		c.rtCachedIndexes.UnregisterName(routeID)
		c.log.Debugf("Route entry %v found in cache, removed", routeID)
		// Cached route is not configured on the VPP, return
		return nil
	}

	swIdx, err := resolveInterfaceSwIndex(route.OutgoingInterface, c.ifIndexes)
	if err != nil {
		return err
	}

	// Remove and unregister route.
	if err := c.rtHandler.VppDelRoute(route, swIdx); err != nil {
		return errors.Errorf("failed to delete VPP route %s: %v", routeID, err)
	}

	routeIdentifier := routeIdentifier(route.VrfId, route.DstIpAddr, route.NextHopAddr)
	_, _, found = c.rtIndexes.UnregisterName(routeIdentifier)
	if found {
		c.log.Infof("Route %v unregistered", routeIdentifier)
	}

	c.log.Infof("Route %v -> %v removed", route.DstIpAddr, route.NextHopAddr)
	return nil
}

// DiffRoutes calculates route diff from two sets of routes and returns routes to be added and removed
func (c *RouteConfigurator) DiffRoutes(new, old []*l3.StaticRoutes_Route) (toBeDeleted, toBeAdded []*l3.StaticRoutes_Route) {
	oldSorted, newSorted := SortedRoutes(old), SortedRoutes(new)
	sort.Sort(newSorted)
	sort.Sort(oldSorted)

	// Compare.
	i, j := 0, 0
	for i < len(newSorted) && j < len(oldSorted) {
		if proto.Equal(newSorted[i], oldSorted[j]) {
			i++
			j++
		} else {
			if lessRoute(newSorted[i], oldSorted[j]) {
				toBeAdded = append(toBeAdded, newSorted[i])
				i++
			} else {
				toBeDeleted = append(toBeDeleted, oldSorted[j])
				j++
			}
		}
	}

	for ; i < len(newSorted); i++ {
		toBeAdded = append(toBeAdded, newSorted[i])
	}

	for ; j < len(oldSorted); j++ {
		toBeDeleted = append(toBeDeleted, oldSorted[j])
	}
	return
}

// ResolveCreatedInterface is responsible for reconfiguring cached routes and then from removing them from route cache
func (c *RouteConfigurator) ResolveCreatedInterface(ifName string, swIdx uint32) error {
	routesWithIndex := c.rtCachedIndexes.LookupRouteAndIDByOutgoingIfc(ifName)
	if len(routesWithIndex) == 0 {
		return nil
	}
	for _, routeWithIndex := range routesWithIndex {
		route := routeWithIndex.Route
		vrf := strconv.FormatUint(uint64(route.VrfId), 10)
		if err := c.recreateRoute(route, vrf); err != nil {
			return errors.Errorf("Error recreating route %s with interface %s: %v",
				routeWithIndex.RouteID, ifName, err)
		}
		c.rtCachedIndexes.UnregisterName(routeWithIndex.RouteID)
		c.log.Debugf("Route %s removed from cache", routeWithIndex.RouteID)
	}

	return nil
}

// ResolveDeletedInterface is responsible for moving routes of deleted interface to cache
func (c *RouteConfigurator) ResolveDeletedInterface(ifName string, swIdx uint32) error {
	routesWithIndex := c.rtIndexes.LookupRouteAndIDByOutgoingIfc(ifName)
	if len(routesWithIndex) == 0 {
		return nil
	}
	for _, routeWithIndex := range routesWithIndex {
		route := routeWithIndex.Route
		if err := c.moveRouteToCache(route); err != nil {
			return err
		}
	}

	return nil
}

func (c *RouteConfigurator) validateVrfFromKey(config *l3.StaticRoutes_Route, vrfFromKey string) error {
	intVrfFromKey, err := strconv.Atoi(vrfFromKey)
	if intVrfFromKey != int(config.VrfId) {
		if err != nil {
			return errors.Errorf("failed to validate route VRF value from key: %v", err)
		}
		c.log.Warnf("VRF index from key (%v) and from config (%v) does not match, using value from the key",
			intVrfFromKey, config.VrfId)
		config.VrfId = uint32(intVrfFromKey)
	}
	return nil
}

/**
recreateRoute calls delete and configure route.

This is type of workaround because when outgoing interface is deleted then it isn't possible to remove
associated routes. they stay in following state:
- oper-flags:drop
- routing section: unresolved
It is neither possible to recreate interface and then create route.
It is only possible to recreate interface, delete old associated routes (like clean old mess)
and then add them again.
*/
func (c *RouteConfigurator) recreateRoute(route *l3.StaticRoutes_Route, vrf string) error {
	if err := c.DeleteRoute(route, vrf); err != nil {
		return errors.Errorf("failed to remove route which should be recreated: %v", err)
	}
	return c.ConfigureRoute(route, vrf)
}

func (c *RouteConfigurator) moveRouteToCache(config *l3.StaticRoutes_Route) (wasError error) {
	routeID := routeIdentifier(config.VrfId, config.DstIpAddr, config.NextHopAddr)
	_, _, found := c.rtIndexes.UnregisterName(routeID)
	if found {
		c.log.Infof("Route %v unregistered", routeID)
	}

	c.rtCachedIndexes.RegisterName(routeID, c.rtIndexSeq, config)
	c.rtIndexSeq++
	c.log.Debugf("Route %s registered to cache", routeID)

	return nil
}

func resolveInterfaceSwIndex(ifName string, index ifaceidx.SwIfIndex) (uint32, error) {
	ifIndex := vppcalls.NextHopOutgoingIfUnset
	if ifName != "" {
		var exists bool
		ifIndex, _, exists = index.LookupIdx(ifName)
		if !exists {
			return ifIndex, errors.Errorf("route outgoing interface %s not found", ifName)
		}
	}
	return ifIndex, nil
}

func lessRoute(a, b *l3.StaticRoutes_Route) bool {
	if a.Type != b.Type {
		return a.Type < b.Type
	}
	if a.VrfId != b.VrfId {
		return a.VrfId < b.VrfId
	}
	if !strings.EqualFold(a.DstIpAddr, b.DstIpAddr) {
		return strings.Compare(a.DstIpAddr, b.DstIpAddr) < 0
	}
	if !strings.EqualFold(a.NextHopAddr, b.NextHopAddr) {
		return strings.Compare(a.NextHopAddr, b.NextHopAddr) < 0
	}
	if a.ViaVrfId != b.ViaVrfId {
		return a.ViaVrfId < b.ViaVrfId
	}
	if a.OutgoingInterface != b.OutgoingInterface {
		return a.OutgoingInterface < b.OutgoingInterface
	}
	if a.Preference != b.Preference {
		return a.Preference < b.Preference
	}
	return a.Weight < b.Weight
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *RouteConfigurator) LogError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *errors.Error:
		c.log.WithField("logger", c.log).Errorf(string(err.Error() + "\n" + string(err.(*errors.Error).Stack())))
	default:
		c.log.Error(err)
	}
	return err
}
