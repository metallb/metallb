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

package l4

// Prefixes
const (
	// Prefix is common relative key for VPP L4 plugin.
	Prefix = "vpp/config/v1/l4/"

	// L4FeaturesPrefix is the relative key prefix for VPP L4 features.
	FeaturesPrefix = Prefix + "features/"

	// L4NamespacesPrefix is the relative key prefix for VPP L4 application namespaces.
	NamespacesPrefix = Prefix + "namespaces/"
)

// AppNamespacesKey returns the key used in ETCD to store namespaces
func AppNamespacesKey(namespaceID string) string {
	return NamespacesPrefix + namespaceID
}

// FeatureKey returns the key used in ETCD to store L4Feature flag
func FeatureKey() string {
	return FeaturesPrefix + "feature"
}
