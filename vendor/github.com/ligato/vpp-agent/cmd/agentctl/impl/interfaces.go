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

package impl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"

	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/spf13/cobra"
)

// IfaceCommonFields are fields common for interfaces of any type.
type IfaceCommonFields struct {
	Name      string
	Desc      string
	IfType    string
	Enabled   bool
	PhysAddr  string
	Mtu       uint32
	Ipv4Addrs []string
	Ipv6Addrs []string
}

var ifCommonFields IfaceCommonFields

// PutAfPkt creates an Af-packet type interface.
func PutAfPkt(endpoints []string, label string, flags *interfaces.Interfaces_Interface_Afpacket) {
	found, key, ifc, db := utils.GetInterfaceKeyAndValue(endpoints, label, ifCommonFields.Name)
	processCommonIfFlags(found, interfaces.InterfaceType_AF_PACKET_INTERFACE, ifc)

	// Process Af-Packet specific flags.
	if flags.HostIfName != "" {
		if ifc.Afpacket == nil {
			ifc.Afpacket = &interfaces.Interfaces_Interface_Afpacket{}
		}
		ifc.Afpacket.HostIfName = flags.HostIfName
	}
	utils.WriteInterfaceToDb(db, key, ifc)
}

// PutEthernet creates an ethernet type interface.
func PutEthernet(endpoints []string, label string) {
	found, key, ifc, db := utils.GetInterfaceKeyAndValue(endpoints, label, ifCommonFields.Name)
	processCommonIfFlags(found, interfaces.InterfaceType_ETHERNET_CSMACD, ifc)
	utils.WriteInterfaceToDb(db, key, ifc)
}

// PutLoopback creates a loopback type interface.
func PutLoopback(endpoints []string, label string) {
	found, key, ifc, db := utils.GetInterfaceKeyAndValue(endpoints, label, ifCommonFields.Name)
	processCommonIfFlags(found, interfaces.InterfaceType_TAP_INTERFACE, ifc)
	utils.WriteInterfaceToDb(db, key, ifc)
}

// PutMemif creates a memif type interface.
func PutMemif(endpoints []string, label string, flags *interfaces.Interfaces_Interface_Memif) {
	found, key, ifc, db := utils.GetInterfaceKeyAndValue(endpoints, label, ifCommonFields.Name)
	processCommonIfFlags(found, interfaces.InterfaceType_MEMORY_INTERFACE, ifc)

	// Process MEMIF-specific flags.
	if utils.IsFlagPresent(utils.MemifMaster) {
		if ifc.Memif == nil {
			ifc.Memif = &interfaces.Interfaces_Interface_Memif{}
		}
		ifc.Memif.Master = flags.Master
	}
	if utils.IsFlagPresent(utils.MemifMode) {
		if ifc.Memif == nil {
			ifc.Memif = &interfaces.Interfaces_Interface_Memif{}
		}
		ifc.Memif.Mode = flags.Mode
	}
	if utils.IsFlagPresent(utils.MemifID) {
		if ifc.Memif == nil {
			ifc.Memif = &interfaces.Interfaces_Interface_Memif{}
		}
		ifc.Memif.Id = flags.Id
	}
	if flags.SocketFilename != "" {
		if ifc.Memif == nil {
			ifc.Memif = &interfaces.Interfaces_Interface_Memif{}
		}
		ifc.Memif.SocketFilename = flags.SocketFilename
	}
	if flags.Secret != "" {
		if ifc.Memif == nil {
			ifc.Memif = &interfaces.Interfaces_Interface_Memif{}
		}
		ifc.Memif.Secret = flags.Secret
	}
	if utils.IsFlagPresent(utils.MemifRingSize) {
		if ifc.Memif == nil {
			ifc.Memif = &interfaces.Interfaces_Interface_Memif{}
		}
		ifc.Memif.RingSize = flags.RingSize
	}
	if utils.IsFlagPresent(utils.MemifBufferSize) {
		if ifc.Memif == nil {
			ifc.Memif = &interfaces.Interfaces_Interface_Memif{}
		}
		ifc.Memif.BufferSize = flags.BufferSize
	}
	if utils.IsFlagPresent(utils.MemifRxQueues) {
		if ifc.Memif == nil {
			ifc.Memif = &interfaces.Interfaces_Interface_Memif{}
		}
		ifc.Memif.RxQueues = flags.RxQueues
	}
	if utils.IsFlagPresent(utils.MemifTxQueues) {
		if ifc.Memif == nil {
			ifc.Memif = &interfaces.Interfaces_Interface_Memif{}
		}
		ifc.Memif.TxQueues = flags.TxQueues
	}
	utils.WriteInterfaceToDb(db, key, ifc)
}

// PutTap creates a tap type interface.
func PutTap(endpoints []string, label string) {
	found, key, ifc, db := utils.GetInterfaceKeyAndValue(endpoints, label, ifCommonFields.Name)
	processCommonIfFlags(found, interfaces.InterfaceType_TAP_INTERFACE, ifc)
	utils.WriteInterfaceToDb(db, key, ifc)
}

