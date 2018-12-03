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

package testing

import (
	"strconv"

	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// TableData with 3x VPP, each with 3 interfaces. With such a data, all filtering options can be tested.
func TableData() utils.EtcdDump {
	// Non-zero statistics
	statistics := &interfaces.InterfacesState_Interface_Statistics{
		InPackets:     uint64(10),
		OutPackets:    uint64(20),
		InMissPackets: uint64(5),
	}

	ifStateWithMD := &utils.IfStateWithMD{
		InterfaceState: &interfaces.InterfacesState_Interface{
			AdminStatus:  1,
			OperStatus:   1,
			InternalName: "Test-Interface",
			Statistics:   statistics,
		},
	}

	interfaceState := utils.InterfaceWithMD{
		State: ifStateWithMD,
	}

	// Full-zero statistics
	zeroStatistics := &interfaces.InterfacesState_Interface_Statistics{}

	zeroIfStateWithMD := &utils.IfStateWithMD{
		InterfaceState: &interfaces.InterfacesState_Interface{
			AdminStatus:  2,
			OperStatus:   2,
			InternalName: "Test-Interface",
			Statistics:   zeroStatistics,
		},
	}

	zeroInterfaceState := utils.InterfaceWithMD{
		State: zeroIfStateWithMD,
	}

	// Prepare the test table with values and several full-zero columns and full-zero rows.
	etcdDump := make(map[string]*utils.VppData)
	for i := 1; i <= 3; i++ {
		vppName := "vpp-" + strconv.Itoa(i)

		interfaceStateMap := make(map[string]utils.InterfaceWithMD)
		for j := 1; j <= 3; j++ {
			interfaceName := vppName + "-interface-" + strconv.Itoa(j)
			if j == 2 {
				interfaceStateMap[interfaceName] = zeroInterfaceState
			} else {
				interfaceStateMap[interfaceName] = interfaceState
			}
		}
		vppData := utils.VppData{
			Interfaces: interfaceStateMap,
		}
		etcdDump[vppName] = &vppData
	}

	return etcdDump
}

// JSONData - every type of data to test all JSON possibilities
func JSONData() utils.EtcdDump {
	// Interface data
	interfaceData := utils.InterfaceWithMD{
		Config: &utils.IfConfigWithMD{
			Interface: &interfaces.Interfaces_Interface{
				Name: "iface",
			},
		},
		State: &utils.IfStateWithMD{
			InterfaceState: &interfaces.InterfacesState_Interface{
				Name:         "iface",
				AdminStatus:  1,
				OperStatus:   1,
				InternalName: "Test-Interface",
				Statistics: &interfaces.InterfacesState_Interface_Statistics{
					InPackets:     uint64(10),
					OutPackets:    uint64(20),
					InMissPackets: uint64(5),
				},
			},
		},
	}

	// Bridge domain data
	bdData := utils.BdWithMD{
		Config: &utils.BdConfigWithMD{
			Metadata: utils.VppMetaData{},
			BridgeDomain: &l2.BridgeDomains_BridgeDomain{
				Name: "bd",
			},
		},
		State: &utils.BdStateWithMD{
			Metadata: utils.VppMetaData{},
			BridgeDomainState: &l2.BridgeDomainState_BridgeDomain{
				Index: 1,
			},
		},
	}

	// L2 Fib data
	fibTableEntries := []*l2.FibTable_FibEntry{}
	fibTableEntry := &l2.FibTable_FibEntry{
		PhysAddress: "ff:ff:ff:ff:ff:ff",
	}
	fibTableEntries = append(fibTableEntries, fibTableEntry)

	fibData := utils.FibTableWithMD{
		FibTable: fibTableEntries,
	}

	// L3 Fib data
	l3FibTableEntries := []*l3.StaticRoutes_Route{}
	l3FibTableEntry := &l3.StaticRoutes_Route{
		VrfId:             1,
		DstIpAddr:         "192.168.2.0/24",
		NextHopAddr:       "192.168.1.1",
		OutgoingInterface: "eth0",
		Weight:            5,
		Preference:        0,
	}
	l3FibTableEntries = append(l3FibTableEntries, l3FibTableEntry)

	l3FibData := utils.StaticRoutesWithMD{
		Routes: l3FibTableEntries,
	}

	etcdDump := make(map[string]*utils.VppData)
	interfaceMap := make(map[string]utils.InterfaceWithMD)
	bridgeDomainMap := make(map[string]utils.BdWithMD)

	// Fill maps.
	interfaceMap["test-interface"] = interfaceData
	bridgeDomainMap["test-bd"] = bdData

	// Add the same data twice under different VPPs.
	etcdDump["vpp1"] = &utils.VppData{
		Interfaces:      interfaceMap,
		BridgeDomains:   bridgeDomainMap,
		FibTableEntries: fibData,
		StaticRoutes:    l3FibData,
	}

	etcdDump["vpp2"] = &utils.VppData{
		Interfaces:      interfaceMap,
		BridgeDomains:   bridgeDomainMap,
		FibTableEntries: fibData,
		StaticRoutes:    l3FibData,
	}

	return etcdDump
}
