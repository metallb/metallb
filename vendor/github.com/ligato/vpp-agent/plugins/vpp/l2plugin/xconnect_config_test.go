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
	"testing"

	"git.fd.io/govpp.git/core"

	"git.fd.io/govpp.git/adapter/mock"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	l2api "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

/* XConnect configurator init and close */

/* XConnect configurator test cases */

// Basic configuration of cross connect pair
func TestConfigureXConnectPair(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Configure cross connect pair while receive interface is missing, check correct caching
func TestConfigureXConnectPairMissingRcIf(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register interfaces
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, meta, found := plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Configure cross connect pair while transmit interface is missing, check correct caching
func TestConfigureXConnectPairMissingTxIf(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, meta, found := plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Test return value error while configuring cross connect
func TestConfigureXConnectPairAddErr(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{
		Retval: 1,
	})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).ToNot(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Test error case where receive interface is invalid (not set)
func TestConfigureXConnectPairInvalidRxConfig(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, _ := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Data
	data := getTestXConnect("", "txIf")
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).ToNot(BeNil())
}

// Test error case where transmit interface is invalid (not set)
func TestConfigureXConnectPairInvalidTxConfig(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, _ := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Data
	data := getTestXConnect("rcIf", "")
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).ToNot(BeNil())
}

// Test error case where receive and transmit interfaces are the same
func TestConfigureXConnectPairInvalidIfConfig(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, _ := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Data
	data := getTestXConnect("rcIf", "rcIf")
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).ToNot(BeNil())
}

