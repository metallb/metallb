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

package vppcalls

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/dhcp"
	binapi_interface "github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ipsec"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/memif"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/tap"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/tapv2"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vmxnet3"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vxlan"
)

// Default VPP MTU value
const defaultVPPMtu = 9216

func getMtu(vppMtu uint16) uint32 {
	// If default VPP MTU value is set, return 0 (it means MTU was not set in the NB config)
	if vppMtu == defaultVPPMtu {
		return 0
	}
	return uint32(vppMtu)
}

// InterfaceDetails is the wrapper structure for the interface northbound API structure.
type InterfaceDetails struct {
	Interface *interfaces.Interface `json:"interface"`
	Meta      *InterfaceMeta        `json:"interface_meta"`
}

// InterfaceMeta is combination of proto-modelled Interface data and VPP provided metadata
type InterfaceMeta struct {
	SwIfIndex    uint32 `json:"sw_if_index"`
	SupSwIfIndex uint32 `json:"sub_sw_if_index"`
	Tag          string `json:"tag"`
	InternalName string `json:"internal_name"`
	Dhcp         *Dhcp  `json:"dhcp"`
	SubID        uint32 `json:"sub_id"`
	VrfIPv4      uint32 `json:"vrf_ipv4"`
	VrfIPv6      uint32 `json:"vrf_ipv6"`
	Pci          uint32 `json:"pci"`
}

// Dhcp is helper struct for DHCP metadata, split to client and lease (similar to VPP binary API)
type Dhcp struct {
	Client *Client `json:"dhcp_client"`
	Lease  *Lease  `json:"dhcp_lease"`
}

// Client is helper struct grouping DHCP client data
type Client struct {
	SwIfIndex        uint32
	Hostname         string
	ID               string
	WantDhcpEvent    bool
	SetBroadcastFlag bool
	Pid              uint32
}

// Lease is helper struct grouping DHCP lease data
type Lease struct {
	SwIfIndex     uint32
	State         uint8
	Hostname      string
	IsIPv6        bool
	HostAddress   string
	RouterAddress string
	HostMac       string
}

// DumpInterfacesByType implements interface handler.
func (h *IfVppHandler) DumpInterfacesByType(reqType interfaces.Interface_Type) (map[uint32]*InterfaceDetails, error) {
	// Dump all
	ifs, err := h.DumpInterfaces()
	if err != nil {
		return nil, err
	}
	// Filter by type
	for ifIdx, ifData := range ifs {
		if ifData.Interface.Type != reqType {
			delete(ifs, ifIdx)
		}
	}

	return ifs, nil
}

