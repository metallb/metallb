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

// package vpp-agent-ctl implements the vpp-agent-ctl test tool for testing
// VPP Agent plugins. In addition to testing, the vpp-agent-ctl tool can
// be used to demonstrate the usage of VPP Agent plugins and their APIs.

package main

import (
	"log"
	"os"

	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/servicelabel"
	linuxIf "github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	linuxL3 "github.com/ligato/vpp-agent/plugins/linux/model/l3"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/model/bfd"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/ipsec"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l4"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
	"github.com/ligato/vpp-agent/plugins/vpp/model/stn"
	"github.com/namsral/flag"
)

// VppAgentCtl is ctl context
type VppAgentCtl struct {
	Log             logging.Logger
	Commands        []string
	serviceLabel    servicelabel.Plugin
	bytesConnection *etcd.BytesConnectionEtcd
	broker          keyval.ProtoBroker
}

// Init creates new VppAgentCtl object with initialized fields
func initCtl(etcdCfg string, cmdSet []string) (*VppAgentCtl, error) {
	var err error
	ctl := &VppAgentCtl{
		Log:      logrus.DefaultLogger(),
		Commands: cmdSet,
	}
	// Parse service label
	flag.CommandLine.ParseEnv(os.Environ())
	ctl.serviceLabel.Init()
	// Establish ETCD connection
	ctl.bytesConnection, ctl.broker, err = ctl.createEtcdClient(etcdCfg)

	return ctl, err
}

// Access lists

// CreateACL puts access list config to the ETCD
func (ctl *VppAgentCtl) createACL() {
	accessList := acl.AccessLists{
		Acls: []*acl.AccessLists_Acl{
			// Single ACL entry
			{
				AclName: "acl1",
				// ACL rules
				Rules: []*acl.AccessLists_Acl_Rule{
					// ACL IP rule
					{
						AclAction: acl.AclAction_PERMIT,
						Match: &acl.AccessLists_Acl_Rule_Match{
							IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
								Ip: &acl.AccessLists_Acl_Rule_Match_IpRule_Ip{
									SourceNetwork:      "192.168.1.1/32",
									DestinationNetwork: "10.20.0.1/24",
								},
							},
						},
					},
					// ACL ICMP rule
					{
						AclAction: acl.AclAction_PERMIT,
						Match: &acl.AccessLists_Acl_Rule_Match{
							IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
								Icmp: &acl.AccessLists_Acl_Rule_Match_IpRule_Icmp{
									Icmpv6: false,
									IcmpCodeRange: &acl.AccessLists_Acl_Rule_Match_IpRule_Icmp_Range{
										First: 150,
										Last:  250,
									},
									IcmpTypeRange: &acl.AccessLists_Acl_Rule_Match_IpRule_Icmp_Range{
										First: 1150,
										Last:  1250,
									},
								},
							},
						},
					},
					// ACL TCP rule
					{
						AclAction: acl.AclAction_PERMIT,
						Match: &acl.AccessLists_Acl_Rule_Match{
							IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
								Tcp: &acl.AccessLists_Acl_Rule_Match_IpRule_Tcp{
									TcpFlagsMask:  20,
									TcpFlagsValue: 10,
									SourcePortRange: &acl.AccessLists_Acl_Rule_Match_IpRule_PortRange{
										LowerPort: 150,
										UpperPort: 250,
									},
									DestinationPortRange: &acl.AccessLists_Acl_Rule_Match_IpRule_PortRange{
										LowerPort: 1150,
										UpperPort: 1250,
									},
								},
							},
						},
					},
					// ACL UDP rule
					{
						AclAction: acl.AclAction_PERMIT,
						Match: &acl.AccessLists_Acl_Rule_Match{
							IpRule: &acl.AccessLists_Acl_Rule_Match_IpRule{
								Udp: &acl.AccessLists_Acl_Rule_Match_IpRule_Udp{
									SourcePortRange: &acl.AccessLists_Acl_Rule_Match_IpRule_PortRange{
										LowerPort: 150,
										UpperPort: 250,
									},
									DestinationPortRange: &acl.AccessLists_Acl_Rule_Match_IpRule_PortRange{
										LowerPort: 1150,
										UpperPort: 1250,
									},
								},
							},
						},
					},
					// ACL MAC IP rule. Note: do not combine ACL ip and mac ip rules in single acl
					//{
					//	Actions: &acl.AccessLists_Acl_Rule_Actions{
					//		AclAction: acl.AclAction_PERMIT,
					//	},
					//	Match: &acl.AccessLists_Acl_Rule_Match{
					//		MacipRule: &acl.AccessLists_Acl_Rule_Match_MacIpRule{
					//			SourceAddress: "192.168.0.1",
					//			SourceAddressPrefix: uint32(16),
					//			SourceMacAddress: "11:44:0A:B8:4A:35",
					//			SourceMacAddressMask: "ff:ff:ff:ff:00:00",
					//		},
					//	},
					//},
				},
				Interfaces: &acl.AccessLists_Acl_Interfaces{
					Ingress: []string{"tap1", "tap2"},
					Egress:  []string{"tap1", "tap2"},
				},
			},
		},
	}

	ctl.Log.Print(accessList.Acls[0])
	ctl.broker.Put(acl.Key(accessList.Acls[0].AclName), accessList.Acls[0])
}

// DeleteACL removes access list config from the ETCD
func (ctl *VppAgentCtl) deleteACL() {
	aclKey := acl.Key("acl1")

	ctl.Log.Println("Deleting", aclKey)
	ctl.broker.Delete(aclKey)
}

// Bidirectional forwarding detection

