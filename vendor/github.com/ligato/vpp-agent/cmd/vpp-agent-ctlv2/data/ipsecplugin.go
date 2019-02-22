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

import ipsec "github.com/ligato/vpp-agent/api/models/vpp/ipsec"

// IPSecCtl IPSec plugin related methods for vpp-agent-ctl (SPD, SA)
type IPSecCtl interface {
	// PutIPSecSPD puts STD configuration to the ETCD
	PutIPSecSPD() error
	// DeleteIPSecSPD removes STD configuration from the ETCD
	DeleteIPSecSPD() error
	// PutIPSecSA puts two security association configurations to the ETCD
	PutIPSecSA() error
	// DeleteIPSecSA removes SA configuration from the ETCD
	DeleteIPSecSA() error
}

// PutIPSecSPD puts STD configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutIPSecSPD() error {
	spd := ipsec.SecurityPolicyDatabase{
		Index: "1",
		Interfaces: []*ipsec.SecurityPolicyDatabase_Interface{
			{
				Name: "tap1",
			},
			{
				Name: "loop1",
			},
		},
		PolicyEntries: []*ipsec.SecurityPolicyDatabase_PolicyEntry{
			{
				Priority:        10,
				IsOutbound:      false,
				RemoteAddrStart: "10.0.0.1",
				RemoteAddrStop:  "10.0.0.1",
				LocalAddrStart:  "10.0.0.2",
				LocalAddrStop:   "10.0.0.2",
				Action:          3,
				SaIndex:         "1",
			},
			{
				Priority:        10,
				IsOutbound:      true,
				RemoteAddrStart: "10.0.0.1",
				RemoteAddrStop:  "10.0.0.1",
				LocalAddrStart:  "10.0.0.2",
				LocalAddrStop:   "10.0.0.2",
				Action:          3,
				SaIndex:         "2",
			},
		},
	}

	ctl.Log.Infof("IPSec SPD put: %v", spd.Index)
	return ctl.broker.Put(ipsec.SPDKey(spd.Index), &spd)
}

// DeleteIPSecSPD removes STD configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteIPSecSPD() error {
	spdKey := ipsec.SPDKey("1")

	ctl.Log.Infof("IPSec SPD delete: %v", spdKey)
	_, err := ctl.broker.Delete(spdKey)
	return err
}

// PutIPSecSA puts two security association configurations to the ETCD
func (ctl *VppAgentCtlImpl) PutIPSecSA() error {
	sa1 := ipsec.SecurityAssociation{
		Index:          "1",
		Spi:            1001,
		Protocol:       1,
		CryptoAlg:      1,
		CryptoKey:      "4a506a794f574265564551694d653768",
		IntegAlg:       2,
		IntegKey:       "4339314b55523947594d6d3547666b45764e6a58",
		EnableUdpEncap: true,
	}
	sa2 := ipsec.SecurityAssociation{
		Index:          "2",
		Spi:            1000,
		Protocol:       1,
		CryptoAlg:      1,
		CryptoKey:      "4a506a794f574265564551694d653768",
		IntegAlg:       2,
		IntegKey:       "4339314b55523947594d6d3547666b45764e6a58",
		EnableUdpEncap: false,
	}

	ctl.Log.Infof("IPSec SA put: %v", sa1.Index)
	if err := ctl.broker.Put(ipsec.SAKey(sa1.Index), &sa1); err != nil {
		return err
	}
	ctl.Log.Infof("IPSec SA put: %v", sa2.Index)
	return ctl.broker.Put(ipsec.SAKey(sa2.Index), &sa2)
}

// DeleteIPSecSA removes SA configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteIPSecSA() error {
	saKey1 := ipsec.SAKey("1")
	saKey2 := ipsec.SAKey("2")

	ctl.Log.Infof("IPSec SA delete: %v", saKey1)
	if _, err := ctl.broker.Delete(saKey1); err != nil {
		return err
	}
	ctl.Log.Infof("IPSec SA delete: %v", saKey2)
	_, err := ctl.broker.Delete(saKey2)
	return err
}
