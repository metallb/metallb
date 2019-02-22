// Copyright (c) 2017 Cisco and/or its affiliates.
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
	"github.com/ligato/vpp-agent/pkg/models"

	acl "github.com/ligato/vpp-agent/api/models/vpp/acl"
	intf "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	ipsec "github.com/ligato/vpp-agent/api/models/vpp/ipsec"
	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	nat "github.com/ligato/vpp-agent/api/models/vpp/nat"
	punt "github.com/ligato/vpp-agent/api/models/vpp/punt"
	stn "github.com/ligato/vpp-agent/api/models/vpp/stn"
	"github.com/ligato/vpp-agent/clientv2/vpp"
)

// NewDataChangeDSL returns a new instance of DataChangeDSL which implements
// the data change DSL for VPP config.
// Transaction <txn> is used to propagate changes to plugins.
func NewDataChangeDSL(txn keyval.ProtoTxn) *DataChangeDSL {
	return &DataChangeDSL{txn: txn}
}

// DataChangeDSL is an implementation of Domain Specific Language (DSL)
// for changes of the VPP configuration.
type DataChangeDSL struct {
	txn keyval.ProtoTxn
}

// PutDSL implements put operations of data change DSL.
type PutDSL struct {
	parent *DataChangeDSL
}

// DeleteDSL implements delete operations of data change DSL.
type DeleteDSL struct {
	parent *DataChangeDSL
}

// Put initiates a chained sequence of data change DSL statements declaring
// new configurable objects or changing existing ones.
func (dsl *DataChangeDSL) Put() vppclient.PutDSL {
	return &PutDSL{dsl}
}

// Delete initiates a chained sequence of data change DSL statements
// removing existing configurable objects.
func (dsl *DataChangeDSL) Delete() vppclient.DeleteDSL {
	return &DeleteDSL{dsl}
}

// Send propagates requested changes to the plugins.
func (dsl *DataChangeDSL) Send() vppclient.Reply {
	err := dsl.txn.Commit()
	return &Reply{err}
}

// Interface adds a request to create or update VPP network interface.
func (dsl *PutDSL) Interface(val *intf.Interface) vppclient.PutDSL {
	dsl.parent.txn.Put(intf.InterfaceKey(val.Name), val)
	return dsl
}

// ACL adds a request to create or update VPP Access Control List.
func (dsl *PutDSL) ACL(val *acl.ACL) vppclient.PutDSL {
	dsl.parent.txn.Put(acl.Key(val.Name), val)
	return dsl
}

// BD adds a request to create or update VPP Bridge Domain.
func (dsl *PutDSL) BD(val *l2.BridgeDomain) vppclient.PutDSL {
	dsl.parent.txn.Put(l2.BridgeDomainKey(val.Name), val)
	return dsl
}

// BDFIB adds a request to create or update VPP L2 Forwarding Information Base.
func (dsl *PutDSL) BDFIB(val *l2.FIBEntry) vppclient.PutDSL {
	dsl.parent.txn.Put(l2.FIBKey(val.BridgeDomain, val.PhysAddress), val)
	return dsl
}

// XConnect adds a request to create or update VPP Cross Connect.
func (dsl *PutDSL) XConnect(val *l2.XConnectPair) vppclient.PutDSL {
	dsl.parent.txn.Put(l2.XConnectKey(val.ReceiveInterface), val)
	return dsl
}

// StaticRoute adds a request to create or update VPP L3 Static Route.
func (dsl *PutDSL) StaticRoute(val *l3.Route) vppclient.PutDSL {
	dsl.parent.txn.Put(l3.RouteKey(val.VrfId, val.DstNetwork, val.NextHopAddr), val)
	return dsl
}

// Arp adds a request to create or update VPP L3 ARP entry.
func (dsl *PutDSL) Arp(arp *l3.ARPEntry) vppclient.PutDSL {
	dsl.parent.txn.Put(l3.ArpEntryKey(arp.Interface, arp.IpAddress), arp)
	return dsl
}

// ProxyArp adds a request to create or update VPP L3 proxy ARP.
func (dsl *PutDSL) ProxyArp(proxyArp *l3.ProxyARP) vppclient.PutDSL {
	dsl.parent.txn.Put(models.Key(&l3.ProxyARP{}), proxyArp)
	return dsl
}

// IPScanNeighbor adds L3 IP Scan Neighbor to the RESYNC request.
func (dsl *PutDSL) IPScanNeighbor(ipScanNeigh *l3.IPScanNeighbor) vppclient.PutDSL {
	dsl.parent.txn.Put(models.Key(&l3.IPScanNeighbor{}), ipScanNeigh)
	return dsl
}

// StnRule adds a request to create or update STN rule.
func (dsl *PutDSL) StnRule(val *stn.Rule) vppclient.PutDSL {
	dsl.parent.txn.Put(stn.Key(val.Interface, val.IpAddress), val)
	return dsl
}

// NAT44Global adds a request to set global configuration for NAT44
func (dsl *PutDSL) NAT44Global(nat44 *nat.Nat44Global) vppclient.PutDSL {
	dsl.parent.txn.Put(models.Key(&nat.Nat44Global{}), nat44)
	return dsl
}

// DNAT44 adds a request to create or update DNAT44 configuration
func (dsl *PutDSL) DNAT44(nat44 *nat.DNat44) vppclient.PutDSL {
	dsl.parent.txn.Put(nat.DNAT44Key(nat44.Label), nat44)
	return dsl
}

// IPSecSA adds request to create a new Security Association
func (dsl *PutDSL) IPSecSA(sa *ipsec.SecurityAssociation) vppclient.PutDSL {
	dsl.parent.txn.Put(ipsec.SAKey(sa.Index), sa)
	return dsl
}

