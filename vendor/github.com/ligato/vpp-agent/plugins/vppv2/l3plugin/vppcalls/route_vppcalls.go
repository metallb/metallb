//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package vppcalls

import (
	"fmt"
	"net"

	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/ligato/vpp-agent/api/models/vpp/l3"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/pkg/errors"
)

const (
	// NextHopViaLabelUnset constant has to be assigned into the field next hop
	// via label in ip_add_del_route binary message if next hop via label is not defined.
	// Equals to MPLS_LABEL_INVALID defined in VPP
	NextHopViaLabelUnset uint32 = 0xfffff + 1

	// ClassifyTableIndexUnset is a default value for field classify_table_index in ip_add_del_route binary message.
	ClassifyTableIndexUnset = ^uint32(0)

	// NextHopOutgoingIfUnset constant has to be assigned into the field next_hop_outgoing_interface
	// in ip_add_del_route binary message if outgoing interface for next hop is not defined.
	NextHopOutgoingIfUnset = ^uint32(0)
)

// vppAddDelRoute adds or removes route, according to provided input. Every route has to contain VRF ID (default is 0).
func (h *RouteHandler) vppAddDelRoute(route *vpp_l3.Route, rtIfIdx uint32, delete bool) error {
	req := &ip.IPAddDelRoute{}
	if delete {
		req.IsAdd = 0
	} else {
		req.IsAdd = 1
	}

	// Destination address (route set identifier)
	parsedDstIP, isIpv6, err := addrs.ParseIPWithPrefix(route.DstNetwork)
	if err != nil {
		return err
	}
	parsedNextHopIP := net.ParseIP(route.NextHopAddr)
	prefix, _ := parsedDstIP.Mask.Size()
	if isIpv6 {
		req.IsIPv6 = 1
		req.DstAddress = []byte(parsedDstIP.IP.To16())
		req.NextHopAddress = []byte(parsedNextHopIP.To16())
	} else {
		req.IsIPv6 = 0
		req.DstAddress = []byte(parsedDstIP.IP.To4())
		req.NextHopAddress = []byte(parsedNextHopIP.To4())
	}
	req.DstAddressLength = byte(prefix)

	// Common route parameters
	req.NextHopWeight = uint8(route.Weight)
	req.NextHopPreference = uint8(route.Preference)
	req.NextHopViaLabel = NextHopViaLabelUnset
	req.ClassifyTableIndex = ClassifyTableIndexUnset

	// VRF/Other route parameters based on type
	req.TableID = route.VrfId
	if route.Type == vpp_l3.Route_INTER_VRF {
		req.NextHopSwIfIndex = rtIfIdx
		req.NextHopTableID = route.ViaVrfId
	} else if route.Type == vpp_l3.Route_DROP {
		req.IsDrop = 1
	} else {
		req.NextHopSwIfIndex = rtIfIdx
		req.NextHopTableID = route.VrfId
	}

	// Multi path is always true
	req.IsMultipath = 1

	// Send message
	reply := &ip.IPAddDelRouteReply{}
	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

// VppAddRoute implements route handler.
func (h *RouteHandler) VppAddRoute(route *vpp_l3.Route) error {
	// Evaluate route IP version
	_, isIPv6, err := addrs.ParseIPWithPrefix(route.DstNetwork)
	if err != nil {
		return err
	}

	// Configure IPv6 VRF
	if err := h.createVrfIfNeeded(route.VrfId, isIPv6); err != nil {
		return err
	}
	if route.Type == vpp_l3.Route_INTER_VRF {
		if err := h.createVrfIfNeeded(route.ViaVrfId, isIPv6); err != nil {
			return err
		}
	}

	swIfIdx, err := h.getRouteSwIfIndex(route.OutgoingInterface)
	if err != nil {
		return err
	}

	return h.vppAddDelRoute(route, swIfIdx, false)
}

// VppDelRoute implements route handler.
func (h *RouteHandler) VppDelRoute(route *vpp_l3.Route) error {
	swIfIdx, err := h.getRouteSwIfIndex(route.OutgoingInterface)
	if err != nil {
		return err
	}

	return h.vppAddDelRoute(route, swIfIdx, true)
}

func (h *RouteHandler) getRouteSwIfIndex(ifName string) (swIfIdx uint32, err error) {
	swIfIdx = NextHopOutgoingIfUnset
	if ifName != "" {
		meta, found := h.ifIndexes.LookupByName(ifName)
		if !found {
			return 0, errors.Errorf("interface %s not found", ifName)
		}
		swIfIdx = meta.SwIfIndex
	}
	return
}

// New VRF with provided ID for IPv4 or IPv6 will be created if missing.
func (h *RouteHandler) createVrfIfNeeded(vrfID uint32, isIPv6 bool) error {
	// Zero VRF exists by default
	if vrfID == 0 {
		return nil
	}

	// Get all VRFs for IPv4 or IPv6
	var exists bool
	if isIPv6 {
		ipv6Tables, err := h.dumpVrfTablesIPv6()
		if err != nil {
			return fmt.Errorf("dumping IPv6 VRF tables failed: %v", err)
		}
		_, exists = ipv6Tables[vrfID]
	} else {
		tables, err := h.dumpVrfTables()
		if err != nil {
			return fmt.Errorf("dumping IPv4 VRF tables failed: %v", err)
		}
		_, exists = tables[vrfID]
	}
	// Create new VRF if needed
	if !exists {
		h.log.Debugf("VRF table %d does not exists and will be created", vrfID)
		return h.vppAddIPTable(vrfID, isIPv6)
	}

	return nil
}

// Returns all IPv4 VRF tables
func (h *RouteHandler) dumpVrfTables() (map[uint32][]*ip.IPFibDetails, error) {
	fibs := map[uint32][]*ip.IPFibDetails{}
	reqCtx := h.callsChannel.SendMultiRequest(&ip.IPFibDump{})
	for {
		fibDetails := &ip.IPFibDetails{}
		stop, err := reqCtx.ReceiveReply(fibDetails)
		if stop {
			break
		}
		if err != nil {
			return nil, err
		}

		tableID := fibDetails.TableID
		fibs[tableID] = append(fibs[tableID], fibDetails)
	}

	return fibs, nil
}

// Returns all IPv6 VRF tables
func (h *RouteHandler) dumpVrfTablesIPv6() (map[uint32][]*ip.IP6FibDetails, error) {
	fibs := map[uint32][]*ip.IP6FibDetails{}
	reqCtx := h.callsChannel.SendMultiRequest(&ip.IP6FibDump{})
	for {
		fibDetails := &ip.IP6FibDetails{}
		stop, err := reqCtx.ReceiveReply(fibDetails)
		if stop {
			break
		}
		if err != nil {
			return nil, err
		}

		tableID := fibDetails.TableID
		fibs[tableID] = append(fibs[tableID], fibDetails)
	}

	return fibs, nil
}

// Creates new VRF table with provided ID and for desired IP version
func (h *RouteHandler) vppAddIPTable(vrfID uint32, isIPv6 bool) error {
	req := &ip.IPTableAddDel{
		TableID: vrfID,
		IsIPv6:  boolToUint(isIPv6),
		IsAdd:   1,
	}
	reply := &ip.IPTableAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}
