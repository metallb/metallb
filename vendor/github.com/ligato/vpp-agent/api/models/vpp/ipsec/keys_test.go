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

package vpp_ipsec_test

import (
	"testing"

	ipsec "github.com/ligato/vpp-agent/api/models/vpp/ipsec"
)

/*func TestIPSecSPDKey(t *testing.T) {
	tests := []struct {
		name        string
		spdIndex    string
		expectedKey string
	}{
		{
			name:        "valid SPD index",
			spdIndex:    "1",
			expectedKey: "vpp/config/v2/ipsec/spd/1",
		},
		{
			name:        "empty SPD index",
			spdIndex:    "",
			expectedKey: "vpp/config/v2/ipsec/spd/<invalid>",
		},
		{
			name:        "invalid SPD index",
			spdIndex:    "spd1",
			expectedKey: "vpp/config/v2/ipsec/spd/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := ipsec.SPDKey(test.spdIndex)
			if key != test.expectedKey {
				t.Errorf("failed for: spdName=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.name, test.expectedKey, key)
			}
		})
	}
}

func TestParseIPSecSPDNameFromKey(t *testing.T) {
	tests := []struct {
		name             string
		key              string
		expectedSPDIndex string
		expectedIsSPDKey bool
	}{
		{
			name:             "valid SPD index",
			key:              "vpp/config/v2/ipsec/spd/1",
			expectedSPDIndex: "1",
			expectedIsSPDKey: true,
		},
		{
			name:             "empty SPD index",
			key:              "vpp/config/v2/ipsec/spd/<invalid>",
			expectedSPDIndex: "<invalid>",
			expectedIsSPDKey: true,
		},
		{
			name:             "invalid SPD index",
			key:              "vpp/config/v2/ipsec/spd/spd1",
			expectedSPDIndex: "",
			expectedIsSPDKey: true,
		},
		{
			name:             "not SPD key",
			key:              "vpp/config/v2/ipsec/sa/spd1",
			expectedSPDIndex: "",
			expectedIsSPDKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spdName, isSPDKey := models.Model(&ipsec.SecurityPolicyDatabase{}).ParseKey(test.key)
			if isSPDKey != test.expectedIsSPDKey {
				t.Errorf("expected isSPDKey: %v\tgot: %v", test.expectedIsSPDKey, isSPDKey)
			}
			if spdName != test.expectedSPDIndex {
				t.Errorf("expected spdName: %s\tgot: %s", test.expectedSPDIndex, spdName)
			}
		})
	}
}*/

func TestSPDInterfaceKey(t *testing.T) {
	tests := []struct {
		name        string
		spdIndex    string
		ifName      string
		expectedKey string
	}{
		{
			name:        "valid SPD index & iface name",
			spdIndex:    "1",
			ifName:      "if1",
			expectedKey: "vpp/spd/1/interface/if1",
		},
		{
			name:        "empty SPD & valid interface",
			spdIndex:    "",
			ifName:      "if1",
			expectedKey: "vpp/spd/<invalid>/interface/if1",
		},
		{
			name:        "invalid SPD but valid interface",
			spdIndex:    "spd1",
			ifName:      "if1",
			expectedKey: "vpp/spd/<invalid>/interface/if1",
		},
		{
			name:        "valid SPD but invalid interface",
			spdIndex:    "1",
			ifName:      "",
			expectedKey: "vpp/spd/1/interface/<invalid>",
		},
		{
			name:        "invalid parameters",
			spdIndex:    "",
			ifName:      "",
			expectedKey: "vpp/spd/<invalid>/interface/<invalid>",
		},
		{
			name:        "Gbe interface",
			spdIndex:    "1",
			ifName:      "GigabitEthernet0/a/0",
			expectedKey: "vpp/spd/1/interface/GigabitEthernet0/a/0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := ipsec.SPDInterfaceKey(test.spdIndex, test.ifName)
			if key != test.expectedKey {
				t.Errorf("failed for: spdIdx=%s idName=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.spdIndex, test.ifName, test.expectedKey, key)
			}
		})
	}
}

func TestParseSPDInterfaceKey(t *testing.T) {
	tests := []struct {
		name                 string
		key                  string
		expectedSPDIndex     string
		expectedIfName       string
		expectedIsSAIfaceKey bool
	}{
		{
			name:                 "valid SPD & iface name",
			key:                  "vpp/spd/1/interface/if1",
			expectedSPDIndex:     "1",
			expectedIfName:       "if1",
			expectedIsSAIfaceKey: true,
		},
		{
			name:                 "invalid SPD but valid interface",
			key:                  "vpp/spd/<invalid>/interface/if1",
			expectedSPDIndex:     "<invalid>",
			expectedIfName:       "if1",
			expectedIsSAIfaceKey: true,
		},
		{
			name:                 "valid SPD but invalid interface",
			key:                  "vpp/spd/1/interface/<invalid>",
			expectedSPDIndex:     "1",
			expectedIfName:       "<invalid>",
			expectedIsSAIfaceKey: true,
		},
		{
			name:                 "Gbe interface",
			key:                  "vpp/spd/1/interface/GigabitEthernet0/8/0",
			expectedSPDIndex:     "1",
			expectedIfName:       "GigabitEthernet0/8/0",
			expectedIsSAIfaceKey: true,
		},
		{
			name:                 "not SPD-interface key",
			key:                  "vpp/config/v2/ipsec/spd/1",
			expectedSPDIndex:     "",
			expectedIfName:       "",
			expectedIsSAIfaceKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spdIdx, ifName, isSPDIfaceKey := ipsec.ParseSPDInterfaceKey(test.key)
			if isSPDIfaceKey != test.expectedIsSAIfaceKey {
				t.Errorf("expected isSPDIfaceKey: %v\tgot: %v", test.expectedIsSAIfaceKey, isSPDIfaceKey)
			}
			if spdIdx != test.expectedSPDIndex {
				t.Errorf("expected spdIdx: %s\tgot: %s", test.expectedSPDIndex, spdIdx)
			}
			if ifName != test.expectedIfName {
				t.Errorf("expected ifName: %s\tgot: %s", test.expectedIfName, ifName)
			}
		})
	}
}

