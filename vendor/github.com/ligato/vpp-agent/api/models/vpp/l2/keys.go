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

package vpp_l2

import (
	"strings"

	"github.com/ligato/vpp-agent/pkg/models"
)

// ModuleName is the module name used for models.
const ModuleName = "vpp.l2"

var (
	ModelBridgeDomain = models.Register(&BridgeDomain{}, models.Spec{
		Module:  ModuleName,
		Type:    "bridge-domain",
		Version: "v2",
	})

	ModelFIBEntry = models.Register(&FIBEntry{}, models.Spec{
		Module:  ModuleName,
		Type:    "fib",
		Version: "v2",
	}, models.WithNameTemplate("{{.BridgeDomain}}/mac/{{.PhysAddress}}"))

	ModelXConnectPair = models.Register(&XConnectPair{}, models.Spec{
		Module:  ModuleName,
		Type:    "xconnect",
		Version: "v2",
	}, models.WithNameTemplate("{{.ReceiveInterface}}"))
)

// BridgeDomainKey returns the key used in NB DB to store the configuration of the
// given bridge domain.
func BridgeDomainKey(bdName string) string {
	return models.Key(&BridgeDomain{
		Name: bdName,
	})
}

// FIBKey returns the key used in NB DB to store the configuration of the
// given L2 FIB entry.
func FIBKey(bdName string, fibMac string) string {
	return models.Key(&FIBEntry{
		BridgeDomain: bdName,
		PhysAddress:  fibMac,
	})
}

// XConnectKey returns the key used in NB DB to store the configuration of the
// given xConnect (identified by RX interface).
func XConnectKey(rxIface string) string {
	return models.Key(&XConnectPair{
		ReceiveInterface: rxIface,
	})
}

/* BD <-> interface binding (derived) */
const (
	// bdInterfaceKeyTemplate is a template for (derived) key representing binding
	// between interface and a bridge domain.
	bdInterfaceKeyTemplate = "vpp/bd/{bd}/interface/{iface}"
)

const (
	// InvalidKeyPart is used in key for parts which are invalid
	InvalidKeyPart = "<invalid>"
)

/* BD <-> interface binding (derived) */

// BDInterfaceKey returns the key used to represent binding between the given interface
// and the bridge domain.
func BDInterfaceKey(bdName string, iface string) string {
	if bdName == "" {
		bdName = InvalidKeyPart
	}
	if iface == "" {
		iface = InvalidKeyPart
	}
	key := strings.Replace(bdInterfaceKeyTemplate, "{bd}", bdName, 1)
	key = strings.Replace(key, "{iface}", iface, 1)
	return key
}

// ParseBDInterfaceKey parses key representing binding between interface and a bridge
// domain.
func ParseBDInterfaceKey(key string) (bdName string, iface string, isBDIfaceKey bool) {
	keyComps := strings.Split(key, "/")
	if len(keyComps) >= 5 && keyComps[0] == "vpp" && keyComps[1] == "bd" && keyComps[3] == "interface" {
		iface = strings.Join(keyComps[4:], "/")
		return keyComps[2], iface, true
	}
	return "", "", false
}
