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

package linuxcalls

import (
	"net"
	"strings"

	"github.com/go-errors/errors"
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
	"github.com/ligato/vpp-agent/plugins/linux/nsplugin"
	"github.com/vishvananda/netlink"
)

// LinuxArpDetails is the wrapper structure for the linux ARP northbound API structure.
type LinuxArpDetails struct {
	Arp  *l3.LinuxStaticArpEntries_ArpEntry `json:"linux_arp"`
	Meta *LinuxArpMeta                      `json:"linux_arp_meta"`
}

// LinuxArpMeta is combination of proto-modelled ARP data and linux provided metadata
type LinuxArpMeta struct {
	LinkIndex int `json:"link_index"`
	Family    int `json:"family"`
	State     int `json:"state"`
	Type      int `json:"type"`
	Flags     int `json:"flags"`
	Vlan      int `json:"vlan"`
	VNI       int `json:"vni"`
}

// DumpArpEntries is an implementation of linux L3 handler
func (h *NetLinkHandler) DumpArpEntries() ([]*LinuxArpDetails, error) {
	var arps []*LinuxArpDetails

	ctx := nsplugin.NewNamespaceMgmtCtx()

	// Iterate over all known ARP entries
	for _, arpName := range h.arpIndexes.GetMapping().ListNames() {
		arpDetails := &LinuxArpDetails{}

		_, meta, found := h.arpIndexes.LookupIdx(arpName)
		if !found {
			h.log.Warnf("Expected ARP %s not found in the mapping", arpName)
			continue
		}
		if meta == nil {
			h.log.Warnf("Expected ARP %s metadata are missing", arpName)
			continue
		}

		// Copy base configuration from mapping metadata. Linux specific fields are stored in LinuxArpMeta.
		arpDetails.Arp = meta
		linuxArp, err := h.dumpArpData(meta, ctx)
		if err != nil {
			// Do not return error, read what is possible
			h.log.Errorf("failed to get ARP %s data: %v", arpName, err)
			continue
		}

		if linuxArp == nil {
			h.log.Warnf("Linux equivalent for ARP %s not found", arpName)
			continue
		}

		// Base fields
		arpMeta := &LinuxArpMeta{
			LinkIndex: linuxArp.LinkIndex,
			Family:    linuxArp.Family,
			State:     linuxArp.State,
			Type:      linuxArp.Type,
			Flags:     linuxArp.Flags,
			Vlan:      linuxArp.Vlan,
			VNI:       linuxArp.VNI,
		}

		arpDetails.Meta = arpMeta
		arps = append(arps, arpDetails)
	}

	return arps, nil
}

// LinuxRouteDetails is the wrapper structure for the linux route northbound API structure.
type LinuxRouteDetails struct {
	Route *l3.LinuxStaticRoutes_Route `json:"linux_route"`
	Meta  *LinuxRouteMeta             `json:"linux_route_meta"`
}

// LinuxRouteMeta is combination of proto-modelled route data and linux provided metadata
type LinuxRouteMeta struct {
	LinkIndex  int        `json:"link_index"`
	ILinkIndex int        `json:"ilink_index"`
	Scope      int        `json:"scope"`
	Protocol   int        `json:"protocol"`
	Priority   int        `json:"priority"`
	Table      int        `json:"table"`
	Type       int        `json:"type"`
	Tos        int        `json:"tos"`
	Flags      int        `json:"flags"`
	MTU        int        `json:"mtu"`
	AdvMSS     int        `json:"adv_mss"`
	NextHops   []*nextHop `json:"next_hops"`
}

// Helper struct for next hops
type nextHop struct {
	Index int
	Hops  int
	GwIP  string
	Flags int
}

// DumpRoutes is an implementation of linux route handler
func (h *NetLinkHandler) DumpRoutes() ([]*LinuxRouteDetails, error) {
	var routes []*LinuxRouteDetails

	ctx := nsplugin.NewNamespaceMgmtCtx()

	// Iterate over all known Route entries
	for _, rtName := range h.routeIndexes.GetMapping().ListNames() {
		rtDetails := &LinuxRouteDetails{}

		_, meta, found := h.routeIndexes.LookupIdx(rtName)
		if !found {
			h.log.Warnf("Expected route %s not found in the mapping", rtName)
			continue
		}
		if meta == nil {
			h.log.Warnf("Expected route %s metadata are missing", rtName)
			continue
		}

		// Copy base configuration from mapping metadata. Linux specific fields are stored in LinuxRouteMeta.
		rtDetails.Route = meta
		linuxRt, err := h.dumpRouteData(meta, ctx)
		if err != nil {
			// Do not return error, read what is possible
			h.log.Errorf("failed to get route %s data: %v", rtName, err)
			continue
		}

		if linuxRt == nil {
			h.log.Warnf("Linux equivalent for route %s not found", rtName)
			continue
		}

		// Base fields
		rtMeta := &LinuxRouteMeta{
			LinkIndex:  linuxRt.LinkIndex,
			ILinkIndex: linuxRt.ILinkIndex,
			Scope:      int(linuxRt.Scope),
			Protocol:   linuxRt.Protocol,
			Priority:   linuxRt.Priority,
			Table:      linuxRt.Table,
			Type:       linuxRt.Type,
			Tos:        linuxRt.Tos,
			Flags:      linuxRt.Flags,
			MTU:        linuxRt.MTU,
			AdvMSS:     linuxRt.AdvMSS,
			NextHops: func(nhInfo []*netlink.NexthopInfo) (hops []*nextHop) {
				for _, nh := range nhInfo {
					hops = append(hops, &nextHop{
						Index: nh.LinkIndex,
						Hops:  nh.Hops,
						GwIP:  nh.Gw.String(),
						Flags: nh.Flags,
					})
				}
				return hops
			}(linuxRt.MultiPath),
		}

		rtDetails.Meta = rtMeta
		routes = append(routes, rtDetails)
	}

	return routes, nil
}

