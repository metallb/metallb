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
	acl "github.com/ligato/vpp-agent/api/models/vpp/acl"
	acl_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

var aclNoRules []*acl.ACL_Rule

var aclErr1Rules = []*acl.ACL_Rule{
	{
		Action: acl.ACL_Rule_PERMIT,
		IpRule: &acl.ACL_Rule_IpRule{
			Ip: &acl.ACL_Rule_IpRule_Ip{
				SourceNetwork:      ".0.",
				DestinationNetwork: "10.20.0.0/24",
			},
		},
	},
}

var aclErr2Rules = []*acl.ACL_Rule{
	{
		Action: acl.ACL_Rule_PERMIT,
		IpRule: &acl.ACL_Rule_IpRule{
			Ip: &acl.ACL_Rule_IpRule_Ip{
				SourceNetwork:      "192.168.1.1/32",
				DestinationNetwork: ".0.",
			},
		},
	},
}

var aclErr3Rules = []*acl.ACL_Rule{
	{
		Action: acl.ACL_Rule_PERMIT,
		IpRule: &acl.ACL_Rule_IpRule{
			Ip: &acl.ACL_Rule_IpRule_Ip{
				SourceNetwork:      "192.168.1.1/32",
				DestinationNetwork: "dead::1/64",
			},
		},
	},
}

var aclErr4Rules = []*acl.ACL_Rule{
	{
		Action: acl.ACL_Rule_PERMIT,
		MacipRule: &acl.ACL_Rule_MacIpRule{
			SourceAddress:        "192.168.0.1",
			SourceAddressPrefix:  uint32(16),
			SourceMacAddress:     "",
			SourceMacAddressMask: "ff:ff:ff:ff:00:00",
		},
	},
}

var aclErr5Rules = []*acl.ACL_Rule{
	{
		Action: acl.ACL_Rule_PERMIT,
		MacipRule: &acl.ACL_Rule_MacIpRule{
			SourceAddress:        "192.168.0.1",
			SourceAddressPrefix:  uint32(16),
			SourceMacAddress:     "11:44:0A:B8:4A:36",
			SourceMacAddressMask: "",
		},
	},
}

var aclErr6Rules = []*acl.ACL_Rule{
	{
		Action: acl.ACL_Rule_PERMIT,
		MacipRule: &acl.ACL_Rule_MacIpRule{
			SourceAddress:        "",
			SourceAddressPrefix:  uint32(16),
			SourceMacAddress:     "11:44:0A:B8:4A:36",
			SourceMacAddressMask: "ff:ff:ff:ff:00:00",
		},
	},
}

var aclIPrules = []*acl.ACL_Rule{
	{
		//RuleName:  "permitIPv4",
		Action: acl.ACL_Rule_PERMIT,
		IpRule: &acl.ACL_Rule_IpRule{
			Ip: &acl.ACL_Rule_IpRule_Ip{
				SourceNetwork:      "192.168.1.1/32",
				DestinationNetwork: "10.20.0.0/24",
			},
		},
	},
	{
		//RuleName:  "permitIPv6",
		Action: acl.ACL_Rule_PERMIT,
		IpRule: &acl.ACL_Rule_IpRule{
			Ip: &acl.ACL_Rule_IpRule_Ip{
				SourceNetwork:      "dead::1/64",
				DestinationNetwork: "dead::2/64",
			},
		},
	},
	{
		//RuleName:  "permitIP",
		Action: acl.ACL_Rule_PERMIT,
		IpRule: &acl.ACL_Rule_IpRule{
			Ip: &acl.ACL_Rule_IpRule_Ip{
				SourceNetwork:      "",
				DestinationNetwork: "",
			},
		},
	},
	{
		//RuleName:  "denyICMP",
		Action: acl.ACL_Rule_DENY,
		IpRule: &acl.ACL_Rule_IpRule{
			Icmp: &acl.ACL_Rule_IpRule_Icmp{
				Icmpv6: false,
				IcmpCodeRange: &acl.ACL_Rule_IpRule_Icmp_Range{
					First: 150,
					Last:  250,
				},
				IcmpTypeRange: &acl.ACL_Rule_IpRule_Icmp_Range{
					First: 1150,
					Last:  1250,
				},
			},
		},
	},
	{
		//RuleName:  "denyICMPv6",
		Action: acl.ACL_Rule_DENY,
		IpRule: &acl.ACL_Rule_IpRule{
			Icmp: &acl.ACL_Rule_IpRule_Icmp{
				Icmpv6: true,
				IcmpCodeRange: &acl.ACL_Rule_IpRule_Icmp_Range{
					First: 150,
					Last:  250,
				},
				IcmpTypeRange: &acl.ACL_Rule_IpRule_Icmp_Range{
					First: 1150,
					Last:  1250,
				},
			},
		},
	},
	{
		//RuleName:  "permitTCP",
		Action: acl.ACL_Rule_PERMIT,
		IpRule: &acl.ACL_Rule_IpRule{
			Tcp: &acl.ACL_Rule_IpRule_Tcp{
				TcpFlagsMask:  20,
				TcpFlagsValue: 10,
				SourcePortRange: &acl.ACL_Rule_IpRule_PortRange{
					LowerPort: 150,
					UpperPort: 250,
				},
				DestinationPortRange: &acl.ACL_Rule_IpRule_PortRange{
					LowerPort: 1150,
					UpperPort: 1250,
				},
			},
		},
	},
	{
		//RuleName:  "denyUDP",
		Action: acl.ACL_Rule_DENY,
		IpRule: &acl.ACL_Rule_IpRule{
			Udp: &acl.ACL_Rule_IpRule_Udp{
				SourcePortRange: &acl.ACL_Rule_IpRule_PortRange{
					LowerPort: 150,
					UpperPort: 250,
				},
				DestinationPortRange: &acl.ACL_Rule_IpRule_PortRange{
					LowerPort: 1150,
					UpperPort: 1250,
				},
			},
		},
	},
}

