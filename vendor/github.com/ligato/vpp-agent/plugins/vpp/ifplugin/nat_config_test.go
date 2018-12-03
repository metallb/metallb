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

package ifplugin_test

import (
	"testing"

	"git.fd.io/govpp.git/core"

	"git.fd.io/govpp.git/adapter/mock"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	nat_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/nat"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

/* Global NAT Test Cases */

// Enable NAT forwarding
func TestNatConfiguratorEnableForwarding(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	// Data
	data := getTestNatForwardingConfig(true)

	// Test
	err := plugin.SetNatGlobalConfig(data)
	Expect(err).To(BeNil())
}

// Disable NAT forwarding
func TestNatConfiguratorDisableForwarding(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	// Data
	data := getTestNatForwardingConfig(false)

	// Test
	err := plugin.SetNatGlobalConfig(data)
	Expect(err).To(BeNil())
}

// Modify NAT forwarding
func TestNatConfiguratorModifyForwarding(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // Create
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // Modify
	// Data
	oldData := getTestNatForwardingConfig(true)
	newData := getTestNatForwardingConfig(false)
	// Test create
	err := plugin.SetNatGlobalConfig(oldData)
	Expect(err).To(BeNil())
	Expect(plugin.GetGlobalNat().Forwarding).To(BeTrue())
	// Test modify
	err = plugin.ModifyNatGlobalConfig(oldData, newData)
	Expect(err).To(BeNil())
	Expect(plugin.GetGlobalNat().Forwarding).To(BeFalse())
}

// NAT set forwarding return error
func TestNatConfiguratorCreateForwardingError(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{
		Retval: 1,
	})
	// Data
	data := getTestNatForwardingConfig(false)

	// Test
	err := plugin.SetNatGlobalConfig(data)
	Expect(err).ToNot(BeNil())
}

// Modify NAT forwarding error
func TestNatConfiguratorModifyForwardingError(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // Create
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{   // Modify
		Retval: 1,
	})
	// Data
	oldData := getTestNatForwardingConfig(true)
	newData := getTestNatForwardingConfig(false)

	// Test create
	err := plugin.SetNatGlobalConfig(oldData)
	Expect(err).To(BeNil())
	Expect(plugin.GetGlobalNat().Forwarding).To(BeTrue())
	// Test modify
	err = plugin.ModifyNatGlobalConfig(oldData, newData)
	Expect(err).ToNot(BeNil())
}

// Enable two interfaces for NAT, then remove one
func TestNatConfiguratorEnableDisableInterfaces(t *testing.T) {
	ctx, connection, plugin, ifIndexes := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // First case
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // Second case
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{})
	// Registration
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 2, nil)
	// Data
	var ifs1, ifs2 []*nat.Nat44Global_NatInterface
	firstData := &nat.Nat44Global{NatInterfaces: append(ifs1,
		getTestNatInterfaceConfig("if1", true, false),
		getTestNatInterfaceConfig("if2", true, false))}
	secondData := &nat.Nat44Global{NatInterfaces: append(ifs2,
		getTestNatInterfaceConfig("if1", true, false))}

	// Test set interfaces
	err := plugin.SetNatGlobalConfig(firstData)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
	Expect(plugin.GetGlobalNat().NatInterfaces).To(HaveLen(2))
	// Test disable one interface
	err = plugin.SetNatGlobalConfig(secondData)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
	Expect(plugin.GetGlobalNat().NatInterfaces).To(HaveLen(1))
}

// Attempt to enable interface resulting in return value error
func TestNatConfiguratorEnableDisableInterfacesError(t *testing.T) {
	ctx, connection, plugin, ifIndexes := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // First case
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{
		Retval: 1,
	})
	// Registration
	ifIndexes.RegisterName("if1", 1, nil)
	// Data
	var ifs1 []*nat.Nat44Global_NatInterface
	firstData := &nat.Nat44Global{NatInterfaces: append(ifs1,
		getTestNatInterfaceConfig("if1", true, false))}

	// Test set interfaces
	err := plugin.SetNatGlobalConfig(firstData)
	Expect(err).ToNot(BeNil())
}

