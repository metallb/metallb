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
	"testing"

	"github.com/ligato/cn-infra/logging/logrus"
	acl_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
	. "github.com/onsi/gomega"
)

// Test translation of IP rule into ACL Plugin's format
func TestGetIPRuleMatch(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	icmpV4Rule := ctx.aclHandler.getIPRuleMatches(acl_api.ACLRule{
		SrcIPAddr:      []byte{10, 0, 0, 1},
		SrcIPPrefixLen: 24,
		DstIPAddr:      []byte{20, 0, 0, 1},
		DstIPPrefixLen: 24,
		Proto:          ICMPv4Proto,
	})
	if icmpV4Rule.GetIcmp() == nil {
		t.Fatal("should have icmp match")
	}

	icmpV6Rule := ctx.aclHandler.getIPRuleMatches(acl_api.ACLRule{
		IsIPv6:         1,
		SrcIPAddr:      []byte{'d', 'e', 'd', 'd', 1},
		SrcIPPrefixLen: 64,
		DstIPAddr:      []byte{'d', 'e', 'd', 'd', 2},
		DstIPPrefixLen: 32,
		Proto:          ICMPv6Proto,
	})
	if icmpV6Rule.GetIcmp() == nil {
		t.Fatal("should have icmpv6 match")
	}

	tcpRule := ctx.aclHandler.getIPRuleMatches(acl_api.ACLRule{
		SrcIPAddr:      []byte{10, 0, 0, 1},
		SrcIPPrefixLen: 24,
		DstIPAddr:      []byte{20, 0, 0, 1},
		DstIPPrefixLen: 24,
		Proto:          TCPProto,
	})
	if tcpRule.GetTcp() == nil {
		t.Fatal("should have tcp match")
	}

	udpRule := ctx.aclHandler.getIPRuleMatches(acl_api.ACLRule{
		SrcIPAddr:      []byte{10, 0, 0, 1},
		SrcIPPrefixLen: 24,
		DstIPAddr:      []byte{20, 0, 0, 1},
		DstIPPrefixLen: 24,
		Proto:          UDPProto,
	})
	if udpRule.GetUdp() == nil {
		t.Fatal("should have udp match")
	}
}

// Test translation of MACIP rule into ACL Plugin's format
func TestGetMACIPRuleMatches(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	macipV4Rule := ctx.aclHandler.getMACIPRuleMatches(acl_api.MacipACLRule{
		IsPermit:       1,
		SrcMac:         []byte{2, 'd', 'e', 'a', 'd', 2},
		SrcMacMask:     []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		SrcIPAddr:      []byte{10, 0, 0, 1},
		SrcIPPrefixLen: 32,
	})
	if macipV4Rule.GetSourceMacAddress() == "" {
		t.Fatal("should have mac match")
	}
	macipV6Rule := ctx.aclHandler.getMACIPRuleMatches(acl_api.MacipACLRule{
		IsPermit:       0,
		IsIPv6:         1,
		SrcMac:         []byte{2, 'd', 'e', 'a', 'd', 2},
		SrcMacMask:     []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		SrcIPAddr:      []byte{'d', 'e', 'a', 'd', 1},
		SrcIPPrefixLen: 64,
	})
	if macipV6Rule.GetSourceMacAddress() == "" {
		t.Fatal("should have mac match")
	}
}

