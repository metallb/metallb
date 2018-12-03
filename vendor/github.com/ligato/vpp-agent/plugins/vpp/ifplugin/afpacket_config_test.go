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

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	ap_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/af_packet"
	if_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

/* AF_PACKET test cases */

// Configure af packet interface with unavailable host
func TestAfPacketConfigureHostNotAvail(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Data
	data := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")

	// Test configure af packet with host unavailable
	swIfIdx, pending, err := plugin.ConfigureAfPacketInterface(data)
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeZero())
	Expect(pending).To(BeTrue())
	exists, pending, cachedData := plugin.GetAfPacketStatusByName("if1")
	Expect(exists).To(BeTrue())
	Expect(pending).To(BeTrue())
	Expect(cachedData).ToNot(BeNil())
	exists, pending, cachedData = plugin.GetAfPacketStatusByHost("host1")
	Expect(exists).To(BeTrue())
	Expect(pending).To(BeTrue())
	Expect(cachedData).ToNot(BeNil())
}

// Configure af packet interface
func TestAfPacketConfigureHostAvail(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{
		SwIfIndex: 2,
	})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")

	// Test af packet
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	swIfIdx, pending, err := plugin.ConfigureAfPacketInterface(data)
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(2))
	Expect(pending).To(BeFalse())
	exists, pending, cachedData := plugin.GetAfPacketStatusByName("if1")
	Expect(exists).To(BeTrue())
	Expect(pending).To(BeFalse())
	Expect(cachedData).ToNot(BeNil())
	exists, pending, cachedData = plugin.GetAfPacketStatusByHost("host1")
	Expect(exists).To(BeTrue())
	Expect(pending).To(BeFalse())
	Expect(cachedData).ToNot(BeNil())
}

// Configure af packet with error reply from VPP API
func TestAfPacketConfigureHostAvailError(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{
		Retval:    1,
		SwIfIndex: 2,
	})
	// Data
	data := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")

	// Test configure af packet with return value != 0
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	swIfIdx, pending, err := plugin.ConfigureAfPacketInterface(data)
	Expect(err).ToNot(BeNil())
	Expect(swIfIdx).To(BeZero())
	Expect(pending).To(BeTrue())
	exists, pending, cachedData := plugin.GetAfPacketStatusByName("if1")
	Expect(exists).To(BeTrue())
	Expect(cachedData).ToNot(BeNil())
	exists, pending, cachedData = plugin.GetAfPacketStatusByHost("host1")
	Expect(exists).To(BeTrue())
	Expect(cachedData).ToNot(BeNil())
}

// Configure af packet as incorrect interface type
func TestAfPacketConfigureIncorrectTypeError(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Data
	data := getTestAfPacketData("host1", []string{"10.0.0.1/24"}, "host1")
	data.Type = interfaces.InterfaceType_SOFTWARE_LOOPBACK

	// Test configure af packet with incorrect type
	swIfIdx, pending, err := plugin.ConfigureAfPacketInterface(data)
	Expect(err).ToNot(BeNil())
	Expect(swIfIdx).To(BeZero())
	Expect(pending).To(BeFalse())
	exists, _, _ := plugin.GetAfPacketStatusByName("if1")
	Expect(exists).To(BeFalse())
	exists, _, _ = plugin.GetAfPacketStatusByHost("host1")
	Expect(exists).To(BeFalse())
}

// Call af packet modification which causes recreation of the interface
func TestAfPacketModifyRecreateChangedHost(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	oldData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")
	newData := getTestAfPacketData("if1", []string{"10.0.0.2/24"}, "host2")

	// Test configure initial af packet data
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	swIfIdx, pending, err := plugin.ConfigureAfPacketInterface(oldData)
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	Expect(pending).To(BeFalse())
	// Test modify af packet
	recreate, err := plugin.ModifyAfPacketInterface(newData, oldData)
	Expect(err).To(BeNil())
	Expect(recreate).To(BeTrue())
}

// Test modify pending af packet interface
func TestAfPacketModifyRecreatePending(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)

	defer afPacketTestTeardown(ctx)
	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	oldData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")
	newData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")

	// Test configure initial af packet data
	_, pending, err := plugin.ConfigureAfPacketInterface(oldData)
	Expect(err).To(BeNil())
	Expect(pending).To(BeTrue())
	// Test modify
	recreate, err := plugin.ModifyAfPacketInterface(newData, oldData)
	Expect(err).To(BeNil())
	Expect(recreate).To(BeTrue())
}

// Modify recreate of af packet interface which was not found
func TestAfPacketModifyRecreateNotFound(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Data
	oldData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")
	newData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host2")

	// Test af packet modify
	recreate, err := plugin.ModifyAfPacketInterface(newData, oldData)
	Expect(err).To(BeNil())
	Expect(recreate).To(BeTrue())
}

