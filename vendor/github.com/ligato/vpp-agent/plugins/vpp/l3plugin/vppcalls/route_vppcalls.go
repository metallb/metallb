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

package vppcalls

import (
	"fmt"
	"net"

	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	ifvppcalls "github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
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
func (h *RouteHandler) vppAddDelRoute(route *l3.StaticRoutes_Route, rtIfIdx uint32, delete bool) error {
	req := &ip.IPAddDelRoute{}
	if delete {
		req.IsAdd = 0
	} else {
		req.IsAdd = 1
	}

	// Destination address (route set identifier)
	parsedDstIP, isIpv6, err := addrs.ParseIPWithPrefix(route.DstIpAddr)
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
	if route.Type == l3.StaticRoutes_Route_INTER_VRF {
		req.NextHopSwIfIndex = rtIfIdx
		req.NextHopTableID = route.ViaVrfId
	} else if route.Type == l3.StaticRoutes_Route_DROP {
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
	if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// VppAddRoute implements route handler.
func (h *RouteHandler) VppAddRoute(ifHandler ifvppcalls.IfVppWrite, route *l3.StaticRoutes_Route, rtIfIdx uint32) error {
	// Evaluate route IP version
	_, isIPv6, err := addrs.ParseIPWithPrefix(route.DstIpAddr)
	if err != nil {
		return err
	}

	if isIPv6 {
		// Configure IPv6 VRF
		if err := ifHandler.CreateVrfIPv6(route.VrfId); err != nil {
			return err
		}
		if route.Type == l3.StaticRoutes_Route_INTER_VRF {
			if err := ifHandler.CreateVrfIPv6(route.ViaVrfId); err != nil {
				return err
			}
		}
	} else {
		// Configure IPv4 VRF
		if err := ifHandler.CreateVrf(route.VrfId); err != nil {
			return err
		}
		if route.Type == l3.StaticRoutes_Route_INTER_VRF {
			if err := ifHandler.CreateVrf(route.ViaVrfId); err != nil {
				return err
			}
		}
	}
	return h.vppAddDelRoute(route, rtIfIdx, false)
}

// VppDelRoute implements route handler.
func (h *RouteHandler) VppDelRoute(route *l3.StaticRoutes_Route, rtIfIdx uint32) error {
	return h.vppAddDelRoute(route, rtIfIdx, true)
}
