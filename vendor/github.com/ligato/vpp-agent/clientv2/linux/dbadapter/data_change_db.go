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

package dbadapter

import (
	"github.com/ligato/cn-infra/db/keyval"

	"github.com/ligato/vpp-agent/api/models/linux/interfaces"
	"github.com/ligato/vpp-agent/api/models/linux/l3"
	acl "github.com/ligato/vpp-agent/api/models/vpp/acl"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	ipsec "github.com/ligato/vpp-agent/api/models/vpp/ipsec"
	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	nat "github.com/ligato/vpp-agent/api/models/vpp/nat"
	punt "github.com/ligato/vpp-agent/api/models/vpp/punt"
	stn "github.com/ligato/vpp-agent/api/models/vpp/stn"
	"github.com/ligato/vpp-agent/clientv2/linux"
	"github.com/ligato/vpp-agent/clientv2/vpp"
	"github.com/ligato/vpp-agent/clientv2/vpp/dbadapter"
)

// NewDataChangeDSL returns a new instance of DataChangeDSL which implements
// the data change DSL for both Linux and VPP config (inherits dbadapter
// from vppplugin).
// Transaction <txn> is used to propagate changes to plugins.
func NewDataChangeDSL(txn keyval.ProtoTxn) *DataChangeDSL {
	vppDbAdapter := dbadapter.NewDataChangeDSL(txn)
	return &DataChangeDSL{txn: txn, vppDataChange: vppDbAdapter}
}

// DataChangeDSL is an implementation of Domain Specific Language (DSL)
// for changes of both Linux and VPP configuration.
type DataChangeDSL struct {
	txn           keyval.ProtoTxn
	vppDataChange vppclient.DataChangeDSL
}

// PutDSL implements put operations of data change DSL.
type PutDSL struct {
	parent *DataChangeDSL
	vppPut vppclient.PutDSL
}

// DeleteDSL implements delete operations of data change DSL.
type DeleteDSL struct {
	parent    *DataChangeDSL
	vppDelete vppclient.DeleteDSL
}

// Put initiates a chained sequence of data change DSL statements and declares
// new configurable objects or changes existing ones.
func (dsl *DataChangeDSL) Put() linuxclient.PutDSL {
	return &PutDSL{dsl, dsl.vppDataChange.Put()}
}

// Delete initiates a chained sequence of data change DSL statements
// removing existing configurable objects.
func (dsl *DataChangeDSL) Delete() linuxclient.DeleteDSL {
	return &DeleteDSL{dsl, dsl.vppDataChange.Delete()}
}

// Send propagates requested changes to the plugins.
func (dsl *DataChangeDSL) Send() vppclient.Reply {
	return dsl.vppDataChange.Send()
}

// LinuxInterface adds a request to create or update Linux network interface.
func (dsl *PutDSL) LinuxInterface(val *linux_interfaces.Interface) linuxclient.PutDSL {
	dsl.parent.txn.Put(linux_interfaces.InterfaceKey(val.Name), val)
	return dsl
}

// LinuxArpEntry adds a request to create or update Linux ARP entry.
func (dsl *PutDSL) LinuxArpEntry(val *linux_l3.ARPEntry) linuxclient.PutDSL {
	dsl.parent.txn.Put(linux_l3.ArpKey(val.Interface, val.IpAddress), val)
	return dsl
}

// LinuxRoute adds a request to create or update Linux route.
func (dsl *PutDSL) LinuxRoute(val *linux_l3.Route) linuxclient.PutDSL {
	dsl.parent.txn.Put(linux_l3.RouteKey(val.DstNetwork, val.OutgoingInterface), val)
	return dsl
}

// VppInterface adds a request to create or update VPP network interface.
func (dsl *PutDSL) VppInterface(val *interfaces.Interface) linuxclient.PutDSL {
	dsl.vppPut.Interface(val)
	return dsl
}

// ACL adds a request to create or update VPP Access Control List.
func (dsl *PutDSL) ACL(acl *acl.ACL) linuxclient.PutDSL {
	dsl.vppPut.ACL(acl)
	return dsl
}

