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

package l3plugin

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linux/l3plugin/l3idx"
	"github.com/ligato/vpp-agent/plugins/linux/l3plugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
	"github.com/ligato/vpp-agent/plugins/linux/nsplugin"
	"github.com/vishvananda/netlink"
)

const (
	ipv4AddrAny = "0.0.0.0/0"
	ipv6AddrAny = "::/0"
)

// LinuxRouteConfigurator watches for any changes in the configuration of static routes as modelled by the proto file
// "model/l3/l3.proto" and stored in ETCD under the key "/vnf-agent/{vnf-agent}/linux/config/v1/route".
// Updates received from the northbound API are compared with the Linux network configuration and differences
// are applied through the Netlink AP
type LinuxRouteConfigurator struct {
	log logging.Logger

	// Mappings
	ifIndexes        ifaceidx.LinuxIfIndexRW
	rtIndexes        l3idx.LinuxRouteIndexRW                // Index mapping for ETCD route configuration
	rtAutoIndexes    l3idx.LinuxRouteIndexRW                // Index mapping for automatic interface routes (sometimes needed to evaluate network accessibility)
	rtCachedIfRoutes l3idx.LinuxRouteIndexRW                // Cache for routes requiring interface which is missing
	rtCachedGwRoutes map[string]*l3.LinuxStaticRoutes_Route // Cache for gateway routes which cannot be created at the time due to unreachable network
	rtIdxSeq         uint32

	// Linux namespace/calls handler
	l3Handler linuxcalls.NetlinkAPI
	nsHandler nsplugin.NamespaceAPI
}

// Init initializes static route configurator and starts goroutines
func (c *LinuxRouteConfigurator) Init(logger logging.PluginLogger, l3Handler linuxcalls.NetlinkAPI, nsHandler nsplugin.NamespaceAPI,
	rtIndexes l3idx.LinuxRouteIndexRW, ifIndexes ifaceidx.LinuxIfIndexRW) error {
	// Logger
	c.log = logger.NewLogger("route-conf")

	// Mappings
	c.ifIndexes = ifIndexes
	c.rtIndexes = rtIndexes
	c.rtAutoIndexes = l3idx.NewLinuxRouteIndex(nametoidx.NewNameToIdx(c.log, "linux_auto_route_indexes", nil))
	c.rtCachedIfRoutes = l3idx.NewLinuxRouteIndex(nametoidx.NewNameToIdx(c.log, "linux_cached_route_indexes", nil))
	c.rtCachedGwRoutes = make(map[string]*l3.LinuxStaticRoutes_Route)

	// L3 and namespace handler
	c.l3Handler = l3Handler
	c.nsHandler = nsHandler

	c.log.Debug("Linux Route configurator initialized")

	return nil
}

// Close does nothing for route configurator
func (c *LinuxRouteConfigurator) Close() error {
	return nil
}

// GetRouteIndexes returns route in-memory indexes
func (c *LinuxRouteConfigurator) GetRouteIndexes() l3idx.LinuxRouteIndexRW {
	return c.rtIndexes
}

// GetAutoRouteIndexes returns automatic route in-memory indexes
func (c *LinuxRouteConfigurator) GetAutoRouteIndexes() l3idx.LinuxRouteIndexRW {
	return c.rtAutoIndexes
}

// GetCachedRoutes returns cached route in-memory indexes
func (c *LinuxRouteConfigurator) GetCachedRoutes() l3idx.LinuxRouteIndexRW {
	return c.rtCachedIfRoutes
}

// GetCachedGatewayRoutes returns in-memory indexes of unreachable gateway routes
func (c *LinuxRouteConfigurator) GetCachedGatewayRoutes() map[string]*l3.LinuxStaticRoutes_Route {
	return c.rtCachedGwRoutes
}

