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
	"fmt"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/graph"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
)

const (
	nodeVisitBeginMark = "[BEGIN]"
	nodeVisitEndMark   = "[END]"
)

// resyncData stores data to be used for resync after refresh.
type resyncData struct {
	first  bool // true if startup-resync
	values []kvForTxn
}

// refreshGraph updates all/some values in the graph to their *real* state
// using the Dump methods from descriptors.
func (s *Scheduler) refreshGraph(graphW graph.RWAccess, keys utils.KeySet, resyncData *resyncData, verbose bool) {
	if s.logGraphWalk {
		keysToRefresh := "<ALL>"
		if keys != nil && keys.Length() > 0 {
			keysToRefresh = keys.String()
		}
		msg := fmt.Sprintf("refreshGrap (keys=%s)", keysToRefresh)
		fmt.Printf("%s %s\n", nodeVisitBeginMark, msg)
		defer fmt.Printf("%s %s\n", nodeVisitEndMark, msg)
	}
	refreshedKeys := utils.NewMapBasedKeySet()

	// iterate over all descriptors, in order given by dump dependencies
	for _, descriptor := range s.registry.GetAllDescriptors() {
		handler := &descriptorHandler{descriptor}

		// check if this descriptor's key space should be refreshed as well
		var skip bool
		if keys != nil {
			skip = keys.Length() > 0
			for _, key := range keys.Iterate() {
				if descriptor.KeySelector(key) {
					skip = false
					break
				}
			}
		}
		if skip {
			// nothing to refresh in the key space of this descriptor
			s.skipRefresh(graphW, descriptor.Name, nil, refreshedKeys)
			continue
		}

		// get available base values for this descriptor from memory before
		// refresh
		prevAvailNodes := graphW.GetNodes(nil, correlateValsSelectors(descriptor.Name)...)

		// get key-value pairs for correlation
		var correlate []kvs.KVWithMetadata
		if resyncData != nil && resyncData.first {
			// for startup resync, use data received from NB
			for _, kv := range resyncData.values {
				if descriptor.KeySelector(kv.key) {
					correlate = append(correlate,
						kvs.KVWithMetadata{
							Key:    kv.key,
							Value:  kv.value,
							Origin: kv.origin,
						})
				}
			}
		} else {
			// for refresh of failed values or run-time resync, use in-memory
			// kv-pairs for correlation
			correlate = nodesToKVPairsWithMetadata(prevAvailNodes)
		}

		// execute Dump operation
		dump, ableToDump, err := handler.dump(correlate)

		// mark un-dumpable as refreshed
		if !ableToDump || err != nil {
			if err != nil {
				s.Log.WithField("descriptor", descriptor.Name).
					Error("failed to dump values, refresh for the descriptor will be skipped")
			}
			s.skipRefresh(graphW, descriptor.Name, nil, refreshedKeys)
			continue
		} else if verbose {
			plural := "s"
			if len(dump) == 1 {
				plural = ""
			}

			var dumpList string
			for _, d := range dump {
				dumpList += fmt.Sprintf("\n - %+v", d)
			}

			s.Log.Debugf("Descriptor %s dumped %d item%s: %v",
				descriptor.Name, len(dump), plural, dumpList)

		}

		if keys != nil && keys.Length() > 0 {
			// mark keys that should not be touched as refreshed
			s.skipRefresh(graphW, descriptor.Name, keys, refreshedKeys)
		}

		// process dumped kv-pairs
		for _, dumpedKV := range dump {
			if keys != nil && keys.Length() > 0 {
				// do no touch values that aren't meant to be refreshed
				if toRefresh := keys.Has(dumpedKV.Key); !toRefresh {
					continue
				}
			}
			if !s.validDumpedKV(dumpedKV, descriptor, refreshedKeys) {
				continue
			}

			// 1st attempt to determine value origin
			if dumpedKV.Origin == kvs.UnknownOrigin {
				// determine value origin based on the values for correlation
				for _, kv := range correlate {
					if kv.Key == dumpedKV.Key {
						dumpedKV.Origin = kv.Origin
						break
					}
				}
			}

			// 2nd attempt to determine value origin
			if dumpedKV.Origin == kvs.UnknownOrigin {
				// determine value origin based on the last revision
				timeline := graphW.GetNodeTimeline(dumpedKV.Key)
				if len(timeline) > 0 {
					lastRev := timeline[len(timeline)-1]
					valueStateFlag := lastRev.Flags.GetFlag(ValueStateFlagName)
					valueState := valueStateFlag.(*ValueStateFlag).valueState
					dumpedKV.Origin = valueStateToOrigin(valueState)
				}
			}

			if dumpedKV.Origin == kvs.UnknownOrigin {
				// will assume this is from SB
				dumpedKV.Origin = kvs.FromSB
			}

			// refresh node that represents this kv-pair
			s.refreshValue(graphW, dumpedKV, handler, refreshedKeys, 2)
		}

		// unset the metadata from base NB values that do not actually exists
		for _, node := range prevAvailNodes {
			if refreshed := refreshedKeys.Has(node.GetKey()); !refreshed {
				if getNodeOrigin(node) == kvs.FromNB {
					if s.logGraphWalk {
						fmt.Printf("  -> unset metadata for key=%s\n", node.GetKey())
					}
					missingNode := graphW.SetNode(node.GetKey())
					missingNode.SetMetadata(nil)
				}
			}
		}

		// in-progress save to expose changes in the metadata for dumps of the following
		// descriptors
		graphW.Save()
	}

	// update state of values that do not actually exist
	for _, node := range graphW.GetNodes(nil) {
		if refreshed := refreshedKeys.Has(node.GetKey()); refreshed {
			continue
		}
		s.refreshUnavailNode(graphW, node, refreshedKeys, 2)
	}

	if verbose {
		fmt.Println(dumpGraph(graphW))
	}
}

