//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package vpp_nat

import (
	"testing"
)

/*func TestDNAT44Key(t *testing.T) {
	tests := []struct {
		name        string
		label       string
		expectedKey string
	}{
		{
			name:        "valid DNAT44 label",
			label:       "dnat1",
			expectedKey: "vpp/config/v2/nat44/dnat/dnat1",
		},
		{
			name:        "invalid DNAT44 label",
			label:       "",
			expectedKey: "vpp/config/v2/nat44/dnat/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := DNAT44Key(test.label)
			if key != test.expectedKey {
				t.Errorf("failed for: label=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.label, test.expectedKey, key)
			}
		})
	}
}*/

func TestInterfaceNAT44Key(t *testing.T) {
	tests := []struct {
		name        string
		iface       string
		isInside    bool
		expectedKey string
	}{
		{
			name:        "interface-with-IN-feature",
			iface:       "tap0",
			isInside:    true,
			expectedKey: "vpp/nat44/interface/tap0/feature/in",
		},
		{
			name:        "interface-with-OUT-feature",
			iface:       "tap1",
			isInside:    false,
			expectedKey: "vpp/nat44/interface/tap1/feature/out",
		},
		{
			name:        "gbe-interface-OUT",
			iface:       "GigabitEthernet0/8/0",
			isInside:    false,
			expectedKey: "vpp/nat44/interface/GigabitEthernet0/8/0/feature/out",
		},
		{
			name:        "gbe-interface-IN",
			iface:       "GigabitEthernet0/8/0",
			isInside:    true,
			expectedKey: "vpp/nat44/interface/GigabitEthernet0/8/0/feature/in",
		},
		{
			name:        "invalid-interface-with-IN-feature",
			iface:       "",
			isInside:    true,
			expectedKey: "vpp/nat44/interface/<invalid>/feature/in",
		},
		{
			name:        "invalid-interface-with-OUT-feature",
			iface:       "",
			isInside:    false,
			expectedKey: "vpp/nat44/interface/<invalid>/feature/out",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := InterfaceNAT44Key(test.iface, test.isInside)
			if key != test.expectedKey {
				t.Errorf("failed for: iface=%s isInside=%t\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.iface, test.isInside, test.expectedKey, key)
			}
		})
	}
}

func TestParseInterfaceNAT44Key(t *testing.T) {
	tests := []struct {
		name                        string
		key                         string
		expectedIface               string
		expectedIsInside            bool
		expectedIsInterfaceNAT44Key bool
	}{
		{
			name:                        "interface-with-IN-feature",
			key:                         "vpp/nat44/interface/tap0/feature/in",
			expectedIface:               "tap0",
			expectedIsInside:            true,
			expectedIsInterfaceNAT44Key: true,
		},
		{
			name:                        "interface-with-OUT-feature",
			key:                         "vpp/nat44/interface/tap1/feature/out",
			expectedIface:               "tap1",
			expectedIsInside:            false,
			expectedIsInterfaceNAT44Key: true,
		},
		{
			name:                        "gbe-interface-OUT",
			key:                         "vpp/nat44/interface/GigabitEthernet0/8/0/feature/out",
			expectedIface:               "GigabitEthernet0/8/0",
			expectedIsInside:            false,
			expectedIsInterfaceNAT44Key: true,
		},
		{
			name:                        "gbe-interface-IN",
			key:                         "vpp/nat44/interface/GigabitEthernet0/8/0/feature/in",
			expectedIface:               "GigabitEthernet0/8/0",
			expectedIsInside:            true,
			expectedIsInterfaceNAT44Key: true,
		},
		{
			name:                        "invalid-interface",
			key:                         "vpp/nat44/interface/<invalid>/feature/in",
			expectedIface:               "<invalid>",
			expectedIsInside:            true,
			expectedIsInterfaceNAT44Key: true,
		},
		{
			name:                        "not interface key 1",
			key:                         "vpp/nat44/address/192.168.1.1",
			expectedIface:               "",
			expectedIsInside:            false,
			expectedIsInterfaceNAT44Key: false,
		},
		{
			name:                        "not interface key 2",
			key:                         "vpp/config/v2/nat44/dnat/dnat1",
			expectedIface:               "",
			expectedIsInside:            false,
			expectedIsInterfaceNAT44Key: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			iface, isInside, isInterfaceNAT44Key := ParseInterfaceNAT44Key(test.key)
			if isInterfaceNAT44Key != test.expectedIsInterfaceNAT44Key {
				t.Errorf("expected isInterfaceNAT44Key: %v\tgot: %v", test.expectedIsInterfaceNAT44Key, isInterfaceNAT44Key)
			}
			if iface != test.expectedIface {
				t.Errorf("expected iface: %s\tgot: %s", test.expectedIface, iface)
			}
			if isInside != test.expectedIsInside {
				t.Errorf("expected isInside: %t\tgot: %t", test.expectedIsInside, isInside)
			}
		})
	}
}
