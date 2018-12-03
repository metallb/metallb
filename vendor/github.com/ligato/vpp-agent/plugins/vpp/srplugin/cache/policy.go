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

package cache

import (
	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/idxmap/mem"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
)

// PolicyCache is cache for remembering policies and it can be used to handle different cases involving previously already created/delete policies.
type PolicyCache struct {
	log      logging.Logger
	internal idxmap.NamedMappingRW
}

// NewPolicyCache creates instance of PolicyCache
func NewPolicyCache(logger logging.Logger) *PolicyCache {
	return &PolicyCache{
		log:      logger,
		internal: mem.NewNamedMapping(logger, "policy-cache", nil),
	}
}

// Put adds new policy <policy> identified by its binding SID <bsid> into cache
func (cache *PolicyCache) Put(bsid srv6.SID, policy *srv6.Policy) {
	cache.internal.Put(bsid.String(), policy)
}

// Delete removes policy identified by its binding SID <bsid> from cache
func (cache *PolicyCache) Delete(bsid srv6.SID) (policy *srv6.Policy, exists bool) {
	value, exists := cache.internal.Delete(bsid.String())
	if exists {
		policy = value.(*srv6.Policy)
	}
	return
}

// GetValue retries policy identified by its binding SID <bsid> from cache
func (cache *PolicyCache) GetValue(bsid srv6.SID) (policy *srv6.Policy, exists bool) {
	value, exists := cache.internal.GetValue(bsid.String())
	if exists {
		policy = value.(*srv6.Policy)
	}
	return
}