// IPSecSPD adds request to create a new Security Policy Database
func (dsl *PutDSL) IPSecSPD(spd *ipsec.SecurityPolicyDatabase) vppclient.PutDSL {
	dsl.parent.txn.Put(ipsec.SPDKey(spd.Index), spd)
	return dsl
}

// PuntIPRedirect adds request to create or update rule to punt L3 traffic via interface.
func (dsl *PutDSL) PuntIPRedirect(val *punt.IPRedirect) vppclient.PutDSL {
	dsl.parent.txn.Put(punt.IPRedirectKey(val.L3Protocol, val.TxInterface), val)
	return dsl
}

// PuntToHost adds request to create or update rule to punt L4 traffic to a host.
func (dsl *PutDSL) PuntToHost(val *punt.ToHost) vppclient.PutDSL {
	dsl.parent.txn.Put(punt.ToHostKey(val.L3Protocol, val.L4Protocol, val.Port), val)
	return dsl
}

// Delete changes the DSL mode to allow removal of an existing configuration.
func (dsl *PutDSL) Delete() vppclient.DeleteDSL {
	return &DeleteDSL{dsl.parent}
}

// Send propagates requested changes to the plugins.
func (dsl *PutDSL) Send() vppclient.Reply {
	return dsl.parent.Send()
}

// Interface adds a request to delete an existing VPP network interface.
func (dsl *DeleteDSL) Interface(interfaceName string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(intf.InterfaceKey(interfaceName))
	return dsl
}

// ACL adds a request to delete an existing VPP Access Control List.
func (dsl *DeleteDSL) ACL(aclName string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(acl.Key(aclName))
	return dsl
}

// BD adds a request to delete an existing VPP Bridge Domain.
func (dsl *DeleteDSL) BD(bdName string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(l2.BridgeDomainKey(bdName))
	return dsl
}

// BDFIB adds a request to delete an existing VPP L2 Forwarding Information
// Base.
func (dsl *DeleteDSL) BDFIB(bdName string, mac string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(l2.FIBKey(bdName, mac))
	return dsl
}

// XConnect adds a request to delete an existing VPP Cross Connect.
func (dsl *DeleteDSL) XConnect(rxIfName string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(l2.XConnectKey(rxIfName))
	return dsl
}

// StaticRoute adds a request to delete an existing VPP L3 Static Route.
func (dsl *DeleteDSL) StaticRoute(vrf uint32, dstAddr string, nextHopAddr string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(l3.RouteKey(vrf, dstAddr, nextHopAddr))
	return dsl
}

// Arp adds a request to delete an existing VPP L3 ARP entry.
func (dsl *DeleteDSL) Arp(ifaceName string, ipAddr string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(l3.ArpEntryKey(ifaceName, ipAddr))
	return dsl
}

// ProxyArp adds a request to delete an existing VPP L3 proxy ARP.
func (dsl *DeleteDSL) ProxyArp() vppclient.DeleteDSL {
	dsl.parent.txn.Delete(models.Key(&l3.ProxyARP{}))
	return dsl
}

// IPScanNeighbor adds a request to delete an existing VPP L3 IP Scan Neighbor.
func (dsl *DeleteDSL) IPScanNeighbor() vppclient.DeleteDSL {
	dsl.parent.txn.Delete(models.Key(&l3.IPScanNeighbor{}))
	return dsl
}

// StnRule adds request to delete Stn rule.
func (dsl *DeleteDSL) StnRule(iface, addr string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(stn.Key(iface, addr))
	return dsl
}

// NAT44Global adds a request to remove global configuration for NAT44
func (dsl *DeleteDSL) NAT44Global() vppclient.DeleteDSL {
	dsl.parent.txn.Delete(models.Key(&nat.Nat44Global{}))
	return dsl
}

// DNAT44 adds a request to delete an existing DNAT44 configuration
func (dsl *DeleteDSL) DNAT44(label string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(nat.DNAT44Key(label))
	return dsl
}

// IPSecSA adds request to create a new Security Association
func (dsl *DeleteDSL) IPSecSA(saIndex string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(ipsec.SAKey(saIndex))
	return dsl
}

// IPSecSPD adds request to create a new Security Policy Database
func (dsl *DeleteDSL) IPSecSPD(spdIndex string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(ipsec.SPDKey(spdIndex))
	return dsl
}

// PuntIPRedirect adds request to delete a rule used to punt L3 traffic via interface.
func (dsl *DeleteDSL) PuntIPRedirect(l3Proto punt.L3Protocol, txInterface string) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(punt.IPRedirectKey(l3Proto, txInterface))
	return dsl
}

// PuntToHost adds request to delete a rule used to punt L4 traffic to a host.
func (dsl *DeleteDSL) PuntToHost(l3Proto punt.L3Protocol, l4Proto punt.L4Protocol, port uint32) vppclient.DeleteDSL {
	dsl.parent.txn.Delete(punt.ToHostKey(l3Proto, l4Proto, port))
	return dsl
}

// Put changes the DSL mode to allow configuration editing.
func (dsl *DeleteDSL) Put() vppclient.PutDSL {
	return &PutDSL{dsl.parent}
}

// Send propagates requested changes to the plugins.
func (dsl *DeleteDSL) Send() vppclient.Reply {
	return dsl.parent.Send()
}

// Reply interface allows to wait for a reply to previously called Send() and
// extract the result from it (success/error).
type Reply struct {
	err error
}

// ReceiveReply waits for a reply to previously called Send() and returns
// the result (error or nil).
func (dsl Reply) ReceiveReply() error {
	return dsl.err
}
