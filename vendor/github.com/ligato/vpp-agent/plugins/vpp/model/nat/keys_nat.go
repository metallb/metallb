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

package nat

const (
	// Prefix is NAT prefix
	Prefix = "vpp/config/v1/nat/"
	// GlobalPrefix is relative prefix for global config
	GlobalPrefix = Prefix + "global/"
	// SNatPrefix is relative prefix for SNAT setup
	SNatPrefix = Prefix + "snat/"
	// DNatPrefix is relative prefix for DNAT setup
	DNatPrefix = Prefix + "dnat/"
)

// SNatKey returns the key used in ETCD to store SNAT config
func SNatKey(label string) string {
	return SNatPrefix + label
}

// DNatKey returns the key used in ETCD to store DNAT config
func DNatKey(label string) string {
	return DNatPrefix + label
}
