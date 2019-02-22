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
	"bytes"
	"fmt"
	"net"

	"github.com/ligato/cn-infra/logging/logrus"

	acl "github.com/ligato/vpp-agent/api/models/vpp/acl"
	acl_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
)

// Protocol types that can occur in ACLs
const (
	ICMPv4Proto = 1
	TCPProto    = 6
	UDPProto    = 17
	ICMPv6Proto = 58
)

// ACLDetails is combination of proto-modelled ACL data and VPP provided metadata
type ACLDetails struct {
	ACL  *acl.ACL `json:"acl"`
	Meta *ACLMeta `json:"acl_meta"`
}

// ACLMeta holds VPP-specific metadata
type ACLMeta struct {
	Index uint32 `json:"acl_index"`
	Tag   string `json:"acl_tag"`
}

// ACLToInterface is definition of interface and all ACLs which are bound to
// the interface either as ingress or egress
type ACLToInterface struct {
	SwIfIdx    uint32
	IngressACL []uint32
	EgressACL  []uint32
}

// DumpACL implements ACL handler.
func (h *ACLVppHandler) DumpACL() ([]*ACLDetails, error) {
	ruleIPData := make(map[ACLMeta][]*acl.ACL_Rule)

	// get all ACLs with IP ruleData
	IPRuleACLs, err := h.DumpIPAcls()
	if len(IPRuleACLs) < 1 || err != nil {
		return nil, err
	}

	// resolve IP rules for every ACL
	// Note: currently ACL may have only IP ruleData or only MAC IP ruleData
	var wasErr error
	for identifier, IPRules := range IPRuleACLs {
		var rulesDetails []*acl.ACL_Rule

		if len(IPRules) > 0 {
			for _, IPRule := range IPRules {
				ruleDetails, err := h.getIPRuleDetails(IPRule)
				if err != nil {
					return nil, fmt.Errorf("failed to get IP Rule %v details: %v", IPRule, err)
				}
				rulesDetails = append(rulesDetails, ruleDetails)
			}
		}
		ruleIPData[identifier] = rulesDetails
	}

	// Prepare separate list of all active ACL indices on the VPP
	var indices []uint32
	for identifier := range ruleIPData {
		indices = append(indices, identifier.Index)
	}

	// Get all ACL indices with ingress and egress interfaces
	interfaceData, err := h.DumpACLInterfaces(indices)
	if err != nil {
		return nil, err
	}

	var ACLs []*ACLDetails
	// Build a list of ACL ruleData with ruleData, interfaces, index and tag (name)
	for identifier, rules := range ruleIPData {
		ACLs = append(ACLs, &ACLDetails{
			ACL: &acl.ACL{
				Name:       identifier.Tag,
				Rules:      rules,
				Interfaces: interfaceData[identifier.Index],
			},
			Meta: &ACLMeta{
				Index: identifier.Index,
				Tag:   identifier.Tag,
			},
		})
	}

	return ACLs, wasErr
}