// CreateBfdSession puts bidirectional forwarding detection session config to the ETCD
func (ctl *VppAgentCtl) createBfdSession() {
	session := bfd.SingleHopBFD{
		Sessions: []*bfd.SingleHopBFD_Session{
			{
				Interface:             "memif1",
				Enabled:               true,
				SourceAddress:         "172.125.40.1",
				DestinationAddress:    "20.10.0.5",
				RequiredMinRxInterval: 8,
				DesiredMinTxInterval:  3,
				DetectMultiplier:      9,
				Authentication: &bfd.SingleHopBFD_Session_Authentication{
					KeyId:           1,
					AdvertisedKeyId: 1,
				},
			},
		},
	}

	ctl.Log.Println(session)
	ctl.broker.Put(bfd.SessionKey(session.Sessions[0].Interface), session.Sessions[0])
}

// DeleteBfdSession removes bidirectional forwarding detection session config from the ETCD
func (ctl *VppAgentCtl) deleteBfdSession() {
	sessionKey := bfd.SessionKey("memif1")

	ctl.Log.Println("Deleting", sessionKey)
	ctl.broker.Delete(sessionKey)
}

// CreateBfdKey puts bidirectional forwarding detection authentication key config to the ETCD
func (ctl *VppAgentCtl) createBfdKey() {
	authKey := bfd.SingleHopBFD{
		Keys: []*bfd.SingleHopBFD_Key{
			{
				Name:               "bfdKey1",
				Id:                 1,
				AuthenticationType: bfd.SingleHopBFD_Key_METICULOUS_KEYED_SHA1, // or Keyed sha1
				Secret:             "1981491891941891",
			},
		},
	}

	ctl.Log.Println(authKey)
	ctl.broker.Put(bfd.AuthKeysKey(string(authKey.Keys[0].Id)), authKey.Keys[0])
}

// DeleteBfdKey removes bidirectional forwarding detection authentication key config from the ETCD
func (ctl *VppAgentCtl) deleteBfdKey() {
	bfdAuthKeyKey := bfd.AuthKeysKey(string(1))

	ctl.Log.Println("Deleting", bfdAuthKeyKey)
	ctl.broker.Delete(bfdAuthKeyKey)
}

// CreateBfdEcho puts bidirectional forwarding detection echo detection config to the ETCD
func (ctl *VppAgentCtl) createBfdEcho() {
	echoFunction := bfd.SingleHopBFD{
		EchoFunction: &bfd.SingleHopBFD_EchoFunction{
			EchoSourceInterface: "memif1",
		},
	}

	ctl.Log.Println(echoFunction)
	ctl.broker.Put(bfd.EchoFunctionKey("memif1"), echoFunction.EchoFunction)
}

// DeleteBfdEcho removes bidirectional forwarding detection echo detection config from the ETCD
func (ctl *VppAgentCtl) deleteBfdEcho() {
	echoFunctionKey := bfd.EchoFunctionKey("memif1")

	ctl.Log.Println("Deleting", echoFunctionKey)
	ctl.broker.Delete(echoFunctionKey)
}

// VPP interfaces

// CreateEthernet puts ethernet type interface config to the ETCD
func (ctl *VppAgentCtl) createEthernet() {
	ethernet := &interfaces.Interfaces{
		Interfaces: []*interfaces.Interfaces_Interface{
			{
				Name:    "GigabitEthernet0/8/0",
				Type:    interfaces.InterfaceType_ETHERNET_CSMACD,
				Enabled: true,
				IpAddresses: []string{
					"192.168.1.1",
					"2001:db8:0:0:0:ff00:5168:2bc8/48",
				},
				//RxPlacementSettings: &interfaces.Interfaces_Interface_RxPlacementSettings{
				//	Queue: 0,
				//	Worker: 1,
				//},
				//Unnumbered: &interfaces.Interfaces_Interface_Unnumbered{
				//	IsUnnumbered: true,
				//	InterfaceWithIP: "memif1",
				//},
			},
		},
	}

	ctl.Log.Println(ethernet)
	ctl.broker.Put(interfaces.InterfaceKey(ethernet.Interfaces[0].Name), ethernet.Interfaces[0])
}

// DeleteEthernet removes ethernet type interface config from the ETCD
func (ctl *VppAgentCtl) deleteEthernet() {
	ethernetKey := interfaces.InterfaceKey("GigabitEthernet0/8/0")

	ctl.Log.Println("Deleting", ethernetKey)
	ctl.broker.Delete(ethernetKey)
}

// CreateTap puts TAP type interface config to the ETCD
func (ctl *VppAgentCtl) createTap() {
	tap := &interfaces.Interfaces{
		Interfaces: []*interfaces.Interfaces_Interface{
			{
				Name:        "tap1",
				Type:        interfaces.InterfaceType_TAP_INTERFACE,
				Enabled:     true,
				PhysAddress: "12:E4:0E:D5:BC:DC",
				IpAddresses: []string{
					"192.168.20.3/24",
				},
				//Unnumbered: &interfaces.Interfaces_Interface_Unnumbered{
				//	IsUnnumbered: true,
				//	InterfaceWithIP: "memif1",
				//},
				Tap: &interfaces.Interfaces_Interface_Tap{
					HostIfName: "tap-host",
				},
			},
		},
	}

	ctl.Log.Println(tap)
	ctl.broker.Put(interfaces.InterfaceKey(tap.Interfaces[0].Name), tap.Interfaces[0])
}

// DeleteTap removes TAP type interface config from the ETCD
func (ctl *VppAgentCtl) deleteTap() {
	tapKey := interfaces.InterfaceKey("tap1")

	ctl.Log.Println("Deleting", tapKey)
	ctl.broker.Delete(tapKey)
}

// CreateLoopback puts loopback type interface config to the ETCD
func (ctl *VppAgentCtl) createLoopback() {
	loopback := &interfaces.Interfaces{
		Interfaces: []*interfaces.Interfaces_Interface{
			{
				Name:        "loop1",
				Type:        interfaces.InterfaceType_SOFTWARE_LOOPBACK,
				Enabled:     true,
				PhysAddress: "7C:4E:E7:8A:63:68",
				Mtu:         1478,
				IpAddresses: []string{
					"192.168.25.3/24",
					"172.125.45.1/24",
				},
				//Unnumbered: &interfaces.Interfaces_Interface_Unnumbered{
				//	IsUnnumbered: true,
				//	InterfaceWithIP: "memif1",
				//},
			},
		},
	}

	ctl.Log.Println(loopback)
	ctl.broker.Put(interfaces.InterfaceKey(loopback.Interfaces[0].Name), loopback.Interfaces[0])
}

