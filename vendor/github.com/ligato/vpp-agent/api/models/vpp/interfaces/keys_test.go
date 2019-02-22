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

package vpp_interfaces

import (
	"testing"
)

/*func TestInterfaceKey(t *testing.T) {
	tests := []struct {
		name        string
		iface       string
		expectedKey string
	}{
		{
			name:        "valid interface name",
			iface:       "memif0",
			expectedKey: "vpp/config/v2/interface/memif0",
		},
		{
			name:        "invalid interface name",
			iface:       "",
			expectedKey: "vpp/config/v2/interface/<invalid>",
		},
		{
			name:        "Gbe interface",
			iface:       "GigabitEthernet0/8/0",
			expectedKey: "vpp/config/v2/interface/GigabitEthernet0/8/0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := InterfaceKey(test.iface)
			if key != test.expectedKey {
				t.Errorf("failed for: iface=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.iface, test.expectedKey, key)
			}
		})
	}
}

func TestParseNameFromKey(t *testing.T) {
	tests := []struct {
		name               string
		key                string
		expectedIface      string
		expectedIsIfaceKey bool
	}{
		{
			name:               "valid interface name",
			key:                "vpp/config/v2/interface/memif0",
			expectedIface:      "memif0",
			expectedIsIfaceKey: true,
		},
		{
			name:               "invalid interface name",
			key:                "vpp/config/v2/interface/<invalid>",
			expectedIface:      "<invalid>",
			expectedIsIfaceKey: true,
		},
		{
			name:               "Gbe interface",
			key:                "vpp/config/v2/interface/GigabitEthernet0/8/0",
			expectedIface:      "GigabitEthernet0/8/0",
			expectedIsIfaceKey: true,
		},
		{
			name:               "not an interface key",
			key:                "vpp/config/v2/bd/bd1",
			expectedIface:      "",
			expectedIsIfaceKey: false,
		},
		{
			name:               "not an interface key (empty interface)",
			key:                "vpp/config/v2/interface/",
			expectedIface:      "",
			expectedIsIfaceKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			iface, isInterfaceKey := models.Model(&Interface{}).ParseKey(test.key)
			if isInterfaceKey != test.expectedIsIfaceKey {
				t.Errorf("expected isInterfaceKey: %v\tgot: %v", test.expectedIsIfaceKey, isInterfaceKey)
			}
			if iface != test.expectedIface {
				t.Errorf("expected iface: %s\tgot: %s", test.expectedIface, iface)
			}
		})
	}
}*/

func TestInterfaceErrorKey(t *testing.T) {
	tests := []struct {
		name        string
		iface       string
		expectedKey string
	}{
		{
			name:        "valid interface name",
			iface:       "memif0",
			expectedKey: "vpp/status/v2/interface/error/memif0",
		},
		{
			name:        "invalid interface name",
			iface:       "",
			expectedKey: "vpp/status/v2/interface/error/<invalid>",
		},
		{
			name:        "Gbe interface",
			iface:       "GigabitEthernet0/8/0",
			expectedKey: "vpp/status/v2/interface/error/GigabitEthernet0/8/0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := InterfaceErrorKey(test.iface)
			if key != test.expectedKey {
				t.Errorf("failed for: iface=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.iface, test.expectedKey, key)
			}
		})
	}
}

func TestInterfaceStateKey(t *testing.T) {
	tests := []struct {
		name        string
		iface       string
		expectedKey string
	}{
		{
			name:        "valid interface name",
			iface:       "memif0",
			expectedKey: "vpp/status/v2/interface/memif0",
		},
		{
			name:        "invalid interface name",
			iface:       "",
			expectedKey: "vpp/status/v2/interface/<invalid>",
		},
		{
			name:        "Gbe interface",
			iface:       "GigabitEthernet0/8/0",
			expectedKey: "vpp/status/v2/interface/GigabitEthernet0/8/0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := InterfaceStateKey(test.iface)
			if key != test.expectedKey {
				t.Errorf("failed for: iface=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.iface, test.expectedKey, key)
			}
		})
	}
}

