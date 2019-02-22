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
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/gomega"

	. "github.com/ligato/vpp-agent/plugins/kvscheduler/internal/test"
	. "github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
)

const (
	minutesInOneDay = uint32(1440)
	minutesInOneHour = uint32(60)
)

func TestEmptyGraph(t *testing.T) {
	RegisterTestingT(t)

	graph := NewGraph(true, minutesInOneDay, minutesInOneHour)
	Expect(graph).ToNot(BeNil())

	graphR := graph.Read()
	Expect(graphR).ToNot(BeNil())

	Expect(graphR.GetNode(keyA1)).To(BeNil())
	Expect(graphR.GetNodeTimeline(keyA1)).To(BeEmpty())
	Expect(graphR.GetNodes(prefixASelector)).To(BeEmpty())
	Expect(graphR.GetMetadataMap(metadataMapA)).To(BeNil())
	Expect(graphR.GetSnapshot(time.Now())).To(BeEmpty())
	flagStats := graphR.GetFlagStats(ColorFlagName, nil)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(0))
	Expect(flagStats.PerValueCount).To(BeEmpty())
	graphR.Release()
}

func TestSingleNode(t *testing.T) {
	RegisterTestingT(t)

	startTime := time.Now()

	graph := NewGraph(true, minutesInOneDay, minutesInOneHour)
	graphW := graph.Write(true)

	graphW.RegisterMetadataMap(metadataMapA, NewNameToInteger(metadataMapA))

	nodeW := graphW.SetNode(keyA1)
	// new node, everything except the key is unset:
	Expect(nodeW.GetKey()).To(BeEquivalentTo(keyA1))
	Expect(nodeW.GetValue()).To(BeNil())
	Expect(nodeW.GetTargets(relation1)).To(BeEmpty())
	Expect(nodeW.GetSources(relation1)).To(BeEmpty())
	Expect(nodeW.GetMetadata()).To(BeNil())
	Expect(nodeW.GetFlag(ColorFlagName)).To(BeNil())

	// set attributes:
	nodeW.SetLabel(value1Label)
	nodeW.SetValue(value1)
	nodeW.SetMetadata(&OnlyInteger{Integer: 1})
	nodeW.SetMetadataMap(metadataMapA)
	nodeW.SetFlags(ColorFlag(Red), AbstractFlag())

	// check attributes:
	Expect(nodeW.GetLabel()).To(Equal(value1Label))
	Expect(nodeW.GetValue()).To(Equal(value1))
	Expect(nodeW.GetMetadata().(MetaWithInteger).GetInteger()).To(Equal(1))
	flag := nodeW.GetFlag(ColorFlagName)
	Expect(flag).ToNot(BeNil())
	colorFlag := flag.(*ColorFlagImpl)
	Expect(colorFlag.Color).To(Equal(Red))
	Expect(nodeW.GetFlag(AbstractFlagName)).ToNot(BeNil())
	Expect(nodeW.GetFlag(TemporaryFlagName)).To(BeNil())
	Expect(nodeW.GetTargets(relation1)).To(BeEmpty())
	Expect(nodeW.GetSources(relation1)).To(BeEmpty())

	// not applied into the graph until saved
	graphR := graph.Read()
	Expect(graphR.GetNode(keyA1)).To(BeNil())
	Expect(graphR.GetMetadataMap(metadataMapA)).To(BeNil())
	graphR.Release()

	// save new node
	graphW.Save()
	graphW.Release()

	// check that the new node was saved correctly
	graphR = graph.Read()
	nodeR := graphR.GetNode(keyA1)
	Expect(nodeR).ToNot(BeNil())
	Expect(nodeR.GetLabel()).To(Equal(value1Label))
	Expect(nodeR.GetValue()).To(Equal(value1))
	Expect(nodeR.GetMetadata().(MetaWithInteger).GetInteger()).To(Equal(1))
	flag = nodeR.GetFlag(ColorFlagName)
	Expect(flag).ToNot(BeNil())
	colorFlag = flag.(*ColorFlagImpl)
	Expect(colorFlag.Color).To(Equal(Red))
	Expect(nodeR.GetFlag(AbstractFlagName)).ToNot(BeNil())
	Expect(nodeR.GetFlag(TemporaryFlagName)).To(BeNil())
	Expect(nodeR.GetTargets(relation1)).To(BeEmpty())
	Expect(nodeR.GetSources(relation1)).To(BeEmpty())

	// check metadata
	metaMap := graphR.GetMetadataMap(metadataMapA)
	Expect(metaMap).ToNot(BeNil())
	Expect(metaMap.ListAllNames()).To(Equal([]string{value1Label}))
	intMap := metaMap.(NameToInteger)
	metadata, exists := intMap.LookupByName(value1Label)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(1))
	label, metadata, exists := intMap.LookupByIndex(1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(1))
	Expect(label).To(Equal(value1Label))

	// check history
	flagStats := graphR.GetFlagStats(ColorFlagName, prefixASelector)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(1))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{Red.String(): 1}))
	timeline := graphR.GetNodeTimeline(keyA1)
	Expect(timeline).To(HaveLen(1))
	record := timeline[0]
	Expect(record.Key).To(Equal(keyA1))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(time.Now())).To(BeTrue())
	Expect(record.Until.IsZero()).To(BeTrue())
	Expect(record.Label).To(Equal(value1Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value1))).To(BeTrue())
	Expect(record.Targets).To(BeEmpty())
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"1"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Red), AbstractFlag()}}))
}

