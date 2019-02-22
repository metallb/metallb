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

package vppclient

import (
	acl "github.com/ligato/vpp-agent/api/models/vpp/acl"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	ipsec "github.com/ligato/vpp-agent/api/models/vpp/ipsec"
	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	nat "github.com/ligato/vpp-agent/api/models/vpp/nat"
	punt "github.com/ligato/vpp-agent/api/models/vpp/punt"
	stn "github.com/ligato/vpp-agent/api/models/vpp/stn"
)

// DataChangeDSL defines Domain Specific Language (DSL) for data change.
// of the VPP configuration.
// Use this interface to make your implementation independent of the local
// and any remote client.
// Every DSL statement (apart from Send) returns the receiver (possibly wrapped
// to change the scope of DSL), allowing the calls to be chained together
// conveniently in a single statement.
type DataChangeDSL interface {
	// Put initiates a chained sequence of data change DSL statements, declaring
	// new or changing existing configurable objects, e.g.:
	//     Put().Interface(&memif).XConnect(&xconnect).BD(&BD) ... Send()
	// The set of available objects to be created or changed is defined by PutDSL.
	Put() PutDSL

	// Delete initiates a chained sequence of data change DSL statements,
	// removing existing configurable objects (by name), e.g.:
	//     Delete().Interface(memifName).XConnect(xconnectName).BD(BDName) ... Send()
	// The set of available objects to be removed is defined by DeleteDSL.
	Delete() DeleteDSL

	// Send propagates requested changes to the plugins.
	Send() Reply
}

// PutDSL is a subset of data change DSL statements, used to declare new
// VPP configuration or to change an existing one.
type PutDSL interface {
	// Interface adds a request to create or update VPP network interface.
	Interface(val *interfaces.Interface) PutDSL
	// ACL adds a request to create or update VPP Access Control List.
	ACL(acl *acl.ACL) PutDSL
	// BD adds a request to create or update VPP Bridge Domain.
	BD(val *l2.BridgeDomain) PutDSL
	// BDFIB adds a request to create or update VPP L2 Forwarding Information Base.
	BDFIB(fib *l2.FIBEntry) PutDSL
	// XConnect adds a request to create or update VPP Cross Connect.
	XConnect(val *l2.XConnectPair) PutDSL
	// StaticRoute adds a request to create or update VPP L3 Static Route.
	StaticRoute(val *l3.Route) PutDSL
	// Arp adds a request to create or update VPP L3 ARP.
	Arp(arp *l3.ARPEntry) PutDSL
	// ProxyArpInterfaces adds a request to create or update VPP L3 proxy ARP interfaces
	ProxyArp(proxyArp *l3.ProxyARP) PutDSL
	// IPScanNeighbor adds L3 IP Scan Neighbor to the RESYNC request.
	IPScanNeighbor(ipScanNeigh *l3.IPScanNeighbor) PutDSL
	// StnRule adds a request to create or update Stn rule.
	StnRule(stn *stn.Rule) PutDSL
	// NAT44Global adds a request to set global configuration for NAT44
	NAT44Global(nat *nat.Nat44Global) PutDSL
	// DNAT44 adds a request to create or update DNAT44 configuration
	DNAT44(dnat *nat.DNat44) PutDSL
	// IPSecSA adds request to create a new Security Association
	IPSecSA(sa *ipsec.SecurityAssociation) PutDSL
	// IPSecSPD adds request to create a new Security Policy Database
	IPSecSPD(spd *ipsec.SecurityPolicyDatabase) PutDSL
	// PuntIPRedirect adds request to create or update rule to punt L3 traffic via interface.
	PuntIPRedirect(val *punt.IPRedirect) PutDSL
	// PuntToHost adds request to create or update rule to punt L4 traffic to a host.
	PuntToHost(val *punt.ToHost) PutDSL

	// Delete changes the DSL mode to allow removal of an existing configuration.
	// See documentation for DataChangeDSL.Delete().
	Delete() DeleteDSL

	// Send propagates requested changes to the plugins.
	Send() Reply
}

// DeleteDSL is a subset of data change DSL statements, used to remove
// an existing VPP configuration.
type DeleteDSL interface {
	// Interface adds a request to delete an existing VPP network interface.
	Interface(ifaceName string) DeleteDSL
	// ACL adds a request to delete an existing VPP Access Control List.
	ACL(aclName string) DeleteDSL
	// BD adds a request to delete an existing VPP Bridge Domain.
	BD(bdName string) DeleteDSL
	// BDFIB adds a request to delete an existing VPP L2 Forwarding Information
	// Base.
	BDFIB(bdName string, mac string) DeleteDSL
	// XConnect adds a request to delete an existing VPP Cross Connect.
	XConnect(rxIfaceName string) DeleteDSL
	// StaticRoute adds a request to delete an existing VPP L3 Static Route.
	StaticRoute(vrf uint32, dstAddr string, nextHopAddr string) DeleteDSL
	// Arp adds a request to delete an existing VPP L3 ARP.
	Arp(ifaceName string, ipAddr string) DeleteDSL
	// ProxyArpInterfaces adds a request to delete an existing VPP L3 proxy ARP interfaces
	ProxyArp() DeleteDSL
	// IPScanNeighbor adds a request to delete an existing VPP L3 IP Scan Neighbor.
	IPScanNeighbor() DeleteDSL
	// StnRule adds a request to delete an existing Stn rule.
	StnRule(iface, addr string) DeleteDSL
	// NAT44Global adds a request to remove global configuration for NAT44
	NAT44Global() DeleteDSL
	// DNAT44 adds a request to delete an existing DNAT44 configuration
	DNAT44(label string) DeleteDSL
	// IPSecSA adds request to delete a Security Association
	IPSecSA(saIndex string) DeleteDSL
	// IPSecSPD adds request to delete a Security Policy Database
	IPSecSPD(spdIndex string) DeleteDSL
	// PuntIPRedirect adds request to delete a rule used to punt L3 traffic via interface.
	PuntIPRedirect(l3Proto punt.L3Protocol, txInterface string) DeleteDSL
	// PuntToHost adds request to delete a rule used to punt L4 traffic to a host.
	PuntToHost(l3Proto punt.L3Protocol, l4Proto punt.L4Protocol, port uint32) DeleteDSL

	// Put changes the DSL mode to allow configuration editing.
	// See documentation for DataChangeDSL.Put().
	Put() PutDSL

	// Send propagates requested changes to the plugins.
	Send() Reply
}

// Reply interface allows to wait for a reply to previously called Send() and
// extract the result from it (success/error).
type Reply interface {
	// ReceiveReply waits for a reply to previously called Send() and returns
	// the result (error or nil).
	ReceiveReply() error
}
