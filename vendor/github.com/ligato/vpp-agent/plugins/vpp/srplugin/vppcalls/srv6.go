// Copyright (c) 2018 Bell Canada, Pantheon Technologies and/or its affiliates.
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
	"strings"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/sr"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
)

// Constants for behavior function hardcoded into VPP (there can be also custom behavior functions implemented as VPP plugins)
// Constants are taken from VPP's vnet/srv6/sr.h (names are modified to Golang from original C form in VPP code)
const (
	BehaviorEnd    uint8 = iota + 1 // Behavior of simple endpoint
	BehaviorX                       // Behavior of endpoint with Layer-3 cross-connect
	BehaviorT                       // Behavior of endpoint with specific IPv6 table lookup
	BehaviorDfirst                  // Unused. Separator in between regular and D
	BehaviorDX2                     // Behavior of endpoint with decapulation and Layer-2 cross-connect (or DX2 with egress VLAN rewrite when VLAN notzero - not supported this variant yet)
	BehaviorDX6                     // Behavior of endpoint with decapsulation and IPv6 cross-connect
	BehaviorDX4                     // Behavior of endpoint with decapsulation and IPv4 cross-connect
	BehaviorDT6                     // Behavior of endpoint with decapsulation and specific IPv6 table lookup
	BehaviorDT4                     // Behavior of endpoint with decapsulation and specific IPv4 table lookup
	BehaviorLast                    // seems unused, note in VPP: "Must always be the last one"
)

// Constants for steering type
// Constants are taken from VPP's vnet/srv6/sr.h (names are modified to Golang from original C form in VPP code)
const (
	SteerTypeL2   uint8 = 2
	SteerTypeIPv4 uint8 = 4
	SteerTypeIPv6 uint8 = 6
)

// Constants for operation of SR policy modify binary API method
const (
	AddSRList            uint8 = iota + 1 // Add SR List to an existing SR policy
	DeleteSRList                          // Delete SR List from an existing SR policy
	ModifyWeightOfSRList                  // Modify the weight of an existing SR List
)

// AddLocalSid adds local sid given by <sidAddr> and <localSID> into VPP
func (h *SRv6VppHandler) AddLocalSid(sidAddr net.IP, localSID *srv6.LocalSID, swIfIndex ifaceidx.SwIfIndex) error {
	return h.addDelLocalSid(false, sidAddr, localSID, swIfIndex)
}

// DeleteLocalSid delets local sid given by <sidAddr> in VPP
func (h *SRv6VppHandler) DeleteLocalSid(sidAddr net.IP) error {
	return h.addDelLocalSid(true, sidAddr, nil, nil)
}

func (h *SRv6VppHandler) addDelLocalSid(deletion bool, sidAddr net.IP, localSID *srv6.LocalSID, swIfIndex ifaceidx.SwIfIndex) error {
	h.log.WithFields(logging.Fields{"localSID": sidAddr, "delete": deletion, "FIB table ID": h.fibTableID(localSID), "end function": h.endFunction(localSID)}).
		Debug("Adding/deleting Local SID", sidAddr)
	req := &sr.SrLocalsidAddDel{
		IsDel:    boolToUint(deletion),
		Localsid: sr.Srv6Sid{Addr: []byte(sidAddr)},
	}
	if !deletion {
		req.FibTable = localSID.FibTableId // where to install localsid entry
		if err := h.writeEndFunction(req, sidAddr, localSID, swIfIndex); err != nil {
			return err
		}
	}
	reply := &sr.SrLocalsidAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp call %q returned: %d", reply.GetMessageName(), reply.Retval)
	}

	h.log.WithFields(logging.Fields{"localSID": sidAddr, "delete": deletion, "FIB table ID": h.fibTableID(localSID), "end function": h.endFunction(localSID)}).
		Debug("Added/deleted Local SID ", sidAddr)

	return nil
}

func (h *SRv6VppHandler) fibTableID(localSID *srv6.LocalSID) string {
	if localSID != nil {
		return string(localSID.FibTableId)
	}
	return "<nil>"
}