// DeleteLoopback removes loopback type interface config from the ETCD
func (ctl *VppAgentCtl) deleteLoopback() {
	loopbackKey := interfaces.InterfaceKey("loop1")

	ctl.Log.Println("Deleting", loopbackKey)
	ctl.broker.Delete(loopbackKey)
}

// CreateMemif puts memif type interface config to the ETCD
func (ctl *VppAgentCtl) createMemif() {
	memif := &interfaces.Interfaces{
		Interfaces: []*interfaces.Interfaces_Interface{
			{
				Name:        "memif1",
				Type:        interfaces.InterfaceType_MEMORY_INTERFACE,
				Enabled:     true,
				PhysAddress: "4E:93:2A:38:A7:77",
				Mtu:         1478,
				IpAddresses: []string{
					"172.125.40.1/24",
				},
				//Unnumbered: &interfaces.Interfaces_Interface_Unnumbered{
				//	IsUnnumbered: true,
				//	InterfaceWithIP: "memif1",
				//},
				Memif: &interfaces.Interfaces_Interface_Memif{
					Id:             1,
					Secret:         "secret",
					Master:         true,
					SocketFilename: "/tmp/memif1.sock",
				},
			},
		},
	}

	ctl.Log.Println(memif)
	ctl.broker.Put(interfaces.InterfaceKey(memif.Interfaces[0].Name), memif.Interfaces[0])
}

// DeleteMemif removes memif type interface config from the ETCD
func (ctl *VppAgentCtl) deleteMemif() {
	memifKey := interfaces.InterfaceKey("memif1")

	ctl.Log.Println("Deleting", memifKey)
	ctl.broker.Delete(memifKey)
}

// CreateVxLan puts VxLAN type interface config to the ETCD
func (ctl *VppAgentCtl) createVxlan() {
	vxlan := &interfaces.Interfaces{
		Interfaces: []*interfaces.Interfaces_Interface{
			{
				Name:    "vxlan1",
				Type:    interfaces.InterfaceType_VXLAN_TUNNEL,
				Enabled: true,
				Mtu:     1478,
				IpAddresses: []string{
					"172.125.40.1/24",
				},
				//Unnumbered: &interfaces.Interfaces_Interface_Unnumbered{
				//	IsUnnumbered: true,
				//	InterfaceWithIP: "memif1",
				//},
				Vxlan: &interfaces.Interfaces_Interface_Vxlan{
					//Multicast:  "if1",
					SrcAddress: "192.168.42.1",
					DstAddress: "192.168.42.2",
					Vni:        13,
				},
			},
		},
	}

	ctl.Log.Println(vxlan)
	ctl.broker.Put(interfaces.InterfaceKey(vxlan.Interfaces[0].Name), vxlan.Interfaces[0])
}

// DeleteVxlan removes VxLAN type interface config from the ETCD
func (ctl *VppAgentCtl) deleteVxlan() {
	vxlanKey := interfaces.InterfaceKey("vxlan1")

	ctl.Log.Println("Deleting", vxlanKey)
	ctl.broker.Delete(vxlanKey)
}

// CreateAfPacket puts Af-packet type interface config to the ETCD
func (ctl *VppAgentCtl) createAfPacket() {
	ifs := interfaces.Interfaces{
		Interfaces: []*interfaces.Interfaces_Interface{
			{
				Name:    "afpacket1",
				Type:    interfaces.InterfaceType_AF_PACKET_INTERFACE,
				Enabled: true,
				Mtu:     1500,
				IpAddresses: []string{
					"172.125.40.1/24",
					"192.168.12.1/24",
					"fd21:7408:186f::/48",
				},
				//Unnumbered: &interfaces.Interfaces_Interface_Unnumbered{
				//	IsUnnumbered: true,
				//	InterfaceWithIP: "memif1",
				//},
				Afpacket: &interfaces.Interfaces_Interface_Afpacket{
					HostIfName: "lo",
				},
			},
		},
	}

	ctl.Log.Println(ifs)
	ctl.broker.Put(interfaces.InterfaceKey(ifs.Interfaces[0].Name), ifs.Interfaces[0])
}

// DeleteAfPacket removes AF-Packet type interface config from the ETCD
func (ctl *VppAgentCtl) deleteAfPacket() {
	afPacketKey := interfaces.InterfaceKey("afpacket1")

	ctl.Log.Println("Deleting", afPacketKey)
	ctl.broker.Delete(afPacketKey)
}

// Linux interfaces

// CreateVethPair puts two VETH type interfaces to the ETCD
func (ctl *VppAgentCtl) createVethPair() {
	// Note: VETH interfaces are created in pairs
	veths := linuxIf.LinuxInterfaces{
		Interface: []*linuxIf.LinuxInterfaces_Interface{
			{
				Name:        "veth1",
				Type:        linuxIf.LinuxInterfaces_VETH,
				Enabled:     true,
				PhysAddress: "D2:74:8C:12:67:D2",
				Namespace: &linuxIf.LinuxInterfaces_Interface_Namespace{
					Name: "ns1",
					Type: linuxIf.LinuxInterfaces_Interface_Namespace_NAMED_NS,
				},
				Mtu: 1500,
				IpAddresses: []string{
					"192.168.22.1/24",
					"10.0.2.2/24",
				},
				Veth: &linuxIf.LinuxInterfaces_Interface_Veth{
					PeerIfName: "veth2",
				},
			},
			{
				Name:        "veth2",
				Type:        linuxIf.LinuxInterfaces_VETH,
				Enabled:     true,
				PhysAddress: "92:C7:42:67:AB:CD",
				Namespace: &linuxIf.LinuxInterfaces_Interface_Namespace{
					Name: "ns2",
					Type: linuxIf.LinuxInterfaces_Interface_Namespace_NAMED_NS,
				},
				Mtu: 1500,
				IpAddresses: []string{
					"192.168.22.5/24",
				},
				Veth: &linuxIf.LinuxInterfaces_Interface_Veth{
					PeerIfName: "veth1",
				},
			},
		},
	}

	ctl.Log.Println(veths)
	ctl.broker.Put(linuxIf.InterfaceKey(veths.Interface[0].Name), veths.Interface[0])
	ctl.broker.Put(linuxIf.InterfaceKey(veths.Interface[1].Name), veths.Interface[1])
}

