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

package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	"github.com/logrusorgru/aurora.git"
)

const (
	// IfConfig labels used by json formatter
	IfConfig = "INTERFACE CONFIG"
	// IfState labels used by json formatter
	IfState = "INTERFACE STATE"
	// BdConfig labels used by json formatter
	BdConfig = "BRIDGE DOMAINS CONFIG"
	// BdState labels used by json formatter
	BdState = "BRIDGE DOMAINS State"
	// L2FibConfig labels used by json formatter
	L2FibConfig = "L2 FIB TABLE"
	// L3FibConfig labels used by json formatter
	L3FibConfig = "L3 FIB TABLE"
	// Format
	indent    = "  "
	emptyJSON = "{}"
)

// PrintDataAsJSON prints etcd data in JSON format.
func (ed EtcdDump) PrintDataAsJSON(filter []string) (*bytes.Buffer, error) {
	buffer := new(bytes.Buffer)
	keys := ed.getSortedKeys()
	var wasError error

	vpps, isData := processFilter(keys, filter)
	if !isData {
		fmt.Fprintf(buffer, "No data to display for VPPS: %s\n", vpps)
		return buffer, wasError
	}

	for _, key := range keys {
		if isNotInFilter(key, vpps) {
			continue
		}

		vd, _ := ed[key]
		// Obtain raw data.
		ifaceConfDataRoot, ifaceConfKeys := getInterfaceConfigData(vd.Interfaces)
		ifaceStateDataRoot, ifaceStateKeys := getInterfaceStateData(vd.Interfaces)
		l2ConfigDataRoot, l2Keys := getL2ConfigData(vd.BridgeDomains)
		l2StateDataRoot, l2Keys := getL2StateData(vd.BridgeDomains)
		l2FibDataRoot, l2FibKeys := getL2FIBData(vd.FibTableEntries)
		l3FibDataRoot, l3FibKeys := getL3FIBData(vd.StaticRoutes)

		// Interface config data
		jsConfData, err := json.MarshalIndent(ifaceConfDataRoot, "", indent)
		if err != nil {
			wasError = err
		}
		// Interface state data
		jsStateData, err := json.MarshalIndent(ifaceStateDataRoot, "", indent)
		if err != nil {
			wasError = err
		}
		// L2 config data
		jsL2ConfigData, err := json.MarshalIndent(l2ConfigDataRoot, "", indent)
		if err != nil {
			wasError = err
		}
		// L2 state data
		jsL2StateData, err := json.MarshalIndent(l2StateDataRoot, "", indent)
		if err != nil {
			wasError = err
		}
		// L2 FIB data
		jsL2FIBData, err := json.MarshalIndent(l2FibDataRoot, "", indent)
		if err != nil {
			wasError = err
		}
		// L3 FIB data
		jsL3FIBData, err := json.MarshalIndent(l3FibDataRoot, "", indent)
		if err != nil {
			wasError = err
		}

		// Add data to buffer.
		if string(jsConfData) != emptyJSON {
			printLabel(buffer, key+": - "+IfConfig+"\n", indent, ifaceConfKeys)
			fmt.Fprintf(buffer, "%s\n", jsConfData)
		}
		if string(jsStateData) != emptyJSON {
			printLabel(buffer, key+": - "+IfState+"\n", indent, ifaceStateKeys)
			fmt.Fprintf(buffer, "%s\n", jsStateData)
		}
		if string(jsL2ConfigData) != emptyJSON {
			printLabel(buffer, key+": - "+BdConfig+"\n", indent, l2Keys)
			fmt.Fprintf(buffer, "%s\n", jsL2ConfigData)
		}
		if string(jsL2ConfigData) != emptyJSON {
			printLabel(buffer, key+": - "+BdState+"\n", indent, l2Keys)
			fmt.Fprintf(buffer, "%s\n", jsL2StateData)
		}
		if string(jsL2FIBData) != emptyJSON {
			printLabel(buffer, key+": -"+L2FibConfig+"\n", indent, l2FibKeys)
			fmt.Fprintf(buffer, "%s\n", jsL2FIBData)
		}
		if string(jsL3FIBData) != emptyJSON {
			printLabel(buffer, key+": - "+L3FibConfig+"\n", indent, l3FibKeys)
			fmt.Fprintf(buffer, "%s\n", jsL3FIBData)
		}

	}

	return buffer, wasError
}

// 'processFilter' function returns a list of VPPs that satisfy the provided filter.
// If the filter is empty, all VPPs will be shown.
// If no data satisfy the filter, isData flag is returned as false.
func processFilter(keys []string, filter []string) ([]string, bool) {
	var vpps []string
	if len(filter) > 0 {
		// Ignore all parameters but first.
		vpps = strings.Split(filter[0], ",")
	} else {
		// Show all if there is no filter.
		vpps = keys
	}
	var isData bool
	// Find at least one match.
	for _, key := range keys {
		for _, vpp := range vpps {
			if key == vpp {
				isData = true
				break
			}
		}
	}
	return vpps, isData
}

