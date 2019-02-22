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

// +build windows darwin

package linuxcalls

import (
	"net"

	intf "github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
)

// NamespaceMgmtCtx is a mock type definition of the real NamespaceMgmtCtx from ns_linuxcalls.go.
type NamespaceMgmtCtx struct {
}

// InterfaceAdminDown is a mock implementation of the real InterfaceAdminDown from admin_linuxcalls.go,
// doing absolutely nothing.
func InterfaceAdminDown(ifName string) error {
	return nil
}

// InterfaceAdminUp is a mock implementation of the real InterfaceAdminUp from admin_linuxcalls.go,
// doing absolutely nothing.
func InterfaceAdminUp(ifName string) error {
	return nil
}

// AddInterfaceIP is a mock implementation of the real AddInterfaceIp from ip_linuxcalls.go,
// doing absolutely nothing.
func AddInterfaceIP(ifName string, addr *net.IPNet) error {
	return nil
}

// DelInterfaceIP is a mock implementation of the real DelInterfaceIp from ip_linuxcalls.go,
// doing absolutely nothing.
func DelInterfaceIP(ifName string, addr *net.IPNet) error {
	return nil
}

// SetInterfaceMTU is a mock implementation of the real SetInterfaceMTU from ip_linuxcalls.go,
// doing absolutely nothing.
func SetInterfaceMTU(ifName string, mtu int) error {
	return nil
}

// SetInterfaceMac is a mock implementation of the real SetInterfaceMac from mac_linuxcalls.go,
// doing absolutely nothing.
func SetInterfaceMac(ifName string, macAddress string) error {
	return nil
}

// NewNamespaceMgmtCtx is a mock implementation of the real NewNamespaceMgmtCtx from ns_linuxcalls.go,
// doing absolutely nothing.
func NewNamespaceMgmtCtx() *NamespaceMgmtCtx {
	return nil
}

// CompareNamespaces is a mock implementation of the real CompareNamespaces from ns_linuxcalls.go,
// doing absolutely nothing.
func CompareNamespaces(ns1 *intf.LinuxInterfaces_Interface_Namespace, ns2 *intf.LinuxInterfaces_Interface_Namespace) int {
	return 0
}

// NamespaceToStr is a mock implementation of the real NamespaceToStr from ns_linuxcalls.go,
// doing absolutely nothing.
func NamespaceToStr(namespace *intf.LinuxInterfaces_Interface_Namespace) string {
	return ""
}

// GetDefaultNamespace is a mock implementation of the real GetDefaultNamespace from ns_linuxcalls.go,
// doing absolutely nothing.
func GetDefaultNamespace() *intf.LinuxInterfaces_Interface_Namespace {
	return nil
}

// SetInterfaceNamespace is a mock implementation of the real SetInterfaceNamespace from ns_linuxcalls.go,
// doing absolutely nothing.
func SetInterfaceNamespace(ctx *NamespaceMgmtCtx, ifName string, namespace *intf.LinuxInterfaces_Interface_Namespace) error {
	return nil
}

// SwitchNamespace is a mock implementation of the real SwitchNamespace from ns_linuxcalls.go,
// doing absolutely nothing.
func SwitchNamespace(ctx *NamespaceMgmtCtx, namespace *intf.LinuxInterfaces_Interface_Namespace) (revert func(), err error) {
	return func() {}, nil
}

// AddVethInterface is a mock implementation of the real AddVethInterface from veth_linuxcalls.go,
// doing absolutely nothing.
func AddVethInterface(ifName, peerIfName string) error {
	return nil
}

// DelVethInterface is a mock implementation of the real DelVethInterface from veth_linuxcalls.go,
// doing absolutely nothing.
func DelVethInterface(ifName, peerIfName string) error {
	return nil
}

// GetInterfaceType is a mock implementation of the real GetInterfaceType from link_linuxcalls.go,
// doing absolutely nothing.
func GetInterfaceType(ifName string) (string, error) {
	return "", nil
}

// InterfaceExists is a mock implementation of the real InterfaceExists from link_linuxcalls.go,
// doing absolutely nothing.
func InterfaceExists(ifName string) (bool, error) {
	return false, nil
}

// GetVethPeerName is a mock implementation of the real GetVethPeerName from veth_linuxcalls.go,
// doing absolutely nothing.
func GetVethPeerName(ifName string) (string, error) {
	return "", nil
}