// DumpMACIPACL implements ACL handler.
func (h *ACLVppHandler) DumpMACIPACL() ([]*ACLDetails, error) {
	ruleMACIPData := make(map[ACLMeta][]*acl.ACL_Rule)

	// get all ACLs with MACIP ruleData
	MACIPRuleACLs, err := h.DumpMacIPAcls()
	if len(MACIPRuleACLs) < 1 || err != nil {
		return nil, err
	}

	// resolve MACIP rules for every ACL
	for metadata, MACIPRules := range MACIPRuleACLs {
		var rulesDetails []*acl.ACL_Rule

		for _, MACIPRule := range MACIPRules {
			ruleDetails, err := h.getMACIPRuleDetails(MACIPRule)
			if err != nil {
				return nil, fmt.Errorf("failed to get MACIP Rule %v details: %v", MACIPRule, err)
			}
			rulesDetails = append(rulesDetails, ruleDetails)
		}
		ruleMACIPData[metadata] = rulesDetails
	}

	// Prepare separate list of all active ACL indices on the VPP
	var indices []uint32
	for identifier := range ruleMACIPData {
		indices = append(indices, identifier.Index)
	}

	// Get all ACL indices with ingress and egress interfaces
	interfaceData, err := h.DumpMACIPACLInterfaces(indices)
	if err != nil {
		return nil, err
	}

	var ACLs []*ACLDetails
	// Build a list of ACL ruleData with ruleData, interfaces, index and tag (name)
	for metadata, rules := range ruleMACIPData {
		ACLs = append(ACLs, &ACLDetails{
			ACL: &acl.ACL{
				Name:       metadata.Tag,
				Rules:      rules,
				Interfaces: interfaceData[metadata.Index],
			},
			Meta: &ACLMeta{
				Index: metadata.Index,
				Tag:   metadata.Tag,
			},
		})
	}
	return ACLs, nil
}

// DumpACLInterfaces implements ACL handler.
func (h *ACLVppHandler) DumpACLInterfaces(indices []uint32) (map[uint32]*acl.ACL_Interfaces, error) {
	// list of ACL-to-interfaces
	aclsWithInterfaces := make(map[uint32]*acl.ACL_Interfaces)

	var interfaceData []*ACLToInterface
	var wasErr error

	msgIP := &acl_api.ACLInterfaceListDump{
		SwIfIndex: 0xffffffff, // dump all
	}
	reqIP := h.dumpChannel.SendMultiRequest(msgIP)
	for {
		replyIP := &acl_api.ACLInterfaceListDetails{}
		stop, err := reqIP.ReceiveReply(replyIP)
		if stop {
			break
		}
		if err != nil {
			return aclsWithInterfaces, fmt.Errorf("ACL interface list dump reply error: %v", err)
		}

		if replyIP.Count > 0 {
			data := &ACLToInterface{
				SwIfIdx: replyIP.SwIfIndex,
			}
			for i, aclIdx := range replyIP.Acls {
				if i < int(replyIP.NInput) {
					data.IngressACL = append(data.IngressACL, aclIdx)
				} else {
					data.EgressACL = append(data.EgressACL, aclIdx)
				}
			}
			interfaceData = append(interfaceData, data)
		}
	}

	// sort interfaces for every ACL
	for _, aclIdx := range indices {
		var ingress []string
		var egress []string
		for _, data := range interfaceData {
			// look for ingress
			for _, ingressACLIdx := range data.IngressACL {
				if ingressACLIdx == aclIdx {
					name, _, found := h.ifIndexes.LookupBySwIfIndex(data.SwIfIdx)
					if !found {
						continue
					}
					ingress = append(ingress, name)
				}
			}
			// look for egress
			for _, egressACLIdx := range data.EgressACL {
				if egressACLIdx == aclIdx {
					name, _, found := h.ifIndexes.LookupBySwIfIndex(data.SwIfIdx)
					if !found {
						continue
					}
					egress = append(egress, name)
				}
			}
		}

		aclsWithInterfaces[aclIdx] = &acl.ACL_Interfaces{
			Egress:  egress,
			Ingress: ingress,
		}
	}

	return aclsWithInterfaces, wasErr
}

