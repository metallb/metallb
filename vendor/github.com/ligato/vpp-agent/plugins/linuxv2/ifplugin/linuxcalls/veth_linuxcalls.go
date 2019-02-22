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
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

// AddVethInterfacePair calls LinkAdd Netlink API for the Netlink.Veth interface type.
func (h *NetLinkHandler) AddVethInterfacePair(ifName, peerIfName string) error {
	attrs := netlink.NewLinkAttrs()
	attrs.Name = ifName

	// Veth params
	veth := &netlink.Veth{
		LinkAttrs: attrs,
		PeerName:  peerIfName,
	}

	// Add the link
	if err := netlink.LinkAdd(veth); err != nil {
		return errors.Wrapf(err, "add link failed")
	}

	return nil
}
