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
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ligato/cn-infra/idxmap"

	. "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
)

// printDelimiter is used in pretty-printing of the graph.
const printDelimiter = ", "

// graphR implements ReadAccess.
type graphR struct {
	parent   *kvgraph
	nodes    map[string]*node
	mappings map[string]idxmap.NamedMappingRW
	timeline map[string][]*RecordedNode // key -> node records (from the oldest to the newest)
}

// newGraphR creates and initializes a new instance of graphR.
func newGraphR() *graphR {
	return &graphR{
		nodes:    make(map[string]*node),
		mappings: make(map[string]idxmap.NamedMappingRW),
		timeline: make(map[string][]*RecordedNode),
	}
}

// GetMetadataMap returns registered metadata map.
func (graph *graphR) GetMetadataMap(mapName string) idxmap.NamedMapping {
	metadataMap, has := graph.mappings[mapName]
	if !has {
		return nil
	}
	return metadataMap
}

// GetNode returns node with the given key or nil if the key is unused.
func (graph *graphR) GetNode(key string) Node {
	node, has := graph.nodes[key]
	if !has {
		return nil
	}
	return node.nodeR
}

// GetNodes returns a set of nodes matching the key selector (can be nil)
// and every provided flag selector.
func (graph *graphR) GetNodes(keySelector KeySelector, flagSelectors ...FlagSelector) (nodes []Node) {
	for key, node := range graph.nodes {
		if keySelector != nil && !keySelector(key) {
			continue
		}
		selected := true
		for _, flagSelector := range flagSelectors {
			for _, flag := range flagSelector.flags {
				hasFlag := false
				for _, nodeFlag := range node.flags {
					if nodeFlag.GetName() == flag.GetName() &&
						(flag.GetValue() == "" || (nodeFlag.GetValue() == flag.GetValue())) {
						hasFlag = true
						break
					}
				}
				if hasFlag != flagSelector.with {
					selected = false
					break
				}
			}
			if !selected {
				break
			}
		}
		if !selected {
			continue
		}
		nodes = append(nodes, node.nodeR)
	}
	return nodes
}

// GetNodeTimeline returns timeline of all node revisions, ordered from
// the oldest to the newest.
func (graph *graphR) GetNodeTimeline(key string) []*RecordedNode {
	timeline, has := graph.timeline[key]
	if !has {
		return nil
	}
	return timeline
}

// GetFlagStats returns stats for a given flag.
func (graph *graphR) GetFlagStats(flagName string, selector KeySelector) FlagStats {
	stats := FlagStats{PerValueCount: make(map[string]uint)}

	for key, timeline := range graph.timeline {
		if selector != nil && !selector(key) {
			continue
		}
		for _, record := range timeline {
			if record.TargetUpdateOnly {
				continue
			}
			if flag := record.Flags.GetFlag(flagName); flag != nil {
				flagValue := flag.GetValue()
				stats.TotalCount++
				if _, hasValue := stats.PerValueCount[flagValue]; !hasValue {
					stats.PerValueCount[flagValue] = 0
				}
				stats.PerValueCount[flagValue]++
			}
		}
	}

	return stats
}

// GetSnapshot returns the snapshot of the graph at a given time.
func (graph *graphR) GetSnapshot(time time.Time) (nodes []*RecordedNode) {
	for _, timeline := range graph.timeline {
		for _, record := range timeline {
			if record.Since.Before(time) &&
				(record.Until.IsZero() || record.Until.After(time)) {
				nodes = append(nodes, record)
				break
			}
		}
	}
	return nodes
}