// ConfigureLinuxStaticRoute reacts to a new northbound Linux static route config by creating and configuring
// the route in the host network stack through Netlink API.
func (c *LinuxRouteConfigurator) ConfigureLinuxStaticRoute(route *l3.LinuxStaticRoutes_Route) error {
	// Prepare route object
	netLinkRoute := &netlink.Route{}

	if route.Interface != "" {
		// Find interface
		_, ifData, foundIface := c.ifIndexes.LookupIdx(route.Interface)
		if !foundIface || ifData == nil {
			c.rtCachedIfRoutes.RegisterName(route.Name, c.rtIdxSeq, route)
			c.rtIdxSeq++
			c.log.Debugf("Static route %s requires non-existing interface %s, moved to cache", route.Name, route.Interface)
			return nil
		}
		netLinkRoute.LinkIndex = int(ifData.Index)
	}

	// Check gateway reachability
	if route.Default || route.GwAddr != "" {
		if !c.networkReachable(route.Namespace, route.GwAddr) {
			c.rtCachedGwRoutes[route.Name] = route
			c.log.Debugf("Default/Gateway route %s cached, gateway address %s is currently unreachable",
				route.Name, route.GwAddr)
			return nil
		}
	}

	// Check if route was not cached before, eventually remove it
	_, ok := c.rtCachedGwRoutes[route.Name]
	if ok {
		delete(c.rtCachedGwRoutes, route.Name)
		c.log.Debugf("route %s previously cached as unreachable was removed from cache", route.Name)
	}

	// Default route
	if route.Default {
		if err := c.createDefaultRoute(netLinkRoute, route); err != nil {
			return err
		}
	} else {
		// Static route
		if err := c.createStaticRoute(netLinkRoute, route); err != nil {
			return err
		}
	}

	// Prepare and switch to namespace where the route belongs
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
	routeNs := c.nsHandler.RouteNsToGeneric(route.Namespace)
	revertNs, err := c.nsHandler.SwitchNamespace(routeNs, nsMgmtCtx)
	if err != nil {
		return errors.Errorf("failed to switch namespace for route %s: %v", route.Name, err)
	}
	defer revertNs()

	err = c.l3Handler.AddStaticRoute(route.Name, netLinkRoute)
	if err != nil {
		return errors.Errorf("failed to add static route %s: %v", route.Name, err)
	}

	c.rtIndexes.RegisterName(RouteIdentifier(netLinkRoute), c.rtIdxSeq, route)
	c.rtIdxSeq++
	c.log.Debugf("Route %s registered", route.Name)

	c.log.Infof("Linux static route %s configured", route.Name)

	// Retry default routes if some of them is not configurable now
	if !route.Default {
		if err := c.retryDefaultRoutes(route); err != nil {
			return errors.Errorf("failed to retry default routes (after configuration of %s): %v",
				route.Name, err)
		}
	}

	return nil
}

