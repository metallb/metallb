//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

// +build !windows,!darwin

package linuxcalls

import (
	"github.com/vishvananda/netlink"
)

// AddStaticRoute creates the new static route
func (h *NetLinkHandler) AddStaticRoute(name string, route *netlink.Route) error {
	return netlink.RouteAdd(route)
}

// ReplaceStaticRoute removes the static route
func (h *NetLinkHandler) ReplaceStaticRoute(name string, route *netlink.Route) error {
	return netlink.RouteReplace(route)
}

// DelStaticRoute removes the static route
func (h *NetLinkHandler) DelStaticRoute(name string, route *netlink.Route) error {
	return netlink.RouteDel(route)
}

// GetStaticRoutes reads linux routes. Possible to filter by interface and IP family.
func (h *NetLinkHandler) GetStaticRoutes(link netlink.Link, family int) ([]netlink.Route, error) {
	return netlink.RouteList(link, family)
}
