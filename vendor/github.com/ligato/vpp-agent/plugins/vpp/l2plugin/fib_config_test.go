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

package l2plugin_test

import (
	"fmt"
	"testing"
	"time"

	"git.fd.io/govpp.git/core"

	"git.fd.io/govpp.git/adapter/mock"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	l2Api "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

// FIB entries are configured asynchronously. In order to test it, callback function is used to notify test case that
// the request was processed
type mockCallback struct {
	doneChan chan error
}

// Done sends error if present
func (m *mockCallback) Done(err error) {
	m.doneChan <- err
}

/* FIB configurator init and close */

/* FIB configurator test cases */

// Configure FIB entry with all initial data available
func TestFIBConfiguratorAdd(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	_, meta, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
}

// Configure FIB with undefined mac address
func TestFIBConfiguratorAddWithoutPhysicalAddress(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, _, _ := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Data
	data := getTestFIB("if1", "bd1", "")
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).ToNot(BeNil())
}

// Configure FIB with undefined bridge domain
func TestFIBConfiguratorAddWithoutBd(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, _, _ := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Data
	data := getTestFIB("if1", "", "00:00:00:00:00:01")
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).ToNot(BeNil())
}

// Configure FIB with existing interface and bridge domain, but interface is not a part of bridge domain.
func TestFIBConfiguratorAddUntiedIfBd(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain. Bridge domain does not contain interface.
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{}, []string{}))
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Verify that entry was removed from 'del' cache before configuration
func TestFIBConfiguratorRemoveObsoleteDelCache(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Register to 'del' cache
	plugin.GetFibDelCacheIndexes().RegisterName("00:00:00:00:00:01", 1, data)
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	_, _, found := plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Validate correct caching if interface is not registered yet
func TestFIBConfiguratorMissingInterface(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, _, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain
	bdIndexes.RegisterName("bd1", 1, nil)
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Validate correct caching if bridge domain is not registered yet
func TestFIBConfiguratorMissingBridgeDomain(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, ifIndexes, _ := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Test modification of existing FIB
func TestFIBConfiguratorModify(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	oldData := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	newData := getTestFIB("if2", "bd2", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 2, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	bdIndexes.RegisterName("bd2", 2, getBdMetaWithInterfaces("bd2", []string{"if2"}, []string{"if2"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, oldData)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Test modify
	errs := blockingModify(plugin, oldData, newData, false)
	Expect(errs[0]).To(BeNil())
	Expect(errs[1]).To(BeNil())
	Expect(errs[2]).To(BeNil())
	_, _, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Test modification of existing FIB where old entry cannot be removed due to missing interface
func TestFIBConfiguratorModifyWithMissingOldInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	oldData := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	newData := getTestFIB("if2", "bd2", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 2, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	bdIndexes.RegisterName("bd2", 2, getBdMetaWithInterfaces("bd2", []string{"if2"}, []string{"if2"}))
	// Test configure FIB
	err, callBackErr := blockingAdd(plugin, oldData)
	Expect(err).To(BeNil())
	Expect(callBackErr).To(BeNil())
	// Remove old interface
	ifIndexes.UnregisterName("if1")
	// Test modify
	errs := blockingModify(plugin, oldData, newData, true)
	Expect(errs[0]).To(BeNil())
	Expect(errs[1]).To(BeNil())
	_, _, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Test modification of existing FIB where old entry cannot be removed due to missing bridge domain
func TestFIBConfiguratorModifyWithMissingOldBd(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	oldData := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	newData := getTestFIB("if2", "bd2", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 2, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	bdIndexes.RegisterName("bd2", 2, getBdMetaWithInterfaces("bd2", []string{"if2"}, []string{"if2"}))
	// Test configure FIB
	err, callBackErr := blockingAdd(plugin, oldData)
	Expect(err).To(BeNil())
	Expect(callBackErr).To(BeNil())
	// Remove old interface
	bdIndexes.UnregisterName("bd1")
	// Test modify
	errs := blockingModify(plugin, oldData, newData, true)
	Expect(errs[0]).To(BeNil())
	Expect(errs[1]).To(BeNil())
	_, _, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Test modification of existing FIB and simulate non-critical error while removing old entry
func TestFIBConfiguratorModifyOldError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{
		Retval: 1,
	})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	oldData := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	newData := getTestFIB("if2", "bd2", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 2, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	bdIndexes.RegisterName("bd2", 2, getBdMetaWithInterfaces("bd1", []string{"if2"}, []string{"if2"}))
	// Test configure FIB
	err, callBackErr := blockingAdd(plugin, oldData)
	Expect(err).To(BeNil())
	Expect(callBackErr).To(BeNil())
	// Test modify
	errs := blockingModify(plugin, oldData, newData, false)
	Expect(errs[0]).To(BeNil())
	Expect(errs[1]).ToNot(BeNil()) // expect error
	Expect(errs[2]).To(BeNil())
	_, _, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Test unregistering from 'add' cache while modification
func TestFIBConfiguratorModifyAndUnregisterAdd(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	oldData := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	newData := getTestFIB("if1", "bd2", "00:00:00:00:00:01")
	// Register, interface and bridge domain for modified data
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd2", 2, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(oldData, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Test modify
	errs := blockingModify(plugin, oldData, newData, true)
	Expect(errs[0]).To(BeNil())
	Expect(errs[1]).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Test modification of existing FIB while new interface is missing
func TestFIBConfiguratorModifyWithMissingNewInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	oldData := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	newData := getTestFIB("if2", "bd2", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	bdIndexes.RegisterName("bd2", 2, getBdMetaWithInterfaces("bd1", []string{"if2"}, []string{}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, oldData)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Test modify
	errs := blockingModify(plugin, oldData, newData, true)
	Expect(errs[0]).To(BeNil())
	Expect(errs[1]).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Test modification of existing FIB while new bridge domain is missing
func TestFIBConfiguratorModifyWithMissingNewBD(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	oldData := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	newData := getTestFIB("if2", "bd2", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, oldData)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Test modify
	errs := blockingModify(plugin, oldData, newData, true)
	Expect(errs[0]).To(BeNil())
	Expect(errs[1]).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Configures and Remvoes FIB entry
func TestFIBConfiguratorDelete(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	_, _, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Test delete FIB
	err, callbackErr = blockingDel(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	_, _, found = plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Verifies that the entry is removed from 'add' cache if deleted
func TestFIBConfiguratorDeleteFromAdd(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, ifIndexes, _ := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Test delete FIB
	err = plugin.Delete(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Verifies that the entry is correctly added to 'del' cache if unable to remove because of missing interface
func TestFIBConfiguratorDeleteMissingInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Unregister interface
	ifIndexes.UnregisterName("if1")
	// Test delete FIB
	callback := &mockCallback{}
	err = plugin.Delete(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Verifies that the entry is correctly added to 'del' cache if unable to remove because of missing bridge domain
func TestFIBConfiguratorDeleteMissingBD(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Unregister interface
	bdIndexes.UnregisterName("bd1")
	// Test delete FIB
	callback := &mockCallback{}
	err = plugin.Delete(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Configures FIB while interface is missing, then create it
func TestFIBConfiguratorResolveCreatedInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register bridge domain
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Register different interface
	ifIndexes.RegisterName("if2", 2, getTestInterface("if2", []string{"10.0.0.1/24"}))
	err = plugin.ResolveCreatedInterface("if2", 2, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Register interface
	ifIndexes.RegisterName("if1", 1, nil)
	err, callbackErr := blockingResolveCreatedInterface(plugin, "if1", 1)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Configures FIB while both interface and bridge domain is missing, then create interface
func TestFIBConfiguratorResolveCreatedInterfaceMissingBridgeDomain(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, ifIndexes, _ := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Register interface
	ifIndexes.RegisterName("if1", 1, nil)
	err = plugin.ResolveCreatedInterface("if1", 1, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Configures FIB while interface is missing, then create it (without metadata)
func TestFIBConfiguratorResolveCreatedInterfaceWithoutMeta(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register bridge domain
	bdIndexes.RegisterName("bd1", 1, nil)
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Remove metadata manually
	plugin.GetFibAddCacheIndexes().UpdateMetadata("00:00:00:00:00:01", nil)
	// Register interface
	ifIndexes.RegisterName("if1", 1, nil)
	err = plugin.ResolveCreatedInterface("if1", 1, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Removes FIB cached as un-removable because of missing interface
func TestFIBConfiguratorResolveCreatedInterfaceDeleteFIB(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Unregister interface
	ifIndexes.UnregisterName("if1")
	// Attempt to remove FIB
	callback := &mockCallback{}
	err = plugin.Delete(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Register different interface
	ifIndexes.RegisterName("if2", 2, nil)
	err = plugin.ResolveCreatedInterface("if2", 2, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Re-register interface
	ifIndexes.RegisterName("if1", 1, nil)
	err, callbackErr = blockingResolveCreatedInterface(plugin, "if1", 1)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Removes FIB cached as un-removable because of missing interface while 'del' throws error
func TestFIBConfiguratorResolveCreatedInterfaceDeleteFIBError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{
		Retval: 1,
	})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Unregister interface
	ifIndexes.UnregisterName("if1")
	// Attempt to remove FIB
	callback := &mockCallback{}
	err = plugin.Delete(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Re-register interface
	ifIndexes.RegisterName("if1", 1, nil)
	err, callbackErr = blockingResolveCreatedInterface(plugin, "if1", 1)
	Expect(err).To(BeNil())
	Expect(callbackErr).ToNot(BeNil()) // expect error
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Removes FIB cached as un-removable because of missing interface (without metadata)
func TestFIBConfiguratorResolveCreatedInterfaceDeleteFIBWithoutMeta(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Unregister interface
	ifIndexes.UnregisterName("if1")
	// Attempt to remove FIB
	callback := &mockCallback{}
	err = plugin.Delete(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Remove metadata manually
	plugin.GetFibDelCacheIndexes().UpdateMetadata("00:00:00:00:00:01", nil)
	// Re-register interface
	ifIndexes.RegisterName("if1", 1, nil)
	err = plugin.ResolveCreatedInterface("if1", 1, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Verify FIB configurator behavior if interface is removed (coverage test, ResolveDeletedInterface is
// a kind of informative)
func TestFIBConfiguratorResolveDeletedInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data1 := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	data2 := getTestFIB("if2", "bd2", "00:00:00:00:00:02")
	// Register bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	bdIndexes.RegisterName("bd2", 1, getBdMetaWithInterfaces("bd2", []string{"if2"}, []string{"if2"}))
	// Test configure FIB 1
	callback := &mockCallback{}
	err, callbackErr := blockingAdd(plugin, data1)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Test configure FIB 2
	err, callbackErr = blockingAdd(plugin, data2)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Manually remove metadata from FIB 1
	plugin.GetFibIndexes().UpdateMetadata("00:00:00:00:00:01", nil)
	// Register different interface 1
	ifIndexes.RegisterName("if3", 3, nil)
	err = plugin.ResolveDeletedInterface("if3", 3, callback.Done)
	Expect(err).To(BeNil())
	// Register different interface 2
	ifIndexes.RegisterName("if2", 2, nil)
	err = plugin.ResolveDeletedInterface("if2", 2, callback.Done)
	Expect(err).To(BeNil())
	// Register interface
	ifIndexes.RegisterName("if1", 1, nil)
	err = plugin.ResolveDeletedInterface("if", 1, callback.Done)
	Expect(err).To(BeNil())
}

// Configures FIB while bridge domain is missing, then create it
func TestFIBConfiguratorResolveCreatedBridgeDomain(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Register different bridge domain
	bdIndexes.RegisterName("bd2", 2, nil)
	err = plugin.ResolveCreatedBridgeDomain("bd2", 2, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Register bridge domain
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	err, callbackErr := blockingResolveCreatedBridgeDomain(plugin, "bd1", 1)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Configures FIB while both interface and bridge domain is missing, then create bridge domain
func TestFIBConfiguratorResolveCreatedBridgeDomainMissingInterface(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, _, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Register bridge domain
	bdIndexes.RegisterName("bd1", 1, nil)
	err = plugin.ResolveCreatedBridgeDomain("bd1", 1, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Configures FIB while bridge domain is missing, then create it (without metadata)
func TestFIBConfiguratorResolveCreatedBDWithoutMeta(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface
	ifIndexes.RegisterName("if1", 1, nil)
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Remove metadata manually
	plugin.GetFibAddCacheIndexes().UpdateMetadata("00:00:00:00:00:01", nil)
	// Register bridge domain
	bdIndexes.RegisterName("bd1", 1, nil)
	err = plugin.ResolveCreatedBridgeDomain("bd1", 1, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Removes FIB cached as un-removable because of missing bridge domain
func TestFIBConfiguratorResolveCreatedBDDeleteFIB(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Unregister bridge domain
	bdIndexes.UnregisterName("bd1")
	// Attempt to remove FIB
	callback := &mockCallback{}
	err = plugin.Delete(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Register different bridge domain
	bdIndexes.RegisterName("bd2", 2, getBdMetaWithInterfaces("bd1", []string{"if2"}, []string{"if2"}))
	err = plugin.ResolveCreatedBridgeDomain("bd2", 2, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Re-register bridge domain
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	err, callbackErr = blockingResolveCreatedBridgeDomain(plugin, "bd1", 1)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Removes FIB cached as un-removable because of missing bridge domain while 'del' throws error
func TestFIBConfiguratorResolveCreatedBDDeleteFIBError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{
		Retval: 1,
	})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Unregister bridge domain
	bdIndexes.UnregisterName("bd1")
	// Attempt to remove FIB
	callback := &mockCallback{}
	err = plugin.Delete(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Re-register bridge domain
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	err, callbackErr = blockingResolveCreatedBridgeDomain(plugin, "bd1", 1)
	Expect(err).To(BeNil())
	Expect(callbackErr).ToNot(BeNil()) // expect error
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Removes FIB cached as un-removable because of missing bridge domain (without metadata)
func TestFIBConfiguratorResolveCreatedBDDeleteFIBWithoutMeta(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	// Test configure FIB
	err, callbackErr := blockingAdd(plugin, data)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Unregister bridge domain
	bdIndexes.UnregisterName("bd1")
	// Attempt to remove FIB
	callback := &mockCallback{}
	err = plugin.Delete(data, callback.Done)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	// Remove metadata manually
	plugin.GetFibDelCacheIndexes().UpdateMetadata("00:00:00:00:00:01", nil)
	// Re-register bridge domain
	ifIndexes.RegisterName("bd1", 1, nil)
	err = plugin.ResolveCreatedBridgeDomain("bd1", 1, callback.Done)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetFibDelCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
}

// Configure FIB with existing interface and bridge domain, but interface is not a part of bridge domain.
// Then update the bridge domain.
func TestFIBConfiguratorResolveUpdatedBridgeDomain(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	// Register interface and bridge domain. Bridge domain does not contain interface.
	ifIndexes.RegisterName("if1", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{}, []string{}))
	// Test configure FIB
	callback := &mockCallback{}
	err = plugin.Add(data, callback.Done)
	Expect(err).To(BeNil())
	// Update BD metadata
	bdIndexes.UpdateMetadata("bd1", getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	err, callbackErr := blockingResolveUpdatedBridgeDomain(plugin, "bd1", 1)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	_, _, found := plugin.GetFibIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeTrue())
	_, _, found = plugin.GetFibAddCacheIndexes().LookupIdx("00:00:00:00:00:01")
	Expect(found).To(BeFalse())
}

// Verify FIB configurator behavior if bridge domain is removed (coverage test, ResolveDeletedBridgeDomain is
// a kind of informative)
func TestFIBConfiguratorResolveDeletedBridgeDomain(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes, bdIndexes := fibTestSetup(t)
	defer fibTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	ctx.MockVpp.MockReply(&l2Api.L2fibAddDelReply{})
	// Data
	data1 := getTestFIB("if1", "bd1", "00:00:00:00:00:01")
	data2 := getTestFIB("if2", "bd2", "00:00:00:00:00:02")
	// Register bridge domain
	ifIndexes.RegisterName("if1", 1, nil)
	ifIndexes.RegisterName("if2", 1, nil)
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	bdIndexes.RegisterName("bd2", 2, getBdMetaWithInterfaces("bd2", []string{"if2"}, []string{"if2"}))
	// Test configure FIB 1
	callback := &mockCallback{}
	err, callbackErr := blockingAdd(plugin, data1)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Test configure FIB 2
	err, callbackErr = blockingAdd(plugin, data2)
	Expect(err).To(BeNil())
	Expect(callbackErr).To(BeNil())
	// Manually remove metadata from FIB 1
	plugin.GetFibIndexes().UpdateMetadata("00:00:00:00:00:01", nil)
	// Register different bridge domain 1
	bdIndexes.RegisterName("bd3", 3, getBdMetaWithInterfaces("bd3", []string{}, []string{}))
	err = plugin.ResolveDeletedBridgeDomain("bd3", 3, callback.Done)
	Expect(err).To(BeNil())
	// Register different bridge domain 2
	bdIndexes.RegisterName("bd2", 2, getBdMetaWithInterfaces("bd2", []string{}, []string{}))
	err = plugin.ResolveDeletedBridgeDomain("bd2", 2, callback.Done)
	Expect(err).To(BeNil())
	// Register bridge domain
	bdIndexes.RegisterName("bd1", 1, getBdMetaWithInterfaces("bd1", []string{"if1"}, []string{"if1"}))
	err = plugin.ResolveDeletedBridgeDomain("bd", 1, callback.Done)
	Expect(err).To(BeNil())
}

/* FIB crud auxiliary methods */

// Test function calling 'Add' in FIB configurator. The custom method waits on callback according to input parameters
func blockingAdd(plugin *l2plugin.FIBConfigurator, data *l2.FibTable_FibEntry) (error, error) {
	// Init callback with unbuffered channel
	callback := getCallback(0)
	err := plugin.Add(data, callback.Done)
	// Wait for async reply
	resultErr := <-callback.doneChan

	return err, resultErr
}

// Test function calling 'Modify' in FIB configurator. Modify uses callback twice; while attempting to remove old entry
// and while adding a new one. Also the removal/adding may be omitted, in such a case it expects only first value
func blockingModify(plugin *l2plugin.FIBConfigurator, oldData, newData *l2.FibTable_FibEntry, expectOne bool) (errs []error) {
	// Init callback with single-value buffer
	callback := getCallback(1)
	errs = append(errs, plugin.Modify(oldData, newData, callback.Done))
	// Wait for async replies
	var callbackErr error
loop:
	for {
		select {
		case resultErr := <-callback.doneChan:
			errs = append(errs, resultErr)
			if expectOne {
				break loop
			} else {
				expectOne = true
			}
		case <-time.After(2 * time.Second):
			callbackErr = fmt.Errorf("FIB call timed out")
			break loop
		}
	}
	Expect(callbackErr).To(BeNil())

	return errs
}

// Test function calling 'Delete' in FIB configurator. The custom method waits on callback according to input parameters
func blockingDel(plugin *l2plugin.FIBConfigurator, data *l2.FibTable_FibEntry) (error, error) {
	// Init callback with unbuffered channel
	callback := getCallback(0)
	err := plugin.Delete(data, callback.Done)
	// Wait for async reply
	resultErr := <-callback.doneChan

	return err, resultErr
}

// Test function calling 'ResolveCreatedInterface' in FIB configurator. The custom method waits on callback.
func blockingResolveCreatedInterface(plugin *l2plugin.FIBConfigurator, ifName string, ifIdx uint32) (error, error) {
	// Init callback with unbuffered channel
	callback := getCallback(0)
	err := plugin.ResolveCreatedInterface(ifName, ifIdx, callback.Done)
	// Wait for async reply
	resultErr := <-callback.doneChan

	return err, resultErr
}

// Test function calling 'ResolveCreatedBridgeDomain' in FIB configurator. The custom method waits on callback.
func blockingResolveCreatedBridgeDomain(plugin *l2plugin.FIBConfigurator, bdName string, bdIdx uint32) (error, error) {
	// Init callback with unbuffered channel
	callback := getCallback(0)
	err := plugin.ResolveCreatedBridgeDomain(bdName, bdIdx, callback.Done)
	// Wait for async reply
	resultErr := <-callback.doneChan

	return err, resultErr
}

// Test function calling 'ResolveUpdatedBridgeDomain' in FIB configurator. The custom method waits on callback.
func blockingResolveUpdatedBridgeDomain(plugin *l2plugin.FIBConfigurator, bdName string, bdIdx uint32) (error, error) {
	// Init callback with unbuffered channel
	callback := getCallback(0)
	err := plugin.ResolveUpdatedBridgeDomain(bdName, bdIdx, callback.Done)
	// Wait for async reply
	resultErr := <-callback.doneChan

	return err, resultErr
}

/* FIB Test Setup */

func fibTestSetup(t *testing.T) (*vppcallmock.TestCtx, *core.Connection, *l2plugin.FIBConfigurator, ifaceidx.SwIfIndexRW, l2idx.BDIndexRW) {
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
	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "fib-if", nil))
	bdIndexes := l2idx.NewBDIndex(nametoidx.NewNameToIdx(log, "fib-bd", nil))
	// Configurator
	plugin := &l2plugin.FIBConfigurator{}
	err = plugin.Init(logging.ForPlugin("test-log"), connection, swIfIndexes, bdIndexes)
	Expect(err).To(BeNil())

	return ctx, connection, plugin, swIfIndexes, bdIndexes
}

func getCallback(buffer int) *mockCallback {
	return &mockCallback{
		doneChan: make(chan error, buffer),
	}
}

func fibTestTeardown(connection *core.Connection, plugin *l2plugin.FIBConfigurator) {
	connection.Disconnect()
	Expect(plugin.Close()).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

/* FIB Test Data */

func getTestFIB(ifName, bdName, mac string) *l2.FibTable_FibEntry {
	return &l2.FibTable_FibEntry{
		PhysAddress:             mac,
		BridgeDomain:            bdName,
		Action:                  l2.FibTable_FibEntry_FORWARD,
		OutgoingInterface:       ifName,
		StaticConfig:            true,
		BridgedVirtualInterface: false,
	}
}

func getBdMetaWithInterfaces(bdName string, ifs, configured []string) *l2idx.BdMetadata {
	var bdIfs []*l2.BridgeDomains_BridgeDomain_Interfaces
	for _, bdIf := range ifs {
		bdIfs = append(bdIfs, &l2.BridgeDomains_BridgeDomain_Interfaces{
			Name: bdIf,
		})
	}

	return &l2idx.BdMetadata{
		BridgeDomain: &l2.BridgeDomains_BridgeDomain{
			Name:       bdName,
			Interfaces: bdIfs,
		},
		ConfiguredInterfaces: configured,
	}
}
