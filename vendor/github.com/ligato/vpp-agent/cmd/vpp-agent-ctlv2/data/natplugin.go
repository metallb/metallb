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

package data

import nat "github.com/ligato/vpp-agent/api/models/vpp/nat"

// NatCtl NAT plugin related methods for vpp-agent-ctl
type NatCtl interface {
	// PutGlobalNat puts global NAT44 configuration to the ETCD
	PutGlobalNat() error
	// DeleteGlobalNat removes global NAT configuration from the ETCD
	DeleteGlobalNat() error
	// PutDNat puts DNAT configuration to the ETCD
	PutDNat() error
	// DeleteDNat removes DNAT configuration from the ETCD
	DeleteDNat() error
}

// PutGlobalNat puts global NAT44 configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutGlobalNat() error {
	natGlobal := &nat.Nat44Global{
		Forwarding: false,
		NatInterfaces: []*nat.Nat44Global_Interface{
			{
				Name:          "tap1",
				IsInside:      false,
				OutputFeature: false,
			},
			{
				Name:          "tap2",
				IsInside:      false,
				OutputFeature: true,
			},
			{
				Name:          "tap3",
				IsInside:      true,
				OutputFeature: false,
			},
		},
		AddressPool: []*nat.Nat44Global_Address{
			{
				VrfId:    0,
				Address:  "192.168.0.1",
				TwiceNat: false,
			},
			{
				VrfId:    0,
				Address:  "175.124.0.1",
				TwiceNat: false,
			},
			{
				VrfId:    0,
				Address:  "10.10.0.1",
				TwiceNat: false,
			},
		},
		VirtualReassembly: &nat.VirtualReassembly{
			Timeout:         10,
			MaxReassemblies: 20,
			MaxFragments:    10,
			DropFragments:   true,
		},
	}

	ctl.Log.Info("Global NAT put")
	return ctl.broker.Put(nat.GlobalNAT44Key(), natGlobal)
}

// DeleteGlobalNat removes global NAT configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteGlobalNat() error {
	ctl.Log.Info("Global NAT delete")
	_, err := ctl.broker.Delete(nat.GlobalNAT44Key())
	return err
}

// PutDNat puts DNAT configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutDNat() error {
	dNat := &nat.DNat44{
		Label: "dnat1",
		StMappings: []*nat.DNat44_StaticMapping{
			{
				ExternalInterface: "tap1",
				ExternalIp:        "192.168.0.1",
				ExternalPort:      8989,
				LocalIps: []*nat.DNat44_StaticMapping_LocalIP{
					{
						VrfId:       0,
						LocalIp:     "172.124.0.2",
						LocalPort:   6500,
						Probability: 40,
					},
					{
						VrfId:       0,
						LocalIp:     "172.125.10.5",
						LocalPort:   2300,
						Probability: 40,
					},
				},
				Protocol: 1,
				TwiceNat: nat.DNat44_StaticMapping_ENABLED,
			},
		},
		IdMappings: []*nat.DNat44_IdentityMapping{
			{
				VrfId:     0,
				IpAddress: "10.10.0.1",
				Port:      2525,
				Protocol:  0,
			},
		},
	}

	ctl.Log.Info("DNAT put: %v", dNat.Label)
	return ctl.broker.Put(nat.DNAT44Key(dNat.Label), dNat)
}

// DeleteDNat removes DNAT configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteDNat() error {
	dNat := nat.DNAT44Key("dnat1")

	ctl.Log.Infof("DNAt delete: %v", dNat)
	_, err := ctl.broker.Delete(dNat)
	return err
}