func (h *SRv6VppHandler) endFunction(localSID *srv6.LocalSID) string {
	if localSID == nil {
		return "<nil>"
	} else if localSID.BaseEndFunction != nil {
		return fmt.Sprintf("End{psp: %v}", localSID.BaseEndFunction.Psp)
	} else if localSID.EndFunction_X != nil {
		return fmt.Sprintf("X{psp: %v, OutgoingInterface: %v, NextHop: %v}", localSID.EndFunction_X.Psp, localSID.EndFunction_X.OutgoingInterface, localSID.EndFunction_X.NextHop)
	} else if localSID.EndFunction_T != nil {
		return fmt.Sprintf("T{psp: %v}", localSID.EndFunction_T.Psp)
	} else if localSID.EndFunction_DX2 != nil {
		return fmt.Sprintf("DX2{VlanTag: %v, OutgoingInterface: %v, NextHop: %v}", localSID.EndFunction_DX2.VlanTag, localSID.EndFunction_DX2.OutgoingInterface, localSID.EndFunction_DX2.NextHop)
	} else if localSID.EndFunction_DX4 != nil {
		return fmt.Sprintf("DX4{OutgoingInterface: %v, NextHop: %v}", localSID.EndFunction_DX4.OutgoingInterface, localSID.EndFunction_DX4.NextHop)
	} else if localSID.EndFunction_DX6 != nil {
		return fmt.Sprintf("DX6{OutgoingInterface: %v, NextHop: %v}", localSID.EndFunction_DX6.OutgoingInterface, localSID.EndFunction_DX6.NextHop)
	} else if localSID.EndFunction_DT4 != nil {
		return fmt.Sprint("DT4")
	} else if localSID.EndFunction_DT6 != nil {
		return fmt.Sprint("DT6")
	}
	return "unknown end function"
}

func (h *SRv6VppHandler) writeEndFunction(req *sr.SrLocalsidAddDel, sidAddr net.IP, localSID *srv6.LocalSID, swIfIndex ifaceidx.SwIfIndex) error {
	if localSID.BaseEndFunction != nil {
		req.Behavior = BehaviorEnd
		req.EndPsp = boolToUint(localSID.BaseEndFunction.Psp)
	} else if localSID.EndFunction_X != nil {
		req.Behavior = BehaviorX
		req.EndPsp = boolToUint(localSID.EndFunction_X.Psp)
		interfaceSwIndex, _, exists := swIfIndex.LookupIdx(localSID.EndFunction_X.OutgoingInterface)
		if !exists {
			return fmt.Errorf("for interface %v doesn't exist sw index", localSID.EndFunction_X.OutgoingInterface)
		}
		req.SwIfIndex = interfaceSwIndex
		nhAddr, err := parseIPv6(localSID.EndFunction_X.NextHop) // parses also ipv4 addresses but into ipv6 address form
		if err != nil {
			return err
		}
		if nhAddr4 := nhAddr.To4(); nhAddr4 != nil { // ipv4 address in ipv6 address form?
			req.NhAddr4 = nhAddr4
		} else {
			req.NhAddr6 = []byte(nhAddr)
		}
	} else if localSID.EndFunction_T != nil {
		req.Behavior = BehaviorT
		req.EndPsp = boolToUint(localSID.EndFunction_T.Psp)
	} else if localSID.EndFunction_DX2 != nil {
		req.Behavior = BehaviorDX2
		req.VlanIndex = localSID.EndFunction_DX2.VlanTag
		interfaceSwIndex, _, exists := swIfIndex.LookupIdx(localSID.EndFunction_DX2.OutgoingInterface)
		if !exists {
			return fmt.Errorf("for interface %v doesn't exist sw index", localSID.EndFunction_DX2.OutgoingInterface)
		}
		req.SwIfIndex = interfaceSwIndex
		nhAddr, err := parseIPv6(localSID.EndFunction_DX2.NextHop) // parses also ipv4 addresses but into ipv6 address form
		if err != nil {
			return err
		}
		if nhAddr4 := nhAddr.To4(); nhAddr4 != nil { // ipv4 address in ipv6 address form?
			req.NhAddr4 = nhAddr4
		} else {
			req.NhAddr6 = []byte(nhAddr)
		}
	} else if localSID.EndFunction_DX4 != nil {
		req.Behavior = BehaviorDX4
		interfaceSwIndex, _, exists := swIfIndex.LookupIdx(localSID.EndFunction_DX4.OutgoingInterface)
		if !exists {
			return fmt.Errorf("for interface %v doesn't exist sw index", localSID.EndFunction_DX4.OutgoingInterface)
		}
		req.SwIfIndex = interfaceSwIndex
		nhAddr, err := parseIPv6(localSID.EndFunction_DX4.NextHop) // parses also IPv4
		if err != nil {
			return err
		}
		nhAddr4 := nhAddr.To4()
		if nhAddr4 == nil {
			return fmt.Errorf("next hop of DX4 end function (%v) is not valid IPv4 address", localSID.EndFunction_DX4.NextHop)
		}
		req.NhAddr4 = []byte(nhAddr4)
	} else if localSID.EndFunction_DX6 != nil {
		req.Behavior = BehaviorDX6
		interfaceSwIndex, _, exists := swIfIndex.LookupIdx(localSID.EndFunction_DX6.OutgoingInterface)
		if !exists {
			return fmt.Errorf("for interface %v doesn't exist sw index", localSID.EndFunction_DX6.OutgoingInterface)
		}
		req.SwIfIndex = interfaceSwIndex
		nhAddr6, err := parseIPv6(localSID.EndFunction_DX6.NextHop)
		if err != nil {
			return err
		}
		req.NhAddr6 = []byte(nhAddr6)
	} else if localSID.EndFunction_DT4 != nil {
		req.Behavior = BehaviorDT4
	} else if localSID.EndFunction_DT6 != nil {
		req.Behavior = BehaviorDT6
	} else {
		return fmt.Errorf("End function not set. Please configure end function for local SID %v ", sidAddr)
	}
	return nil
}

