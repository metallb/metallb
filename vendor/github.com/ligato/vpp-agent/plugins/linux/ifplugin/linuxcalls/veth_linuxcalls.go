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

package linuxcalls

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// AddVethInterfacePair calls LinkAdd Netlink API for the Netlink.Veth interface type.
func (handler *NetLinkHandler) AddVethInterfacePair(ifName, peerIfName string) error {
	// Veth pair params
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:   ifName,
			TxQLen: 0,
		},
		PeerName: peerIfName,
	}

	// Create the veth pair.
	err := netlink.LinkAdd(veth)
	return err
}

// DelVethInterfacePair calls LinkDel Netlink API for the Netlink.Veth interface type.
func (handler *NetLinkHandler) DelVethInterfacePair(ifName, peerIfName string) error {
	// Veth pair params.
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:   ifName,
			TxQLen: 0,
		},
		PeerName: peerIfName,
	}

	// Create the veth pair.
	err := netlink.LinkDel(veth)
	return err
}

// GetVethPeerName return the peer name for a given VETH interface.
func (handler *NetLinkHandler) GetVethPeerName(ifName string) (string, error) {
	link, err := handler.GetLinkByName(ifName)
	if err != nil {
		return "", err
	}
	veth, isVeth := link.(*netlink.Veth)
	if !isVeth {
		return "", fmt.Errorf("interface '%s' is not VETH", ifName)
	}
	return veth.PeerName, nil
}