// DumpInterfaces implements interface handler.
func (h *IfVppHandler) DumpInterfaces() (map[uint32]*InterfaceDetails, error) {
	// map for the resulting interfaces
	ifs := make(map[uint32]*InterfaceDetails)

	// First, dump all interfaces to create initial data.
	reqCtx := h.callsChannel.SendMultiRequest(&binapi_interface.SwInterfaceDump{})
	for {
		ifDetails := &binapi_interface.SwInterfaceDetails{}
		stop, err := reqCtx.ReceiveReply(ifDetails)
		if stop {
			break // Break from the loop.
		}
		if err != nil {
			return nil, fmt.Errorf("failed to dump interface: %v", err)
		}

		ifaceName := cleanString(ifDetails.InterfaceName)
		details := &InterfaceDetails{
			Interface: &interfaces.Interface{
				Name:        cleanString(ifDetails.Tag),
				Type:        guessInterfaceType(ifaceName), // the type may be amended later by further dumps
				Enabled:     ifDetails.AdminUpDown > 0,
				PhysAddress: net.HardwareAddr(ifDetails.L2Address[:ifDetails.L2AddressLength]).String(),
				Mtu:         getMtu(ifDetails.LinkMtu),
			},
			Meta: &InterfaceMeta{
				SwIfIndex:    ifDetails.SwIfIndex,
				Tag:          cleanString(ifDetails.Tag),
				InternalName: ifaceName,
				SubID:        ifDetails.SubID,
				SupSwIfIndex: ifDetails.SupSwIfIndex,
			},
		}

		// sub interface
		if ifDetails.SupSwIfIndex != ifDetails.SwIfIndex {
			details.Interface.Type = interfaces.Interface_SUB_INTERFACE
			details.Interface.Link = &interfaces.Interface_Sub{
				Sub: &interfaces.SubInterface{
					ParentName: ifs[ifDetails.SupSwIfIndex].Interface.Name,
					SubId:      ifDetails.SubID,
				},
			}
		}
		// Fill name for physical interfaces (they are mostly without tag)
		switch details.Interface.Type {
		case interfaces.Interface_DPDK:
			details.Interface.Name = ifaceName
		case interfaces.Interface_AF_PACKET:
			details.Interface.Link = &interfaces.Interface_Afpacket{
				Afpacket: &interfaces.AfpacketLink{
					HostIfName: strings.TrimPrefix(ifaceName, "host-"),
				},
			}
		}
		ifs[ifDetails.SwIfIndex] = details
	}

	// Get DHCP clients
	dhcpClients, err := h.DumpDhcpClients()
	if err != nil {
		return nil, fmt.Errorf("failed to dump interface DHCP clients: %v", err)
	}

	// Get IP addresses before VRF
	err = h.dumpIPAddressDetails(ifs, false, dhcpClients)
	if err != nil {
		return nil, err
	}
	err = h.dumpIPAddressDetails(ifs, true, dhcpClients)
	if err != nil {
		return nil, err
	}

	// Get unnumbered interfaces
	unnumbered, err := h.dumpUnnumberedDetails()
	if err != nil {
		return nil, fmt.Errorf("failed to dump unnumbered interfaces: %v", err)
	}
	// Get interface VRF for every IP family, fill DHCP if set and resolve unnumbered interface setup
	for _, ifData := range ifs {
		// VRF is stored in metadata for both, IPv4 and IPv6. If the interface is an IPv6 interface (it contains at least
		// one IPv6 address), appropriate VRF is stored also in modelled data
		ipv4Vrf, err := h.GetInterfaceVrf(ifData.Meta.SwIfIndex)
		if err != nil {
			return nil, fmt.Errorf("interface dump: failed to get IPv4 VRF from interface %d: %v",
				ifData.Meta.SwIfIndex, err)
		}
		ifData.Meta.VrfIPv4 = ipv4Vrf
		ipv6Vrf, err := h.GetInterfaceVrfIPv6(ifData.Meta.SwIfIndex)
		if err != nil {
			return nil, fmt.Errorf("interface dump: failed to get IPv6 VRF from interface %d: %v",
				ifData.Meta.SwIfIndex, err)
		}
		ifData.Meta.VrfIPv6 = ipv6Vrf
		if isIPv6If, err := h.isIpv6Interface(ifData.Interface); err != nil {
			return ifs, err
		} else if isIPv6If {
			ifData.Interface.Vrf = ipv6Vrf
		} else {
			ifData.Interface.Vrf = ipv4Vrf
		}

		// DHCP
		dhcpData, ok := dhcpClients[ifData.Meta.SwIfIndex]
		if ok {
			ifData.Interface.SetDhcpClient = true
			ifData.Meta.Dhcp = dhcpData
		}
		// Unnumbered
		ifWithIPIdx, ok := unnumbered[ifData.Meta.SwIfIndex]
		if ok {
			// Find unnumbered interface
			var ifWithIPName string
			ifWithIP, ok := ifs[ifWithIPIdx]
			if ok {
				ifWithIPName = ifWithIP.Interface.Name
			} else {
				h.log.Debugf("cannot find name of the ip-interface for unnumbered %s", ifData.Interface.Name)
				ifWithIPName = "<unknown>"
			}
			ifData.Interface.Unnumbered = &interfaces.Interface_Unnumbered{
				InterfaceWithIp: ifWithIPName,
			}
		}
	}

	err = h.dumpMemifDetails(ifs)
	if err != nil {
		return nil, err
	}

	err = h.dumpTapDetails(ifs)
	if err != nil {
		return nil, err
	}

	err = h.dumpVxlanDetails(ifs)
	if err != nil {
		return nil, err
	}

	err = h.dumpIPSecTunnelDetails(ifs)
	if err != nil {
		return nil, err
	}

	err = h.dumpVmxNet3Details(ifs)
	if err != nil {
		return nil, err
	}

	// Rx-placement dump is last since it uses interface type-specific data
	err = h.dumpRxPlacement(ifs)
	if err != nil {
		return nil, err
	}

	return ifs, nil
}