// refreshValue refreshes node that represents the given dumped key-value pair.
func (s *Scheduler) refreshValue(graphW graph.RWAccess, dumpedKV kvs.KVWithMetadata,
	handler *descriptorHandler, refreshed utils.KeySet, indent int) {
	if s.logGraphWalk {
		indentStr := strings.Repeat(" ", indent)
		msg := fmt.Sprintf("refreshValue (key=%s)", dumpedKV.Key)
		fmt.Printf("%s%s %s\n", indentStr, nodeVisitBeginMark, msg)
		defer fmt.Printf("%s%s %s\n", indentStr, nodeVisitEndMark, msg)
	}

	// refresh node that represents this kv-pair
	node := graphW.SetNode(dumpedKV.Key)
	node.SetLabel(handler.keyLabel(node.GetKey()))
	node.SetValue(dumpedKV.Value)
	if handler.descriptor.WithMetadata {
		node.SetMetadataMap(handler.descriptor.Name)
		node.SetMetadata(dumpedKV.Metadata)
	}
	s.refreshAvailNode(graphW, node, dumpedKV.Origin, false, node.GetKey(), refreshed, indent+2)

	// determine the set of unavailable derived values
	obsolete := getDerivedKeys(node)
	derives := handler.derivedValues(node.GetKey(), node.GetValue())
	for _, newDerived := range derives {
		obsolete.Del(newDerived.Key)
	}

	// keep obsolete derived values still in the relation
	for _, key := range obsolete.Iterate() {
		derives = append(derives, kvs.KeyValuePair{Key: key}) // value unused
	}

	// refresh relations
	dependencies := handler.dependencies(node.GetKey(), node.GetValue())
	node.SetTargets(constructTargets(dependencies, derives))

	// refresh derived values
	for _, kv := range derives {
		isObsolete := obsolete.Has(kv.Key)
		derNode := graphW.SetNode(kv.Key)
		if !isObsolete {
			derDescr := s.registry.GetDescriptorForKey(kv.Key)
			derHandler := descriptorHandler{derDescr}
			derNode.SetValue(kv.Value)
			dependencies := derHandler.dependencies(derNode.GetKey(), derNode.GetValue())
			derNode.SetTargets(constructTargets(dependencies, nil))
			s.refreshAvailNode(graphW, derNode, dumpedKV.Origin, true, node.GetKey(), refreshed, indent+2)
		} else {
			s.refreshUnavailNode(graphW, derNode, refreshed, indent+2)
		}
	}
}

