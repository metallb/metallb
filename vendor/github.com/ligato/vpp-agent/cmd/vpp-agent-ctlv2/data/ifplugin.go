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

import (
	linuxIf "github.com/ligato/vpp-agent/api/models/linux/interfaces"
	"github.com/ligato/vpp-agent/api/models/linux/namespace"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
)

// InterfacesCtl interface plugin related methods for vpp-agent-ctl (interfaces including linux ones)
type InterfacesCtl interface {
	// PutPhysicalInterface puts ethernet type interface config to the ETCD
	PutDPDKInterface() error
	// DeleteDPDKInterface removes ethernet type interface config from the ETCD
	DeleteDPDKInterface() error
	// PutTap puts TAP type interface config to the ETCD
	PutTap() error
	// DeleteTap removes TAP type interface config from the ETCD
	DeleteTap() error
	// PutLoopback puts loopback type interface config to the ETCD
	PutLoopback() error
	// DeleteLoopback removes loopback type interface config from the ETCD
	DeleteLoopback() error
	// PutMemoryInterface puts memory type interface config to the ETCD
	PutMemoryInterface() error
	// DeleteMemoryInterface removes memory type interface config from the ETCD
	DeleteMemoryInterface() error
	// PutVxLan puts VxLAN type interface config to the ETCD
	PutVxLan() error
	// DeleteVxLan removes VxLAN type interface config from the ETCD
	DeleteVxLan() error
	// PutAfPacket puts Af-packet type interface config to the ETCD
	PutAfPacket() error
	// DeleteAfPacket removes AF-Packet type interface config from the ETCD
	DeleteAfPacket() error
	// PutIPSecTunnelInterface configures IPSec tunnel interface
	PutIPSecTunnelInterface() error
	// DeleteIPSecTunnelInterface removes IPSec tunnel interface
	DeleteIPSecTunnelInterface() error
	// PutVEthPair puts two VETH type interfaces to the ETCD
	PutVEthPair() error
	// DeleteVEthPair removes VETH pair interfaces from the ETCD
	DeleteVEthPair() error
	// PutLinuxTap puts linux TAP type interface configuration to the ETCD
	PutLinuxTap() error
	// DeleteLinuxTap removes linux TAP type interface configuration from the ETCD
	DeleteLinuxTap() error
}

// PutDPDKInterface puts ethernet type interface config to the ETCD
func (ctl *VppAgentCtlImpl) PutDPDKInterface() error {
	ethernet := &interfaces.Interface{
		Name:    "GigabitEthernet0/8/0",
		Type:    interfaces.Interface_DPDK,
		Enabled: true,
		IpAddresses: []string{
			"192.168.1.1",
			"2001:db8:0:0:0:ff00:5168:2bc8/48",
		},
	}

	ctl.Log.Infof("Interface put: %v", ethernet)
	return ctl.broker.Put(interfaces.InterfaceKey(ethernet.Name), ethernet)
}

// DeleteDPDKInterface removes ethernet type interface config from the ETCD
func (ctl *VppAgentCtlImpl) DeleteDPDKInterface() error {
	ethernetKey := interfaces.InterfaceKey("GigabitEthernet0/8/0")

	ctl.Log.Infof("Interface delete: %v", ethernetKey)
	_, err := ctl.broker.Delete(ethernetKey)
	return err
}

// PutTap puts TAP type interface config to the ETCD
func (ctl *VppAgentCtlImpl) PutTap() error {
	tap := &interfaces.Interface{
		Name:        "tap1",
		Type:        interfaces.Interface_TAP,
		Enabled:     true,
		PhysAddress: "12:E4:0E:D5:BC:DC",
		IpAddresses: []string{
			"192.168.20.3/24",
		},
		Link: &interfaces.Interface_Tap{
			Tap: &interfaces.TapLink{
				HostIfName: "tap-host",
			},
		},
	}

	ctl.Log.Infof("Interface put: %v", tap)
	return ctl.broker.Put(interfaces.InterfaceKey(tap.Name), tap)
}

// DeleteTap removes TAP type interface config from the ETCD
func (ctl *VppAgentCtlImpl) DeleteTap() error {
	tapKey := interfaces.InterfaceKey("tap1")

	ctl.Log.Infof("Interface delete: %v", tapKey)
	_, err := ctl.broker.Delete(tapKey)
	return err
}