// DumpMemifSocketDetails implements interface handler.
func (h *IfVppHandler) DumpMemifSocketDetails() (map[string]uint32, error) {
	memifSocketMap := make(map[string]uint32)

	reqCtx := h.callsChannel.SendMultiRequest(&memif.MemifSocketFilenameDump{})
	for {
		socketDetails := &memif.MemifSocketFilenameDetails{}
		stop, err := reqCtx.ReceiveReply(socketDetails)
		if stop {
			break // Break from the loop.
		}
		if err != nil {
			return memifSocketMap, fmt.Errorf("failed to dump memif socket filename details: %v", err)
		}

		filename := string(bytes.SplitN(socketDetails.SocketFilename, []byte{0x00}, 2)[0])
		memifSocketMap[filename] = socketDetails.SocketID
	}

	h.log.Debugf("Memif socket dump completed, found %d entries: %v", len(memifSocketMap), memifSocketMap)

	return memifSocketMap, nil
}

// DumpDhcpClients returns a slice of DhcpMeta with all interfaces and other DHCP-related information available
func (h *IfVppHandler) DumpDhcpClients() (map[uint32]*Dhcp, error) {
	dhcpData := make(map[uint32]*Dhcp)
	reqCtx := h.callsChannel.SendMultiRequest(&dhcp.DHCPClientDump{})

	for {
		dhcpDetails := &dhcp.DHCPClientDetails{}
		last, err := reqCtx.ReceiveReply(dhcpDetails)
		if last {
			break
		}
		if err != nil {
			return nil, err
		}
		client := dhcpDetails.Client
		lease := dhcpDetails.Lease

		var hostMac net.HardwareAddr = lease.HostMac
		var hostAddr, routerAddr string
		if uintToBool(lease.IsIPv6) {
			hostAddr = fmt.Sprintf("%s/%d", net.IP(lease.HostAddress).To16().String(), uint32(lease.MaskWidth))
			routerAddr = fmt.Sprintf("%s/%d", net.IP(lease.RouterAddress).To16().String(), uint32(lease.MaskWidth))
		} else {
			hostAddr = fmt.Sprintf("%s/%d", net.IP(lease.HostAddress[:4]).To4().String(), uint32(lease.MaskWidth))
			routerAddr = fmt.Sprintf("%s/%d", net.IP(lease.RouterAddress[:4]).To4().String(), uint32(lease.MaskWidth))
		}

		// DHCP client data
		dhcpClient := &Client{
			SwIfIndex:        client.SwIfIndex,
			Hostname:         string(bytes.SplitN(client.Hostname, []byte{0x00}, 2)[0]),
			ID:               string(bytes.SplitN(client.ID, []byte{0x00}, 2)[0]),
			WantDhcpEvent:    uintToBool(client.WantDHCPEvent),
			SetBroadcastFlag: uintToBool(client.SetBroadcastFlag),
			Pid:              client.PID,
		}

		// DHCP lease data
		dhcpLease := &Lease{
			SwIfIndex:     lease.SwIfIndex,
			State:         lease.State,
			Hostname:      string(bytes.SplitN(lease.Hostname, []byte{0x00}, 2)[0]),
			IsIPv6:        uintToBool(lease.IsIPv6),
			HostAddress:   hostAddr,
			RouterAddress: routerAddr,
			HostMac:       hostMac.String(),
		}

		// DHCP metadata
		dhcpData[client.SwIfIndex] = &Dhcp{
			Client: dhcpClient,
			Lease:  dhcpLease,
		}
	}

	return dhcpData, nil
}

// Returns true if given interface contains at least one IPv6 address. For VxLAN, source and destination
// addresses are also checked
func (h *IfVppHandler) isIpv6Interface(iface *interfaces.Interface) (bool, error) {
	if iface.Type == interfaces.Interface_VXLAN_TUNNEL && iface.GetVxlan() != nil {
		if ipAddress := net.ParseIP(iface.GetVxlan().SrcAddress); ipAddress.To4() == nil {
			return true, nil
		}
		if ipAddress := net.ParseIP(iface.GetVxlan().DstAddress); ipAddress.To4() == nil {
			return true, nil
		}
	}
	for _, ifAddress := range iface.IpAddresses {
		if ipAddress, _, err := net.ParseCIDR(ifAddress); err != nil {
			return false, err
		} else if ipAddress.To4() == nil {
			return true, nil
		}
	}
	return false, nil
}

