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

	acl_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

var aclNoRules []*acl.AccessLists_Acl_Rule

var aclErr1Rules = []*acl.AccessLists_Acl_Rule{
	{
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
					SourceNetwork:      ".0.",
					DestinationNetwork: "10.20.0.0/24",
				},
			},
		},
	},
}

var aclErr2Rules = []*acl.AccessLists_Acl_Rule{
	{
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
					SourceNetwork:      "192.168.1.1/32",
					DestinationNetwork: ".0.",
				},
			},
		},
	},
}

var aclErr3Rules = []*acl.AccessLists_Acl_Rule{
	{
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
					SourceNetwork:      "192.168.1.1/32",
					DestinationNetwork: "dead::1/64",
				},
			},
		},
	},
}

var aclErr4Rules = []*acl.AccessLists_Acl_Rule{
	{
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
				SourceAddress:        "192.168.0.1",
				SourceAddressPrefix:  uint32(16),
				SourceMacAddress:     "",
				SourceMacAddressMask: "ff:ff:ff:ff:00:00",
			},
		},
	},
}

var aclErr5Rules = []*acl.AccessLists_Acl_Rule{
	{
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
				SourceAddress:        "192.168.0.1",
				SourceAddressPrefix:  uint32(16),
				SourceMacAddress:     "11:44:0A:B8:4A:36",
				SourceMacAddressMask: "",
			},
		},
	},
}

var aclErr6Rules = []*acl.AccessLists_Acl_Rule{
	{
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
				SourceAddress:        "",
				SourceAddressPrefix:  uint32(16),
				SourceMacAddress:     "11:44:0A:B8:4A:36",
				SourceMacAddressMask: "ff:ff:ff:ff:00:00",
			},
		},
	},
}

var aclIPrules = []*acl.AccessLists_Acl_Rule{
	{
		RuleName:  "permitIPv4",
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
					SourceNetwork:      "192.168.1.1/32",
					DestinationNetwork: "10.20.0.0/24",
				},
			},
		},
	},
	{
		RuleName:  "permitIPv6",
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
					SourceNetwork:      "dead::1/64",
					DestinationNetwork: "dead::2/64",
				},
			},
		},
	},
	{
		RuleName:  "permitIP",
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
					SourceNetwork:      "",
					DestinationNetwork: "",
				},
			},
		},
	},
	{
		RuleName:  "denyICMP",
		AclAction: acl.AclAction_DENY,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Icmp: &acl.AccessLists_Acl_Rule_Match_IpRule_Icmp{
					Icmpv6: false,
					IcmpCodeRange: &acl.AccessLists_Acl_Rule_Match_IpRule_Icmp_Range{
						First: 150,
						Last:  250,
					},
					IcmpTypeRange: &acl.AccessLists_Acl_Rule_Match_IpRule_Icmp_Range{
						First: 1150,
						Last:  1250,
					},
				},
			},
		},
	},
	{
		RuleName:  "denyICMPv6",
		AclAction: acl.AclAction_DENY,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Icmp: &acl.AccessLists_Acl_Rule_Match_IpRule_Icmp{
					Icmpv6: true,
					IcmpCodeRange: &acl.AccessLists_Acl_Rule_Match_IpRule_Icmp_Range{
						First: 150,
						Last:  250,
					},
					IcmpTypeRange: &acl.AccessLists_Acl_Rule_Match_IpRule_Icmp_Range{
						First: 1150,
						Last:  1250,
					},
				},
			},
		},
	},
	{
		RuleName:  "permitTCP",
		AclAction: acl.AclAction_PERMIT,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Tcp: &acl.AccessLists_Acl_Rule_Match_IpRule_Tcp{
					TcpFlagsMask:  20,
					TcpFlagsValue: 10,
					SourcePortRange: &acl.AccessLists_Acl_Rule_Match_IpRule_PortRange{
						LowerPort: 150,
						UpperPort: 250,
					},
					DestinationPortRange: &acl.AccessLists_Acl_Rule_Match_IpRule_PortRange{
						LowerPort: 1150,
						UpperPort: 1250,
					},
				},
			},
		},
	},
	{
		RuleName:  "denyUDP",
		AclAction: acl.AclAction_DENY,
		Match: &acl.AccessLists_Acl_Rule_Match{
			IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
				Udp: &acl.AccessLists_Acl_Rule_Match_IpRule_Udp{
					SourcePortRange: &acl.AccessLists_Acl_Rule_Match_IpRule_PortRange{
						LowerPort: 150,
						UpperPort: 250,
					},
					DestinationPortRange: &acl.AccessLists_Acl_Rule_Match_IpRule_PortRange{
						LowerPort: 1150,
						UpperPort: 1250,
					},
				},
			},
		},
	},
}

