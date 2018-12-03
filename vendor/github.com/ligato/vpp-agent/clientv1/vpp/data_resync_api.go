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
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/model/bfd"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/ipsec"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l4"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
	"github.com/ligato/vpp-agent/plugins/vpp/model/stn"
)

// DataResyncDSL defines the Domain Specific Language (DSL) for data RESYNC
// of the VPP configuration.
// Use this interface to make your implementation independent of the local
// and any remote client.
// Each method (apart from Send) returns the receiver, allowing the calls
// to be chained together conveniently in a single statement.
type DataResyncDSL interface {
	// Interface adds interface to the RESYNC request.
	Interface(intf *interfaces.Interfaces_Interface) DataResyncDSL
	// BfdSession adds bidirectional forwarding detection session to the RESYNC
	// request.
	BfdSession(val *bfd.SingleHopBFD_Session) DataResyncDSL
	// BfdAuthKeys adds bidirectional forwarding detection key to the RESYNC
	// request.
	BfdAuthKeys(val *bfd.SingleHopBFD_Key) DataResyncDSL
	// BfdEchoFunction adds bidirectional forwarding detection echo function
	// to the RESYNC request.
	BfdEchoFunction(val *bfd.SingleHopBFD_EchoFunction) DataResyncDSL
	// BD adds Bridge Domain to the RESYNC request.
	BD(bd *l2.BridgeDomains_BridgeDomain) DataResyncDSL
	// BDFIB adds L2 Forwarding Information Base.
	BDFIB(fib *l2.FibTable_FibEntry) DataResyncDSL
	// XConnect adds Cross Connect to the RESYNC request.
	XConnect(xcon *l2.XConnectPairs_XConnectPair) DataResyncDSL
	// StaticRoute adds L3 Static Route to the RESYNC request.
	StaticRoute(staticRoute *l3.StaticRoutes_Route) DataResyncDSL
	// ACL adds Access Control List to the RESYNC request.
	ACL(acl *acl.AccessLists_Acl) DataResyncDSL
	// Arp adds VPP L3 ARP to the RESYNC request.
	Arp(arp *l3.ArpTable_ArpEntry) DataResyncDSL
	// ProxyArpInterfaces adds L3 proxy ARP interfaces to the RESYNC request.
	ProxyArpInterfaces(pArpIfs *l3.ProxyArpInterfaces_InterfaceList) DataResyncDSL
	// ProxyArpRanges adds L3 proxy ARP ranges to the RESYNC request.
	ProxyArpRanges(pArpRng *l3.ProxyArpRanges_RangeList) DataResyncDSL
	// L4Features adds L4 features to the RESYNC request
	L4Features(val *l4.L4Features) DataResyncDSL
	// AppNamespace adds VPP Application namespaces to the RESYNC request
	AppNamespace(appNs *l4.AppNamespaces_AppNamespace) DataResyncDSL
	// StnRule adds Stn rule to the RESYNC request.
	StnRule(stn *stn.STN_Rule) DataResyncDSL
	// NAT44Global adds a request to RESYNC global configuration for NAT44
	NAT44Global(nat *nat.Nat44Global) DataResyncDSL
	// NAT44DNat adds a request to RESYNC a new DNAT configuration
	NAT44DNat(dnat *nat.Nat44DNat_DNatConfig) DataResyncDSL
	// IPSecSA adds request to RESYNC a new Security Association
	IPSecSA(sa *ipsec.SecurityAssociations_SA) DataResyncDSL
	// IPSecSPD adds request to RESYNC a new Security Policy Database
	IPSecSPD(spd *ipsec.SecurityPolicyDatabases_SPD) DataResyncDSL

	// Send propagates the RESYNC request to the plugins.
	Send() Reply
}