var aclMACIPrules = []*acl.ACL_Rule{
	{
		//RuleName:  "denyIPv4",
		Action: acl.ACL_Rule_DENY,
		MacipRule: &acl.ACL_Rule_MacIpRule{
			SourceAddress:        "192.168.0.1",
			SourceAddressPrefix:  uint32(16),
			SourceMacAddress:     "11:44:0A:B8:4A:35",
			SourceMacAddressMask: "ff:ff:ff:ff:00:00",
		},
	},
	{
		//RuleName:  "denyIPv6",
		Action: acl.ACL_Rule_DENY,
		MacipRule: &acl.ACL_Rule_MacIpRule{
			SourceAddress:        "dead::1",
			SourceAddressPrefix:  uint32(64),
			SourceMacAddress:     "11:44:0A:B8:4A:35",
			SourceMacAddressMask: "ff:ff:ff:ff:00:00",
		},
	},
}

type testCtx struct {
	*vppcallmock.TestCtx
	aclHandler *ACLVppHandler
	ifIndexes  ifaceidx.IfaceMetadataIndexRW
}

func setupACLTest(t *testing.T) *testCtx {
	ctx := vppcallmock.SetupTestCtx(t)

	ifaceIdx := ifaceidx.NewIfaceIndex(logrus.NewLogger("test"), "test")
	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel, ifaceIdx)

	return &testCtx{
		TestCtx:    ctx,
		aclHandler: aclHandler,
		ifIndexes:  ifaceIdx,
	}
}

func (ctx *testCtx) teardownACLTest() {
	ctx.TeardownTestCtx()
}

// Test add IP acl rules
func TestAddIPAcl(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()
	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})

	aclIndex, err := ctx.aclHandler.AddACL(aclIPrules, "test0")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	_, err = ctx.aclHandler.AddACL(aclNoRules, "test1")
	Expect(err).To(Not(BeNil()))

	_, err = ctx.aclHandler.AddACL(aclErr1Rules, "test2")
	Expect(err).To(Not(BeNil()))

	_, err = ctx.aclHandler.AddACL(aclErr2Rules, "test3")
	Expect(err).To(Not(BeNil()))

	_, err = ctx.aclHandler.AddACL(aclErr3Rules, "test4")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})
	_, err = ctx.aclHandler.AddACL(aclIPrules, "test5")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{Retval: -1})
	_, err = ctx.aclHandler.AddACL(aclIPrules, "test6")
	Expect(err).To(Not(BeNil()))
}

// Test add MACIP acl rules
func TestAddMacIPAcl(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})

	aclIndex, err := ctx.aclHandler.AddMACIPACL(aclMACIPrules, "test6")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	_, err = ctx.aclHandler.AddMACIPACL(aclNoRules, "test7")
	Expect(err).To(Not(BeNil()))

	_, err = ctx.aclHandler.AddMACIPACL(aclErr4Rules, "test8")
	Expect(err).To(Not(BeNil()))

	_, err = ctx.aclHandler.AddMACIPACL(aclErr5Rules, "test9")
	Expect(err).To(Not(BeNil()))

	_, err = ctx.aclHandler.AddMACIPACL(aclErr6Rules, "test10")
	Expect(err).To(Not(BeNil()))
	Expect(err.Error()).To(BeEquivalentTo("invalid IP address "))

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	_, err = ctx.aclHandler.AddMACIPACL(aclMACIPrules, "test11")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{Retval: -1})
	_, err = ctx.aclHandler.AddMACIPACL(aclMACIPrules, "test12")
	Expect(err).To(Not(BeNil()))
}

