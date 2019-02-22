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

package vpp_interfaces

import (
	"net"
	"strings"

	"github.com/gogo/protobuf/jsonpb"

	"github.com/ligato/vpp-agent/pkg/models"
)

// ModuleName is the module name used for models.
const ModuleName = "vpp"

var (
	ModelInterface = models.Register(&Interface{}, models.Spec{
		Module:  ModuleName,
		Version: "v2",
		Type:    "interfaces",
	})
)

// InterfaceKey returns the key used in NB DB to store the configuration of the
// given vpp interface.
func InterfaceKey(name string) string {
	return models.Key(&Interface{
		Name: name,
	})
}

/* Interface State */
const (
	// StatePrefix is a key prefix used in NB DB to store interface states.
	StatePrefix = "vpp/status/v2/interface/"
)

/* Interface Error */
const (
	// ErrorPrefix is a key prefix used in NB DB to store interface errors.
	ErrorPrefix = "vpp/status/v2/interface/error/"
)

/* Interface Address (derived) */
const (
	// AddressKeyPrefix is used as a common prefix for keys derived from
	// interfaces to represent assigned IP addresses.
	AddressKeyPrefix = "vpp/interface/address/"

	// addressKeyTemplate is a template for (derived) key representing IP address
	// (incl. mask) assigned to a VPP interface.
	addressKeyTemplate = AddressKeyPrefix + "{iface}/{addr}/{mask}"
)

/* Unnumbered interface (derived) */
const (
	// UnnumberedKeyPrefix is used as a common prefix for keys derived from
	// interfaces to represent unnumbered interfaces.
	UnnumberedKeyPrefix = "vpp/interface/unnumbered/"
)

/* DHCP (client - derived, lease - notification) */
const (
	// DHCPClientKeyPrefix is used as a common prefix for keys derived from
	// interfaces to represent enabled DHCP clients.
	DHCPClientKeyPrefix = "vpp/interface/dhcp-client/"

	// DHCPLeaseKeyPrefix is used as a common prefix for keys representing
	// notifications with DHCP leases.
	DHCPLeaseKeyPrefix = "vpp/interface/dhcp-lease/"
)

const (
	// InvalidKeyPart is used in key for parts which are invalid
	InvalidKeyPart = "<invalid>"
)

/* Interface Error */

// InterfaceErrorKey returns the key used in NB DB to store the interface errors.
func InterfaceErrorKey(iface string) string {
	if iface == "" {
		iface = InvalidKeyPart
	}
	return ErrorPrefix + iface
}

/* Interface State */

// InterfaceStateKey returns the key used in NB DB to store the state data of the
// given vpp interface.
func InterfaceStateKey(iface string) string {
	if iface == "" {
		iface = InvalidKeyPart
	}
	return StatePrefix + iface
}

/* Interface Address (derived) */

// InterfaceAddressKey returns key representing IP address assigned to VPP interface.
func InterfaceAddressKey(iface string, address string) string {
	if iface == "" {
		iface = InvalidKeyPart
	}

	// parse address
	ipAddr, addrNet, err := net.ParseCIDR(address)
	if err != nil {
		address = InvalidKeyPart + "/" + InvalidKeyPart
	} else {
		addrNet.IP = ipAddr
		address = addrNet.String()
	}

	key := strings.Replace(addressKeyTemplate, "{iface}", iface, 1)
	key = strings.Replace(key, "{addr}/{mask}", address, 1)

	return key
}

// ParseInterfaceAddressKey parses interface address from key derived
// from interface by InterfaceAddressKey().
func ParseInterfaceAddressKey(key string) (iface string, ipAddr net.IP, ipAddrNet *net.IPNet, isAddrKey bool) {
	if suffix := strings.TrimPrefix(key, AddressKeyPrefix); suffix != key {
		parts := strings.Split(suffix, "/")

		// beware: interface name may contain forward slashes (e.g. ETHERNET_CSMACD)
		if len(parts) < 3 {
			return "", nil, nil, false
		}

		// parse IP address
		lastIdx := len(parts) - 1
		var err error
		ipAddr, ipAddrNet, err = net.ParseCIDR(parts[lastIdx-1] + "/" + parts[lastIdx])
		if err != nil {
			return "", nil, nil, false
		}

		// parse interface name
		iface = strings.Join(parts[:lastIdx-1], "/")
		if iface == "" {
			return "", nil, nil, false
		}
		return iface, ipAddr, ipAddrNet, true
	}
	return
}

/* Unnumbered interface (derived) */

// UnnumberedKey returns key representing unnumbered interface.
func UnnumberedKey(iface string) string {
	if iface == "" {
		iface = InvalidKeyPart
	}
	return UnnumberedKeyPrefix + iface
}

// ParseNameFromUnnumberedKey returns suffix of the key.
func ParseNameFromUnnumberedKey(key string) (iface string, isUnnumberedKey bool) {
	suffix := strings.TrimPrefix(key, UnnumberedKeyPrefix)
	if suffix != key && suffix != "" {
		return suffix, true
	}
	return
}

/* DHCP (client - derived, lease - notification) */

// DHCPClientKey returns a (derived) key used to represent enabled DHCP lease.
func DHCPClientKey(iface string) string {
	if iface == "" {
		iface = InvalidKeyPart
	}
	return DHCPClientKeyPrefix + iface
}

// ParseNameFromDHCPClientKey returns suffix of the key.
func ParseNameFromDHCPClientKey(key string) (iface string, isDHCPClientKey bool) {
	if suffix := strings.TrimPrefix(key, DHCPClientKeyPrefix); suffix != key && suffix != "" {
		return suffix, true
	}
	return
}

// DHCPLeaseKey returns a key used to represent DHCP lease for the given interface.
func DHCPLeaseKey(iface string) string {
	if iface == "" {
		iface = InvalidKeyPart
	}
	return DHCPLeaseKeyPrefix + iface
}

// ParseNameFromDHCPLeaseKey returns suffix of the key.
func ParseNameFromDHCPLeaseKey(key string) (iface string, isDHCPLeaseKey bool) {
	if suffix := strings.TrimPrefix(key, DHCPLeaseKeyPrefix); suffix != key && suffix != "" {
		return suffix, true
	}
	return
}

// MarshalJSON ensures that field of type 'oneOf' is correctly marshaled
// by using gogo lib marshaller
func (m *Interface) MarshalJSON() ([]byte, error) {
	marshaller := &jsonpb.Marshaler{}
	str, err := marshaller.MarshalToString(m)
	if err != nil {
		return nil, err
	}
	return []byte(str), nil
}

// UnmarshalJSON ensures that field of type 'oneOf' is correctly unmarshaled
func (m *Interface) UnmarshalJSON(data []byte) error {
	return jsonpb.UnmarshalString(string(data), m)
}
