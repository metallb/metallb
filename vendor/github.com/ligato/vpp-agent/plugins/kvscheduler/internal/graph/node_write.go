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
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
)

type node struct {
	*nodeR

	metaInSync     bool
	dataUpdated    bool
	targetsUpdated bool
	sourcesUpdated bool
}

// newNode creates a new instance of node, either built from the scratch or
// extending existing nodeR.
func newNode(nodeR *nodeR) *node {
	if nodeR == nil {
		return &node{
			nodeR:       newNodeR(),
			metaInSync:  true,
			dataUpdated: true, /* completely new node */
		}
	}
	return &node{
		nodeR:      nodeR,
		metaInSync: true,
	}
}

// SetLabel associates given label with this node.
func (node *node) SetLabel(label string) {
	node.label = label
	node.dataUpdated = true
}

// SetValue associates given value with this node.
func (node *node) SetValue(value proto.Message) {
	node.value = value
	node.dataUpdated = true
}

// SetFlags associates given flag with this node.
func (node *node) SetFlags(flags ...Flag) {
	toBeSet := make(map[string]struct{})
	for _, flag := range flags {
		toBeSet[flag.GetName()] = struct{}{}
	}

	var otherFlags []Flag
	for _, flag := range node.flags {
		if _, set := toBeSet[flag.GetName()]; !set {
			otherFlags = append(otherFlags, flag)
		}
	}

	node.flags = append(otherFlags, flags...)
	node.dataUpdated = true
}

// DelFlags removes given flag from this node.
func (node *node) DelFlags(names ...string) {
	var otherFlags []Flag
	for _, flag := range node.flags {
		delete := false
		for _, flagName := range names {
			if flag.GetName() == flagName {
				delete = true
				break
			}
		}
		if !delete {
			otherFlags = append(otherFlags, flag)
		}
	}

	node.flags = otherFlags
	node.dataUpdated = true
}

// SetMetadataMap chooses metadata map to be used to store the association
// between this node's value label and metadata.
func (node *node) SetMetadataMap(mapName string) {
	if node.metadataMap == "" { // cannot be changed
		node.metadataMap = mapName
		node.dataUpdated = true
		node.metaInSync = false
	}
}

// SetMetadata associates given value metadata with this node.
func (node *node) SetMetadata(metadata interface{}) {
	node.metadata = metadata
	node.dataUpdated = true
	node.metaInSync = false
}

// SetTargets provides definition of all edges pointing from this node.
func (node *node) SetTargets(targetsDef []RelationTargetDef) {
	node.targetsDef = targetsDef
	node.dataUpdated = true

	// remove obsolete targets
	for _, relTargets := range node.targets {
		for labelIdx := 0; labelIdx < len(relTargets.Targets); {
			targets := relTargets.Targets[labelIdx]

			// collect keys to remove for this relation+label
			var toRemove []string
			for _, target := range targets.MatchingKeys.Iterate() {
				obsolete := true
				targetDefs := node.getTargetDefsForKey(target, relTargets.Relation)
				for _, targetDef := range targetDefs {
					if targetDef.Label == targets.Label {
						obsolete = false
						break
					}
				}
				if len(targetDefs) == 0 {
					// this is no longer target for any label of this relation
					targetNode := node.graph.nodes[target]
					targetNode.removeFromSources(relTargets.Relation, node.GetKey())
				}
				if obsolete {
					toRemove = append(toRemove, target)
				}
			}

			// remove the entire label if it is no longer defined
			obsoleteLabel := true
			for _, targetDef := range node.targetsDef {
				if targetDef.Relation == relTargets.Relation &&
					targetDef.Label == targets.Label {
					obsoleteLabel = false
					break
				}
			}
			if obsoleteLabel {
				newLen := len(relTargets.Targets) - 1
				copy(relTargets.Targets[labelIdx:], relTargets.Targets[labelIdx+1:])
				relTargets.Targets = relTargets.Targets[:newLen]
			} else {
				// remove just obsolete targets, not the entire label
				for _, target := range toRemove {
					targets.MatchingKeys.Del(target)
				}
				labelIdx++
			}
		}
	}

	// build new targets
	var usesSelector bool
	for _, targetDef := range node.targetsDef {
		node.createEntryForTarget(targetDef)
		if targetDef.Key != "" {
			// without selectors, the lookup procedure has complexity O(m*log(n))
			// where n = number of nodes; m = number of edges defined for this node
			if node2, hasTarget := node.graph.nodes[targetDef.Key]; hasTarget {
				node.addToTargets(node2, targetDef)
			}
		} else {
			usesSelector = true // have to use the less efficient O(mn) lookup
		}
	}
	if usesSelector {
		for _, otherNode := range node.graph.nodes {
			if otherNode.key == node.key {
				continue
			}
			node.checkPotentialTarget(otherNode)
		}
	}
}