// Modify af packet interface without recreation
func TestAfPacketModifyNoRecreate(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	oldData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")
	newData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")

	// Test configure initial data
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	swIfIdx, pending, err := plugin.ConfigureAfPacketInterface(oldData)
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	Expect(pending).To(BeFalse())
	// Test modify
	recreate, err := plugin.ModifyAfPacketInterface(newData, oldData)
	Expect(err).To(BeNil())
	Expect(recreate).To(BeFalse())
	exists, pending, cachedData := plugin.GetAfPacketStatusByName("if1")
	Expect(exists).To(BeTrue())
	Expect(pending).To(BeFalse())
	Expect(cachedData).ToNot(BeNil())
	exists, pending, cachedData = plugin.GetAfPacketStatusByHost("host1")
	Expect(exists).To(BeTrue())
	Expect(pending).To(BeFalse())
	Expect(cachedData).ToNot(BeNil())
}

// Modify af packet with incorrect interface type
func TestAfPacketModifyIncorrectType(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	oldData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")
	newData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host2")
	newData.Type = interfaces.InterfaceType_SOFTWARE_LOOPBACK

	// Test configure initial data
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	swIfIdx, pending, err := plugin.ConfigureAfPacketInterface(oldData)
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	Expect(pending).To(BeFalse())
	// Test modify with incorrect type
	_, err = plugin.ModifyAfPacketInterface(newData, oldData)
	Expect(err).ToNot(BeNil())
}

// Af packet delete
func TestAfPacketDelete(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{ // Create
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&ap_api.AfPacketDeleteReply{}) // Delete
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")

	// Test configure initial af packet data
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	swIfIdx, pending, err := plugin.ConfigureAfPacketInterface(data)
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	Expect(pending).To(BeFalse())
	exists, pending, cachedData := plugin.GetAfPacketStatusByName("if1")
	Expect(exists).To(BeTrue())
	Expect(pending).To(BeFalse())
	Expect(cachedData).ToNot(BeNil())
	exists, pending, cachedData = plugin.GetAfPacketStatusByHost("host1")
	Expect(exists).To(BeTrue())
	Expect(pending).To(BeFalse())
	Expect(cachedData).ToNot(BeNil())
	// Test af packet delete
	err = plugin.DeleteAfPacketInterface(data, 1)
	Expect(err).To(BeNil())
	exists, _, _ = plugin.GetAfPacketStatusByName("if1")
	Expect(exists).To(BeFalse())
	exists, _, _ = plugin.GetAfPacketStatusByHost("host1")
	Expect(exists).To(BeFalse())
}

// Delete af packet with incorrect interface type data
func TestAfPacketDeleteIncorrectType(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")
	modifiedData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")
	modifiedData.Type = interfaces.InterfaceType_SOFTWARE_LOOPBACK

	// Test configure initial af packet
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	swIfIdx, pending, err := plugin.ConfigureAfPacketInterface(data)
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	Expect(pending).To(BeFalse())
	// Test delete with incorrect type
	err = plugin.DeleteAfPacketInterface(modifiedData, 1)
	Expect(err).ToNot(BeNil())
}

// Register new linux interface and test af packet behaviour
func TestAfPacketNewLinuxInterfaceHostFound(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")

	// Test registered linux interface
	_, pending, err := plugin.ConfigureAfPacketInterface(data)
	Expect(err).To(BeNil())
	Expect(pending).To(BeTrue())
	config, err := plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	Expect(err).To(BeNil())
	Expect(config).ToNot(BeNil())
	Expect(config.Afpacket.HostIfName).To(BeEquivalentTo("host1"))
	Expect(plugin.GetHostInterfacesEntry("host1")).To(BeTrue())
}

// Register new linux interface while af packet is not pending. Note: this is a case which should NOT happen
func TestAfPacketNewLinuxInterfaceHostNotPending(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&ap_api.AfPacketDeleteReply{})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")

	// Test registered linux interface
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	_, pending, err := plugin.ConfigureAfPacketInterface(data)
	Expect(err).To(BeNil())
	Expect(pending).To(BeFalse())
	config, err := plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	Expect(err).To(BeNil())
	Expect(config).ToNot(BeNil())
	Expect(config.Afpacket.HostIfName).To(BeEquivalentTo("host1"))
	Expect(plugin.GetHostInterfacesEntry("host1")).To(BeTrue())
}

// Test new linux interface which is not a host
func TestAfPacketNewLinuxInterfaceHostNotFound(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	Expect(plugin.GetHostInterfacesEntry("host1")).To(BeFalse())

	// Test registered linux interface
	config, err := plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	Expect(err).To(BeNil())
	Expect(config).To(BeNil())
	Expect(plugin.GetHostInterfacesEntry("host1")).To(BeTrue())
}

// Test new linux interface while linux plugin is not available
func TestAfPacketNewLinuxInterfaceNoLinux(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	// Logger
	log := logrus.DefaultLogger()
	log.SetLevel(logging.DebugLevel)
	// Interface indices
	swIfIndices := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "afpacket", nil))
	// Configurator
	plugin := &ifplugin.AFPacketConfigurator{}
	ifHandler := vppcalls.NewIfVppHandler(ctx.MockChannel, log)
	err := plugin.Init(log, ifHandler, nil, swIfIndices)
	Expect(err).To(BeNil())
	// Test registered linux interface
	config, err := plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	Expect(err).To(BeNil())
	Expect(config).To(BeNil())
}

