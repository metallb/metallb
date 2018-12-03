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
	"net"
	"testing"
	"time"

	"git.fd.io/govpp.git/core"

	"git.fd.io/govpp.git/adapter/mock"
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/af_packet"
	dhcp_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/dhcp"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/memif"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/tap"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/tapv2"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vxlan"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin"
	if_api "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

// Test dhcp
func TestInterfaceConfiguratorDHCPNotifications(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Register
	plugin.GetSwIfIndexes().RegisterName("if1", 1, nil)
	plugin.GetSwIfIndexes().RegisterName("if2", 2, nil)
	// Test DHCP notifications
	dhcpIpv4 := &dhcp_api.DHCPComplEvent{
		Lease: dhcp_api.DHCPLease{
			HostAddress:   net.ParseIP("10.0.0.1"),
			RouterAddress: net.ParseIP("10.0.0.2"),
			HostMac: func(mac string) []byte {
				parsed, _ := net.ParseMAC(mac)
				return parsed
			}("7C:4E:E7:8A:63:68"),
			Hostname: []byte("if1"),
			IsIPv6:   0,
		},
	}
	dhcpIpv6 := &dhcp_api.DHCPComplEvent{
		Lease: dhcp_api.DHCPLease{
			HostAddress:   net.ParseIP("fd21:7408:186f::/48"),
			RouterAddress: net.ParseIP("2001:db8:a0b:12f0::1/48"),
			HostMac: func(mac string) []byte {
				parsed, err := net.ParseMAC(mac)
				Expect(err).To(BeNil())
				return parsed
			}("7C:4E:E7:8A:63:68"),
			Hostname: []byte("if2"),
			IsIPv6:   1,
		},
	}
	plugin.DhcpChan <- dhcpIpv4
	Eventually(func() bool {
		_, _, found := plugin.GetDHCPIndexes().LookupIdx("if1")
		return found
	}, 2).Should(BeTrue())
	plugin.DhcpChan <- dhcpIpv6
	Eventually(func() bool {
		_, _, found := plugin.GetDHCPIndexes().LookupIdx("if2")
		return found
	}, 2).Should(BeTrue())
	// Test close
	err = plugin.Close()
	Expect(err).To(BeNil())
}

/* Interface configurator test cases */

// Get interface details and propagate it to status
func TestInterfaceConfiguratorPropagateIfDetailsToStatus(t *testing.T) {
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(
		&interfaces.SwInterfaceDetails{
			SwIfIndex: 1,
		},
		&interfaces.SwInterfaceDetails{
			SwIfIndex: 2,
		})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	// Register
	plugin.GetSwIfIndexes().RegisterName("if1", 1, nil)
	// Do not register second interface

	// Process notifications
	done := make(chan int)
	go func() {
		var counter int
		for {
			select {
			case notification := <-plugin.NotifChan:
				Expect(notification).ShouldNot(BeNil())
				counter++
			case <-time.After(time.Second / 2):
				done <- counter
				break
			}
		}
	}()

	// Test notifications
	Expect(ifplugin.PropagateIfDetailsToStatus(plugin)).To(Succeed())
	// This blocks until the result is sent
	Eventually(done).Should(Receive(Equal(1)))
}

// Configure new TAPv1 interface with IP address
func TestInterfacesConfigureTapV1(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&tap.TapConnectReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_TAP_INTERFACE, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1000)
	data.Tap = getTestTapInterface(1, "if1")
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_DEFAULT)
	// Test configure TAP
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
}

// Configure new TAPv2 interface without IP set as dhcp
func TestInterfacesConfigureTapV2(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&tapv2.TapCreateV2Reply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&dhcp_api.DHCPClientConfigReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_TAP_INTERFACE, []string{}, true, "46:06:18:DB:05:3A", 1500)
	data.Tap = getTestTapInterface(2, "if1")
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_DEFAULT)
	// Test configure TAPv2
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
}

// Configure new memory interface without IP set unnumbered, master and without socket filename registered
func TestInterfacesConfigureMemif(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&memif.MemifSocketFilenameAddDelReply{}) // Memif socket filename registration
	ctx.MockVpp.MockReply(&memif.MemifCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetUnnumberedReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_MEMORY_INTERFACE, []string{}, false, "46:06:18:DB:05:3A", 1500)
	data.Memif = getTestMemifInterface(true, 1)
	data.Unnumbered = getTestUnnumberedSettings("if2")
	// Register unnumbered interface
	plugin.GetSwIfIndexes().RegisterName("if2", 2, nil)
	// Test configure Memif
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(plugin.IsSocketFilenameCached("socket-filename")).To(BeTrue())
}

