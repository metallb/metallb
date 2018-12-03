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

package vppcalls

import (
	"net"

	"git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp"
	bfd_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/bfd"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/bfd"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
)

// IfVppAPI provides methods for creating and managing interface plugin
type IfVppAPI interface {
	IfVppWrite
	IfVppRead
}

// IfVppWrite provides write methods for interface plugin
type IfVppWrite interface {
	// AddAfPacketInterface calls AfPacketCreate VPP binary API.
	AddAfPacketInterface(ifName string, hwAddr string, afPacketIntf *interfaces.Interfaces_Interface_Afpacket) (swIndex uint32, err error)
	// DeleteAfPacketInterface calls AfPacketDelete VPP binary API.
	DeleteAfPacketInterface(ifName string, idx uint32, afPacketIntf *interfaces.Interfaces_Interface_Afpacket) error
	// AddLoopbackInterface calls CreateLoopback bin API.
	AddLoopbackInterface(ifName string) (swIndex uint32, err error)
	// DeleteLoopbackInterface calls DeleteLoopback bin API.
	DeleteLoopbackInterface(ifName string, idx uint32) error
	// AddMemifInterface calls MemifCreate bin API.
	AddMemifInterface(ifName string, memIface *interfaces.Interfaces_Interface_Memif, socketID uint32) (swIdx uint32, err error)
	// DeleteMemifInterface calls MemifDelete bin API.
	DeleteMemifInterface(ifName string, idx uint32) error
	// AddTapInterface calls TapConnect bin API.
	AddTapInterface(ifName string, tapIf *interfaces.Interfaces_Interface_Tap) (swIfIdx uint32, err error)
	// DeleteTapInterface calls TapDelete bin API.
	DeleteTapInterface(ifName string, idx uint32, version uint32) error
	// AddVxLanTunnel calls AddDelVxLanTunnelReq with flag add=1.
	AddVxLanTunnel(ifName string, vrf, multicastIf uint32, vxLan *interfaces.Interfaces_Interface_Vxlan) (swIndex uint32, err error)
	// DeleteVxLanTunnel calls AddDelVxLanTunnelReq with flag add=0.
	DeleteVxLanTunnel(ifName string, idx, vrf uint32, vxLan *interfaces.Interfaces_Interface_Vxlan) error
	// InterfaceAdminDown calls binary API SwInterfaceSetFlagsReply with AdminUpDown=0.
	InterfaceAdminDown(ifIdx uint32) error
	// InterfaceAdminUp calls binary API SwInterfaceSetFlagsReply with AdminUpDown=1.
	InterfaceAdminUp(ifIdx uint32) error
	// SetInterfaceTag registers new interface index/tag pair
	SetInterfaceTag(tag string, ifIdx uint32) error
	// RemoveInterfaceTag un-registers new interface index/tag pair
	RemoveInterfaceTag(tag string, ifIdx uint32) error
	// SetInterfaceAsDHCPClient sets provided interface as a DHCP client
	SetInterfaceAsDHCPClient(ifIdx uint32, hostName string) error
	// UnsetInterfaceAsDHCPClient un-sets interface as DHCP client
	UnsetInterfaceAsDHCPClient(ifIdx uint32, hostName string) error
	// AddContainerIP calls IPContainerProxyAddDel VPP API with IsAdd=1
	AddContainerIP(ifIdx uint32, addr string) error
	// DelContainerIP calls IPContainerProxyAddDel VPP API with IsAdd=0
	DelContainerIP(ifIdx uint32, addr string) error
	// AddInterfaceIP calls SwInterfaceAddDelAddress bin API with IsAdd=1.
	AddInterfaceIP(ifIdx uint32, addr *net.IPNet) error
	// DelInterfaceIP calls SwInterfaceAddDelAddress bin API with IsAdd=00.
	DelInterfaceIP(ifIdx uint32, addr *net.IPNet) error
	// SetUnnumberedIP sets interface as un-numbered, linking IP address of the another interface (ifIdxWithIP)
	SetUnnumberedIP(uIfIdx uint32, ifIdxWithIP uint32) error
	// UnsetUnnumberedIP unset provided interface as un-numbered. IP address of the linked interface is removed
	UnsetUnnumberedIP(uIfIdx uint32) error
	// SetInterfaceMac calls SwInterfaceSetMacAddress bin API.
	SetInterfaceMac(ifIdx uint32, macAddress string) error
	// RegisterMemifSocketFilename registers new socket file name with provided ID.
	RegisterMemifSocketFilename(filename []byte, id uint32) error
	// SetInterfaceMtu calls HwInterfaceSetMtu bin API with desired MTU value.
	SetInterfaceMtu(ifIdx uint32, mtu uint32) error
	// SetRxMode calls SwInterfaceSetRxMode bin
	SetRxMode(ifIdx uint32, rxModeSettings *interfaces.Interfaces_Interface_RxModeSettings) error
	// SetRxPlacement configures rx-placement for interface
	SetRxPlacement(ifIdx uint32, rxPlacement *interfaces.Interfaces_Interface_RxPlacementSettings) error
	// CreateVrf checks if VRF exists and creates it if not
	CreateVrf(vrfID uint32) error
	// CreateVrfIPv6 checks if IPv6 VRF exists and creates it if not
	CreateVrfIPv6(vrfID uint32) error
	// SetInterfaceVrf retrieves VRF table from interface
	SetInterfaceVrf(ifaceIndex, vrfID uint32) error
	// SetInterfaceVrfIPv6 retrieves IPV6 VRF table from interface
	SetInterfaceVrfIPv6(ifaceIndex, vrfID uint32) error
}