// DeleteVethPair removes VETH pair interfaces from the ETCD
func (ctl *VppAgentCtl) deleteVethPair() {
	veth1Key := linuxIf.InterfaceKey("veth1")
	veth2Key := linuxIf.InterfaceKey("veth2")

	ctl.Log.Println("Deleting", veth1Key)
	ctl.broker.Delete(veth1Key)
	ctl.Log.Println("Deleting", veth2Key)
	ctl.broker.Delete(veth2Key)
}

// CreateLinuxTap puts linux TAP type interface configuration to the ETCD
func (ctl *VppAgentCtl) createLinuxTap() {
	linuxTap := linuxIf.LinuxInterfaces{
		Interface: []*linuxIf.LinuxInterfaces_Interface{
			{
				Name:        "tap1",
				HostIfName:  "tap-host",
				Type:        linuxIf.LinuxInterfaces_AUTO_TAP,
				Enabled:     true,
				PhysAddress: "BC:FE:E9:5E:07:04",
				Namespace: &linuxIf.LinuxInterfaces_Interface_Namespace{
					Name: "ns1",
					Type: linuxIf.LinuxInterfaces_Interface_Namespace_NAMED_NS,
				},
				Mtu: 1500,
				IpAddresses: []string{
					"172.52.45.127/24",
				},
			},
		},
	}

	ctl.Log.Println(linuxTap)
	ctl.broker.Put(linuxIf.InterfaceKey(linuxTap.Interface[0].Name), linuxTap.Interface[0])
}

// DeleteLinuxTap removes linux TAP type interface configuration from the ETCD
func (ctl *VppAgentCtl) deleteLinuxTap() {
	linuxTapKey := linuxIf.InterfaceKey("tap1")

	ctl.Log.Println("Deleting", linuxTapKey)
	ctl.broker.Delete(linuxTapKey)
}

// IPsec

// createIPsecSPD puts STD configuration to the ETCD
func (ctl *VppAgentCtl) createIPsecSPD() {
	spd := ipsec.SecurityPolicyDatabases_SPD{
		Name: "spd1",
		Interfaces: []*ipsec.SecurityPolicyDatabases_SPD_Interface{
			{
				Name: "tap1",
			},
			{
				Name: "loop1",
			},
		},
		PolicyEntries: []*ipsec.SecurityPolicyDatabases_SPD_PolicyEntry{
			{
				Priority:   100,
				IsOutbound: false,
				Action:     0,
				Protocol:   50,
			},
			{
				Priority:   100,
				IsOutbound: true,
				Action:     0,
				Protocol:   50,
			},
			{
				Priority:        10,
				IsOutbound:      false,
				RemoteAddrStart: "10.0.0.1",
				RemoteAddrStop:  "10.0.0.1",
				LocalAddrStart:  "10.0.0.2",
				LocalAddrStop:   "10.0.0.2",
				Action:          3,
				Sa:              "sa1",
			},
			{
				Priority:        10,
				IsOutbound:      true,
				RemoteAddrStart: "10.0.0.1",
				RemoteAddrStop:  "10.0.0.1",
				LocalAddrStart:  "10.0.0.2",
				LocalAddrStop:   "10.0.0.2",
				Action:          3,
				Sa:              "sa2",
			},
		},
	}

	ctl.Log.Println(spd)
	ctl.broker.Put(ipsec.SPDKey(spd.Name), &spd)
}

// deleteIPsecSPD removes STD configuration from the ETCD
func (ctl *VppAgentCtl) deleteIPsecSPD() {
	stdKey := ipsec.SPDKey("spd1")

	ctl.Log.Println("Deleting", stdKey)
	ctl.broker.Delete(stdKey)
}

// creteIPsecSA puts two security association configurations to the ETCD
func (ctl *VppAgentCtl) createIPsecSA() {
	sa1 := ipsec.SecurityAssociations_SA{
		Name:           "sa1",
		Spi:            1001,
		Protocol:       1,
		CryptoAlg:      1,
		CryptoKey:      "4a506a794f574265564551694d653768",
		IntegAlg:       2,
		IntegKey:       "4339314b55523947594d6d3547666b45764e6a58",
		EnableUdpEncap: true,
	}
	sa2 := ipsec.SecurityAssociations_SA{
		Name:           "sa2",
		Spi:            1000,
		Protocol:       1,
		CryptoAlg:      1,
		CryptoKey:      "4a506a794f574265564551694d653768",
		IntegAlg:       2,
		IntegKey:       "4339314b55523947594d6d3547666b45764e6a58",
		EnableUdpEncap: false,
	}

	ctl.Log.Println(sa1)
	ctl.broker.Put(ipsec.SAKey(sa1.Name), &sa1)
	ctl.Log.Println(sa2)
	ctl.broker.Put(ipsec.SAKey(sa2.Name), &sa2)
}

// deleteIPsecSA removes SA configuration from the ETCD
func (ctl *VppAgentCtl) deleteIPsecSA() {
	saKey1 := ipsec.SPDKey("sa1")
	saKey2 := ipsec.SPDKey("sa2")

	ctl.Log.Println("Deleting", saKey1)
	ctl.broker.Delete(saKey1)
	ctl.Log.Println("Deleting", saKey2)
	ctl.broker.Delete(saKey2)
}

