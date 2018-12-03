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
	"fmt"
	"strings"

	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/idxmap/mem"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
)

// Names of secondary indexes of SteeringCache
const (
	policyBSIDIndex  = "policyBSID"
	policyIndexIndex = "policyIndex"
)

// SteeringCache is cache for remembering steerings and it can be used to handle different cases involving previously already created/delete steerings.
type SteeringCache struct {
	log      logging.Logger
	internal idxmap.NamedMappingRW
}

// NewSteeringCache creates instance of SteeringCache
func NewSteeringCache(logger logging.Logger) *SteeringCache {
	return &SteeringCache{
		log: logger,
		internal: mem.NewNamedMapping(logger, "steering-cache", func(item interface{}) map[string][]string {
			res := map[string][]string{}
			if steering, ok := item.(*srv6.Steering); ok {
				if len(strings.Trim(steering.PolicyBsid, " ")) > 0 {
					res[policyBSIDIndex] = []string{strings.ToLower(steering.PolicyBsid)}
					res[policyIndexIndex] = []string{""}
				} else {
					res[policyBSIDIndex] = []string{""}
					res[policyIndexIndex] = []string{fmt.Sprint(steering.PolicyIndex)}
				}
			}
			return res
		}),
	}
}

// Put adds into cache new steering <steering> identified by its name <name>
func (cache *SteeringCache) Put(name string, steering *srv6.Steering) {
	cache.internal.Put(name, steering)
}

// Delete removes from cache steering identified by its name <name>
func (cache *SteeringCache) Delete(name string) (steering *srv6.Steering, exists bool) {
	value, exists := cache.internal.Delete(name)
	if exists {
		steering = value.(*srv6.Steering)
	}
	return
}

// GetValue retrieves from cache steering identified by its name <name>
func (cache *SteeringCache) GetValue(name string) (steering *srv6.Steering, exists bool) {
	value, exists := cache.internal.GetValue(name)
	if exists {
		steering = value.(*srv6.Steering)
	}
	return
}

// LookupByPolicyBSID lookups in cache all steerings belonging to policy with binding sid <bsid>
func (cache *SteeringCache) LookupByPolicyBSID(bsid srv6.SID) ([]*srv6.Steering, []string) {
	return cache.getValues(cache.internal.ListNames(policyBSIDIndex, strings.ToLower(bsid.String())))
}

// LookupByPolicyIndex lookups in cache all steerings belonging to policy with internal VPP index <index>
func (cache *SteeringCache) LookupByPolicyIndex(index uint32) ([]*srv6.Steering, []string) {
	return cache.getValues(cache.internal.ListNames(policyIndexIndex, fmt.Sprint(index)))
}

func (cache *SteeringCache) getValues(names []string) ([]*srv6.Steering, []string) {
	steerings := make([]*srv6.Steering, 0, len(names))
	for _, name := range names {
		item, exists := cache.internal.GetValue(name)
		if !exists {
			cache.log.Warnf("Search by policy found not existing name(%v) for steering cache", name)
			continue
		}
		steerings = append(steerings, item.(*srv6.Steering))
	}
	return steerings, names
}