// refreshAvailNode refreshes state of a node whose value was returned by Dump.
func (s *Scheduler) refreshAvailNode(graphW graph.RWAccess, node graph.NodeRW,
	origin kvs.ValueOrigin, derived bool, baseKey string, refreshed utils.KeySet, indent int) {
	if s.logGraphWalk {
		indentStr := strings.Repeat(" ", indent)
		msg := fmt.Sprintf("refreshAvailNode (key=%s, isDerived=%t)", node.GetKey(), derived)
		fmt.Printf("%s%s %s\n", indentStr, nodeVisitBeginMark, msg)
		defer fmt.Printf("%s%s %s\n", indentStr, nodeVisitEndMark, msg)
	}

	// validate first
	descriptor := s.registry.GetDescriptorForKey(node.GetKey()) // nil for properties
	if derived && !s.validDumpedDerivedKV(node, descriptor, refreshed) {
		graphW.DeleteNode(node.GetKey())
		return
	}

	// update availability
	if !isNodeAvailable(node) {
		s.updatedStates.Add(baseKey)
		node.DelFlags(UnavailValueFlagName)
	}
	refreshed.Add(node.GetKey())

	// refresh state
	if getNodeState(node) == kvs.ValueState_NONEXISTENT {
		// newly found node
		if origin == kvs.FromSB {
			s.refreshNodeState(node, kvs.ValueState_RETRIEVED, indent)
		} else {
			s.refreshNodeState(node, kvs.ValueState_FOUND, indent)
		}
	}
	if getNodeState(node) == kvs.ValueState_PENDING {
		// no longer pending apparently
		s.refreshNodeState(node, kvs.ValueState_CONFIGURED, indent)
	}

	// update descriptor flag
	if descriptor != nil {
		node.SetFlags(&DescriptorFlag{descriptor.Name})
	} else {
		node.DelFlags(DescriptorFlagName)
	}

	// updated flags for derived values
	if !derived {
		node.DelFlags(DerivedFlagName)
	} else {
		node.SetFlags(&DerivedFlag{baseKey})
	}
}

// refreshUnavailNode refreshes state of a node whose value is found to be unavailable.
func (s *Scheduler) refreshUnavailNode(graphW graph.RWAccess, node graph.Node, refreshed utils.KeySet, indent int) {
	if s.logGraphWalk {
		indentStr := strings.Repeat(" ", indent)
		msg := fmt.Sprintf("refreshUnavailNode (key=%s, isDerived=%t)", node.GetKey(), isNodeDerived(node))
		fmt.Printf("%s%s %s\n", indentStr, nodeVisitBeginMark, msg)
		defer fmt.Printf("%s%s %s\n", indentStr, nodeVisitEndMark, msg)
	}

	refreshed.Add(node.GetKey())
	if isNodeAvailable(node) {
		s.updatedStates.Add(getNodeBaseKey(node))
	}
	state := getNodeState(node)
	if getNodeOrigin(node) == kvs.FromSB || state == kvs.ValueState_FOUND {
		// just remove from the graph
		graphW.DeleteNode(node.GetKey())
		return
	}

	// mark node as unavailable, but do not delete
	nodeW := graphW.SetNode(node.GetKey())
	if isNodeAvailable(node) {
		nodeW.SetFlags(&UnavailValueFlag{})
	}

	// update state
	if state == kvs.ValueState_UNIMPLEMENTED {
		// it is expected that unimplemented value is not dumped
		return
	}
	if state == kvs.ValueState_CONFIGURED {
		if getNodeLastUpdate(node).value == nil {
			s.refreshNodeState(nodeW, kvs.ValueState_REMOVED, indent)
		} else {
			s.refreshNodeState(nodeW, kvs.ValueState_MISSING, indent)
		}
	}
}