// createIPSecTunnelInterface configures IPSec tunnel interface
func (ctl *VppAgentCtl) createIPSecTunnelInterface() {
	tunnelIf := ipsec.TunnelInterfaces_Tunnel{
		Name:            "ipsec0",
		Esn:             false,
		AntiReplay:      false,
		LocalSpi:        1000,
		RemoteSpi:       1001,
		LocalIp:         "10.0.0.2",
		RemoteIp:        "10.0.0.1",
		CryptoAlg:       1,
		LocalCryptoKey:  "4a506a794f574265564551694d653768",
		RemoteCryptoKey: "4a506a794f574265564551694d653768",
		IntegAlg:        2,
		LocalIntegKey:   "4339314b55523947594d6d3547666b45764e6a58",
		RemoteIntegKey:  "4339314b55523947594d6d3547666b45764e6a58",
		Enabled:         true,
		IpAddresses:     []string{"20.0.0.0/24"},
		Vrf:             0,
	}

	ctl.Log.Println(tunnelIf)
	ctl.broker.Put(ipsec.TunnelKey(tunnelIf.Name), &tunnelIf)
}

// deleteIPSecTunnelInterface removes IPSec tunnel interface
func (ctl *VppAgentCtl) deleteIPSecTunnelInterface() {
	tunnelKey := ipsec.TunnelKey("ipsec0")

	ctl.Log.Println("Deleting", tunnelKey)
	ctl.broker.Delete(tunnelKey)
}

// STN

// CreateStn puts STN configuration to the ETCD
func (ctl *VppAgentCtl) createStn() {
	stnRule := stn.STN_Rule{
		RuleName:  "rule1",
		IpAddress: "192.168.50.12",
		Interface: "memif1",
	}

	ctl.Log.Println(stnRule)
	ctl.broker.Put(stn.Key(stnRule.RuleName), &stnRule)
}

// DeleteStn removes STN configuration from the ETCD
func (ctl *VppAgentCtl) deleteStn() {
	stnRuleKey := stn.Key("rule1")

	ctl.Log.Println("Deleting", stnRuleKey)
	ctl.broker.Delete(stnRuleKey)
}

// Network address translation

// CreateGlobalNat puts global NAT44 configuration to the ETCD
func (ctl *VppAgentCtl) createGlobalNat() {
	natGlobal := &nat.Nat44Global{
		Forwarding: false,
		NatInterfaces: []*nat.Nat44Global_NatInterface{
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
		AddressPools: []*nat.Nat44Global_AddressPool{
			{
				VrfId:           0,
				FirstSrcAddress: "192.168.0.1",
				TwiceNat:        false,
			},
			{
				VrfId:           0,
				FirstSrcAddress: "175.124.0.1",
				LastSrcAddress:  "175.124.0.3",
				TwiceNat:        false,
			},
			{
				VrfId:           0,
				FirstSrcAddress: "10.10.0.1",
				LastSrcAddress:  "10.10.0.2",
				TwiceNat:        false,
			},
		},
		VirtualReassemblyIpv4: &nat.Nat44Global_VirtualReassembly{
			Timeout:  10,
			MaxReass: 20,
			MaxFrag:  10,
			DropFrag: true,
		},
		VirtualReassemblyIpv6: &nat.Nat44Global_VirtualReassembly{
			Timeout:  15,
			MaxReass: 25,
			MaxFrag:  15,
			DropFrag: false,
		},
	}

	ctl.Log.Println(natGlobal)
	ctl.broker.Put(nat.GlobalPrefix, natGlobal)
}

// DeleteGlobalNat removes global NAT configuration from the ETCD
func (ctl *VppAgentCtl) deleteGlobalNat() {
	globalNat := nat.GlobalPrefix

	ctl.Log.Println("Deleting", globalNat)
	ctl.broker.Delete(globalNat)
}

// CreateSNat puts SNAT configuration to the ETCD
func (ctl *VppAgentCtl) createSNat() {
	// Note: SNAT not implemented
	sNat := &nat.Nat44SNat_SNatConfig{
		Label: "snat1",
	}

	ctl.Log.Println(sNat)
	ctl.broker.Put(nat.SNatKey(sNat.Label), sNat)
}

// DeleteSNat removes SNAT configuration from the ETCD
func (ctl *VppAgentCtl) deleteSNat() {
	sNat := nat.SNatKey("snat1")

	ctl.Log.Println("Deleting", sNat)
	ctl.broker.Delete(sNat)
}

// CreateDNat puts DNAT configuration to the ETCD
func (ctl *VppAgentCtl) createDNat() {
	// DNat config
	dNat := &nat.Nat44DNat_DNatConfig{
		Label: "dnat1",
		StMappings: []*nat.Nat44DNat_DNatConfig_StaticMapping{
			{
				ExternalInterface: "tap1",
				ExternalIp:        "192.168.0.1",
				ExternalPort:      8989,
				LocalIps: []*nat.Nat44DNat_DNatConfig_StaticMapping_LocalIP{
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
				TwiceNat: nat.TwiceNatMode_ENABLED,
			},
		},
		IdMappings: []*nat.Nat44DNat_DNatConfig_IdentityMapping{
			{
				VrfId:     0,
				IpAddress: "10.10.0.1",
				Port:      2525,
				Protocol:  0,
			},
		},
	}

	ctl.Log.Println(dNat)
	ctl.broker.Put(nat.DNatKey(dNat.Label), dNat)
}

// DeleteDNat removes DNAT configuration from the ETCD
func (ctl *VppAgentCtl) deleteDNat() {
	dNat := nat.DNatKey("dnat1")

	ctl.Log.Println("Deleting", dNat)
	ctl.broker.Delete(dNat)
}

// Bridge domains

// CreateBridgeDomain puts L2 bridge domain configuration to the ETCD
func (ctl *VppAgentCtl) createBridgeDomain() {
	bd := l2.BridgeDomains{
		BridgeDomains: []*l2.BridgeDomains_BridgeDomain{
			{
				Name:                "bd1",
				Learn:               true,
				ArpTermination:      true,
				Flood:               true,
				UnknownUnicastFlood: true,
				Forward:             true,
				MacAge:              0,
				Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
					{
						Name: "loop1",
						BridgedVirtualInterface: true,
						SplitHorizonGroup:       0,
					},
					{
						Name: "tap1",
						BridgedVirtualInterface: false,
						SplitHorizonGroup:       1,
					},
					{
						Name: "memif1",
						BridgedVirtualInterface: false,
						SplitHorizonGroup:       2,
					},
				},
				ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
					{
						IpAddress:   "192.168.50.20",
						PhysAddress: "A7:5D:44:D8:E6:51",
					},
				},
			},
		},
	}

	ctl.Log.Println(bd)
	ctl.broker.Put(l2.BridgeDomainKey(bd.BridgeDomains[0].Name), bd.BridgeDomains[0])
}