// SetEncapsSourceAddress sets for SRv6 in VPP the source address used for encapsulated packet
func (h *SRv6VppHandler) SetEncapsSourceAddress(address string) error {
	h.log.Debugf("Configuring encapsulation source address to address %v", address)
	ipAddress, err := parseIPv6(address)
	if err != nil {
		return err
	}
	req := &sr.SrSetEncapSource{
		EncapsSource: []byte(ipAddress),
	}
	reply := &sr.SrSetEncapSourceReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp call %q returned: %d", reply.GetMessageName(), reply.Retval)
	}

	h.log.WithFields(logging.Fields{"Encapsulation source address": address}).
		Debug("Encapsulation source address configured.")

	return nil
}

// AddPolicy adds SRv6 policy given by identified <bindingSid>,initial segment for policy <policySegment> and other policy settings in <policy>
func (h *SRv6VppHandler) AddPolicy(bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment) error {
	h.log.Debugf("Adding SR policy with binding SID %v and list of next SIDs %v", bindingSid, policySegment.Segments)
	sids, err := h.convertPolicySegment(policySegment)
	if err != nil {
		return err
	}
	// Note: Weight in sr.SrPolicyAdd is leftover from API changes that moved weight into sr.Srv6SidList (it is weight of sid list not of the whole policy)
	req := &sr.SrPolicyAdd{
		BsidAddr: []byte(bindingSid),
		Sids:     *sids,
		IsEncap:  boolToUint(policy.SrhEncapsulation),
		Type:     boolToUint(policy.SprayBehaviour),
		FibTable: policy.FibTableId,
	}
	reply := &sr.SrPolicyAddReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp call %q returned: %d", reply.GetMessageName(), reply.Retval)
	}

	h.log.WithFields(logging.Fields{"binding SID": bindingSid, "list of next SIDs": policySegment.Segments}).
		Debug("SR policy added")

	return nil
}

