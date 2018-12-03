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
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	acl_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

// Test translation of IP rule into ACL Plugin's format
func TestGetIPRuleMatch(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()
	aclHandler := NewACLVppHandler(ctx.MockChannel, nil)

	icmpV4Rule := aclHandler.getIPRuleMatches(acl_api.ACLRule{
		SrcIPAddr:      []byte{10, 0, 0, 1},
		SrcIPPrefixLen: 24,
		DstIPAddr:      []byte{20, 0, 0, 1},
		DstIPPrefixLen: 24,
		Proto:          ICMPv4Proto,
	})
	if icmpV4Rule.GetIcmp() == nil {
		t.Fatal("should have icmp match")
	}

	icmpV6Rule := aclHandler.getIPRuleMatches(acl_api.ACLRule{
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

	tcpRule := aclHandler.getIPRuleMatches(acl_api.ACLRule{
		SrcIPAddr:      []byte{10, 0, 0, 1},
		SrcIPPrefixLen: 24,
		DstIPAddr:      []byte{20, 0, 0, 1},
		DstIPPrefixLen: 24,
		Proto:          TCPProto,
	})
	if tcpRule.GetTcp() == nil {
		t.Fatal("should have tcp match")
	}

	udpRule := aclHandler.getIPRuleMatches(acl_api.ACLRule{
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
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()
	aclHandler := NewACLVppHandler(ctx.MockChannel, nil)

	macipV4Rule := aclHandler.getMACIPRuleMatches(acl_api.MacipACLRule{
		IsPermit:       1,
		SrcMac:         []byte{2, 'd', 'e', 'a', 'd', 2},
		SrcMacMask:     []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		SrcIPAddr:      []byte{10, 0, 0, 1},
		SrcIPPrefixLen: 32,
	})
	if macipV4Rule.GetSourceMacAddress() == "" {
		t.Fatal("should have mac match")
	}
	macipV6Rule := aclHandler.getMACIPRuleMatches(acl_api.MacipACLRule{
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
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

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

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "test", nil))
	swIfIndexes.RegisterName("if0", 1, nil)

	ifaces, err := aclHandler.DumpIPACL(swIfIndexes)
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(3))
	//Expect(ifaces[0].Identifier.ACLIndex).To(Equal(uint32(0)))
	//Expect(ifaces[0].ACLDetails.Rules[0].AclAction).To(Equal(uint32(1)))
	//Expect(ifaces[1].Identifier.ACLIndex).To(Equal(uint32(1)))
	//Expect(ifaces[2].Identifier.ACLIndex).To(Equal(uint32(2)))
}

// Test dumping of MACIP rules
func TestDumpMACIPACL(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

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

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "test", nil))
	swIfIndexes.RegisterName("if0", 1, nil)

	ifaces, err := aclHandler.DumpMACIPACL(swIfIndexes)
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(3))
	//Expect(ifaces[0].Identifier.ACLIndex).To(Equal(uint32(0)))
	//Expect(ifaces[0].ACLDetails.Rules[0].AclAction).To(Equal(uint32(1)))
	//Expect(ifaces[1].Identifier.ACLIndex).To(Equal(uint32(1)))
	//Expect(ifaces[2].Identifier.ACLIndex).To(Equal(uint32(2)))
}

// Test dumping of interfaces with assigned IP rules
func TestDumpACLInterfaces(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{
		SwIfIndex: 1,
		Count:     2,
		NInput:    1,
		Acls:      []uint32{0, 2},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "test", nil))
	swIfIndexes.RegisterName("if0", 1, nil)

	indexes := []uint32{0, 2}
	ifaces, err := aclHandler.DumpIPACLInterfaces(indexes, swIfIndexes)
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(2))
	Expect(ifaces[0].Ingress).To(Equal([]string{"if0"}))
	Expect(ifaces[2].Egress).To(Equal([]string{"if0"}))
}

// Test dumping of interfaces with assigned MACIP rules
func TestDumpMACIPACLInterfaces(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{
		SwIfIndex: 1,
		Count:     2,
		Acls:      []uint32{0, 1},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "test-sw_if_indexes", ifaceidx.IndexMetadata))
	swIfIndexes.RegisterName("if0", 1, nil)

	indexes := []uint32{0, 1}
	ifaces, err := aclHandler.DumpMACIPACLInterfaces(indexes, swIfIndexes)
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(2))
	Expect(ifaces[0].Ingress).To(Equal([]string{"if0"}))
	Expect(ifaces[0].Egress).To(BeNil())
	Expect(ifaces[1].Ingress).To(Equal([]string{"if0"}))
	Expect(ifaces[1].Egress).To(BeNil())
}

// Test dumping of all configured ACLs with IP-type ruleData
func TestDumpIPAcls(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&acl_api.ACLDetails{
		ACLIndex: 0,
		Count:    1,
		R:        []acl_api.ACLRule{{IsPermit: 1}},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	IPRuleACLs, err := aclHandler.DumpIPAcls()
	Expect(err).To(Succeed())
	Expect(IPRuleACLs).To(HaveLen(1))
}

// Test dumping of all configured ACLs with MACIP-type ruleData
func TestDumpMacIPAcls(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&acl_api.MacipACLDetails{
		ACLIndex: 0,
		Count:    1,
		R:        []acl_api.MacipACLRule{{IsPermit: 1}},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	MacIPRuleACLs, err := aclHandler.DumpMacIPAcls()
	Expect(err).To(Succeed())
	Expect(MacIPRuleACLs).To(HaveLen(1))
}

func TestDumpInterfaceIPAcls(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

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

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	ACLs, err := aclHandler.DumpInterfaceIPAcls(0)
	Expect(err).To(Succeed())
	Expect(ACLs.Acls).To(HaveLen(2))
}

func TestDumpInterfaceMACIPAcls(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

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

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	ACLs, err := aclHandler.DumpInterfaceMACIPAcls(0)
	Expect(err).To(Succeed())
	Expect(ACLs.Acls).To(HaveLen(2))
}

func TestDumpInterface(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{
		SwIfIndex: 0,
		Count:     2,
		NInput:    1,
		Acls:      []uint32{0, 1},
	})
	IPacls, err := aclHandler.DumpInterfaceIPACLs(0)
	Expect(err).To(BeNil())
	Expect(IPacls.Acls).To(HaveLen(2))

	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	IPacls, err = aclHandler.DumpInterfaceIPACLs(0)
	Expect(err).To(BeNil())
	Expect(IPacls.Acls).To(HaveLen(0))

	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{
		SwIfIndex: 0,
		Count:     2,
		Acls:      []uint32{0, 1},
	})
	MACIPacls, err := aclHandler.DumpInterfaceMACIPACLs(0)
	Expect(err).To(BeNil())
	Expect(MACIPacls.Acls).To(HaveLen(2))

	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{})
	MACIPacls, err = aclHandler.DumpInterfaceMACIPACLs(0)
	Expect(err).To(BeNil())
	Expect(MACIPacls.Acls).To(HaveLen(0))
}

func TestDumpInterfaces(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

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

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	IPacls, MACIPacls, err := aclHandler.DumpInterfaces()
	Expect(err).To(BeNil())
	Expect(IPacls).To(HaveLen(3))
	Expect(MACIPacls).To(HaveLen(2))
}
