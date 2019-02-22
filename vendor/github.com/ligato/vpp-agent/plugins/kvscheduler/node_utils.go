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

package kvscheduler

import (
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/graph"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
	"github.com/gogo/protobuf/proto"
)

func nodesToKVPairsWithMetadata(nodes []graph.Node) (kvPairs []kvs.KVWithMetadata) {
	for _, node := range nodes {
		kvPairs = append(kvPairs, kvs.KVWithMetadata{
			Key:      node.GetKey(),
			Value:    node.GetValue(),
			Metadata: node.GetMetadata(),
			Origin:   getNodeOrigin(node),
		})
	}
	return kvPairs
}

// constructTargets builds targets for the graph based on derived values and dependencies.
func constructTargets(deps []kvs.Dependency, derives []kvs.KeyValuePair) (targets []graph.RelationTargetDef) {
	for _, dep := range deps {
		target := graph.RelationTargetDef{
			Relation: DependencyRelation,
			Label:    dep.Label,
			Key:      dep.Key,
			Selector: dep.AnyOf,
		}
		targets = append(targets, target)
	}

	for _, derived := range derives {
		target := graph.RelationTargetDef{
			Relation: DerivesRelation,
			Label:    derived.Key,
			Key:      derived.Key,
			Selector: nil,
		}
		targets = append(targets, target)
	}

	return targets
}