// PutLoopback puts loopback type interface config to the ETCD
func (ctl *VppAgentCtlImpl) PutLoopback() error {
	loopback := &interfaces.Interface{
		Name:        "loop1",
		Type:        interfaces.Interface_SOFTWARE_LOOPBACK,
		Enabled:     true,
		PhysAddress: "7C:4E:E7:8A:63:68",
		Mtu:         1478,
		IpAddresses: []string{
			"192.168.25.3/24",
			"172.125.45.1/24",
		},
	}

	ctl.Log.Infof("Interface put: %v", loopback)
	return ctl.broker.Put(interfaces.InterfaceKey(loopback.Name), loopback)
}

// DeleteLoopback removes loopback type interface config from the ETCD
func (ctl *VppAgentCtlImpl) DeleteLoopback() error {
	loopbackKey := interfaces.InterfaceKey("loop1")

	ctl.Log.Infof("Interface delete: %v", loopbackKey)
	_, err := ctl.broker.Delete(loopbackKey)
	return err
}

// PutMemoryInterface puts memif type interface config to the ETCD
func (ctl *VppAgentCtlImpl) PutMemoryInterface() error {
	memif := &interfaces.Interface{
		Name:        "memif1",
		Type:        interfaces.Interface_MEMIF,
		Enabled:     true,
		PhysAddress: "4E:93:2A:38:A7:77",
		Mtu:         1478,
		IpAddresses: []string{
			"172.125.40.1/24",
		},
		Link: &interfaces.Interface_Memif{
			Memif: &interfaces.MemifLink{
				Id:             1,
				Secret:         "secret",
				Master:         true,
				SocketFilename: "/tmp/memif1.sock",
			},
		},
	}

	ctl.Log.Infof("Interface put: %v", memif)
	return ctl.broker.Put(interfaces.InterfaceKey(memif.Name), memif)
}

// DeleteMemoryInterface removes memif type interface config from the ETCD
func (ctl *VppAgentCtlImpl) DeleteMemoryInterface() error {
	memifKey := interfaces.InterfaceKey("memif1")

	ctl.Log.Infof("Interface delete: %v", memifKey)
	_, err := ctl.broker.Delete(memifKey)
	return err
}

// PutVxLan puts VxLAN type interface config to the ETCD
func (ctl *VppAgentCtlImpl) PutVxLan() error {
	vxlan := &interfaces.Interface{

		Name:    "vxlan1",
		Type:    interfaces.Interface_VXLAN_TUNNEL,
		Enabled: true,
		IpAddresses: []string{
			"172.125.40.1/24",
		},
		Link: &interfaces.Interface_Vxlan{
			Vxlan: &interfaces.VxlanLink{
				SrcAddress: "192.168.42.1",
				DstAddress: "192.168.42.2",
				Vni:        13,
			},
		},
	}

	ctl.Log.Infof("Interface put: %v", vxlan)
	return ctl.broker.Put(interfaces.InterfaceKey(vxlan.Name), vxlan)
}

// DeleteVxLan removes VxLAN type interface config from the ETCD
func (ctl *VppAgentCtlImpl) DeleteVxLan() error {
	vxlanKey := interfaces.InterfaceKey("vxlan1")

	ctl.Log.Infof("Interface delete: %v", vxlanKey)
	_, err := ctl.broker.Delete(vxlanKey)
	return err
}

// PutAfPacket puts Af-packet type interface config to the ETCD
func (ctl *VppAgentCtlImpl) PutAfPacket() error {
	afPacket := &interfaces.Interface{
		Name:    "afpacket1",
		Type:    interfaces.Interface_AF_PACKET,
		Enabled: true,
		Mtu:     1500,
		IpAddresses: []string{
			"172.125.40.1/24",
			"192.168.12.1/24",
			"fd21:7408:186f::/48",
		},
		Link: &interfaces.Interface_Afpacket{
			Afpacket: &interfaces.AfpacketLink{
				HostIfName: "lo",
			},
		},
	}

	ctl.Log.Infof("Interface put: %v", afPacket)
	return ctl.broker.Put(interfaces.InterfaceKey(afPacket.Name), afPacket)
}

// DeleteAfPacket removes AF-Packet type interface config from the ETCD
func (ctl *VppAgentCtlImpl) DeleteAfPacket() error {
	afPacketKey := interfaces.InterfaceKey("afpacket1")

	ctl.Log.Infof("Interface delete: %v", afPacketKey)
	_, err := ctl.broker.Delete(afPacketKey)
	return err
}