// ModifyLinuxStaticRoute applies changes in the NB configuration of a Linux static route into the host network stack
// through Netlink API.
func (c *LinuxRouteConfigurator) ModifyLinuxStaticRoute(newRoute *l3.LinuxStaticRoutes_Route, oldRoute *l3.LinuxStaticRoutes_Route) error {
	var err error
	// Prepare route object
	netLinkRoute := &netlink.Route{}

	if newRoute.Interface != "" {
		// Find interface
		_, ifData, foundIface := c.ifIndexes.LookupIdx(newRoute.Interface)
		if !foundIface || ifData == nil {
			c.rtCachedIfRoutes.RegisterName(newRoute.Name, c.rtIdxSeq, newRoute)
			c.rtIdxSeq++
			c.log.Debugf("Modified static route %s requires non-existing interface %s, moving to cache",
				newRoute.Name, newRoute.Interface)
			return nil
		}
		netLinkRoute.LinkIndex = int(ifData.Index)
	}

	// Check gateway reachability
	if newRoute.Default || newRoute.GwAddr != "" {
		if !c.networkReachable(newRoute.Namespace, newRoute.GwAddr) {
			c.rtCachedGwRoutes[newRoute.Name] = newRoute
			c.log.Debugf("Default/Gateway route %s cached, gateway address %s is currently unreachable",
				newRoute.Name, newRoute.GwAddr)
			return nil
		}
	}

	// Check if route was not cached before, eventually remove it
	_, ok := c.rtCachedGwRoutes[newRoute.Name]
	if ok {
		delete(c.rtCachedGwRoutes, newRoute.Name)
		c.log.Debugf("route %s previously cached as unreachable was removed from cache", newRoute.Name)
	}

	// If the namespace of the new route was changed, the old route needs to be removed and the new one created in the
	// new namespace
	// If interface or destination IP address was changed, the old entry needs to be removed and recreated as well.
	// Otherwise, ModifyRouteEntry (analogy to 'ip route replace') would create a new route instead of modifying
	// the existing one
	var replace bool

	oldRouteNs := c.nsHandler.RouteNsToGeneric(oldRoute.Namespace)
	newRouteNs := c.nsHandler.RouteNsToGeneric(newRoute.Namespace)
	result := oldRouteNs.CompareNamespaces(newRouteNs)
	if result != 0 || oldRoute.Interface != newRoute.Interface {
		replace = true
	}

	// Default route
	if newRoute.Default {
		if !oldRoute.Default {
			// In this case old route has to be removed
			replace = true
		}
		err := c.createDefaultRoute(netLinkRoute, newRoute)
		if err != nil {
			return err
		}
	} else {
		if oldRoute.DstIpAddr != newRoute.Interface {
			replace = true
		}
		if err = c.createStaticRoute(netLinkRoute, newRoute); err != nil {
			return err
		}
	}

	// Static route will be removed and created anew
	if replace {
		return c.recreateLinuxStaticRoute(netLinkRoute, newRoute)
	}

	// Prepare namespace of related interface
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
	routeNs := c.nsHandler.RouteNsToGeneric(newRoute.Namespace)

	// route has to be created in the same namespace as the interface
	revertNs, err := c.nsHandler.SwitchNamespace(routeNs, nsMgmtCtx)
	if err != nil {
		return errors.Errorf("failed to switch namspace while modifying route %s: %v", newRoute.Name, err)
	}
	defer revertNs()

	// Remove old route and create a new one
	if err = c.DeleteLinuxStaticRoute(oldRoute); err != nil {
		return errors.Errorf("modify linux route: failed to remove obsolete route %s: %v", oldRoute.Name, err)
	}
	if err = c.l3Handler.AddStaticRoute(newRoute.Name, netLinkRoute); err != nil {
		return errors.Errorf("modify linux route: failed to add new route %s: %v", newRoute.Name, err)
	}

	c.log.Infof("Linux static route %s modified", newRoute.Name)

	// Retry default routes if some of them is not configurable
	if !newRoute.Default {
		if err := c.retryDefaultRoutes(newRoute); err != nil {
			return errors.Errorf("failed to retry default routes (after modification of %s): %v",
				newRoute.Name, err)
		}
	}

	return nil
}

