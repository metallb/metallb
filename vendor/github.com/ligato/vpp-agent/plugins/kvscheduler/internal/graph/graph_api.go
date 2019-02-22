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
	"bytes"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/ligato/cn-infra/idxmap"
	. "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
)

// Graph is an in-memory graph representation of key-value pairs and their
// relations, where nodes are kv-pairs and each relation is a separate set of direct
// labeled edges.
//
// The graph furthermore allows to associate metadata and flags (name:value pairs)
// with every node. It is possible to register instances of NamedMapping, each
// for a different set of selected nodes, and the graph will keep them up-to-date
// with the latest value-label->metadata associations.
//
// The graph provides various getter method, for example it is possible to select
// a set of nodes using a key selector and/or a flag selector.
// As for editing, Graph allows to prepare new changes and then save them or let
// them get discarded by GC.
//
// The graph supports multiple-readers single-writer access, i.e. it is assumed
// there is no write-concurrency.
//
// Last but not least, the graph maintains a history of revisions for all nodes
// that have ever existed. The history of changes and a graph snapshot from
// a selected moment in time are exposed via ReadAccess interface.
type Graph interface {
	// Read returns a graph handle for read-only access.
	// The graph supports multiple concurrent readers.
	// Release eventually using Release() method.
	Read() ReadAccess // acquires R-lock

	// Write returns a graph handle for read-write access.
	// The graph supports at most one writer at a time - i.e. it is assumed
	// there is no write-concurrency.
	// The changes are propagated to the graph using Save().
	// If <record> is true, the changes will be recorded once the handle is
	// released.
	// Release eventually using Release() method.
	Write(record bool) RWAccess
}

// ReadAccess lists operations provided by the read-only graph handle.
type ReadAccess interface {
	// GetMetadataMap returns registered metadata map.
	GetMetadataMap(mapName string) idxmap.NamedMapping

	// GetKeys returns sorted keys.
	GetKeys() []string

	// GetNode returns node with the given key or nil if the key is unused.
	GetNode(key string) Node

	// GetNodes returns a set of nodes matching the key selector (can be nil)
	// and every provided flag selector.
	GetNodes(keySelector KeySelector, flagSelectors ...FlagSelector) []Node

	// GetFlagStats returns stats for a given flag.
	GetFlagStats(flagName string, filter KeySelector) FlagStats

	// GetNodeTimeline returns timeline of all node revisions, ordered from
	// the oldest to the newest.
	GetNodeTimeline(key string) []*RecordedNode

	// GetSnapshot returns the snapshot of the graph at a given time.
	GetSnapshot(time time.Time) []*RecordedNode

	// Dump returns a human-readable string representation of the current graph
	// content for debugging purposes.
	Dump() string

	// Release releases the graph handle (both Read() & Write() should end with
	// release).
	Release() // for reader release R-lock
}

// RWAccess lists operations provided by the read-write graph handle.
type RWAccess interface {
	ReadAccess

	// RegisterMetadataMap registers new metadata map for value-label->metadata
	// associations of selected node.
	RegisterMetadataMap(mapName string, mapping idxmap.NamedMappingRW)

	// SetNode creates new node or returns read-write handle to an existing node.
	// The changes are propagated to the graph only after Save() is called.
	SetNode(key string) NodeRW

	// DeleteNode deletes node with the given key.
	// Returns true if the node really existed before the operation.
	DeleteNode(key string) bool

	// Save propagates all changes to the graph.
	Save() // noop if no changes performed, acquires RW-lock for the time of the operation
}

// Node is a read-only handle to a single graph node.
type Node interface {
	// GetKey returns the key associated with the node.
	GetKey() string

	// GetLabel returns the label associated with this node.
	GetLabel() string

	// GetKey returns the value associated with the node.
	GetValue() proto.Message

	// GetFlag returns reference to the given flag or nil if the node doesn't have
	// this flag associated.
	GetFlag(name string) Flag

	// GetMetadata returns the value metadata associated with the node.
	GetMetadata() interface{}

	// GetTargets returns a set of nodes, indexed by relation labels, that the
	// edges of the given relation points to.
	GetTargets(relation string) RuntimeTargetsByLabel

	// GetSources returns a set of nodes with edges of the given relation
	// pointing to this node.
	GetSources(relation string) []Node
}

// NodeRW is a read-write handle to a single graph node.
type NodeRW interface {
	Node

	// SetLabel associates given label with this node.
	SetLabel(label string)

	// SetValue associates given value with this node.
	SetValue(value proto.Message)

	// SetFlags associates given flag with this node.
	SetFlags(flags ...Flag)

	// DelFlags removes given flag from this node.
	DelFlags(names ...string)

	// SetMetadataMap chooses metadata map to be used to store the association
	// between this node's value label and metadata.
	SetMetadataMap(mapName string)

	// SetMetadata associates given value metadata with this node.
	SetMetadata(metadata interface{})

	// SetTargets provides definition of all edges pointing from this node.
	SetTargets(targets []RelationTargetDef)
}

// Flag is a name:value pair.
type Flag interface {
	// GetName should return name of the flag.
	GetName() string

	// GetValue return the associated value. Can be empty.
	GetValue() string
}

