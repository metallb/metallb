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
	"strings"

	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/idxmap/mem"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
)

// Names of secondary indexes of PolicySegmentCache
const (
	segmentIndex = "policySegmentName"
	policyIndex  = "policyBSID"
)

// PolicySegmentCache is cache for remembering policy segments and it can be used to handle different cases involving previously already created/delete policies segments.
type PolicySegmentCache struct {
	log      logging.Logger
	internal idxmap.NamedMappingRW
}

type segmentCacheItem struct {
	policyBSID    srv6.SID
	segmentName   string
	policySegment *srv6.PolicySegment
}

// NewPolicySegmentCache creates instance of PolicySegmentCache
func NewPolicySegmentCache(logger logging.Logger) *PolicySegmentCache {
	return &PolicySegmentCache{
		log: logger,
		internal: mem.NewNamedMapping(logger, "policy-segment-cache", func(item interface{}) map[string][]string {
			res := map[string][]string{}
			if sci, ok := item.(segmentCacheItem); ok {
				res[policyIndex] = []string{sci.policyBSID.String()}
				res[segmentIndex] = []string{sci.segmentName}
			}
			return res
		}),
	}
}

// Put adds into cache new policy segment <policySegment> identified by its name <segmentName> and belonging to policy with binding sid <bsid>
func (cache *PolicySegmentCache) Put(policyBSID srv6.SID, segmentName string, policySegment *srv6.PolicySegment) {
	cache.internal.Put(cache.uniqueName(policyBSID, segmentName), segmentCacheItem{policyBSID: policyBSID, segmentName: segmentName, policySegment: policySegment})
}

// Delete removes from cache policy segment identified by its name <segmentName> and belonging to policy with binding sid <bsid>
func (cache *PolicySegmentCache) Delete(policyBSID srv6.SID, segmentName string) (segment *srv6.PolicySegment, exists bool) {
	value, exists := cache.internal.Delete(cache.uniqueName(policyBSID, segmentName))
	if exists {
		segment = value.(segmentCacheItem).policySegment
	}
	return
}

func (cache *PolicySegmentCache) uniqueName(policyBSID srv6.SID, segmentName string) string {
	return policyBSID.String() + srv6.EtcdKeyPathDelimiter + segmentName
}

func (cache *PolicySegmentCache) segmentName(primaryKey string) string {
	parts := strings.Split(primaryKey, srv6.EtcdKeyPathDelimiter)
	return parts[1]
}

// LookupByPolicy lookups in cache all policy segments belonging to policy with binding sid <bsid>
func (cache *PolicySegmentCache) LookupByPolicy(bsid srv6.SID) ([]*srv6.PolicySegment, []string) {
	primaryKeys := cache.internal.ListNames(policyIndex, bsid.String())
	segments := make([]*srv6.PolicySegment, 0, len(primaryKeys))
	segmentNames := make([]string, 0, len(primaryKeys))
	for _, pk := range primaryKeys {
		item, exists := cache.internal.GetValue(pk)
		if !exists {
			cache.log.Warnf("Search by policy index found not existing primary key(%v) for policy segment cache", pk)
			continue
		}
		segments = append(segments, item.(segmentCacheItem).policySegment)
		segmentNames = append(segmentNames, cache.segmentName(pk))
	}
	return segments, segmentNames
}
