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
	"net"

	"github.com/go-errors/errors"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
	"github.com/ligato/vpp-agent/plugins/linux/nsplugin"
	"github.com/vishvananda/netlink"
)

// Resync configures an initial set of ARPs. Existing Linux ARPs are registered and potentially re-configured.
func (c *LinuxArpConfigurator) Resync(arpEntries []*l3.LinuxStaticArpEntries_ArpEntry) error {
	// Create missing arp entries and update existing ones
	for _, entry := range arpEntries {
		err := c.ConfigureLinuxStaticArpEntry(entry)
		if err != nil {
			return errors.Errorf("linux ARP resync: failed to configure ARP %s: %v", entry.Name, err)
		}
	}

	// Dump pre-existing not managed arp entries
	err := c.LookupLinuxArpEntries()
	if err != nil {
		return errors.Errorf("linux ARP resync: failed to lookup ARP entries: %v", err)
	}

	c.log.Info("Linux ARP resync done")

	return nil
}

// Resync configures an initial set of static routes. Existing Linux static routes are registered and potentially
// re-configured. Resync does not remove any linux route.
func (c *LinuxRouteConfigurator) Resync(nbRoutes []*l3.LinuxStaticRoutes_Route) error {
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()

	// First step is to find a linux equivalent for NB route config
	for _, nbRoute := range nbRoutes {
		// Route interface exists
		if nbRoute.Interface != "" {
			_, _, found := c.ifIndexes.LookupIdx(nbRoute.Interface)
			if !found {
				// If route interface does not exist, cache it
				c.rtCachedIfRoutes.RegisterName(nbRoute.Name, c.rtIdxSeq, nbRoute)
				c.rtIdxSeq++
				c.log.Debugf("Linux route %s resync: interface %s does not exists, moved to cache",
					nbRoute.Name, nbRoute.Interface)
				continue
			}
		}

		// There can be several routes found according to matching parameters
		linuxRtList, err := c.findLinuxRoutes(nbRoute, nsMgmtCtx)
		if err != nil {
			return errors.Errorf("linux route %s resync: %v", nbRoute.Name, err)
		}
		c.log.Debugf("found %d linux routes to compare for %s", len(linuxRtList), nbRoute.Name)
		// Find at least one route which has the same parameters
		var rtFound bool
		for rtIdx, linuxRtEntry := range linuxRtList {
			// Route interface interface
			var hostName string
			var ifData *ifaceidx.IndexedLinuxInterface
			if linuxRtEntry.LinkIndex != 0 {
				var found bool
				var nsName string
				if nbRoute.Namespace == nil {
					nsName = ifaceidx.DefNs
				} else {
					nsName = nbRoute.Namespace.Name
				}
				_, ifData, found = c.ifIndexes.LookupNameByNamespace(uint32(linuxRtEntry.LinkIndex), nsName)
				if !found || ifData == nil {
					c.log.Debugf("Interface %d (data %v) not found for route", linuxRtEntry.LinkIndex, ifData)
				} else {
					hostName = ifData.Data.HostIfName
				}
			}
			linuxRt := c.transformRoute(linuxRtEntry, hostName)
			if c.isRouteEqual(rtIdx, nbRoute, linuxRt) {
				rtFound = true
				break
			}
		}
		if rtFound {
			// Register route if found
			c.rtIndexes.RegisterName(nbRoute.Name, c.rtIdxSeq, nbRoute)
			c.rtIdxSeq++
			c.log.Debugf("Linux route resync: %s was found and registered without additional changes", nbRoute.Name)
			// Resolve cached routes
			if !nbRoute.Default {
				if err := c.retryDefaultRoutes(nbRoute); err != nil {
					return errors.Errorf("Linux route resync error: retrying cached default routes caused %v", err)
				}
			}
		} else {
			// Configure route if not found
			c.log.Debugf("RLinux route resync: %s was not found and will be configured", nbRoute.Name)
			if err := c.ConfigureLinuxStaticRoute(nbRoute); err != nil {
				return errors.Errorf("linux route resync error: failed to configure %s: %v", nbRoute.Name, err)
			}
		}
	}

	return nil
}