// DeletePolicy deletes SRv6 policy given by binding SID <bindingSid>
func (h *SRv6VppHandler) DeletePolicy(bindingSid net.IP) error {
	h.log.Debugf("Deleting SR policy with binding SID %v ", bindingSid)
	req := &sr.SrPolicyDel{
		BsidAddr: sr.Srv6Sid{Addr: []byte(bindingSid)}, // TODO add ability to define policy also by index (SrPolicyIndex)
	}
	reply := &sr.SrPolicyDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp call %q returned: %d", reply.GetMessageName(), reply.Retval)
	}

	h.log.WithFields(logging.Fields{"binding SID": bindingSid}).
		Debug("SR policy deleted")

	return nil
}

// AddPolicySegment adds segment <policySegment> to SRv6 policy <policy> that has policy BSID <bindingSid>
func (h *SRv6VppHandler) AddPolicySegment(bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment) error {
	h.log.Debugf("Adding segment %v to SR policy with binding SID %v", policySegment.Segments, bindingSid)
	err := h.modPolicy(AddSRList, bindingSid, policy, policySegment, 0)
	if err == nil {
		h.log.WithFields(logging.Fields{"binding SID": bindingSid, "list of next SIDs": policySegment.Segments}).
			Debug("SR policy modified(added another segment list)")
	}
	return err
}

// DeletePolicySegment removes segment <policySegment> (with segment index <segmentIndex>) from SRv6 policy <policy> that has policy BSID <bindingSid>
func (h *SRv6VppHandler) DeletePolicySegment(bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment,
	segmentIndex uint32) error {
	h.log.Debugf("Removing segment %v (index %v) from SR policy with binding SID %v", policySegment.Segments, segmentIndex, bindingSid)
	err := h.modPolicy(DeleteSRList, bindingSid, policy, policySegment, segmentIndex)
	if err == nil {
		h.log.WithFields(logging.Fields{"binding SID": bindingSid, "list of next SIDs": policySegment.Segments, "segmentIndex": segmentIndex}).
			Debug("SR policy modified(removed segment list)")
	}
	return err
}

func (h *SRv6VppHandler) modPolicy(operation uint8, bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment,
	segmentIndex uint32) error {
	sids, err := h.convertPolicySegment(policySegment)
	if err != nil {
		return err
	}
	// Note: Weight in sr.SrPolicyMod is leftover from API changes that moved weight into sr.Srv6SidList (it is weight of sid list not of the whole policy)
	req := &sr.SrPolicyMod{
		BsidAddr:  []byte(bindingSid), // TODO add ability to define policy also by index (SrPolicyIndex)
		Operation: operation,
		Sids:      *sids,
		FibTable:  policy.FibTableId,
	}
	if operation == DeleteSRList || operation == ModifyWeightOfSRList {
		req.SlIndex = segmentIndex
	}

	reply := &sr.SrPolicyModReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp call %q returned: %d", reply.GetMessageName(), reply.Retval)
	}
	return nil
}

func (h *SRv6VppHandler) convertPolicySegment(policySegment *srv6.PolicySegment) (*sr.Srv6SidList, error) {
	var segments []sr.Srv6Sid
	for _, sid := range policySegment.Segments {
		// parse to IPv6 address
		parserSid, err := parseIPv6(sid)
		if err != nil {
			return nil, err
		}

		// add sid to segment list
		ipv6Segment := sr.Srv6Sid{
			Addr: make([]byte, 16), // sr.Srv6Sid.Addr = [16]byte
		}
		copy(ipv6Segment.Addr, parserSid)
		segments = append(segments, ipv6Segment)
	}
	return &sr.Srv6SidList{
		NumSids: uint8(len(segments)),
		Sids:    segments,
		Weight:  policySegment.Weight,
	}, nil
}

// AddSteering sets in VPP steering into SRv6 policy.
func (h *SRv6VppHandler) AddSteering(steering *srv6.Steering, swIfIndex ifaceidx.SwIfIndex) error {
	return h.addDelSteering(false, steering, swIfIndex)
}

// RemoveSteering removes in VPP steering into SRv6 policy.
func (h *SRv6VppHandler) RemoveSteering(steering *srv6.Steering, swIfIndex ifaceidx.SwIfIndex) error {
	return h.addDelSteering(true, steering, swIfIndex)
}