// Enable two output interfaces for NAT, then remove one
func TestNatConfiguratorEnableDisableOutputInterfaces(t *testing.T) {
	ctx, connection, plugin, ifIndexes := natTestSetup(t)

	defer natTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // First case
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // Second case
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	// Registration
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 2, nil)
	// Data
	var ifs1, ifs2 []*nat.Nat44Global_NatInterface
	firstData := &nat.Nat44Global{NatInterfaces: append(ifs1,
		getTestNatInterfaceConfig("if1", true, true),
		getTestNatInterfaceConfig("if2", true, true))}
	secondData := &nat.Nat44Global{NatInterfaces: append(ifs2,
		getTestNatInterfaceConfig("if1", true, true))}

	// Test set output interfaces
	err := plugin.SetNatGlobalConfig(firstData)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
	Expect(plugin.GetGlobalNat().NatInterfaces).To(HaveLen(2))
	// Test disable one output interface
	err = plugin.SetNatGlobalConfig(secondData)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
	Expect(plugin.GetGlobalNat().NatInterfaces).To(HaveLen(1))
}

// Create and modify NAT interfaces and output interfaces
func TestNatConfiguratorModifyInterfaces(t *testing.T) {
	ctx, connection, plugin, ifIndexes := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // Create
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{}) // Modify
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	// Registration
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 2, nil)
	ifIndexes.RegisterName("if3", 3, nil)
	// Data
	var ifs1, ifs2 []*nat.Nat44Global_NatInterface
	oldData := &nat.Nat44Global{NatInterfaces: append(ifs1,
		getTestNatInterfaceConfig("if1", true, false),
		getTestNatInterfaceConfig("if2", false, true),
		getTestNatInterfaceConfig("if3", true, true))}
	newData := &nat.Nat44Global{NatInterfaces: append(ifs2,
		getTestNatInterfaceConfig("if1", false, true),
		getTestNatInterfaceConfig("if2", true, false),
		getTestNatInterfaceConfig("if3", true, true))}

	// Test create
	err := plugin.SetNatGlobalConfig(oldData)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
	Expect(plugin.GetGlobalNat().NatInterfaces).To(HaveLen(3))
	// Test modify
	err = plugin.ModifyNatGlobalConfig(oldData, newData)
	Expect(err).To(BeNil())
}

// Test interface cache registering and un-registering interfaces after configuration
func TestNatConfiguratorInterfaceCache(t *testing.T) {
	ctx, connection, plugin, ifIndexes := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	// Data
	var ifs []*nat.Nat44Global_NatInterface
	data := &nat.Nat44Global{NatInterfaces: append(ifs,
		getTestNatInterfaceConfig("if1", true, false),
		getTestNatInterfaceConfig("if2", true, true))}

	// Test create
	err := plugin.SetNatGlobalConfig(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeTrue())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeTrue())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
	// Test register first interface
	ifIndexes.RegisterName("if1", 1, nil)
	err = plugin.ResolveCreatedInterface("if1", 1)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeTrue())
	// Test register second interface
	ifIndexes.RegisterName("if2", 2, nil)
	err = plugin.ResolveCreatedInterface("if2", 2)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	// Test un-register second interface
	_, _, found := ifIndexes.UnregisterName("if2")
	Expect(found).To(BeTrue())
	err = plugin.ResolveDeletedInterface("if2", 1)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeTrue())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
	// Test re-enable second interface
	ifIndexes.RegisterName("if2", 2, nil)
	err = plugin.ResolveCreatedInterface("if2", 2)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
}

// Set NAT address pools
func TestNatConfiguratorCreateAddressPool(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	// Data
	var aps []*nat.Nat44Global_AddressPool
	data := &nat.Nat44Global{AddressPools: append(aps,
		getTestNatAddressPoolConfig("10.0.0.1", "10.0.0.2", 0, true),
		getTestNatAddressPoolConfig("10.0.0.3", "", 1, false),
		getTestNatAddressPoolConfig("", "10.0.0.4", 1, false))}

	// Test set address pool
	err := plugin.SetNatGlobalConfig(data)
	Expect(err).To(BeNil())
	Expect(plugin.GetGlobalNat().AddressPools).To(HaveLen(3))
}