func TestMultipleNodes(t *testing.T) {
	RegisterTestingT(t)

	startTime := time.Now()
	graph := buildGraph(nil, true, true, selectNodesToBuild(1, 2, 3, 4))

	// check graph content
	graphR := graph.Read()

	// -> node1:
	node1 := graphR.GetNode(keyA1)
	Expect(node1).ToNot(BeNil())
	Expect(node1.GetLabel()).To(Equal(value1Label))
	Expect(node1.GetValue()).To(Equal(value1))
	Expect(node1.GetMetadata().(MetaWithInteger).GetInteger()).To(Equal(1))
	flag := node1.GetFlag(ColorFlagName)
	Expect(flag).ToNot(BeNil())
	colorFlag := flag.(*ColorFlagImpl)
	Expect(colorFlag.Color).To(Equal(Red))
	Expect(node1.GetFlag(AbstractFlagName)).ToNot(BeNil())
	Expect(node1.GetFlag(TemporaryFlagName)).To(BeNil())
	Expect(node1.GetTargets(relation1)).To(HaveLen(1))
	checkTargets(node1, relation1, "node2", keyA2)
	checkSources(node1, relation1, keyB1)
	Expect(node1.GetTargets(relation2)).To(HaveLen(1))
	checkTargets(node1, relation2, "prefixB", keyB1)
	checkSources(node1, relation2, keyA3)

	// -> node2:
	node2 := graphR.GetNode(keyA2)
	Expect(node2).ToNot(BeNil())
	Expect(node2.GetLabel()).To(Equal(value2Label))
	Expect(node2.GetValue()).To(Equal(value2))
	Expect(node2.GetMetadata().(MetaWithInteger).GetInteger()).To(Equal(2))
	flag = node2.GetFlag(ColorFlagName)
	Expect(flag).ToNot(BeNil())
	colorFlag = flag.(*ColorFlagImpl)
	Expect(colorFlag.Color).To(Equal(Blue))
	Expect(node2.GetFlag(AbstractFlagName)).To(BeNil())
	Expect(node2.GetFlag(TemporaryFlagName)).To(BeNil())
	Expect(node2.GetTargets(relation1)).To(HaveLen(1))
	checkTargets(node2, relation1, "node3", keyA3)
	checkSources(node2, relation1, keyA1, keyB1)
	Expect(node2.GetTargets(relation2)).To(HaveLen(0))
	checkSources(node2, relation2, keyA3)

	// -> node3:
	node3 := graphR.GetNode(keyA3)
	Expect(node3).ToNot(BeNil())
	Expect(node3.GetLabel()).To(Equal(value3Label))
	Expect(node3.GetValue()).To(Equal(value3))
	Expect(node3.GetMetadata().(MetaWithInteger).GetInteger()).To(Equal(3))
	flag = node3.GetFlag(ColorFlagName)
	Expect(flag).ToNot(BeNil())
	colorFlag = flag.(*ColorFlagImpl)
	Expect(colorFlag.Color).To(Equal(Green))
	Expect(node3.GetFlag(AbstractFlagName)).ToNot(BeNil())
	Expect(node3.GetFlag(TemporaryFlagName)).ToNot(BeNil())
	Expect(node3.GetTargets(relation1)).To(BeEmpty())
	checkSources(node3, relation1, keyA2, keyB1)
	Expect(node3.GetTargets(relation2)).To(HaveLen(2))
	checkTargets(node3, relation2, "node1+node2", keyA1, keyA2)
	checkTargets(node3, relation2, "prefixB", keyB1)
	checkSources(node3, relation2)

	// -> node4:
	node4 := graphR.GetNode(keyB1)
	Expect(node4).ToNot(BeNil())
	Expect(node4.GetLabel()).To(Equal(value4Label))
	Expect(node4.GetValue()).To(Equal(value4))
	Expect(node4.GetMetadata().(MetaWithInteger).GetInteger()).To(Equal(1))
	Expect(node4.GetFlag(ColorFlagName)).To(BeNil())
	Expect(node4.GetFlag(AbstractFlagName)).To(BeNil())
	Expect(node4.GetFlag(TemporaryFlagName)).ToNot(BeNil())
	Expect(node4.GetTargets(relation1)).To(HaveLen(1))
	checkTargets(node4, relation1, "prefixA", keyA1, keyA2, keyA3)
	checkSources(node4, relation1)
	Expect(node4.GetTargets(relation2)).To(HaveLen(1))
	checkTargets(node4, relation2, "non-existing-key")
	checkSources(node4, relation2, keyA1, keyA3)

	// check metadata

	// -> metadata for prefixA:
	metaMap := graphR.GetMetadataMap(metadataMapA)
	Expect(metaMap).ToNot(BeNil())
	checkMetadataValues(metaMap, value1Label, value2Label, value3Label)
	intMap := metaMap.(NameToInteger)
	label, metadata, exists := intMap.LookupByIndex(1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(1))
	Expect(label).To(Equal(value1Label))
	label, metadata, exists = intMap.LookupByIndex(2)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(2))
	Expect(label).To(Equal(value2Label))
	label, metadata, exists = intMap.LookupByIndex(3)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(3))
	Expect(label).To(Equal(value3Label))

	// -> metadata for prefixB:
	metaMap = graphR.GetMetadataMap(metadataMapB)
	Expect(metaMap).ToNot(BeNil())
	checkMetadataValues(metaMap, value4Label)
	intMap = metaMap.(NameToInteger)
	label, metadata, exists = intMap.LookupByIndex(1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(1))
	Expect(label).To(Equal(value4Label))

	// check history

	// -> flags:
	flagStats := graphR.GetFlagStats(ColorFlagName, prefixASelector)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(3))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{
		Red.String():   1,
		Blue.String():  1,
		Green.String(): 1,
	}))
	flagStats = graphR.GetFlagStats(ColorFlagName, prefixBSelector)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(0))
	Expect(flagStats.PerValueCount).To(BeEmpty())
	flagStats = graphR.GetFlagStats(AbstractFlagName, nil)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(2))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{"": 2}))
	flagStats = graphR.GetFlagStats(TemporaryFlagName, nil)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(2))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{"": 2}))

	// -> timeline node1:
	timeline := graphR.GetNodeTimeline(keyA1)
	Expect(timeline).To(HaveLen(1))
	record := timeline[0]
	Expect(record.Key).To(Equal(keyA1))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(time.Now())).To(BeTrue())
	Expect(record.Until.IsZero()).To(BeTrue())
	Expect(record.Label).To(Equal(value1Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value1))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(2))
	checkRecordedTargets(record.Targets, relation1, 1, "node2", keyA2)
	checkRecordedTargets(record.Targets, relation2, 1, "prefixB", keyB1)
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"1"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Red), AbstractFlag()}}))

	// -> timeline node2:
	timeline = graphR.GetNodeTimeline(keyA2)
	Expect(timeline).To(HaveLen(1))
	record = timeline[0]
	Expect(record.Key).To(Equal(keyA2))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(time.Now())).To(BeTrue())
	Expect(record.Until.IsZero()).To(BeTrue())
	Expect(record.Label).To(Equal(value2Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value2))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(1))
	checkRecordedTargets(record.Targets, relation1, 1, "node3", keyA3)
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"2"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Blue)}}))

	// -> timeline node3:
	timeline = graphR.GetNodeTimeline(keyA3)
	Expect(timeline).To(HaveLen(1))
	record = timeline[0]
	Expect(record.Key).To(Equal(keyA3))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(time.Now())).To(BeTrue())
	Expect(record.Until.IsZero()).To(BeTrue())
	Expect(record.Label).To(Equal(value3Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value3))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(1))
	checkRecordedTargets(record.Targets, relation2, 2, "node1+node2", keyA1, keyA2)
	checkRecordedTargets(record.Targets, relation2, 2, "prefixB", keyB1)
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"3"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Green), AbstractFlag(), TemporaryFlag()}}))

	// -> timeline node4:
	timeline = graphR.GetNodeTimeline(keyB1)
	Expect(timeline).To(HaveLen(1))
	record = timeline[0]
	Expect(record.Key).To(Equal(keyB1))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(time.Now())).To(BeTrue())
	Expect(record.Until.IsZero()).To(BeTrue())
	Expect(record.Label).To(Equal(value4Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value4))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(2))
	checkRecordedTargets(record.Targets, relation1, 1, "prefixA", keyA1, keyA2, keyA3)
	checkRecordedTargets(record.Targets, relation2, 1, "non-existing-key")
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"1"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{TemporaryFlag()}}))

	// check snapshot:
	// -> before the changes
	records := graphR.GetSnapshot(startTime)
	checkRecordedNodes(records)
	// -> after the changes
	records = graphR.GetSnapshot(time.Now())
	checkRecordedNodes(records, keyA1, keyA2, keyA3, keyB1)

	graphR.Release()
}