// Look for routes similar to provided NB config in respective namespace. Routes can be read using destination address
// or interface. FOr every config, both ways are used.
func (c *LinuxRouteConfigurator) findLinuxRoutes(nbRoute *l3.LinuxStaticRoutes_Route, nsMgmtCtx *nsplugin.NamespaceMgmtCtx) ([]netlink.Route, error) {
	c.log.Debugf("Looking for equivalent linux routes for %s", nbRoute.Name)
	// Move to proper namespace
	if nbRoute.Namespace != nil {
		// Switch to namespace
		routeNs := c.nsHandler.RouteNsToGeneric(nbRoute.Namespace)
		revertNs, err := c.nsHandler.SwitchNamespace(routeNs, nsMgmtCtx)
		if err != nil {
			return nil, errors.Errorf("Linux route %s resync error: failed to switch to namespace %s: %v",
				nbRoute.Name, nbRoute.Namespace.Name, err)
		}
		defer revertNs()
	}
	var linuxRoutes []netlink.Route
	// Look for routes using destination IP address
	if nbRoute.DstIpAddr != "" && c.networkReachable(nbRoute.Namespace, nbRoute.DstIpAddr) {
		_, dstNetIP, err := net.ParseCIDR(nbRoute.DstIpAddr)
		if err != nil {
			return nil, errors.Errorf("failed to parse destination IP address %s: %v", nbRoute.DstIpAddr, err)
		}
		linuxRts, err := netlink.RouteGet(dstNetIP.IP)
		if err != nil {
			return nil, errors.Errorf("failed to read linux route %s using address %s: %v",
				nbRoute.Name, nbRoute.DstIpAddr, err)
		}
		if linuxRts != nil {
			linuxRoutes = append(linuxRoutes, linuxRts...)
		}
	}
	// Look for routes using interface
	if nbRoute.Interface != "" {
		// Look whether interface is registered
		_, meta, found := c.ifIndexes.LookupIdx(nbRoute.Interface)
		if !found {
			// Should not happen, was successfully checked before
			return nil, errors.Errorf("route %s interface %s is missing from the mapping", nbRoute.Name, nbRoute.Interface)
		} else if meta == nil || meta.Data == nil {
			return nil, errors.Errorf("interface %s data missing", nbRoute.Interface)
		} else {
			// Look for interface using host name
			link, err := netlink.LinkByName(meta.Data.HostIfName)
			if err != nil {
				return nil, errors.Errorf("failed to read interface %s: %v", meta.Data.HostIfName, err)
			}
			linuxRts, err := netlink.RouteList(link, netlink.FAMILY_ALL)
			if err != nil {
				return nil, errors.Errorf("failed to read linux route %s using interface %s: %v",
					nbRoute.Name, meta.Data.HostIfName, err)
			}
			if linuxRts != nil {
				linuxRoutes = append(linuxRoutes, linuxRts...)
			}
		}
	}

	if len(linuxRoutes) == 0 {
		c.log.Debugf("Equivalent for route %s was not found", nbRoute.Name)
	}

	return linuxRoutes, nil
}