// dumpIPAddressDetails dumps IP address details of interfaces from VPP and fills them into the provided interface map.
func (h *IfVppHandler) dumpIPAddressDetails(ifs map[uint32]*InterfaceDetails, isIPv6 bool, dhcpClients map[uint32]*Dhcp) error {
	// Dump IP addresses of each interface.
	for idx := range ifs {
		reqCtx := h.callsChannel.SendMultiRequest(&ip.IPAddressDump{
			SwIfIndex: idx,
			IsIPv6:    boolToUint(isIPv6),
		})
		for {
			ipDetails := &ip.IPAddressDetails{}
			stop, err := reqCtx.ReceiveReply(ipDetails)
			if stop {
				break // Break from the loop.
			}
			if err != nil {
				return fmt.Errorf("failed to dump interface %d IP address details: %v", idx, err)
			}
			h.processIPDetails(ifs, ipDetails, dhcpClients)
		}
	}

	return nil
}

// processIPDetails processes ip.IPAddressDetails binary API message and fills the details into the provided interface map.
func (h *IfVppHandler) processIPDetails(ifs map[uint32]*InterfaceDetails, ipDetails *ip.IPAddressDetails, dhcpClients map[uint32]*Dhcp) {
	ifDetails, ifIdxExists := ifs[ipDetails.SwIfIndex]
	if !ifIdxExists {
		return
	}

	var ipAddr string
	if ipDetails.IsIPv6 == 1 {
		ipAddr = fmt.Sprintf("%s/%d", net.IP(ipDetails.IP).To16().String(), uint32(ipDetails.PrefixLength))
	} else {
		ipAddr = fmt.Sprintf("%s/%d", net.IP(ipDetails.IP[:4]).To4().String(), uint32(ipDetails.PrefixLength))
	}

	// skip IP addresses given by DHCP
	if dhcpClient, hasDhcpClient := dhcpClients[ipDetails.SwIfIndex]; hasDhcpClient {
		if dhcpClient.Lease != nil && dhcpClient.Lease.HostAddress == ipAddr {
			return
		}
	}

	ifDetails.Interface.IpAddresses = append(ifDetails.Interface.IpAddresses, ipAddr)
}

// dumpMemifDetails dumps memif interface details from VPP and fills them into the provided interface map.
func (h *IfVppHandler) dumpMemifDetails(ifs map[uint32]*InterfaceDetails) error {
	// Dump all memif sockets
	memifSocketMap, err := h.DumpMemifSocketDetails()
	if err != nil {
		return err
	}

	reqCtx := h.callsChannel.SendMultiRequest(&memif.MemifDump{})
	for {
		memifDetails := &memif.MemifDetails{}
		stop, err := reqCtx.ReceiveReply(memifDetails)
		if stop {
			break // Break from the loop.
		}
		if err != nil {
			return fmt.Errorf("failed to dump memif interface: %v", err)
		}
		_, ifIdxExists := ifs[memifDetails.SwIfIndex]
		if !ifIdxExists {
			continue
		}
		ifs[memifDetails.SwIfIndex].Interface.Link = &interfaces.Interface_Memif{
			Memif: &interfaces.MemifLink{
				Master: memifDetails.Role == 0,
				Mode:   memifModetoNB(memifDetails.Mode),
				Id:     memifDetails.ID,
				//Secret: // TODO: Secret - not available in the binary API
				SocketFilename: func(socketMap map[string]uint32) (filename string) {
					for filename, id := range socketMap {
						if memifDetails.SocketID == id {
							return filename
						}
					}
					// Socket for configured memif should exist
					h.log.Warnf("Socket ID not found for memif %v", memifDetails.SwIfIndex)
					return
				}(memifSocketMap),
				RingSize:   memifDetails.RingSize,
				BufferSize: uint32(memifDetails.BufferSize),
				// TODO: RxQueues, TxQueues - not available in the binary API
				//RxQueues:
				//TxQueues:
			},
		}
		ifs[memifDetails.SwIfIndex].Interface.Type = interfaces.Interface_MEMIF
	}

	return nil
}