// Set NAT address pool with return value error
func TestNatConfiguratorCreateAddressPoolRetvalError(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{
		Retval: 1,
	})
	// Data
	var aps []*nat.Nat44Global_AddressPool
	data := &nat.Nat44Global{AddressPools: append(aps,
		getTestNatAddressPoolConfig("10.0.0.1", "10.0.0.2", 0, true))}

	// Test set address pool
	err := plugin.SetNatGlobalConfig(data)
	Expect(err).ToNot(BeNil())
}

// Set and modify NAT address pools
func TestNatConfiguratorModifyAddressPool(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // Configure
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{}) // Modify
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	// Data
	var aps1, aps2 []*nat.Nat44Global_AddressPool
	oldData := &nat.Nat44Global{AddressPools: append(aps1,
		getTestNatAddressPoolConfig("10.0.0.1", "", 0, true),
		getTestNatAddressPoolConfig("", "10.0.0.2", 1, false))}
	newData := &nat.Nat44Global{AddressPools: append(aps2,
		getTestNatAddressPoolConfig("10.0.0.1", "", 0, true),
		getTestNatAddressPoolConfig("", "10.0.0.3", 1, false))}

	// Test set address pool
	err := plugin.SetNatGlobalConfig(oldData)
	Expect(err).To(BeNil())
	Expect(plugin.GetGlobalNat().AddressPools).To(HaveLen(2))
	// Test modify address pool
	err = plugin.ModifyNatGlobalConfig(oldData, newData)
	Expect(err).To(BeNil())
	Expect(plugin.GetGlobalNat().AddressPools).To(HaveLen(2))
}

// Test various errors which may occur during address pool configuration
func TestNatConfiguratorAddressPoolErrors(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	// Data
	var aps1, aps2, aps3 []*nat.Nat44Global_AddressPool
	data1 := &nat.Nat44Global{AddressPools: append(aps1, getTestNatAddressPoolConfig("", "", 0, true))}
	data2 := &nat.Nat44Global{AddressPools: append(aps2, getTestNatAddressPoolConfig("invalid-ip", "", 0, true))}
	data3 := &nat.Nat44Global{AddressPools: append(aps3, getTestNatAddressPoolConfig("", "invalid-ip", 0, true))}

	// Test no IP address provided
	err := plugin.SetNatGlobalConfig(data1)
	Expect(err).ToNot(BeNil())
	// Test invalid first IP
	err = plugin.SetNatGlobalConfig(data2)
	Expect(err).ToNot(BeNil())
	// Test invalid last IP
	err = plugin.SetNatGlobalConfig(data3)
	Expect(err).ToNot(BeNil())
}

// Set NAT address pool with invalid ip addresses
func TestNatConfiguratorModifyAddressPoolErrors(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	// Data
	var aps []*nat.Nat44Global_AddressPool
	oldData := &nat.Nat44Global{}
	newData := &nat.Nat44Global{AddressPools: append(aps,
		getTestNatAddressPoolConfig("", "", 0, true),                    // no IP
		getTestNatAddressPoolConfig("invalid-ip", "", 0, true),          // invalid first IP
		getTestNatAddressPoolConfig("10.0.0.1", "invalid-ip", 0, true))} // invalid last
	// Test set address pool

	// Test
	err := plugin.SetNatGlobalConfig(oldData)
	Expect(err).To(BeNil())
	err = plugin.ModifyNatGlobalConfig(oldData, newData)
	Expect(err).ToNot(BeNil())
}

// Set NAT address pool with invalid ip addresses and then modify it with correct one
func TestNatConfiguratorModifyAddressPoolModifyErrors(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	// Data
	var aps []*nat.Nat44Global_AddressPool
	oldData := &nat.Nat44Global{AddressPools: append(aps,
		getTestNatAddressPoolConfig("invalid-ip", "", 0, true))} // invalid
	newData := &nat.Nat44Global{AddressPools: append(aps,
		getTestNatAddressPoolConfig("10.0.0.1", "", 0, true))} // valid
	// Test set address pool

	// Test
	err := plugin.SetNatGlobalConfig(oldData)
	Expect(err).ToNot(BeNil())
	err = plugin.ModifyNatGlobalConfig(oldData, newData)
	Expect(err).To(BeNil())
}

