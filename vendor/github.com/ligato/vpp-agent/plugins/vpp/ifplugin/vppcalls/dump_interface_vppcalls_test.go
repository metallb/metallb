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

package vppcalls_test

import (
	"net"
	"testing"

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/dhcp"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/memif"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/tap"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/tapv2"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vxlan"
	interfaces2 "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

// Test dump of interfaces with vxlan type
func TestDumpInterfacesVxLan(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ipv61Parse := net.ParseIP("dead:beef:feed:face:cafe:babe:baad:c0de").To16()
	ipv62Parse := net.ParseIP("d3ad:beef:feed:face:cafe:babe:baad:c0de").To16()

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name: (&interfaces.SwInterfaceDump{}).GetMessageName(),
			Ping: true,
			Message: &interfaces.SwInterfaceDetails{
				InterfaceName: []byte("vxlan1"),
			},
		},
		{
			Name:    (&interfaces.SwInterfaceGetTable{}).GetMessageName(),
			Ping:    false,
			Message: &interfaces.SwInterfaceGetTableReply{},
		},
		{
			Name:    (&ip.IPAddressDump{}).GetMessageName(),
			Ping:    true,
			Message: &ip.IPAddressDetails{},
		},
		{
			Name:    (&memif.MemifSocketFilenameDump{}).GetMessageName(),
			Ping:    true,
			Message: &memif.MemifSocketFilenameDetails{},
		},
		{
			Name:    (&memif.MemifDump{}).GetMessageName(),
			Ping:    true,
			Message: &memif.MemifDetails{},
		},
		{
			Name:    (&tap.SwInterfaceTapDump{}).GetMessageName(),
			Ping:    true,
			Message: &tap.SwInterfaceTapDetails{},
		},
		{
			Name:    (&tapv2.SwInterfaceTapV2Dump{}).GetMessageName(),
			Ping:    true,
			Message: &tapv2.SwInterfaceTapV2Details{},
		},
		{
			Name: (&vxlan.VxlanTunnelDump{}).GetMessageName(),
			Ping: true,
			Message: &vxlan.VxlanTunnelDetails{
				IsIPv6:     1,
				SwIfIndex:  0,
				SrcAddress: ipv61Parse,
				DstAddress: ipv62Parse,
			},
		},
	})

	intfs, err := ifHandler.DumpInterfaces()
	Expect(err).To(BeNil())
	Expect(intfs).To(HaveLen(1))
	intface := intfs[0].Interface

	// Check vxlan
	Expect(intface.Vxlan.SrcAddress).To(Equal("dead:beef:feed:face:cafe:babe:baad:c0de"))
	Expect(intface.Vxlan.DstAddress).To(Equal("d3ad:beef:feed:face:cafe:babe:baad:c0de"))
}

// Test dump of interfaces with host type
func TestDumpInterfacesHost(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name: (&interfaces.SwInterfaceDump{}).GetMessageName(),
			Ping: true,
			Message: &interfaces.SwInterfaceDetails{
				InterfaceName: []byte("host-localhost"),
			},
		},
		{
			Name:    (&interfaces.SwInterfaceGetTable{}).GetMessageName(),
			Ping:    false,
			Message: &interfaces.SwInterfaceGetTableReply{},
		},
		{
			Name:    (&ip.IPAddressDump{}).GetMessageName(),
			Ping:    true,
			Message: &ip.IPAddressDetails{},
		},
		{
			Name:    (&memif.MemifSocketFilenameDump{}).GetMessageName(),
			Ping:    true,
			Message: &memif.MemifSocketFilenameDetails{},
		},
		{
			Name:    (&memif.MemifDump{}).GetMessageName(),
			Ping:    true,
			Message: &memif.MemifDetails{},
		},
		{
			Name:    (&tap.SwInterfaceTapDump{}).GetMessageName(),
			Ping:    true,
			Message: &tap.SwInterfaceTapDetails{},
		},
		{
			Name:    (&tapv2.SwInterfaceTapV2Dump{}).GetMessageName(),
			Ping:    true,
			Message: &tapv2.SwInterfaceTapV2Details{},
		},
		{
			Name:    (&vxlan.VxlanTunnelDump{}).GetMessageName(),
			Ping:    true,
			Message: &vxlan.VxlanTunnelDetails{},
		},
	})

	intfs, err := ifHandler.DumpInterfaces()
	Expect(err).To(BeNil())
	Expect(intfs).To(HaveLen(1))
	intface := intfs[0].Interface

	// Check interface data
	Expect(intface.Afpacket.HostIfName).To(Equal("localhost"))
}