func TestInterfaceAddressKey(t *testing.T) {
	tests := []struct {
		name        string
		iface       string
		address     string
		expectedKey string
	}{
		{
			name:        "IPv4 address",
			iface:       "memif0",
			address:     "192.168.1.12/24",
			expectedKey: "vpp/interface/address/memif0/192.168.1.12/24",
		},
		{
			name:        "IPv6 address",
			iface:       "memif0",
			address:     "2001:db8:0000:0000:0000:0000:0000:0000/32",
			expectedKey: "vpp/interface/address/memif0/2001:db8::/32",
		},
		{
			name:        "invalid interface",
			iface:       "",
			address:     "10.10.10.10/32",
			expectedKey: "vpp/interface/address/<invalid>/10.10.10.10/32",
		},
		{
			name:        "invalid address",
			iface:       "tap0",
			address:     "invalid-addr",
			expectedKey: "vpp/interface/address/tap0/<invalid>/<invalid>",
		},
		{
			name:        "missing mask",
			iface:       "tap1",
			address:     "10.10.10.10",
			expectedKey: "vpp/interface/address/tap1/<invalid>/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := InterfaceAddressKey(test.iface, test.address)
			if key != test.expectedKey {
				t.Errorf("failed for: iface=%s address=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.iface, test.address, test.expectedKey, key)
			}
		})
	}
}

func TestParseInterfaceAddressKey(t *testing.T) {
	tests := []struct {
		name                 string
		key                  string
		expectedIface        string
		expectedIfaceAddr    string
		expectedIfaceAddrNet string
		expectedIsAddrKey    bool
	}{
		{
			name:                 "IPv4 address",
			key:                  "vpp/interface/address/memif0/192.168.1.12/24",
			expectedIface:        "memif0",
			expectedIfaceAddr:    "192.168.1.12",
			expectedIfaceAddrNet: "192.168.1.0/24",
			expectedIsAddrKey:    true,
		},
		{
			name:                 "IPv6 address",
			key:                  "vpp/interface/address/tap1/2001:db8:85a3::8a2e:370:7334/48",
			expectedIface:        "tap1",
			expectedIfaceAddr:    "2001:db8:85a3::8a2e:370:7334",
			expectedIfaceAddrNet: "2001:db8:85a3::/48",
			expectedIsAddrKey:    true,
		},
		{
			name:                 "invalid interface",
			key:                  "vpp/interface/address/<invalid>/10.10.10.10/30",
			expectedIface:        "<invalid>",
			expectedIfaceAddr:    "10.10.10.10",
			expectedIfaceAddrNet: "10.10.10.8/30",
			expectedIsAddrKey:    true,
		},
		{
			name:                 "gbe interface",
			key:                  "vpp/interface/address/GigabitEthernet0/8/0/192.168.5.5/16",
			expectedIface:        "GigabitEthernet0/8/0",
			expectedIfaceAddr:    "192.168.5.5",
			expectedIfaceAddrNet: "192.168.0.0/16",
			expectedIsAddrKey:    true,
		},
		{
			name:                 "not valid key (missing interface)",
			key:                  "vpp/interface/address//192.168.5.5/16",
			expectedIface:        "",
			expectedIfaceAddr:    "",
			expectedIfaceAddrNet: "",
			expectedIsAddrKey:    false,
		},
		{
			name:                 "not valid key (missing mask)",
			key:                  "vpp/interface/address/tap3/192.168.5.5",
			expectedIface:        "",
			expectedIfaceAddr:    "",
			expectedIfaceAddrNet: "",
			expectedIsAddrKey:    false,
		},
		{
			name:                 "not valid key (missing address and mask)",
			key:                  "vpp/interface/address/tap3",
			expectedIface:        "",
			expectedIfaceAddr:    "",
			expectedIfaceAddrNet: "",
			expectedIsAddrKey:    false,
		},
		{
			name:                 "not interface address key",
			key:                  "vpp/config/v2/interface/GigabitEthernet0/8/0",
			expectedIface:        "",
			expectedIfaceAddr:    "",
			expectedIfaceAddrNet: "",
			expectedIsAddrKey:    false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			iface, ipAddr, ipAddrNet, isAddrKey := ParseInterfaceAddressKey(test.key)
			var ipAddrStr, ipAddrNetStr string
			if ipAddr != nil {
				ipAddrStr = ipAddr.String()
			}
			if ipAddrNet != nil {
				ipAddrNetStr = ipAddrNet.String()
			}
			if isAddrKey != test.expectedIsAddrKey {
				t.Errorf("expected isAddrKey: %v\tgot: %v", test.expectedIsAddrKey, isAddrKey)
			}
			if iface != test.expectedIface {
				t.Errorf("expected iface: %s\tgot: %s", test.expectedIface, iface)
			}
			if ipAddrStr != test.expectedIfaceAddr {
				t.Errorf("expected ipAddr: %s\tgot: %s", test.expectedIface, ipAddrStr)
			}
			if ipAddrNetStr != test.expectedIfaceAddrNet {
				t.Errorf("expected ipAddrNet: %s\tgot: %s", test.expectedIfaceAddrNet, ipAddrNetStr)
			}
		})
	}
}