// PutVxLan creates a vxlan type interface.
func PutVxLan(endpoints []string, label string, flags *interfaces.Interfaces_Interface_Vxlan) {
	found, key, ifc, db := utils.GetInterfaceKeyAndValue(endpoints, label, ifCommonFields.Name)
	processCommonIfFlags(found, interfaces.InterfaceType_VXLAN_TUNNEL, ifc)

	// Process VXLAN-specific flags.
	if flags.SrcAddress != "" {
		if ifc.Vxlan == nil {
			ifc.Vxlan = &interfaces.Interfaces_Interface_Vxlan{}
		}
		if utils.ValidateIpv4Addr(flags.SrcAddress) || utils.ValidateIpv6Addr(flags.SrcAddress) {
			ifc.Vxlan.SrcAddress = flags.SrcAddress
		} else {
			utils.ExitWithError(utils.ExitBadArgs, errors.New("Invalid "+utils.VxLanSrcAddr+" "+flags.SrcAddress))
		}
	}
	if flags.DstAddress != "" {
		if ifc.Vxlan == nil {
			ifc.Vxlan = &interfaces.Interfaces_Interface_Vxlan{}
		}
		if utils.ValidateIpv4Addr(flags.DstAddress) || utils.ValidateIpv6Addr(flags.DstAddress) {
			ifc.Vxlan.DstAddress = flags.DstAddress
		} else {
			utils.ExitWithError(utils.ExitBadArgs, errors.New("Invalid "+utils.VxLanDstAddr+" "+flags.DstAddress))
		}
	}
	if utils.IsFlagPresent(utils.VxLanVni) && flags.Vni > 0 {
		if ifc.Vxlan == nil {
			ifc.Vxlan = &interfaces.Interfaces_Interface_Vxlan{}
		}
		ifc.Vxlan.Vni = flags.Vni
	}
	utils.WriteInterfaceToDb(db, key, ifc)
}

// IfJSONPut creates an interface according to json configuration.
func IfJSONPut(endpoints []string, label string) {
	bio := bufio.NewReader(os.Stdin)
	buf := new(bytes.Buffer)
	buf.ReadFrom(bio)
	input := buf.Bytes()

	ifc := &interfaces.Interfaces_Interface{}
	err := json.Unmarshal(input, ifc)
	if err != nil {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Invalid json, error "+err.Error()))
	}

	key := interfaces.InterfaceKey(ifc.Name)
	db, err := utils.GetDbForOneAgent(endpoints, label)
	if err != nil {
		utils.ExitWithError(utils.ExitBadConnection, err)
	}
	utils.WriteInterfaceToDb(db, key, ifc)
}

// InterfaceDel removes the interface with defined name.
func InterfaceDel(endpoints []string, label string) {
	found, key, _, db := utils.GetInterfaceKeyAndValue(endpoints, label, ifCommonFields.Name)
	if found {
		db.Delete(key)
	}
}

// AddInterfaceNameFlag adds a name flag to interface flags.
func AddInterfaceNameFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&ifCommonFields.Name, "name", "n", "", "Interface name")
}

// AddCommonIfPutFlags adds flags common for all interface types.
func AddCommonIfPutFlags(cmd *cobra.Command) {
	AddInterfaceNameFlag(cmd)
	cmd.Flags().StringVarP(&ifCommonFields.Desc, "description", "d", "", "Interface description (ascii text)")
	cmd.Flags().BoolVarP(&ifCommonFields.Enabled, "enabled", "", true, "Enables/disables the interface")
	cmd.Flags().StringVarP(&ifCommonFields.PhysAddr, "phy-addr", "p", "", "Physical (MAC) address for the interface")
	cmd.Flags().Uint32Var(&ifCommonFields.Mtu, "mtu", 1500, "MTU for the interface")
	cmd.Flags().StringSliceVar(&ifCommonFields.Ipv4Addrs, "ipv4-addr", nil, "Comma-separated list of IPv4 addresses in CIDR format, e.g. 172.17.0.1/16")
	cmd.Flags().StringSliceVar(&ifCommonFields.Ipv6Addrs, "ipv6-addr", nil, "Comma-separated list of IPv6 addresses in CIDR format, e.g. 2001:cdba::3257:9652/48")
}

func processCommonIfFlags(found bool, ifType interfaces.InterfaceType, ifc *interfaces.Interfaces_Interface) *interfaces.Interfaces_Interface {

	if found && ifc.Type != ifType {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Bad type for interface '"+ifCommonFields.Name+
			"'. Interface with this name but a different type already exists."))
	}

	if ifType == interfaces.InterfaceType_TAP_INTERFACE {
		ifc.Tap = &interfaces.Interfaces_Interface_Tap{HostIfName: ifCommonFields.Name}
	}

	// Set in case interface is empty.
	ifc.Name = ifCommonFields.Name
	ifc.Type = ifType
	ifc.Enabled = ifCommonFields.Enabled

	if ifCommonFields.Desc != "" {
		ifc.Description = ifCommonFields.Desc
	}
	if ifCommonFields.PhysAddr != "" {
		utils.ValidatePhyAddr(ifCommonFields.PhysAddr)
		ifc.PhysAddress = ifCommonFields.PhysAddr
	}
	if utils.IsFlagPresent("mtu") {
		ifc.Mtu = ifCommonFields.Mtu
	}

	if len(ifCommonFields.Ipv4Addrs) > 0 {
		if ifc.IpAddresses == nil {
			ifc.IpAddresses = []string{}
		}
		ifc.IpAddresses = utils.UpdateIpv4Address(ifc.IpAddresses, ifCommonFields.Ipv4Addrs)
	}

	if len(ifCommonFields.Ipv6Addrs) > 0 {
		if ifc.IpAddresses != nil {
			ifc.IpAddresses = []string{}
		}
		ifc.IpAddresses = utils.UpdateIpv6Address(ifc.IpAddresses, ifCommonFields.Ipv6Addrs)
	}

	return ifc
}