// Reads interface data and ip addresses from provided namespace
func (h *NetLinkHandler) dumpArpData(arp *l3.LinuxStaticArpEntries_ArpEntry, ctx *nsplugin.NamespaceMgmtCtx) (*netlink.Neigh, error) {
	revert, err := h.nsHandler.SwitchNamespace(h.nsHandler.ArpNsToGeneric(arp.Namespace), ctx)
	defer revert()

	var linuxArpEntry *netlink.Neigh

	if err != nil {
		return nil, errors.Errorf("failed to switch to namespace: %v", err)
	}
	linuxArps, err := h.GetArpEntries(0, 0) // No interface/family filter
	if err != nil {
		return nil, errors.Errorf("failed to get ARPs from namespace: %v", err)
	}

	// Parse correct ARP
	for _, linuxArp := range linuxArps {
		// Parse interface
		if arp.Interface != "" {
			_, meta, found := h.ifIndexes.LookupIdx(arp.Interface)
			if !found || meta == nil {
				h.log.Warnf("Interface %s for ARP %s not found", arp.Namespace, arp.Name)
				continue
			}
			if meta.Index != uint32(linuxArp.LinkIndex) {
				continue
			}
		}
		// Parse MAC address
		nbMac := strings.ToLower(arp.HwAddress)
		linuxMac := strings.ToLower(linuxArp.HardwareAddr.String())
		if nbMac != linuxMac {
			continue
		}
		// Parse IP address
		if arp.IpAddr != linuxArp.IP.String() {
			continue
		}

		linuxArpEntry = &linuxArp
	}

	return linuxArpEntry, nil
}

// Reads interface data and ip addresses from provided namespace
func (h *NetLinkHandler) dumpRouteData(rt *l3.LinuxStaticRoutes_Route, ctx *nsplugin.NamespaceMgmtCtx) (*netlink.Route, error) {
	revert, err := h.nsHandler.SwitchNamespace(h.nsHandler.RouteNsToGeneric(rt.Namespace), ctx)
	defer revert()

	var linuxRtEntry *netlink.Route

	if err != nil {
		return nil, errors.Errorf("failed to switch to namespace: %v", err)
	}

	linuxRoutes, err := h.GetStaticRoutes(nil, 0) // Means no filter, dump all
	if err != nil {
		return nil, errors.Errorf("failed to read linux routes: %v", err)
	}

	// Parse correct Route
	for _, linuxRt := range linuxRoutes {
		// Parse interface
		if rt.Interface != "" {
			_, meta, found := h.ifIndexes.LookupIdx(rt.Interface)
			if !found || meta == nil {
				h.log.Warnf("Interface %s for Route %s not found", rt.Namespace, rt.Name)
				continue
			}
			if meta.Index != uint32(linuxRt.LinkIndex) {
				continue
			}
		}
		// Parse Dst if exists
		if rt.DstIpAddr != "" {
			if linuxRt.Dst == nil {
				continue
			}
			_, rtIP, err := net.ParseCIDR(rt.DstIpAddr)
			if err != nil {
				h.log.Errorf("Failed to parse IP address %s: %v", rt.DstIpAddr, err)
				continue
			}
			if rtIP.String() != linuxRt.Dst.String() {
				continue
			}
		}
		// Parse Gw if exists
		if rt.GwAddr != "" {
			rtIP := net.ParseIP(rt.GwAddr)
			if rtIP.String() != linuxRt.Gw.String() {
				continue
			}
		}
		// Parse Src if exists
		if rt.SrcIpAddr != "" {
			rtIP := net.ParseIP(rt.SrcIpAddr)
			if rtIP.String() != linuxRt.Src.String() {
				continue
			}
		}

		linuxRtEntry = &linuxRt
	}

	return linuxRtEntry, nil
}