// Configure new memory interface without IP set unnumbered, slave and with socket filename registered
func TestInterfacesConfigureMemifAsSlave(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&memif.MemifSocketFilenameAddDelReply{}) // Memif socket filename registration
	ctx.MockVpp.MockReply(&memif.MemifCreateReply{                 // Initial memif interface (just to register filename)
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&memif.MemifCreateReply{ // Configure memif interface
		SwIfIndex: 2,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{ // Configure loopback with IP address for unnumbered memif
		SwIfIndex: 3,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetUnnumberedReply{}) // After unnumbered registration
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	// Data
	initialData := getTestInterface("if0", if_api.InterfaceType_MEMORY_INTERFACE, []string{}, false, "51:07:28:DB:05:B4", 1000)
	initialData.Memif = getTestMemifInterface(false, 2)
	data := getTestInterface("if1", if_api.InterfaceType_MEMORY_INTERFACE, []string{}, false, "46:06:18:DB:05:3A", 1500)
	data.Memif = getTestMemifInterface(true, 1)
	data.Unnumbered = getTestUnnumberedSettings("if2")
	// Test configure initial
	err = plugin.ConfigureVPPInterface(initialData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(initialData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	// Configure memif
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.Memif.Mode).To(BeEquivalentTo(0)) // Slave
	Expect(plugin.IsSocketFilenameCached("socket-filename")).To(BeTrue())
	Expect(plugin.IsUnnumberedIfCached("if1")).To(BeTrue())
	// Configure Unnumbered interface
	unnumberedData := getTestInterface("if2", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "", 0)
	err = plugin.ConfigureVPPInterface(unnumberedData)
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetSwIfIndexes().LookupIdx(unnumberedData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(plugin.IsUnnumberedIfCached("if1")).To(BeFalse())
}

// Configure new VxLAN interface
func TestInterfacesConfigureVxLAN(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&vxlan.VxlanAddDelTunnelReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{"10.0.0.1/24"}, false, "", 0)
	data.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "", 1)
	// Test configure VxLAN
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
}

// Configure new VxLAN interface with multicast
func TestInterfacesConfigureVxLANWithMulticast(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{ // Multicast
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                              // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})       // Break status propagation
	ctx.MockVpp.MockReply(&vxlan.VxlanAddDelTunnelReply{ // VxLAN
		SwIfIndex: 2,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{"10.0.0.1/24"}, false, "", 0)
	multicast := getTestInterface("multicastIf", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"239.0.0.1/24"}, false, "", 0)
	data.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "multicastIf", 1)
	// Configure multicast
	err = plugin.ConfigureVPPInterface(multicast)
	Expect(err).To(BeNil())
	// Test configure VxLAN
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.Vxlan).ToNot(BeNil())
	Expect(meta.Vxlan.Multicast).To(BeEquivalentTo(multicast.Name))
	Expect(plugin.IsMulticastVxLanIfCached("if1")).To(BeFalse())
}

// Configure new VxLAN interface with multicast cache
func TestInterfacesConfigureVxLANWithMulticastCache(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{ // Multicast
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                              // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})       // Break status propagation
	ctx.MockVpp.MockReply(&vxlan.VxlanAddDelTunnelReply{ // VxLAN
		SwIfIndex: 2,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{"10.0.0.1/24"}, false, "", 0)
	multicast := getTestInterface("multicastIf", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"239.0.0.1/24"}, false, "", 0)
	data.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "multicastIf", 1)
	// Test configure VxLAN
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
	Expect(plugin.IsMulticastVxLanIfCached("if1")).To(BeTrue())
	// Configure Multicast
	err = plugin.ConfigureVPPInterface(multicast)
	Expect(err).To(BeNil())
	// Check VxLAN again
	_, _, found = plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(plugin.IsMulticastVxLanIfCached("if1")).To(BeFalse())
}

// Configure new VxLAN interface with multicast cache delete
func TestInterfacesConfigureVxLANWithMulticastCacheDelete(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{"10.0.0.1/24"}, false, "", 0)
	data.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "multicastIf", 1)
	// Test configure VxLAN
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
	Expect(plugin.IsMulticastVxLanIfCached("if1")).To(BeTrue())
	// Delete VxLAN
	err = plugin.DeleteVPPInterface(data)
	Expect(err).To(BeNil())
	Expect(plugin.IsMulticastVxLanIfCached("if1")).To(BeFalse())
}

