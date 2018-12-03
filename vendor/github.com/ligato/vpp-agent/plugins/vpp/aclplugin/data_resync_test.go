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

package aclplugin_test

import (
	"testing"

	acl_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

var acls = []*acl.AccessLists_Acl{
	{AclName: "acl1",
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
	},
	{AclName: "acl2",
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
	},
}

// Test synchronisation - writes ACLs to the empty VPP
func TestResyncEmpty(t *testing.T) {
	// Setup
	ctx, connection, plugin := aclTestSetup(t, false)
	defer aclTestTeardown(connection, plugin)

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name:    (&acl_api.ACLDump{}).GetMessageName(),
			Ping:    true,
			Message: &acl_api.ACLDetails{},
		},
		{
			Name:    (&acl_api.ACLInterfaceListDump{}).GetMessageName(),
			Ping:    true,
			Message: &acl_api.ACLInterfaceListDetails{},
		},
		{
			Name:    (&acl_api.MacipACLDump{}).GetMessageName(),
			Ping:    true,
			Message: &acl_api.MacipACLDetails{},
		},
		{
			Name:    (&acl_api.MacipACLInterfaceListDump{}).GetMessageName(),
			Ping:    true,
			Message: &acl_api.MacipACLInterfaceListDetails{},
		},
		{
			Name:    (&acl_api.ACLAddReplace{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.ACLAddReplaceReply{},
		},
		{
			Name:    (&acl_api.MacipACLAdd{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.MacipACLAddReply{},
		},
	})

	err := plugin.Resync(acls)
	Expect(err).To(BeNil())

	_, metaIpACL, found := plugin.GetL3L4AclIfIndexes().LookupIdx(acls[0].AclName)
	Expect(found).To(BeTrue())
	Expect(metaIpACL).ToNot(BeNil())

	_, metaMacIpACL, found := plugin.GetL2AclIfIndexes().LookupIdx(acls[1].AclName)
	Expect(found).To(BeTrue())
	Expect(metaMacIpACL).ToNot(BeNil())
}

// Test synchronisation - writes ACLs to the already configured VPP
func TestResyncConfigured(t *testing.T) {
	// Setup
	ctx, connection, plugin := aclTestSetup(t, false)
	defer aclTestTeardown(connection, plugin)

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name: (&acl_api.ACLDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.ACLDetails{
				ACLIndex: 0,
				Tag:      []byte("acl3"),
				Count:    1,
				R:        []acl_api.ACLRule{{IsPermit: 1}},
			},
		},
		{
			Name: (&acl_api.ACLInterfaceListDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.ACLInterfaceListDetails{
				SwIfIndex: 1,
				Count:     2,
				NInput:    1,
				Acls:      []uint32{0, 2},
			},
		},
		{
			Name: (&acl_api.MacipACLDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.MacipACLDetails{
				ACLIndex: 0,
				Tag:      []byte("acl4"),
				Count:    2,
				R:        []acl_api.MacipACLRule{{IsPermit: 0}, {IsPermit: 2}},
			},
		},
		{
			Name: (&acl_api.MacipACLInterfaceListDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.MacipACLInterfaceListDetails{
				SwIfIndex: 1,
				Count:     1,
				Acls:      []uint32{1},
			},
		},
		{
			Name:    (&acl_api.ACLDel{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.ACLDelReply{},
		},
		{
			Name:    (&acl_api.MacipACLDel{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.MacipACLDelReply{},
		},
		{
			Name:    (&acl_api.ACLAddReplace{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.ACLAddReplaceReply{},
		},
		{
			Name:    (&acl_api.MacipACLAdd{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.MacipACLAddReply{},
		},
	})

	plugin.GetL3L4AclIfIndexes().RegisterName("acl3", 1, nil)
	plugin.GetL2AclIfIndexes().RegisterName("acl4", 1, nil)

	// new acls do not exist
	_, _, found := plugin.GetL3L4AclIfIndexes().LookupIdx(acls[0].AclName)
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetL2AclIfIndexes().LookupIdx(acls[1].AclName)
	Expect(found).To(BeFalse())

	err := plugin.Resync(acls)
	Expect(err).To(BeNil())

	// new acls are present
	_, metaIpACL, found := plugin.GetL3L4AclIfIndexes().LookupIdx(acls[0].AclName)
	Expect(found).To(BeTrue())
	Expect(metaIpACL).ToNot(BeNil())

	_, metaMacIpACL, found := plugin.GetL2AclIfIndexes().LookupIdx(acls[1].AclName)
	Expect(found).To(BeTrue())
	Expect(metaMacIpACL).ToNot(BeNil())

	// old acls do not exist
	_, _, found = plugin.GetL3L4AclIfIndexes().LookupIdx("acl3")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetL2AclIfIndexes().LookupIdx("acl4")
	Expect(found).To(BeFalse())
}

// Test Resync with error when removing existing IP ACL
func TestResyncErr1(t *testing.T) {
	// Setup
	ctx, connection, plugin := aclTestSetup(t, false)
	defer aclTestTeardown(connection, plugin)

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name: (&acl_api.ACLDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.ACLDetails{
				ACLIndex: 0,
				Tag:      []byte("acl3"),
				Count:    1,
				R:        []acl_api.ACLRule{{IsPermit: 1}},
			},
		},
		{
			Name: (&acl_api.ACLInterfaceListDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.ACLInterfaceListDetails{
				SwIfIndex: 1,
				Count:     2,
				NInput:    1,
				Acls:      []uint32{0, 2},
			},
		},
		{
			Name: (&acl_api.MacipACLDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.MacipACLDetails{
				ACLIndex: 0,
				Tag:      []byte("acl4"),
				Count:    2,
				R:        []acl_api.MacipACLRule{{IsPermit: 0}, {IsPermit: 2}},
			},
		},
		{
			Name: (&acl_api.MacipACLInterfaceListDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.MacipACLInterfaceListDetails{
				SwIfIndex: 1,
				Count:     1,
				Acls:      []uint32{1},
			},
		},
		// wrong msg
		{
			Name:    (&acl_api.ACLDel{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.MacipACLDelReply{},
		},
	})

	plugin.GetL3L4AclIfIndexes().RegisterName("acl3", 1, nil)
	plugin.GetL2AclIfIndexes().RegisterName("acl4", 1, nil)

	err := plugin.Resync(acls)
	Expect(err).ToNot(BeNil())

	Expect(plugin.GetL3L4AclIfIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetL2AclIfIndexes().GetMapping().ListNames()).To(HaveLen(0))

	// Old ACLs should be removed during resync
	_, _, found := plugin.GetL3L4AclIfIndexes().LookupIdx("acl3")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetL2AclIfIndexes().LookupIdx("acl4")
	Expect(found).To(BeFalse())
}

// Test Resync with error when removing existing IP ACL
func TestResyncErr2(t *testing.T) {
	// Setup
	ctx, connection, plugin := aclTestSetup(t, false)
	defer aclTestTeardown(connection, plugin)

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name: (&acl_api.ACLDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.ACLDetails{
				ACLIndex: 0,
				Tag:      []byte("acl3"),
				Count:    1,
				R:        []acl_api.ACLRule{{IsPermit: 1}},
			},
		},
		{
			Name: (&acl_api.ACLInterfaceListDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.ACLInterfaceListDetails{
				SwIfIndex: 1,
				Count:     2,
				NInput:    1,
				Acls:      []uint32{0, 2},
			},
		},
		{
			Name: (&acl_api.MacipACLDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.MacipACLDetails{
				ACLIndex: 0,
				Tag:      []byte("acl4"),
				Count:    2,
				R:        []acl_api.MacipACLRule{{IsPermit: 0}, {IsPermit: 2}},
			},
		},
		{
			Name: (&acl_api.MacipACLInterfaceListDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.MacipACLInterfaceListDetails{
				SwIfIndex: 1,
				Count:     1,
				Acls:      []uint32{1},
			},
		},
		{
			Name:    (&acl_api.ACLDel{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.ACLDelReply{},
		},
		// wrong msg
		{
			Name:    (&acl_api.MacipACLDel{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.ACLDelReply{},
		},
	})

	plugin.GetL3L4AclIfIndexes().RegisterName("acl3", 1, nil)
	plugin.GetL2AclIfIndexes().RegisterName("acl4", 1, nil)

	err := plugin.Resync(acls)
	Expect(err).ToNot(BeNil())

	// Both ACLs were removed
	Expect(plugin.GetL3L4AclIfIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetL2AclIfIndexes().GetMapping().ListNames()).To(HaveLen(0))

	// All old ACLs should be removed from mapping s
	_, _, found := plugin.GetL3L4AclIfIndexes().LookupIdx("acl3")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetL2AclIfIndexes().LookupIdx("acl4")
	Expect(found).To(BeFalse())
}

// Test Resync with error when configuring new ALCs
func TestResyncErr3(t *testing.T) {
	// Setup
	ctx, connection, plugin := aclTestSetup(t, false)
	defer aclTestTeardown(connection, plugin)

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name: (&acl_api.ACLDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.ACLDetails{
				ACLIndex: 0,
				Tag:      []byte("acl3"),
				Count:    1,
				R:        []acl_api.ACLRule{{IsPermit: 1}},
			},
		},
		{
			Name: (&acl_api.ACLInterfaceListDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.ACLInterfaceListDetails{
				SwIfIndex: 1,
				Count:     2,
				NInput:    1,
				Acls:      []uint32{0, 2},
			},
		},
		{
			Name: (&acl_api.MacipACLDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.MacipACLDetails{
				ACLIndex: 0,
				Tag:      []byte("acl4"),
				Count:    2,
				R:        []acl_api.MacipACLRule{{IsPermit: 0}, {IsPermit: 2}},
			},
		},
		{
			Name: (&acl_api.MacipACLInterfaceListDump{}).GetMessageName(),
			Ping: true,
			Message: &acl_api.MacipACLInterfaceListDetails{
				SwIfIndex: 1,
				Count:     1,
				Acls:      []uint32{1},
			},
		},
		{
			Name:    (&acl_api.ACLDel{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.ACLDelReply{},
		},
		{
			Name:    (&acl_api.MacipACLDel{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.MacipACLDelReply{},
		},
		// wrong msg
		{
			Name:    (&acl_api.ACLAddReplace{}).GetMessageName(),
			Ping:    false,
			Message: &acl_api.MacipACLAddReplaceReply{},
		},
	})

	err := plugin.Resync(acls)
	Expect(err).To(Not(BeNil()))

	// old acls have been removed, but no new added - wrong msg during configure
	Expect(plugin.GetL3L4AclIfIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetL2AclIfIndexes().GetMapping().ListNames()).To(HaveLen(0))
}