// Test deletion of IP acl rules
func TestDeleteIPAcl(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()
	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})

	aclIndex, err := ctx.aclHandler.AddACL(aclIPrules, "test_del0")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	rule2del := []*acl.ACL_Rule{
		{
			Action: acl.ACL_Rule_PERMIT,
			IpRule: &acl.ACL_Rule_IpRule{
				Ip: &acl.ACL_Rule_IpRule_Ip{
					SourceNetwork:      "10.20.30.1/32",
					DestinationNetwork: "10.20.0.0/24",
				},
			},
		},
	}

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{ACLIndex: 1})
	aclIndex, err = ctx.aclHandler.AddACL(rule2del, "test_del1")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(1))

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	err = ctx.aclHandler.DeleteACL(5)
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.ACLDelReply{Retval: -1})
	err = ctx.aclHandler.DeleteACL(5)
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.ACLDelReply{})
	err = ctx.aclHandler.DeleteACL(1)
	Expect(err).To(BeNil())
}

// Test deletion of MACIP acl rules
func TestDeleteMACIPAcl(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})

	aclIndex, err := ctx.aclHandler.AddMACIPACL(aclMACIPrules, "test_del2")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	rule2del := []*acl.ACL_Rule{
		{
			Action: acl.ACL_Rule_PERMIT,
			MacipRule: &acl.ACL_Rule_MacIpRule{
				SourceAddress:        "192.168.0.1",
				SourceAddressPrefix:  uint32(16),
				SourceMacAddress:     "11:44:0A:B8:4A:35",
				SourceMacAddressMask: "ff:ff:ff:ff:00:00",
			},
		},
	}

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{ACLIndex: 1})
	aclIndex, err = ctx.aclHandler.AddMACIPACL(rule2del, "test_del3")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(1))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})
	err = ctx.aclHandler.DeleteMACIPACL(5)
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLDelReply{Retval: -1})
	err = ctx.aclHandler.DeleteMACIPACL(5)
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLDelReply{})
	err = ctx.aclHandler.DeleteMACIPACL(1)
	Expect(err).To(BeNil())
}

// Test modification of IP acl rule
func TestModifyIPAcl(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()
	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})

	aclIndex, err := ctx.aclHandler.AddACL(aclIPrules, "test_modify")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	rule2modify := []*acl.ACL_Rule{
		{
			Action: acl.ACL_Rule_PERMIT,
			IpRule: &acl.ACL_Rule_IpRule{
				Ip: &acl.ACL_Rule_IpRule_Ip{
					SourceNetwork:      "10.20.30.1/32",
					DestinationNetwork: "10.20.0.0/24",
				},
			},
		},
		{
			Action: acl.ACL_Rule_PERMIT,
			IpRule: &acl.ACL_Rule_IpRule{
				Ip: &acl.ACL_Rule_IpRule_Ip{
					SourceNetwork:      "dead:dead::3/64",
					DestinationNetwork: "dead:dead::4/64",
				},
			},
		},
	}

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	err = ctx.aclHandler.ModifyACL(0, rule2modify, "test_modify0")
	Expect(err).To(BeNil())

	err = ctx.aclHandler.ModifyACL(0, aclErr1Rules, "test_modify1")
	Expect(err).To(Not(BeNil()))

	err = ctx.aclHandler.ModifyACL(0, aclNoRules, "test_modify2")
	Expect(err).To(BeNil())

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReplaceReply{})
	err = ctx.aclHandler.ModifyACL(0, aclIPrules, "test_modify3")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{Retval: -1})
	err = ctx.aclHandler.ModifyACL(0, aclIPrules, "test_modify4")
	Expect(err).To(Not(BeNil()))
}

// Test modification of MACIP acl rule
func TestModifyMACIPAcl(t *testing.T) {
	ctx := setupACLTest(t)
	defer ctx.teardownACLTest()
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})

	aclIndex, err := ctx.aclHandler.AddMACIPACL(aclMACIPrules, "test_modify")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	rule2modify := []*acl.ACL_Rule{
		{
			Action: acl.ACL_Rule_DENY,
			MacipRule: &acl.ACL_Rule_MacIpRule{
				SourceAddress:        "192.168.10.1",
				SourceAddressPrefix:  uint32(24),
				SourceMacAddress:     "11:44:0A:B8:4A:37",
				SourceMacAddressMask: "ff:ff:ff:ff:00:00",
			},
		},
		{
			Action: acl.ACL_Rule_DENY,
			MacipRule: &acl.ACL_Rule_MacIpRule{
				SourceAddress:        "dead::2",
				SourceAddressPrefix:  uint32(64),
				SourceMacAddress:     "11:44:0A:B8:4A:38",
				SourceMacAddressMask: "ff:ff:ff:ff:00:00",
			},
		},
	}

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReplaceReply{})
	err = ctx.aclHandler.ModifyMACIPACL(0, rule2modify, "test_modify0")
	Expect(err).To(BeNil())

	err = ctx.aclHandler.ModifyMACIPACL(0, aclErr1Rules, "test_modify1")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReplaceReply{})
	err = ctx.aclHandler.ModifyMACIPACL(0, aclIPrules, "test_modify3")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReplaceReply{Retval: -1})
	err = ctx.aclHandler.ModifyMACIPACL(0, aclIPrules, "test_modify4")
	Expect(err).To(Not(BeNil()))
}
