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

package aclplugin_test

import (
	"git.fd.io/govpp.git/adapter/mock"
	"git.fd.io/govpp.git/core"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/aclplugin"
	acl_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
	"testing"
)

var ipAcls = acl.AccessLists_Acl{
	AclName: "acl1",
	Rules: []*acl.AccessLists_Acl_Rule{
		{
			AclAction: acl.AclAction_PERMIT,
			Match: &acl.AccessLists_Acl_Rule_Match{
				IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
					Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
						SourceNetwork:      "192.168.1.1/32",
						DestinationNetwork: "10.20.0.1/24",
					},
				},
			},
		},
	},
	Interfaces: &acl.AccessLists_Acl_Interfaces{
		Ingress: []string{"if1"},
		Egress:  []string{"if2"},
	},
}

var macipAcls = acl.AccessLists_Acl{
	AclName: "acl2",
	Rules: []*acl.AccessLists_Acl_Rule{
		{
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
	},
	Interfaces: &acl.AccessLists_Acl_Interfaces{
		Ingress: []string{"if3"},
		Egress:  nil,
	},
}

// Test creation of ACLs and sets acls to interfaces
func TestConfigureACL(t *testing.T) {
	ctx, connection, plugin := aclTestSetup(t, false)
	defer aclTestTeardown(connection, plugin)
	// ipAcl Replies
	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})

	// Test configure ipAcl
	err := plugin.ConfigureACL(&ipAcls)
	Expect(err).To(BeNil())

	// macipAcl Replies
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})

	// Test configure macipAcl
	err = plugin.ConfigureACL(&macipAcls)
	Expect(err).To(BeNil())
}

// Test modification of non existing acl
func TestModifyNonExistingACL(t *testing.T) {
	ctx, connection, plugin := aclTestSetup(t, true)
	defer aclTestTeardown(connection, plugin)

	ipAcl := acl.AccessLists_Acl{
		AclName: "acl3",
		Rules: []*acl.AccessLists_Acl_Rule{
			{
				Match: &acl.AccessLists_Acl_Rule_Match{
					IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{},
				},
			},
		},
	}
	macipAcl := acl.AccessLists_Acl{
		AclName: "acl2",
		Rules: []*acl.AccessLists_Acl_Rule{
			{
				Match: &acl.AccessLists_Acl_Rule_Match{
					MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{},
				},
			},
		},
	}
	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})

	// Test modify ipAcl
	err := plugin.ModifyACL(&ipAcls, &ipAcl)
	Expect(err).ToNot(BeNil())
	err = plugin.ModifyACL(&macipAcls, &macipAcl)
	Expect(err).ToNot(BeNil())
}

// Test modification of given acl
func TestModifyACL(t *testing.T) {
	ctx, connection, plugin := aclTestSetup(t, true)
	defer aclTestTeardown(connection, plugin)

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	err := plugin.ConfigureACL(&ipAcls)
	Expect(err).To(BeNil())

	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceAddDelReply{})
	err = plugin.ConfigureACL(&macipAcls)
	Expect(err).To(BeNil())

	ipAcl := acl.AccessLists_Acl{
		AclName: "acl1",
		Rules: []*acl.AccessLists_Acl_Rule{
			{
				AclAction: acl.AclAction_DENY,
				Match: &acl.AccessLists_Acl_Rule_Match{
					IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
						Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
							SourceNetwork:      "19.16.0.1/32",
							DestinationNetwork: "100.200.1.0/24",
						},
					},
				},
			},
		},
		Interfaces: &acl.AccessLists_Acl_Interfaces{
			Ingress: []string{"if2"},
			Egress:  []string{"if3", "if4"},
		},
	}
	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})

	// Test modify ipAcl
	err = plugin.ModifyACL(&ipAcls, &ipAcl)
	Expect(err).To(BeNil())

	macipAcl := acl.AccessLists_Acl{
		AclName: "acl2",
		Rules: []*acl.AccessLists_Acl_Rule{
			{
				AclAction: acl.AclAction_DENY,
				Match: &acl.AccessLists_Acl_Rule_Match{
					MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
						SourceAddress:        "102.16.1.1",
						SourceAddressPrefix:  uint32(16),
						SourceMacAddress:     "11:44:DE:AD:4A:35",
						SourceMacAddressMask: "ff:ff:ff:ff:00:00",
					},
				},
			},
		},
		Interfaces: &acl.AccessLists_Acl_Interfaces{
			Ingress: []string{"if1"},
			Egress:  nil,
		},
	}
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReplaceReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceAddDelReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceAddDelReply{})

	// Test modify macipAcl
	err = plugin.ModifyACL(&macipAcls, &macipAcl)
	Expect(err).To(BeNil())
}

// Test deletion of non existing acl
func TestDeleteNonExistingACL(t *testing.T) {
	ctx, connection, plugin := aclTestSetup(t, false)
	defer aclTestTeardown(connection, plugin)

	ctx.MockVpp.MockReply(&acl_api.ACLDelReply{})
	// Test delete non-existing ipAcl
	err := plugin.DeleteACL(&ipAcls)
	Expect(err).To(Not(BeNil()))

	ctx.MockVpp.MockReply(&acl_api.ACLDelReply{})
	// Test delete non-existing ipAcl
	err = plugin.DeleteACL(&macipAcls)
	Expect(err).To(Not(BeNil()))
}

// Test deletion of given acl
func TestDeleteACL(t *testing.T) {
	ctx, connection, plugin := aclTestSetup(t, true)
	defer aclTestTeardown(connection, plugin)

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	err := plugin.ConfigureACL(&ipAcls)
	Expect(err).To(BeNil())
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceAddDelReply{})
	err = plugin.ConfigureACL(&macipAcls)
	Expect(err).To(BeNil())

	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLDelReply{})

	// Test delete ipAcl
	err = plugin.DeleteACL(&ipAcls)
	Expect(err).To(BeNil())

	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceAddDelReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLDelReply{})
	// Test delete macipAcl
	err = plugin.DeleteACL(&macipAcls)
	Expect(err).To(BeNil())
}