// Test cross connect modification of transmit interface
func TestModifyXConnectPair(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	oldData := getTestXConnect("rcIf", "txIf1")
	newData := getTestXConnect("rcIf", "txIf2")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf1", 1, getTestInterface("txIf1", []string{"10.0.0.2/24"}))
	ifIndexes.RegisterName("txIf2", 1, getTestInterface("txIf2", []string{"10.0.0.3/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(oldData)
	Expect(err).To(BeNil())
	err = plugin.ModifyXConnectPair(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.TransmitInterface).To(BeEquivalentTo("txIf2"))
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Test cross connect modification while receive interface disappear
func TestModifyXConnectPairMissingRcIf(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	oldData := getTestXConnect("rcIf", "txIf1")
	newData := getTestXConnect("rcIf", "txIf2")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf1", 1, getTestInterface("txIf1", []string{"10.0.0.2/24"}))
	ifIndexes.RegisterName("txIf2", 1, getTestInterface("txIf2", []string{"10.0.0.3/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(oldData)
	Expect(err).To(BeNil())
	// Unregister rcIf
	ifIndexes.UnregisterName("rcIf")
	// Test Modify
	err = plugin.ModifyXConnectPair(newData, oldData)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Test modify cross connect while new transmit interface does not exist yet
func TestModifyXConnectPairMissingTxIfRemoveOld(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	oldData := getTestXConnect("rcIf", "txIf1")
	newData := getTestXConnect("rcIf", "txIf2")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf1", 1, getTestInterface("txIf1", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(oldData)
	Expect(err).To(BeNil())
	// Test Modify
	err = plugin.ModifyXConnectPair(newData, oldData)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Test cross connect modify where old transmit interface does not exist and cannot be removed yet
func TestModifyXConnectPairMissingTxIfKeepOld(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	oldData := getTestXConnect("rcIf", "txIf1")
	newData := getTestXConnect("rcIf", "txIf2")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf1", 1, getTestInterface("txIf1", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(oldData)
	Expect(err).To(BeNil())
	// Unregister txIf1
	ifIndexes.UnregisterName("txIf1")
	// Test Modify
	err = plugin.ModifyXConnectPair(newData, oldData)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Test modify with incorrect receive interface
func TestModifyXConnectPairInvalidRxConfig(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, _ := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Data
	oldData := getTestXConnect("rcIf", "txIf1")
	newData := getTestXConnect("", "txIf2")
	// Test configure XConnect
	err = plugin.ModifyXConnectPair(newData, oldData)
	Expect(err).ToNot(BeNil())
}

// Test removal of cross connect pair
func TestDeleteXConnectPair(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	// Delete
	err = plugin.DeleteXConnectPair(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Test removal of cross connect pair where receive interface was already removed
func TestDeleteXConnectPairMissingRcIf(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	// Unregister RcIf
	ifIndexes.UnregisterName("rcIf")
	// Delete
	err = plugin.DeleteXConnectPair(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Test removal of cross connect where transmit interface was already removed
func TestDeleteXConnectPairMissingTxIf(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	// Unregister RcIf
	ifIndexes.UnregisterName("txIf")
	// Delete
	err = plugin.DeleteXConnectPair(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
}

// Test cross connect pair modify error during replacing of the old value
func TestDeleteXConnectPairError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{
		Retval: 1,
	})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register interfaces
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	// Delete
	err = plugin.DeleteXConnectPair(data)
	Expect(err).ToNot(BeNil())
}

// Test cross connect removal with invalid receive interface config
func TestDeleteXConnectPairInvalidRxConfig(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin, _ := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Data
	data := getTestXConnect("", "txIf")
	// Test configure XConnect
	err = plugin.DeleteXConnectPair(data)
	Expect(err).ToNot(BeNil())
}

// Configure cross connect and create (register) interfaces later
func TestConfigureXConnectPairResolveCreatedInterfaceAdd(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, meta, found := plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	// Register first
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	err = plugin.ResolveCreatedInterface("rcIf")
	Expect(err).To(BeNil())
	_, _, found = plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	// Register second
	ifIndexes.RegisterName("txIf", 2, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	err = plugin.ResolveCreatedInterface("txIf")
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Configure cross connect and create (register) interfaces later, simulate creation error
func TestConfigureXConnectPairResolveCreatedInterfaceAddError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{
		Retval: 1,
	})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, meta, found := plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	// Register first
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	err = plugin.ResolveCreatedInterface("rcIf")
	Expect(err).To(BeNil())
	_, _, found = plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	// Register second
	ifIndexes.RegisterName("txIf", 2, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	err = plugin.ResolveCreatedInterface("txIf")
	Expect(err).ToNot(BeNil())
}

// Add cross connect, then unregister (remove) transmit interface. Delete cross connect and re-register
// transmit interface. Verify correct internal memory state after each step.
func TestConfigureXConnectPairResolveCreatedInterfaceDel(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	// Unregister txIf (create un-removable XConnect)
	ifIndexes.UnregisterName("txIf")
	// Delete
	err = plugin.DeleteXConnectPair(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	// Register txIf
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	err = plugin.ResolveCreatedInterface("txIf")
	Expect(err).To(BeNil())
	_, _, found = plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
}

// Same as before but add error while deleting.
func TestConfigureXConnectPairResolveCreatedInterfaceDelError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{
		Retval: 1,
	})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Test configure XConnect
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	// Unregister txIf (create un-removable XConnect)
	ifIndexes.UnregisterName("txIf")
	// Delete
	err = plugin.DeleteXConnectPair(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcDelCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
	// Register txIf
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	err = plugin.ResolveCreatedInterface("txIf")
	Expect(err).ToNot(BeNil())
}

// Verify behavior of cross connect while receive interface is removed
func TestConfigureXConnectPairResolveDeletedRcInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin, ifIndexes := xcTestSetup(t)
	defer xcTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&l2api.SwInterfaceSetL2XconnectReply{})
	// Data
	data := getTestXConnect("rcIf", "txIf")
	// Register
	ifIndexes.RegisterName("rcIf", 1, getTestInterface("rcIf", []string{"10.0.0.1/24"}))
	ifIndexes.RegisterName("txIf", 1, getTestInterface("txIf", []string{"10.0.0.2/24"}))
	// Configure
	err = plugin.ConfigureXConnectPair(data)
	Expect(err).To(BeNil())
	ifIndexes.UnregisterName("rcIf")
	err = plugin.ResolveDeletedInterface("rcIf")
	Expect(err).To(BeNil())
	_, _, found := plugin.GetXcIndexes().LookupIdx("rcIf")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetXcAddCache().LookupIdx("rcIf")
	Expect(found).To(BeTrue())
}

/* XConnect Test Setup */

func xcTestSetup(t *testing.T) (*vppcallmock.TestCtx, *core.Connection, *l2plugin.XConnectConfigurator, ifaceidx.SwIfIndexRW) {
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
	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "xc-if", nil))
	// Configurator
	plugin := &l2plugin.XConnectConfigurator{}
	err = plugin.Init(log, connection, swIfIndexes)
	Expect(err).To(BeNil())

	return ctx, connection, plugin, swIfIndexes
}

func xcTestTeardown(connection *core.Connection, plugin *l2plugin.XConnectConfigurator) {
	connection.Disconnect()
	err := plugin.Close()
	Expect(err).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

/* XConnect Test Data */

func getTestXConnect(rxIfName, txIfName string) *l2.XConnectPairs_XConnectPair {
	return &l2.XConnectPairs_XConnectPair{
		ReceiveInterface:  rxIfName,
		TransmitInterface: txIfName,
	}
}

func getTestInterface(name string, ip []string) *interfaces.Interfaces_Interface {
	return &interfaces.Interfaces_Interface{
		Name:        name,
		Enabled:     true,
		Type:        interfaces.InterfaceType_MEMORY_INTERFACE,
		IpAddresses: ip,
	}
}