func TestSelectors(t *testing.T) {
	RegisterTestingT(t)

	graph := buildGraph(nil, true, true, selectNodesToBuild(1, 2, 3, 4))
	graphR := graph.Read()

	// test key selector
	checkNodes(graphR.GetNodes(prefixASelector), keyA1, keyA2, keyA3)
	checkNodes(graphR.GetNodes(prefixBSelector), keyB1)
	checkNodes(graphR.GetNodes(keySelector(keyA1, keyB1)), keyA1, keyB1)
	checkNodes(graphR.GetNodes(func(key string) bool { return false }))

	// test flag selectors
	checkNodes(graphR.GetNodes(nil, WithFlags(AnyColorFlag())), keyA1, keyA2, keyA3)
	checkNodes(graphR.GetNodes(nil, WithFlags(ColorFlag(Red))), keyA1)
	checkNodes(graphR.GetNodes(nil, WithFlags(ColorFlag(Blue))), keyA2)
	checkNodes(graphR.GetNodes(nil, WithFlags(ColorFlag(Green))), keyA3)
	checkNodes(graphR.GetNodes(nil, WithFlags(AnyColorFlag()), WithoutFlags(TemporaryFlag())), keyA1, keyA2)
	checkNodes(graphR.GetNodes(nil, WithoutFlags(TemporaryFlag())), keyA1, keyA2)
	checkNodes(graphR.GetNodes(nil, WithoutFlags(AbstractFlag())), keyA2, keyB1)

	// test combination of key selector and flag selector
	checkNodes(graphR.GetNodes(prefixASelector, WithoutFlags(AbstractFlag())), keyA2)
	checkNodes(graphR.GetNodes(prefixBSelector, WithoutFlags(TemporaryFlag())))
	checkNodes(graphR.GetNodes(keySelector(keyA1, keyB1), WithFlags(AnyColorFlag())), keyA1)

	// change flags and re-test flag selectors
	graphR.Release()
	graphW := graph.Write(false)
	graphW.SetNode(keyA1).SetFlags(ColorFlag(Green), TemporaryFlag())
	graphW.SetNode(keyA1).DelFlags(AbstractFlagName)
	graphW.SetNode(keyA3).DelFlags(ColorFlagName)
	graphW.Save()
	graphW.Release()

	graphR = graph.Read()
	checkNodes(graphR.GetNodes(nil, WithFlags(AnyColorFlag())), keyA1, keyA2)
	checkNodes(graphR.GetNodes(nil, WithFlags(ColorFlag(Red))))
	checkNodes(graphR.GetNodes(nil, WithFlags(ColorFlag(Blue))), keyA2)
	checkNodes(graphR.GetNodes(nil, WithFlags(ColorFlag(Green))), keyA1)
	checkNodes(graphR.GetNodes(nil, WithFlags(AnyColorFlag()), WithoutFlags(TemporaryFlag())), keyA2)
	checkNodes(graphR.GetNodes(nil, WithoutFlags(TemporaryFlag())), keyA2)
	checkNodes(graphR.GetNodes(nil, WithoutFlags(AbstractFlag())), keyA1, keyA2, keyB1)
	graphR.Release()
}

