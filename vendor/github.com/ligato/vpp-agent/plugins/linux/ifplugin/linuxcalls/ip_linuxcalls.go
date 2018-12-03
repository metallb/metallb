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
	"net"

	"github.com/vishvananda/netlink"
)

// GetAddressList calls AddrList netlink API
func (handler *NetLinkHandler) GetAddressList(ifName string) ([]netlink.Addr, error) {
	link, err := handler.GetLinkByName(ifName)
	if err != nil {
		return nil, err
	}

	return netlink.AddrList(link, netlink.FAMILY_ALL)
}

// AddInterfaceIP calls AddrAdd Netlink API.
func (handler *NetLinkHandler) AddInterfaceIP(ifName string, addr *net.IPNet) error {
	link, err := handler.GetLinkByName(ifName)
	if err != nil {
		return err
	}

	return netlink.AddrAdd(link, &netlink.Addr{IPNet: addr})
}

// DelInterfaceIP calls AddrDel Netlink API.
func (handler *NetLinkHandler) DelInterfaceIP(ifName string, addr *net.IPNet) error {
	link, err := handler.GetLinkByName(ifName)
	if err != nil {
		return err
	}

	return netlink.AddrDel(link, &netlink.Addr{IPNet: addr})
}

// SetInterfaceMTU calls LinkSetMTU Netlink API.
func (handler *NetLinkHandler) SetInterfaceMTU(ifName string, mtu int) error {
	link, err := handler.GetLinkByName(ifName)
	if err != nil {
		return err
	}
	return netlink.LinkSetMTU(link, mtu)
}