// DumpMACIPACLInterfaces implements ACL handler.
func (h *ACLVppHandler) DumpMACIPACLInterfaces(indices []uint32) (map[uint32]*acl.ACL_Interfaces, error) {
	// list of ACL-to-interfaces
	aclsWithInterfaces := make(map[uint32]*acl.ACL_Interfaces)

	var interfaceData []*ACLToInterface

	msgMACIP := &acl_api.MacipACLInterfaceListDump{
		SwIfIndex: 0xffffffff, // dump all
	}
	reqMACIP := h.dumpChannel.SendMultiRequest(msgMACIP)
	for {
		replyMACIP := &acl_api.MacipACLInterfaceListDetails{}
		stop, err := reqMACIP.ReceiveReply(replyMACIP)
		if stop {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("MACIP ACL interface list dump reply error: %v", err)
		}
		if replyMACIP.Count > 0 {
			data := &ACLToInterface{
				SwIfIdx: replyMACIP.SwIfIndex,
			}
			for _, aclIdx := range replyMACIP.Acls {
				data.IngressACL = append(data.IngressACL, aclIdx)
			}
			interfaceData = append(interfaceData, data)
		}
	}

	for _, aclIdx := range indices {
		var ingress []string
		for _, data := range interfaceData {
			// look for ingress
			for _, ingressACLIdx := range data.IngressACL {
				if ingressACLIdx == aclIdx {
					name, _, found := h.ifIndexes.LookupBySwIfIndex(data.SwIfIdx)
					if !found {
						continue
					}
					ingress = append(ingress, name)
				}
			}
		}
		var ifaces *acl.ACL_Interfaces
		if len(ingress) > 0 {
			ifaces = &acl.ACL_Interfaces{
				Egress:  nil,
				Ingress: ingress,
			}
		}
		aclsWithInterfaces[aclIdx] = ifaces
	}

	return aclsWithInterfaces, nil
}

// DumpIPAcls implements ACL handler.
func (h *ACLVppHandler) DumpIPAcls() (map[ACLMeta][]acl_api.ACLRule, error) {
	aclIPRules := make(map[ACLMeta][]acl_api.ACLRule)
	var wasErr error

	req := &acl_api.ACLDump{
		ACLIndex: 0xffffffff,
	}
	reqContext := h.dumpChannel.SendMultiRequest(req)
	for {
		msg := &acl_api.ACLDetails{}
		stop, err := reqContext.ReceiveReply(msg)
		if err != nil {
			return aclIPRules, fmt.Errorf("ACL dump reply error: %v", err)
		}
		if stop {
			break
		}

		metadata := ACLMeta{
			Index: msg.ACLIndex,
			Tag:   string(bytes.SplitN(msg.Tag, []byte{0x00}, 2)[0]),
		}

		aclIPRules[metadata] = msg.R
	}

	return aclIPRules, wasErr
}

// DumpMacIPAcls implements ACL handler.
func (h *ACLVppHandler) DumpMacIPAcls() (map[ACLMeta][]acl_api.MacipACLRule, error) {
	aclMACIPRules := make(map[ACLMeta][]acl_api.MacipACLRule)

	req := &acl_api.MacipACLDump{
		ACLIndex: 0xffffffff,
	}
	reqContext := h.dumpChannel.SendMultiRequest(req)
	for {
		msg := &acl_api.MacipACLDetails{}
		stop, err := reqContext.ReceiveReply(msg)
		if err != nil {
			return nil, fmt.Errorf("ACL MACIP dump reply error: %v", err)
		}
		if stop {
			break
		}

		metadata := ACLMeta{
			Index: msg.ACLIndex,
			Tag:   string(bytes.Trim(msg.Tag, "\x00")),
		}

		aclMACIPRules[metadata] = msg.R
	}
	return aclMACIPRules, nil
}

// DumpInterfaceACLs implements ACL handler.
func (h *ACLVppHandler) DumpInterfaceACLs(swIndex uint32) (acls []*acl.ACL, err error) {
	res, err := h.DumpInterfaceACLList(swIndex)
	if err != nil {
		return nil, err
	}

	if res.SwIfIndex != swIndex {
		return nil, fmt.Errorf("returned interface index %d does not match request", res.SwIfIndex)
	}

	for aidx := range res.Acls {
		ipACL, err := h.getIPACLDetails(uint32(aidx))
		if err != nil {
			return nil, err
		}
		acls = append(acls, ipACL)
	}
	return acls, nil
}