// Configure new VxLAN interface with multicast, where target interface does not contain multicast address
func TestInterfacesConfigureVxLANWithMulticastError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{ // Multicast
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{"10.0.0.1/24"}, false, "", 0)
	multicast := getTestInterface("multicastIf", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"20.0.0.1/24"}, false, "", 0)
	data.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "multicastIf", 1)
	// Configure multicast
	err = plugin.ConfigureVPPInterface(multicast)
	Expect(err).To(BeNil())
	// Test configure VxLAN
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsMulticastVxLanIfCached("if1")).To(BeFalse())
}

// Configure new VxLAN interface with multicast, where target interface does not contain IP address
func TestInterfacesConfigureVxLANWithMulticastIPError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{ // Multicast
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{"10.0.0.1/24"}, false, "", 0)
	multicast := getTestInterface("multicastIf", if_api.InterfaceType_SOFTWARE_LOOPBACK, nil, false, "", 0)
	data.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "multicastIf", 1)
	// Configure multicast
	err = plugin.ConfigureVPPInterface(multicast)
	Expect(err).To(BeNil())
	// Test configure VxLAN
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	Expect(plugin.IsMulticastVxLanIfCached("if1")).To(BeFalse())
}

// Configure new VxLAN interface with default MTU
func TestInterfacesConfigureLoopback(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 0)
	// Test configure loopback
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.Mtu).To(BeEquivalentTo(1500))
}

// Configure existing Ethernet interface
func TestInterfacesConfigureEthernet(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_ETHERNET_CSMACD, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	// Register ethernet
	plugin.GetSwIfIndexes().RegisterName("if1", 1, nil)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
}

// Configure non-existing Ethernet interface
func TestInterfacesConfigureEthernetNonExisting(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_ETHERNET_CSMACD, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
	Expect(meta).To(BeNil())
}

// Configure AfPacket interface
func TestInterfacesConfigureAfPacket(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&af_packet.AfPacketCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	data := getTestAfPacket("if1", []string{"10.0.0.1/24"}, "host1")
	// Register host
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 2)
	// Test configure AF packet
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
}

// Configure AfPacket interface
func TestInterfacesConfigureAfPacketPending(t *testing.T) {
	var err error
	// Setup
	_, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Data
	data := getTestAfPacket("if1", []string{"10.0.0.1/24"}, "host1")
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

// Configure new interface and tests error propagation during configuration
func TestInterfacesConfigureInterfaceLoopbackError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{
		SwIfIndex: 1,
	})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

// Configure new interface and tests error propagation during configuration
func TestInterfacesConfigureInterfaceRxModeError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{
		Retval: 1, // Simulate Rx mode error
	})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

// Configure new interface and tests error propagation during configuration
func TestInterfacesConfigureInterfaceMacError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{
		Retval: 1, // Simulate MAC error
	})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

// Configure new interface and tests error propagation during configuration
func TestInterfacesConfigureInterfaceVrfError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{
		Retval: 1, // Interface VRF error
	})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

// Configure new interface and tests error propagation during configuration
func TestInterfacesConfigureInterfaceIPAddressError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{
		Retval: 1, // IP address error
	})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

// Configure new interface and tests error propagation during configuration
func TestInterfacesConfigureInterfaceContainerIPAddressError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{
		Retval: 1, // Container IP error
	})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

// Configure new interface and tests error propagation during configuration
func TestInterfacesConfigureInterfaceMtuError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{
		Retval: 1, // MTU error
	})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	data.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

// Configure new interface and tests admin up error
func TestInterfacesConfigureInterfaceAdminUpError(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{
		Retval: 1,
	})
	// Data
	data := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 0)
	// Test configure TAP
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).ToNot(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
}

// Modify TAPv1 interface
func TestInterfacesModifyTapV1WithoutTapData(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	tapData := getTestTapInterface(1, "if1")
	oldData := getTestInterface("if1", if_api.InterfaceType_TAP_INTERFACE, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	oldData.Tap = tapData
	oldData.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_DEFAULT)
	newData := getTestInterface("if1", if_api.InterfaceType_TAP_INTERFACE, []string{"10.0.0.2/24"}, false, "BC:FE:E9:5E:07:04", 2000)
	newData.Tap = tapData
	newData.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_INTERRUPT)
	// Register old config
	plugin.GetSwIfIndexes().RegisterName("if1", 1, oldData)
	// Test configure TAP
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.IpAddresses).To(HaveLen(1))
	Expect(meta.IpAddresses[0]).To(BeEquivalentTo("10.0.0.2/24"))
	Expect(meta.PhysAddress).To(BeEquivalentTo("BC:FE:E9:5E:07:04"))
}

