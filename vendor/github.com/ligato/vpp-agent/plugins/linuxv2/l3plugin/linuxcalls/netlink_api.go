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

package linuxcalls

import (
	"github.com/vishvananda/netlink"
)

// NetlinkAPI interface covers all methods inside linux calls package needed
// to manage linux ARP entries and routes.
type NetlinkAPI interface {
	NetlinkAPIWrite
	NetlinkAPIRead
}

// NetlinkAPIWrite interface covers write methods inside linux calls package
// needed to manage linux ARP entries and routes.
type NetlinkAPIWrite interface {
	/* ARP */
	// SetARPEntry adds/modifies existing linux ARP entry.
	SetARPEntry(arpEntry *netlink.Neigh) error
	// DelARPEntry removes linux ARP entry.
	DelARPEntry(arpEntry *netlink.Neigh) error

	/* Routes */
	// AddRoute adds new linux static route.
	AddRoute(route *netlink.Route) error
	// ReplaceRoute changes existing linux static route.
	ReplaceRoute(route *netlink.Route) error
	// DelRoute removes linux static route.
	DelRoute(route *netlink.Route) error
}

// NetlinkAPIRead interface covers read methods inside linux calls package
// needed to manage linux ARP entries and routes.
type NetlinkAPIRead interface {
	// GetARPEntries reads all configured static ARP entries for given interface.
	// <interfaceIdx> works as filter, if set to zero, all arp entries in the namespace
	// are returned.
	GetARPEntries(interfaceIdx int) ([]netlink.Neigh, error)

	// GetRoutes reads all configured static routes with the given outgoing
	// interface.
	// <interfaceIdx> works as filter, if set to zero, all routes in the namespace
	// are returned.
	GetRoutes(interfaceIdx int) (v4Routes, v6Routes []netlink.Route, err error)
}

// NetLinkHandler is accessor for Netlink methods.
type NetLinkHandler struct {
}

// NewNetLinkHandler creates new instance of Netlink handler.
func NewNetLinkHandler() *NetLinkHandler {
	return &NetLinkHandler{}
}