/*// BfdSession adds a request to create or update VPP bidirectional forwarding
// detection session.
func (dsl *PutDSL) BfdSession(val *bfd.SingleHopBFD_Session) linuxclient.PutDSL {
	dsl.vppPut.BfdSession(val)
	return dsl
}

// BfdAuthKeys adds a request to create or update VPP bidirectional forwarding
// detection key.
func (dsl *PutDSL) BfdAuthKeys(val *bfd.SingleHopBFD_Key) linuxclient.PutDSL {
	dsl.vppPut.BfdAuthKeys(val)
	return dsl
}

// BfdEchoFunction adds a request to create or update VPP bidirectional forwarding
// detection echo function.
func (dsl *PutDSL) BfdEchoFunction(val *bfd.SingleHopBFD_EchoFunction) linuxclient.PutDSL {
	dsl.vppPut.BfdEchoFunction(val)
	return dsl
}*/

// BD adds a request to create or update VPP Bridge Domain.
func (dsl *PutDSL) BD(val *l2.BridgeDomain) linuxclient.PutDSL {
	dsl.vppPut.BD(val)
	return dsl
}

// BDFIB adds a request to create or update VPP L2 Forwarding Information Base.
func (dsl *PutDSL) BDFIB(fib *l2.FIBEntry) linuxclient.PutDSL {
	dsl.vppPut.BDFIB(fib)
	return dsl
}

// XConnect adds a request to create or update VPP Cross Connect.
func (dsl *PutDSL) XConnect(val *l2.XConnectPair) linuxclient.PutDSL {
	dsl.vppPut.XConnect(val)
	return dsl
}

// StaticRoute adds a request to create or update VPP L3 Static Route.
func (dsl *PutDSL) StaticRoute(val *l3.Route) linuxclient.PutDSL {
	dsl.vppPut.StaticRoute(val)
	return dsl
}

// Arp adds a request to create or update VPP L3 ARP.
func (dsl *PutDSL) Arp(arp *l3.ARPEntry) linuxclient.PutDSL {
	dsl.vppPut.Arp(arp)
	return dsl
}

// ProxyArp adds a request to create or update VPP L3 proxy ARP.
func (dsl *PutDSL) ProxyArp(proxyArp *l3.ProxyARP) linuxclient.PutDSL {
	dsl.vppPut.ProxyArp(proxyArp)
	return dsl
}

// IPScanNeighbor adds a request to delete an existing VPP L3 IP Scan Neighbor.
func (dsl *PutDSL) IPScanNeighbor(ipScanNeigh *l3.IPScanNeighbor) linuxclient.PutDSL {
	dsl.vppPut.IPScanNeighbor(ipScanNeigh)
	return dsl
}

/*// L4Features adds a request to enable or disable L4 features
func (dsl *PutDSL) L4Features(val *l4.L4Features) linuxclient.PutDSL {
	dsl.vppPut.L4Features(val)
	return dsl
}

// AppNamespace adds a request to create or update VPP Application namespace
func (dsl *PutDSL) AppNamespace(appNs *l4.AppNamespaces_AppNamespace) linuxclient.PutDSL {
	dsl.vppPut.AppNamespace(appNs)
	return dsl
}*/

// StnRule adds a request to create or update VPP Stn rule.
func (dsl *PutDSL) StnRule(stn *stn.Rule) linuxclient.PutDSL {
	dsl.vppPut.StnRule(stn)
	return dsl
}

// NAT44Global adds a request to set global configuration for NAT44
func (dsl *PutDSL) NAT44Global(nat44 *nat.Nat44Global) linuxclient.PutDSL {
	dsl.vppPut.NAT44Global(nat44)
	return dsl
}

// DNAT44 adds a request to create or update DNAT44 configuration
func (dsl *PutDSL) DNAT44(nat44 *nat.DNat44) linuxclient.PutDSL {
	dsl.vppPut.DNAT44(nat44)
	return dsl
}

// IPSecSA adds request to create a new Security Association
func (dsl *PutDSL) IPSecSA(sa *ipsec.SecurityAssociation) linuxclient.PutDSL {
	dsl.vppPut.IPSecSA(sa)
	return dsl
}

