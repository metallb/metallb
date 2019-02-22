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
	"strings"

	acl "github.com/ligato/vpp-agent/api/models/vpp/acl"
	aclapi "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
)

// AddACL implements ACL handler.
func (h *ACLVppHandler) AddACL(rules []*acl.ACL_Rule, aclName string) (uint32, error) {
	// Prepare Ip rules
	aclIPRules, err := transformACLIpRules(rules)
	if err != nil {
		return 0, err
	}
	if len(aclIPRules) == 0 {
		return 0, fmt.Errorf("no rules found for ACL %v", aclName)
	}

	req := &aclapi.ACLAddReplace{
		ACLIndex: 0xffffffff, // to make new Entry
		Count:    uint32(len(aclIPRules)),
		Tag:      []byte(aclName),
		R:        aclIPRules,
	}
	reply := &aclapi.ACLAddReplaceReply{}

	if err = h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, fmt.Errorf("failed to write ACL %v: %v", aclName, err)
	}

	return reply.ACLIndex, nil
}

// AddMACIPACL implements ACL handler.
func (h *ACLVppHandler) AddMACIPACL(rules []*acl.ACL_Rule, aclName string) (uint32, error) {
	// Prepare MAc Ip rules
	aclMacIPRules, err := h.transformACLMacIPRules(rules)
	if err != nil {
		return 0, err
	}
	if len(aclMacIPRules) == 0 {
		return 0, fmt.Errorf("no rules found for ACL %v", aclName)
	}

	req := &aclapi.MacipACLAdd{
		Count: uint32(len(aclMacIPRules)),
		Tag:   []byte(aclName),
		R:     aclMacIPRules,
	}
	reply := &aclapi.MacipACLAddReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, fmt.Errorf("failed to write ACL %v: %v", aclName, err)
	}

	return reply.ACLIndex, nil
}

// ModifyACL implements ACL handler.
func (h *ACLVppHandler) ModifyACL(aclIndex uint32, rules []*acl.ACL_Rule, aclName string) error {
	// Prepare Ip rules
	aclIPRules, err := transformACLIpRules(rules)
	if err != nil {
		return err
	}
	if len(aclIPRules) == 0 {
		return nil
	}

	req := &aclapi.ACLAddReplace{
		ACLIndex: aclIndex,
		Count:    uint32(len(aclIPRules)),
		Tag:      []byte(aclName),
		R:        aclIPRules,
	}
	reply := &aclapi.ACLAddReplaceReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("failed to write ACL %v: %v", aclName, err)
	}

	return nil
}

// ModifyMACIPACL implements ACL handler.
func (h *ACLVppHandler) ModifyMACIPACL(aclIndex uint32, rules []*acl.ACL_Rule, aclName string) error {
	// Prepare MAc Ip rules
	aclMacIPRules, err := h.transformACLMacIPRules(rules)
	if err != nil {
		return err
	}
	if len(aclMacIPRules) == 0 {
		return fmt.Errorf("no rules found for ACL %v", aclName)
	}

	req := &aclapi.MacipACLAddReplace{
		ACLIndex: aclIndex,
		Count:    uint32(len(aclMacIPRules)),
		Tag:      []byte(aclName),
		R:        aclMacIPRules,
	}
	reply := &aclapi.MacipACLAddReplaceReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("failed to write ACL %v: %v", aclName, err)
	}

	return nil
}

// DeleteACL implements ACL handler.
func (h *ACLVppHandler) DeleteACL(aclIndex uint32) error {
	req := &aclapi.ACLDel{
		ACLIndex: aclIndex,
	}
	reply := &aclapi.ACLDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("failed to remove L3/L4 ACL %v: %v", aclIndex, err)
	}

	return nil
}

// DeleteMACIPACL implements ACL handler.
func (h *ACLVppHandler) DeleteMACIPACL(aclIndex uint32) error {
	req := &aclapi.MacipACLDel{
		ACLIndex: aclIndex,
	}
	reply := &aclapi.MacipACLDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("failed to remove L2 ACL %v: %v", aclIndex, err)
	}

	return nil
}