// equalValueDetails compares value state details for equality.
func equalValueDetails(details1, details2 []string) bool {
	if len(details1) != len(details2) {
		return false
	}
	for _, d1 := range details1 {
		found := false
		for _, d2 := range details2 {
			if d1 == d2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// getValueDetails returns further details about the value state.
func getValueDetails(node graph.Node) (details []string) {
	state := getNodeState(node)
	_, err := getNodeError(node)
	if state == kvs.ValueState_INVALID {
		if ivErr, isIVErr := err.(*kvs.InvalidValueError); isIVErr {
			details = ivErr.GetInvalidFields()
			return
		}
	}
	if state == kvs.ValueState_PENDING {
		for _, targets := range node.GetTargets(DependencyRelation) {
			satisfied := false
			for _, target := range targets.Nodes {
				if isNodeAvailable(target) {
					satisfied = true
				}
			}
			if !satisfied {
				details = append(details, targets.Label)
			}
		}
	}
	return details
}

// getValueStatus reads the value status from the corresponding node.
func getValueStatus(node graph.Node, key string) *kvs.BaseValueStatus {
	status := &kvs.BaseValueStatus{
		Value: &kvs.ValueStatus{
			Key: key,
		},
	}

	status.Value.State = getNodeState(node)
	if status.Value.State == kvs.ValueState_NONEXISTENT {
		// nothing else to get for non-existent value
		return status
	}
	_, err := getNodeError(node)
	if err != nil {
		status.Value.Error = err.Error()
	}
	status.Value.LastOperation = getNodeLastOperation(node)
	status.Value.State = getNodeState(node)
	status.Value.Details = getValueDetails(node)

	// derived nodes
	if !isNodeDerived(node) {
		for _, derivedNode := range getDerivedNodes(node) {
			derValStatus := getValueStatus(derivedNode, derivedNode.GetKey())
			status.DerivedValues = append(status.DerivedValues, derValStatus.Value)
		}
	}

	return status
}

// functions returns selectors selecting non-derived NB values.
func nbBaseValsSelectors() []graph.FlagSelector {
	return []graph.FlagSelector{
		graph.WithoutFlags(&DerivedFlag{}),
		graph.WithoutFlags(&ValueStateFlag{kvs.ValueState_RETRIEVED}),
	}
}

// functions returns selectors selecting non-derived SB values.
func sbBaseValsSelectors() []graph.FlagSelector {
	return []graph.FlagSelector{
		graph.WithoutFlags(&DerivedFlag{}),
		graph.WithFlags(&ValueStateFlag{kvs.ValueState_RETRIEVED}),
	}
}

// function returns selectors selecting values to be used for correlation for
// the Dump operation of the given descriptor.
func correlateValsSelectors(descriptor string) []graph.FlagSelector {
	return []graph.FlagSelector{
		graph.WithFlags(&DescriptorFlag{descriptor}),
		graph.WithoutFlags(&UnavailValueFlag{}, &DerivedFlag{}),
	}
}

// getNodeState returns state stored in the ValueState flag.
func getNodeState(node graph.Node) kvs.ValueState {
	if node != nil {
		flag := node.GetFlag(ValueStateFlagName)
		if flag != nil {
			return flag.(*ValueStateFlag).valueState
		}
	}
	return kvs.ValueState_NONEXISTENT
}

func valueStateToOrigin(state kvs.ValueState) kvs.ValueOrigin {
	switch state {
	case kvs.ValueState_NONEXISTENT:
		return kvs.UnknownOrigin
	case kvs.ValueState_RETRIEVED:
		return kvs.FromSB
	}
	return kvs.FromNB
}

// getNodeOrigin returns node origin based on the value state.
func getNodeOrigin(node graph.Node) kvs.ValueOrigin {
	state := getNodeState(node)
	return valueStateToOrigin(state)
}

// getNodeError returns node error stored in Error flag.
func getNodeError(node graph.Node) (retriable bool, err error) {
	if node != nil {
		errorFlag := node.GetFlag(ErrorFlagName)
		if errorFlag != nil {
			flag := errorFlag.(*ErrorFlag)
			return flag.retriable, flag.err
		}
	}
	return false, nil
}

// getNodeErrorString returns node error stored in Error flag as string.
func getNodeErrorString(node graph.Node) string {
	_, err := getNodeError(node)
	if err == nil {
		return ""
	}
	return err.Error()
}

// getNodeLastUpdate returns info about the last update for a given node, stored in LastUpdate flag.
func getNodeLastUpdate(node graph.Node) *LastUpdateFlag {
	if node == nil {
		return nil
	}
	flag := node.GetFlag(LastUpdateFlagName)
	if flag == nil {
		return nil
	}
	return flag.(*LastUpdateFlag)
}

// getNodeLastAppliedValue return the last applied value for the given node
func getNodeLastAppliedValue(node graph.Node) proto.Message {
	lastUpdate := getNodeLastUpdate(node)
	if lastUpdate == nil {
		return nil
	}
	return lastUpdate.value
}

// getNodeLastOperation returns last operation executed over the given node.
func getNodeLastOperation(node graph.Node) kvs.TxnOperation {
	if node != nil && getNodeState(node) != kvs.ValueState_RETRIEVED {
		lastUpdate := getNodeLastUpdate(node)
		if lastUpdate != nil {
			return lastUpdate.txnOp
		}
	}
	return kvs.TxnOperation_UNDEFINED
}

// getNodeDescriptor returns name of the descriptor associated with the given node.
// Empty for properties and unimplemented values.
func getNodeDescriptor(node graph.Node) string {
	if node == nil {
		return ""
	}
	flag := node.GetFlag(DescriptorFlagName)
	if flag == nil {
		return ""
	}
	return flag.(*DescriptorFlag).descriptorName
}

func isNodeDerived(node graph.Node) bool {
	return node.GetFlag(DerivedFlagName) != nil
}

func getNodeBaseKey(node graph.Node) string {
	flag := node.GetFlag(DerivedFlagName)
	if flag == nil {
		return node.GetKey()
	}
	return flag.(*DerivedFlag).baseKey
}

// isNodePending checks whether the node is available for dependency resolution.
func isNodeAvailable(node graph.Node) bool {
	if node == nil {
		return false
	}
	return node.GetFlag(UnavailValueFlagName) == nil
}

// isNodeReady return true if the given node has all dependencies satisfied.
// Recursive calls are needed to handle circular dependencies - nodes of a strongly
// connected component are treated as if they were squashed into one.
func isNodeReady(node graph.Node) bool {
	if getNodeOrigin(node) == kvs.FromSB {
		// for SB values dependencies are not checked
		return true
	}
	ready, _ := isNodeReadyRec(node, 0, make(map[string]int))
	return ready
}

// isNodeReadyRec is a recursive call from within isNodeReady.
// visited = map{ key -> depth }
func isNodeReadyRec(node graph.Node, depth int, visited map[string]int) (ready bool, cycleDepth int) {
	if targetDepth, wasVisited := visited[node.GetKey()]; wasVisited {
		return true, targetDepth
	}
	cycleDepth = depth
	visited[node.GetKey()] = depth
	defer delete(visited, node.GetKey())

	for _, targets := range node.GetTargets(DependencyRelation) {
		satisfied := false
		for _, target := range targets.Nodes {
			if isNodeAvailable(target) {
				satisfied = true
			}

			// test if node is inside a strongly-connected component (treated as one node)
			targetReady, targetCycleDepth := isNodeReadyRec(target, depth+1, visited)
			if targetReady && targetCycleDepth <= depth {
				// this node is reachable from the target
				satisfied = true
				if targetCycleDepth < cycleDepth {
					// update how far back in the branch this node can reach following dependencies
					cycleDepth = targetCycleDepth
				}
			}
		}
		if !satisfied {
			return false, cycleDepth
		}
	}
	return true, cycleDepth
}

func canNodeHaveMetadata(node graph.Node) bool {
	return !isNodeDerived(node)
}

func getDerivedNodes(node graph.Node) (derived []graph.Node) {
	for _, derivedNodes := range node.GetTargets(DerivesRelation) {
		derived = append(derived, derivedNodes.Nodes...)
	}
	return derived
}

func getDerivedKeys(node graph.Node) utils.KeySet {
	set := utils.NewSliceBasedKeySet()
	for _, derived := range getDerivedNodes(node) {
		set.Add(derived.GetKey())
	}
	return set
}
