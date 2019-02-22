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

import acl "github.com/ligato/vpp-agent/api/models/vpp/acl"

// ACLCtl provides access list related methods for vpp-agent-ctl
type ACLCtl interface {
	// PutIPAcl puts IPO access list config to the ETCD
	PutIPAcl() error
	// DeleteIPAcl removes IP access list config from the ETCD
	DeleteIPAcl() error
	// PutMACIPAcl puts MAC IP access list config to the ETCD
	PutMACIPAcl() error
	// DeleteMACIPAcl removes MAC IP access list config from the ETCD
	DeleteMACIPAcl() error
}

// PutIPAcl puts IPO access list config to the ETCD
func (ctl *VppAgentCtlImpl) PutIPAcl() error {
	accessList := &acl.ACL{
		Name: "aclip1",
		Rules: []*acl.ACL_Rule{
			// ACL IP rule
			{
				Action: acl.ACL_Rule_PERMIT,
				IpRule: &acl.ACL_Rule_IpRule{
					Ip: &acl.ACL_Rule_IpRule_Ip{
						SourceNetwork:      "192.168.1.1/32",
						DestinationNetwork: "10.20.0.1/24",
					},
				},
			},
			// ACL ICMP rule
			{
				Action: acl.ACL_Rule_PERMIT,
				IpRule: &acl.ACL_Rule_IpRule{
					Icmp: &acl.ACL_Rule_IpRule_Icmp{
						Icmpv6: false,
						IcmpCodeRange: &acl.ACL_Rule_IpRule_Icmp_Range{
							First: 150,
							Last:  250,
						},
						IcmpTypeRange: &acl.ACL_Rule_IpRule_Icmp_Range{
							First: 1150,
							Last:  1250,
						},
					},
				},
			},
			// ACL TCP rule
			{
				Action: acl.ACL_Rule_PERMIT,
				IpRule: &acl.ACL_Rule_IpRule{
					Tcp: &acl.ACL_Rule_IpRule_Tcp{
						TcpFlagsMask:  20,
						TcpFlagsValue: 10,
						SourcePortRange: &acl.ACL_Rule_IpRule_PortRange{
							LowerPort: 150,
							UpperPort: 250,
						},
						DestinationPortRange: &acl.ACL_Rule_IpRule_PortRange{
							LowerPort: 1150,
							UpperPort: 1250,
						},
					},
				},
			},
			// ACL UDP rule
			{
				Action: acl.ACL_Rule_PERMIT,
				IpRule: &acl.ACL_Rule_IpRule{
					Udp: &acl.ACL_Rule_IpRule_Udp{
						SourcePortRange: &acl.ACL_Rule_IpRule_PortRange{
							LowerPort: 150,
							UpperPort: 250,
						},
						DestinationPortRange: &acl.ACL_Rule_IpRule_PortRange{
							LowerPort: 1150,
							UpperPort: 1250,
						},
					},
				},
			},
		},
		Interfaces: &acl.ACL_Interfaces{
			Ingress: []string{"tap1", "tap2"},
			Egress:  []string{"tap1", "tap2"},
		},
	}

	ctl.Log.Infof("Access list put: %v", accessList)
	return ctl.broker.Put(acl.Key(accessList.Name), accessList)
}

// DeleteIPAcl removes IP access list config from the ETCD
func (ctl *VppAgentCtlImpl) DeleteIPAcl() error {
	aclKey := acl.Key("aclip1")

	ctl.Log.Infof("Deleted acl: %v", aclKey)
	_, err := ctl.broker.Delete(aclKey)
	return err
}

// PutMACIPAcl puts MAC IP access list config to the ETCD
func (ctl *VppAgentCtlImpl) PutMACIPAcl() error {
	accessList := &acl.ACL{
		Name: "aclmac1",
		// ACL rules
		Rules: []*acl.ACL_Rule{
			// ACL MAC IP rule. Note: do not combine ACL ip and mac ip rules in single acl
			{
				Action: acl.ACL_Rule_PERMIT,
				MacipRule: &acl.ACL_Rule_MacIpRule{
					SourceAddress:        "192.168.0.1",
					SourceAddressPrefix:  uint32(16),
					SourceMacAddress:     "11:44:0A:B8:4A:35",
					SourceMacAddressMask: "ff:ff:ff:ff:00:00",
				},
			},
		},
		Interfaces: &acl.ACL_Interfaces{
			Ingress: []string{"tap1", "tap2"},
			Egress:  []string{"tap1", "tap2"},
		},
	}

	ctl.Log.Infof("Access list put: %v", accessList)
	return ctl.broker.Put(acl.Key(accessList.Name), accessList)
}

// DeleteMACIPAcl removes MAC IP access list config from the ETCD
func (ctl *VppAgentCtlImpl) DeleteMACIPAcl() error {
	aclKey := acl.Key("aclmac1")

	ctl.Log.Infof("Deleted acl: %v", aclKey)
	_, err := ctl.broker.Delete(aclKey)
	return err
}
