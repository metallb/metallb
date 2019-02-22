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

package vpp_acl

import (
	"testing"
)

/*func TestACLKey(t *testing.T) {
	tests := []struct {
		name        string
		aclName     string
		expectedKey string
	}{
		{
			name:        "valid ACL name",
			aclName:     "acl1",
			expectedKey: "vpp/config/v2/acl/acl1",
		},
		{
			name:        "invalid ACL name",
			aclName:     "",
			expectedKey: "vpp/config/v2/acl/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := Key(test.aclName)
			if key != test.expectedKey {
				t.Errorf("failed for: aclName=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.aclName, test.expectedKey, key)
			}
		})
	}
}

func TestParseNameFromKey(t *testing.T) {
	tests := []struct {
		name             string
		key              string
		expectedACLName  string
		expectedIsACLKey bool
	}{
		{
			name:             "valid ACL name",
			key:              "vpp/config/v2/acl/acl1",
			expectedACLName:  "acl1",
			expectedIsACLKey: true,
		},
		{
			name:             "invalid ACL name",
			key:              "vpp/config/v2/acl/<invalid>",
			expectedACLName:  "<invalid>",
			expectedIsACLKey: true,
		},
		{
			name:             "not an ACL key",
			key:              "vpp/config/v2/bd/bd1",
			expectedACLName:  "",
			expectedIsACLKey: false,
		},
		{
			name:             "not an ACL key (empty name)",
			key:              "vpp/config/v2/acl/",
			expectedACLName:  "",
			expectedIsACLKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			aclName, isACLKey := models.Model(&Acl{}).ParseKey(test.key)
			if isACLKey != test.expectedIsACLKey {
				t.Errorf("expected isACLKey: %v\tgot: %v", test.expectedIsACLKey, isACLKey)
			}
			if aclName != test.expectedACLName {
				t.Errorf("expected aclName: %s\tgot: %s", test.expectedACLName, aclName)
			}
		})
	}
}*/

func TestACLToInterfaceKey(t *testing.T) {
	tests := []struct {
		name        string
		aclName     string
		iface       string
		flow        string
		expectedKey string
	}{
		{
			name:        "ingress interface",
			aclName:     "acl1",
			iface:       "tap0",
			flow:        "ingress",
			expectedKey: "vpp/acl/acl1/interface/ingress/tap0",
		},
		{
			name:        "egress interface",
			aclName:     "acl2",
			iface:       "memif0",
			flow:        "egress",
			expectedKey: "vpp/acl/acl2/interface/egress/memif0",
		},
		{
			name:        "Gbe interface",
			aclName:     "acl1",
			iface:       "GigabitEthernet0/8/0",
			flow:        "ingress",
			expectedKey: "vpp/acl/acl1/interface/ingress/GigabitEthernet0/8/0",
		},
		{
			name:        "empty acl name",
			aclName:     "",
			iface:       "memif0",
			flow:        "egress",
			expectedKey: "vpp/acl/<invalid>/interface/egress/memif0",
		},
		{
			name:        "invalid flow",
			aclName:     "acl2",
			iface:       "memif0",
			flow:        "invalid-value",
			expectedKey: "vpp/acl/acl2/interface/<invalid>/memif0",
		},
		{
			name:        "empty interface",
			aclName:     "acl2",
			iface:       "",
			flow:        "egress",
			expectedKey: "vpp/acl/acl2/interface/egress/<invalid>",
		},
		{
			name:        "empty parameters",
			aclName:     "",
			iface:       "",
			flow:        "",
			expectedKey: "vpp/acl/<invalid>/interface/<invalid>/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := ToInterfaceKey(test.aclName, test.iface, test.flow)
			if key != test.expectedKey {
				t.Errorf("failed for: aclName=%s iface=%s flow=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.aclName, test.iface, test.flow, test.expectedKey, key)
			}
		})
	}
}

