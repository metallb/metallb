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

	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vxlan"
)

func (h *IfVppHandler) addDelVxLanTunnel(vxLan *interfaces.VxlanLink, vrf, multicastIf uint32, isAdd bool) (swIdx uint32, err error) {
	req := &vxlan.VxlanAddDelTunnel{
		IsAdd:          boolToUint(isAdd),
		Vni:            vxLan.Vni,
		DecapNextIndex: 0xFFFFFFFF,
		Instance:       ^uint32(0),
		EncapVrfID:     vrf,
		McastSwIfIndex: multicastIf,
	}

	srcAddr := net.ParseIP(vxLan.SrcAddress).To4()
	dstAddr := net.ParseIP(vxLan.DstAddress).To4()
	if srcAddr == nil && dstAddr == nil {
		srcAddr = net.ParseIP(vxLan.SrcAddress).To16()
		dstAddr = net.ParseIP(vxLan.DstAddress).To16()
		req.IsIPv6 = 1
		if srcAddr == nil || dstAddr == nil {
			return 0, fmt.Errorf("invalid VXLAN address, src: %s, dst: %s", srcAddr, dstAddr)
		}
	} else if srcAddr == nil && dstAddr != nil || srcAddr != nil && dstAddr == nil {
		return 0, fmt.Errorf("IP version mismatch for VXLAN destination and source IP addresses")
	}

	req.SrcAddress = []byte(srcAddr)
	req.DstAddress = []byte(dstAddr)

	// before the VxLAN is added, create a VRF table if needed
	if req.IsIPv6 == 1 {
		if err := h.CreateVrfIPv6(vrf); err != nil {
			return 0, err
		}
	} else {
		if err := h.CreateVrf(vrf); err != nil {
			return 0, err
		}
	}
	reply := &vxlan.VxlanAddDelTunnelReply{}
	if err = h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, err
	}

	return reply.SwIfIndex, nil
}

// AddVxLanTunnel implements VxLan handler.
func (h *IfVppHandler) AddVxLanTunnel(ifName string, vrf, multicastIf uint32, vxLan *interfaces.VxlanLink) (swIndex uint32, err error) {
	swIfIdx, err := h.addDelVxLanTunnel(vxLan, vrf, multicastIf, true)
	if err != nil {
		return 0, err
	}
	return swIfIdx, h.SetInterfaceTag(ifName, swIfIdx)
}

// DeleteVxLanTunnel implements VxLan handler.
func (h *IfVppHandler) DeleteVxLanTunnel(ifName string, idx, vrf uint32, vxLan *interfaces.VxlanLink) error {
	// Multicast does not need to be set
	if _, err := h.addDelVxLanTunnel(vxLan, vrf, 0, false); err != nil {
		return err
	}
	return h.RemoveInterfaceTag(ifName, idx)
}