// DeleteBridgeDomain removes bridge domain configuration from the ETCD
func (ctl *VppAgentCtl) deleteBridgeDomain() {
	bdKey := l2.BridgeDomainKey("bd1")

	ctl.Log.Println("Deleting", bdKey)
	ctl.broker.Delete(bdKey)
}

// FIB

// CreateFib puts L2 FIB entry configuration to the ETCD
func (ctl *VppAgentCtl) createFib() {
	fib := l2.FibTable{
		FibTableEntries: []*l2.FibTable_FibEntry{
			{
				PhysAddress:             "EA:FE:3C:64:A7:44",
				BridgeDomain:            "bd1",
				OutgoingInterface:       "loop1",
				StaticConfig:            true,
				BridgedVirtualInterface: true,
				Action:                  l2.FibTable_FibEntry_FORWARD, // or DROP
			},
		},
	}

	ctl.Log.Println(fib)
	ctl.broker.Put(l2.FibKey(fib.FibTableEntries[0].BridgeDomain, fib.FibTableEntries[0].PhysAddress), fib.FibTableEntries[0])
}

// DeleteFib removes FIB entry configuration from the ETCD
func (ctl *VppAgentCtl) deleteFib() {
	fibKey := l2.FibKey("bd1", "EA:FE:3C:64:A7:44")

	ctl.Log.Println("Deleting", fibKey)
	ctl.broker.Delete(fibKey)
}

// L2 xConnect

// CreateXConn puts L2 cross connect configuration to the ETCD
func (ctl *VppAgentCtl) createXConn() {
	xc := l2.XConnectPairs{
		XConnectPairs: []*l2.XConnectPairs_XConnectPair{
			{
				ReceiveInterface:  "tap1",
				TransmitInterface: "loop1",
			},
		},
	}

	ctl.Log.Println(xc)
	ctl.broker.Put(l2.XConnectKey(xc.XConnectPairs[0].ReceiveInterface), xc.XConnectPairs[0])
}

// DeleteXConn removes cross connect configuration from the ETCD
func (ctl *VppAgentCtl) deleteXConn() {
	xcKey := l2.XConnectKey("loop1")

	ctl.Log.Println("Deleting", xcKey)
	ctl.broker.Delete(xcKey)
}

// VPP routes

// CreateRoute puts VPP route configuration to the ETCD
func (ctl *VppAgentCtl) createRoute() {
	routes := l3.StaticRoutes{
		Routes: []*l3.StaticRoutes_Route{
			{
				VrfId:             0,
				DstIpAddr:         "10.1.1.3/32",
				NextHopAddr:       "192.168.1.13",
				Weight:            6,
				OutgoingInterface: "tap1",
			},
			// inter-vrf route without next hop addr (recursive lookup)
			//{
			//	Type:      l3.StaticRoutes_Route_INTER_VRF,
			//	VrfId:     0,
			//	DstIpAddr: "1.2.3.4/32",
			//	ViaVrfId:  1,
			//},
			// inter-vrf route with next hop addr
			//{
			//	Type:        l3.StaticRoutes_Route_INTER_VRF,
			//	VrfId:       1,
			//	DstIpAddr:   "10.1.1.3/32",
			//	NextHopAddr: "192.168.1.13",
			//	ViaVrfId:    0,
			//},
		},
	}

	for _, r := range routes.Routes {
		ctl.Log.Print(r)
		ctl.broker.Put(l3.RouteKey(r.VrfId, r.DstIpAddr, r.NextHopAddr), r)
	}
}

// DeleteRoute removes VPP route configuration from the ETCD
func (ctl *VppAgentCtl) deleteRoute() {
	routeKey := l3.RouteKey(0, "10.1.1.3/32", "192.168.1.13")

	ctl.Log.Println("Deleting", routeKey)
	ctl.broker.Delete(routeKey)
}

// Linux routes

// CreateLinuxRoute puts linux route configuration to the ETCD
func (ctl *VppAgentCtl) createLinuxRoute() {
	linuxRoutes := linuxL3.LinuxStaticRoutes{
		Route: []*linuxL3.LinuxStaticRoutes_Route{
			// Static route
			{
				Name:      "route1",
				DstIpAddr: "10.0.2.0/24",
				Interface: "veth1",
				Metric:    100,
				Namespace: &linuxL3.LinuxStaticRoutes_Route_Namespace{
					Name: "ns1",
					Type: linuxL3.LinuxStaticRoutes_Route_Namespace_NAMED_NS,
				},
			},
			// Default route
			{
				Name:      "defRoute",
				Default:   true,
				GwAddr:    "10.0.2.2",
				Interface: "veth1",
				Metric:    100,
				Namespace: &linuxL3.LinuxStaticRoutes_Route_Namespace{
					Name: "ns1",
					Type: linuxL3.LinuxStaticRoutes_Route_Namespace_NAMED_NS,
				},
			},
		},
	}

	ctl.Log.Println(linuxRoutes)
	ctl.broker.Put(linuxL3.StaticRouteKey(linuxRoutes.Route[0].Name), linuxRoutes.Route[0])
	ctl.broker.Put(linuxL3.StaticRouteKey(linuxRoutes.Route[1].Name), linuxRoutes.Route[1])
}