// Modify TAPv1 interface including tap data
func TestInterfacesModifyTapV1TapData(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{}) // Delete
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&tap.TapDeleteReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&tap.TapConnectReply{ // Create
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	oldData := getTestInterface("if1", if_api.InterfaceType_TAP_INTERFACE, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	oldData.Tap = getTestTapInterface(1, "if1")
	oldData.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_DEFAULT)
	newData := getTestInterface("if1", if_api.InterfaceType_TAP_INTERFACE, []string{"10.0.0.2/24"}, false, "BC:FE:E9:5E:07:04", 1500)
	newData.Tap = getTestTapInterface(1, "if2")
	newData.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_INTERRUPT)
	// Register old config
	plugin.GetSwIfIndexes().RegisterName("if1", 1, oldData)
	// Test configure TAP
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.IpAddresses[0]).To(BeEquivalentTo("10.0.0.2/24"))
	Expect(meta.PhysAddress).To(BeEquivalentTo("BC:FE:E9:5E:07:04"))
}

// Modify memif interface
func TestInterfacesModifyMemifWithoutMemifData(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&dhcp_api.DHCPClientConfigReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	memifData := getTestMemifInterface(true, 1)
	oldData := getTestInterface("if1", if_api.InterfaceType_MEMORY_INTERFACE, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	oldData.Memif = memifData
	newData := getTestInterface("if1", if_api.InterfaceType_MEMORY_INTERFACE, []string{}, true, "46:06:18:DB:05:3A", 1500)
	newData.Memif = memifData
	newData.Enabled = false
	newData.ContainerIpAddress = "10.0.0.4"
	// Register old config
	plugin.GetSwIfIndexes().RegisterName("if1", 1, oldData)
	// Test configure TAP
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.SetDhcpClient).To(BeTrue())
}

// Modify memif interface including memif data
func TestInterfacesModifyMemifData(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&memif.MemifSocketFilenameAddDelReply{}) // Create - configure old data
	ctx.MockVpp.MockReply(&memif.MemifCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{}) // Modify - delete old data
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&memif.MemifDeleteReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&memif.MemifCreateReply{ // Modify - crate new data
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	// Data
	oldData := getTestInterface("if1", if_api.InterfaceType_MEMORY_INTERFACE, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	oldData.Memif = getTestMemifInterface(true, 1)
	newData := getTestInterface("if1", if_api.InterfaceType_MEMORY_INTERFACE, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	newData.Memif = getTestMemifInterface(false, 2)
	// Register old config and socket filename
	plugin.GetSwIfIndexes().RegisterName("if1", 1, oldData)
	// Test configure memif
	err = plugin.ConfigureVPPInterface(oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	// Test modify memif
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.Memif.Mode).To(BeEquivalentTo(0)) // Slave
	Expect(meta.Memif.Id).To(BeEquivalentTo(2))
}

// Modify VxLAN interface
func TestInterfacesModifyVxLanSimple(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&vxlan.VxlanAddDelTunnelReply{ // Create - configure old data
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&dhcp_api.DHCPClientConfigReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&dhcp_api.DHCPClientConfigReply{}) // Modify - delete old data
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	// Data
	oldData := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{}, true, "", 0)
	oldData.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "", 1)
	newData := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{}, false, "", 0)
	newData.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "", 1)
	// Test configure vxlan
	err = plugin.ConfigureVPPInterface(oldData)
	Expect(err).To(BeNil())
	// Test modify vxlan
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.SetDhcpClient).To(BeFalse())
}

// Modify VxLAN interface multicast
func TestInterfacesModifyVxLanMulticast(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&vxlan.VxlanAddDelTunnelReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation
	// Data
	oldData := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{}, true, "", 0)
	oldData.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "multicastIf", 1)
	newData := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{}, false, "", 0)
	newData.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "", 1)
	// Test configure vxlan
	err = plugin.ConfigureVPPInterface(oldData)
	Expect(err).To(BeNil())
	Expect(plugin.IsMulticastVxLanIfCached("if1")).To(BeTrue())
	// Test modify vxlan
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(plugin.IsMulticastVxLanIfCached("if1")).To(BeFalse())
}