// DeleteLinuxStaticRoute reacts to a removed NB configuration of a Linux static route entry.
func (c *LinuxRouteConfigurator) DeleteLinuxStaticRoute(route *l3.LinuxStaticRoutes_Route) error {
	var err error
	// Check if route is in cache waiting on interface
	if _, _, found := c.rtCachedIfRoutes.LookupIdx(route.Name); found {
		c.rtCachedIfRoutes.UnregisterName(route.Name)
		c.log.Debugf("Route %s removed from interface cache", route.Name)
		return nil
	}
	// Check if route is in cache waiting for gateway address reachability
	for _, cachedRoute := range c.rtCachedGwRoutes {
		if cachedRoute.Name == route.Name {
			delete(c.rtCachedGwRoutes, cachedRoute.Name)
			c.log.Debugf("Route %s removed from gw cache", route.Name)
			return nil
		}
	}

	// Prepare route object
	netLinkRoute := &netlink.Route{}

	if route.Interface != "" {
		// Find interface
		_, ifData, foundIface := c.ifIndexes.LookupIdx(route.Interface)
		if !foundIface || ifData == nil {
			return errors.Errorf("cannot delete static route %s, interface %s not found", route.Name, route.Interface)
		}
		netLinkRoute.LinkIndex = int(ifData.Index)
	}

	// Destination IP address
	if route.DstIpAddr != "" {
		addressWithPrefix := strings.Split(route.DstIpAddr, "/")
		if len(addressWithPrefix) > 1 {
			dstIPAddr := &net.IPNet{}
			_, dstIPAddr, err = net.ParseCIDR(route.DstIpAddr)
			if err != nil {
				c.log.Error(err)
				return err
			}
			netLinkRoute.Dst = dstIPAddr
		} else {
			// Do not return error
			c.log.Error("static route's dst address mask not set, route %s may not be removable", route.Name)
		}
	}
	// Gateway IP address
	if route.GwAddr != "" {
		gateway := net.ParseIP(route.GwAddr)
		if gateway != nil {
			netLinkRoute.Gw = gateway
		} else {
			// Do not return error
			c.log.Error("static route's gateway address %s has incorrect format, route %s may not be removable",
				route.GwAddr, route.Name)
		}
	}
	if netLinkRoute.Dst == nil && netLinkRoute.Gw == nil {
		return errors.Errorf("cannot delete static route %s, required at least destination or gateway address", route.Name)
	}

	// Scope
	if route.Scope != nil {
		netLinkRoute.Scope = c.parseRouteScope(route.Scope)
	}

	// Prepare and switch to the namespace where the route belongs
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
	routeNs := c.nsHandler.RouteNsToGeneric(route.Namespace)
	revertNs, err := c.nsHandler.SwitchNamespace(routeNs, nsMgmtCtx)
	if err != nil {
		return errors.Errorf("failed to switch namespace while removing route %s: %v", route.Name, err)
	}
	defer revertNs()

	err = c.l3Handler.DelStaticRoute(route.Name, netLinkRoute)
	if err != nil {
		return errors.Errorf("failed to remove linux static route %s: %v", route.Name, err)
	}

	_, _, found := c.rtIndexes.UnregisterName(RouteIdentifier(netLinkRoute))
	if found {
		c.log.Debugf("Route %s unregistered", route.Name)
	}

	c.log.Infof("Linux static route %s removed", route.Name)

	return nil
}

// ResolveCreatedInterface manages static routes for new interface. Linux interface also creates its own route which
// can make other routes accessible and ready to create - the case is also resolved here.
func (c *LinuxRouteConfigurator) ResolveCreatedInterface(ifName string, ifIdx uint32) error {
	// Search mapping for cached routes using the new interface
	cachedIfRoutes := c.rtCachedIfRoutes.LookupNamesByInterface(ifName)
	if len(cachedIfRoutes) > 0 {
		// Store default routes, they have to be configured as the last ones
		var defRoutes []*l3.LinuxStaticRoutes_Route
		// Static routes
		for _, cachedRoute := range cachedIfRoutes {
			if cachedRoute.Default {
				defRoutes = append(defRoutes, cachedRoute)
				continue
			}
			if err := c.ConfigureLinuxStaticRoute(cachedRoute); err != nil {
				return errors.Errorf("failed to configure cached route %s with registered interface %s: %v",
					cachedRoute.Name, ifName, err)
			}
			// Remove from cache
			c.rtCachedIfRoutes.UnregisterName(cachedRoute.Name)
			c.log.Debugf("cached linux route %s unregistered", cachedRoute.Name)
		}
		// Default routes
		for _, cachedDefaultRoute := range defRoutes {
			if err := c.ConfigureLinuxStaticRoute(cachedDefaultRoute); err != nil {
				return errors.Errorf("failed to configure cached default route %s with registered interface %s: %v",
					cachedDefaultRoute.Name, ifName, err)
			}
			// Remove from cache
			c.rtCachedIfRoutes.UnregisterName(cachedDefaultRoute.Name)
			c.log.Debugf("cached default linux route %s unregistered", cachedDefaultRoute.Name)
		}
	}

	// Interface also created its own route, so try to re-configure default routes
	err := c.processAutoRoutes(ifName, ifIdx)
	if err != nil {
		return err
	}

	// Try to reconfigure cached gateway routes
	if len(c.rtCachedGwRoutes) > 0 {
		// Store default routes, they have to be configured as the last ones
		defRoutes := make(map[string]*l3.LinuxStaticRoutes_Route)
		for _, cachedRoute := range c.rtCachedGwRoutes {
			// Check accessibility
			if !c.networkReachable(cachedRoute.Namespace, cachedRoute.GwAddr) {
				continue
			} else {
			}
			if cachedRoute.Default {
				defRoutes[cachedRoute.Name] = cachedRoute
				continue
			}
			if err := c.ConfigureLinuxStaticRoute(cachedRoute); err != nil {
				return errors.Errorf("failed to configure cached gateway route %s with registered interface %s: %v",
					cachedRoute.Name, ifName, err)
			}
			// Remove from cache
			delete(c.rtCachedGwRoutes, cachedRoute.Name)
			c.log.Debugf("cached gateway route %s unregistered", cachedRoute.Name)
		}
		// Default routes
		for _, cachedDefaultRoute := range defRoutes {
			if err := c.ConfigureLinuxStaticRoute(cachedDefaultRoute); err != nil {
				return errors.Errorf("failed to configure cached default gateway route %s with registered interface %s: %v",
					cachedDefaultRoute.Name, ifName, err)
			}
			// Remove from cache
			delete(c.rtCachedGwRoutes, cachedDefaultRoute.Name)
			c.log.Debugf("cached default gateway route %s unregistered", cachedDefaultRoute.Name)
		}
	}

	return err
}

