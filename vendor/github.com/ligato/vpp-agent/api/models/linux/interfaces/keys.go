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

package linux_interfaces

import (
	"net"
	"strings"

	"github.com/gogo/protobuf/jsonpb"

	"github.com/ligato/vpp-agent/pkg/models"
)

// ModuleName is the module name used for models.
const ModuleName = "linux.interfaces"

var (
	ModelInterface = models.Register(&Interface{}, models.Spec{
		Module:  ModuleName,
		Version: "v2",
		Type:    "interface",
	})
)

// InterfaceKey returns the key used in ETCD to store configuration of a particular Linux interface.
func InterfaceKey(name string) string {
	return models.Key(&Interface{
		Name: name,
	})
}

const (
	/* Interface host-name (default ns only, notifications) */

	// InterfaceHostNameKeyPrefix is the common prefix of all keys representing
	// existing Linux interfaces in the default namespace (referenced by host names).
	InterfaceHostNameKeyPrefix = "linux/interface/host-name/"

	/* Interface State (derived) */

	// InterfaceStateKeyPrefix is used as a common prefix for keys derived from
	// interfaces to represent the interface admin state (up/down).
	InterfaceStateKeyPrefix = "linux/interface/state/"

	// interfaceStateKeyTemplate is a template for (derived) key representing interface
	// admin state (up/down).
	interfaceStateKeyTemplate = InterfaceStateKeyPrefix + "{ifName}/{ifState}"

	// interface admin state as printed in derived keys.
	interfaceUpState   = "UP"
	interfaceDownState = "DOWN"

	/* Interface Address (derived) */

	// InterfaceAddressKeyPrefix is used as a common prefix for keys derived from
	// interfaces to represent assigned IP addresses.
	InterfaceAddressKeyPrefix = "linux/interface/address/"

	// interfaceAddressKeyTemplate is a template for (derived) key representing IP address
	// (incl. mask) assigned to a Linux interface (referenced by the logical name).
	interfaceAddressKeyTemplate = InterfaceAddressKeyPrefix + "{ifName}/{addr}/{mask}"
)

/* Interface host-name (default ns only, notifications) */

// InterfaceHostNameKey returns key representing Linux interface host name.
func InterfaceHostNameKey(hostName string) string {
	return InterfaceHostNameKeyPrefix + hostName
}

/* Interface State (derived) */

// InterfaceStateKey returns key representing admin state of a Linux interface.
func InterfaceStateKey(ifName string, ifIsUp bool) string {
	ifState := interfaceDownState
	if ifIsUp {
		ifState = interfaceUpState
	}
	key := strings.Replace(interfaceStateKeyTemplate, "{ifName}", ifName, 1)
	key = strings.Replace(key, "{ifState}", ifState, 1)
	return key
}

// ParseInterfaceStateKey parses interface name and state from key derived
// from interface by InterfaceStateKey().
func ParseInterfaceStateKey(key string) (ifName string, ifIsUp bool, isStateKey bool) {
	if strings.HasPrefix(key, InterfaceStateKeyPrefix) {
		keySuffix := strings.TrimPrefix(key, InterfaceStateKeyPrefix)
		keyComps := strings.Split(keySuffix, "/")
		if len(keyComps) != 2 {
			return "", false, false
		}
		ifName = keyComps[0]
		isStateKey = true
		if keyComps[1] == interfaceUpState {
			ifIsUp = true
		}
		return
	}
	return "", false, false
}

/* Interface Address (derived) */

// InterfaceAddressKey returns key representing IP address assigned to Linux interface.
func InterfaceAddressKey(ifName string, address string) string {
	var mask string
	addrComps := strings.Split(address, "/")
	addr := addrComps[0]
	if len(addrComps) > 0 {
		mask = addrComps[1]
	}
	key := strings.Replace(interfaceAddressKeyTemplate, "{ifName}", ifName, 1)
	key = strings.Replace(key, "{addr}", addr, 1)
	key = strings.Replace(key, "{mask}", mask, 1)
	return key
}

// ParseInterfaceAddressKey parses interface address from key derived
// from interface by InterfaceAddressKey().
func ParseInterfaceAddressKey(key string) (ifName string, ifAddr *net.IPNet, isAddrKey bool) {
	var err error
	if strings.HasPrefix(key, InterfaceAddressKeyPrefix) {
		keySuffix := strings.TrimPrefix(key, InterfaceAddressKeyPrefix)
		keyComps := strings.Split(keySuffix, "/")
		if len(keyComps) != 3 {
			return "", nil, false
		}
		_, ifAddr, err = net.ParseCIDR(keyComps[1] + "/" + keyComps[2])
		if err != nil {
			return "", nil, false
		}
		ifName = keyComps[0]
		isAddrKey = true
		return
	}
	return "", nil, false
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
