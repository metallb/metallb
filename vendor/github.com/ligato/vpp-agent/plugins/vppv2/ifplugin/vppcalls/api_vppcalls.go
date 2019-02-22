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

	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
)

// IfVppAPI provides methods for creating and managing interface plugin
type IfVppAPI interface {
	IfVppWrite
	IfVppRead
}

// IfVppWrite provides write methods for interface plugin
type IfVppWrite interface {
	// AddAfPacketInterface calls AfPacketCreate VPP binary API.
	AddAfPacketInterface(ifName string, hwAddr string, afPacketIntf *interfaces.AfpacketLink) (swIndex uint32, err error)
	// DeleteAfPacketInterface calls AfPacketDelete VPP binary API.
	DeleteAfPacketInterface(ifName string, idx uint32, afPacketIntf *interfaces.AfpacketLink) error
	// AddLoopbackInterface calls CreateLoopback bin API.
	AddLoopbackInterface(ifName string) (swIndex uint32, err error)
	// DeleteLoopbackInterface calls DeleteLoopback bin API.
	DeleteLoopbackInterface(ifName string, idx uint32) error
	// AddMemifInterface calls MemifCreate bin API.
	AddMemifInterface(ifName string, memIface *interfaces.MemifLink, socketID uint32) (swIdx uint32, err error)
	// DeleteMemifInterface calls MemifDelete bin API.
	DeleteMemifInterface(ifName string, idx uint32) error
	// AddTapInterface calls TapConnect bin API.
	AddTapInterface(ifName string, tapIf *interfaces.TapLink) (swIfIdx uint32, err error)
	// DeleteTapInterface calls TapDelete bin API.
	DeleteTapInterface(ifName string, idx uint32, version uint32) error
	// AddVxLanTunnel calls AddDelVxLanTunnelReq with flag add=1.
	// Note: VxLAN tunnel also creates a VRF table with proper IP version if needed
	AddVxLanTunnel(ifName string, vrf, multicastIf uint32, vxLan *interfaces.VxlanLink) (swIndex uint32, err error)
	// DeleteVxLanTunnel calls AddDelVxLanTunnelReq with flag add=0.
	DeleteVxLanTunnel(ifName string, idx, vrf uint32, vxLan *interfaces.VxlanLink) error
	// AddIPSecTunnelInterface adds a new IPSec tunnel interface
	AddIPSecTunnelInterface(ifName string, ipSecLink *interfaces.IPSecLink) (uint32, error)
	// DeleteIPSecTunnelInterface removes existing IPSec tunnel interface
	DeleteIPSecTunnelInterface(ifName string, ipSecLink *interfaces.IPSecLink) error
	// AddVmxNet3 configures vmxNet3 interface. Second parameter is optional in this case.
	AddVmxNet3(ifName string, vmxNet3 *interfaces.VmxNet3Link) (uint32, error)
	// DeleteVmxNet3 removes vmxNet3 interface
	DeleteVmxNet3(ifName string, ifIdx uint32) error
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
	SetRxMode(ifIdx uint32, rxModeSettings *interfaces.Interface_RxModeSettings) error
	// SetRxPlacement configures rx-placement for interface
	SetRxPlacement(ifIdx uint32, rxPlacement *interfaces.Interface_RxPlacementSettings) error
	// CreateVrf checks if VRF exists and creates it if not
	CreateVrf(vrfID uint32) error
	// CreateVrfIPv6 checks if IPv6 VRF exists and creates it if not
	CreateVrfIPv6(vrfID uint32) error
	// SetInterfaceVrf retrieves VRF table from interface
	SetInterfaceVrf(ifaceIndex, vrfID uint32) error
	// SetInterfaceVrfIPv6 retrieves IPV6 VRF table from interface
	SetInterfaceVrfIPv6(ifaceIndex, vrfID uint32) error
	// CreateSubif creates sub interface.
	CreateSubif(ifIdx, vlanID uint32) (swIfIdx uint32, err error)
	// DeleteSubif deletes sub interface.
	DeleteSubif(ifIdx uint32) error
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
	DumpInterfacesByType(reqType interfaces.Interface_Type) (map[uint32]*InterfaceDetails, error)
	// GetInterfaceVrf reads VRF table to interface
	GetInterfaceVrf(ifIdx uint32) (vrfID uint32, err error)
	// GetInterfaceVrfIPv6 reads IPv6 VRF table to interface
	GetInterfaceVrfIPv6(ifIdx uint32) (vrfID uint32, err error)
	// DumpMemifSocketDetails dumps memif socket details from the VPP
	DumpMemifSocketDetails() (map[string]uint32, error)
	// DumpDhcpClients dumps DHCP-related information for all interfaces.
	DumpDhcpClients() (map[uint32]*Dhcp, error)
}

// IfVppHandler is accessor for interface-related vppcalls methods
type IfVppHandler struct {
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
