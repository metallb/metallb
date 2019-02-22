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

// SetInterfaceMac calls LinkSetHardwareAddr netlink API.
func (h *NetLinkHandler) SetInterfaceMac(ifName string, macAddress string) error {
	link, err := h.GetLinkByName(ifName)
	if err != nil {
		return err
	}
	hwAddr, err := net.ParseMAC(macAddress)
	if err != nil {
		return err
	}
	return netlink.LinkSetHardwareAddr(link, hwAddr)
}