// dumpTapDetails dumps tap interface details from VPP and fills them into the provided interface map.
func (h *IfVppHandler) dumpTapDetails(ifs map[uint32]*InterfaceDetails) error {
	// Original TAP.
	reqCtx := h.callsChannel.SendMultiRequest(&tap.SwInterfaceTapDump{})
	for {
		tapDetails := &tap.SwInterfaceTapDetails{}
		stop, err := reqCtx.ReceiveReply(tapDetails)
		if stop {
			break // Break from the loop.
		}
		if err != nil {
			return fmt.Errorf("failed to dump TAP interface details: %v", err)
		}
		_, ifIdxExists := ifs[tapDetails.SwIfIndex]
		if !ifIdxExists {
			continue
		}
		ifs[tapDetails.SwIfIndex].Interface.Link = &interfaces.Interface_Tap{
			Tap: &interfaces.TapLink{
				Version:    1,
				HostIfName: string(bytes.SplitN(tapDetails.DevName, []byte{0x00}, 2)[0]),
			},
		}
		ifs[tapDetails.SwIfIndex].Interface.Type = interfaces.Interface_TAP
	}

	// TAP v.2
	reqCtx = h.callsChannel.SendMultiRequest(&tapv2.SwInterfaceTapV2Dump{})
	for {
		tapDetails := &tapv2.SwInterfaceTapV2Details{}
		stop, err := reqCtx.ReceiveReply(tapDetails)
		if stop {
			break // Break from the loop.
		}
		if err != nil {
			return fmt.Errorf("failed to dump TAPv2 interface details: %v", err)
		}
		_, ifIdxExists := ifs[tapDetails.SwIfIndex]
		if !ifIdxExists {
			continue
		}
		ifs[tapDetails.SwIfIndex].Interface.Link = &interfaces.Interface_Tap{
			Tap: &interfaces.TapLink{
				Version:    2,
				HostIfName: string(bytes.SplitN(tapDetails.HostIfName, []byte{0x00}, 2)[0]),
				// Other parameters are not not yet part of the dump.
			},
		}
		ifs[tapDetails.SwIfIndex].Interface.Type = interfaces.Interface_TAP
	}

	return nil
}

// dumpVxlanDetails dumps VXLAN interface details from VPP and fills them into the provided interface map.
func (h *IfVppHandler) dumpVxlanDetails(ifs map[uint32]*InterfaceDetails) error {
	reqCtx := h.callsChannel.SendMultiRequest(&vxlan.VxlanTunnelDump{SwIfIndex: ^uint32(0)})
	for {
		vxlanDetails := &vxlan.VxlanTunnelDetails{}
		stop, err := reqCtx.ReceiveReply(vxlanDetails)
		if stop {
			break // Break from the loop.
		}
		if err != nil {
			return fmt.Errorf("failed to dump VxLAN tunnel interface details: %v", err)
		}
		_, ifIdxExists := ifs[vxlanDetails.SwIfIndex]
		if !ifIdxExists {
			continue
		}
		// Multicast interface
		var multicastIfName string
		_, exists := ifs[vxlanDetails.McastSwIfIndex]
		if exists {
			multicastIfName = ifs[vxlanDetails.McastSwIfIndex].Interface.Name
		}

		if vxlanDetails.IsIPv6 == 1 {
			ifs[vxlanDetails.SwIfIndex].Interface.Link = &interfaces.Interface_Vxlan{
				Vxlan: &interfaces.VxlanLink{
					Multicast:  multicastIfName,
					SrcAddress: net.IP(vxlanDetails.SrcAddress).To16().String(),
					DstAddress: net.IP(vxlanDetails.DstAddress).To16().String(),
					Vni:        vxlanDetails.Vni,
				},
			}
		} else {
			ifs[vxlanDetails.SwIfIndex].Interface.Link = &interfaces.Interface_Vxlan{
				Vxlan: &interfaces.VxlanLink{
					Multicast:  multicastIfName,
					SrcAddress: net.IP(vxlanDetails.SrcAddress[:4]).To4().String(),
					DstAddress: net.IP(vxlanDetails.DstAddress[:4]).To4().String(),
					Vni:        vxlanDetails.Vni,
				},
			}
		}
		ifs[vxlanDetails.SwIfIndex].Interface.Type = interfaces.Interface_VXLAN_TUNNEL
	}

	return nil
}