// DumpInterfaceMACIPACLs implements ACL handler.
func (h *ACLVppHandler) DumpInterfaceMACIPACLs(swIndex uint32) (acls []*acl.ACL, err error) {
	resMacIP, err := h.DumpInterfaceMACIPACLList(swIndex)
	if err != nil {
		return nil, err
	}

	if resMacIP.SwIfIndex != swIndex {
		return nil, fmt.Errorf("returned interface index %d does not match request", resMacIP.SwIfIndex)
	}

	for aidx := range resMacIP.Acls {
		macipACL, err := h.getMACIPACLDetails(uint32(aidx))
		if err != nil {
			return nil, err
		}
		acls = append(acls, macipACL)
	}
	return acls, nil
}

// DumpInterfaceACLList implements ACL handler.
func (h *ACLVppHandler) DumpInterfaceACLList(swIndex uint32) (*acl_api.ACLInterfaceListDetails, error) {
	req := &acl_api.ACLInterfaceListDump{
		SwIfIndex: swIndex,
	}
	reply := &acl_api.ACLInterfaceListDetails{}

	if err := h.dumpChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// DumpInterfaceMACIPACLList implements ACL handler.
func (h *ACLVppHandler) DumpInterfaceMACIPACLList(swIndex uint32) (*acl_api.MacipACLInterfaceListDetails, error) {
	req := &acl_api.MacipACLInterfaceListDump{
		SwIfIndex: swIndex,
	}
	reply := &acl_api.MacipACLInterfaceListDetails{}

	if err := h.dumpChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// DumpInterfacesLists implements ACL handler.
func (h *ACLVppHandler) DumpInterfacesLists() ([]*acl_api.ACLInterfaceListDetails, []*acl_api.MacipACLInterfaceListDetails, error) {
	msgIPACL := &acl_api.ACLInterfaceListDump{
		SwIfIndex: 0xffffffff, // dump all
	}

	reqIPACL := h.dumpChannel.SendMultiRequest(msgIPACL)

	var IPaclInterfaces []*acl_api.ACLInterfaceListDetails
	for {
		reply := &acl_api.ACLInterfaceListDetails{}
		stop, err := reqIPACL.ReceiveReply(reply)
		if stop {
			break
		}
		if err != nil {
			logrus.DefaultLogger().Error(err)
			return nil, nil, err
		}
		IPaclInterfaces = append(IPaclInterfaces, reply)
	}

	msgMACIPACL := &acl_api.ACLInterfaceListDump{
		SwIfIndex: 0xffffffff, // dump all
	}

	reqMACIPACL := h.dumpChannel.SendMultiRequest(msgMACIPACL)

	var MACIPaclInterfaces []*acl_api.MacipACLInterfaceListDetails
	for {
		reply := &acl_api.MacipACLInterfaceListDetails{}
		stop, err := reqMACIPACL.ReceiveReply(reply)
		if stop {
			break
		}
		if err != nil {
			logrus.DefaultLogger().Error(err)
			return nil, nil, err
		}
		MACIPaclInterfaces = append(MACIPaclInterfaces, reply)
	}

	return IPaclInterfaces, MACIPaclInterfaces, nil
}

func (h *ACLVppHandler) getIPRuleDetails(rule acl_api.ACLRule) (*acl.ACL_Rule, error) {
	// Resolve rule actions
	aclAction, err := h.resolveRuleAction(rule.IsPermit)
	if err != nil {
		return nil, err
	}

	return &acl.ACL_Rule{
		Action: aclAction,
		IpRule: h.getIPRuleMatches(rule),
	}, nil
}

// getIPACLDetails gets details for a given IP ACL from VPP and translates
// them from the binary VPP API format into the ACL Plugin's NB format.
func (h *ACLVppHandler) getIPACLDetails(idx uint32) (aclRule *acl.ACL, err error) {
	req := &acl_api.ACLDump{
		ACLIndex: uint32(idx),
	}

	reply := &acl_api.ACLDetails{}
	if err := h.dumpChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return nil, err
	}

	var ruleData []*acl.ACL_Rule
	for _, r := range reply.R {
		rule := &acl.ACL_Rule{}

		ipRule, err := h.getIPRuleDetails(r)
		if err != nil {
			return nil, err
		}

		aclAction, err := h.resolveRuleAction(r.IsPermit)
		if err != nil {
			return nil, err
		}

		rule.IpRule = ipRule.GetIpRule()
		rule.Action = aclAction
		ruleData = append(ruleData, rule)
	}

	return &acl.ACL{Rules: ruleData, Name: string(bytes.SplitN(reply.Tag, []byte{0x00}, 2)[0])}, nil
}

func (h *ACLVppHandler) getMACIPRuleDetails(rule acl_api.MacipACLRule) (*acl.ACL_Rule, error) {
	// Resolve rule actions
	aclAction, err := h.resolveRuleAction(rule.IsPermit)
	if err != nil {
		return nil, err
	}

	return &acl.ACL_Rule{
		Action:    aclAction,
		MacipRule: h.getMACIPRuleMatches(rule),
	}, nil
}

// getMACIPACLDetails gets details for a given MACIP ACL from VPP and translates
// them from the binary VPP API format into the ACL Plugin's NB format.
func (h *ACLVppHandler) getMACIPACLDetails(idx uint32) (aclRule *acl.ACL, err error) {
	req := &acl_api.MacipACLDump{
		ACLIndex: uint32(idx),
	}

	reply := &acl_api.MacipACLDetails{}
	if err := h.dumpChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return nil, err
	}

	var ruleData []*acl.ACL_Rule
	for _, r := range reply.R {
		rule := &acl.ACL_Rule{}

		ipRule, err := h.getMACIPRuleDetails(r)
		if err != nil {
			return nil, err
		}

		aclAction, err := h.resolveRuleAction(r.IsPermit)
		if err != nil {
			return nil, err
		}

		rule.IpRule = ipRule.GetIpRule()
		rule.Action = aclAction
		ruleData = append(ruleData, rule)
	}

	return &acl.ACL{Rules: ruleData, Name: string(bytes.SplitN(reply.Tag, []byte{0x00}, 2)[0])}, nil
}