// IfVppRead provides read methods for interface plugin
type IfVppRead interface {
	// DumpInterfaces dumps VPP interface data into the northbound API data structure
	// map indexed by software interface index.
	//
	// LIMITATIONS:
	// - there is no af_packet dump binary API. We relay on naming conventions of the internal VPP interface names
	// - ip.IPAddressDetails has wrong internal structure, as a workaround we need to handle them as notifications
	DumpInterfaces() (map[uint32]*InterfaceDetails, error)
	// DumpInterfacesByType returns all VPP interfaces of the specified type
	DumpInterfacesByType(reqType interfaces.InterfaceType) (map[uint32]*InterfaceDetails, error)
	// GetInterfaceVrf reads VRF table to interface
	GetInterfaceVrf(ifIdx uint32) (vrfID uint32, err error)
	// GetInterfaceVrfIPv6 reads IPv6 VRF table to interface
	GetInterfaceVrfIPv6(ifIdx uint32) (vrfID uint32, err error)
	// DumpMemifSocketDetails dumps memif socket details from the VPP
	DumpMemifSocketDetails() (map[string]uint32, error)
}

// BfdVppAPI provides methods for managing BFD
type BfdVppAPI interface {
	BfdVppWrite
	BfdVppRead
}

// BfdVppWrite provides write methods for BFD
type BfdVppWrite interface {
	// AddBfdUDPSession adds new BFD session with authentication if available.
	AddBfdUDPSession(bfdSess *bfd.SingleHopBFD_Session, ifIdx uint32, bfdKeyIndexes idxvpp.NameToIdx) error
	// AddBfdUDPSessionFromDetails adds new BFD session with authentication if available.
	AddBfdUDPSessionFromDetails(bfdSess *bfd_api.BfdUDPSessionDetails, bfdKeyIndexes idxvpp.NameToIdx) error
	// ModifyBfdUDPSession modifies existing BFD session excluding authentication which cannot be changed this way.
	ModifyBfdUDPSession(bfdSess *bfd.SingleHopBFD_Session, swIfIndexes ifaceidx.SwIfIndex) error
	// DeleteBfdUDPSession removes an existing BFD session.
	DeleteBfdUDPSession(ifIndex uint32, sourceAddress string, destAddress string) error
	// SetBfdUDPAuthenticationKey creates new authentication key.
	SetBfdUDPAuthenticationKey(bfdKey *bfd.SingleHopBFD_Key) error
	// DeleteBfdUDPAuthenticationKey removes the authentication key.
	DeleteBfdUDPAuthenticationKey(bfdKey *bfd.SingleHopBFD_Key) error
	// AddBfdEchoFunction sets up an echo function for the interface.
	AddBfdEchoFunction(bfdInput *bfd.SingleHopBFD_EchoFunction, swIfIndexes ifaceidx.SwIfIndex) error
	// DeleteBfdEchoFunction removes an echo function.
	DeleteBfdEchoFunction() error
}

// BfdVppRead provides read methods for BFD
type BfdVppRead interface {
	// DumpBfdSingleHop returns complete BFD configuration
	DumpBfdSingleHop() (*BfdDetails, error)
	// DumpBfdUDPSessions returns a list of BFD session's metadata
	DumpBfdSessions() (*BfdSessionDetails, error)
	// DumpBfdUDPSessionsWithID returns a list of BFD session's metadata filtered according to provided authentication key
	DumpBfdUDPSessionsWithID(authKeyIndex uint32) (*BfdSessionDetails, error)
	// DumpBfdKeys looks up all BFD auth keys and saves their name-to-index mapping
	DumpBfdAuthKeys() (*BfdAuthKeyDetails, error)
}

// NatVppAPI provides methods for managing NAT
type NatVppAPI interface {
	NatVppWrite
	NatVppRead
}