// dumpIPSecTunnelDetails dumps IPSec tunnel interfaces from the VPP and fills them into the provided interface map.
func (h *IfVppHandler) dumpIPSecTunnelDetails(ifs map[uint32]*InterfaceDetails) error {
	// tunnel interfaces are a part of security association dump
	var tunnels []*ipsec.IpsecSaDetails
	req := &ipsec.IpsecSaDump{
		SaID: ^uint32(0),
	}
	requestCtx := h.callsChannel.SendMultiRequest(req)

	for {
		saDetails := &ipsec.IpsecSaDetails{}
		stop, err := requestCtx.ReceiveReply(saDetails)
		if stop {
			break
		}
		if err != nil {
			return err
		}
		// skip non-tunnel security associations
		if saDetails.SwIfIndex != ^uint32(0) {
			tunnels = append(tunnels, saDetails)
		}
	}

	// every tunnel interface is returned in two API calls. To reconstruct the correct proto-modelled data,
	// first appearance is cached, and when the second part arrives, data are completed and stored.
	tunnelParts := make(map[uint32]*ipsec.IpsecSaDetails)

	for _, tunnel := range tunnels {
		// first appearance is stored in the map, the second one is used in configuration.
		firstSaData, ok := tunnelParts[tunnel.SwIfIndex]
		if !ok {
			tunnelParts[tunnel.SwIfIndex] = tunnel
			continue
		}

		var tunnelSrcAddrStr, tunnelDstAddrStr string
		if uintToBool(tunnel.IsTunnelIP6) {
			var tunnelSrcAddr, tunnelDstAddr net.IP = tunnel.TunnelSrcAddr, tunnel.TunnelDstAddr
			tunnelSrcAddrStr, tunnelDstAddrStr = tunnelSrcAddr.String(), tunnelDstAddr.String()
		} else {
			var tunnelSrcAddr, tunnelDstAddr net.IP = tunnel.TunnelSrcAddr[:4], tunnel.TunnelDstAddr[:4]
			tunnelSrcAddrStr, tunnelDstAddrStr = tunnelSrcAddr.String(), tunnelDstAddr.String()
		}

		ifs[tunnel.SwIfIndex].Interface.Link = &interfaces.Interface_Ipsec{
			Ipsec: &interfaces.IPSecLink{
				Esn:        uintToBool(tunnel.UseEsn),
				AntiReplay: uintToBool(tunnel.UseAntiReplay),
				LocalIp:    tunnelSrcAddrStr,
				RemoteIp:   tunnelDstAddrStr,
				LocalSpi:   tunnel.Spi,
				// fll remote SPI from stored SA data
				RemoteSpi:      firstSaData.Spi,
				CryptoAlg:      interfaces.IPSecLink_CryptoAlg(tunnel.CryptoAlg),
				IntegAlg:       interfaces.IPSecLink_IntegAlg(tunnel.IntegAlg),
				EnableUdpEncap: uintToBool(tunnel.UDPEncap),
			},
		}
		ifs[tunnel.SwIfIndex].Interface.Type = interfaces.Interface_IPSEC_TUNNEL
	}

	return nil
}

// dumpVmxNet3Details dumps VmxNet3 interface details from VPP and fills them into the provided interface map.
func (h *IfVppHandler) dumpVmxNet3Details(ifs map[uint32]*InterfaceDetails) error {
	reqCtx := h.callsChannel.SendMultiRequest(&vmxnet3.Vmxnet3Dump{})
	for {
		vmxnet3Details := &vmxnet3.Vmxnet3Details{}
		stop, err := reqCtx.ReceiveReply(vmxnet3Details)
		if stop {
			break // Break from the loop.
		}
		if err != nil {
			return fmt.Errorf("failed to dump VmxNet3 tunnel interface details: %v", err)
		}
		_, ifIdxExists := ifs[vmxnet3Details.SwIfIndex]
		if !ifIdxExists {
			continue
		}
		ifs[vmxnet3Details.SwIfIndex].Interface.Link = &interfaces.Interface_VmxNet3{
			VmxNet3: &interfaces.VmxNet3Link{
				RxqSize: uint32(vmxnet3Details.RxQsize),
				TxqSize: uint32(vmxnet3Details.TxQsize),
			},
		}
		ifs[vmxnet3Details.SwIfIndex].Interface.Type = interfaces.Interface_VMXNET3_INTERFACE
		ifs[vmxnet3Details.SwIfIndex].Meta.Pci = vmxnet3Details.PciAddr
	}
	return nil
}

