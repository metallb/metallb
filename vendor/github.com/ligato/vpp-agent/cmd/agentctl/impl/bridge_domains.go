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
	"errors"

	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/spf13/cobra"
)

// BridgeDomainCommonFields defines all fields which can be set using flags in bridge domain config.
type BridgeDomainCommonFields struct {
	Name    string
	Flood   bool
	UUFlood bool
	Forward bool
	Learn   bool
	Arp     bool
	MacAge  uint32
}

// BridgeDomainInterfaceFields defines all fields used as flags in bridge domain interface command.
type BridgeDomainInterfaceFields struct {
	BdName            string
	IfName            string
	Bvi               bool
	SplitHorizonGroup uint32
	IsDelete          bool
}

// BridgeDomainArpFields defines all fields used as flags in bridge domain arp command.
type BridgeDomainArpFields struct {
	BdName      string
	IPAddress   string
	PhysAddress string
	IsDelete    bool
}

// L2FIBEntryFields defines all fields used as flags in bridge domain fib command.
type L2FIBEntryFields struct {
	PhysAddress       string
	BdName            string
	Action            uint8
	OutgoingInterface string
	StaticConfig      bool
	BVI               bool
	IsDelete          bool
}

var bdCommonFields BridgeDomainCommonFields

// CreateUpdateBridgeDomain creates a new bridge domain or updates an old one. All bridge domain attributes
// are set here. New bridge domain is created without attached interfaces, or ARP table, or FIB entries.
func CreateUpdateBridgeDomain(endpoints []string, label string) {
	_, key, bd, db := utils.GetBridgeDomainKeyAndValue(endpoints, label, bdCommonFields.Name)

	bd.Flood = bdCommonFields.Flood
	bd.UnknownUnicastFlood = bdCommonFields.UUFlood
	bd.Forward = bdCommonFields.Forward
	bd.Learn = bdCommonFields.Learn
	bd.ArpTermination = bdCommonFields.Arp
	bd.MacAge = bdCommonFields.MacAge

	utils.WriteBridgeDomainToDb(db, key, bd)
}

// DeleteBridgeDomain removes bridge domain from the configuration, including all attached interfaces and ARP table entries.
func DeleteBridgeDomain(endpoints []string, label string) {
	found, key, _, db := utils.GetBridgeDomainKeyAndValue(endpoints, label, bdCommonFields.Name)
	if found {
		db.Delete(key)
	}
}

// AddUpdateInterfaceToBridgeDomain adds interface to bridge domain.
func AddUpdateInterfaceToBridgeDomain(endpoints []string, label string, iface *BridgeDomainInterfaceFields) {
	// Name flag is mandatory
	if iface.IfName == "" {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Interface does not have name"))
	}
	// Bridge domain has to exist
	found, key, bd, db := utils.GetBridgeDomainKeyAndValue(endpoints, label, iface.BdName)
	if !found {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Interface configured from a nonexisting bridge domain"))
	}
	// Obtain current list of interfaces attached.
	interfaceList := bd.Interfaces
	for i, ifaceEntry := range interfaceList {
		// If interface already exists, remove it. It will be created anew (update).
		if ifaceEntry.Name == iface.IfName {
			interfaceList = append(interfaceList[:i], interfaceList[i+1:]...)
			break
		}
	}
	// Create new bridge domain interface and add it to the list.
	interfaceToAdd := new(l2.BridgeDomains_BridgeDomain_Interfaces)
	interfaceToAdd.Name = iface.IfName
	interfaceToAdd.BridgedVirtualInterface = iface.Bvi
	interfaceToAdd.SplitHorizonGroup = iface.SplitHorizonGroup
	interfaceList = append(interfaceList, interfaceToAdd)
	bd.Interfaces = interfaceList

	utils.WriteBridgeDomainToDb(db, key, bd)
}

// DeleteInterfaceFromBridgeDomain removes interface from bridge domain.
func DeleteInterfaceFromBridgeDomain(endpoints []string, label string, iface *BridgeDomainInterfaceFields) {
	// Name flag is mandatory
	if iface.IfName == "" {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Interface does not have name"))
	}
	// Bridge domain has to exist
	found, key, bd, db := utils.GetBridgeDomainKeyAndValue(endpoints, label, iface.BdName)
	if !found {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Unable to remove interface from a nonexisting bridge domain"))
	}
	// Create a new set of bridge domain interfaces without the removed one.
	var interfaceList []*l2.BridgeDomains_BridgeDomain_Interfaces
	for _, existingInterface := range bd.Interfaces {
		if existingInterface.Name == iface.IfName {
			continue
		}
		interfaceList = append(interfaceList, existingInterface)
	}
	bd.Interfaces = interfaceList

	utils.WriteBridgeDomainToDb(db, key, bd)
}

