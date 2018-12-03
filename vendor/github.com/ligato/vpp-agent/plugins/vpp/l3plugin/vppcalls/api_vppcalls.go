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

package vppcalls

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// ArpVppAPI provides methods for managing ARP entries
type ArpVppAPI interface {
	ArpVppWrite
	ArpVppRead
}

// ArpVppWrite provides write methods for ARPs
type ArpVppWrite interface {
	// VppAddArp adds ARP entry according to provided input
	VppAddArp(entry *ArpEntry) error
	// VppDelArp removes old ARP entry according to provided input
	VppDelArp(entry *ArpEntry) error
}

// ArpVppRead provides read methods for ARPs
type ArpVppRead interface {
	// DumpArpEntries dumps ARPs from VPP and fills them into the provided static route map.
	DumpArpEntries() ([]*ArpDetails, error)
}

// ProxyArpVppAPI provides methods for managing proxy ARP entries
type ProxyArpVppAPI interface {
	ProxyArpVppWrite
	ProxyArpVppRead
}

// ProxyArpVppWrite provides write methods for proxy ARPs
type ProxyArpVppWrite interface {
	// EnableProxyArpInterface enables interface for proxy ARP
	EnableProxyArpInterface(swIfIdx uint32) error
	// DisableProxyArpInterface disables interface for proxy ARP
	DisableProxyArpInterface(swIfIdx uint32) error
	// AddProxyArpRange adds new IP range for proxy ARP
	AddProxyArpRange(firstIP, lastIP []byte) error
	// DeleteProxyArpRange removes proxy ARP IP range
	DeleteProxyArpRange(firstIP, lastIP []byte) error
}

// ProxyArpVppRead provides read methods for proxy ARPs
type ProxyArpVppRead interface {
	// DumpProxyArpRanges returns configured proxy ARP ranges
	DumpProxyArpRanges() ([]*ProxyArpRangesDetails, error)
	// DumpProxyArpRanges returns configured proxy ARP interfaces
	DumpProxyArpInterfaces() ([]*ProxyArpInterfaceDetails, error)
}

// RouteVppAPI provides methods for managing routes
type RouteVppAPI interface {
	RouteVppWrite
	RouteVppRead
}

// RouteVppWrite provides write methods for routes
type RouteVppWrite interface {
	// VppAddRoute adds new route, according to provided input. Every route has to contain VRF ID (default is 0).
	VppAddRoute(ifHandler vppcalls.IfVppWrite, route *l3.StaticRoutes_Route, rtIfIdx uint32) error
	// VppDelRoute removes old route, according to provided input. Every route has to contain VRF ID (default is 0).
	VppDelRoute(route *l3.StaticRoutes_Route, rtIfIdx uint32) error
}

// RouteVppRead provides read methods for routes
type RouteVppRead interface {
	// DumpStaticRoutes dumps l3 routes from VPP and fills them into the provided static route map.
	DumpStaticRoutes() ([]*RouteDetails, error)
}

// IPNeighVppAPI provides methods for managing IP scan neighbor configuration
type IPNeighVppAPI interface {
	// SetIPScanNeighbor configures IP scan neighbor to the VPP
	SetIPScanNeighbor(data *l3.IPScanNeighbor) error
}

// ArpVppHandler is accessor for ARP-related vppcalls methods
type ArpVppHandler struct {
	callsChannel govppapi.Channel
	ifIndexes    ifaceidx.SwIfIndex
	log          logging.Logger
}

// ProxyArpVppHandler is accessor for proxy ARP-related vppcalls methods
type ProxyArpVppHandler struct {
	callsChannel govppapi.Channel
	ifIndexes    ifaceidx.SwIfIndex
	log          logging.Logger
}

// RouteHandler is accessor for route-related vppcalls methods
type RouteHandler struct {
	callsChannel govppapi.Channel
	ifIndexes    ifaceidx.SwIfIndex
	log          logging.Logger
}

// IPNeighHandler is accessor for ip-neighbor-related vppcalls methods
type IPNeighHandler struct {
	callsChannel govppapi.Channel
	log          logging.Logger
}

// NewArpVppHandler creates new instance of IPsec vppcalls handler
func NewArpVppHandler(callsChan govppapi.Channel, ifIndexes ifaceidx.SwIfIndex, log logging.Logger) *ArpVppHandler {
	return &ArpVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}

// NewProxyArpVppHandler creates new instance of proxy ARP vppcalls handler
func NewProxyArpVppHandler(callsChan govppapi.Channel, ifIndexes ifaceidx.SwIfIndex, log logging.Logger) *ProxyArpVppHandler {
	return &ProxyArpVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}

// NewRouteVppHandler creates new instance of route vppcalls handler
func NewRouteVppHandler(callsChan govppapi.Channel, ifIndexes ifaceidx.SwIfIndex, log logging.Logger) *RouteHandler {
	return &RouteHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}

// NewIPNeighVppHandler creates new instance of ip neighbor vppcalls handler
func NewIPNeighVppHandler(callsChan govppapi.Channel, log logging.Logger) *IPNeighHandler {
	return &IPNeighHandler{
		callsChannel: callsChan,
		log:          log,
	}
}
