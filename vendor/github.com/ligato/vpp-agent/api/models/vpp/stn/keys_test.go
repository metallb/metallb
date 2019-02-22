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

package vpp_stn_test

/*func TestSTNKey(t *testing.T) {
	tests := []struct {
		name         string
		stnInterface string
		stnIP        string
		expectedKey  string
	}{
		{
			name:         "valid STN case",
			stnInterface: "if1",
			stnIP:        "10.0.0.1",
			expectedKey:  "vpp/config/v2/stn/rule/if1/ip/10.0.0.1",
		},
		{
			name:         "invalid STN case (undefined interface)",
			stnInterface: "",
			stnIP:        "10.0.0.1",
			expectedKey:  "vpp/config/v2/stn/rule/<invalid>/ip/10.0.0.1",
		},
		{
			name:         "invalid STN case (undefined address)",
			stnInterface: "if1",
			stnIP:        "",
			expectedKey:  "vpp/config/v2/stn/rule/if1/ip/<invalid>",
		},
		{
			name:         "invalid STN case (IP address with mask provided)",
			stnInterface: "if1",
			stnIP:        "10.0.0.1/24",
			expectedKey:  "vpp/config/v2/stn/rule/if1/ip/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := stn.Key(test.stnInterface, test.stnIP)
			if key != test.expectedKey {
				t.Errorf("failed for: stnName=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.name, test.expectedKey, key)
			}
		})
	}
}*/

/*func TestParseSTNKey(t *testing.T) {
	tests := []struct {
		name             string
		key              string
		expectedIfName   string
		expectedIP       string
		expectedIsSTNKey bool
	}{
		{
			name:             "valid STN key",
			key:              "vpp/config/v2/stn/rule/if1/ip/10.0.0.1",
			expectedIfName:   "if1",
			expectedIP:       "10.0.0.1",
			expectedIsSTNKey: true,
		},
		{
			name:             "invalid if",
			key:              "vpp/config/v2/stn/rule/<invalid>/ip/10.0.0.1",
			expectedIfName:   "<invalid>",
			expectedIP:       "10.0.0.1",
			expectedIsSTNKey: true,
		},
		{
			name:             "invalid STN",
			key:              "vpp/config/v2/stn/rule/if1/ip/<invalid>",
			expectedIfName:   "if1",
			expectedIP:       "<invalid>",
			expectedIsSTNKey: true,
		},
		{
			name:             "invalid all",
			key:              "vpp/config/v2/stn/rule/<invalid>/ip/<invalid>",
			expectedIfName:   "<invalid>",
			expectedIP:       "<invalid>",
			expectedIsSTNKey: true,
		},
		{
			name:             "not STN key",
			key:              "vpp/config/v2/bd/bd1",
			expectedIfName:   "",
			expectedIP:       "",
			expectedIsSTNKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ifName, ip, isSTNKey := stn.ParseKey(test.key)
			if isSTNKey != test.expectedIsSTNKey {
				t.Errorf("expected isFIBKey: %v\tgot: %v", test.expectedIsSTNKey, isSTNKey)
			}
			if ifName != test.expectedIfName {
				t.Errorf("expected ifName: %s\tgot: %s", test.expectedIfName, ifName)
			}
			if ip != test.expectedIP {
				t.Errorf("expected IP: %s\tgot: %s", test.expectedIP, ip)
			}
		})
	}
}*/