// DeleteLinuxRoute removes linux route configuration from the ETCD
func (ctl *VppAgentCtl) deleteLinuxRoute() {
	linuxStaticRouteKey := linuxL3.StaticRouteKey("route1")
	linuxDefaultRouteKey := linuxL3.StaticRouteKey("defRoute")

	ctl.Log.Println("Deleting", linuxStaticRouteKey)
	ctl.broker.Delete(linuxStaticRouteKey)
	ctl.Log.Println("Deleting", linuxDefaultRouteKey)
	ctl.broker.Delete(linuxDefaultRouteKey)
}

// VPP ARP

// CreateArp puts VPP ARP entry configuration to the ETCD
func (ctl *VppAgentCtl) createArp() {
	arp := l3.ArpTable{
		ArpEntries: []*l3.ArpTable_ArpEntry{
			{
				Interface:   "tap1",
				IpAddress:   "192.168.10.21",
				PhysAddress: "59:6C:45:59:8E:BD",
				Static:      true,
			},
		},
	}

	ctl.Log.Println(arp)
	ctl.broker.Put(l3.ArpEntryKey(arp.ArpEntries[0].Interface, arp.ArpEntries[0].IpAddress), arp.ArpEntries[0])
}

// DeleteArp removes VPP ARP entry configuration from the ETCD
func (ctl *VppAgentCtl) deleteArp() {
	arpKey := l3.ArpEntryKey("tap1", "192.168.10.21")

	ctl.Log.Println("Deleting", arpKey)
	ctl.broker.Delete(arpKey)
}

// AddProxyArpInterfaces puts VPP proxy ARP interface configuration to the ETCD
func (ctl *VppAgentCtl) addProxyArpInterfaces() {
	proxyArpIf := l3.ProxyArpInterfaces{
		InterfaceLists: []*l3.ProxyArpInterfaces_InterfaceList{
			{
				Label: "proxyArpIf1",
				Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{
					{
						Name: "tap1",
					},
					{
						Name: "tap2",
					},
				},
			},
		},
	}

	log.Println(proxyArpIf)
	ctl.broker.Put(l3.ProxyArpInterfaceKey(proxyArpIf.InterfaceLists[0].Label), proxyArpIf.InterfaceLists[0])
}

// DeleteProxyArpInterfaces removes VPP proxy ARP interface configuration from the ETCD
func (ctl *VppAgentCtl) deleteProxyArpInterfaces() {
	arpKey := l3.ProxyArpInterfaceKey("proxyArpIf1")

	ctl.Log.Println("Deleting", arpKey)
	ctl.broker.Delete(arpKey)
}

// AddProxyArpRanges puts VPP proxy ARP range configuration to the ETCD
func (ctl *VppAgentCtl) addProxyArpRanges() {
	proxyArpRng := l3.ProxyArpRanges{
		RangeLists: []*l3.ProxyArpRanges_RangeList{
			{
				Label: "proxyArpRng1",
				Ranges: []*l3.ProxyArpRanges_RangeList_Range{
					{
						FirstIp: "124.168.10.5",
						LastIp:  "124.168.10.10",
					},
					{
						FirstIp: "172.154.10.5",
						LastIp:  "172.154.10.10",
					},
				},
			},
		},
	}

	log.Println(proxyArpRng)
	ctl.broker.Put(l3.ProxyArpRangeKey(proxyArpRng.RangeLists[0].Label), proxyArpRng.RangeLists[0])
}

// DeleteProxyArpranges removes VPP proxy ARP range configuration from the ETCD
func (ctl *VppAgentCtl) deleteProxyArpRanges() {
	arpKey := l3.ProxyArpRangeKey("proxyArpRng1")

	ctl.Log.Println("Deleting", arpKey)
	ctl.broker.Delete(arpKey)
}

// SetIPScanNeigh puts VPP IP scan neighbor configuration to the ETCD
func (ctl *VppAgentCtl) setIPScanNeigh() {
	ipScanNeigh := &l3.IPScanNeighbor{
		Mode:           l3.IPScanNeighbor_BOTH,
		ScanInterval:   11,
		MaxProcTime:    36,
		MaxUpdate:      5,
		ScanIntDelay:   16,
		StaleThreshold: 26,
	}

	log.Println(ipScanNeigh)
	ctl.broker.Put(l3.IPScanNeighPrefix, ipScanNeigh)
}

// UnsetIPScanNeigh removes VPP IP scan neighbor configuration from the ETCD
func (ctl *VppAgentCtl) unsetIPScanNeigh() {
	ctl.Log.Println("Deleting", l3.IPScanNeighPrefix)
	ctl.broker.Delete(l3.IPScanNeighPrefix)
}

// Linux ARP

// CreateLinuxArp puts linux ARP entry configuration to the ETCD
func (ctl *VppAgentCtl) createLinuxArp() {
	linuxArp := linuxL3.LinuxStaticArpEntries{
		ArpEntry: []*linuxL3.LinuxStaticArpEntries_ArpEntry{
			{
				Name:      "arp1",
				Interface: "veth1",
				Namespace: &linuxL3.LinuxStaticArpEntries_ArpEntry_Namespace{
					Name: "ns1",
					Type: linuxL3.LinuxStaticArpEntries_ArpEntry_Namespace_NAMED_NS,
				},
				IpAddr:    "130.0.0.1",
				HwAddress: "46:06:18:DB:05:3A",
				State: &linuxL3.LinuxStaticArpEntries_ArpEntry_NudState{
					Type: linuxL3.LinuxStaticArpEntries_ArpEntry_NudState_PERMANENT, // or NOARP, REACHABLE, STALE
				},
				IpFamily: &linuxL3.LinuxStaticArpEntries_ArpEntry_IpFamily{
					Family: linuxL3.LinuxStaticArpEntries_ArpEntry_IpFamily_IPV4, // or IPv6, ALL, MPLS
				},
			},
		},
	}

	ctl.Log.Println(linuxArp)
	ctl.broker.Put(linuxL3.StaticArpKey(linuxArp.ArpEntry[0].Name), linuxArp.ArpEntry[0])
}