// PutIPSecTunnelInterface configures IPSec tunnel interface
func (ctl *VppAgentCtlImpl) PutIPSecTunnelInterface() error {
	tunnelIf := &interfaces.Interface{
		Name:        "ipsec0",
		Enabled:     true,
		IpAddresses: []string{"20.0.0.0/24"},
		Vrf:         0,
		Type:        interfaces.Interface_IPSEC_TUNNEL,
		Link: &interfaces.Interface_Ipsec{
			Ipsec: &interfaces.IPSecLink{
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
				EnableUdpEncap:  true,
			},
		},
	}
	ctl.Log.Infof("Interface put: %v", tunnelIf)
	return ctl.broker.Put(interfaces.InterfaceKey(tunnelIf.Name), tunnelIf)
}

// DeleteIPSecTunnelInterface removes IPSec tunnel interface
func (ctl *VppAgentCtlImpl) DeleteIPSecTunnelInterface() error {
	tunnelKey := interfaces.InterfaceKey("ipsec0")

	ctl.Log.Infof("Interface delete: %v", tunnelKey)
	_, err := ctl.broker.Delete(tunnelKey)
	return err
}

// PutVEthPair puts two VETH type interfaces to the ETCD
func (ctl *VppAgentCtlImpl) PutVEthPair() error {
	// Note: VETH interfaces are created in pairs
	veth1 := &linuxIf.Interface{
		Name:        "veth1",
		Type:        linuxIf.Interface_VETH,
		Enabled:     true,
		PhysAddress: "D2:74:8C:12:67:D2",
		Namespace: &linux_namespace.NetNamespace{
			Reference: "ns1",
			Type:      linux_namespace.NetNamespace_NSID,
		},
		Mtu: 1500,
		IpAddresses: []string{
			"192.168.22.1/24",
			"10.0.2.2/24",
		},
		Link: &linuxIf.Interface_Veth{
			Veth: &linuxIf.VethLink{
				PeerIfName: "veth2",
			},
		},
	}

	veth2 := &linuxIf.Interface{
		Name:        "veth2",
		Type:        linuxIf.Interface_VETH,
		Enabled:     true,
		PhysAddress: "92:C7:42:67:AB:CD",
		Namespace: &linux_namespace.NetNamespace{
			Reference: "ns2",
			Type:      linux_namespace.NetNamespace_NSID,
		},
		Mtu: 1500,
		IpAddresses: []string{
			"192.168.22.5/24",
		},
		Link: &linuxIf.Interface_Veth{
			Veth: &linuxIf.VethLink{
				PeerIfName: "veth1",
			},
		},
	}

	ctl.Log.Infof("Interfaces put: %v, v%", veth1, veth2)
	if err := ctl.broker.Put(linuxIf.InterfaceKey(veth1.Name), veth1); err != nil {
		return err
	}
	return ctl.broker.Put(linuxIf.InterfaceKey(veth2.Name), veth2)
}

// DeleteVEthPair removes VETH pair interfaces from the ETCD
func (ctl *VppAgentCtlImpl) DeleteVEthPair() error {
	veth1Key := linuxIf.InterfaceKey("veth1")
	veth2Key := linuxIf.InterfaceKey("veth2")

	ctl.Log.Infof("Interface delete: %v", veth1Key)
	if _, err := ctl.broker.Delete(veth1Key); err != nil {
		return err
	}
	ctl.Log.Infof("Interface delete: %v", veth2Key)
	_, err := ctl.broker.Delete(veth2Key)
	return err
}

// PutLinuxTap puts linux TAP type interface configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutLinuxTap() error {
	linuxTap := &linuxIf.Interface{
		Name:        "tap1",
		HostIfName:  "tap-host",
		Type:        linuxIf.Interface_TAP_TO_VPP,
		Enabled:     true,
		PhysAddress: "BC:FE:E9:5E:07:04",
		Namespace: &linux_namespace.NetNamespace{
			Reference: "ns2",
			Type:      linux_namespace.NetNamespace_NSID,
		},
		Mtu: 1500,
		IpAddresses: []string{
			"172.52.45.127/24",
		},
		Link: &linuxIf.Interface_Tap{
			Tap: &linuxIf.TapLink{
				VppTapIfName: "tap-host",
			},
		},
	}

	ctl.Log.Println(linuxTap)
	return ctl.broker.Put(linuxIf.InterfaceKey(linuxTap.Name), linuxTap)
}

// DeleteLinuxTap removes linux TAP type interface configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteLinuxTap() error {
	linuxTapKey := linuxIf.InterfaceKey("tap1")

	ctl.Log.Println("Deleting", linuxTapKey)
	_, err := ctl.broker.Delete(linuxTapKey)
	return err
}