// getIPRuleMatches translates an IP rule from the binary VPP API format into the
// ACL Plugin's NB format
func (h *ACLVppHandler) getIPRuleMatches(r acl_api.ACLRule) *acl.ACL_Rule_IpRule {
	var srcIP, dstIP string
	if r.IsIPv6 == 1 {
		srcIP = net.IP(r.SrcIPAddr).To16().String()
		dstIP = net.IP(r.DstIPAddr).To16().String()
	} else {
		srcIP = net.IP(r.SrcIPAddr[:4]).To4().String()
		dstIP = net.IP(r.DstIPAddr[:4]).To4().String()
	}

	ipRule := &acl.ACL_Rule_IpRule{
		Ip: &acl.ACL_Rule_IpRule_Ip{
			SourceNetwork:      fmt.Sprintf("%s/%d", srcIP, r.SrcIPPrefixLen),
			DestinationNetwork: fmt.Sprintf("%s/%d", dstIP, r.DstIPPrefixLen),
		},
	}

	switch r.Proto {
	case TCPProto:
		ipRule.Tcp = h.getTCPMatchRule(r)
	case UDPProto:
		ipRule.Udp = h.getUDPMatchRule(r)
	case ICMPv4Proto, ICMPv6Proto:
		ipRule.Icmp = h.getIcmpMatchRule(r)
	}
	return ipRule
}