// Compare all route parameters and returns true if routes are equal, false otherwise
func (c *LinuxRouteConfigurator) isRouteEqual(rtIdx int, nbRoute, linuxRt *l3.LinuxStaticRoutes_Route) bool {
	// Interface (if exists)
	if nbRoute.Interface != "" && nbRoute.Interface != linuxRt.Interface {
		c.log.Debugf("Linux route %d: interface is different (NB: %s, Linux: %s)",
			rtIdx, nbRoute.Interface, linuxRt.Interface)
		return false
	}
	// Default route
	if nbRoute.Default {
		if !linuxRt.Default {
			c.log.Debugf("Linux route %d: NB route is default, but linux route is not", rtIdx)
			return false
		}
		if nbRoute.GwAddr != linuxRt.GwAddr {
			c.log.Debugf("Linux route %d: gateway is different (NB: %s, Linux: %s)",
				rtIdx, nbRoute.GwAddr, linuxRt.GwAddr)
			return false
		}
		if nbRoute.Metric != linuxRt.Metric {
			c.log.Debugf("Linux route %d: metric is different (NB: %s, Linux: %s)",
				rtIdx, nbRoute.Metric, linuxRt.Metric)
			return false
		}
		return true
	}
	// Static route
	_, nbIPNet, err := net.ParseCIDR(nbRoute.DstIpAddr)
	if err != nil {
		c.log.Error(err)
		return false
	}
	if nbIPNet.IP.String() != linuxRt.DstIpAddr {
		c.log.Debugf("Linux route %d: destination address is different (NB: %s, Linux: %s)",
			rtIdx, nbIPNet.IP.String(), linuxRt.DstIpAddr)
		return false
	}
	// Compare source IP/gateway
	if nbRoute.SrcIpAddr == "" && linuxRt.SrcIpAddr != "" || nbRoute.SrcIpAddr != "" && linuxRt.SrcIpAddr == "" {
		if nbRoute.SrcIpAddr == "" && nbRoute.SrcIpAddr != linuxRt.GwAddr {
			c.log.Debugf("Linux route %d: source does not match gateway (NB: %s, Linux: %s)",
				rtIdx, nbRoute.SrcIpAddr, linuxRt.SrcIpAddr)
			return false
		} else if linuxRt.SrcIpAddr == "" && nbRoute.GwAddr != linuxRt.SrcIpAddr {
			c.log.Debugf("Linux route %d: source does not match gateway (NB: %s, Linux: %s)",
				rtIdx, nbRoute.SrcIpAddr, linuxRt.SrcIpAddr)
			return false
		}
	} else if nbRoute.SrcIpAddr != "" && linuxRt.SrcIpAddr != "" && nbRoute.SrcIpAddr != linuxRt.SrcIpAddr {
		c.log.Debugf("Linux route %d: source address is different (NB: %s, Linux: %s)",
			rtIdx, nbRoute.SrcIpAddr, linuxRt.SrcIpAddr)
		return false
	}

	if nbRoute.SrcIpAddr != "" && nbRoute.SrcIpAddr != linuxRt.SrcIpAddr {
		c.log.Debugf("Linux route %d: source address is different (NB: %s, Linux: %s)",
			rtIdx, nbRoute.SrcIpAddr, linuxRt.SrcIpAddr)
		return false
	}
	// If NB scope is nil, set scope type LINK (default value)
	if nbRoute.Scope == nil {
		nbRoute.Scope = &l3.LinuxStaticRoutes_Route_Scope{
			Type: l3.LinuxStaticRoutes_Route_Scope_LINK,
		}
	} else if linuxRt.Scope != nil {
		if nbRoute.Scope.Type != linuxRt.Scope.Type {
			c.log.Debugf("Linux route %d: scope is different (NB: %s, Linux: %s)",
				rtIdx, nbRoute.Scope.Type, linuxRt.Scope.Type)
			return false
		}
	}

	return true
}

// Parse netlink type scope to proto
func (c *LinuxRouteConfigurator) parseLinuxRouteScope(scope netlink.Scope) *l3.LinuxStaticRoutes_Route_Scope {
	switch scope {
	case netlink.SCOPE_UNIVERSE:
		return &l3.LinuxStaticRoutes_Route_Scope{
			Type: l3.LinuxStaticRoutes_Route_Scope_GLOBAL,
		}
	case netlink.SCOPE_HOST:
		return &l3.LinuxStaticRoutes_Route_Scope{
			Type: l3.LinuxStaticRoutes_Route_Scope_HOST,
		}
	case netlink.SCOPE_LINK:
		return &l3.LinuxStaticRoutes_Route_Scope{
			Type: l3.LinuxStaticRoutes_Route_Scope_LINK,
		}
	case netlink.SCOPE_SITE:
		return &l3.LinuxStaticRoutes_Route_Scope{
			Type: l3.LinuxStaticRoutes_Route_Scope_SITE,
		}
	default:
		c.log.Infof("Unknown scope type, setting to default (link): %v", scope)
		return &l3.LinuxStaticRoutes_Route_Scope{
			Type: l3.LinuxStaticRoutes_Route_Scope_LINK,
		}
	}
}