// Method transforms provided set of IP proto ACL rules to binapi ACL rules.
func transformACLIpRules(rules []*acl.ACL_Rule) (aclIPRules []aclapi.ACLRule, err error) {
	for _, rule := range rules {
		aclRule := &aclapi.ACLRule{
			IsPermit: uint8(rule.Action),
		}
		// Match
		if ipRule := rule.GetIpRule(); ipRule != nil {
			// Concerned to IP rules only
			// L3
			if ipRule.Ip != nil {
				aclRule, err = ipACL(ipRule.Ip, aclRule)
				if err != nil {
					return nil, err
				}
			}
			// ICMP/L4
			if ipRule.Icmp != nil {
				aclRule = icmpACL(ipRule.Icmp, aclRule)
			} else if ipRule.Tcp != nil {
				aclRule = tcpACL(ipRule.Tcp, aclRule)
			} else if ipRule.Udp != nil {
				aclRule = udpACL(ipRule.Udp, aclRule)
			}
			aclIPRules = append(aclIPRules, *aclRule)
		}
	}
	return aclIPRules, nil
}

func (h *ACLVppHandler) transformACLMacIPRules(rules []*acl.ACL_Rule) (aclMacIPRules []aclapi.MacipACLRule, err error) {
	for _, rule := range rules {
		aclMacIPRule := &aclapi.MacipACLRule{
			IsPermit: uint8(rule.Action),
		}
		// Matche
		if macIPRule := rule.GetMacipRule(); macIPRule != nil {
			// Concerned to MAC IP rules only
			// Source IP Address + Prefix
			srcIPAddress := net.ParseIP(macIPRule.SourceAddress)
			if srcIPAddress.To4() != nil {
				aclMacIPRule.IsIPv6 = 0
				aclMacIPRule.SrcIPAddr = srcIPAddress.To4()
				aclMacIPRule.SrcIPPrefixLen = uint8(macIPRule.SourceAddressPrefix)
			} else if srcIPAddress.To16() != nil {
				aclMacIPRule.IsIPv6 = 1
				aclMacIPRule.SrcIPAddr = srcIPAddress.To16()
				aclMacIPRule.SrcIPPrefixLen = uint8(macIPRule.SourceAddressPrefix)
			} else {
				return nil, fmt.Errorf("invalid IP address %v", macIPRule.SourceAddress)
			}
			// MAC + mask
			srcMac, err := net.ParseMAC(macIPRule.SourceMacAddress)
			if err != nil {
				return aclMacIPRules, err
			}
			srcMacMask, err := net.ParseMAC(macIPRule.SourceMacAddressMask)
			if err != nil {
				return aclMacIPRules, err
			}
			aclMacIPRule.SrcMac = srcMac
			aclMacIPRule.SrcMacMask = srcMacMask
			aclMacIPRules = append(aclMacIPRules, *aclMacIPRule)
		}
	}
	return aclMacIPRules, nil
}

// The function sets an IP ACL rule fields into provided ACL Rule object. Source
// and destination addresses have to be the same IP version and contain a network mask.
func ipACL(ipRule *acl.ACL_Rule_IpRule_Ip, aclRule *aclapi.ACLRule) (*aclapi.ACLRule, error) {
	var (
		err        error
		srcIP      net.IP
		srcNetwork *net.IPNet
		dstIP      net.IP
		dstNetwork *net.IPNet
		srcMask    uint8
		dstMask    uint8
	)

	if strings.TrimSpace(ipRule.SourceNetwork) != "" {
		// Resolve source address
		srcIP, srcNetwork, err = net.ParseCIDR(ipRule.SourceNetwork)
		if err != nil {
			return nil, err
		}
		if srcIP.To4() == nil && srcIP.To16() == nil {
			return aclRule, fmt.Errorf("source address %v is invalid", ipRule.SourceNetwork)
		}
		maskSize, _ := srcNetwork.Mask.Size()
		srcMask = uint8(maskSize)
	}

	if strings.TrimSpace(ipRule.DestinationNetwork) != "" {
		// Resolve destination address
		dstIP, dstNetwork, err = net.ParseCIDR(ipRule.DestinationNetwork)
		if err != nil {
			return nil, err
		}
		if dstIP.To4() == nil && dstIP.To16() == nil {
			return aclRule, fmt.Errorf("destination address %v is invalid", ipRule.DestinationNetwork)
		}
		maskSize, _ := dstNetwork.Mask.Size()
		dstMask = uint8(maskSize)
	}

	// Check IP version (they should be the same), beware: IPv4 address can be converted to IPv6.
	if (srcIP.To4() != nil && dstIP.To4() == nil && dstIP.To16() != nil) ||
		(srcIP.To4() == nil && srcIP.To16() != nil && dstIP.To4() != nil) {
		return aclRule, fmt.Errorf("source address %v and destionation address %v have different IP versions",
			ipRule.SourceNetwork, ipRule.DestinationNetwork)
	}

	if srcIP.To4() != nil || dstIP.To4() != nil {
		// Ipv4 case
		aclRule.IsIPv6 = 0
		aclRule.SrcIPAddr = srcIP.To4()
		aclRule.SrcIPPrefixLen = srcMask
		aclRule.DstIPAddr = dstIP.To4()
		aclRule.DstIPPrefixLen = dstMask
	} else if srcIP.To16() != nil || dstIP.To16() != nil {
		// Ipv6 case
		aclRule.IsIPv6 = 1
		aclRule.SrcIPAddr = srcIP.To16()
		aclRule.SrcIPPrefixLen = srcMask
		aclRule.DstIPAddr = dstIP.To16()
		aclRule.DstIPPrefixLen = dstMask
	} else {
		// Both empty
		aclRule.IsIPv6 = 0
	}
	return aclRule, nil
}

