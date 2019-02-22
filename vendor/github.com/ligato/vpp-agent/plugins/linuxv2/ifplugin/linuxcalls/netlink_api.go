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
	"net"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// NetlinkAPI interface covers all methods inside linux calls package
// needed to manage linux interfaces.
type NetlinkAPI interface {
	NetlinkAPIWrite
	NetlinkAPIRead
}

// NetlinkAPIWrite interface covers write methods inside linux calls package
// needed to manage linux interfaces.
type NetlinkAPIWrite interface {
	// AddVethInterfacePair configures two connected VETH interfaces
	AddVethInterfacePair(ifName, peerIfName string) error
	// DeleteInterface removes the given interface.
	DeleteInterface(ifName string) error
	// SetInterfaceUp sets interface state to 'up'
	SetInterfaceUp(ifName string) error
	// SetInterfaceDown sets interface state to 'down'
	SetInterfaceDown(ifName string) error
	// AddInterfaceIP adds new IP address
	AddInterfaceIP(ifName string, addr *net.IPNet) error
	// DelInterfaceIP removes IP address from linux interface
	DelInterfaceIP(ifName string, addr *net.IPNet) error
	// SetInterfaceMac sets MAC address
	SetInterfaceMac(ifName string, macAddress string) error
	// SetInterfaceMTU set maximum transmission unit for interface
	SetInterfaceMTU(ifName string, mtu int) error
	// RenameInterface changes interface host name
	RenameInterface(ifName string, newName string) error
	// SetInterfaceAlias sets the alias of the given interface.
	// Equivalent to: `ip link set dev $ifName alias $alias`
	SetInterfaceAlias(ifName, alias string) error
	// SetLinkNamespace puts link into a network namespace.
	SetLinkNamespace(link netlink.Link, ns netns.NsHandle) error
	// SetChecksumOffloading enables/disables Rx/Tx checksum offloading
	// for the given interface.
	SetChecksumOffloading(ifName string, rxOn, txOn bool) error
}

// NetlinkAPIRead interface covers read methods inside linux calls package
// needed to manage linux interfaces.
type NetlinkAPIRead interface {
	// GetLinkByName returns netlink interface type
	GetLinkByName(ifName string) (netlink.Link, error)
	// GetLinkList return all links from namespace
	GetLinkList() ([]netlink.Link, error)
	// LinkSubscribe takes a channel to which notifications will be sent
	// when links change. Close the 'done' chan to stop subscription.
	LinkSubscribe(ch chan<- netlink.LinkUpdate, done <-chan struct{}) error
	// GetAddressList reads all IP addresses
	GetAddressList(ifName string) ([]netlink.Addr, error)
	// InterfaceExists verifies interface existence
	InterfaceExists(ifName string) (bool, error)
	// IsInterfaceUp checks if the interface is UP.
	IsInterfaceUp(ifName string) (bool, error)
	// GetInterfaceType returns linux interface type
	GetInterfaceType(ifName string) (string, error)
	// GetChecksumOffloading returns the state of Rx/Tx checksum offloading
	// for the given interface.
	GetChecksumOffloading(ifName string) (rxOn, txOn bool, err error)
}

// NetLinkHandler is accessor for Netlink methods.
type NetLinkHandler struct {
}

// NewNetLinkHandler creates new instance of Netlink handler.
func NewNetLinkHandler() *NetLinkHandler {
	return &NetLinkHandler{}
}
