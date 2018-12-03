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

package l3

const (
	// StaticArpPrefix is a prefix used in ETCD to store configuration for Linux static ARPs.
	StaticArpPrefix = "linux/config/v1/arp/"
	// StaticRoutePrefix is a prefix used in ETCD to store configuration for Linux static routes.
	StaticRoutePrefix = "linux/config/v1/route/"
)

// StaticArpKeyPrefix returns the prefix used in ETCD to store config for Linux static ARPs
func StaticArpKeyPrefix() string {
	return StaticArpPrefix
}

// StaticArpKey returns the prefix used in ETCD to store configuration of a particular Linux ARP entry.
func StaticArpKey(arpLabel string) string {
	return StaticArpPrefix + arpLabel
}

// StaticRouteKeyPrefix returns the prefix used in ETCD to store config for Linux static routes
func StaticRouteKeyPrefix() string {
	return StaticRoutePrefix
}

// StaticRouteKey returns the prefix used in ETCD to store configuration of a particular Linux route.
func StaticRouteKey(routeLabel string) string {
	return StaticRoutePrefix + routeLabel
}
