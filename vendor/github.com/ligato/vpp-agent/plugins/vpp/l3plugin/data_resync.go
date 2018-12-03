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
	"fmt"

	"net"

	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// Resync configures the VPP static routes.
func (c *RouteConfigurator) Resync(nbRoutes []*l3.StaticRoutes_Route) error {
	// Re-initialize cache
	c.clearMapping()

	// Retrieve VPP route configuration
	vppRouteDetails, err := c.rtHandler.DumpStaticRoutes()
	if err != nil {
		return errors.Errorf("failed to dump VPP routes: %v", err)
	}

	// Correlate NB and VPP configuration
	for _, nbRoute := range nbRoutes {
		nbRouteID := routeIdentifier(nbRoute.VrfId, nbRoute.DstIpAddr, nbRoute.NextHopAddr)
		nbIfIdx, _, found := c.ifIndexes.LookupIdx(nbRoute.OutgoingInterface)
		if !found {
			if nbRoute.Type == l3.StaticRoutes_Route_INTER_VRF {
				// expected by inter VRF-routes
				nbIfIdx = vppcalls.NextHopOutgoingIfUnset
			} else {
				c.rtCachedIndexes.RegisterName(nbRouteID, c.rtIndexSeq, nbRoute)
				c.log.Debugf("VPP route resync: outgoing interface not found for %s, cached", nbRouteID)
				c.rtIndexSeq++
				continue
			}
		}
		// Default VPP value for weight in case it is not set
		if nbRoute.Weight == 0 {
			nbRoute.Weight = 1
		}
		// Look for the same route in the configuration
		for _, vppRouteDetail := range vppRouteDetails {
			if vppRouteDetail.Route == nil {
				continue
			}
			vppRoute := vppRouteDetail.Route
			vppRouteID := routeIdentifier(vppRoute.VrfId, vppRoute.DstIpAddr, vppRoute.NextHopAddr)
			c.log.Debugf("VPP route resync: comparing %s and %s", nbRouteID, vppRouteID)
			if int32(vppRoute.Type) != int32(nbRoute.Type) {
				c.log.Debugf("VPP route resync: route type is different (NB: %d, VPP %d)",
					nbRoute.Type, vppRoute.Type)
				continue
			}
			if vppRoute.OutgoingInterface != nbRoute.OutgoingInterface {
				c.log.Debugf("VPP route resync: interface index is different (NB: %d, VPP %d)",
					nbIfIdx, vppRoute.OutgoingInterface)
				continue
			}
			if vppRoute.DstIpAddr != nbRoute.DstIpAddr {
				c.log.Debugf("VPP route resync: dst address is different (NB: %s, VPP %s)",
					nbRoute.DstIpAddr, vppRoute.DstIpAddr)
				continue
			}
			if vppRoute.VrfId != nbRoute.VrfId {
				c.log.Debugf("VPP route resync: VRF ID is different (NB: %d, VPP %d)",
					nbRoute.VrfId, vppRoute.VrfId)
				continue
			}
			if vppRoute.Weight != nbRoute.Weight {
				c.log.Debugf("VPP route resync: weight is different (NB: %d, VPP %d)",
					nbRoute.Weight, vppRoute.Weight)
				continue
			}
			if vppRoute.Preference != nbRoute.Preference {
				c.log.Debugf("VPP route resync: preference is different (NB: %d, VPP %d)",
					nbRoute.Preference, vppRoute.Preference)
				continue
			}
			// Set zero address in correct format if not defined
			if nbRoute.NextHopAddr == "" {
				if nbRoute.NextHopAddr, err = c.fillEmptyNextHop(nbRoute.DstIpAddr); err != nil {
					return err
				}
			}
			if vppRoute.NextHopAddr != nbRoute.NextHopAddr {
				c.log.Debugf("VPP route resync routes: next hop address is different (NB: %s, VPP %s)",
					nbRoute.NextHopAddr, vppRoute.NextHopAddr)
				continue
			}
			if vppRoute.ViaVrfId != nbRoute.ViaVrfId {
				c.log.Debugf("VPP route resync: via VRF ID is different (NB: %d, VPP %d)",
					nbRoute.ViaVrfId, vppRoute.ViaVrfId)
				continue
			}
			// Register existing routes
			c.rtIndexes.RegisterName(nbRouteID, c.rtIndexSeq, nbRoute)
			c.rtIndexSeq++
			c.log.Debugf("VPP route resync: route %s registered without additional changes", nbRouteID)
			break
		}
	}

	// Add missing route configuration
	for _, nbRoute := range nbRoutes {
		routeID := routeIdentifier(nbRoute.VrfId, nbRoute.DstIpAddr, nbRoute.NextHopAddr)
		_, _, found := c.rtIndexes.LookupIdx(routeID)
		if !found {
			// create new route if does not exist yet. VRF ID is already validated at this point.
			if err := c.ConfigureRoute(nbRoute, fmt.Sprintf("%d", nbRoute.VrfId)); err != nil {
				return errors.Errorf("VPP route resync error: failed to configure route %s: %v", routeID, err)
			}
		}
	}

	// Remove other routes except DROP type
	for _, vppRoute := range vppRouteDetails {
		if routeMayBeRemoved(vppRoute) {
			route := vppRoute.Route
			routeID := routeIdentifier(route.VrfId, route.DstIpAddr, route.NextHopAddr)
			_, _, found := c.rtIndexes.LookupIdx(routeID)
			if !found {
				// Register before removal
				c.rtIndexes.RegisterName(routeID, c.rtIndexSeq, route)
				c.rtIndexSeq++
				c.log.Debugf("Route %s registered before removal", routeID)
				if err := c.DeleteRoute(route, fmt.Sprintf("%d", route.VrfId)); err != nil {
					return errors.Errorf("VPP route resync error: failed to remove route %s: %v", routeID, err)
				}
				c.log.Debugf("VPP route resync: vpp route %s removed", routeID)
			}
		}
	}

	c.log.Debugf("VPP route resync done")
	return nil
}