// IPSecSPD adds request to create a new Security Policy Database
func (dsl *PutDSL) IPSecSPD(spd *ipsec.SecurityPolicyDatabase) linuxclient.PutDSL {
	dsl.vppPut.IPSecSPD(spd)
	return dsl
}

// PuntIPRedirect adds request to create or update rule to punt L3 traffic via interface.
func (dsl *PutDSL) PuntIPRedirect(val *punt.IPRedirect) linuxclient.PutDSL {
	dsl.vppPut.PuntIPRedirect(val)
	return dsl
}

// PuntToHost adds request to create or update rule to punt L4 traffic to a host.
func (dsl *PutDSL) PuntToHost(val *punt.ToHost) linuxclient.PutDSL {
	dsl.vppPut.PuntToHost(val)
	return dsl
}

// Delete changes the DSL mode to allow removal of an existing configuration.
func (dsl *PutDSL) Delete() linuxclient.DeleteDSL {
	return &DeleteDSL{dsl.parent, dsl.vppPut.Delete()}
}

// Send propagates requested changes to the plugins.
func (dsl *PutDSL) Send() vppclient.Reply {
	return dsl.parent.Send()
}

// LinuxInterface adds a request to delete an existing Linux network
// interface.
func (dsl *DeleteDSL) LinuxInterface(interfaceName string) linuxclient.DeleteDSL {
	dsl.parent.txn.Delete(linux_interfaces.InterfaceKey(interfaceName))
	return dsl
}

// LinuxArpEntry adds a request to delete Linux ARP entry.
func (dsl *DeleteDSL) LinuxArpEntry(ifaceName string, ipAddr string) linuxclient.DeleteDSL {
	dsl.parent.txn.Delete(linux_l3.ArpKey(ifaceName, ipAddr))
	return dsl
}

// LinuxRoute adds a request to delete Linux route.
func (dsl *DeleteDSL) LinuxRoute(dstAddr, outIfaceName string) linuxclient.DeleteDSL {
	dsl.parent.txn.Delete(linux_l3.RouteKey(dstAddr, outIfaceName))
	return dsl
}

// VppInterface adds a request to delete an existing VPP network interface.
func (dsl *DeleteDSL) VppInterface(ifaceName string) linuxclient.DeleteDSL {
	dsl.vppDelete.Interface(ifaceName)
	return dsl
}

// ACL adds a request to delete an existing VPP Access Control List.
func (dsl *DeleteDSL) ACL(aclName string) linuxclient.DeleteDSL {
	dsl.vppDelete.ACL(aclName)
	return dsl
}

/*// BfdSession adds a request to delete an existing VPP bidirectional forwarding
// detection session.
func (dsl *DeleteDSL) BfdSession(bfdSessionIfaceName string) linuxclient.DeleteDSL {
	dsl.vppDelete.BfdSession(bfdSessionIfaceName)
	return dsl
}

// BfdAuthKeys adds a request to delete an existing VPP bidirectional forwarding
// detection key.
func (dsl *DeleteDSL) BfdAuthKeys(bfdKey string) linuxclient.DeleteDSL {
	dsl.vppDelete.BfdAuthKeys(bfdKey)
	return dsl
}

// BfdEchoFunction adds a request to delete an existing VPP bidirectional
// forwarding detection echo function.
func (dsl *DeleteDSL) BfdEchoFunction(bfdEchoName string) linuxclient.DeleteDSL {
	dsl.vppDelete.BfdEchoFunction(bfdEchoName)
	return dsl
}*/

// BD adds a request to delete an existing VPP Bridge Domain.
func (dsl *DeleteDSL) BD(bdName string) linuxclient.DeleteDSL {
	dsl.vppDelete.BD(bdName)
	return dsl
}

// BDFIB adds a request to delete an existing VPP L2 Forwarding Information Base.
func (dsl *DeleteDSL) BDFIB(bdName string, mac string) linuxclient.DeleteDSL {
	dsl.vppDelete.BDFIB(bdName, mac)
	return dsl
}

