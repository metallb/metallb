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

package vppcalls

import (
	"fmt"

	"github.com/go-errors/errors"

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/nat"
	nat2 "github.com/ligato/vpp-agent/plugins/vpp/model/nat"
)

// Num protocol representation
const (
	ICMP uint8 = 1
	TCP  uint8 = 6
	UDP  uint8 = 17
)

const (
	// NoInterface is sw-if-idx which means 'no interface'
	NoInterface uint32 = 0xffffffff
	// Maximal length of tag
	maxTagLen = 64
)

// StaticMappingContext groups common fields required for static mapping
type StaticMappingContext struct {
	Tag           string
	AddressOnly   bool
	LocalIP       []byte
	LocalPort     uint16
	ExternalIP    []byte
	ExternalPort  uint16
	ExternalIfIdx uint32
	Protocol      uint8
	Vrf           uint32
	TwiceNat      bool
	SelfTwiceNat  bool
}

// StaticMappingLbContext groups common fields required for static mapping with load balancer
type StaticMappingLbContext struct {
	Tag          string
	LocalIPs     []*LocalLbAddress
	ExternalIP   []byte
	ExternalPort uint16
	Protocol     uint8
	TwiceNat     bool
	SelfTwiceNat bool
}

// IdentityMappingContext groups common fields required for identity mapping
type IdentityMappingContext struct {
	Tag       string
	IPAddress []byte
	Protocol  uint8
	Port      uint16
	IfIdx     uint32
	Vrf       uint32
}

// LocalLbAddress represents one local IP and address entry
type LocalLbAddress struct {
	Vrf         uint32
	Tag         string
	LocalIP     []byte
	LocalPort   uint16
	Probability uint8
}