// Un-register linux interface
func TestAfPacketDeletedLinuxInterface(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&ap_api.AfPacketDeleteReply{})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")
	// Prepare
	_, pending, err := plugin.ConfigureAfPacketInterface(data)
	Expect(err).To(BeNil())
	Expect(pending).To(BeTrue())
	// Test un-registered linux interface
	plugin.ResolveDeletedLinuxInterface("host1", "host1", 1)
	exists, _, _ := plugin.GetAfPacketStatusByHost("host1")
	Expect(exists).To(BeTrue())
	exists, _, _ = plugin.GetAfPacketStatusByName("host1")
	Expect(exists).To(BeFalse())
	Expect(plugin.GetHostInterfacesEntry("host1")).To(BeFalse())
}

// Un-register linux interface while host is not found
func TestAfPacketDeletedLinuxInterfaceHostNotFound(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Prepare
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	// Test un-registered linux interface
	plugin.ResolveDeletedLinuxInterface("host1", "host1", 1)
	Expect(plugin.GetHostInterfacesEntry("host1")).To(BeFalse())
}

// Un-register linux interface with linux plugin not initialized
func TestAfPacketDeleteLinuxInterfaceNoLinux(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	// Logger
	log := logrus.DefaultLogger()
	log.SetLevel(logging.DebugLevel)
	// Interface indices
	swIfIndices := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "afpacket", nil))
	// Configurator
	plugin := &ifplugin.AFPacketConfigurator{}
	ifHandler := vppcalls.NewIfVppHandler(ctx.MockChannel, log)
	err := plugin.Init(log, ifHandler, nil, swIfIndices)
	Expect(err).To(BeNil())
	// Prepare
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 1)
	// Test un-registered linux interface
	plugin.ResolveDeletedLinuxInterface("host1", "host1", 1)
	Expect(plugin.GetHostInterfacesEntry("host1")).To(BeFalse())
	err = safeclose.Close(ctx)
	Expect(err).To(BeNil())
}

// Check if 'IsPending' returns correct output
func TestAfPacketIsPending(t *testing.T) {
	ctx, plugin, _ := afPacketTestSetup(t)
	defer afPacketTestTeardown(ctx)

	// Reply set
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&ap_api.AfPacketCreateReply{})
	ctx.MockVpp.MockReply(&if_api.SwInterfaceTagAddDelReply{})
	// Data
	firstData := getTestAfPacketData("if1", []string{"10.0.0.1/24"}, "host1")
	secondData := getTestAfPacketData("if2", []string{"10.0.0.2/24"}, "host2")
	// Prepare
	plugin.ResolveCreatedLinuxInterface("host2", "host2", 3)
	_, pending, err := plugin.ConfigureAfPacketInterface(firstData)
	Expect(err).To(BeNil())
	Expect(pending).To(BeTrue())
	_, pending, err = plugin.ConfigureAfPacketInterface(secondData)
	Expect(err).To(BeNil())
	Expect(pending).To(BeFalse())
	// Test 'IsPending'
	isPending := plugin.IsPendingAfPacket(firstData)
	Expect(isPending).To(BeTrue())
	isPending = plugin.IsPendingAfPacket(secondData)
	Expect(isPending).To(BeFalse())
}

/* AF_PACKET Test Setup */

func afPacketTestSetup(t *testing.T) (*vppcallmock.TestCtx, *ifplugin.AFPacketConfigurator, ifaceidx.SwIfIndexRW) {
	RegisterTestingT(t)

	ctx := vppcallmock.SetupTestCtx(t)
	// Logger
	log := logrus.DefaultLogger()
	log.SetLevel(logging.DebugLevel)

	// Interface indices
	swIfIndices := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "afpacket", nil))
	// Configurator
	plugin := &ifplugin.AFPacketConfigurator{}
	ifHandler := vppcalls.NewIfVppHandler(ctx.MockChannel, log)
	err := plugin.Init(log, ifHandler, struct{}{}, swIfIndices)
	Expect(err).To(BeNil())

	return ctx, plugin, swIfIndices
}

func afPacketTestTeardown(ctx *vppcallmock.TestCtx) {
	ctx.TeardownTestCtx()
	err := safeclose.Close(ctx)
	Expect(err).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

/* AF_PACKET Test Data */

func getTestAfPacketData(ifName string, addresses []string, host string) *interfaces.Interfaces_Interface {
	return &interfaces.Interfaces_Interface{
		Name:        ifName,
		Type:        interfaces.InterfaceType_AF_PACKET_INTERFACE,
		Enabled:     true,
		IpAddresses: addresses,
		Afpacket: &interfaces.Interfaces_Interface_Afpacket{
			HostIfName: host,
		},
	}
}