func TestParseACLToInterfaceKey(t *testing.T) {
	tests := []struct {
		name                  string
		key                   string
		expectedACLName       string
		expectedIface         string
		expectedFlow          string
		expectedIsACLIfaceKey bool
	}{
		{
			name:                  "ingress interface",
			key:                   "vpp/acl/acl1/interface/ingress/tap0",
			expectedACLName:       "acl1",
			expectedIface:         "tap0",
			expectedFlow:          IngressFlow,
			expectedIsACLIfaceKey: true,
		},
		{
			name:                  "egress interface",
			key:                   "vpp/acl/acl1/interface/egress/tap0",
			expectedACLName:       "acl1",
			expectedIface:         "tap0",
			expectedFlow:          EgressFlow,
			expectedIsACLIfaceKey: true,
		},
		{
			name:                  "Gbe interface",
			key:                   "vpp/acl/acl1/interface/ingress/GigabitEthernet0/8/0",
			expectedACLName:       "acl1",
			expectedIface:         "GigabitEthernet0/8/0",
			expectedFlow:          IngressFlow,
			expectedIsACLIfaceKey: true,
		},
		{
			name:                  "invalid acl name",
			key:                   "vpp/acl/<invalid>/interface/egress/tap0",
			expectedACLName:       "<invalid>",
			expectedIface:         "tap0",
			expectedFlow:          EgressFlow,
			expectedIsACLIfaceKey: true,
		},
		{
			name:                  "invalid flow",
			key:                   "vpp/acl/acl1/interface/<invalid>/tap0",
			expectedACLName:       "acl1",
			expectedIface:         "tap0",
			expectedFlow:          "<invalid>",
			expectedIsACLIfaceKey: true,
		},
		{
			name:                  "invalid interface",
			key:                   "vpp/acl/acl1/interface/ingress/<invalid>",
			expectedACLName:       "acl1",
			expectedIface:         "<invalid>",
			expectedFlow:          IngressFlow,
			expectedIsACLIfaceKey: true,
		},
		{
			name:                  "all parameters invalid",
			key:                   "vpp/acl/<invalid>/interface/<invalid>/<invalid>",
			expectedACLName:       "<invalid>",
			expectedIface:         "<invalid>",
			expectedFlow:          "<invalid>",
			expectedIsACLIfaceKey: true,
		},
		{
			name:                  "not ACLToInterface key",
			key:                   "vpp/config/v2/acl/acl1",
			expectedACLName:       "",
			expectedIface:         "",
			expectedFlow:          "",
			expectedIsACLIfaceKey: false,
		},
		{
			name:                  "not ACLToInterface key (cut after interface)",
			key:                   "vpp/acl/acl1/interface/",
			expectedACLName:       "",
			expectedIface:         "",
			expectedFlow:          "",
			expectedIsACLIfaceKey: false,
		},
		{
			name:                  "not ACLToInterface key (cut after flow)",
			key:                   "vpp/acl/acl1/interface/ingress",
			expectedACLName:       "",
			expectedIface:         "",
			expectedFlow:          "",
			expectedIsACLIfaceKey: false,
		},
		{
			name:                  "empty key",
			key:                   "",
			expectedACLName:       "",
			expectedIface:         "",
			expectedFlow:          "",
			expectedIsACLIfaceKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			aclName, iface, flow, isACLIfaceKey := ParseACLToInterfaceKey(test.key)
			if isACLIfaceKey != test.expectedIsACLIfaceKey {
				t.Errorf("expected isACLKey: %v\tgot: %v", test.expectedIsACLIfaceKey, isACLIfaceKey)
			}
			if aclName != test.expectedACLName {
				t.Errorf("expected aclName: %s\tgot: %s", test.expectedACLName, aclName)
			}
			if iface != test.expectedIface {
				t.Errorf("expected iface: %s\tgot: %s", test.expectedIface, iface)
			}
			if flow != test.expectedFlow {
				t.Errorf("expected flow: %s\tgot: %s", test.expectedFlow, flow)
			}
		})
	}
}