func (s *Scheduler) refreshNodeState(node graph.NodeRW, newState kvs.ValueState, indent int) {
	if getNodeState(node) != newState {
		if s.logGraphWalk {
			fmt.Printf("%s  -> change value state from %v to %v\n",
				strings.Repeat(" ", indent), getNodeState(node), newState)
		}
		node.SetFlags(&ValueStateFlag{valueState: newState})
	}
}

// skipRefresh is used to mark nodes as refreshed without actual refreshing
// if they should not (or cannot) be refreshed.
func (s *Scheduler) skipRefresh(graphR graph.ReadAccess, descriptor string, except utils.KeySet, refreshed utils.KeySet) {
	skipped := graphR.GetNodes(nil,
		graph.WithFlags(&DescriptorFlag{descriptor}),
		graph.WithoutFlags(&DerivedFlag{}))
	for _, node := range skipped {
		if except != nil {
			if toRefresh := except.Has(node.GetKey()); toRefresh {
				continue
			}
		}
		refreshed.Add(node.GetKey())

		// skip refresh for derived nodes
		for _, derivedNode := range getDerivedNodes(node) {
			refreshed.Add(derivedNode.GetKey())
		}
	}
}

func dumpGraph(g graph.RWAccess) string {
	keys := g.GetKeys()

	var buf strings.Builder
	graphInfo := fmt.Sprintf("%d nodes", len(keys))
	buf.WriteString("+======================================================================================================================+\n")
	buf.WriteString(fmt.Sprintf("| GRAPH DUMP %105s |\n", graphInfo))
	buf.WriteString("+======================================================================================================================+\n")
	writeLine := func(left, right string) {
		n := 115 - len(left)
		buf.WriteString(fmt.Sprintf("| %s %"+fmt.Sprint(n)+"s |\n", left, right))

	}
	writeLines := func(linesStr string, prefix string) {
		lines := strings.Split(linesStr, "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			writeLine(fmt.Sprintf("%s%s", prefix, line), "")
		}
	}
	for i, key := range keys {
		node := g.GetNode(key)
		keyLabel := key
		if label := node.GetLabel(); label != key && label != "" {
			keyLabel = fmt.Sprintf("%s (%s)", key, label)
		}
		descriptor := ""
		if f := node.GetFlag(DescriptorFlagName); f != nil {
			descriptor = fmt.Sprintf("[%s] ", f.GetValue())
		}
		lastUpdate := "-"
		if f := node.GetFlag(LastUpdateFlagName); f != nil {
			lastUpdate = f.GetValue()
		}
		unavailable := ""
		if f := node.GetFlag(UnavailValueFlagName); f != nil {
			unavailable = "<UNAVAILABLE> "
		}
		writeLine(fmt.Sprintf("%s%s", descriptor, keyLabel), fmt.Sprintf("%s %s %s",
			unavailable,
			lastUpdate,
			getNodeState(node).String(),
		))
		writeLines(proto.MarshalTextString(node.GetValue()), "  ")

		if f := node.GetTargets(DependencyRelation); f != nil && len(f) > 0 {
			writeLine("Depends on:", "")
			for _, dep := range f {
				var nodeDeps []string
				for _, node := range dep.Nodes {
					nodeDeps = append(nodeDeps, node.GetKey())
				}
				if len(nodeDeps) > 1 {
					writeLine(fmt.Sprintf(" - %s", dep.Label), "")
					writeLines(strings.Join(nodeDeps, "\n"), "  -> ")
				} else if len(nodeDeps) == 1 {
					writeLine(fmt.Sprintf(" - %s -> %v", dep.Label, strings.Join(nodeDeps, " ")), "")
				} else {
					writeLine(fmt.Sprintf(" - %s -> <UNAVAILABLE>", dep.Label), "")
				}
			}
		}
		if f := node.GetTargets(DerivesRelation); f != nil && len(f) > 0 {
			writeLine("Derives:", "")
			var nodeDers []string
			for _, der := range f {
				if len(der.Nodes) == 0 {
					nodeDers = append(nodeDers, fmt.Sprintf("%s", der.Label))
				} else {
					for _, node := range der.Nodes {
						desc := ""
						if d := node.GetFlag(DescriptorFlagName); d != nil {
							desc = fmt.Sprintf("[%s] ", d.GetValue())
						}
						nodeDers = append(nodeDers, fmt.Sprintf("%s%s", desc, node.GetKey()))
					}
				}
			}
			writeLines(strings.Join(nodeDers, "\n"), " - ")
		}
		if f := node.GetSources(DependencyRelation); len(f) > 0 {
			writeLine("Dependency for:", "")
			var nodeDeps []string
			for _, node := range f {
				desc := ""
				if d := node.GetFlag(DescriptorFlagName); d != nil {
					desc = fmt.Sprintf("[%s] ", d.GetValue())
				}
				nodeDeps = append(nodeDeps, fmt.Sprintf("%s%s", desc, node.GetKey()))
			}
			writeLines(strings.Join(nodeDeps, "\n"), " - ")
		}
		if f := node.GetSources(DerivesRelation); len(f) > 0 {
			var nodeDers []string
			for _, der := range f {
				nodeDers = append(nodeDers, der.GetKey())
			}
			writeLine(fmt.Sprintf("Derived from: %s", strings.Join(nodeDers, " ")), "")
		}
		if f := node.GetMetadata(); f != nil {
			writeLine(fmt.Sprintf("Metadata: %+v", f), "")
		}
		if f := node.GetFlag(ErrorFlagName); f != nil {
			writeLine(fmt.Sprintf("Errors: %+v", f.GetValue()), "")
		}

		if i+1 != len(keys) {
			buf.WriteString("+======================================================================================================================+\n")
		}
	}
	buf.WriteString("+======================================================================================================================+\n")

	return buf.String()
}