func TestUnnumberedKey(t *testing.T) {
	tests := []struct {
		name        string
		iface       string
		expectedKey string
	}{
		{
			name:        "valid interface name",
			iface:       "memif0",
			expectedKey: "vpp/interface/unnumbered/memif0",
		},
		{
			name:        "invalid interface name",
			iface:       "",
			expectedKey: "vpp/interface/unnumbered/<invalid>",
		},
		{
			name:        "Gbe interface",
			iface:       "GigabitEthernet0/8/0",
			expectedKey: "vpp/interface/unnumbered/GigabitEthernet0/8/0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := UnnumberedKey(test.iface)
			if key != test.expectedKey {
				t.Errorf("failed for: iface=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.iface, test.expectedKey, key)
			}
		})
	}
}

func TestDHCPClientKey(t *testing.T) {
	tests := []struct {
		name        string
		iface       string
		expectedKey string
	}{
		{
			name:        "valid interface name",
			iface:       "memif0",
			expectedKey: "vpp/interface/dhcp-client/memif0",
		},
		{
			name:        "invalid interface name",
			iface:       "",
			expectedKey: "vpp/interface/dhcp-client/<invalid>",
		},
		{
			name:        "Gbe interface",
			iface:       "GigabitEthernet0/8/0",
			expectedKey: "vpp/interface/dhcp-client/GigabitEthernet0/8/0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := DHCPClientKey(test.iface)
			if key != test.expectedKey {
				t.Errorf("failed for: iface=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.iface, test.expectedKey, key)
			}
		})
	}
}

func TestParseNameFromDHCPClientKey(t *testing.T) {
	tests := []struct {
		name                    string
		key                     string
		expectedIface           string
		expectedIsDHCPClientKey bool
	}{
		{
			name:                    "valid interface name",
			key:                     "vpp/interface/dhcp-client/memif0",
			expectedIface:           "memif0",
			expectedIsDHCPClientKey: true,
		},
		{
			name:                    "invalid interface name",
			key:                     "vpp/interface/dhcp-client/<invalid>",
			expectedIface:           "<invalid>",
			expectedIsDHCPClientKey: true,
		},
		{
			name:                    "Gbe interface",
			key:                     "vpp/interface/dhcp-client/GigabitEthernet0/8/0",
			expectedIface:           "GigabitEthernet0/8/0",
			expectedIsDHCPClientKey: true,
		},
		{
			name:                    "not DHCP client key",
			key:                     "vpp/config/v2/bd/bd1",
			expectedIface:           "",
			expectedIsDHCPClientKey: false,
		},
		{
			name:                    "not DHCP client key (empty interface)",
			key:                     "vpp/interface/dhcp-client/",
			expectedIface:           "",
			expectedIsDHCPClientKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			iface, isDHCPClientKey := ParseNameFromDHCPClientKey(test.key)
			if isDHCPClientKey != test.expectedIsDHCPClientKey {
				t.Errorf("expected isInterfaceKey: %v\tgot: %v", test.expectedIsDHCPClientKey, isDHCPClientKey)
			}
			if iface != test.expectedIface {
				t.Errorf("expected iface: %s\tgot: %s", test.expectedIface, iface)
			}
		})
	}
}

func TestDHCPLeaseKey(t *testing.T) {
	tests := []struct {
		name        string
		iface       string
		expectedKey string
	}{
		{
			name:        "valid interface name",
			iface:       "memif0",
			expectedKey: "vpp/interface/dhcp-lease/memif0",
		},
		{
			name:        "invalid interface name",
			iface:       "",
			expectedKey: "vpp/interface/dhcp-lease/<invalid>",
		},
		{
			name:        "Gbe interface",
			iface:       "GigabitEthernet0/8/0",
			expectedKey: "vpp/interface/dhcp-lease/GigabitEthernet0/8/0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := DHCPLeaseKey(test.iface)
			if key != test.expectedKey {
				t.Errorf("failed for: iface=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.iface, test.expectedKey, key)
			}
		})
	}
}
