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

package ipsec

// Prefixes
const (
	// KeyPrefix is the relative key prefix for IPSec
	KeyPrefix = "vpp/config/v1/ipsec/"
	// KeyPrefixSPD is the relative key prefix for IPSec's Security Policy Databases
	KeyPrefixSPD = KeyPrefix + "spd/"
	// KeyPrefixSA is the relative key prefix for IPSec's Security Associations
	KeyPrefixSA = KeyPrefix + "sa/"
	// KeyPrefixTunnel is the relative key prefix for IPSec's Tunnel Interface
	KeyPrefixTunnel = KeyPrefix + "tunnel/"
)

// SPDKey returns key for Security Policy Database
func SPDKey(name string) string {
	return KeyPrefixSPD + name
}

// SAKey returns key for Security Association
func SAKey(name string) string {
	return KeyPrefixSA + name
}

// TunnelKey returns key for Tunnel Interface
func TunnelKey(name string) string {
	return KeyPrefixTunnel + name
}