func TestNodeRemoval(t *testing.T) {
	RegisterTestingT(t)

	startTime := time.Now()
	graph := buildGraph(nil, true, true, selectNodesToBuild(1, 2, 3, 4))

	// delete node2 & node 4
	delTime := time.Now()
	graphW := graph.Write(true)
	graphW.DeleteNode(keyA2)
	graphW.DeleteNode(keyB1)
	graphW.Save()
	graphW.Release()

	// check graph content
	graphR := graph.Read()

	// -> node1:
	node1 := graphR.GetNode(keyA1)
	Expect(node1).ToNot(BeNil())
	Expect(node1.GetLabel()).To(Equal(value1Label))
	Expect(node1.GetValue()).To(Equal(value1))
	Expect(node1.GetMetadata().(MetaWithInteger).GetInteger()).To(Equal(1))
	flag := node1.GetFlag(ColorFlagName)
	Expect(flag).ToNot(BeNil())
	colorFlag := flag.(*ColorFlagImpl)
	Expect(colorFlag.Color).To(Equal(Red))
	Expect(node1.GetFlag(AbstractFlagName)).ToNot(BeNil())
	Expect(node1.GetFlag(TemporaryFlagName)).To(BeNil())
	Expect(node1.GetTargets(relation1)).To(HaveLen(1))
	checkTargets(node1, relation1, "node2")
	checkSources(node1, relation1)
	Expect(node1.GetTargets(relation2)).To(HaveLen(1))
	checkTargets(node1, relation2, "prefixB")
	checkSources(node1, relation2, keyA3)

	// -> node2:
	node2 := graphR.GetNode(keyA2)
	Expect(node2).To(BeNil())

	// -> node3:
	node3 := graphR.GetNode(keyA3)
	Expect(node3).ToNot(BeNil())
	Expect(node3.GetLabel()).To(Equal(value3Label))
	Expect(node3.GetValue()).To(Equal(value3))
	Expect(node3.GetMetadata().(MetaWithInteger).GetInteger()).To(Equal(3))
	flag = node3.GetFlag(ColorFlagName)
	Expect(flag).ToNot(BeNil())
	colorFlag = flag.(*ColorFlagImpl)
	Expect(colorFlag.Color).To(Equal(Green))
	Expect(node3.GetFlag(AbstractFlagName)).ToNot(BeNil())
	Expect(node3.GetFlag(TemporaryFlagName)).ToNot(BeNil())
	Expect(node3.GetTargets(relation1)).To(BeEmpty())
	checkSources(node3, relation1)
	Expect(node3.GetTargets(relation2)).To(HaveLen(2))
	checkTargets(node3, relation2, "node1+node2", keyA1)
	checkTargets(node3, relation2, "prefixB")
	checkSources(node3, relation2)

	// -> node4:
	node4 := graphR.GetNode(keyB1)
	Expect(node4).To(BeNil())

	// check metadata

	// -> metadata for prefixA:
	metaMap := graphR.GetMetadataMap(metadataMapA)
	Expect(metaMap).ToNot(BeNil())
	checkMetadataValues(metaMap, value1Label, value3Label)
	intMap := metaMap.(NameToInteger)
	label, metadata, exists := intMap.LookupByIndex(1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(1))
	Expect(label).To(Equal(value1Label))
	label, metadata, exists = intMap.LookupByIndex(2)
	Expect(exists).To(BeFalse())
	label, metadata, exists = intMap.LookupByIndex(3)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(3))
	Expect(label).To(Equal(value3Label))

	// -> metadata for prefixB:
	metaMap = graphR.GetMetadataMap(metadataMapB)
	Expect(metaMap).ToNot(BeNil())
	checkMetadataValues(metaMap)
	intMap = metaMap.(NameToInteger)
	label, metadata, exists = intMap.LookupByIndex(1)
	Expect(exists).To(BeFalse())

	// check history

	// -> flags:
	flagStats := graphR.GetFlagStats(ColorFlagName, prefixASelector)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(3))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{
		Red.String():   1,
		Blue.String():  1,
		Green.String(): 1,
	}))
	flagStats = graphR.GetFlagStats(ColorFlagName, prefixBSelector)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(0))
	Expect(flagStats.PerValueCount).To(BeEmpty())
	flagStats = graphR.GetFlagStats(AbstractFlagName, nil)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(2))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{"": 2}))
	flagStats = graphR.GetFlagStats(TemporaryFlagName, nil)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(2))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{"": 2}))

	// -> timeline node1:
	timeline := graphR.GetNodeTimeline(keyA1)
	Expect(timeline).To(HaveLen(2))
	//   -> prev record
	record := timeline[0]
	Expect(record.Key).To(Equal(keyA1))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(delTime)).To(BeTrue())
	Expect(record.Until.After(delTime)).To(BeTrue())
	Expect(record.Until.Before(time.Now())).To(BeTrue())
	Expect(record.Label).To(Equal(value1Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value1))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(2))
	checkRecordedTargets(record.Targets, relation1, 1, "node2", keyA2)
	checkRecordedTargets(record.Targets, relation2, 1, "prefixB", keyB1)
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"1"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Red), AbstractFlag()}}))
	//   -> new record
	record = timeline[1]
	Expect(record.Key).To(Equal(keyA1))
	Expect(record.Since.After(delTime)).To(BeTrue())
	Expect(record.Since.Before(time.Now())).To(BeTrue())
	Expect(record.Until.IsZero()).To(BeTrue())
	Expect(record.Label).To(Equal(value1Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value1))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(2))
	checkRecordedTargets(record.Targets, relation1, 1, "node2")
	checkRecordedTargets(record.Targets, relation2, 1, "prefixB")
	Expect(record.TargetUpdateOnly).To(BeTrue())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"1"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Red), AbstractFlag()}}))

	// -> timeline node2:
	timeline = graphR.GetNodeTimeline(keyA2)
	Expect(timeline).To(HaveLen(1))
	//   -> old record
	record = timeline[0]
	Expect(record.Key).To(Equal(keyA2))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(delTime)).To(BeTrue())
	Expect(record.Until.After(delTime)).To(BeTrue())
	Expect(record.Until.Before(time.Now())).To(BeTrue())
	Expect(record.Label).To(Equal(value2Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value2))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(1))
	checkRecordedTargets(record.Targets, relation1, 1, "node3", keyA3)
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"2"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Blue)}}))

	// -> timeline node3:
	timeline = graphR.GetNodeTimeline(keyA3)
	Expect(timeline).To(HaveLen(2))
	//   -> old record
	record = timeline[0]
	Expect(record.Key).To(Equal(keyA3))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(delTime)).To(BeTrue())
	Expect(record.Until.After(delTime)).To(BeTrue())
	Expect(record.Until.Before(time.Now())).To(BeTrue())
	Expect(record.Label).To(Equal(value3Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value3))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(1))
	checkRecordedTargets(record.Targets, relation2, 2, "node1+node2", keyA1, keyA2)
	checkRecordedTargets(record.Targets, relation2, 2, "prefixB", keyB1)
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"3"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Green), AbstractFlag(), TemporaryFlag()}}))
	//   -> new record
	record = timeline[1]
	Expect(record.Key).To(Equal(keyA3))
	Expect(record.Since.After(delTime)).To(BeTrue())
	Expect(record.Since.Before(time.Now())).To(BeTrue())
	Expect(record.Until.IsZero()).To(BeTrue())
	Expect(record.Label).To(Equal(value3Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value3))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(1))
	checkRecordedTargets(record.Targets, relation2, 2, "node1+node2", keyA1)
	checkRecordedTargets(record.Targets, relation2, 2, "prefixB")
	Expect(record.TargetUpdateOnly).To(BeTrue())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"3"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Green), AbstractFlag(), TemporaryFlag()}}))

	// -> timeline node4:
	//   -> old record
	timeline = graphR.GetNodeTimeline(keyB1)
	Expect(timeline).To(HaveLen(1))
	record = timeline[0]
	Expect(record.Key).To(Equal(keyB1))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(delTime)).To(BeTrue())
	Expect(record.Until.After(delTime)).To(BeTrue())
	Expect(record.Until.Before(time.Now())).To(BeTrue())
	Expect(record.Label).To(Equal(value4Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value4))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(2))
	checkRecordedTargets(record.Targets, relation1, 1, "prefixA", keyA1, keyA2, keyA3)
	checkRecordedTargets(record.Targets, relation2, 1, "non-existing-key")
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"1"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{TemporaryFlag()}}))

	// check snapshot:
	records := graphR.GetSnapshot(time.Now())
	checkRecordedNodes(records, keyA1, keyA3)

	graphR.Release()
}