// Test dump of interfaces with memif type
func TestDumpInterfacesMemif(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name: (&interfaces.SwInterfaceDump{}).GetMessageName(),
			Ping: true,
			Message: &interfaces.SwInterfaceDetails{
				InterfaceName: []byte("memif1"),
			},
		},
		{
			Name:    (&interfaces.SwInterfaceGetTable{}).GetMessageName(),
			Ping:    false,
			Message: &interfaces.SwInterfaceGetTableReply{},
		},
		{
			Name:    (&ip.IPAddressDump{}).GetMessageName(),
			Ping:    true,
			Message: &ip.IPAddressDetails{},
		},
		{
			Name: (&memif.MemifSocketFilenameDump{}).GetMessageName(),
			Ping: true,
			Message: &memif.MemifSocketFilenameDetails{
				SocketID:       1,
				SocketFilename: []byte("test"),
			},
		},
		{
			Name: (&memif.MemifDump{}).GetMessageName(),
			Ping: true,
			Message: &memif.MemifDetails{
				ID:         2,
				SwIfIndex:  0,
				Role:       1, // Slave
				Mode:       1, // IP
				SocketID:   1,
				RingSize:   0,
				BufferSize: 0,
			},
		},
		{
			Name:    (&tap.SwInterfaceTapDump{}).GetMessageName(),
			Ping:    true,
			Message: &tap.SwInterfaceTapDetails{},
		},
		{
			Name:    (&tapv2.SwInterfaceTapV2Dump{}).GetMessageName(),
			Ping:    true,
			Message: &tapv2.SwInterfaceTapV2Details{},
		},
		{
			Name:    (&vxlan.VxlanTunnelDump{}).GetMessageName(),
			Ping:    true,
			Message: &vxlan.VxlanTunnelDetails{},
		},
	})

	intfs, err := ifHandler.DumpInterfaces()
	Expect(err).To(BeNil())
	Expect(intfs).To(HaveLen(1))
	intface := intfs[0].Interface

	// Check memif
	Expect(intface.Memif.SocketFilename).To(Equal("test"))
	Expect(intface.Memif.Id).To(Equal(uint32(2)))
	Expect(intface.Memif.Mode).To(Equal(interfaces2.Interfaces_Interface_Memif_IP))
	Expect(intface.Memif.Master).To(BeFalse())
}