// Remove global NAT configuration
func TestNatConfiguratorDeleteGlobalConfig(t *testing.T) {
	ctx, connection, plugin, ifIndexes := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44ForwardingEnableDisableReply{}) // Configure
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{}) // Delete
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{})
	ctx.MockVpp.MockReply(&nat_api.NatSetReassReply{})
	ctx.MockVpp.MockReply(&nat_api.NatSetReassReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelOutputFeatureReply{}) // Re-register
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{})
	// Registration
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 2, nil)
	// Data
	var ifs []*nat.Nat44Global_NatInterface
	var aps []*nat.Nat44Global_AddressPool
	data := &nat.Nat44Global{NatInterfaces: append(ifs,
		getTestNatInterfaceConfig("if1", true, false),
		getTestNatInterfaceConfig("if2", false, true),
		getTestNatInterfaceConfig("if3", false, false)),
		AddressPools: append(aps, getTestNatAddressPoolConfig("10.0.0.1", "10.0.0.2", 0, true))}

	// Test set config
	err := plugin.SetNatGlobalConfig(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if3")).To(BeTrue())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if3")).To(BeFalse())
	// Test un-register interface
	_, _, found := ifIndexes.UnregisterName("if2")
	Expect(found).To(BeTrue())
	err = plugin.ResolveDeletedInterface("if2", 1)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeTrue())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
	// Test delete config
	err = plugin.DeleteNatGlobalConfig(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if3")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeTrue())
	Expect(plugin.IsInNotDisabledIfCache("if3")).To(BeTrue())
	Expect(plugin.GetGlobalNat()).To(BeNil())
	// Test re-create interfaces
	ifIndexes.RegisterName("if2", 2, nil)
	err = plugin.ResolveCreatedInterface("if2", 2)
	Expect(err).To(BeNil())
	ifIndexes.RegisterName("if3", 3, nil)
	err = plugin.ResolveCreatedInterface("if3", 3)
	Expect(err).To(BeNil())
	Expect(plugin.IsInNotEnabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotEnabledIfCache("if2")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if1")).To(BeFalse())
	Expect(plugin.IsInNotDisabledIfCache("if2")).To(BeFalse())
}

// Remove global NAT configuration with errors
func TestNatConfiguratorDeleteGlobalConfigErrors(t *testing.T) {
	ctx, connection, plugin, ifIndexes := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44InterfaceAddDelFeatureReply{
		Retval: 1,
	})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelAddressRangeReply{
		Retval: 1,
	})
	// Registration
	ifIndexes.RegisterName("if1", 1, nil)
	// Data
	var ifs []*nat.Nat44Global_NatInterface
	var aps []*nat.Nat44Global_AddressPool
	data := &nat.Nat44Global{NatInterfaces: append(ifs,
		getTestNatInterfaceConfig("if1", true, false)),
		AddressPools: append(aps, getTestNatAddressPoolConfig("10.0.0.1", "10.0.0.2", 0, true)),
	}

	// Test delete config
	err := plugin.DeleteNatGlobalConfig(data)
	Expect(err).ToNot(BeNil())
}

// Remove empty global NAT configuration
func TestNatConfiguratorDeleteGlobalConfigEmpty(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)
	ctx.MockVpp.MockReply(&nat_api.NatSetReassReply{})
	ctx.MockVpp.MockReply(&nat_api.NatSetReassReply{})
	// Data
	data := &nat.Nat44Global{}

	// Test delete empty config
	err := plugin.DeleteNatGlobalConfig(data)
	Expect(err).To(BeNil())
	Expect(plugin.GetGlobalNat()).To(BeNil())
}

/* SNAT test cases */

// Test SNAT Create
func TestNatConfiguratorSNatCreate(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	data := &nat.Nat44SNat_SNatConfig{
		Label: "test-snat",
	}

	// Test configure SNAT without local IPs
	err := plugin.ConfigureSNat(data)
	Expect(err).To(BeNil())
}

// Test SNAT Modify
func TestNatConfiguratorSNatModify(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	oldData := &nat.Nat44SNat_SNatConfig{
		Label: "test-snat",
	}
	newData := &nat.Nat44SNat_SNatConfig{
		Label: "test-snat",
	}

	// Test configure SNAT without local IPs
	err := plugin.ModifySNat(oldData, newData)
	Expect(err).To(BeNil())
}

