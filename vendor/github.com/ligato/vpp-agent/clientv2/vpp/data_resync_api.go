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

// DataResyncDSL defines the Domain Specific Language (DSL) for data RESYNC
// of the VPP configuration.
// Use this interface to make your implementation independent of the local
// and any remote client.
// Each method (apart from Send) returns the receiver, allowing the calls
// to be chained together conveniently in a single statement.
type DataResyncDSL interface {
	// Interface adds interface to the RESYNC request.
	Interface(intf *interfaces.Interface) DataResyncDSL
	// ACL adds Access Control List to the RESYNC request.
	ACL(acl *acl.ACL) DataResyncDSL
	// BD adds Bridge Domain to the RESYNC request.
	BD(bd *l2.BridgeDomain) DataResyncDSL
	// BDFIB adds L2 Forwarding Information Base.
	BDFIB(fib *l2.FIBEntry) DataResyncDSL
	// XConnect adds Cross Connect to the RESYNC request.
	XConnect(xcon *l2.XConnectPair) DataResyncDSL
	// StaticRoute adds L3 Static Route to the RESYNC request.
	StaticRoute(staticRoute *l3.Route) DataResyncDSL
	// Arp adds VPP L3 ARP to the RESYNC request.
	Arp(arp *l3.ARPEntry) DataResyncDSL
	// ProxyArp adds L3 proxy ARP interfaces to the RESYNC request.
	ProxyArp(proxyArp *l3.ProxyARP) DataResyncDSL
	// IPScanNeighbor adds L3 IP Scan Neighbor to the RESYNC request.
	IPScanNeighbor(ipScanNeigh *l3.IPScanNeighbor) DataResyncDSL
	// StnRule adds Stn rule to the RESYNC request.
	StnRule(stn *stn.Rule) DataResyncDSL
	// NAT44Global adds global NAT44 configuration to the RESYNC request.
	NAT44Global(nat *nat.Nat44Global) DataResyncDSL
	// DNAT44 adds DNAT44 configuration to the RESYNC request
	DNAT44(dnat *nat.DNat44) DataResyncDSL
	// IPSecSA adds request to RESYNC a new Security Association
	IPSecSA(sa *ipsec.SecurityAssociation) DataResyncDSL
	// IPSecSPD adds request to RESYNC a new Security Policy Database
	IPSecSPD(spd *ipsec.SecurityPolicyDatabase) DataResyncDSL
	// PuntIPRedirect adds request to RESYNC a rule used to punt L3 traffic via interface.
	PuntIPRedirect(val *punt.IPRedirect) DataResyncDSL
	// PuntToHost adds request to RESYNC a rule used to punt L4 traffic to a host.
	PuntToHost(val *punt.ToHost) DataResyncDSL

	// Send propagates the RESYNC request to the plugins.
	Send() Reply
}
