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

package registry

import (
	. "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
)

// Registry can be used to register all descriptors and get quick (cached, O(log))
// lookups by keys.
type Registry interface {
	// RegisterDescriptor add new descriptor into the registry.
	RegisterDescriptor(descriptor *KVDescriptor)

	// GetAllDescriptors returns all registered descriptors ordered by dump-dependencies.
	GetAllDescriptors() []*KVDescriptor

	// GetDescriptor returns descriptor with the given name.
	GetDescriptor(name string) *KVDescriptor

	// GetDescriptorForKey returns descriptor handling the given key.
	GetDescriptorForKey(key string) *KVDescriptor
}