// Test SNAT Delete
func TestNatConfiguratorSNatDelete(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	data := &nat.Nat44SNat_SNatConfig{
		Label: "test-snat",
	}

	// Test configure SNAT without local IPs
	err := plugin.DeleteSNat(data)
	Expect(err).To(BeNil())
}

/* DNAT test cases */

// Configure DNAT static mapping without local IP
func TestNatConfiguratorDNatStaticMappingNoLocalIPError(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{})
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 8000, nat.Protocol_TCP))}

	// Test configure DNAT without local IPs
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
}

// Configure DNAT static mapping using external IP
func TestNatConfiguratorDNatStaticMapping(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{})
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 8000, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.2", 9000, 32)))}

	// Test configure DNAT
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
}

// Configure DNAT static mapping using external interface
func TestNatConfiguratorDNatStaticMappingExternalInterface(t *testing.T) {
	ctx, connection, plugin, ifconfig := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{})
	// Registrations
	ifconfig.RegisterName("if1", 1, nil)
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "if1", "10.0.0.1", 8000, nat.Protocol_UDP,
			getTestNatStaticLocalIP("10.0.0.2", 9000, 32)))}

	// Configure DNAT with external interface
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
}

// Configure DNAT static mapping as address-only
func TestNatConfiguratorDNatStaticMappingAddressOnly(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{})
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 0, nat.Protocol_ICMP,
			getTestNatStaticLocalIP("10.0.0.2", 0, 32)))}

	// Test configure DNAT address only
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
}

// Configure DNAT with invalid local IP
func TestNatConfiguratorDNatStaticMappingInvalidLocalAddressError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 0, 0,
			getTestNatStaticLocalIP("no-ip", 0, 32)))}

	// Test configure DNAT with invalid local IP
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeFalse())
}

// Configure DNAT with invalid external IP
func TestNatConfiguratorDNatStaticMappingInvalidExternalAddressError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "no-ip", 0, 0,
			getTestNatStaticLocalIP("10.0.0.1", 0, 32)))}

	// Test configure DNAT with invalid external IP
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeFalse())
}

// Configure DNAT with non-existing external interface
func TestNatConfiguratorDNatStaticMappingMissingInterfaceError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "if1", "10.0.0.1", 0, 0,
			getTestNatStaticLocalIP("10.0.0.2", 0, 32)))}

	// Test configure DNAT with missing external interface
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeFalse())
}

// Configure DNAT with unknown protocol and check whether it will be set to default
func TestNatConfiguratorDNatStaticMappingUnknownProtocol(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{})
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 0, 10,
			getTestNatStaticLocalIP("10.0.0.2", 0, 32)))}

	// Test configure DNAT with unnown protocol
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
}

// Configure DNAT static mapping with load balancer
func TestNatConfiguratorDNatStaticMappingLb(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelLbStaticMappingReply{})
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 8000, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.2", 9000, 35),
			getTestNatStaticLocalIP("10.0.0.3", 9001, 65)))}

	// Test configure DNAT static mapping with load balancer
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
}

// Configure DNAT static mapping with load balancer with invalid local IP
func TestNatConfiguratorDNatStaticMappingLbInvalidLocalError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 0, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.2", 0, 35),
			getTestNatStaticLocalIP("no-ip", 8000, 65)))}

	// Test configure DNAT static mapping with load balancer with invalid local IP
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeFalse())
}

// Configure DNAT static mapping with load balancer with missing external port
func TestNatConfiguratorDNatStaticMappingLbMissingExternalPortError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 0, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.2", 8000, 35),
			getTestNatStaticLocalIP("10.0.0.3", 9000, 65)))}

	// Test configure static mapping with load balancer with missing external port
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeFalse())
}

// Configure DNAT static mapping with load balancer with invalid external IP
func TestNatConfiguratorDNatStaticMappingLbInvalidExternalIPError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "no-ip", 8000, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.1", 9000, 35),
			getTestNatStaticLocalIP("10.0.0.2", 9001, 65)))}

	// Test DNAT static mapping invalid external IP
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeFalse())
}