// Test dump of interfaces using custom mock reply handler to avoid issues with ControlPingMessageReply
// not being properly recognized
func TestDumpInterfacesFull(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	hwAddr1Parse, err := net.ParseMAC("01:23:45:67:89:ab")
	Expect(err).To(BeNil())

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name: (&interfaces.SwInterfaceDump{}).GetMessageName(),
			Ping: true,
			Message: &interfaces.SwInterfaceDetails{
				SwIfIndex:       0,
				InterfaceName:   []byte("memif1"),
				Tag:             []byte("interface2"),
				AdminUpDown:     1,
				LinkMtu:         9216, // Default MTU
				L2Address:       hwAddr1Parse,
				L2AddressLength: uint32(len(hwAddr1Parse)),
			},
		},
		{
			Name: (&interfaces.SwInterfaceGetTable{}).GetMessageName(),
			Ping: false,
			Message: &interfaces.SwInterfaceGetTableReply{
				Retval: 0,
				VrfID:  42,
			},
		},
		{
			Name:    (&ip.IPAddressDump{}).GetMessageName(),
			Ping:    true,
			Message: &ip.IPAddressDetails{},
		},
		{
			Name: (&dhcp.DHCPClientDump{}).GetMessageName(),
			Ping: true,
			Message: &dhcp.DHCPClientDetails{
				Client: dhcp.DHCPClient{
					SwIfIndex: 0,
				},
			},
		},
		{
			Name: (&memif.MemifSocketFilenameDump{}).GetMessageName(),
			Ping: true,
			Message: &memif.MemifSocketFilenameDetails{
				SocketID:       1,
				SocketFilename: []byte("test"),
			},
		},
		{
			Name: (&memif.MemifDump{}).GetMessageName(),
			Ping: true,
			Message: &memif.MemifDetails{
				ID:         2,
				SwIfIndex:  0,
				Role:       0, // Master
				Mode:       0, // Ethernet
				SocketID:   1,
				RingSize:   0,
				BufferSize: 0,
			},
		},
		{
			Name: (&tap.SwInterfaceTapDump{}).GetMessageName(),
			Ping: true,
			Message: &tap.SwInterfaceTapDetails{
				SwIfIndex: 0,
				DevName:   []byte("taptap"),
			},
		},
		{
			Name: (&tapv2.SwInterfaceTapV2Dump{}).GetMessageName(),
			Ping: true,
			Message: &tapv2.SwInterfaceTapV2Details{
				SwIfIndex:  0,
				HostIfName: []byte("taptap2"), // This will overwrite the v1 tap name
			},
		},
		{
			Name: (&vxlan.VxlanTunnelDump{}).GetMessageName(),
			Ping: true,
			Message: &vxlan.VxlanTunnelDetails{
				SwIfIndex:  0,
				SrcAddress: []byte{192, 168, 0, 1},
				DstAddress: []byte{192, 168, 0, 2},
			},
		},
	})

	intfs, err := ifHandler.DumpInterfaces()
	Expect(err).To(BeNil())
	Expect(intfs).To(HaveLen(1))

	intface := intfs[0].Interface

	// This is last checked type, so it will be equal to that
	Expect(intface.Type).To(Equal(interfaces2.InterfaceType_VXLAN_TUNNEL))
	Expect(intface.PhysAddress).To(Equal("01:23:45:67:89:ab"))
	Expect(intface.Name).To(Equal("interface2"))
	Expect(intface.Mtu).To(Equal(uint32(0))) // default mtu
	Expect(intface.Enabled).To(BeTrue())
	Expect(intface.Vrf).To(Equal(uint32(42)))
	Expect(intface.SetDhcpClient).To(BeTrue())

	// Check memif
	Expect(intface.Memif.SocketFilename).To(Equal("test"))
	Expect(intface.Memif.Id).To(Equal(uint32(2)))
	Expect(intface.Memif.Mode).To(Equal(interfaces2.Interfaces_Interface_Memif_ETHERNET))
	Expect(intface.Memif.Master).To(BeTrue())

	// Check tap
	Expect(intface.Tap.HostIfName).To(Equal("taptap2"))
	Expect(intface.Tap.Version).To(Equal(uint32(2)))

	// Check vxlan
	Expect(intface.Vxlan.SrcAddress).To(Equal("192.168.0.1"))
	Expect(intface.Vxlan.DstAddress).To(Equal("192.168.0.2"))
}

// Test dump of memif socket details using standard reply mocking
func TestDumpMemifSocketDetails(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifSocketFilenameDetails{
		SocketID:       1,
		SocketFilename: []byte("test"),
	})

	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	result, err := ifHandler.DumpMemifSocketDetails()
	Expect(err).To(BeNil())
	Expect(result).To(Not(BeEmpty()))

	socketID, ok := result["test"]
	Expect(ok).To(BeTrue())
	Expect(socketID).To(Equal(uint32(1)))
}