// Modify VxLAN interface with recreate
func TestInterfacesModifyVxLanData(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&vxlan.VxlanAddDelTunnelReply{ // Create - configure old data
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{}) // Modify - delete old data
	ctx.MockVpp.MockReply(&vxlan.VxlanAddDelTunnelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&vxlan.VxlanAddDelTunnelReply{ // Modify - configure new data
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	// Data
	oldData := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{}, false, "", 0)
	oldData.Vxlan = getTestVxLanInterface("10.0.0.2", "10.0.0.3", "", 1)
	newData := getTestInterface("if1", if_api.InterfaceType_VXLAN_TUNNEL, []string{}, false, "", 0)
	newData.Vxlan = getTestVxLanInterface("10.0.0.4", "10.0.0.5", "", 1)
	// Register old config and socket filename
	plugin.GetSwIfIndexes().RegisterName("if1", 1, oldData)
	// Test configure vxlan
	err = plugin.ConfigureVPPInterface(oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	// Test modify vxlan
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.Vxlan.SrcAddress).To(BeEquivalentTo("10.0.0.4"))
	Expect(meta.Vxlan.DstAddress).To(BeEquivalentTo("10.0.0.5"))
}

// Modify loopback interface
func TestInterfacesModifyLoopback(t *testing.T) {
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&interfaces.CreateLoopbackReply{ // Create
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{}) // Modify
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply()
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	// Data
	oldData := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK,
		[]string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 0)
	newData := getTestInterface("if1", if_api.InterfaceType_SOFTWARE_LOOPBACK,
		[]string{"10.0.0.1/24", "10.0.0.2/24"}, false, "46:06:18:DB:05:3A", 0)

	// Test configure loopback
	var err error
	err = plugin.ConfigureVPPInterface(oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(oldData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	// Test modify loopback
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetSwIfIndexes().LookupIdx(oldData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.IpAddresses).To(HaveLen(2))
}

// Modify existing Ethernet interface
func TestInterfacesModifyEthernet(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{}) // Configure
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetMacAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{}) // Modify
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	// Data
	oldData := getTestInterface("if1", if_api.InterfaceType_ETHERNET_CSMACD, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	oldData.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	newData := getTestInterface("if1", if_api.InterfaceType_ETHERNET_CSMACD, []string{"10.0.0.2/24"}, false, "46:06:18:DB:05:3A", 1500)
	newData.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	// Register ethernet
	plugin.GetSwIfIndexes().RegisterName("if1", 1, nil)
	// Test configure ethernet
	err = plugin.ConfigureVPPInterface(oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(oldData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	// Test modify ethernet
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.IpAddresses).To(HaveLen(1))
	Expect(meta.IpAddresses[0]).To(BeEquivalentTo("10.0.0.2/24"))
}

// Modify Af-packet interface
func TestInterfacesModifyAfPacket(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&af_packet.AfPacketCreateReply{}) // Create
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{}) // Modify
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	// Data
	oldData := getTestAfPacket("if1", []string{"10.0.0.1/24"}, "host1")
	oldData.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_POLLING)
	oldData.Enabled = false
	newData := getTestAfPacket("if1", []string{"10.0.0.2/24"}, "host1")
	// Register host
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 2)
	// Test configure af-packet
	err = plugin.ConfigureVPPInterface(oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(oldData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	// Test modify af-packet
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.IpAddresses).To(HaveLen(1))
	Expect(meta.IpAddresses[0]).To(BeEquivalentTo("10.0.0.2/24"))
}

// Modify Af-packet interface
func TestInterfacesModifyAfPacketPending(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&af_packet.AfPacketCreateReply{}) // Create
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&af_packet.AfPacketDeleteReply{}) // Modify
	ctx.MockVpp.MockReply(&af_packet.AfPacketCreateReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	// Data
	oldData := getTestAfPacket("if1", []string{"10.0.0.1/24"}, "host1")
	newData := getTestAfPacket("if1", []string{"10.0.0.2/24"}, "host1")
	// Test configure af-packet
	err = plugin.ConfigureVPPInterface(oldData)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(oldData.Name)
	Expect(found).To(BeFalse())
	// Test modify af-packet
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeFalse())
}

