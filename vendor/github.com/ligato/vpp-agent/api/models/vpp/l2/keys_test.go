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

package vpp_l2

import (
	"testing"
)

/*func TestBridgeDomainKey(t *testing.T) {
	tests := []struct {
		name        string
		bdName      string
		expectedKey string
	}{
		{
			name:        "valid BD name",
			bdName:      "bd1",
			expectedKey: "vpp/config/v2/bd/bd1",
		},
		{
			name:        "invalid BD name",
			bdName:      "",
			expectedKey: "vpp/config/v2/bd/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := BridgeDomainKey(test.bdName)
			if key != test.expectedKey {
				t.Errorf("failed for: bdName=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.bdName, test.expectedKey, key)
			}
		})
	}
}

func TestParseBDNameFromKey(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		expectedBDName  string
		expectedIsBDKey bool
	}{
		{
			name:            "valid BD name",
			key:             "vpp/config/v2/bd/bd1",
			expectedBDName:  "bd1",
			expectedIsBDKey: true,
		},
		{
			name:            "invalid BD name",
			key:             "vpp/config/v2/bd/<invalid>",
			expectedBDName:  "<invalid>",
			expectedIsBDKey: true,
		},
		{
			name:            "not BD key",
			key:             "vpp/config/v2/bd/bd1/fib/aa:aa:aa:aa:aa:aa",
			expectedBDName:  "",
			expectedIsBDKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bdName, isBDKey := models.Model(&BridgeDomain{}).ParseKey(test.key)
			if isBDKey != test.expectedIsBDKey {
				t.Errorf("expected isBDKey: %v\tgot: %v", test.expectedIsBDKey, isBDKey)
			}
			if bdName != test.expectedBDName {
				t.Errorf("expected bdName: %s\tgot: %s", test.expectedBDName, bdName)
			}
		})
	}
}*/

func TestBDInterfaceKey(t *testing.T) {
	tests := []struct {
		name        string
		bdName      string
		iface       string
		expectedKey string
	}{
		{
			name:        "valid BD & iface names",
			bdName:      "bd1",
			iface:       "tap0",
			expectedKey: "vpp/bd/bd1/interface/tap0",
		},
		{
			name:        "invalid BD but valid interface",
			bdName:      "",
			iface:       "tap1",
			expectedKey: "vpp/bd/<invalid>/interface/tap1",
		},
		{
			name:        "invalid BD but valid interface",
			bdName:      "",
			iface:       "tap1",
			expectedKey: "vpp/bd/<invalid>/interface/tap1",
		},
		{
			name:        "valid BD but invalid interface",
			bdName:      "bd2",
			iface:       "",
			expectedKey: "vpp/bd/bd2/interface/<invalid>",
		},
		{
			name:        "invalid parameters",
			bdName:      "",
			iface:       "",
			expectedKey: "vpp/bd/<invalid>/interface/<invalid>",
		},
		{
			name:        "Gbe interface",
			bdName:      "bd5",
			iface:       "GigabitEthernet0/8/0",
			expectedKey: "vpp/bd/bd5/interface/GigabitEthernet0/8/0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := BDInterfaceKey(test.bdName, test.iface)
			if key != test.expectedKey {
				t.Errorf("failed for: bdName=%s iface=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.bdName, test.iface, test.expectedKey, key)
			}
		})
	}
}

func TestParseBDInterfaceKey(t *testing.T) {
	tests := []struct {
		name                 string
		key                  string
		expectedBDName       string
		expectedIface        string
		expectedIsBDIfaceKey bool
	}{
		{
			name:                 "valid BD & iface names",
			key:                  "vpp/bd/bd1/interface/tap0",
			expectedBDName:       "bd1",
			expectedIface:        "tap0",
			expectedIsBDIfaceKey: true,
		},
		{
			name:                 "invalid BD but valid interface",
			key:                  "vpp/bd/<invalid>/interface/tap1",
			expectedBDName:       "<invalid>",
			expectedIface:        "tap1",
			expectedIsBDIfaceKey: true,
		},
		{
			name:                 "valid BD but invalid interface",
			key:                  "vpp/bd/bd2/interface/<invalid>",
			expectedBDName:       "bd2",
			expectedIface:        "<invalid>",
			expectedIsBDIfaceKey: true,
		},
		{
			name:                 "Gbe interface",
			key:                  "vpp/bd/bd4/interface/GigabitEthernet0/8/0",
			expectedBDName:       "bd4",
			expectedIface:        "GigabitEthernet0/8/0",
			expectedIsBDIfaceKey: true,
		},
		{
			name:                 "not BD-interface key",
			key:                  "vpp/config/v2/bd/bd1",
			expectedBDName:       "",
			expectedIface:        "",
			expectedIsBDIfaceKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bdName, iface, isBDIfaceKey := ParseBDInterfaceKey(test.key)
			if isBDIfaceKey != test.expectedIsBDIfaceKey {
				t.Errorf("expected isBDIfaceKey: %v\tgot: %v", test.expectedIsBDIfaceKey, isBDIfaceKey)
			}
			if bdName != test.expectedBDName {
				t.Errorf("expected bdName: %s\tgot: %s", test.expectedBDName, bdName)
			}
			if iface != test.expectedIface {
				t.Errorf("expected iface: %s\tgot: %s", test.expectedIface, iface)
			}
		})
	}
}