// FlagSelector is used to select node with(out) given flags assigned.
//
// Flag value=="" => any value
type FlagSelector struct {
	with  bool
	flags []Flag
}

// WithFlags creates flag selector selecting nodes that have all the listed flags
// assigned.
func WithFlags(flags ...Flag) FlagSelector {
	return FlagSelector{with: true, flags: flags}
}

// WithoutFlags creates flag selector selecting nodes that do not have
// any of the listed flags assigned.
func WithoutFlags(flags ...Flag) FlagSelector {
	return FlagSelector{flags: flags}
}

// RelationTargetDef is a definition of a relation between a source node and a set
// of target nodes.
type RelationTargetDef struct {
	// Relation name.
	Relation string

	// Label for the edge.
	Label string // mandatory, unique for a given (source, relation)

	// Either Key or Selector are defined:

	// Key of the target node.
	Key string

	// Selector selecting a set of target nodes.
	Selector KeySelector
}

// Targets groups relation targets with the same label.
// Target nodes are not referenced directly, instead via their keys (suitable
// for recording).
type Targets struct {
	Label        string
	ExpectedKey  string // empty if AnyOf predicate is used instead
	MatchingKeys utils.KeySet
}

// TargetsByLabel is a slice of single-relation targets, grouped by labels.
type TargetsByLabel []*Targets

// String returns human-readable string representation of TargetsByLabel.
func (t TargetsByLabel) String() string {
	str := "{"
	for idx, targets := range t {
		if idx > 0 {
			str += ", "
		}
		str += fmt.Sprintf("%s->%s", targets.Label, targets.MatchingKeys.String())
	}
	str += "}"
	return str
}

// RelationTargets groups targets of the same relation.
type RelationTargets struct {
	Relation string
	Targets  TargetsByLabel
}

// GetTargetsForLabel returns targets (keys) for the given label.
func (t *RelationTargets) GetTargetsForLabel(label string) *Targets {
	for _, targets := range t.Targets {
		if targets.Label == label {
			return targets
		}
	}
	return nil
}

// TargetsByRelation is a slice of all targets, grouped by relations.
type TargetsByRelation []*RelationTargets

// GetTargetsForRelation returns targets (keys by label) for the given relation.
func (t TargetsByRelation) GetTargetsForRelation(relation string) *RelationTargets {
	for _, relTargets := range t {
		if relTargets.Relation == relation {
			return relTargets
		}
	}
	return nil
}

// String returns human-readable string representation of TargetsByRelation.
func (t TargetsByRelation) String() string {
	str := "{"
	for idx, relTargets := range t {
		if idx > 0 {
			str += ", "
		}
		str += fmt.Sprintf("%s->%s", relTargets.Relation, relTargets.Targets.String())
	}
	str += "}"
	return str
}

// RuntimeTargets groups relation targets with the same label.
// Targets are stored as direct runtime references pointing to instances of target
// nodes.
type RuntimeTargets struct {
	Label string
	Nodes []Node
}

// RuntimeTargetsByLabel is a slice of single-relation (runtime reference-based)
// targets, grouped by labels.
type RuntimeTargetsByLabel []*RuntimeTargets

// GetTargetsForLabel returns targets (nodes) for the given label.
func (rt RuntimeTargetsByLabel) GetTargetsForLabel(label string) *RuntimeTargets {
	for _, targets := range rt {
		if targets.Label == label {
			return targets
		}
	}
	return nil
}

// RecordedNode saves all attributes of a single node revision.
type RecordedNode struct {
	Since            time.Time
	Until            time.Time
	Key              string
	Label            string
	Value            proto.Message
	Flags            RecordedFlags
	MetadataFields   map[string][]string // field name -> values
	Targets          TargetsByRelation
	TargetUpdateOnly bool                // true if only runtime Targets have changed since the last rev
}

// GetFlag returns reference to the given flag or nil if the node didn't have
// this flag associated at the time when it was recorded.
func (node *RecordedNode) GetFlag(name string) Flag {
	for _, flag := range node.Flags.Flags {
		if flag.GetName() == name {
			return flag
		}
	}
	return nil
}

// RecordedFlags is a record of assigned flags at a given time.
type RecordedFlags struct {
	Flags []Flag
}

// MarshalJSON marshalls recorded flags into JSON.
func (rf RecordedFlags) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{")
	for idx, flag := range rf.Flags {
		if idx > 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(fmt.Sprintf("\"%s\":\"%s\"", flag.GetName(), flag.GetValue()))
	}
	buffer.WriteString("}")
	return buffer.Bytes(), nil
}

// GetFlag returns reference to the given flag or nil if the node hasn't had
// this flag associated at the given time.
func (rf RecordedFlags) GetFlag(name string) Flag {
	for _, flag := range rf.Flags {
		if flag.GetName() == name {
			return flag
		}
	}
	return nil
}

// FlagStats is a summary of the usage for a given flag.
type FlagStats struct {
	TotalCount    uint            // number of revisions with the given flag assigned
	PerValueCount map[string]uint // number of revisions with the given flag having the given value
}