// Configure NAT identity mapping
func TestNatConfiguratorDNatIdentityMapping(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{})
	// Data
	var idMaps []*nat.Nat44DNat_DNatConfig_IdentityMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", IdMappings: append(idMaps,
		getTestNatIdentityMappingConfig(0, "", "10.0.0.1", 8000, nat.Protocol_TCP))}

	// Test
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetIDMappingIdentifier(data.IdMappings[0])
	Expect(plugin.IsDNatLabelIDMappingRegistered(id)).To(BeTrue())
}

// Configure NAT identity mapping with address interface
func TestNatConfiguratorDNatIdentityMappingInterface(t *testing.T) {
	ctx, connection, plugin, ifIndexes := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{})
	// Register
	ifIndexes.RegisterName("if1", 1, nil)
	// Data
	var idMaps []*nat.Nat44DNat_DNatConfig_IdentityMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", IdMappings: append(idMaps,
		getTestNatIdentityMappingConfig(0, "if1", "", 0, nat.Protocol_TCP))}

	// Test identity mapping with address interface
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetIDMappingIdentifier(data.IdMappings[0])
	Expect(plugin.IsDNatLabelIDMappingRegistered(id)).To(BeTrue())
}

// Configure NAT identity mapping with address interface while interface is not registered
func TestNatConfiguratorDNatIdentityMappingMissingInterfaceError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var idMaps []*nat.Nat44DNat_DNatConfig_IdentityMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", IdMappings: append(idMaps,
		getTestNatIdentityMappingConfig(0, "if1", "", 0, nat.Protocol_TCP))}

	// Test identity mapping with address interface	while interface is not registered
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id := ifplugin.GetIDMappingIdentifier(data.IdMappings[0])
	Expect(plugin.IsDNatLabelIDMappingRegistered(id)).To(BeFalse())
}

// Create NAT identity mapping with invalid IP address
func TestNatConfiguratorDNatIdentityMappingInvalidIPError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var idMaps []*nat.Nat44DNat_DNatConfig_IdentityMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", IdMappings: append(idMaps,
		getTestNatIdentityMappingConfig(0, "", "no-ip", 9000, nat.Protocol_TCP))}

	// Test identity mapping with invalid IP
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id := ifplugin.GetIDMappingIdentifier(data.IdMappings[0])
	Expect(plugin.IsDNatLabelIDMappingRegistered(id)).To(BeFalse())
}

// Create identity mapping without IP address and interface set
func TestNatConfiguratorDNatIdentityMappingNoInterfaceAndIPError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)

	defer natTestTeardown(connection, plugin)
	// Data
	var idMaps []*nat.Nat44DNat_DNatConfig_IdentityMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", IdMappings: append(idMaps,
		getTestNatIdentityMappingConfig(0, "", "", 8000, nat.Protocol_TCP))}

	// Test identity mapping without interface and IP
	err := plugin.ConfigureDNat(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id := ifplugin.GetIDMappingIdentifier(data.IdMappings[0])
	Expect(plugin.IsDNatLabelIDMappingRegistered(id)).To(BeFalse())
}