// NatVppWrite provides write methods for NAT
type NatVppWrite interface {
	// SetNat44Forwarding configures global forwarding setup for NAT44
	SetNat44Forwarding(enableFwd bool) error
	// EnableNat44Interface enables NAT feature for provided interface
	EnableNat44Interface(ifIdx uint32, isInside bool) error
	// DisableNat44Interface enables NAT feature for provided interface
	DisableNat44Interface(ifIdx uint32, isInside bool) error
	// EnableNat44InterfaceOutput enables NAT output feature for provided interface
	EnableNat44InterfaceOutput(ifIdx uint32, isInside bool) error
	// DisableNat44InterfaceOutput disables NAT output feature for provided interface
	DisableNat44InterfaceOutput(ifIdx uint32, isInside bool) error
	// AddNat44AddressPool sets new NAT address pool
	AddNat44AddressPool(first, last []byte, vrf uint32, twiceNat bool) error
	// DelNat44AddressPool removes existing NAT address pool
	DelNat44AddressPool(first, last []byte, vrf uint32, twiceNat bool) error
	// SetVirtualReassemblyIPv4 configures NAT virtual reassembly for IPv4 packets
	SetVirtualReassemblyIPv4(vrCfg *nat.Nat44Global_VirtualReassembly) error
	// SetVirtualReassemblyIPv4 configures NAT virtual reassembly for IPv6 packets
	SetVirtualReassemblyIPv6(vrCfg *nat.Nat44Global_VirtualReassembly) error
	// AddNat44IdentityMapping adds new NAT44 identity mapping
	AddNat44IdentityMapping(ctx *IdentityMappingContext) error
	// DelNat44IdentityMapping removes NAT44 identity mapping
	DelNat44IdentityMapping(ctx *IdentityMappingContext) error
	// AddNat44StaticMapping creates new static mapping entry (considering address only or both, address and port
	// depending on the context)
	AddNat44StaticMapping(ctx *StaticMappingContext) error
	// DelNat44StaticMapping removes existing static mapping entry
	DelNat44StaticMapping(ctx *StaticMappingContext) error
	// AddNat44StaticMappingLb creates new static mapping entry with load balancer
	AddNat44StaticMappingLb(ctx *StaticMappingLbContext) error
	// DelNat44StaticMappingLb removes existing static mapping entry with load balancer
	DelNat44StaticMappingLb(ctx *StaticMappingLbContext) error
}

// NatVppRead provides read methods for NAT
type NatVppRead interface {
	// Nat44Dump retuns global NAT configuration together with the DNAT configs
	Nat44Dump() (*Nat44Details, error)
	// Nat44GlobalConfigDump returns global config in NB format
	Nat44GlobalConfigDump() (*nat.Nat44Global, error)
	// NAT44NatDump dumps all types of mappings, sorts it according to tag (DNAT label) and creates a set of DNAT configurations
	Nat44DNatDump() (*nat.Nat44DNat, error)
	// Nat44InterfaceDump returns a list of interfaces enabled for NAT44
	Nat44InterfaceDump() (interfaces []*nat.Nat44Global_NatInterface, err error)
}

// StnVppAPI provides methods for managing STN
type StnVppAPI interface {
	StnVppWrite
	StnVppRead
}

// StnVppWrite provides write methods for STN
type StnVppWrite interface {
	// AddStnRule calls StnAddDelRule bin API with IsAdd=1
	AddStnRule(ifIdx uint32, addr *net.IP) error
	// DelStnRule calls StnAddDelRule bin API with IsAdd=0
	DelStnRule(ifIdx uint32, addr *net.IP) error
}

// StnVppRead provides read methods for STN
type StnVppRead interface {
	// DumpStnRules returns a list of all STN rules configured on the VPP
	DumpStnRules() (rules *StnDetails, err error)
}

// IfVppHandler is accessor for interface-related vppcalls methods
type IfVppHandler struct {
	callsChannel api.Channel
	log          logging.Logger
}

// BfdVppHandler is accessor for BFD-related vppcalls methods
type BfdVppHandler struct {
	callsChannel api.Channel
	ifIndexes    ifaceidx.SwIfIndex
	log          logging.Logger
}

// NatVppHandler is accessor for NAT-related vppcalls methods
type NatVppHandler struct {
	callsChannel api.Channel
	dumpChannel  api.Channel
	ifIndexes    ifaceidx.SwIfIndex
	log          logging.Logger
}

// StnVppHandler is accessor for STN-related vppcalls methods
type StnVppHandler struct {
	ifIndexes    ifaceidx.SwIfIndex
	callsChannel api.Channel
	log          logging.Logger
}

// NewIfVppHandler creates new instance of interface vppcalls handler
func NewIfVppHandler(callsChan api.Channel, log logging.Logger) *IfVppHandler {
	return &IfVppHandler{
		callsChannel: callsChan,
		log:          log,
	}
}

// NewBfdVppHandler creates new instance of BFD vppcalls handler
func NewBfdVppHandler(callsChan api.Channel, ifIndexes ifaceidx.SwIfIndex, log logging.Logger) *BfdVppHandler {
	return &BfdVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}

// NewNatVppHandler creates new instance of NAT vppcalls handler
func NewNatVppHandler(callsChan, dumpChan api.Channel, ifIndexes ifaceidx.SwIfIndex, log logging.Logger) *NatVppHandler {
	return &NatVppHandler{
		callsChannel: callsChan,
		dumpChannel:  dumpChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}

// NewStnVppHandler creates new instance of STN vppcalls handler
func NewStnVppHandler(callsChan api.Channel, ifIndexes ifaceidx.SwIfIndex, log logging.Logger) *StnVppHandler {
	return &StnVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}