// dumpUnnumberedDetails returns a map of unnumbered interface indexes, every with interface index of element with IP
func (h *IfVppHandler) dumpUnnumberedDetails() (map[uint32]uint32, error) {
	unIfMap := make(map[uint32]uint32) // unnumbered/ip-interface
	reqCtx := h.callsChannel.SendMultiRequest(&ip.IPUnnumberedDump{
		SwIfIndex: ^uint32(0),
	})

	for {
		unDetails := &ip.IPUnnumberedDetails{}
		last, err := reqCtx.ReceiveReply(unDetails)
		if last {
			break
		}
		if err != nil {
			return nil, err
		}

		unIfMap[unDetails.SwIfIndex] = unDetails.IPSwIfIndex
	}

	return unIfMap, nil
}

func (h *IfVppHandler) dumpRxPlacement(ifs map[uint32]*InterfaceDetails) error {
	reqCtx := h.callsChannel.SendMultiRequest(&binapi_interface.SwInterfaceRxPlacementDump{
		SwIfIndex: ^uint32(0),
	})
	for {
		rxDetails := &binapi_interface.SwInterfaceRxPlacementDetails{}
		stop, err := reqCtx.ReceiveReply(rxDetails)
		if err != nil {
			return fmt.Errorf("failed to dump rx-placement details: %v", err)
		}
		if stop {
			break
		}
		ifData, ok := ifs[rxDetails.SwIfIndex]
		if !ok {
			h.log.Warnf("Received rx-placement data for unknown interface with index %d", rxDetails.SwIfIndex)
			continue
		}
		ifData.Interface.RxModeSettings = &interfaces.Interface_RxModeSettings{
			RxMode:  getRxModeType(rxDetails.Mode),
			QueueId: rxDetails.QueueID,
		}
		ifData.Interface.RxPlacementSettings = &interfaces.Interface_RxPlacementSettings{
			Queue:  rxDetails.QueueID,
			Worker: rxDetails.WorkerID,
		}
	}
	return nil
}

// guessInterfaceType attempts to guess the correct interface type from its internal name (as given by VPP).
// This is required mainly for those interface types, that do not provide dump binary API,
// such as loopback of af_packet.
func guessInterfaceType(ifName string) interfaces.Interface_Type {
	switch {
	case strings.HasPrefix(ifName, "loop"),
		strings.HasPrefix(ifName, "local"):
		return interfaces.Interface_SOFTWARE_LOOPBACK

	case strings.HasPrefix(ifName, "memif"):
		return interfaces.Interface_MEMIF

	case strings.HasPrefix(ifName, "tap"):
		return interfaces.Interface_TAP

	case strings.HasPrefix(ifName, "host"):
		return interfaces.Interface_AF_PACKET

	case strings.HasPrefix(ifName, "vxlan"):
		return interfaces.Interface_VXLAN_TUNNEL

	case strings.HasPrefix(ifName, "ipsec"):
		return interfaces.Interface_IPSEC_TUNNEL
	case strings.HasPrefix(ifName, "vmxnet3"):
		return interfaces.Interface_VMXNET3_INTERFACE

	default:
		return interfaces.Interface_DPDK
	}
}

// memifModetoNB converts binary API type of memif mode to the northbound API type memif mode.
func memifModetoNB(mode uint8) interfaces.MemifLink_MemifMode {
	switch mode {
	case 0:
		return interfaces.MemifLink_ETHERNET
	case 1:
		return interfaces.MemifLink_IP
	case 2:
		return interfaces.MemifLink_PUNT_INJECT
	default:
		return interfaces.MemifLink_ETHERNET
	}
}

// Convert binary API rx-mode to northbound representation
func getRxModeType(mode uint8) interfaces.Interface_RxModeSettings_RxModeType {
	switch mode {
	case 1:
		return interfaces.Interface_RxModeSettings_POLLING
	case 2:
		return interfaces.Interface_RxModeSettings_INTERRUPT
	case 3:
		return interfaces.Interface_RxModeSettings_ADAPTIVE
	case 4:
		return interfaces.Interface_RxModeSettings_DEFAULT
	default:
		return interfaces.Interface_RxModeSettings_UNKNOWN
	}
}

func uintToBool(value uint8) bool {
	if value == 0 {
		return false
	}
	return true
}

func cleanString(b []byte) string {
	return string(bytes.SplitN(b, []byte{0x00}, 2)[0])
}