// Test listiong of IP ACLs
func TestDumpIPACL(t *testing.T) {
	ctx, connection, plugin := aclTestSetup(t, true)
	defer aclTestTeardown(connection, plugin)

	ctx.MockVpp.MockReply(
		&acl_api.ACLDetails{
			ACLIndex: 0,
			Tag:      []byte("acl1"),
			Count:    1,
			R:        []acl_api.ACLRule{{IsPermit: 1}},
		},
		&acl_api.ACLDetails{
			ACLIndex: 1,
			Tag:      []byte("acl2"),
			Count:    2,
			R:        []acl_api.ACLRule{{IsPermit: 0}, {IsPermit: 2}},
		},
		&acl_api.ACLDetails{
			ACLIndex: 2,
			Tag:      []byte("acl3"),
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
	acls, err := plugin.DumpIPACL()
	Expect(err).To(BeNil())
	Expect(acls).To(HaveLen(3))
}

// Test listiong of MACIP ACLs
func TestDumpMACIPACL(t *testing.T) {
	ctx, connection, plugin := aclTestSetup(t, true)
	defer aclTestTeardown(connection, plugin)

	ctx.MockVpp.MockReply(
		&acl_api.MacipACLDetails{
			ACLIndex: 0,
			Tag:      []byte("acl4"),
			Count:    1,
			R:        []acl_api.MacipACLRule{{IsPermit: 1}},
		},
		&acl_api.MacipACLDetails{
			ACLIndex: 1,
			Tag:      []byte("acl5"),
			Count:    2,
			R:        []acl_api.MacipACLRule{{IsPermit: 0}, {IsPermit: 2}},
		},
		&acl_api.MacipACLDetails{
			ACLIndex: 2,
			Tag:      []byte("acl6"),
			Count:    3,
			R:        []acl_api.MacipACLRule{{IsPermit: 0}, {IsPermit: 1}, {IsPermit: 2}},
		})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceListDetails{
		SwIfIndex: 1,
		Count:     1,
		Acls:      []uint32{1},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	// Test dump acls
	acls, err := plugin.DumpMACIPACL()
	Expect(err).To(BeNil())
	Expect(acls).To(HaveLen(3))
}

// Test configures new interface for every ACL found in cache
func TestResolveCreatedInterface(t *testing.T) {
	ctx, connection, plugin := aclTestSetup(t, false)
	defer aclTestTeardown(connection, plugin)

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	err := plugin.ConfigureACL(&ipAcls)
	Expect(err).To(BeNil())
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})
	err = plugin.ConfigureACL(&macipAcls)
	Expect(err).To(BeNil())

	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceAddDelReply{})

	// Test
	err = plugin.ResolveCreatedInterface("if1", 1)
	Expect(err).To(BeNil())
	err = plugin.ResolveCreatedInterface("if2", 2)
	Expect(err).To(BeNil())
	err = plugin.ResolveCreatedInterface("if3", 3)
	Expect(err).To(BeNil())
}

// Test configuration of interfaces with deleted ACLs
func TestResolveDeletedInterface(t *testing.T) {
	ctx, connection, plugin := aclTestSetup(t, true)
	defer aclTestTeardown(connection, plugin)

	ctx.MockVpp.MockReply(&acl_api.ACLAddReplaceReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceListDetails{})
	ctx.MockVpp.MockReply(&acl_api.ACLInterfaceSetACLListReply{})
	err := plugin.ConfigureACL(&ipAcls)
	Expect(err).To(BeNil())
	ctx.MockVpp.MockReply(&acl_api.MacipACLAddReply{})
	ctx.MockVpp.MockReply(&acl_api.MacipACLInterfaceAddDelReply{})
	err = plugin.ConfigureACL(&macipAcls)
	Expect(err).To(BeNil())

	// Test ResolveDeletedInterface
	err = plugin.ResolveDeletedInterface("if1", 1)
	Expect(err).To(BeNil())
	err = plugin.ResolveDeletedInterface("if2", 2)
	Expect(err).To(BeNil())
	err = plugin.ResolveDeletedInterface("if3", 3)
	Expect(err).To(BeNil())
}

/* ACL Test Setup */
func aclTestSetup(t *testing.T, createIfs bool) (*vppcallmock.TestCtx, *core.Connection, *aclplugin.ACLConfigurator) {
	RegisterTestingT(t)

	ctx := &vppcallmock.TestCtx{
		MockVpp: mock.NewVppAdapter(),
	}
	connection, err := core.Connect(ctx.MockVpp)
	Expect(err).ShouldNot(HaveOccurred())

	// Logger
	log := logging.ForPlugin("test-log")
	log.SetLevel(logging.DebugLevel)

	// Interface indices
	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "acl-plugin", nil))
	if createIfs {
		ifIndexes.RegisterName("if1", 1, nil)
		ifIndexes.RegisterName("if2", 2, nil)
		ifIndexes.RegisterName("if3", 3, nil)
		ifIndexes.RegisterName("if4", 4, nil)
	}

	// Configurator
	plugin := &aclplugin.ACLConfigurator{}
	err = plugin.Init(log, connection, ifIndexes)
	Expect(err).To(BeNil())

	return ctx, connection, plugin
}

/* ACL Test Teardown */
func aclTestTeardown(connection *core.Connection, plugin *aclplugin.ACLConfigurator) {
	connection.Disconnect()
	Expect(plugin.Close()).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}