// Returns true if provided key is present in filter, false otherwise.
func isNotInFilter(key string, filter []string) bool {
	for _, itemInFilter := range filter {
		if itemInFilter == key {
			return false
		}
	}
	return true
}

// Get interface config data and create full interface config proto structure.
func getInterfaceConfigData(interfaceData map[string]InterfaceWithMD) (*interfaces.Interfaces, []string) {
	// Config data
	ifaceRoot := interfaces.Interfaces{}
	ifaces := []*interfaces.Interfaces_Interface{}
	var keyset []string
	for _, ifaceData := range interfaceData {
		if ifaceData.Config != nil {
			iface := ifaceData.Config.Interface
			ifaces = append(ifaces, iface)
			keyset = append(keyset, ifaceData.Config.Metadata.Key)
		}
	}
	sort.Strings(keyset)
	ifaceRoot.Interfaces = ifaces

	return &ifaceRoot, keyset
}

// Get interface state data and create full interface state proto structure.
func getInterfaceStateData(interfaceData map[string]InterfaceWithMD) (*interfaces.InterfacesState, []string) {
	// Status data
	ifaceStateRoot := interfaces.InterfacesState{}
	ifaceStates := []*interfaces.InterfacesState_Interface{}
	var keyset []string
	for _, ifaceData := range interfaceData {
		if ifaceData.State != nil {
			ifaceState := ifaceData.State.InterfaceState
			ifaceStates = append(ifaceStates, ifaceState)
			keyset = append(keyset, ifaceData.State.Metadata.Key)
		}
	}
	sort.Strings(keyset)
	ifaceStateRoot.Interfaces = ifaceStates

	return &ifaceStateRoot, keyset
}

// Get l2 config data and create full l2 bridge domains proto structure.
func getL2ConfigData(l2Data map[string]BdWithMD) (*l2.BridgeDomains, []string) {
	l2Root := l2.BridgeDomains{}
	l2s := []*l2.BridgeDomains_BridgeDomain{}
	var keyset []string
	for _, bdData := range l2Data {
		if bdData.Config != nil {
			bd := bdData.Config.BridgeDomain
			l2s = append(l2s, bd)
			keyset = append(keyset, bdData.Config.Metadata.Key)
		}
	}
	sort.Strings(keyset)
	l2Root.BridgeDomains = l2s

	return &l2Root, keyset
}

// Get l2 state data and create full l2 bridge domains proto structure.
func getL2StateData(l2Data map[string]BdWithMD) (*l2.BridgeDomainState, []string) {
	l2StateRoot := l2.BridgeDomainState{}
	l2States := []*l2.BridgeDomainState_BridgeDomain{}
	var keyset []string
	for _, bdData := range l2Data {
		if bdData.Config != nil && bdData.State != nil {
			bd := bdData.State.BridgeDomainState
			l2States = append(l2States, bd)
			keyset = append(keyset, bdData.Config.Metadata.Key)
		}
	}
	sort.Strings(keyset)
	l2StateRoot.BridgeDomains = l2States

	return &l2StateRoot, keyset
}

// Get L2 FIB data and create full L2 FIB proto structure.
func getL2FIBData(fibData FibTableWithMD) (*l2.FibTable, []string) {
	fibRoot := l2.FibTable{}
	fibRoot.FibTableEntries = fibData.FibTable
	var keyset []string
	for _, fib := range fibData.FibTable {
		keyset = append(keyset, l2.FibKey(fib.BridgeDomain, fib.PhysAddress))
	}
	sort.Strings(keyset)

	return &fibRoot, keyset
}

// Get L3 FIB data and create full L3 FIB proto structure.
func getL3FIBData(fibData StaticRoutesWithMD) (*l3.StaticRoutes, []string) {
	fibRoot := l3.StaticRoutes{}
	fibRoot.Routes = fibData.Routes
	var keyset []string
	for _, fib := range fibData.Routes {
		keyset = append(keyset, l3.RouteKey(fib.VrfId, fib.DstIpAddr, fib.NextHopAddr))
	}
	sort.Strings(keyset)

	return &fibRoot, keyset
}

// Print label before printing every data structure, including used keys.
func printLabel(buffer *bytes.Buffer, label string, prefix string, keyset []string) {
	// Format output - find longest string in label to make label nicer
	labelLength := len(label)
	for _, key := range keyset {
		if len(key) > labelLength {
			labelLength = len(key)
		}
	}
	ub := prefix + strings.Repeat("-", labelLength) + "\n"

	// Print label.
	fmt.Fprintf(buffer, ub)
	fmt.Fprintf(buffer, "%s%s\n", prefix, aurora.Bold(label))
	fmt.Fprintf(buffer, "%s%s\n", prefix, "Keys:")
	for _, key := range keyset {
		if key != "" {
			fmt.Fprintf(buffer, "%s%s\n", prefix, key)
		}
	}
	fmt.Fprintf(buffer, ub)
}