// getMACIPRuleMatches translates an MACIP rule from the binary VPP API format into the
// ACL Plugin's NB format
func (h *ACLVppHandler) getMACIPRuleMatches(rule acl_api.MacipACLRule) *acl.ACL_Rule_MacIpRule {
	var srcAddr string
	if rule.IsIPv6 == 1 {
		srcAddr = net.IP(rule.SrcIPAddr).To16().String()
	} else {
		srcAddr = net.IP(rule.SrcIPAddr[:4]).To4().String()
	}
	srcMacAddr := net.HardwareAddr(rule.SrcMac)
	srcMacAddrMask := net.HardwareAddr(rule.SrcMacMask)
	return &acl.ACL_Rule_MacIpRule{
		SourceAddress:        srcAddr,
		SourceAddressPrefix:  uint32(rule.SrcIPPrefixLen),
		SourceMacAddress:     srcMacAddr.String(),
		SourceMacAddressMask: srcMacAddrMask.String(),
	}
}

// getTCPMatchRule translates a TCP match rule from the binary VPP API format
// into the ACL Plugin's NB format
func (h *ACLVppHandler) getTCPMatchRule(r acl_api.ACLRule) *acl.ACL_Rule_IpRule_Tcp {
	dstPortRange := &acl.ACL_Rule_IpRule_PortRange{
		LowerPort: uint32(r.DstportOrIcmpcodeFirst),
		UpperPort: uint32(r.DstportOrIcmpcodeLast),
	}
	srcPortRange := &acl.ACL_Rule_IpRule_PortRange{
		LowerPort: uint32(r.SrcportOrIcmptypeFirst),
		UpperPort: uint32(r.SrcportOrIcmptypeLast),
	}
	tcp := acl.ACL_Rule_IpRule_Tcp{
		DestinationPortRange: dstPortRange,
		SourcePortRange:      srcPortRange,
		TcpFlagsMask:         uint32(r.TCPFlagsMask),
		TcpFlagsValue:        uint32(r.TCPFlagsValue),
	}
	return &tcp
}

// getUDPMatchRule translates a UDP match rule from the binary VPP API format
// into the ACL Plugin's NB format
func (h *ACLVppHandler) getUDPMatchRule(r acl_api.ACLRule) *acl.ACL_Rule_IpRule_Udp {
	dstPortRange := &acl.ACL_Rule_IpRule_PortRange{
		LowerPort: uint32(r.DstportOrIcmpcodeFirst),
		UpperPort: uint32(r.DstportOrIcmpcodeLast),
	}
	srcPortRange := &acl.ACL_Rule_IpRule_PortRange{
		LowerPort: uint32(r.SrcportOrIcmptypeFirst),
		UpperPort: uint32(r.SrcportOrIcmptypeLast),
	}
	udp := acl.ACL_Rule_IpRule_Udp{
		DestinationPortRange: dstPortRange,
		SourcePortRange:      srcPortRange,
	}
	return &udp
}

// getIcmpMatchRule translates an ICMP match rule from the binary VPP API
// format into the ACL Plugin's NB format
func (h *ACLVppHandler) getIcmpMatchRule(r acl_api.ACLRule) *acl.ACL_Rule_IpRule_Icmp {
	icmp := &acl.ACL_Rule_IpRule_Icmp{
		Icmpv6:        r.IsIPv6 > 0,
		IcmpCodeRange: &acl.ACL_Rule_IpRule_Icmp_Range{},
		IcmpTypeRange: &acl.ACL_Rule_IpRule_Icmp_Range{},
	}
	return icmp
}

// Returns rule action representation in model according to the vpp input
func (h *ACLVppHandler) resolveRuleAction(isPermit uint8) (acl.ACL_Rule_Action, error) {
	switch isPermit {
	case 0:
		return acl.ACL_Rule_DENY, nil
	case 1:
		return acl.ACL_Rule_PERMIT, nil
	case 2:
		return acl.ACL_Rule_REFLECT, nil
	default:
		return acl.ACL_Rule_DENY, fmt.Errorf("invalid match rule %d", isPermit)
	}
}