var aclMACIPrules = []*acl.AccessLists_Acl_Rule{
	{
		RuleName:  "denyIPv4",
		AclAction: acl.AclAction_DENY,
		Match: &acl.AccessLists_Acl_Rule_Match{
			MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
				SourceAddress:        "192.168.0.1",
				SourceAddressPrefix:  uint32(16),
				SourceMacAddress:     "11:44:0A:B8:4A:35",
				SourceMacAddressMask: "ff:ff:ff:ff:00:00",
			},
		},
	},
	{
		RuleName:  "denyIPv6",
		AclAction: acl.AclAction_DENY,
		Match: &acl.AccessLists_Acl_Rule_Match{
			MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
				SourceAddress:        "dead::1",
				SourceAddressPrefix:  uint32(64),
				SourceMacAddress:     "11:44:0A:B8:4A:35",
				SourceMacAddressMask: "ff:ff:ff:ff:00:00",
			},
		},
	},
}

// Test add IP acl rules
func TestAddIPAcl(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()
	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	aclIndex, err := aclHandler.AddIPACL(aclIPrules, "test0")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	_, err = aclHandler.AddIPACL(aclNoRules, "test1")
	Expect(err).To(Not(BeNil()))

	_, err = aclHandler.AddIPACL(aclErr1Rules, "test2")
	Expect(err).To(Not(BeNil()))

	_, err = aclHandler.AddIPACL(aclErr2Rules, "test3")
	Expect(err).To(Not(BeNil()))

	_, err = aclHandler.AddIPACL(aclErr3Rules, "test4")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})
	_, err = aclHandler.AddIPACL(aclIPrules, "test5")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{Retval: -1})
	_, err = aclHandler.AddIPACL(aclIPrules, "test6")
	Expect(err).To(Not(BeNil()))
}

// Test add MACIP acl rules
func TestAddMacIPAcl(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	aclIndex, err := aclHandler.AddMacIPACL(aclMACIPrules, "test6")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	_, err = aclHandler.AddMacIPACL(aclNoRules, "test7")
	Expect(err).To(Not(BeNil()))

	_, err = aclHandler.AddMacIPACL(aclErr4Rules, "test8")
	Expect(err).To(Not(BeNil()))

	_, err = aclHandler.AddMacIPACL(aclErr5Rules, "test9")
	Expect(err).To(Not(BeNil()))

	_, err = aclHandler.AddMacIPACL(aclErr6Rules, "test10")
	Expect(err).To(Not(BeNil()))
	Expect(err.Error()).To(BeEquivalentTo("invalid IP address "))

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	_, err = aclHandler.AddMacIPACL(aclMACIPrules, "test11")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{Retval: -1})
	_, err = aclHandler.AddMacIPACL(aclMACIPrules, "test12")
	Expect(err).To(Not(BeNil()))
}

// Test deletion of IP acl rules
func TestDeleteIPAcl(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()
	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	aclIndex, err := aclHandler.AddIPACL(aclIPrules, "test_del0")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	rule2del := []*acl.AccessLists_Acl_Rule{
		{
			RuleName:  "permitIP",
			AclAction: acl.AclAction_PERMIT,
			Match: &acl.AccessLists_Acl_Rule_Match{
				IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
					Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
						SourceNetwork:      "10.20.30.1/32",
						DestinationNetwork: "10.20.0.0/24",
					},
				},
			},
		},
	}

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{ACLIndex: 1})
	aclIndex, err = aclHandler.AddIPACL(rule2del, "test_del1")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(1))

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	err = aclHandler.DeleteIPACL(5)
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.ACLDelReply{Retval: -1})
	err = aclHandler.DeleteIPACL(5)
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.ACLDelReply{})
	err = aclHandler.DeleteIPACL(1)
	Expect(err).To(BeNil())
}