func TestInterfacesModifyAfPacketRecreate(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&af_packet.AfPacketCreateReply{}) // Create
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&af_packet.AfPacketDeleteReply{}) // Modify
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&af_packet.AfPacketCreateReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply() // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	// Data
	oldData := getTestAfPacket("if1", []string{"10.0.0.1/24"}, "host1")
	newData := getTestAfPacket("if1", []string{"10.0.0.1/24"}, "host2")
	// Register hosts
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 2)
	plugin.ResolveCreatedLinuxInterface("host2", "host2", 3)
	// Test configure af-packet
	err = plugin.ConfigureVPPInterface(oldData)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(oldData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	// Test modify af-packet
	err = plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
}

func TestInterfacesDeleteTapInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&dhcp_api.DHCPClientConfigReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&tap.TapDeleteReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestInterface("if", if_api.InterfaceType_TAP_INTERFACE, []string{"10.0.0.1/24"}, true, "46:06:18:DB:05:3A", 1500)
	data.Tap = getTestTapInterface(1, "tap-host")
	// Register interface (as if it is configured)
	plugin.GetSwIfIndexes().RegisterName("if", 1, data)
	// Test delete
	err = plugin.DeleteVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

func TestInterfacesDeleteMemifInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&memif.MemifDeleteReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestInterface("if", if_api.InterfaceType_MEMORY_INTERFACE, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	// Register interface (as if it is configured)
	plugin.GetSwIfIndexes().RegisterName("if", 1, data)
	// Test delete
	err = plugin.DeleteVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

func TestInterfacesDeleteVxlanInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&vxlan.VxlanAddDelTunnelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestInterface("if", if_api.InterfaceType_VXLAN_TUNNEL, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	data.Vxlan = getTestVxLanInterface("10.0.0.1", "20.0.0.1", "", 1)
	// Register interface (as if it is configured)
	plugin.GetSwIfIndexes().RegisterName("if", 1, data)
	// Test delete
	err = plugin.DeleteVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

func TestInterfacesDeleteLoopbackInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.DeleteLoopbackReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestInterface("if", if_api.InterfaceType_SOFTWARE_LOOPBACK, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	// Register interface (as if it is configured)
	plugin.GetSwIfIndexes().RegisterName("if", 1, data)
	// Test delete
	err = plugin.DeleteVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

func TestInterfacesDeleteEthernetInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	// Data
	data := getTestInterface("if", if_api.InterfaceType_ETHERNET_CSMACD, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	// Register interface (as if it is configured)
	plugin.GetSwIfIndexes().RegisterName("if", 1, data)
	// Test delete
	err = plugin.DeleteVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue()) // Still in mapping
}

func TestInterfacesDeleteAfPacketInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{}) // Delete
	ctx.MockVpp.MockReply(&af_packet.AfPacketDeleteReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestAfPacket("if1", []string{"10.0.0.1/24"}, "host1")
	// Register hosts
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 2)
	// Register af-packet
	plugin.GetSwIfIndexes().RegisterName(data.Name, 1, data)
	// Test delete
	err = plugin.DeleteVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

func TestInterfacesDeletePendingAfPacketInterface(t *testing.T) {
	var err error
	// Setup
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)
	// Reply set
	ctx.MockVpp.MockReply(&af_packet.AfPacketCreateReply{}) // Create
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.HwInterfaceSetMtuReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetFlagsReply{}) // Delete
	ctx.MockVpp.MockReply(&af_packet.AfPacketDeleteReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&af_packet.AfPacketDeleteReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})
	// Data
	data := getTestAfPacket("if1", []string{"10.0.0.1/24"}, "host1")
	// Register hosts
	plugin.ResolveCreatedLinuxInterface("host1", "host1", 2)
	// Test delete
	err = plugin.ConfigureVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found := plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeTrue())
	err = plugin.ResolveDeletedLinuxInterface("host1", "host1", 2)
	Expect(err).To(BeNil())
	err = plugin.DeleteVPPInterface(data)
	Expect(err).To(BeNil())
	_, _, found = plugin.GetSwIfIndexes().LookupIdx(data.Name)
	Expect(found).To(BeFalse())
}

