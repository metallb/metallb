//  Copyright (c) 2019 Cisco and/or its affiliates.
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

package models_test

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/vpp-agent/api/models/linux/interfaces"
	"github.com/ligato/vpp-agent/pkg/models"
)

func TestEncoding(t *testing.T) {
	in := &linux_interfaces.Interface{
		Name: "testName",
		Type: linux_interfaces.Interface_VETH,
	}

	item, err := models.MarshalItem(in)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	t.Logf("marshalled:\n%+v", proto.MarshalTextString(item))

	out, err := models.UnmarshalItem(item)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	t.Logf("unmarshalled:\n%+v", proto.MarshalTextString(out))
}

/*func TestKeys(t *testing.T) {
	tests := []struct {
		name        string
		model       proto.Message
		expectedKey string
	}{
		{
			name: "linux iface",
			model: &linux_interfaces.Interface{
				Name: "testName",
				Type: linux_interfaces.Interface_VETH,
			},
			expectedKey: "linux/config/v2/interface/testName",
		},
		{
			name: "linux route",
			model: &linux_l3.Route{
				DstNetwork:        "1.1.1.1/24",
				OutgoingInterface: "eth0",
				GwAddr:            "9.9.9.9",
			},
			expectedKey: "linux/config/v2/route/1.1.1.0/24/eth0",
		},
		{
			name: "linux arp",
			model: &linux_l3.ARPEntry{
				Interface: "if1",
				IpAddress: "1.2.3.4",
				HwAddress: "11:22:33:44:55:66",
			},
			expectedKey: "linux/config/v2/arp/if1/1.2.3.4",
		},
		{
			name: "vpp acl",
			model: &vpp_acl.Acl{
				Name: "myacl5",
			},
			expectedKey: "vpp/config/v2/acl/myacl5",
		},
		{
			name: "vpp bd",
			model: &vpp_l2.BridgeDomain{
				Name: "bd3",
			},
			expectedKey: "vpp/config/v2/bd/bd3",
		},
		{
			name: "vpp nat global",
			model: &vpp_nat.Nat44Global{
				Forwarding: true,
			},
			expectedKey: "vpp/config/v2/nat44/GLOBAL",
		},
		{
			name: "vpp dnat",
			model: &vpp_nat.DNat44{
				Label: "mynat1",
			},
			expectedKey: "vpp/config/v2/nat44/dnat/mynat1",
		},
		{
			name: "vpp arp",
			model: &vpp_l3.ARPEntry{
				Interface:   "if1",
				IpAddress:   "1.2.3.4",
				PhysAddress: "11:22:33:44:55:66",
			},
			expectedKey: "vpp/config/v2/arp/if1/1.2.3.4",
		},
		{
			name: "vpp route",
			model: &vpp_l3.Route{
				VrfId:       0,
				DstNetwork:  "10.10.0.10/24",
				NextHopAddr: "0.0.0.0",
			},
			expectedKey: "vpp/config/v2/route/vrf/0/dst/10.10.0.0/24/gw/0.0.0.0",
		},
		{
			name: "vpp stn",
			model: &vpp_stn.Rule{
				Interface: "eth0",
				IpAddress: "1.1.1.1",
			},
			expectedKey: "vpp/config/v2/stn/rule/eth0/ip/1.1.1.1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := models.Key(test.model)

			if key != test.expectedKey {
				t.Errorf("expected key: \n%q\ngot: \n%q", test.expectedKey, key)
			} else {
				spec := models.Model(test.model)
				t.Logf("key: %q (%v)\n", key, spec)
			}
		})
	}
}*/

/*func TestParseKeys(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		expectedParts map[string]string
	}{
		{
			name: "vpp arp",
			key:  "vpp/config/v2/arp/if1/1.2.3.4",
			expectedParts: map[string]string{
				"Interface": "if1",
				"IpAddress": "1.2.3.4",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			parts := models.ParseKey(test.key)
			t.Logf("parts: %q", parts)

			if len(parts) != len(test.expectedParts) {
				t.Errorf("expected parts: %v, got: %v", test.expectedParts, parts)
			}
		})
	}
}*/