// ResolveDeletedInterface manages static routes for removed interface
func (c *LinuxRouteConfigurator) ResolveDeletedInterface(ifName string, ifIdx uint32) error {
	// Search mapping for configured linux routes using the new interface
	confRoutes := c.rtIndexes.LookupNamesByInterface(ifName)
	if len(confRoutes) > 0 {
		for _, rt := range confRoutes {
			// Add to un-configured. If the interface will be recreated, all routes are configured back
			c.rtCachedIfRoutes.RegisterName(rt.Name, c.rtIdxSeq, rt)
			c.rtIdxSeq++
			c.log.Debugf("route %s registered to cache since the interface %s was unregistered", rt.Name, ifName)
		}
	}

	return nil
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *LinuxRouteConfigurator) LogError(err error) error {
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

// RouteIdentifier generates unique route ID used in mapping
func RouteIdentifier(route *netlink.Route) string {
	if route.Dst == nil || route.Dst.String() == ipv4AddrAny || route.Dst.String() == ipv6AddrAny {
		return fmt.Sprintf("default-iface%d-table%v-%s", route.LinkIndex, route.Table, route.Gw.To4().String())
	}
	return fmt.Sprintf("dst%s-iface%d-table%v-%s", route.Dst.IP.String(), route.LinkIndex, route.Table, route.Gw.String())
}

// Create default route object with gateway address. Destination address has to be set in such a case
func (c *LinuxRouteConfigurator) createDefaultRoute(netLinkRoute *netlink.Route, route *l3.LinuxStaticRoutes_Route) (err error) {
	// Gateway
	gateway := net.ParseIP(route.GwAddr)
	if gateway == nil {
		return errors.Errorf("unable to create route %s as default, gateway is nil", route.Name)
	}
	netLinkRoute.Gw = gateway

	// Destination address
	dstIPAddr := route.DstIpAddr
	if dstIPAddr == "" {
		dstIPAddr = ipv4AddrAny
	}
	if dstIPAddr != ipv4AddrAny && dstIPAddr != ipv6AddrAny {
		c.log.Warnf("route marked as default has dst address set to %s. The address will be ignored", dstIPAddr)
		dstIPAddr = ipv4AddrAny
	}
	_, netLinkRoute.Dst, err = net.ParseCIDR(dstIPAddr)
	if err != nil {
		return errors.Errorf("failed to parse destination address %s for route %s: %v", dstIPAddr, route.Name, err)
	}

	// Priority
	if route.Metric != 0 {
		netLinkRoute.Priority = int(route.Metric)
	}

	return nil
}

// Create static route from provided data
func (c *LinuxRouteConfigurator) createStaticRoute(netLinkRoute *netlink.Route, route *l3.LinuxStaticRoutes_Route) error {
	var err error
	// Destination IP address
	if route.DstIpAddr != "" {
		addressWithPrefix := strings.Split(route.DstIpAddr, "/")
		dstIPAddr := &net.IPNet{}
		if len(addressWithPrefix) > 1 {
			_, dstIPAddr, err = net.ParseCIDR(route.DstIpAddr)
			if err != nil {
				return errors.Errorf("failed to parse destination address %s for route %s: %v",
					route.DstIpAddr, route.Name, err)
			}
		} else {
			return errors.Errorf("cannot create static route %s, dst address net mask not set", route.Name)
		}
		c.log.Debugf("IP address %s set as dst for route %s", route.DstIpAddr, route.Name)
		netLinkRoute.Dst = dstIPAddr
	} else {
		return errors.Errorf("cannot create static route %s, destination address not set", route.Name)
	}

	// Set gateway if exists
	gateway := net.ParseIP(route.GwAddr)
	if gateway != nil {
		netLinkRoute.Gw = gateway
	}

	// Source IP address is exists
	srcIPAddr := net.ParseIP(route.SrcIpAddr)
	if srcIPAddr != nil {
		netLinkRoute.Src = srcIPAddr
	}

	// Scope
	if route.Scope != nil {
		netLinkRoute.Scope = c.parseRouteScope(route.Scope)
	}

	// Priority
	if route.Metric != 0 {
		netLinkRoute.Priority = int(route.Metric)
	}

	// Table
	netLinkRoute.Table = int(route.Table)

	return nil
}

// Update linux static route using modify (analogy to 'ip route replace')
func (c *LinuxRouteConfigurator) recreateLinuxStaticRoute(netLinkRoute *netlink.Route, route *l3.LinuxStaticRoutes_Route) error {
	c.log.Debugf("Route %s modification caused the route to be removed and crated again", route.Name)
	// Prepare namespace of related interface
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
	routeNs := c.nsHandler.RouteNsToGeneric(route.Namespace)

	// route has to be created in the same namespace as the interface
	revertNs, err := c.nsHandler.SwitchNamespace(routeNs, nsMgmtCtx)
	if err != nil {
		return errors.Errorf("failed to switch namespace while configuring route %s: %v", route.Name, err)
	}
	defer revertNs()

	// Update existing route
	if err := c.l3Handler.ReplaceStaticRoute(route.Name, netLinkRoute); err != nil {
		return errors.Errorf("failed to replace static rotue %s: %v", route.Name, err)
	}

	return nil
}

// Tries to configure again cached default/gateway routes (as a reaction to the new route)
func (c *LinuxRouteConfigurator) retryDefaultRoutes(route *l3.LinuxStaticRoutes_Route) error {
	for _, defRoute := range c.rtCachedGwRoutes {
		// Filter routes from different namespaces
		if defRoute.Namespace != nil && route.Namespace == nil || defRoute.Namespace == nil && route.Namespace != nil {
			continue
		}
		if defRoute.Namespace != nil && route.Namespace != nil && defRoute.Namespace.Name != route.Namespace.Name {
			continue
		}

		// Parse gateway and default address
		gwIPParsed := net.ParseIP(defRoute.GwAddr)
		_, dstNet, err := net.ParseCIDR(route.DstIpAddr)
		if err != nil {
			return errors.Errorf("failed to parse destination address %s of cached default route %s: %v",
				route.DstIpAddr, route.Name, err)
		}

		if dstNet.Contains(gwIPParsed) {
			// Default/Gateway route can be now configured
			if err := c.ConfigureLinuxStaticRoute(defRoute); err != nil {
				return errors.Errorf("failed to configure cached default route %s: %v", defRoute.Name, err)
			}
			delete(c.rtCachedGwRoutes, defRoute.Name)
			c.log.Debugf("default route %s removed from cache", defRoute.Name)
		}
	}

	return nil
}

// Handles automatic route created by adding interface. Method look for routes related to the interface and its
// IP address in its namespace.
// Note: read route's destination address does not contain mask. This value is determined from interfaces' IP address.
// Automatic routes are store in separate mapping and their names are generated.
func (c *LinuxRouteConfigurator) processAutoRoutes(ifName string, ifIdx uint32) error {
	// Look for metadata
	_, ifData, found := c.ifIndexes.LookupIdx(ifName)
	if !found {
		return errors.Errorf("interface %s not found in the mapping", ifName)
	}
	if ifData == nil || ifData.Data == nil {
		return errors.Errorf("interface %s data not found in the mapping", ifName)
	}

	// Move to interface with the interface
	if ifData.Data.Namespace != nil {
		nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
		// Switch to namespace
		ifNs := c.nsHandler.IfNsToGeneric(ifData.Data.Namespace)
		revertNs, err := c.nsHandler.SwitchNamespace(ifNs, nsMgmtCtx)
		if err != nil {
			return errors.Errorf("failed to switch to namespace: %v", err)
		}
		defer revertNs()
	}

	// Get interface
	link, err := netlink.LinkByName(ifData.Data.HostIfName)
	if err != nil {
		return errors.Errorf("cannot read linux interface %s (host %s): %v", ifName, ifData.Data.HostIfName, err)
	}

	// Read all routes belonging to the interface
	linuxRts, err := netlink.RouteList(link, netlink.FAMILY_ALL)
	if err != nil {
		return errors.Errorf("cannot read linux routes for interface %s (host %s): %v", ifName, ifData.Data.HostIfName, err)
	}

	// Iterate over link addresses and look for ones related to t
	for rtIdx, linuxRt := range linuxRts {
		if linuxRt.Dst == nil {
			continue
		}
		route := c.transformRoute(linuxRt, ifData.Data.HostIfName)
		// Route's destination address is read without mask. Use interface data to fill it.
		var routeFound bool
		for ipIdx, ifIP := range ifData.Data.IpAddresses {
			_, ifDst, err := net.ParseCIDR(ifIP)
			if err != nil {
				return errors.Errorf("failed to parse IP address %s: %v", ifIP, err)
			}
			if bytes.Compare(linuxRt.Dst.IP, ifDst.IP) == 0 {
				// Transform destination IP and namespace
				route.DstIpAddr = ifData.Data.IpAddresses[ipIdx]
				route.Namespace = transformNamespace(ifData.Data.Namespace)
				routeFound = true
			}
		}
		if !routeFound {
			c.log.Debugf("Route with IP %s skipped", linuxRt.Dst.IP.String())
			continue
		}
		// Generate name
		route.Name = ifName + strconv.Itoa(rtIdx)
		// In case there is obsolete route with the same name, remove it
		// TODO use update metadata (needs to be implemented in custom mapping)
		c.rtAutoIndexes.UnregisterName(route.Name)
		c.rtAutoIndexes.RegisterName(route.Name, c.rtIdxSeq, route)
		c.rtIdxSeq++

		// Also try to configure default routes
		if err := c.retryDefaultRoutes(route); err != nil {
			return errors.Errorf("auto route processing: error retrying default routes: %v", err)
		}
	}

	return nil
}

// Transform linux netlink route type to proto message type
func (c *LinuxRouteConfigurator) transformRoute(linuxRt netlink.Route, ifName string) *l3.LinuxStaticRoutes_Route {
	var dstAddr, srcAddr, gwAddr string
	// Destination address
	if linuxRt.Dst != nil {
		// Transform only IP (without mask)
		dstAddr = linuxRt.Dst.IP.String()
	}
	// Source address
	if linuxRt.Src != nil {
		srcAddr = linuxRt.Src.String()
	}
	// Gateway address
	if linuxRt.Gw != nil {
		gwAddr = linuxRt.Gw.String()
	}

	if dstAddr == "" || dstAddr == ipv4AddrAny || dstAddr == ipv6AddrAny {
		// Default route
		return &l3.LinuxStaticRoutes_Route{
			Default:   true,
			Interface: ifName,
			GwAddr:    gwAddr,
			Metric:    uint32(linuxRt.Priority),
		}
	}
	// Static route
	return &l3.LinuxStaticRoutes_Route{
		Interface: ifName,
		DstIpAddr: dstAddr,
		SrcIpAddr: srcAddr,
		GwAddr:    gwAddr,
		Scope:     c.parseLinuxRouteScope(linuxRt.Scope),
		Metric:    uint32(linuxRt.Priority),
		Table:     uint32(linuxRt.Table),
	}
}

// Interface namespace type -> route namespace type
func transformNamespace(ifNs *interfaces.LinuxInterfaces_Interface_Namespace) *l3.LinuxStaticRoutes_Route_Namespace {
	if ifNs == nil {
		return nil
	}
	return &l3.LinuxStaticRoutes_Route_Namespace{
		Type: func(ifType interfaces.LinuxInterfaces_Interface_Namespace_NamespaceType) l3.LinuxStaticRoutes_Route_Namespace_NamespaceType {
			switch ifType {
			case interfaces.LinuxInterfaces_Interface_Namespace_PID_REF_NS:
				return l3.LinuxStaticRoutes_Route_Namespace_PID_REF_NS
			case interfaces.LinuxInterfaces_Interface_Namespace_MICROSERVICE_REF_NS:
				return l3.LinuxStaticRoutes_Route_Namespace_MICROSERVICE_REF_NS
			case interfaces.LinuxInterfaces_Interface_Namespace_NAMED_NS:
				return l3.LinuxStaticRoutes_Route_Namespace_NAMED_NS
			case interfaces.LinuxInterfaces_Interface_Namespace_FILE_REF_NS:
				return l3.LinuxStaticRoutes_Route_Namespace_FILE_REF_NS
			default:
				return l3.LinuxStaticRoutes_Route_Namespace_PID_REF_NS
			}
		}(ifNs.Type),
		Pid:          ifNs.Pid,
		Microservice: ifNs.Microservice,
		Name:         ifNs.Name,
		Filepath:     ifNs.Filepath,
	}
}

// Agent route scope -> netlink route scope
func (c *LinuxRouteConfigurator) parseRouteScope(scope *l3.LinuxStaticRoutes_Route_Scope) netlink.Scope {
	switch scope.Type {
	case l3.LinuxStaticRoutes_Route_Scope_GLOBAL:
		return netlink.SCOPE_UNIVERSE
	case l3.LinuxStaticRoutes_Route_Scope_HOST:
		return netlink.SCOPE_HOST
	case l3.LinuxStaticRoutes_Route_Scope_LINK:
		return netlink.SCOPE_LINK
	case l3.LinuxStaticRoutes_Route_Scope_SITE:
		return netlink.SCOPE_SITE
	default:
		c.log.Infof("Unknown scope type, setting to default (link): %v", scope.Type)
		return netlink.SCOPE_LINK
	}
}

// Verifies whether address network is reachable.
func (c *LinuxRouteConfigurator) networkReachable(ns *l3.LinuxStaticRoutes_Route_Namespace, ipAddress string) bool {
	// Try for registered configuration routes
	registeredRoute, err := c.rtIndexes.LookupRouteByIP(ns, ipAddress)
	if err != nil {
		c.log.Errorf("Failed to resolve accessibility of %s (registered): %v", ipAddress, err)
	}
	// Try for registered automatic (interface-added) routes
	autoRoute, err := c.rtAutoIndexes.LookupRouteByIP(ns, ipAddress)
	if err != nil {
		c.log.Errorf("Failed to resolve accessibility of %s (auto): %v", ipAddress, err)
	}
	if registeredRoute != nil || autoRoute != nil {
		c.log.Debugf("Network %s is reachable", ipAddress)
		return true
	}
	return false
}