func TestModifyRxMode(t *testing.T) {
	ctx, connection, plugin := ifTestSetup(t)
	defer ifTestTeardown(connection, plugin)

	// Reply set
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetRxModeReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetTableReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})
	ctx.MockVpp.MockReply(&ip.IPContainerProxyAddDelReply{})
	ctx.MockVpp.MockReply()                        // Do not propagate interface details
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{}) // Break status propagation

	// Data
	memifData := getTestMemifInterface(true, 1)
	oldData := getTestInterface("if1", if_api.InterfaceType_MEMORY_INTERFACE, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	oldData.Memif = memifData
	newData := getTestInterface("if1", if_api.InterfaceType_MEMORY_INTERFACE, []string{"10.0.0.1/24"}, false, "46:06:18:DB:05:3A", 1500)
	newData.Memif = memifData
	newData.RxModeSettings = getTestRxModeSettings(if_api.RxModeType_DEFAULT)
	newData.RxModeSettings.QueueId = 5

	// Register old config
	plugin.GetSwIfIndexes().RegisterName("if1", 1, oldData)
	// Test configure
	err := plugin.ModifyVPPInterface(newData, oldData)
	Expect(err).ToNot(HaveOccurred())
	_, meta, found := plugin.GetSwIfIndexes().LookupIdx(newData.Name)
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.RxModeSettings.QueueId).To(Equal(uint32(5)))
}

/* Interface Test Setup */

func ifTestSetup(t *testing.T) (*vppcallmock.TestCtx, *core.Connection, *ifplugin.InterfaceConfigurator) {
	RegisterTestingT(t)

	ctx := &vppcallmock.TestCtx{
		MockVpp: mock.NewVppAdapter(),
	}
	connection, err := core.Connect(ctx.MockVpp)
	Expect(err).ShouldNot(HaveOccurred())

	// Logger
	log := logging.ForPlugin("test-log")
	log.SetLevel(logging.DebugLevel)

	// Configurator
	plugin := &ifplugin.InterfaceConfigurator{}
	notifChan := make(chan govppapi.Message, 5)
	err = plugin.Init(log, connection, 1, notifChan, 1500)
	Expect(err).To(BeNil())

	return ctx, connection, plugin
}

func ifTestTeardown(connection *core.Connection, plugin *ifplugin.InterfaceConfigurator) {
	connection.Disconnect()
	err := plugin.Close()
	Expect(err).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

/* Interface Test Data */

func getTestInterface(name string, ifType if_api.InterfaceType, ip []string, dhcp bool, mac string, mtu uint32) *if_api.Interfaces_Interface {
	return &if_api.Interfaces_Interface{
		Name:               name,
		Enabled:            true,
		Type:               ifType,
		IpAddresses:        ip,
		SetDhcpClient:      dhcp,
		PhysAddress:        mac,
		Mtu:                mtu,
		ContainerIpAddress: "10.0.0.5",
	}
}

func getTestAfPacket(ifName string, addresses []string, host string) *if_api.Interfaces_Interface {
	return &if_api.Interfaces_Interface{
		Name:        ifName,
		Type:        if_api.InterfaceType_AF_PACKET_INTERFACE,
		Enabled:     true,
		IpAddresses: addresses,
		Afpacket: &if_api.Interfaces_Interface_Afpacket{
			HostIfName: host,
		},
	}
}

func getTestMemifInterface(master bool, id uint32) *if_api.Interfaces_Interface_Memif {
	return &if_api.Interfaces_Interface_Memif{
		Master:         master,
		Id:             id,
		SocketFilename: "socket-filename",
	}
}

func getTestVxLanInterface(src, dst, multicastIf string, vni uint32) *if_api.Interfaces_Interface_Vxlan {
	return &if_api.Interfaces_Interface_Vxlan{
		Multicast:  multicastIf,
		SrcAddress: src,
		DstAddress: dst,
		Vni:        vni,
	}
}

func getTestTapInterface(ver uint32, host string) *if_api.Interfaces_Interface_Tap {
	return &if_api.Interfaces_Interface_Tap{
		Version:    ver,
		HostIfName: host,
	}
}

func getTestRxModeSettings(mode if_api.RxModeType) *if_api.Interfaces_Interface_RxModeSettings {
	return &if_api.Interfaces_Interface_RxModeSettings{
		RxMode: mode,
	}
}

func getTestUnnumberedSettings(ifNameWithIP string) *if_api.Interfaces_Interface_Unnumbered {
	return &if_api.Interfaces_Interface_Unnumbered{
		IsUnnumbered:    true,
		InterfaceWithIp: ifNameWithIP,
	}
}
