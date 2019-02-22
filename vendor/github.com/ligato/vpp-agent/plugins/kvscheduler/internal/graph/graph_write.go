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

package graph

import (
	"reflect"
	"time"

	"github.com/ligato/cn-infra/idxmap"
)

// graphRW implements RWAccess.
type graphRW struct {
	*graphR
	record  bool
	deleted []string
	newRevs map[string]bool // key -> data-updated?
}

// newGraphRW creates a new instance of grapRW, which extends an existing
// graph with write-operations.
func newGraphRW(graph *graphR, recordChanges bool) *graphRW {
	graphRCopy := graph.copyNodesOnly()
	return &graphRW{
		graphR:  graphRCopy,
		record:  recordChanges,
		newRevs: make(map[string]bool),
	}
}

// RegisterMetadataMap registers new metadata map for value-label->metadata
// associations of selected node.
func (graph *graphRW) RegisterMetadataMap(mapName string, mapping idxmap.NamedMappingRW) {
	if graph.mappings == nil {
		graph.mappings = make(map[string]idxmap.NamedMappingRW)
	}
	graph.mappings[mapName] = mapping
}

// SetNode creates new node or returns read-write handle to an existing node.
// The changes are propagated to the graph only after Save() is called.
// If <newRev> is true, the changes will recorded as a new revision of the
// node for the history.
func (graph *graphRW) SetNode(key string) NodeRW {
	node, has := graph.nodes[key]
	if has {
		return node
	}
	node = newNode(nil)
	node.graph = graph.graphR
	node.key = key
	for _, otherNode := range graph.nodes {
		otherNode.checkPotentialTarget(node)
	}
	graph.nodes[key] = node

	return node
}

// DeleteNode deletes node with the given key.
// Returns true if the node really existed before the operation.
func (graph *graphRW) DeleteNode(key string) bool {
	node, has := graph.nodes[key]
	if !has {
		return false
	}

	// remove from sources of current targets
	node.removeThisFromSources()

	// delete from graph
	delete(graph.nodes, key)

	// remove from targets of other nodes
	for _, otherNode := range graph.nodes {
		otherNode.removeFromTargets(key)
	}
	graph.deleted = append(graph.deleted, key)
	return true
}

// Save propagates all changes to the graph.
func (graph *graphRW) Save() {
	graph.parent.rwLock.Lock()
	defer graph.parent.rwLock.Unlock()

	destGraph := graph.parent.graph

	// propagate newly registered mappings
	for mapName, mapping := range graph.mappings {
		if _, alreadyReg := destGraph.mappings[mapName]; !alreadyReg {
			destGraph.mappings[mapName] = mapping
		}
	}

	// apply deleted nodes
	for _, key := range graph.deleted {
		if node, has := destGraph.nodes[key]; has {
			// remove metadata
			if node.metadataAdded {
				if mapping, hasMapping := destGraph.mappings[node.metadataMap]; hasMapping {
					mapping.Delete(node.label)
				}
			}
			// remove node from graph
			delete(destGraph.nodes, key)
		}
		graph.newRevs[key] = true
	}
	graph.deleted = []string{}

	// apply new/changes nodes
	for key, node := range graph.nodes {
		if !node.dataUpdated && !node.targetsUpdated && !node.sourcesUpdated {
			continue
		}

		// update metadata
		if !node.metaInSync {
			// update metadata map
			if mapping, hasMapping := destGraph.mappings[node.metadataMap]; hasMapping {
				if node.metadataAdded {
					if node.metadata == nil {
						mapping.Delete(node.label)
						node.metadataAdded = false
					} else {
						prevMeta, _ := mapping.GetValue(node.label)
						if !reflect.DeepEqual(prevMeta, node.metadata) {
							mapping.Update(node.label, node.metadata)
						}
					}
				} else if node.metadata != nil {
					mapping.Put(node.label, node.metadata)
					node.metadataAdded = true
				}
			}
		}

		// mark node for recording during RW-handle release
		// (ignore if only sources have been updated)
		if node.dataUpdated || node.targetsUpdated {
			if _, newRev := graph.newRevs[key]; !newRev {
				graph.newRevs[key] = false
			}
			graph.newRevs[key] = graph.newRevs[key] || node.dataUpdated
		}

		// copy changed node to the actual graph
		nodeCopy := node.copy()
		nodeCopy.graph = destGraph
		destGraph.nodes[key] = newNode(nodeCopy)

		// use copy-on-write targets+sources for the write-handle
		cowTargets := nodeCopy.targets
		nodeCopy.targets = node.targets
		node.targets = cowTargets
		cowSources := nodeCopy.sources
		nodeCopy.sources = node.sources
		node.sources = cowSources

		// working copy is now in-sync
		node.dataUpdated = false
		node.targetsUpdated = false
		node.sourcesUpdated = false
		node.metaInSync = true
	}
}

// Release records changes if requested.
func (graph *graphRW) Release() {
	if graph.record && graph.parent.recordOldRevs {
		graph.parent.rwLock.Lock()
		defer graph.parent.rwLock.Unlock()

		destGraph := graph.parent.graph
		for key, dataUpdated := range graph.newRevs {
			node, exists := destGraph.nodes[key]
			if _, hasTimeline := destGraph.timeline[key]; !hasTimeline {
				if !exists {
					// deleted, but never recorded => skip
					continue
				}
				destGraph.timeline[key] = []*RecordedNode{}
			}
			records := destGraph.timeline[key]
			if len(records) > 0 {
				lastRecord := records[len(records)-1]
				if lastRecord.Until.IsZero() {
					lastRecord.Until = time.Now()
				}
			}
			if exists {
				destGraph.timeline[key] = append(records,
					destGraph.recordNode(node, !dataUpdated))
			}
		}

		// remove past revisions from the log which are too old to keep
		now := time.Now()
		sinceLastTrimming := now.Sub(graph.parent.lastRevTrimming)
		if sinceLastTrimming >= oldRevsTrimmingPeriod {
			for key, records := range destGraph.timeline {
				var i, j int // i = first after init period, j = first after init period to keep
				for i = 0; i < len(records); i++ {
					sinceStart := records[i].Since.Sub(graph.parent.startTime)
					if sinceStart > graph.parent.permanentInitPeriod {
						break
					}
				}
				for j = i; j < len(records); j++ {
					if records[j].Until.IsZero() {
						break
					}
					elapsed := now.Sub(records[j].Until)
					if elapsed <= graph.parent.recordAgeLimit {
						break
					}
				}
				if j > i {
					copy(records[i:], records[j:])
					newLen := len(records) - (j - i)
					for k := newLen; k < len(records); k++ {
						records[k] = nil
					}
					destGraph.timeline[key] = records[:newLen]
				}
				if len(destGraph.timeline[key]) == 0 {
					delete(destGraph.timeline, key)
				}
			}
			graph.parent.lastRevTrimming = now
		}
	}
}
