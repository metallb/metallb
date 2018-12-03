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

// GetLinkByName calls netlink API to get Link type from interface name
func (handler *NetLinkHandler) GetLinkByName(ifName string) (netlink.Link, error) {
	return netlink.LinkByName(ifName)
}

// GetLinkList calls netlink API to get all Links in namespace
func (handler *NetLinkHandler) GetLinkList() ([]netlink.Link, error) {
	return netlink.LinkList()
}

// GetInterfaceType returns the type (string representation) of a given interface.
func (handler *NetLinkHandler) GetInterfaceType(ifName string) (string, error) {
	link, err := handler.GetLinkByName(ifName)
	if err != nil {
		return "", err
	}
	return link.Type(), nil
}

// InterfaceExists checks if interface with a given name exists.
func (handler *NetLinkHandler) InterfaceExists(ifName string) (bool, error) {
	_, err := handler.GetLinkByName(ifName)
	if err == nil {
		return true, nil
	}
	if _, notFound := err.(netlink.LinkNotFoundError); notFound {
		return false, nil
	}
	return false, err
}

// RenameInterface changes the name of the interface <ifName> to <newName>.
func (handler *NetLinkHandler) RenameInterface(ifName string, newName string) error {
	link, err := handler.GetLinkByName(ifName)
	if err != nil {
		return err
	}
	err = handler.SetInterfaceDown(ifName)
	if err != nil {
		return err
	}
	err = netlink.LinkSetName(link, newName)
	if err != nil {
		return err
	}
	err = handler.SetInterfaceUp(newName)
	if err != nil {
		return err
	}
	return nil
}

// GetInterfaceByName return *net.Interface type from interface name
func (handler *NetLinkHandler) GetInterfaceByName(ifName string) (*net.Interface, error) {
	return net.InterfaceByName(ifName)
}