// Configure and modify static and identity mappings
func TestNatConfiguratorDNatModify(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{}) // Configure
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelLbStaticMappingReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelLbStaticMappingReply{}) // Modify
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{})
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	var idMaps []*nat.Nat44DNat_DNatConfig_IdentityMapping
	oldData := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 0, 0,
			getTestNatStaticLocalIP("10.0.0.2", 0, 35)),
		getTestNatStaticMappingConfig(0, "", "10.0.0.2", 8000, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.3", 9000, 35),
			getTestNatStaticLocalIP("10.0.0.4", 9001, 65))),
		IdMappings: append(idMaps,
			getTestNatIdentityMappingConfig(0, "", "10.0.0.4", 9002, nat.Protocol_TCP))}
	newData := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 0, 0,
			getTestNatStaticLocalIP("10.0.0.2", 0, 35)),
		getTestNatStaticMappingConfig(0, "", "10.0.0.5", 8000, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.4", 9000, 0))),
		IdMappings: append(idMaps,
			getTestNatIdentityMappingConfig(0, "", "10.0.0.3", 9000, 0))}

	// Test configure static and identity mappings
	err := plugin.ConfigureDNat(oldData)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(oldData.Label)).To(BeTrue())
	id := ifplugin.GetStMappingIdentifier(oldData.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
	id = ifplugin.GetStMappingIdentifier(oldData.StMappings[1])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
	id = ifplugin.GetIDMappingIdentifier(oldData.IdMappings[0])
	Expect(plugin.IsDNatLabelIDMappingRegistered(id)).To(BeTrue())
	// Test modify static and identity mapping
	err = plugin.ModifyDNat(oldData, newData)
	Expect(err).To(BeNil())
	id = ifplugin.GetStMappingIdentifier(newData.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
	id = ifplugin.GetStMappingIdentifier(newData.StMappings[1])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
	id = ifplugin.GetIDMappingIdentifier(newData.IdMappings[0])
	Expect(plugin.IsDNatLabelIDMappingRegistered(id)).To(BeTrue())
}

// Configure and modify static and identity mappings with errors
func TestNatConfiguratorDNatModifyErrors(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{}) // Configure
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{
		Retval: 1,
	})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{
		Retval: 1,
	})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{
		Retval: 1,
	})
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{
		Retval: 1,
	})
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	var idMaps []*nat.Nat44DNat_DNatConfig_IdentityMapping
	oldData := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 0, 0,
			getTestNatStaticLocalIP("10.0.0.2", 0, 35))),
		IdMappings: append(idMaps,
			getTestNatIdentityMappingConfig(0, "", "10.0.0.4", 9002, nat.Protocol_TCP))}
	newData := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 0, 0,
			getTestNatStaticLocalIP("10.0.0.3", 0, 65))),
		IdMappings: append(idMaps,
			getTestNatIdentityMappingConfig(0, "", "10.0.0.3", 9000, 0))}

	// Test configure static and identity mappings
	err := plugin.ConfigureDNat(oldData)
	Expect(err).To(BeNil())
	// Test modify static and identity mapping
	err = plugin.ModifyDNat(oldData, newData)
	Expect(err).ToNot(BeNil())
}

//  Configure and delete static mapping
func TestNatConfiguratorDeleteDNatStaticMapping(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{}) // Configure
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelStaticMappingReply{}) // Delete
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 8000, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.1", 9000, 35)))}

	// Test configure DNAT static mapping
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
	// Test delete static mapping
	err = plugin.DeleteDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id = ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeFalse())
}

// Delete static mapping with invalid IP
func TestNatConfiguratorDeleteDNatStaticMappingInvalidIPError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "no-ip", 8000, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.1", 9000, 35)))}

	// Test delete static mapping with invalid interface
	err := plugin.DeleteDNat(data)
	Expect(err).ToNot(BeNil())
}

// Configure and delete static mapping with load balancer
func TestNatConfiguratorDeleteDNatStaticMappingLb(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelLbStaticMappingReply{}) // Configure
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelLbStaticMappingReply{}) // Delete
	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "test-dnat", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "10.0.0.1", 8000, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.2", 9000, 35),
			getTestNatStaticLocalIP("10.0.0.3", 9001, 65)))}

	// Test configure static mapping with load balancer
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeTrue())
	// Test delete static mapping with load balancer
	err = plugin.DeleteDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id = ifplugin.GetStMappingIdentifier(data.StMappings[0])
	Expect(plugin.IsDNatLabelStMappingRegistered(id)).To(BeFalse())
}

// Delete static mapping with load ballancer with invalid IP address
func TestNatConfiguratorDeleteDNatStaticMappingLbInvalidIPError(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var stMaps []*nat.Nat44DNat_DNatConfig_StaticMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "test-dnat", StMappings: append(stMaps,
		getTestNatStaticMappingConfig(0, "", "invalid-ip", 8000, nat.Protocol_TCP,
			getTestNatStaticLocalIP("10.0.0.2", 9000, 35),
			getTestNatStaticLocalIP("10.0.0.3", 9001, 65)))}

	// Test delete static mapping with load balancer with invalid IP
	err := plugin.DeleteDNat(data)
	Expect(err).ToNot(BeNil())
}