// Following rules are currently applied:
// - no DROP type route can be removed in order to prevent removal of VPP default routes
// - IPv6 link local route cannot be removed
func routeMayBeRemoved(route *vppcalls.RouteDetails) bool {
	if route.Route.Type == l3.StaticRoutes_Route_DROP {
		return false
	}
	if route.Meta.IsIPv6 && net.ParseIP(route.Route.DstIpAddr).IsLinkLocalUnicast() {
		return false
	}
	return true
}

// Resync confgures the empty VPP (overwrites the arp entries)
func (c *ArpConfigurator) Resync(arpEntries []*l3.ArpTable_ArpEntry) error {
	// Re-initialize cache
	c.clearMapping()

	// todo dump arp

	if len(arpEntries) > 0 {
		for _, entry := range arpEntries {
			if err := c.AddArp(entry); err != nil {
				return errors.Errorf("ARP resync error: failed to configure ARP (MAC %s): %v",
					entry.PhysAddress, err)
			}
		}
	}

	c.log.Info("VPP ARP resync done")
	return nil
}

// ResyncInterfaces confgures the empty VPP (overwrites the proxy arp entries)
func (c *ProxyArpConfigurator) ResyncInterfaces(nbProxyArpIfs []*l3.ProxyArpInterfaces_InterfaceList) error {
	// Re-initialize cache
	c.clearMapping()

	// Todo: dump proxy arp

	if len(nbProxyArpIfs) > 0 {
		for _, entry := range nbProxyArpIfs {
			if err := c.AddInterface(entry); err != nil {
				return errors.Errorf("Proxy ARP interface resync error: failed to add interfaces to %s: %v",
					entry.Label, err)
			}
		}
	}

	c.log.Info("Proxy ARP interface resync done")
	return nil
}

// ResyncRanges confgures the empty VPP (overwrites the proxy arp ranges)
func (c *ProxyArpConfigurator) ResyncRanges(nbProxyArpRanges []*l3.ProxyArpRanges_RangeList) error {
	// Todo: dump proxy arp

	if len(nbProxyArpRanges) > 0 {
		for _, entry := range nbProxyArpRanges {
			if err := c.AddRange(entry); err != nil {
				return errors.Errorf("Proxy ARP range resync error: failed to set range to %s: %v",
					entry.Label, err)
			}
		}
	}

	c.log.Info("Proxy ARP interface resync done")
	return nil
}

// Resync configures the empty VPP (adds IP scan neigh config)
func (c *IPNeighConfigurator) Resync(config *l3.IPScanNeighbor) error {
	if err := c.Set(config); err != nil {
		return errors.Errorf("failed to set IP scan neighbor: %v", err)
	}

	c.log.Info("IP scan neighbor resync done")
	return nil
}

// Takes route destination address used to derive IP version and returns zero IP without mask
func (c *RouteConfigurator) fillEmptyNextHop(dstIP string) (string, error) {
	_, isIPv6, err := addrs.ParseIPWithPrefix(dstIP)
	if err != nil {
		return "", errors.Errorf("route resync error: failed to parse IP address %s: %v", dstIP, err)
	}
	if isIPv6 {
		return net.IPv6zero.String(), nil
	}
	return net.IPv4zero.String(), nil
}