// checkPotentialTarget checks if node2 is target of node in any of the relations.
func (node *node) checkPotentialTarget(node2 *node) {
	targetDefs := node.getTargetDefsForKey(node2.key, "") // for any relation
	for _, targetDef := range targetDefs {
		node.addToTargets(node2, targetDef)
	}
}

// getTargetDefsForKey returns all target definitions that select the given key.
// Target definitions can be further filtered by the relation.
func (node *node) getTargetDefsForKey(key, relation string) (defs []RelationTargetDef) {
	for _, targetDef := range node.targetsDef {
		if relation != "" && targetDef.Relation != relation {
			continue
		}
		if targetDef.Key == key ||
			(targetDef.Key == "" && targetDef.Selector(key)) {
			defs = append(defs, targetDef)
		}
	}
	return defs
}

// createEntryForTarget creates entry for target(s) with the given definition
// if it does not exist yet.
func (node *node) createEntryForTarget(targetDef RelationTargetDef) {
	relTargets := node.targets.GetTargetsForRelation(targetDef.Relation)
	if relTargets == nil {
		// new relation
		relTargets = &RelationTargets{Relation: targetDef.Relation}
		node.targets = append(node.targets, relTargets)
	}
	targets := relTargets.GetTargetsForLabel(targetDef.Label)
	if targets == nil {
		// new relation label
		targets = &Targets{Label: targetDef.Label, ExpectedKey: targetDef.Key}
		if targetDef.Key != "" {
			targets.MatchingKeys = utils.NewSingletonKeySet("")
		} else {
			// selector
			targets.MatchingKeys = utils.NewSliceBasedKeySet()
		}
		relTargets.Targets = append(relTargets.Targets, targets)
	}
	targets.ExpectedKey = targetDef.Key
}

// addToTargets adds node2 into the set of targets for this node. Sources of node2
// are also updated accordingly.
func (node *node) addToTargets(node2 *node, targetDef RelationTargetDef) {
	// update targets of node
	relTargets := node.targets.GetTargetsForRelation(targetDef.Relation)
	targets := relTargets.GetTargetsForLabel(targetDef.Label)
	node.targetsUpdated = targets.MatchingKeys.Add(node2.key) || node.targetsUpdated

	// update sources of node2
	relSources := node2.sources.getSourcesForRelation(targetDef.Relation)
	if relSources == nil {
		relSources = &relationSources{
			relation: targetDef.Relation,
			sources:  utils.NewSliceBasedKeySet(),
		}
		node2.sources = append(node2.sources, relSources)
	}
	node2.sourcesUpdated = relSources.sources.Add(node.key) || node2.sourcesUpdated
}

// removeFromTargets removes given key from the set of targets.
func (node *node) removeFromTargets(key string) {
	for _, relTargets := range node.targets {
		for _, targets := range relTargets.Targets {
			node.targetsUpdated = targets.MatchingKeys.Del(key) || node.targetsUpdated
		}
	}
}

// removeFromTargets removes this node from the set of sources of all the other nodes.
func (node *node) removeThisFromSources() {
	for _, relTargets := range node.targets {
		for _, targets := range relTargets.Targets {
			for _, key := range targets.MatchingKeys.Iterate() {
				targetNode := node.graph.nodes[key]
				targetNode.removeFromSources(relTargets.Relation, node.GetKey())
			}
		}
	}
}

// removeFromSources removes given key from the sources for the given relation.
func (node *node) removeFromSources(relation string, key string) {
	updated := node.sources.getSourcesForRelation(relation).sources.Del(key)
	node.sourcesUpdated = updated || node.sourcesUpdated
}