// AddUpdateArpEntry creates or updates ARP entry in the bridge domain.
func AddUpdateArpEntry(endpoints []string, label string, arp *BridgeDomainArpFields) {
	// IP address is mandatory (identification)
	if arp.IPAddress == "" {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Arp entry does not contain IP address"))
	}
	// Bridge domain has to exist
	found, key, bd, db := utils.GetBridgeDomainKeyAndValue(endpoints, label, arp.BdName)
	if !found {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Arp entry configured from a nonexisting bridge domain"))
	}
	// Obtain current list of ARP entries.
	arpTable := bd.ArpTerminationTable
	for i, arpEntry := range bd.ArpTerminationTable {
		// If ARP entry already exists, remove it. It will be created anew (update).
		if arpEntry.IpAddress == arp.IPAddress {
			arpTable = append(arpTable[:i], arpTable[i+1:]...)
			break
		}
	}
	// Create new bridge domain ARP and add it to the list.
	arpToAdd := new(l2.BridgeDomains_BridgeDomain_ArpTerminationEntry)
	arpToAdd.IpAddress = arp.IPAddress
	arpToAdd.PhysAddress = arp.PhysAddress
	arpTable = append(arpTable, arpToAdd)
	bd.ArpTerminationTable = arpTable

	utils.WriteBridgeDomainToDb(db, key, bd)
}

// DeleteArpEntry removes ARP entry from the bridge domain.
func DeleteArpEntry(endpoints []string, label string, arp *BridgeDomainArpFields) {
	// IP address is mandatory (identification)
	if arp.IPAddress == "" {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Arp entry does not contain IP address"))
	}
	// Bridge domain has to exist
	found, key, bd, db := utils.GetBridgeDomainKeyAndValue(endpoints, label, arp.BdName)
	if !found {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Unable to remove ARP entry from a nonexisting bridge domain"))
	}
	// Remove ARP table from the list.
	var newArpTable []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry
	for _, existingArpEntry := range bd.ArpTerminationTable {
		if existingArpEntry.IpAddress == arp.IPAddress {
			continue
		}
		newArpTable = append(newArpTable, existingArpEntry)
	}
	bd.ArpTerminationTable = newArpTable

	utils.WriteBridgeDomainToDb(db, key, bd)
}

// AddFibEntry adds new FIB entry to the FIB table.
func AddFibEntry(endpoints []string, label string, fib *L2FIBEntryFields) {
	// MAC address is required because it serves as an identification.
	if fib.PhysAddress == "" {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("FIB entry does not contain physical address"))
	}
	// Bridge domain has to exist
	found, _, _, db := utils.GetBridgeDomainKeyAndValue(endpoints, label, fib.BdName)
	if !found {
		utils.ExitWithError(utils.ExitInvalidInput, errors.New("Fib entry configured for a nonexisting bridge domain"))
	}
	// If FIB with the same MAC address exists, remove it first.
	found, key, _ := utils.GetFibEntry(endpoints, label, fib.BdName, fib.PhysAddress)
	if found {
		utils.DeleteFibDataFromDb(db, key)
	}
	// Create new FIB entry.
	fibToAdd := new(l2.FibTable_FibEntry)
	fibToAdd.PhysAddress = fib.PhysAddress
	fibToAdd.BridgeDomain = fib.BdName
	if fib.Action == 0 {
		fibToAdd.Action = l2.FibTable_FibEntry_FORWARD
	} else {
		fibToAdd.Action = l2.FibTable_FibEntry_DROP
	}
	fibToAdd.OutgoingInterface = fib.OutgoingInterface
	fibToAdd.StaticConfig = fib.StaticConfig
	fibToAdd.BridgedVirtualInterface = fib.BVI

	utils.WriteFibDataToDb(db, key, fibToAdd)
}

// DelFibEntry from the FIB table.
func DelFibEntry(endpoints []string, label string, fib *L2FIBEntryFields) {
	db, err := utils.GetDbForOneAgent(endpoints, label)
	if err != nil {
		utils.ExitWithError(utils.ExitBadConnection, err)
	}
	_, key, _ := utils.GetFibEntry(endpoints, label, fib.BdName, fib.PhysAddress)
	utils.DeleteFibDataFromDb(db, key)
}

// AddBridgeDomainNameFlag adds 'name' flag to the common fields.
func AddBridgeDomainNameFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&bdCommonFields.Name, "name", "n", "", "Bridge domain name")
}

// AddBridgeDomainFlags adds all bridge domain flags.
func AddBridgeDomainFlags(cmd *cobra.Command) {
	AddBridgeDomainNameFlag(cmd)
	cmd.Flags().BoolVarP(&bdCommonFields.Flood, "flood", "", false, "Enable/disable bcast/mcast flooding ")
	cmd.Flags().BoolVarP(&bdCommonFields.UUFlood, "uuflood", "", false, "Enable/disable uknown unicast flood ")
	cmd.Flags().BoolVarP(&bdCommonFields.Forward, "forward", "", false, "Enable/disable forwarding on all interfaces ")
	cmd.Flags().BoolVarP(&bdCommonFields.Learn, "learn", "", false, "Enable/disable learning on all interfaces ")
	cmd.Flags().BoolVarP(&bdCommonFields.Arp, "arp", "", false, "Enable/disable ARP termination ")
	cmd.Flags().Uint32VarP(&bdCommonFields.MacAge, "mac", "", 0, "MAC aging time in min, 0 for disabled aging ")

}