// Configure and delete NAT identity mapping
func TestNatConfiguratorDNatDeleteIdentityMapping(t *testing.T) {
	ctx, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{}) // Configure
	ctx.MockVpp.MockReply(&nat_api.Nat44AddDelIdentityMappingReply{}) // Delete
	// Data
	var idMaps []*nat.Nat44DNat_DNatConfig_IdentityMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "dNatLabel", IdMappings: append(idMaps,
		getTestNatIdentityMappingConfig(0, "", "10.0.0.1", 9000, nat.Protocol_TCP))}

	// Test configure identity mapping
	err := plugin.ConfigureDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeTrue())
	id := ifplugin.GetIDMappingIdentifier(data.IdMappings[0])
	Expect(plugin.IsDNatLabelIDMappingRegistered(id)).To(BeTrue())
	// Test delete identity mapping
	err = plugin.DeleteDNat(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsDNatLabelRegistered(data.Label)).To(BeFalse())
	id = ifplugin.GetIDMappingIdentifier(data.IdMappings[0])
	Expect(plugin.IsDNatLabelIDMappingRegistered(id)).To(BeFalse())
}

// Delete NAT identity mapping without ip address set
func TestNatConfiguratorDNatDeleteIdentityMappingNoInterfaceAndIP(t *testing.T) {
	_, connection, plugin, _ := natTestSetup(t)
	defer natTestTeardown(connection, plugin)

	// Data
	var idMaps []*nat.Nat44DNat_DNatConfig_IdentityMapping
	data := &nat.Nat44DNat_DNatConfig{Label: "test-dnat", IdMappings: append(idMaps,
		getTestNatIdentityMappingConfig(0, "", "", 8000, nat.Protocol_TCP))}

	// Test delete identity mapping without interface IP
	err := plugin.DeleteDNat(data)
	Expect(err).ToNot(BeNil())
}

/* NAT Test Setup */

func natTestSetup(t *testing.T) (*vppcallmock.TestCtx, *core.Connection, *ifplugin.NatConfigurator, ifaceidx.SwIfIndexRW) {
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
	swIfIndices := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "nat", nil))

	// Configurator
	plugin := &ifplugin.NatConfigurator{}
	err = plugin.Init(log, connection, swIfIndices)
	Expect(err).To(BeNil())

	return ctx, connection, plugin, swIfIndices
}

func natTestTeardown(connection *core.Connection, plugin *ifplugin.NatConfigurator) {
	connection.Disconnect()
	err := plugin.Close()
	Expect(err).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

/* NAT Test Data */

func getTestNatForwardingConfig(fwd bool) *nat.Nat44Global {
	return &nat.Nat44Global{Forwarding: fwd}
}

func getTestNatInterfaceConfig(name string, inside, output bool) *nat.Nat44Global_NatInterface {
	return &nat.Nat44Global_NatInterface{
		Name:          name,
		IsInside:      inside,
		OutputFeature: output,
	}
}

func getTestNatAddressPoolConfig(first, last string, vrf uint32, twn bool) *nat.Nat44Global_AddressPool {
	return &nat.Nat44Global_AddressPool{
		FirstSrcAddress: first,
		LastSrcAddress:  last,
		VrfId:           vrf,
		TwiceNat:        twn,
	}
}

func getTestNatStaticMappingConfig(vrf uint32, ifName, externalIP string, externalPort uint32, proto nat.Protocol, locals ...*nat.Nat44DNat_DNatConfig_StaticMapping_LocalIP) *nat.Nat44DNat_DNatConfig_StaticMapping {
	return &nat.Nat44DNat_DNatConfig_StaticMapping{
		ExternalInterface: ifName,
		ExternalIp:        externalIP,
		ExternalPort:      externalPort,
		LocalIps:          locals,
		Protocol:          proto,
	}
}

func getTestNatIdentityMappingConfig(vrf uint32, ifName, ip string, port uint32, proto nat.Protocol) *nat.Nat44DNat_DNatConfig_IdentityMapping {
	return &nat.Nat44DNat_DNatConfig_IdentityMapping{
		VrfId:              vrf,
		AddressedInterface: ifName,
		IpAddress:          ip,
		Port:               port,
		Protocol:           proto,
	}
}

func getTestNatStaticLocalIP(ip string, port, probability uint32) *nat.Nat44DNat_DNatConfig_StaticMapping_LocalIP {
	return &nat.Nat44DNat_DNatConfig_StaticMapping_LocalIP{
		LocalIp:     ip,
		LocalPort:   port,
		Probability: probability,
	}
}