// GetKeys returns sorted keys.
func (graph *graphR) GetKeys() []string {
	var keys []string
	for key := range graph.nodes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

// Dump returns a human-readable string representation of the current graph
// content for debugging purposes.
func (graph *graphR) Dump() string {
	// order nodes by keys
	var keys []string
	for key := range graph.nodes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	var buf strings.Builder
	graphInfo := fmt.Sprintf("%d nodes", len(keys))
	buf.WriteString("+======================================================================================================================+\n")
	buf.WriteString(fmt.Sprintf("| GRAPH DUMP %105s |\n", graphInfo))
	buf.WriteString("+======================================================================================================================+\n")

	for i, key := range keys {
		node := graph.nodes[key]

		buf.WriteString(fmt.Sprintf("| Key: %111q |\n", key))
		if label := node.GetLabel(); label != key {
			buf.WriteString(fmt.Sprintf("| Label: %109s |\n", label))
		}
		buf.WriteString(fmt.Sprintf("| Value: %109s |\n", utils.ProtoToString(node.GetValue())))
		buf.WriteString(fmt.Sprintf("| Flags: %109v |\n", prettyPrintFlags(node.flags)))
		if len(node.targets) > 0 {
			buf.WriteString(fmt.Sprintf("| Targets: %107v |\n", prettyPrintTargets(node.targets)))
		}
		if len(node.sources) > 0 {
			buf.WriteString(fmt.Sprintf("| Sources: %107v |\n", prettyPrintSources(node.sources)))
		}
		if metadata := graph.getMetadataFields(node); len(metadata) > 0 {
			buf.WriteString(fmt.Sprintf("| Metadata: %106v |\n", metadata))
		}
		if i+1 != len(keys) {
			buf.WriteString("+----------------------------------------------------------------------------------------------------------------------+\n")
		}
	}
	buf.WriteString("+----------------------------------------------------------------------------------------------------------------------+\n")

	return buf.String()
}

// Release releases the graph handle (both Read() & Write() should end with
// release).
func (graph *graphR) Release() {
	graph.parent.rwLock.RUnlock()
}

// copyNodesOnly returns a deep-copy of the graph, excluding the timelines
// and the map with mappings.
func (graph *graphR) copyNodesOnly() *graphR {
	graphCopy := &graphR{
		parent: graph.parent,
		nodes:  make(map[string]*node),
	}
	for key, node := range graph.nodes {
		nodeCopy := node.copy()
		nodeCopy.graph = graphCopy
		graphCopy.nodes[key] = newNode(nodeCopy)
	}
	return graphCopy
}

// recordNode builds a record for the node to be added into the timeline.
func (graph *graphR) recordNode(node *node, targetUpdateOnly bool) *RecordedNode {
	record := &RecordedNode{
		Since:            time.Now(),
		Key:              node.key,
		Label:            node.label,
		Value:            utils.RecordProtoMessage(node.value),
		Flags:            RecordedFlags{Flags: node.flags},
		MetadataFields:   graph.getMetadataFields(node), // returned already copied
		Targets:          node.targets,                  // no need to copy, never changed in graphR
		TargetUpdateOnly: targetUpdateOnly,
	}
	return record
}

// getMetadataFields returns secondary fields from metadata attached to the given node.
func (graph *graphR) getMetadataFields(node *node) map[string][]string {
	writeCopy := graph.parent.graph != graph
	if !writeCopy && node.metadataAdded {
		mapping := graph.mappings[node.metadataMap]
		return mapping.ListFields(node.label)
	}
	return nil
}

// prettyPrintFlags returns nicely formatted string representation of the given list of flags.
func prettyPrintFlags(flags []Flag) string {
	var str string
	for idx, flag := range flags {
		if flag.GetValue() == "" {
			str += flag.GetName()
		} else {
			str += fmt.Sprintf("%s:<%s>", flag.GetName(), flag.GetValue())
		}
		if idx < len(flags)-1 {
			str += printDelimiter
		}
	}
	return str
}

// prettyPrintTargets returns nicely formatted relation targets.
func prettyPrintTargets(targets TargetsByRelation) string {
	if len(targets) == 0 {
		return "<NONE>"
	}
	var str string
	idx := 0
	for _, relation := range targets {
		str += fmt.Sprintf("[%s]{%s}", relation.Relation, prettyPrintEdges(relation.Targets))
		if idx < len(targets)-1 {
			str += printDelimiter
		}
		idx++
	}
	return str
}

// prettyPrintSources returns nicely formatted relation sources.
func prettyPrintSources(sources []*relationSources) string {
	if len(sources) == 0 {
		return "<NONE>"
	}
	var str string
	idx := 0
	for _, relSources := range sources {
		str += fmt.Sprintf("[%s]%s", relSources.relation, relSources.sources.String())
		if idx < len(sources)-1 {
			str += printDelimiter
		}
		idx++
	}
	return str
}

// prettyPrintEdges returns nicely formatted node edges.
func prettyPrintEdges(edges TargetsByLabel) string {
	var str string
	idx := 0
	for _, edge := range edges {
		if edge.MatchingKeys.Length() == 1 && edge.MatchingKeys.Has(edge.Label) {
			// special case: there 1:1 between label and the key
			str += edge.Label
		} else {
			str += edge.Label + " -> " + edge.MatchingKeys.String()
		}
		if idx < len(edges)-1 {
			str += printDelimiter
		}
		idx++
	}
	return str
}