// DeleteLinuxArp removes Linux ARP entry configuration from the ETCD
func (ctl *VppAgentCtl) deleteLinuxArp() {
	linuxArpKey := linuxL3.StaticArpKey("arp1")

	ctl.Log.Println("Deleting", linuxArpKey)
	ctl.broker.Delete(linuxArpKey)
}

// L4 plugin

// EnableL4Features enables L4 configuration on the VPP
func (ctl *VppAgentCtl) enableL4Features() {
	l4Features := &l4.L4Features{
		Enabled: true,
	}

	ctl.Log.Println(l4Features)
	ctl.broker.Put(l4.FeatureKey(), l4Features)
}

// DisableL4Features disables L4 configuration on the VPP
func (ctl *VppAgentCtl) disableL4Features() {
	l4Features := &l4.L4Features{
		Enabled: false,
	}

	ctl.Log.Println(l4Features)
	ctl.broker.Put(l4.FeatureKey(), l4Features)
}

// CreateAppNamespace puts application namespace configuration to the ETCD
func (ctl *VppAgentCtl) createAppNamespace() {
	appNs := l4.AppNamespaces{
		AppNamespaces: []*l4.AppNamespaces_AppNamespace{
			{
				NamespaceId: "appns1",
				Secret:      1,
				Interface:   "tap1",
			},
		},
	}

	ctl.Log.Println(appNs)
	ctl.broker.Put(l4.AppNamespacesKey(appNs.AppNamespaces[0].NamespaceId), appNs.AppNamespaces[0])
}

// DeleteAppNamespace removes application namespace configuration from the ETCD
func (ctl *VppAgentCtl) deleteAppNamespace() {
	// Note: application namespace cannot be removed, missing API in VPP
	ctl.Log.Println("App namespace delete not supported")
}

// TXN transactions

// CreateTxn demonstrates transaction - two interfaces and bridge domain put to the ETCD using txn
func (ctl *VppAgentCtl) createTxn() {
	ifs := interfaces.Interfaces{
		Interfaces: []*interfaces.Interfaces_Interface{
			{
				Name:    "tap1",
				Type:    interfaces.InterfaceType_TAP_INTERFACE,
				Enabled: true,
				Mtu:     1500,
				IpAddresses: []string{
					"10.4.4.1/24",
				},
				Tap: &interfaces.Interfaces_Interface_Tap{
					HostIfName: "tap1",
				},
			},
			{
				Name:    "tap2",
				Type:    interfaces.InterfaceType_TAP_INTERFACE,
				Enabled: true,
				Mtu:     1500,
				IpAddresses: []string{
					"10.4.4.2/24",
				},
				Tap: &interfaces.Interfaces_Interface_Tap{
					HostIfName: "tap2",
				},
			},
		},
	}

	bd := l2.BridgeDomains{
		BridgeDomains: []*l2.BridgeDomains_BridgeDomain{
			{
				Name:                "bd1",
				Flood:               false,
				UnknownUnicastFlood: false,
				Forward:             true,
				Learn:               true,
				ArpTermination:      false,
				MacAge:              0,
				Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
					{
						Name: "tap1",
						BridgedVirtualInterface: true,
						SplitHorizonGroup:       0,
					},
					{
						Name: "tap2",
						BridgedVirtualInterface: false,
						SplitHorizonGroup:       0,
					},
				},
			},
		},
	}

	t := ctl.broker.NewTxn()
	t.Put(interfaces.InterfaceKey(ifs.Interfaces[0].Name), ifs.Interfaces[0])
	t.Put(interfaces.InterfaceKey(ifs.Interfaces[1].Name), ifs.Interfaces[1])
	t.Put(l2.BridgeDomainKey(bd.BridgeDomains[0].Name), bd.BridgeDomains[0])

	t.Commit()
}

// DeleteTxn demonstrates transaction - two interfaces and bridge domain removed from the ETCD using txn
func (ctl *VppAgentCtl) deleteTxn() {
	ctl.Log.Println("Deleting txn items")
	t := ctl.broker.NewTxn()
	t.Delete(interfaces.InterfaceKey("tap1"))
	t.Delete(interfaces.InterfaceKey("tap2"))
	t.Delete(l2.BridgeDomainKey("bd1"))

	t.Commit()
}

// Error reporting

// ReportIfaceErrorState reports interface status data to the ETCD
func (ctl *VppAgentCtl) reportIfaceErrorState() {
	ifErr, err := ctl.broker.ListValues(interfaces.ErrorPrefix)
	if err != nil {
		ctl.Log.Fatal(err)
		return
	}
	for {
		kv, allReceived := ifErr.GetNext()
		if allReceived {
			break
		}
		entry := &interfaces.InterfaceErrors_Interface{}
		err := kv.GetValue(entry)
		if err != nil {
			ctl.Log.Fatal(err)
			return
		}
		ctl.Log.Println(entry)
	}
}

// ReportBdErrorState reports bridge domain status data to the ETCD
func (ctl *VppAgentCtl) reportBdErrorState() {
	bdErr, err := ctl.broker.ListValues(l2.BdErrPrefix)
	if err != nil {
		ctl.Log.Fatal(err)
		return
	}
	for {
		kv, allReceived := bdErr.GetNext()
		if allReceived {
			break
		}
		entry := &l2.BridgeDomainErrors_BridgeDomain{}
		err := kv.GetValue(entry)
		if err != nil {
			ctl.Log.Fatal(err)
			return
		}

		ctl.Log.Println(entry)
	}
}