// XConnect adds a request to delete an existing VPP Cross Connect.
func (dsl *DeleteDSL) XConnect(rxIfaceName string) linuxclient.DeleteDSL {
	dsl.vppDelete.XConnect(rxIfaceName)
	return dsl
}

// StaticRoute adds a request to delete an existing VPP L3 Static Route.
func (dsl *DeleteDSL) StaticRoute(vrf uint32, dstAddr string, nextHopAddr string) linuxclient.DeleteDSL {
	dsl.vppDelete.StaticRoute(vrf, dstAddr, nextHopAddr)
	return dsl
}

// IPScanNeighbor adds a request to delete an existing VPP L3 IP Scan Neighbor.
func (dsl *DeleteDSL) IPScanNeighbor() linuxclient.DeleteDSL {
	dsl.vppDelete.IPScanNeighbor()
	return dsl
}

// Arp adds a request to delete an existing VPP L3 ARP.
func (dsl *DeleteDSL) Arp(ifaceName string, ipAddr string) linuxclient.DeleteDSL {
	dsl.vppDelete.Arp(ifaceName, ipAddr)
	return dsl
}

// ProxyArp adds a request to delete an existing VPP L3 proxy ARP.
func (dsl *DeleteDSL) ProxyArp() linuxclient.DeleteDSL {
	dsl.vppDelete.ProxyArp()
	return dsl
}

/*// L4Features adds a request to enable or disable L4 features
func (dsl *DeleteDSL) L4Features() linuxclient.DeleteDSL {
	dsl.vppDelete.L4Features()
	return dsl
}

// AppNamespace adds a request to delete VPP Application namespace
// Note: current version does not support application namespace deletion
func (dsl *DeleteDSL) AppNamespace(id string) linuxclient.DeleteDSL {
	dsl.vppDelete.AppNamespace(id)
	return dsl
}*/

// StnRule adds a request to delete an existing VPP Stn rule.
func (dsl *DeleteDSL) StnRule(iface, addr string) linuxclient.DeleteDSL {
	dsl.vppDelete.StnRule(iface, addr)
	return dsl
}

// NAT44Global adds a request to remove global configuration for NAT44
func (dsl *DeleteDSL) NAT44Global() linuxclient.DeleteDSL {
	dsl.vppDelete.NAT44Global()
	return dsl
}

// DNAT44 adds a request to delete an existing DNAT44 configuration
func (dsl *DeleteDSL) DNAT44(label string) linuxclient.DeleteDSL {
	dsl.vppDelete.DNAT44(label)
	return dsl
}

// IPSecSA adds request to delete a Security Association
func (dsl *DeleteDSL) IPSecSA(saIndex string) linuxclient.DeleteDSL {
	dsl.vppDelete.IPSecSA(saIndex)
	return dsl
}

// IPSecSPD adds request to delete a Security Policy Database
func (dsl *DeleteDSL) IPSecSPD(spdIndex string) linuxclient.DeleteDSL {
	dsl.vppDelete.IPSecSPD(spdIndex)
	return dsl
}

// PuntIPRedirect adds request to delete a rule used to punt L3 traffic via interface.
func (dsl *DeleteDSL) PuntIPRedirect(l3Proto punt.L3Protocol, txInterface string) linuxclient.DeleteDSL {
	dsl.vppDelete.PuntIPRedirect(l3Proto, txInterface)
	return dsl
}

// PuntToHost adds request to delete a rule used to punt L4 traffic to a host.
func (dsl *DeleteDSL) PuntToHost(l3Proto punt.L3Protocol, l4Proto punt.L4Protocol, port uint32) linuxclient.DeleteDSL {
	dsl.vppDelete.PuntToHost(l3Proto, l4Proto, port)
	return dsl
}

// Put changes the DSL mode to allow configuration editing.
func (dsl *DeleteDSL) Put() linuxclient.PutDSL {
	return &PutDSL{dsl.parent, dsl.vppDelete.Put()}
}

// Send propagates requested changes to the plugins.
func (dsl *DeleteDSL) Send() vppclient.Reply {
	return dsl.parent.Send()
}
