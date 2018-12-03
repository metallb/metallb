// Copyright (c) 2018 Bell Canada, Pantheon Technologies and/or its affiliates.
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

package srv6

import (
	"net"

	"fmt"
	"regexp"
	"strings"
)

// Keys and prefixes(to keys) used for SRv6 in ETCD key-value store
const (
	basePrefix     = "vpp/config/v1/srv6/"
	localSIDPrefix = basePrefix + "localsid/" // full key is in form .../localsid/{name}
	policyPrefix   = basePrefix + "policy/"   // full key is in form .../policy/{name}
	steeringPrefix = basePrefix + "steering/" // full key is in form .../steering/{name}
)

var policySegmentPrefixRegExp = regexp.MustCompile(policyPrefix + "([^/]+)/segment/") // full key is in form .../policy/{policyName}/segment/{name}

// EtcdKeyPathDelimiter is delimiter used in ETCD keys and can be used to combine multiple etcd key parts together
// (without worry that key part has accidentally this delimiter because otherwise it would not be one key part)
const EtcdKeyPathDelimiter = "/"

// SID (in srv6 package) is SRv6's segment id. It is always represented as IPv6 address
type SID = net.IP

// BasePrefix returns the prefix used in ETCD to store vpp SRv6 config.
func BasePrefix() string {
	return basePrefix
}

// LocalSIDPrefix returns longest common prefix for all local SID keys
func LocalSIDPrefix() string {
	return localSIDPrefix
}

// PolicyPrefix returns longest common prefix for all policy keys
func PolicyPrefix() string {
	return policyPrefix
}

// IsPolicySegmentPrefix check whether key has policy segment prefix
func IsPolicySegmentPrefix(key string) bool {
	return policySegmentPrefixRegExp.MatchString(key)
}

// SteeringPrefix returns longest common prefix for all steering keys
func SteeringPrefix() string {
	return steeringPrefix
}

// ParsePolicySegmentKey parses policy segment name.
func ParsePolicySegmentKey(key string) (string, error) {
	if !policySegmentPrefixRegExp.MatchString(key) {
		return "", fmt.Errorf("key %v is not policy segment key", key)
	}
	suffix := strings.TrimPrefix(key, policyPrefix)
	keyComponents := strings.Split(suffix, EtcdKeyPathDelimiter)
	if len(keyComponents) != 3 {
		return "", fmt.Errorf("key %q should have policy name and policy segment name", key)
	}
	return keyComponents[2], nil
}
