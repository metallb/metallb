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

package grpcadapter

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/vpp-agent/clientv1/vpp"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/model/bfd"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/ipsec"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l4"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
	"github.com/ligato/vpp-agent/plugins/vpp/model/rpc"
	"github.com/ligato/vpp-agent/plugins/vpp/model/stn"
	"golang.org/x/net/context"
)

// NewDataResyncDSL is a constructor.
func NewDataResyncDSL(client rpc.DataResyncServiceClient) *DataResyncDSL {
	return &DataResyncDSL{client, make([]proto.Message, 0)}
}

// DataResyncDSL is used to conveniently assign all the data that are needed for the RESYNC.
// This is implementation of Domain Specific Language (DSL) for data RESYNC of the VPP configuration.
type DataResyncDSL struct {
	client rpc.DataResyncServiceClient
	put    []proto.Message
}

// Interface adds Bridge Domain to the RESYNC request.
func (dsl *DataResyncDSL) Interface(val *interfaces.Interfaces_Interface) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// BfdSession adds BFD session to the RESYNC request.
func (dsl *DataResyncDSL) BfdSession(val *bfd.SingleHopBFD_Session) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// BfdAuthKeys adds BFD key to the RESYNC request.
func (dsl *DataResyncDSL) BfdAuthKeys(val *bfd.SingleHopBFD_Key) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// BfdEchoFunction adds BFD echo function to the RESYNC request.
func (dsl *DataResyncDSL) BfdEchoFunction(val *bfd.SingleHopBFD_EchoFunction) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// BD adds Bridge Domain to the RESYNC request.
func (dsl *DataResyncDSL) BD(val *l2.BridgeDomains_BridgeDomain) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// BDFIB adds Bridge Domain to the RESYNC request.
func (dsl *DataResyncDSL) BDFIB(val *l2.FibTable_FibEntry) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// XConnect adds Cross Connect to the RESYNC request.
func (dsl *DataResyncDSL) XConnect(val *l2.XConnectPairs_XConnectPair) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// StaticRoute adds L3 Static Route to the RESYNC request.
func (dsl *DataResyncDSL) StaticRoute(val *l3.StaticRoutes_Route) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// ACL adds Access Control List to the RESYNC request.
func (dsl *DataResyncDSL) ACL(val *acl.AccessLists_Acl) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// L4Features adds L4Features to the RESYNC request.
func (dsl *DataResyncDSL) L4Features(val *l4.L4Features) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// AppNamespace adds Application Namespace to the RESYNC request.
func (dsl *DataResyncDSL) AppNamespace(val *l4.AppNamespaces_AppNamespace) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// Arp adds VPP L3 ARP to the RESYNC request.
func (dsl *DataResyncDSL) Arp(val *l3.ArpTable_ArpEntry) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// ProxyArpInterfaces adds L3 proxy ARP interfaces to the RESYNC request.
func (dsl *DataResyncDSL) ProxyArpInterfaces(val *l3.ProxyArpInterfaces_InterfaceList) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// ProxyArpRanges adds L3 proxy ARP ranges to the RESYNC request.
func (dsl *DataResyncDSL) ProxyArpRanges(val *l3.ProxyArpRanges_RangeList) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// StnRule adds Stn rule to the RESYNC request.
func (dsl *DataResyncDSL) StnRule(val *stn.STN_Rule) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// NAT44Global adds a request to RESYNC global configuration for NAT44
func (dsl *DataResyncDSL) NAT44Global(val *nat.Nat44Global) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// NAT44DNat adds a request to RESYNC a new DNAT configuration
func (dsl *DataResyncDSL) NAT44DNat(val *nat.Nat44DNat_DNatConfig) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// IPSecSA adds request to create a new Security Association
func (dsl *DataResyncDSL) IPSecSA(val *ipsec.SecurityAssociations_SA) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// IPSecSPD adds request to create a new Security Policy Database
func (dsl *DataResyncDSL) IPSecSPD(val *ipsec.SecurityPolicyDatabases_SPD) vppclient.DataResyncDSL {
	dsl.put = append(dsl.put, val)
	return dsl
}

// Send propagates the request to the plugins. It deletes obsolete keys if listKeys() function is not null.
// The listkeys() function is used to list all current keys.
func (dsl *DataResyncDSL) Send() vppclient.Reply {
	var wasErr error

	// Prepare requests with data todo can be scalable
	resyncRequest := getRequestFromData(dsl.put)

	ctx := context.Background()

	if _, err := dsl.client.Resync(ctx, resyncRequest); err != nil {
		wasErr = err
	}

	return &Reply{err: wasErr}
}