// Test dumping of IP rules
func TestDumpIPACL(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(
		&acl_api.ACLDetails{
			ACLIndex: 0,
			Tag:      []byte{'a', 'c', 'l', '1'},
			Count:    1,
			R:        []acl_api.ACLRule{{IsPermit: 1}},
		},
		&acl_api.ACLDetails{
			ACLIndex: 1,
			Tag:      []byte{'a', 'c', 'l', '2'},
			Count:    2,
			R:        []acl_api.ACLRule{{IsPermit: 0}, {IsPermit: 2}},
		},
		&acl_api.ACLDetails{
			ACLIndex: 2,
			Tag:      []byte{'a', 'c', 'l', '3'},
			Count:    3,
			R:        []acl_api.ACLRule{{IsPermit: 0}, {IsPermit: 1}, {IsPermit: 2}},
		})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{
		SwIfIndex: 1,
		Count:     2,
		NInput:    1,
		Acls:      []uint32{0, 2},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	ctx.ifIndexes.Put("if0", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	ifaces, err := ctx.aclHandler.DumpACL()
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(3))
	//Expect(ifaces[0].Identifier.ACLIndex).To(Equal(uint32(0)))
	//Expect(ifaces[0].ACLDetails.Rules[0].AclAction).To(Equal(uint32(1)))
	//Expect(ifaces[1].Identifier.ACLIndex).To(Equal(uint32(1)))
	//Expect(ifaces[2].Identifier.ACLIndex).To(Equal(uint32(2)))
}

// Test dumping of MACIP rules
func TestDumpMACIPACL(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(
		&acl_api.MacipACLDetails{
			ACLIndex: 0,
			Tag:      []byte{'a', 'c', 'l', '1'},
			Count:    1,
			R:        []acl_api.MacipACLRule{{IsPermit: 1}},
		},
		&acl_api.MacipACLDetails{
			ACLIndex: 1,
			Tag:      []byte{'a', 'c', 'l', '2'},
			Count:    2,
			R:        []acl_api.MacipACLRule{{IsPermit: 0}, {IsPermit: 2}},
		},
		&acl_api.MacipACLDetails{
			ACLIndex: 2,
			Tag:      []byte{'a', 'c', 'l', '3'},
			Count:    3,
			R:        []acl_api.MacipACLRule{{IsPermit: 0}, {IsPermit: 1}, {IsPermit: 2}},
		})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{
		SwIfIndex: 1,
		Count:     2,
		Acls:      []uint32{0, 2},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	swIfIndexes := ifaceidx.NewIfaceIndex(logrus.DefaultLogger(), "test")
	swIfIndexes.Put("if0", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	ifaces, err := ctx.aclHandler.DumpMACIPACL()
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(3))
	//Expect(ifaces[0].Identifier.ACLIndex).To(Equal(uint32(0)))
	//Expect(ifaces[0].ACLDetails.Rules[0].AclAction).To(Equal(uint32(1)))
	//Expect(ifaces[1].Identifier.ACLIndex).To(Equal(uint32(1)))
	//Expect(ifaces[2].Identifier.ACLIndex).To(Equal(uint32(2)))
}

// Test dumping of interfaces with assigned IP rules
func TestDumpACLInterfaces(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{
		SwIfIndex: 1,
		Count:     2,
		NInput:    1,
		Acls:      []uint32{0, 2},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	ctx.ifIndexes.Put("if0", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	indexes := []uint32{0, 2}
	ifaces, err := ctx.aclHandler.DumpACLInterfaces(indexes)
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(2))
	Expect(ifaces[0].Ingress).To(Equal([]string{"if0"}))
	Expect(ifaces[2].Egress).To(Equal([]string{"if0"}))
}

// Test dumping of interfaces with assigned MACIP rules
func TestDumpMACIPACLInterfaces(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{
		SwIfIndex: 1,
		Count:     2,
		Acls:      []uint32{0, 1},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	ctx.ifIndexes.Put("if0", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	indexes := []uint32{0, 1}
	ifaces, err := ctx.aclHandler.DumpMACIPACLInterfaces(indexes)
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(2))
	Expect(ifaces[0].Ingress).To(Equal([]string{"if0"}))
	Expect(ifaces[0].Egress).To(BeNil())
	Expect(ifaces[1].Ingress).To(Equal([]string{"if0"}))
	Expect(ifaces[1].Egress).To(BeNil())
}

// Test dumping of all configured ACLs with IP-type ruleData
func TestDumpIPAcls(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(&acl_api.ACLDetails{
		ACLIndex: 0,
		Count:    1,
		R:        []acl_api.ACLRule{{IsPermit: 1}},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	IPRuleACLs, err := ctx.aclHandler.DumpIPAcls()
	Expect(err).To(Succeed())
	Expect(IPRuleACLs).To(HaveLen(1))
}

// Test dumping of all configured ACLs with MACIP-type ruleData
func TestDumpMacIPAcls(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(&acl_api.MacipACLDetails{
		ACLIndex: 0,
		Count:    1,
		R:        []acl_api.MacipACLRule{{IsPermit: 1}},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	MacIPRuleACLs, err := ctx.aclHandler.DumpMacIPAcls()
	Expect(err).To(Succeed())
	Expect(MacIPRuleACLs).To(HaveLen(1))
}

func TestDumpInterfaceIPAcls(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{
		SwIfIndex: 0,
		Count:     2,
		NInput:    1,
		Acls:      []uint32{0, 1},
	})
	ctx.MockVpp.MockReply(&acl_api.ACLDetails{
		ACLIndex: 0,
		Count:    1,
		R:        []acl_api.ACLRule{{IsPermit: 1}, {IsPermit: 0}},
	})
	ctx.MockVpp.MockReply(&acl_api.ACLDetails{
		ACLIndex: 1,
		Count:    1,
		R:        []acl_api.ACLRule{{IsPermit: 2}, {IsPermit: 0}},
	})

	ACLs, err := ctx.aclHandler.DumpInterfaceACLs(0)
	Expect(err).To(Succeed())
	Expect(ACLs).To(HaveLen(2))
}

func TestDumpInterfaceMACIPAcls(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{
		SwIfIndex: 0,
		Count:     2,
		Acls:      []uint32{0, 1},
	})
	ctx.MockVpp.MockReply(&acl_api.MacipACLDetails{
		ACLIndex: 0,
		Count:    1,
		R:        []acl_api.MacipACLRule{{IsPermit: 1}, {IsPermit: 0}},
	})
	ctx.MockVpp.MockReply(&acl_api.MacipACLDetails{
		ACLIndex: 1,
		Count:    1,
		R:        []acl_api.MacipACLRule{{IsPermit: 2}, {IsPermit: 1}},
	})

	ACLs, err := ctx.aclHandler.DumpInterfaceMACIPACLs(0)
	Expect(err).To(Succeed())
	Expect(ACLs).To(HaveLen(2))
}

func TestDumpInterface(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{
		SwIfIndex: 0,
		Count:     2,
		NInput:    1,
		Acls:      []uint32{0, 1},
	})
	IPacls, err := ctx.aclHandler.DumpInterfaceACLList(0)
	Expect(err).To(BeNil())
	Expect(IPacls.Acls).To(HaveLen(2))

	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	IPacls, err = ctx.aclHandler.DumpInterfaceACLList(0)
	Expect(err).To(BeNil())
	Expect(IPacls.Acls).To(HaveLen(0))

	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{
		SwIfIndex: 0,
		Count:     2,
		Acls:      []uint32{0, 1},
	})
	MACIPacls, err := ctx.aclHandler.DumpInterfaceMACIPACLList(0)
	Expect(err).To(BeNil())
	Expect(MACIPacls.Acls).To(HaveLen(2))

	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{})
	MACIPacls, err = ctx.aclHandler.DumpInterfaceMACIPACLList(0)
	Expect(err).To(BeNil())
	Expect(MACIPacls.Acls).To(HaveLen(0))
}

func TestDumpInterfaces(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()

	ctx.MockVpp.MockReply(
		&acl_api.ACLInterfaceListDetails{
			SwIfIndex: 0,
			Count:     2,
			NInput:    1,
			Acls:      []uint32{0, 1},
		},
		&acl_api.ACLInterfaceListDetails{
			SwIfIndex: 1,
			Count:     1,
			NInput:    1,
			Acls:      []uint32{2},
		},
		&acl_api.ACLInterfaceListDetails{
			SwIfIndex: 2,
			Count:     2,
			NInput:    1,
			Acls:      []uint32{3, 4},
		})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{
		SwIfIndex: 3,
		Count:     2,
		Acls:      []uint32{6, 7},
	},
		&acl_api.MacipACLInterfaceListDetails{
			SwIfIndex: 4,
			Count:     1,
			Acls:      []uint32{5},
		})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	IPacls, MACIPacls, err := ctx.aclHandler.DumpInterfacesLists()
	Expect(err).To(BeNil())
	Expect(IPacls).To(HaveLen(3))
	Expect(MACIPacls).To(HaveLen(2))
}
