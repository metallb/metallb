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

	"github.com/gogo/protobuf/proto"

	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
)

// nodeR implements Node.
type nodeR struct {
	graph *graphR

	key           string
	label         string
	value         proto.Message
	flags         []Flag
	metadata      interface{}
	metadataAdded bool
	metadataMap   string
	targetsDef    []RelationTargetDef
	targets       TargetsByRelation
	sources       sourcesByRelation
}

// relationSources groups all sources for a single relation.
type relationSources struct {
	relation string
	sources  utils.KeySet
}

// sourcesByRelation is a slice of all sources, grouped by relations.
type sourcesByRelation []*relationSources

// String returns human-readable string representation of sourcesByRelation.
func (s sourcesByRelation) String() string {
	str := "{"
	for idx, sources := range s {
		if idx > 0 {
			str += ", "
		}
		str += fmt.Sprintf("%s->%s", sources.relation, sources.sources.String())
	}
	str += "}"
	return str
}

// getSourcesForRelation returns sources (keys) for the given relation.
func (s sourcesByRelation) getSourcesForRelation(relation string) *relationSources {
	for _, relSources := range s {
		if relSources.relation == relation {
			return relSources
		}
	}
	return nil
}

// newNodeR creates a new instance of nodeR.
func newNodeR() *nodeR {
	return &nodeR{}
}

// GetKey returns the key associated with the node.
func (node *nodeR) GetKey() string {
	return node.key
}

// GetLabel returns the label associated with this node.
func (node *nodeR) GetLabel() string {
	return node.label
}

// GetKey returns the value associated with the node.
func (node *nodeR) GetValue() proto.Message {
	return node.value
}

// GetFlag returns reference to the given flag or nil if the node doesn't have
// this flag associated.
func (node *nodeR) GetFlag(name string) Flag {
	for _, flag := range node.flags {
		if flag.GetName() == name {
			return flag
		}
	}
	return nil
}

// GetMetadata returns the value metadata associated with the node.
func (node *nodeR) GetMetadata() interface{} {
	return node.metadata
}

// GetTargets returns a set of nodes, indexed by relation labels, that the
// edges of the given relation points to.
func (node *nodeR) GetTargets(relation string) (runtimeTargets RuntimeTargetsByLabel) {
	relTargets := node.targets.GetTargetsForRelation(relation)
	if relTargets == nil {
		return nil
	}
	for _, targets := range relTargets.Targets {
		var nodes []Node
		for _, key := range targets.MatchingKeys.Iterate() {
			nodes = append(nodes, node.graph.nodes[key])
		}
		runtimeTargets = append(runtimeTargets, &RuntimeTargets{
			Label: targets.Label,
			Nodes: nodes,
		})
	}
	return runtimeTargets
}


// GetSources returns a set of nodes with edges of the given relation
// pointing to this node.
func (node *nodeR) GetSources(relation string) (nodes []Node) {
	relSources := node.sources.getSourcesForRelation(relation)
	if relSources == nil {
		return nil
	}

	for _, key := range relSources.sources.Iterate() {
		nodes = append(nodes, node.graph.nodes[key])
	}
	return nodes
}

// copy returns a deep copy of the node.
func (node *nodeR) copy() *nodeR {
	nodeCopy := newNodeR()
	nodeCopy.key = node.key
	nodeCopy.label = node.label
	nodeCopy.value = node.value
	nodeCopy.metadata = node.metadata
	nodeCopy.metadataAdded = node.metadataAdded
	nodeCopy.metadataMap = node.metadataMap

	// shallow-copy flags (immutable)
	nodeCopy.flags = node.flags

	// shallow-copy target definitions (immutable)
	nodeCopy.targetsDef = node.targetsDef

	// copy targets
	nodeCopy.targets = make(TargetsByRelation, 0, len(node.targets))
	for _, relTargets := range node.targets {
		targets := make(TargetsByLabel, 0, len(relTargets.Targets))
		for _, target := range relTargets.Targets {
			targets = append(targets, &Targets{
				Label:        target.Label,
				ExpectedKey:  target.ExpectedKey,
				MatchingKeys: target.MatchingKeys.CopyOnWrite(),
			})
		}
		nodeCopy.targets = append(nodeCopy.targets, &RelationTargets{
			Relation: relTargets.Relation,
			Targets:  targets,
		})
	}

	// copy sources
	nodeCopy.sources = make(sourcesByRelation, 0, len(node.sources))
	for _, relSources := range node.sources {
		nodeCopy.sources = append(nodeCopy.sources, &relationSources{
			relation: relSources.relation,
			sources:  relSources.sources.CopyOnWrite(),
		})
	}
	return nodeCopy
}