// validDumpedKV verifies validity of a dumped KV-pair.
func (s *Scheduler) validDumpedKV(kv kvs.KVWithMetadata, descriptor *kvs.KVDescriptor, refreshed utils.KeySet) bool {
	if kv.Key == "" {
		s.Log.WithFields(logging.Fields{
			"descriptor": descriptor.Name,
		}).Warn("Descriptor dumped value with empty key")
		return false
	}
	if alreadyDumped := refreshed.Has(kv.Key); alreadyDumped {
		s.Log.WithFields(logging.Fields{
			"descriptor": descriptor.Name,
			"key":        kv.Key,
		}).Warn("The same value was dumped more than once")
		return false
	}
	if kv.Value == nil {
		s.Log.WithFields(logging.Fields{
			"descriptor": descriptor.Name,
			"key":        kv.Key,
		}).Warn("Descriptor dumped nil value")
		return false
	}
	if !descriptor.KeySelector(kv.Key) {
		s.Log.WithFields(logging.Fields{
			"descriptor": descriptor.Name,
			"key":        kv.Key,
			"value":      kv.Value,
		}).Warn("Descriptor dumped value outside of its key space")
		return false
	}
	return true
}

// validDumpedKV verifies validity of a KV-pair derived from a dumped value.
func (s *Scheduler) validDumpedDerivedKV(node graph.Node, descriptor *kvs.KVDescriptor, refreshed utils.KeySet) bool {
	descriptorName := "<NONE>"
	if descriptor != nil {
		descriptorName = descriptor.Name
	}
	if node.GetValue() == nil {
		s.Log.WithFields(logging.Fields{
			"descriptor": descriptorName,
			"key":        node.GetKey(),
		}).Warn("Derived nil value")
		return false
	}
	if alreadyDumped := refreshed.Has(node.GetKey()); alreadyDumped {
		s.Log.WithFields(logging.Fields{
			"descriptor": descriptorName,
			"key":        node.GetKey(),
		}).Warn("The same value was dumped more than once")
		// return true -> let's overwrite invalidly dumped derived value
	}
	return true
}