/*func TestFIBKey(t *testing.T) {
	tests := []struct {
		name        string
		bdName      string
		fibMac      string
		expectedKey string
	}{
		{
			name:        "valid parameters",
			bdName:      "bd1",
			fibMac:      "12:34:56:78:9a:bc",
			expectedKey: "vpp/config/v2/bd/bd1/fib/12:34:56:78:9a:bc",
		},
		{
			name:        "invalid bd",
			bdName:      "",
			fibMac:      "aa:aa:aa:bb:bb:bb",
			expectedKey: "vpp/config/v2/bd/<invalid>/fib/aa:aa:aa:bb:bb:bb",
		},
		{
			name:        "invalid hw address",
			bdName:      "bd2",
			fibMac:      "in:va:li:d",
			expectedKey: "vpp/config/v2/bd/bd2/fib/<invalid>",
		},
		{
			name:        "invalid parameters",
			bdName:      "",
			fibMac:      "192.168.1.1",
			expectedKey: "vpp/config/v2/bd/<invalid>/fib/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := FIBKey(test.bdName, test.fibMac)
			if key != test.expectedKey {
				t.Errorf("failed for: bdName=%s fibMac=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.bdName, test.fibMac, test.expectedKey, key)
			}
		})
	}
}

func TestParseFIBKey(t *testing.T) {
	tests := []struct {
		name             string
		key              string
		expectedBDName   string
		expectedfibMac   string
		expectedIsFIBKey bool
	}{
		{
			name:             "valid FIB key",
			key:              "vpp/config/v2/bd/bd1/fib/12:34:56:78:9a:bc",
			expectedBDName:   "bd1",
			expectedfibMac:   "12:34:56:78:9a:bc",
			expectedIsFIBKey: true,
		},
		{
			name:             "invalid bd",
			key:              "vpp/config/v2/bd/<invalid>/fib/aa:bb:cc:dd:ee:ff",
			expectedBDName:   "<invalid>",
			expectedfibMac:   "aa:bb:cc:dd:ee:ff",
			expectedIsFIBKey: true,
		},
		{
			name:             "invalid fib",
			key:              "vpp/config/v2/bd/bd2/fib/<invalid>",
			expectedBDName:   "bd2",
			expectedfibMac:   "<invalid>",
			expectedIsFIBKey: true,
		},
		{
			name:             "invalid params",
			key:              "vpp/config/v2/bd/<invalid>/fib/<invalid>",
			expectedBDName:   "<invalid>",
			expectedfibMac:   "<invalid>",
			expectedIsFIBKey: true,
		},
		{
			name:             "not FIB key",
			key:              "vpp/bd/bd1/interface/tap0",
			expectedBDName:   "",
			expectedfibMac:   "",
			expectedIsFIBKey: false,
		},
		{
			name:             "not FIB key",
			key:              "vpp/config/v2/bd/bd1",
			expectedBDName:   "",
			expectedfibMac:   "",
			expectedIsFIBKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name, isFIBKey := models.Model(&FIBEntry{}).ParseKey(test.key)
			nameParts := strings.Split(name, "/")
			if len(nameParts) != 3 {
				t.Fatalf("invalid name: %q", name)
			}
			bdName, fibMac := nameParts[0], nameParts[2]
			if isFIBKey != test.expectedIsFIBKey {
				t.Errorf("expected isFIBKey: %v\tgot: %v", test.expectedIsFIBKey, isFIBKey)
			}
			if bdName != test.expectedBDName {
				t.Errorf("expected bdName: %s\tgot: %s", test.expectedBDName, bdName)
			}
			if fibMac != test.expectedfibMac {
				t.Errorf("expected iface: %s\tgot: %s", test.expectedfibMac, fibMac)
			}
		})
	}
}*/

/*func TestXConnectKey(t *testing.T) {
	tests := []struct {
		name        string
		rxIface     string
		expectedKey string
	}{
		{
			name:        "valid interface",
			rxIface:     "memif0",
			expectedKey: "vpp/config/v2/xconnect/memif0",
		},
		{
			name:        "invalid interface",
			rxIface:     "",
			expectedKey: "vpp/config/v2/xconnect/<invalid>",
		},
		{
			name:        "gbe",
			rxIface:     "GigabitEthernet0/8/0",
			expectedKey: "vpp/config/v2/xconnect/GigabitEthernet0/8/0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := XConnectKey(test.rxIface)
			if key != test.expectedKey {
				t.Errorf("failed for: rxIface=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.rxIface, test.expectedKey, key)
			}
		})
	}
}*/