func TestNodeTimeline(t *testing.T) {
	RegisterTestingT(t)

	// add node1
	startTime := time.Now()
	graph := buildGraph(nil, true, true, selectNodesToBuild(1))

	// delete node1
	delTime := time.Now()
	graphW := graph.Write(true)
	graphW.DeleteNode(keyA1)
	graphW.Save()
	graphW.Release()

	// re-create node1, but without recording
	buildGraph(graph, false, false, selectNodesToBuild(1))

	// change flags
	changeTime1 := time.Now()
	graphW = graph.Write(true)
	node := graphW.SetNode(keyA1)
	node.SetFlags(ColorFlag(Blue))
	graphW.Save()
	graphW.Release()

	// change metadata + flags
	changeTime2 := time.Now()
	graphW = graph.Write(true)
	node = graphW.SetNode(keyA1)
	node.SetFlags(TemporaryFlag())
	node.DelFlags(AbstractFlagName)
	node.SetMetadata(&OnlyInteger{Integer: 2})
	graphW.Save()
	graphW.Release()

	// check history
	graphR := graph.Read()

	// -> flags:
	flagStats := graphR.GetFlagStats(ColorFlagName, nil)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(3))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{
		Red.String():  1,
		Blue.String(): 2,
	}))
	flagStats = graphR.GetFlagStats(AbstractFlagName, nil)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(2))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{"": 2}))
	flagStats = graphR.GetFlagStats(TemporaryFlagName, nil)
	Expect(flagStats.TotalCount).To(BeEquivalentTo(1))
	Expect(flagStats.PerValueCount).To(BeEquivalentTo(map[string]uint{"": 1}))

	// -> timeline node1:
	timeline := graphR.GetNodeTimeline(keyA1)
	Expect(timeline).To(HaveLen(3))
	//   -> first record
	record := timeline[0]
	Expect(record.Key).To(Equal(keyA1))
	Expect(record.Since.After(startTime)).To(BeTrue())
	Expect(record.Since.Before(delTime)).To(BeTrue())
	Expect(record.Until.After(delTime)).To(BeTrue())
	Expect(record.Until.Before(changeTime1)).To(BeTrue())
	Expect(record.Label).To(Equal(value1Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value1))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(2))
	checkRecordedTargets(record.Targets, relation1, 1, "node2")
	checkRecordedTargets(record.Targets, relation2, 1, "prefixB")
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"1"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Red), AbstractFlag()}}))
	//   -> second record
	record = timeline[1]
	Expect(record.Key).To(Equal(keyA1))
	Expect(record.Since.After(changeTime1)).To(BeTrue())
	Expect(record.Since.Before(changeTime2)).To(BeTrue())
	Expect(record.Until.After(changeTime2)).To(BeTrue())
	Expect(record.Until.Before(time.Now())).To(BeTrue())
	Expect(record.Label).To(Equal(value1Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value1))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(2))
	checkRecordedTargets(record.Targets, relation1, 1, "node2")
	checkRecordedTargets(record.Targets, relation2, 1, "prefixB")
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"1"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{AbstractFlag(), ColorFlag(Blue)}}))
	//   -> third record
	record = timeline[2]
	Expect(record.Key).To(Equal(keyA1))
	Expect(record.Since.After(changeTime2)).To(BeTrue())
	Expect(record.Since.Before(time.Now())).To(BeTrue())
	Expect(record.Until.IsZero()).To(BeTrue())
	Expect(record.Label).To(Equal(value1Label))
	Expect(proto.Equal(record.Value, RecordProtoMessage(value1))).To(BeTrue())
	Expect(record.Targets).To(HaveLen(2))
	checkRecordedTargets(record.Targets, relation1, 1, "node2")
	checkRecordedTargets(record.Targets, relation2, 1, "prefixB")
	Expect(record.TargetUpdateOnly).To(BeFalse())
	Expect(record.MetadataFields).To(BeEquivalentTo(map[string][]string{IntegerKey: {"2"}}))
	Expect(record.Flags).To(BeEquivalentTo(RecordedFlags{[]Flag{ColorFlag(Blue), TemporaryFlag()}}))

	graphR.Release()
}