// SetNat44Forwarding implements NAT handler.
func (h *NatVppHandler) SetNat44Forwarding(enableFwd bool) error {
	req := &nat.Nat44ForwardingEnableDisable{
		Enable: boolToUint(enableFwd),
	}
	reply := &nat.Nat44ForwardingEnableDisableReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// Calls VPP binary API to set/unset interface as NAT
func (h *NatVppHandler) handleNat44Interface(ifIdx uint32, isInside, isAdd bool) error {
	req := &nat.Nat44InterfaceAddDelFeature{
		SwIfIndex: ifIdx,
		IsInside:  boolToUint(isInside),
		IsAdd:     boolToUint(isAdd),
	}
	reply := &nat.Nat44InterfaceAddDelFeatureReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// Calls VPP binary API to set/unset interface as NAT with output feature
func (h *NatVppHandler) handleNat44InterfaceOutputFeature(ifIdx uint32, isInside, isAdd bool) error {
	req := &nat.Nat44InterfaceAddDelOutputFeature{
		SwIfIndex: ifIdx,
		IsInside:  boolToUint(isInside),
		IsAdd:     boolToUint(isAdd),
	}
	reply := &nat.Nat44InterfaceAddDelOutputFeatureReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// Calls VPP binary API to add/remove address pool
func (h *NatVppHandler) handleNat44AddressPool(first, last []byte, vrf uint32, twiceNat, isAdd bool) error {
	req := &nat.Nat44AddDelAddressRange{
		FirstIPAddress: first,
		LastIPAddress:  last,
		VrfID:          vrf,
		TwiceNat:       boolToUint(twiceNat),
		IsAdd:          boolToUint(isAdd),
	}
	reply := &nat.Nat44AddDelAddressRangeReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// Calls VPP binary API to setup NAT virtual reassembly
func (h *NatVppHandler) handleNat44VirtualReassembly(timeout, maxReass, maxFrag uint32, dropFrag, isIpv6 bool) error {
	req := &nat.NatSetReass{
		Timeout:  timeout,
		MaxReass: uint16(maxReass),
		MaxFrag:  uint8(maxFrag),
		DropFrag: boolToUint(dropFrag),
		IsIP6:    boolToUint(isIpv6),
	}
	reply := &nat.NatSetReassReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// Calls VPP binary API to add/remove static mapping
func (h *NatVppHandler) handleNat44StaticMapping(ctx *StaticMappingContext, isAdd, addrOnly bool) error {
	if err := checkTagLength(ctx.Tag); err != nil {
		return err
	}

	req := &nat.Nat44AddDelStaticMapping{
		Tag:               []byte(ctx.Tag),
		LocalIPAddress:    ctx.LocalIP,
		LocalPort:         ctx.LocalPort,
		ExternalIPAddress: ctx.ExternalIP,
		ExternalPort:      ctx.ExternalPort,
		Protocol:          ctx.Protocol,
		ExternalSwIfIndex: ctx.ExternalIfIdx,
		VrfID:             ctx.Vrf,
		TwiceNat:          boolToUint(ctx.TwiceNat),
		SelfTwiceNat:      boolToUint(ctx.SelfTwiceNat),
		Out2inOnly:        1,
		IsAdd:             boolToUint(isAdd),
	}
	if addrOnly {
		req.AddrOnly = 1
	} else {
		req.LocalPort = ctx.LocalPort
		req.ExternalPort = ctx.ExternalPort
	}
	reply := &nat.Nat44AddDelStaticMappingReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// Calls VPP binary API to add/remove static mapping with load balancer
func (h *NatVppHandler) handleNat44StaticMappingLb(ctx *StaticMappingLbContext, isAdd bool) error {
	if err := checkTagLength(ctx.Tag); err != nil {
		return err
	}

	// Transform local IP/Ports
	var localAddrPorts []nat.Nat44LbAddrPort
	for _, ctxLocal := range ctx.LocalIPs {
		localAddrPort := nat.Nat44LbAddrPort{
			Addr:        ctxLocal.LocalIP,
			Port:        ctxLocal.LocalPort,
			Probability: ctxLocal.Probability,
			VrfID:       ctxLocal.Vrf,
		}
		localAddrPorts = append(localAddrPorts, localAddrPort)
	}

	req := &nat.Nat44AddDelLbStaticMapping{
		Tag:          []byte(ctx.Tag),
		Locals:       localAddrPorts,
		LocalNum:     uint8(len(localAddrPorts)),
		ExternalAddr: ctx.ExternalIP,
		ExternalPort: ctx.ExternalPort,
		Protocol:     ctx.Protocol,
		TwiceNat:     boolToUint(ctx.TwiceNat),
		SelfTwiceNat: boolToUint(ctx.SelfTwiceNat),
		Out2inOnly:   1,
		IsAdd:        boolToUint(isAdd),
	}
	reply := &nat.Nat44AddDelLbStaticMappingReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// Calls VPP binary API to add/remove identity mapping
func (h *NatVppHandler) handleNat44IdentityMapping(ctx *IdentityMappingContext, isAdd bool) error {
	if err := checkTagLength(ctx.Tag); err != nil {
		return err
	}

	req := &nat.Nat44AddDelIdentityMapping{
		Tag: []byte(ctx.Tag),
		AddrOnly: func(port uint16, ip []byte) uint8 {
			// Set addr only if port is set to zero
			if port == 0 || ip == nil {
				return 1
			}
			return 0
		}(ctx.Port, ctx.IPAddress),
		IPAddress: ctx.IPAddress,
		Port:      ctx.Port,
		Protocol:  ctx.Protocol,
		SwIfIndex: func(ifIdx uint32) uint32 {
			if ifIdx == 0 {
				return NoInterface
			}
			return ifIdx
		}(ctx.IfIdx),
		VrfID: ctx.Vrf,
		IsAdd: boolToUint(isAdd),
	}
	reply := &nat.Nat44AddDelIdentityMappingReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// checkTagLength serves as a validator for static/identity mapping tag length
func checkTagLength(tag string) error {
	if len(tag) > maxTagLen {
		return errors.Errorf("load-balanced static mapping label '%s' has %d bytes, max allowed is %d",
			tag, len(tag), maxTagLen)
	}
	return nil
}

// EnableNat44Interface implements NAT handler.
func (h *NatVppHandler) EnableNat44Interface(ifIdx uint32, isInside bool) error {
	return h.handleNat44Interface(ifIdx, isInside, true)
}

// DisableNat44Interface implements NAT handler.
func (h *NatVppHandler) DisableNat44Interface(ifIdx uint32, isInside bool) error {
	return h.handleNat44Interface(ifIdx, isInside, false)
}

// EnableNat44InterfaceOutput implements NAT handler.
func (h *NatVppHandler) EnableNat44InterfaceOutput(ifIdx uint32, isInside bool) error {
	return h.handleNat44InterfaceOutputFeature(ifIdx, isInside, true)
}

// DisableNat44InterfaceOutput implements NAT handler.
func (h *NatVppHandler) DisableNat44InterfaceOutput(ifIdx uint32, isInside bool) error {
	return h.handleNat44InterfaceOutputFeature(ifIdx, isInside, false)
}

// AddNat44AddressPool implements NAT handler.
func (h *NatVppHandler) AddNat44AddressPool(first, last []byte, vrf uint32, twiceNat bool) error {
	return h.handleNat44AddressPool(first, last, vrf, twiceNat, true)
}

// DelNat44AddressPool implements NAT handler.
func (h *NatVppHandler) DelNat44AddressPool(first, last []byte, vrf uint32, twiceNat bool) error {
	return h.handleNat44AddressPool(first, last, vrf, twiceNat, false)
}

// SetVirtualReassemblyIPv4 implements NAT handler.
func (h *NatVppHandler) SetVirtualReassemblyIPv4(vrCfg *nat2.Nat44Global_VirtualReassembly) error {
	return h.handleNat44VirtualReassembly(vrCfg.Timeout, vrCfg.MaxReass, vrCfg.MaxFrag, vrCfg.DropFrag, false)
}

// SetVirtualReassemblyIPv6 implements NAT handler.
func (h *NatVppHandler) SetVirtualReassemblyIPv6(vrCfg *nat2.Nat44Global_VirtualReassembly) error {
	return h.handleNat44VirtualReassembly(vrCfg.Timeout, vrCfg.MaxReass, vrCfg.MaxFrag, vrCfg.DropFrag, true)
}

// AddNat44IdentityMapping implements NAT handler.
func (h *NatVppHandler) AddNat44IdentityMapping(ctx *IdentityMappingContext) error {
	return h.handleNat44IdentityMapping(ctx, true)
}

// DelNat44IdentityMapping implements NAT handler.
func (h *NatVppHandler) DelNat44IdentityMapping(ctx *IdentityMappingContext) error {
	return h.handleNat44IdentityMapping(ctx, false)
}

// AddNat44StaticMapping implements NAT handler.
func (h *NatVppHandler) AddNat44StaticMapping(ctx *StaticMappingContext) error {
	if ctx.AddressOnly {
		return h.handleNat44StaticMapping(ctx, true, true)
	}
	return h.handleNat44StaticMapping(ctx, true, false)
}

// DelNat44StaticMapping implements NAT handler.
func (h *NatVppHandler) DelNat44StaticMapping(ctx *StaticMappingContext) error {
	if ctx.AddressOnly {
		return h.handleNat44StaticMapping(ctx, false, true)
	}
	return h.handleNat44StaticMapping(ctx, false, false)
}

// AddNat44StaticMappingLb implements NAT handler.
func (h *NatVppHandler) AddNat44StaticMappingLb(ctx *StaticMappingLbContext) error {
	return h.handleNat44StaticMappingLb(ctx, true)
}

// DelNat44StaticMappingLb implements NAT handler.
func (h *NatVppHandler) DelNat44StaticMappingLb(ctx *StaticMappingLbContext) error {
	return h.handleNat44StaticMappingLb(ctx, false)
}
