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

package l2

import (
	"fmt"
	"strings"
)

// Prefixes
const (
	// bdPrefix is the relative key prefix for bridge domains.
	BdPrefix = "vpp/config/v1/bd/"
	// bdStatePrefix is the relative key prefix for bridge domain state.
	BdStatePrefix = "vpp/status/v1/bd/"
	// bdErrPrefix is the relative key prefix for the bridge domain error.
	BdErrPrefix = "vpp/status/v1/bd/error/"
	// fibPrefix is the relative key prefix for FIB table entries.
	FibPrefix = "vpp/config/v1/bd/{bd}/fib/"
	// xConnectPrefix is the relative key prefix for xconnects.
	XConnectPrefix = "vpp/config/v1/xconnect/"
)

// BridgeDomainKey returns the prefix used in ETCD to store vpp bridge domain config
// of a particular bridge domain in selected vpp instance.
func BridgeDomainKey(bdName string) string {
	return BdPrefix + bdName
}

// BridgeDomainStateKey returns the prefix used in ETCD to store vpp bridge domain state data
// of a particular bridge domain in selected vpp instance.
func BridgeDomainStateKey(ifaceLabel string) string {
	return BdStatePrefix + ifaceLabel
}

// BridgeDomainErrorKey returns the key used in ETCD to store bridge domain errors.
func BridgeDomainErrorKey(bdLabel string) string {
	return BdErrPrefix + bdLabel
}

// ParseBDNameFromKey returns suffix of the key.
func ParseBDNameFromKey(key string) (name string, err error) {
	lastSlashPos := strings.LastIndex(key, "/")
	if lastSlashPos > 0 && lastSlashPos < len(key)-1 {
		return key[lastSlashPos+1:], nil
	}

	return key, fmt.Errorf("wrong format of the key %s", key)
}

// FibKey returns the prefix used in ETCD to store vpp fib table entry config
// of a particular fib in selected vpp instance.
func FibKey(bdLabel string, fibMac string) string {
	return strings.Replace(FibPrefix, "{bd}", bdLabel, 1) + fibMac
}

// ParseFibKey parses bridge domain label and FIB MAC address from a FIB key.
func ParseFibKey(key string) (isFibKey bool, bdName string, fibMac string) {
	if strings.HasPrefix(key, BdPrefix) {
		bdSuffix := strings.TrimPrefix(key, BdPrefix)
		fibComps := strings.Split(bdSuffix, "/")
		if len(fibComps) == 3 && fibComps[1] == "fib" {
			return true, fibComps[0], fibComps[2]
		}
	}
	return false, "", ""
}

// XConnectKey returns the prefix used in ETCD to store vpp xConnect pair config
// of particular xConnect pair in selected vpp instance.
func XConnectKey(rxIface string) string {
	return XConnectPrefix + rxIface
}