func (h *SRv6VppHandler) addDelSteering(delete bool, steering *srv6.Steering, swIfIndex ifaceidx.SwIfIndex) error {
	// defining operation strings for logging
	operationProgressing, operationFinished := "Adding", "Added"
	if delete {
		operationProgressing, operationFinished = "Removing", "Removed"
	}

	// logging info about operation with steering
	if steering.L3Traffic != nil {
		h.log.Debugf("%v steering for l3 traffic with destination %v to SR policy (binding SID %v, policy index %v)",
			operationProgressing, steering.L3Traffic.PrefixAddress, steering.PolicyBsid, steering.PolicyIndex)
	} else {
		h.log.Debugf("%v steering for l2 traffic from interface %v to SR policy (binding SID %v, policy index %v)",
			operationProgressing, steering.L2Traffic.InterfaceName, steering.PolicyBsid, steering.PolicyIndex)
	}

	// converting policy reference
	var bsidAddr []byte
	if len(strings.Trim(steering.PolicyBsid, " ")) > 0 {
		bsid, err := parseIPv6(steering.PolicyBsid)
		if err != nil {
			return fmt.Errorf("can't parse binding SID %q to IP address: %v ", steering.PolicyBsid, err)
		}
		bsidAddr = []byte(bsid)
	}

	// converting target traffic info
	var prefixAddr []byte
	steerType := SteerTypeIPv6
	tableID := uint32(0)
	maskWidth := uint32(0)
	intIndex := uint32(0)
	if steering.L3Traffic != nil {
		ip, ipnet, err := net.ParseCIDR(steering.L3Traffic.PrefixAddress)
		if err != nil {
			return fmt.Errorf("can't parse ip prefix %q: %v", steering.L3Traffic.PrefixAddress, err)
		}
		if ip.To4() != nil { // IPv4 address
			steerType = SteerTypeIPv4
		}
		tableID = steering.L3Traffic.FibTableId
		prefixAddr = []byte(ip.To16())
		ms, _ := ipnet.Mask.Size()
		maskWidth = uint32(ms)
	} else if steering.L2Traffic != nil {
		steerType = SteerTypeL2
		interfaceSwIndex, _, exists := swIfIndex.LookupIdx(steering.L2Traffic.InterfaceName)
		if !exists {
			return fmt.Errorf("for interface %v doesn't exist sw index", steering.L2Traffic.InterfaceName)
		}
		intIndex = interfaceSwIndex
	}
	req := &sr.SrSteeringAddDel{
		IsDel:         boolToUint(delete),
		TableID:       tableID,
		BsidAddr:      bsidAddr,             // policy (to which we want to steer routing into) identified by binding sid (alternativelly it can be used SRPolicyIndex)
		SrPolicyIndex: steering.PolicyIndex, // policy (to which we want to steer routing into) identified by policy index (alternativelly it can be used BsidAddr)
		TrafficType:   steerType,            // type of traffic to steer
		PrefixAddr:    prefixAddr,           // destination prefix address (L3 traffic type only)
		MaskWidth:     maskWidth,            // destination ip prefix mask (L3 traffic type only)
		SwIfIndex:     intIndex,             // incoming interface (L2 traffic type only)
	}
	reply := &sr.SrSteeringAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp call %q returned: %d", reply.GetMessageName(), reply.Retval)
	}

	h.log.WithFields(logging.Fields{"steer type": steerType, "L3 prefix address bytes": prefixAddr,
		"L2 interface index": intIndex, "policy binding SID": bsidAddr, "policy index": steering.PolicyIndex}).
		Debugf("%v steering to SR policy ", operationFinished)

	return nil
}

func boolToUint(input bool) uint8 {
	if input {
		return uint8(1)
	}
	return uint8(0)
}

// parseIPv6 parses string <str> to IPv6 address (including IPv4 address converted to IPv6 address)
func parseIPv6(str string) (net.IP, error) {
	ip := net.ParseIP(str)
	if ip == nil {
		return nil, fmt.Errorf(" %q is not ip address", str)
	}
	ipv6 := ip.To16()
	if ipv6 == nil {
		return nil, fmt.Errorf(" %q is not ipv6 address", str)
	}
	return ipv6, nil
}