func TestSPDPolicyKey(t *testing.T) {
	tests := []struct {
		name        string
		spdIndex    string
		saIndex     string
		expectedKey string
	}{
		{
			name:        "valid SPD & SA index",
			spdIndex:    "1",
			saIndex:     "2",
			expectedKey: "vpp/spd/1/sa/2",
		},
		{
			name:        "empty SPD & valid SA",
			spdIndex:    "",
			saIndex:     "2",
			expectedKey: "vpp/spd/<invalid>/sa/2",
		},
		{
			name:        "invalid SPD and empty SA",
			spdIndex:    "spd1",
			saIndex:     "",
			expectedKey: "vpp/spd/<invalid>/sa/<invalid>",
		},
		{
			name:        "valid SPD but invalid SA",
			spdIndex:    "1",
			saIndex:     "sa2",
			expectedKey: "vpp/spd/1/sa/<invalid>",
		},
		{
			name:        "invalid parameters",
			spdIndex:    "",
			saIndex:     "",
			expectedKey: "vpp/spd/<invalid>/sa/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := ipsec.SPDPolicyKey(test.spdIndex, test.saIndex)
			if key != test.expectedKey {
				t.Errorf("failed for: spdIdx=%s saIdx=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.spdIndex, test.saIndex, test.expectedKey, key)
			}
		})
	}
}

func TestParseSPDPolicyKey(t *testing.T) {
	tests := []struct {
		name                   string
		key                    string
		expectedSPDIndex       string
		expectedSAIndex        string
		expectedIsSPDPolicyKey bool
	}{
		{
			name:                   "valid SPD & SA index",
			key:                    "vpp/spd/1/interface/2",
			expectedSPDIndex:       "1",
			expectedSAIndex:        "2",
			expectedIsSPDPolicyKey: true,
		},
		{
			name:                   "invalid SPD but valid SA",
			key:                    "vpp/spd/<invalid>/interface/2",
			expectedSPDIndex:       "<invalid>",
			expectedSAIndex:        "2",
			expectedIsSPDPolicyKey: true,
		},
		{
			name:                   "valid SPD but invalid SA",
			key:                    "vpp/spd/1/interface/<invalid>",
			expectedSPDIndex:       "1",
			expectedSAIndex:        "<invalid>",
			expectedIsSPDPolicyKey: true,
		},
		{
			name:                   "not SPD-policy key",
			key:                    "vpp/config/v2/ipsec/sa/1",
			expectedSPDIndex:       "",
			expectedSAIndex:        "",
			expectedIsSPDPolicyKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spdIdx, saIdx, isSPDPolicyKey := ipsec.ParseSPDInterfaceKey(test.key)
			if isSPDPolicyKey != test.expectedIsSPDPolicyKey {
				t.Errorf("expected isSPDIfaceKey: %v\tgot: %v", test.expectedIsSPDPolicyKey, isSPDPolicyKey)
			}
			if spdIdx != test.expectedSPDIndex {
				t.Errorf("expected spdIdx: %s\tgot: %s", test.expectedSPDIndex, spdIdx)
			}
			if saIdx != test.expectedSAIndex {
				t.Errorf("expected saIdx: %s\tgot: %s", test.expectedSAIndex, saIdx)
			}
		})
	}
}

/*func TestIPSecSAKey(t *testing.T) {
	tests := []struct {
		name        string
		saIndex     string
		expectedKey string
	}{
		{
			name:        "valid SA index",
			saIndex:     "1",
			expectedKey: "vpp/config/v2/ipsec/sa/1",
		},
		{
			name:        "empty SA index",
			saIndex:     "",
			expectedKey: "vpp/config/v2/ipsec/sa/<invalid>",
		},
		{
			name:        "invalid SA index",
			saIndex:     "sa1",
			expectedKey: "vpp/config/v2/ipsec/sa/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := ipsec.SAKey(test.saIndex)
			if key != test.expectedKey {
				t.Errorf("failed for: saName=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.name, test.expectedKey, key)
			}
		})
	}
}

func TestParseIPSecSANameFromKey(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		expectedSAIndex string
		expectedIsSAKey bool
	}{
		{
			name:            "valid SA index",
			key:             "vpp/config/v2/ipsec/sa/1",
			expectedSAIndex: "1",
			expectedIsSAKey: true,
		},
		{
			name:            "empty SA index",
			key:             "vpp/config/v2/ipsec/sa/<invalid>",
			expectedSAIndex: "<invalid>",
			expectedIsSAKey: true,
		},
		{
			name:            "invalid SPD index",
			key:             "vpp/config/v2/ipsec/sa/sa1",
			expectedSAIndex: "",
			expectedIsSAKey: true,
		},
		{
			name:            "not SA key",
			key:             "vpp/config/v2/ipsec/tunnel/sa1",
			expectedSAIndex: "",
			expectedIsSAKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			saName, isSAKey := models.Model(&ipsec.SecurityAssociation{}).ParseKey(test.key)
			if isSAKey != test.expectedIsSAKey {
				t.Errorf("expected isSAKey: %v\tgot: %v", test.expectedIsSAKey, isSAKey)
			}
			if saName != test.expectedSAIndex {
				t.Errorf("expected saName: %s\tgot: %s", test.expectedSAIndex, saName)
			}
		})
	}
}*/