// Test deletion of MACIP acl rules
func TestDeleteMACIPAcl(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	aclIndex, err := aclHandler.AddMacIPACL(aclMACIPrules, "test_del2")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	rule2del := []*acl.AccessLists_Acl_Rule{
		{
			RuleName:  "permit",
			AclAction: acl.AclAction_PERMIT,
			Match: &acl.AccessLists_Acl_Rule_Match{
				MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
					SourceAddress:        "192.168.0.1",
					SourceAddressPrefix:  uint32(16),
					SourceMacAddress:     "11:44:0A:B8:4A:35",
					SourceMacAddressMask: "ff:ff:ff:ff:00:00",
				},
			},
		},
	}

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{ACLIndex: 1})
	aclIndex, err = aclHandler.AddMacIPACL(rule2del, "test_del3")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(1))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})
	err = aclHandler.DeleteMacIPACL(5)
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLDelReply{Retval: -1})
	err = aclHandler.DeleteMacIPACL(5)
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLDelReply{})
	err = aclHandler.DeleteMacIPACL(1)
	Expect(err).To(BeNil())
}

// Test modification of IP acl rule
func TestModifyIPAcl(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()
	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	aclIndex, err := aclHandler.AddIPACL(aclIPrules, "test_modify")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	rule2modify := []*acl.AccessLists_Acl_Rule{
		{
			RuleName:  "permitIP",
			AclAction: acl.AclAction_PERMIT,
			Match: &acl.AccessLists_Acl_Rule_Match{
				IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
					Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
						SourceNetwork:      "10.20.30.1/32",
						DestinationNetwork: "10.20.0.0/24",
					},
				},
			},
		},
		{
			RuleName:  "permitIP",
			AclAction: acl.AclAction_PERMIT,
			Match: &acl.AccessLists_Acl_Rule_Match{
				IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
					Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
						SourceNetwork:      "dead:dead::3/64",
						DestinationNetwork: "dead:dead::4/64",
					},
				},
			},
		},
	}

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	err = aclHandler.ModifyIPACL(0, rule2modify, "test_modify0")
	Expect(err).To(BeNil())

	err = aclHandler.ModifyIPACL(0, aclErr1Rules, "test_modify1")
	Expect(err).To(Not(BeNil()))

	err = aclHandler.ModifyIPACL(0, aclNoRules, "test_modify2")
	Expect(err).To(BeNil())

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReplaceReply{})
	err = aclHandler.ModifyIPACL(0, aclIPrules, "test_modify3")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{Retval: -1})
	err = aclHandler.ModifyIPACL(0, aclIPrules, "test_modify4")
	Expect(err).To(Not(BeNil()))
}

// Test modification of MACIP acl rule
func TestModifyMACIPAcl(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})

	aclHandler := NewACLVppHandler(ctx.MockChannel, ctx.MockChannel)

	aclIndex, err := aclHandler.AddMacIPACL(aclMACIPrules, "test_modify")
	Expect(err).To(BeNil())
	Expect(aclIndex).To(BeEquivalentTo(0))

	rule2modify := []*acl.AccessLists_Acl_Rule{
		{
			RuleName:  "permitMACIP",
			AclAction: acl.AclAction_DENY,
			Match: &acl.AccessLists_Acl_Rule_Match{
				MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
					SourceAddress:        "192.168.10.1",
					SourceAddressPrefix:  uint32(24),
					SourceMacAddress:     "11:44:0A:B8:4A:37",
					SourceMacAddressMask: "ff:ff:ff:ff:00:00",
				},
			},
		},
		{
			RuleName:  "permitMACIPv6",
			AclAction: acl.AclAction_DENY,
			Match: &acl.AccessLists_Acl_Rule_Match{
				MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
					SourceAddress:        "dead::2",
					SourceAddressPrefix:  uint32(64),
					SourceMacAddress:     "11:44:0A:B8:4A:38",
					SourceMacAddressMask: "ff:ff:ff:ff:00:00",
				},
			},
		},
	}

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReplaceReply{})
	err = aclHandler.ModifyMACIPACL(0, rule2modify, "test_modify0")
	Expect(err).To(BeNil())

	err = aclHandler.ModifyMACIPACL(0, aclErr1Rules, "test_modify1")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReplaceReply{})
	err = aclHandler.ModifyMACIPACL(0, aclIPrules, "test_modify3")
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReplaceReply{Retval: -1})
	err = aclHandler.ModifyMACIPACL(0, aclIPrules, "test_modify4")
	Expect(err).To(Not(BeNil()))
}