func TestNodeMetadata(t *testing.T) {
	RegisterTestingT(t)

	// add node1-node3
	graph := buildGraph(nil, true, true, selectNodesToBuild(1, 2, 3))

	// check metadata
	graphR := graph.Read()

	// -> metadata for prefixA:
	metaMap := graphR.GetMetadataMap(metadataMapA)
	Expect(metaMap).ToNot(BeNil())
	checkMetadataValues(metaMap, value1Label, value2Label, value3Label)
	intMap := metaMap.(NameToInteger)
	label, metadata, exists := intMap.LookupByIndex(1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(1))
	Expect(label).To(Equal(value1Label))
	label, metadata, exists = intMap.LookupByIndex(2)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(2))
	Expect(label).To(Equal(value2Label))
	label, metadata, exists = intMap.LookupByIndex(3)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(3))
	Expect(label).To(Equal(value3Label))

	// -> metadata for prefixB:
	metaMap = graphR.GetMetadataMap(metadataMapB)
	Expect(metaMap).ToNot(BeNil())
	checkMetadataValues(metaMap)
	intMap = metaMap.(NameToInteger)
	label, metadata, exists = intMap.LookupByIndex(1)
	Expect(exists).To(BeFalse())
	graphR.Release()

	// add node4, remove node1 & change metadata for node2
	buildGraph(graph, true, false, selectNodesToBuild(4))
	graphW := graph.Write(true)
	graphW.DeleteNode(keyA1)
	graphW.SetNode(keyA2).SetMetadata(&OnlyInteger{Integer: 4})
	graphW.Save()
	graphW.Release()

	// check metadata after the changes
	graphR = graph.Read()

	// -> metadata for prefixA:
	metaMap = graphR.GetMetadataMap(metadataMapA)
	Expect(metaMap).ToNot(BeNil())
	checkMetadataValues(metaMap, value2Label, value3Label)
	intMap = metaMap.(NameToInteger)
	label, metadata, exists = intMap.LookupByIndex(1)
	Expect(exists).To(BeFalse())
	label, metadata, exists = intMap.LookupByIndex(2)
	Expect(exists).To(BeFalse())
	label, metadata, exists = intMap.LookupByIndex(4)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(4))
	Expect(label).To(Equal(value2Label))
	label, metadata, exists = intMap.LookupByIndex(3)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(3))
	Expect(label).To(Equal(value3Label))

	// -> metadata for prefixB:
	metaMap = graphR.GetMetadataMap(metadataMapB)
	Expect(metaMap).ToNot(BeNil())
	checkMetadataValues(metaMap, value4Label)
	intMap = metaMap.(NameToInteger)
	label, metadata, exists = intMap.LookupByIndex(1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(Equal(1))
	Expect(label).To(Equal(value4Label))
	graphR.Release()
}

func TestReuseNodeAfterSave(t *testing.T) {
	RegisterTestingT(t)

	graph := NewGraph(true, minutesInOneDay, minutesInOneHour)
	graphW := graph.Write(true)

	// add new node
	nodeW := graphW.SetNode(keyA1)
	nodeW.SetValue(value1)
	nodeW.SetFlags(ColorFlag(Red))

	// save new node
	graphW.Save()

	// keep using the same node handle
	nodeW.SetFlags(AbstractFlag())

	// save changes
	graphW.Save()

	// get new handle
	nodeW = graphW.SetNode(keyA1)
	nodeW.SetFlags(TemporaryFlag())

	// save changes
	graphW.Save()
	graphW.Release()

	// check that all 3 flags are applied
	graphR := graph.Read()
	checkNodes(graphR.GetNodes(nil, WithFlags(ColorFlag(Red), AbstractFlag(), TemporaryFlag())), keyA1)
	graphR.Release()
}