// The function sets an ICMP ACL rule fields into provided ACL Rule object.
// The ranges are exclusive, use first = 0 and last = 255/65535 (icmpv4/icmpv6) to match "any".
func icmpACL(icmpRule *acl.ACL_Rule_IpRule_Icmp, aclRule *aclapi.ACLRule) *aclapi.ACLRule {
	if icmpRule == nil {
		return aclRule
	}
	if icmpRule.Icmpv6 {
		aclRule.Proto = ICMPv6Proto // IANA ICMPv6
		aclRule.IsIPv6 = 1
		// ICMPv6 type range
		aclRule.SrcportOrIcmptypeFirst = uint16(icmpRule.IcmpTypeRange.First)
		aclRule.SrcportOrIcmptypeLast = uint16(icmpRule.IcmpTypeRange.Last)
		// ICMPv6 code range
		aclRule.DstportOrIcmpcodeFirst = uint16(icmpRule.IcmpCodeRange.First)
		aclRule.DstportOrIcmpcodeLast = uint16(icmpRule.IcmpCodeRange.First)
	} else {
		aclRule.Proto = ICMPv4Proto // IANA ICMPv4
		aclRule.IsIPv6 = 0
		// ICMPv4 type range
		aclRule.SrcportOrIcmptypeFirst = uint16(icmpRule.IcmpTypeRange.First)
		aclRule.SrcportOrIcmptypeLast = uint16(icmpRule.IcmpTypeRange.Last)
		// ICMPv4 code range
		aclRule.DstportOrIcmpcodeFirst = uint16(icmpRule.IcmpCodeRange.First)
		aclRule.DstportOrIcmpcodeLast = uint16(icmpRule.IcmpCodeRange.Last)
	}
	return aclRule
}

// Sets an TCP ACL rule fields into provided ACL Rule object.
func tcpACL(tcpRule *acl.ACL_Rule_IpRule_Tcp, aclRule *aclapi.ACLRule) *aclapi.ACLRule {
	aclRule.Proto = TCPProto // IANA TCP
	aclRule.SrcportOrIcmptypeFirst = uint16(tcpRule.SourcePortRange.LowerPort)
	aclRule.SrcportOrIcmptypeLast = uint16(tcpRule.SourcePortRange.UpperPort)
	aclRule.DstportOrIcmpcodeFirst = uint16(tcpRule.DestinationPortRange.LowerPort)
	aclRule.DstportOrIcmpcodeLast = uint16(tcpRule.DestinationPortRange.UpperPort)
	aclRule.TCPFlagsValue = uint8(tcpRule.TcpFlagsValue)
	aclRule.TCPFlagsMask = uint8(tcpRule.TcpFlagsMask)
	return aclRule
}

// Sets an UDP ACL rule fields into provided ACL Rule object.
func udpACL(udpRule *acl.ACL_Rule_IpRule_Udp, aclRule *aclapi.ACLRule) *aclapi.ACLRule {
	aclRule.Proto = UDPProto // IANA UDP
	aclRule.SrcportOrIcmptypeFirst = uint16(udpRule.SourcePortRange.LowerPort)
	aclRule.SrcportOrIcmptypeLast = uint16(udpRule.SourcePortRange.UpperPort)
	aclRule.DstportOrIcmpcodeFirst = uint16(udpRule.DestinationPortRange.LowerPort)
	aclRule.DstportOrIcmpcodeLast = uint16(udpRule.DestinationPortRange.UpperPort)
	return aclRule
}
