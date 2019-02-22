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

/* TODO: fix and re-enable UTs
import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/gogo/protobuf/proto"
	. "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/test"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
)

func TestEmptyResync(t *testing.T) {
	RegisterTestingT(t)

	// prepare KV Scheduler
	scheduler := NewPlugin(UseDeps(func(deps *Deps) {
		deps.HTTPHandlers = nil
	}))
	err := scheduler.Init()
	Expect(err).To(BeNil())

	// prepare mocks
	mockSB := test.NewMockSouthbound()
	descriptor1 := test.NewMockDescriptor(&KVDescriptor{
		Name:         descriptor1Name,
		NBKeyPrefix:  prefixA,
		KeySelector:  prefixSelector(prefixA),
		WithMetadata: true,
	}, mockSB, 0)

	// register descriptor with the scheduler
	scheduler.RegisterKVDescriptor(descriptor1)
	nbPrefixes := scheduler.GetRegisteredNBKeyPrefixes()
	Expect(nbPrefixes).To(HaveLen(1))
	Expect(nbPrefixes).To(ContainElement(prefixA))

	// get metadata map created for the descriptor
	metadataMap := scheduler.GetMetadataMap(descriptor1.Name)
	_, withMetadataMap := metadataMap.(test.NameToInteger)
	Expect(withMetadataMap).To(BeTrue())

	// transaction history should be initially empty
	Expect(scheduler.GetTransactionHistory(time.Time{}, time.Time{})).To(BeEmpty())

	// run transaction with empty resync
	startTime := time.Now()
	ctx := WithResync(context.Background(), FullResync, true)
	description := "testing empty resync"
	ctx = WithDescription(ctx, description)
	seqNum, err := scheduler.StartNBTransaction().Commit(ctx)
	stopTime := time.Now()
	Expect(seqNum).To(BeEquivalentTo(0))
	Expect(err).ShouldNot(HaveOccurred())

	// check the state of SB
	Expect(mockSB.GetKeysWithInvalidData()).To(BeEmpty())
	Expect(mockSB.GetValues(nil)).To(BeEmpty())

	// check metadata
	Expect(metadataMap.ListAllNames()).To(BeEmpty())

	// check executed operations
	opHistory := mockSB.PopHistoryOfOps()
	Expect(opHistory).To(HaveLen(1))
	Expect(opHistory[0].OpType).To(Equal(test.MockDump))
	Expect(opHistory[0].CorrelateDump).To(BeEmpty())
	Expect(opHistory[0].Descriptor).To(BeEquivalentTo(descriptor1Name))

	// single transaction consisted of zero operations
	txnHistory := scheduler.GetTransactionHistory(time.Time{}, time.Time{})
	Expect(txnHistory).To(HaveLen(1))
	txn := txnHistory[0]
	Expect(txn.PreRecord).To(BeFalse())
	Expect(txn.Start.After(startTime)).To(BeTrue())
	Expect(txn.Start.Before(txn.Stop)).To(BeTrue())
	Expect(txn.Stop.Before(stopTime)).To(BeTrue())
	Expect(txn.SeqNum).To(BeEquivalentTo(0))
	Expect(txn.TxnType).To(BeEquivalentTo(NBTransaction))
	Expect(txn.ResyncType).To(BeEquivalentTo(FullResync))
	Expect(txn.Description).To(Equal(description))
	Expect(txn.Values).To(BeEmpty())
	Expect(txn.PreErrors).To(BeEmpty())
	Expect(txn.Planned).To(BeEmpty())
	Expect(txn.Executed).To(BeEmpty())

	// check flag stats
	graphR := scheduler.graph.Read()
	errorStats := graphR.GetFlagStats(ErrorFlagName, nil)
	Expect(errorStats.TotalCount).To(BeEquivalentTo(0))
	pendingStats := graphR.GetFlagStats(PendingFlagName, nil)
	Expect(pendingStats.TotalCount).To(BeEquivalentTo(0))
	derivedStats := graphR.GetFlagStats(DerivedFlagName, nil)
	Expect(derivedStats.TotalCount).To(BeEquivalentTo(0))
	lastUpdateStats := graphR.GetFlagStats(LastUpdateFlagName, nil)
	Expect(lastUpdateStats.TotalCount).To(BeEquivalentTo(0))
	lastChangeStats := graphR.GetFlagStats(LastChangeFlagName, nil)
	Expect(lastChangeStats.TotalCount).To(BeEquivalentTo(0))
	descriptorStats := graphR.GetFlagStats(DescriptorFlagName, nil)
	Expect(descriptorStats.TotalCount).To(BeEquivalentTo(0))
	originStats := graphR.GetFlagStats(OriginFlagName, nil)
	Expect(originStats.TotalCount).To(BeEquivalentTo(0))
	graphR.Release()

	// close scheduler
	err = scheduler.Close()
	Expect(err).To(BeNil())
}

func TestResyncWithEmptySB(t *testing.T) {
	RegisterTestingT(t)

	// prepare KV Scheduler
	scheduler := NewPlugin(UseDeps(func(deps *Deps) {
		deps.HTTPHandlers = nil
	}))
	err := scheduler.Init()
	Expect(err).To(BeNil())

	// prepare mocks
	mockSB := test.NewMockSouthbound()
	descriptor1 := test.NewMockDescriptor(&KVDescriptor{
		Name:          descriptor1Name,
		NBKeyPrefix:   prefixA,
		KeySelector:   prefixSelector(prefixA),
		ValueTypeName: proto.MessageName(test.NewArrayValue()),
		DerivedValues: test.ArrayValueDerBuilder,
		Dependencies: func(key string, value proto.Message) []Dependency {
			if key == prefixA+baseValue2 {
				depKey := prefixA + baseValue1 + "/item1" // base value depends on a derived value
				return []Dependency{
					{Label: depKey, Key: depKey},
				}
			}
			if key == prefixA+baseValue1+"/item2" {
				depKey := prefixA + baseValue2 + "/item1" // derived value depends on another derived value
				return []Dependency{
					{Label: depKey, Key: depKey},
				}
			}
			return nil
		},
		WithMetadata: true,
	}, mockSB, 0)

	// register descriptor with the scheduler
	scheduler.RegisterKVDescriptor(descriptor1)

	// get metadata map created for the descriptor
	metadataMap := scheduler.GetMetadataMap(descriptor1.Name)
	nameToInteger, withMetadataMap := metadataMap.(test.NameToInteger)
	Expect(withMetadataMap).To(BeTrue())

	// run resync transaction with empty SB
	startTime := time.Now()
	schedulerTxn := scheduler.StartNBTransaction()
	schedulerTxn.SetValue(prefixA+baseValue2, test.NewLazyArrayValue("item1"))
	schedulerTxn.SetValue(prefixA+baseValue1, test.NewLazyArrayValue("item1", "item2"))
	ctx := WithResync(context.Background(), FullResync, true)
	description := "testing resync against empty SB"
	ctx = WithDescription(ctx, description)
	seqNum, err := schedulerTxn.Commit(ctx)
	stopTime := time.Now()
	Expect(seqNum).To(BeEquivalentTo(0))
	Expect(err).ShouldNot(HaveOccurred())

	// check the state of SB
	Expect(mockSB.GetKeysWithInvalidData()).To(BeEmpty())
	// -> base value 1
	value := mockSB.GetValue(prefixA + baseValue1)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item1", "item2"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(0))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 1
	value = mockSB.GetValue(prefixA + baseValue1 + "/item1")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item1"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item2 derived from base value 1
	value = mockSB.GetValue(prefixA + baseValue1 + "/item2")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item2"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> base value 2
	value = mockSB.GetValue(prefixA + baseValue2)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item1"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(1))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 2
	value = mockSB.GetValue(prefixA + baseValue2 + "/item1")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item1"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	Expect(mockSB.GetValues(nil)).To(HaveLen(5))

	// check scheduler API
	prefixAValues := scheduler.GetValues(prefixSelector(prefixA))
	checkValues(prefixAValues, []KeyValuePair{
		{Key: prefixA + baseValue1, Value: test.NewArrayValue("item1", "item2")},
		{Key: prefixA + baseValue1 + "/item1", Value: test.NewStringValue("item1")},
		{Key: prefixA + baseValue1 + "/item2", Value: test.NewStringValue("item2")},
		{Key: prefixA + baseValue2, Value: test.NewArrayValue("item1")},
		{Key: prefixA + baseValue2 + "/item1", Value: test.NewStringValue("item1")},
	})
	Expect(proto.Equal(scheduler.GetValue(prefixA+baseValue1), test.NewArrayValue("item1", "item2"))).To(BeTrue())
	Expect(proto.Equal(scheduler.GetValue(prefixA+baseValue1+"/item1"), test.NewStringValue("item1"))).To(BeTrue())
	Expect(scheduler.GetFailedValues(nil)).To(BeEmpty())
	Expect(scheduler.GetPendingValues(nil)).To(BeEmpty())

	// check metadata
	metadata, exists := nameToInteger.LookupByName(baseValue1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(0))
	metadata, exists = nameToInteger.LookupByName(baseValue2)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(1))

	// check executed operations
	opHistory := mockSB.PopHistoryOfOps()
	Expect(opHistory).To(HaveLen(6))
	operation := opHistory[0]
	Expect(operation.OpType).To(Equal(test.MockDump))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	checkValuesForCorrelation(operation.CorrelateDump, []KVWithMetadata{
		{
			Key:      prefixA + baseValue1,
			Value:    test.NewArrayValue("item1", "item2"),
			Metadata: nil,
			Origin:   FromNB,
		},
		{
			Key:      prefixA + baseValue2,
			Value:    test.NewArrayValue("item1"),
			Metadata: nil,
			Origin:   FromNB,
		},
	})
	operation = opHistory[1]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[2]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[3]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue2))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[4]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue2 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[5]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item2"))
	Expect(operation.Err).To(BeNil())

	// single transaction consisted of 6 operations
	txnHistory := scheduler.GetTransactionHistory(time.Time{}, time.Now())
	Expect(txnHistory).To(HaveLen(1))
	txn := txnHistory[0]
	Expect(txn.PreRecord).To(BeFalse())
	Expect(txn.Start.After(startTime)).To(BeTrue())
	Expect(txn.Start.Before(txn.Stop)).To(BeTrue())
	Expect(txn.Stop.Before(stopTime)).To(BeTrue())
	Expect(txn.SeqNum).To(BeEquivalentTo(0))
	Expect(txn.TxnType).To(BeEquivalentTo(NBTransaction))
	Expect(txn.ResyncType).To(BeEquivalentTo(FullResync))
	Expect(txn.Description).To(Equal(description))
	checkRecordedValues(txn.Values, []RecordedKVPair{
		{Key: prefixA + baseValue1, Value: utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")), Origin: FromNB},
		{Key: prefixA + baseValue2, Value: utils.RecordProtoMessage(test.NewArrayValue("item1")), Origin: FromNB},
	})
	Expect(txn.PreErrors).To(BeEmpty())

	txnOps := RecordedTxnOps{
		{
			Operation:  Add,
			Key:        prefixA + baseValue1,
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue1 + "/item1",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue2,
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue2 + "/item1",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue1 + "/item2",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
	}
	checkTxnOperations(txn.Planned, txnOps)
	checkTxnOperations(txn.Executed, txnOps)

	// now remove everything using resync with empty data
	startTime = time.Now()
	seqNum, err = scheduler.StartNBTransaction().Commit(WithResync(context.Background(), FullResync, true))
	stopTime = time.Now()
	Expect(seqNum).To(BeEquivalentTo(1))
	Expect(err).ShouldNot(HaveOccurred())

	// check the state of SB
	Expect(mockSB.GetKeysWithInvalidData()).To(BeEmpty())
	Expect(mockSB.GetValues(nil)).To(BeEmpty())

	// check metadata
	Expect(metadataMap.ListAllNames()).To(BeEmpty())

	// check executed operations
	opHistory = mockSB.PopHistoryOfOps()
	Expect(opHistory).To(HaveLen(6))
	operation = opHistory[0]
	Expect(operation.OpType).To(Equal(test.MockDump))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	checkValuesForCorrelation(operation.CorrelateDump, []KVWithMetadata{
		{
			Key:      prefixA + baseValue1,
			Value:    test.NewArrayValue("item1", "item2"),
			Metadata: &test.OnlyInteger{Integer: 0},
			Origin:   FromNB,
		},
		{
			Key:      prefixA + baseValue2,
			Value:    test.NewArrayValue("item1"),
			Metadata: &test.OnlyInteger{Integer: 1},
			Origin:   FromNB,
		},
	})
	operation = opHistory[1]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item2"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[2]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue2 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[3]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue2))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[4]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[5]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1))
	Expect(operation.Err).To(BeNil())

	// this second transaction consisted of 6 operations
	txnHistory = scheduler.GetTransactionHistory(time.Time{}, time.Now())
	Expect(txnHistory).To(HaveLen(2))
	txn = txnHistory[1]
	Expect(txn.PreRecord).To(BeFalse())
	Expect(txn.Start.After(startTime)).To(BeTrue())
	Expect(txn.Start.Before(txn.Stop)).To(BeTrue())
	Expect(txn.Stop.Before(stopTime)).To(BeTrue())
	Expect(txn.SeqNum).To(BeEquivalentTo(1))
	Expect(txn.TxnType).To(BeEquivalentTo(NBTransaction))
	Expect(txn.ResyncType).To(BeEquivalentTo(FullResync))
	Expect(txn.Description).To(BeEmpty())
	checkRecordedValues(txn.Values, []RecordedKVPair{
		{Key: prefixA + baseValue1, Value: utils.RecordProtoMessage(nil), Origin: FromNB},
		{Key: prefixA + baseValue2, Value: utils.RecordProtoMessage(nil), Origin: FromNB},
	})
	Expect(txn.PreErrors).To(BeEmpty())

	txnOps = RecordedTxnOps{
		{
			Operation:  Delete,
			Key:        prefixA + baseValue1 + "/item2",
			Derived:    true,
			PrevValue:  utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Delete,
			Key:        prefixA + baseValue2 + "/item1",
			Derived:    true,
			PrevValue:  utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Delete,
			Key:        prefixA + baseValue2,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Delete,
			Key:        prefixA + baseValue1 + "/item1",
			Derived:    true,
			PrevValue:  utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Delete,
			Key:        prefixA + baseValue1,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
	}
	checkTxnOperations(txn.Planned, txnOps)
	checkTxnOperations(txn.Executed, txnOps)

	// check flag stats
	graphR := scheduler.graph.Read()
	errorStats := graphR.GetFlagStats(ErrorFlagName, nil)
	Expect(errorStats.TotalCount).To(BeEquivalentTo(0))
	pendingStats := graphR.GetFlagStats(PendingFlagName, nil)
	Expect(pendingStats.TotalCount).To(BeEquivalentTo(0))
	derivedStats := graphR.GetFlagStats(DerivedFlagName, nil)
	Expect(derivedStats.TotalCount).To(BeEquivalentTo(3))
	lastUpdateStats := graphR.GetFlagStats(LastUpdateFlagName, nil)
	Expect(lastUpdateStats.TotalCount).To(BeEquivalentTo(5))
	lastChangeStats := graphR.GetFlagStats(LastChangeFlagName, nil)
	Expect(lastChangeStats.TotalCount).To(BeEquivalentTo(2))
	descriptorStats := graphR.GetFlagStats(DescriptorFlagName, nil)
	Expect(descriptorStats.TotalCount).To(BeEquivalentTo(5))
	Expect(descriptorStats.PerValueCount).To(HaveKey(descriptor1Name))
	Expect(descriptorStats.PerValueCount[descriptor1Name]).To(BeEquivalentTo(5))
	originStats := graphR.GetFlagStats(OriginFlagName, nil)
	Expect(originStats.TotalCount).To(BeEquivalentTo(5))
	Expect(originStats.PerValueCount).To(HaveKey(FromNB.String()))
	Expect(originStats.PerValueCount[FromNB.String()]).To(BeEquivalentTo(5))
	graphR.Release()

	// close scheduler
	err = scheduler.Close()
	Expect(err).To(BeNil())
}

func TestResyncWithNonEmptySB(t *testing.T) {
	RegisterTestingT(t)

	// prepare KV Scheduler
	scheduler := NewPlugin(UseDeps(func(deps *Deps) {
		deps.HTTPHandlers = nil
	}))
	err := scheduler.Init()
	Expect(err).To(BeNil())

	// prepare mocks
	mockSB := test.NewMockSouthbound()
	// -> initial content:
	mockSB.SetValue(prefixA+baseValue1, test.NewArrayValue("item1"),
		&test.OnlyInteger{Integer: 0}, FromNB, false)
	mockSB.SetValue(prefixA+baseValue1+"/item1", test.NewStringValue("item1"),
		nil, FromNB, true)
	mockSB.SetValue(prefixA+baseValue2, test.NewArrayValue("item1"),
		&test.OnlyInteger{Integer: 1}, FromNB, false)
	mockSB.SetValue(prefixA+baseValue2+"/item1", test.NewStringValue("item1"),
		nil, FromNB, true)
	mockSB.SetValue(prefixA+baseValue3, test.NewArrayValue("item1"),
		&test.OnlyInteger{Integer: 2}, FromNB, false)
	mockSB.SetValue(prefixA+baseValue3+"/item1", test.NewStringValue("item1"),
		nil, FromNB, true)
	// -> descriptor1:
	descriptor1 := test.NewMockDescriptor(&KVDescriptor{
		Name:          descriptor1Name,
		NBKeyPrefix:   prefixA,
		KeySelector:   prefixSelector(prefixA),
		ValueTypeName: proto.MessageName(test.NewArrayValue()),
		DerivedValues: test.ArrayValueDerBuilder,
		Dependencies: func(key string, value proto.Message) []Dependency {
			if key == prefixA+baseValue2+"/item1" {
				depKey := prefixA + baseValue1
				return []Dependency{
					{Label: depKey, Key: depKey},
				}
			}
			if key == prefixA+baseValue2+"/item2" {
				depKey := prefixA + baseValue1 + "/item1"
				return []Dependency{
					{Label: depKey, Key: depKey},
				}
			}
			return nil
		},
		ModifyWithRecreate: func(key string, oldValue, newValue proto.Message, metadata Metadata) bool {
			return key == prefixA+baseValue3
		},
		WithMetadata: true,
	}, mockSB, 3)

	// register descriptor with the scheduler
	scheduler.RegisterKVDescriptor(descriptor1)

	// get metadata map created for the descriptor
	metadataMap := scheduler.GetMetadataMap(descriptor1.Name)
	nameToInteger, withMetadataMap := metadataMap.(test.NameToInteger)
	Expect(withMetadataMap).To(BeTrue())

	// run resync transaction with SB that already has some values added
	startTime := time.Now()
	schedulerTxn := scheduler.StartNBTransaction()
	schedulerTxn.SetValue(prefixA+baseValue2, test.NewLazyArrayValue("item1", "item2"))
	schedulerTxn.SetValue(prefixA+baseValue1, test.NewLazyArrayValue("item2"))
	schedulerTxn.SetValue(prefixA+baseValue3, test.NewLazyArrayValue("item1", "item2"))
	seqNum, err := schedulerTxn.Commit(WithResync(context.Background(), FullResync, true))
	stopTime := time.Now()
	Expect(seqNum).To(BeEquivalentTo(0))
	Expect(err).ShouldNot(HaveOccurred())

	// check the state of SB
	Expect(mockSB.GetKeysWithInvalidData()).To(BeEmpty())
	// -> base value 1
	value := mockSB.GetValue(prefixA + baseValue1)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item2"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(0))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 1 was removed
	value = mockSB.GetValue(prefixA + baseValue1 + "/item1")
	Expect(value).To(BeNil())
	// -> item2 derived from base value 1
	value = mockSB.GetValue(prefixA + baseValue1 + "/item2")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item2"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> base value 2
	value = mockSB.GetValue(prefixA + baseValue2)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item1", "item2"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(1))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 2
	value = mockSB.GetValue(prefixA + baseValue2 + "/item1")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item1"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item2 derived from base value 2 is pending
	value = mockSB.GetValue(prefixA + baseValue2 + "/item2")
	Expect(value).To(BeNil())
	// -> base value 3
	value = mockSB.GetValue(prefixA + baseValue3)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item1", "item2"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(3))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 3
	value = mockSB.GetValue(prefixA + baseValue3 + "/item1")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item1"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item2 derived from base value 3
	value = mockSB.GetValue(prefixA + baseValue3 + "/item2")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item2"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	Expect(mockSB.GetValues(nil)).To(HaveLen(7))

	// check metadata
	metadata, exists := nameToInteger.LookupByName(baseValue1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(0))
	metadata, exists = nameToInteger.LookupByName(baseValue2)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(1))
	metadata, exists = nameToInteger.LookupByName(baseValue3)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(3))

	// check operations executed in SB
	opHistory := mockSB.PopHistoryOfOps()
	Expect(opHistory).To(HaveLen(11))
	operation := opHistory[0]
	Expect(operation.OpType).To(Equal(test.MockDump))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	checkValuesForCorrelation(operation.CorrelateDump, []KVWithMetadata{
		{
			Key:      prefixA + baseValue1,
			Value:    test.NewArrayValue("item2"),
			Metadata: nil,
			Origin:   FromNB,
		},
		{
			Key:      prefixA + baseValue2,
			Value:    test.NewArrayValue("item1", "item2"),
			Metadata: nil,
			Origin:   FromNB,
		},
		{
			Key:      prefixA + baseValue3,
			Value:    test.NewArrayValue("item1", "item2"),
			Metadata: nil,
			Origin:   FromNB,
		},
	})
	operation = opHistory[1]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue3 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[2]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue3))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[3]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue3))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[4]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue3 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[5]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue3 + "/item2"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[6]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[7]
	Expect(operation.OpType).To(Equal(test.MockModify))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[8]
	Expect(operation.OpType).To(Equal(test.MockUpdate))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue2 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[9]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item2"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[10]
	Expect(operation.OpType).To(Equal(test.MockModify))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue2))
	Expect(operation.Err).To(BeNil())

	// check transaction operations
	txnHistory := scheduler.GetTransactionHistory(time.Time{}, time.Time{})
	Expect(txnHistory).To(HaveLen(1))
	txn := txnHistory[0]
	Expect(txn.PreRecord).To(BeFalse())
	Expect(txn.Start.After(startTime)).To(BeTrue())
	Expect(txn.Start.Before(txn.Stop)).To(BeTrue())
	Expect(txn.Stop.Before(stopTime)).To(BeTrue())
	Expect(txn.SeqNum).To(BeEquivalentTo(0))
	Expect(txn.TxnType).To(BeEquivalentTo(NBTransaction))
	Expect(txn.ResyncType).To(BeEquivalentTo(FullResync))
	Expect(txn.Description).To(BeEmpty())
	checkRecordedValues(txn.Values, []RecordedKVPair{
		{Key: prefixA + baseValue1, Value: utils.RecordProtoMessage(test.NewArrayValue("item2")), Origin: FromNB},
		{Key: prefixA + baseValue2, Value: utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")), Origin: FromNB},
		{Key: prefixA + baseValue3, Value: utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")), Origin: FromNB},
	})
	Expect(txn.PreErrors).To(BeEmpty())

	txnOps := RecordedTxnOps{
		{
			Operation:  Delete,
			Key:        prefixA + baseValue3 + "/item1",
			Derived:    true,
			PrevValue:  utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Delete,
			Key:        prefixA + baseValue3,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
			IsPending:  true,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue3,
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
			WasPending: true,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue3 + "/item1",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue3 + "/item2",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Delete,
			Key:        prefixA + baseValue1 + "/item1",
			Derived:    true,
			PrevValue:  utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Modify,
			Key:        prefixA + baseValue1,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue("item1")),
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Update,
			Key:        prefixA + baseValue2 + "/item1",
			Derived:    true,
			PrevValue:  utils.RecordProtoMessage(test.NewStringValue("item1")),
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue1 + "/item2",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Modify,
			Key:        prefixA + baseValue2,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue("item1")),
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue2 + "/item2",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
			IsPending:  true,
		},
	}
	checkTxnOperations(txn.Planned, txnOps)
	checkTxnOperations(txn.Executed, txnOps)

	// check flag stats
	graphR := scheduler.graph.Read()
	errorStats := graphR.GetFlagStats(ErrorFlagName, nil)
	Expect(errorStats.TotalCount).To(BeEquivalentTo(0))
	pendingStats := graphR.GetFlagStats(PendingFlagName, nil)
	Expect(pendingStats.TotalCount).To(BeEquivalentTo(1))
	derivedStats := graphR.GetFlagStats(DerivedFlagName, nil)
	Expect(derivedStats.TotalCount).To(BeEquivalentTo(5))
	lastUpdateStats := graphR.GetFlagStats(LastUpdateFlagName, nil)
	Expect(lastUpdateStats.TotalCount).To(BeEquivalentTo(8))
	lastChangeStats := graphR.GetFlagStats(LastChangeFlagName, nil)
	Expect(lastChangeStats.TotalCount).To(BeEquivalentTo(3))
	descriptorStats := graphR.GetFlagStats(DescriptorFlagName, nil)
	Expect(descriptorStats.TotalCount).To(BeEquivalentTo(8))
	Expect(descriptorStats.PerValueCount).To(HaveKey(descriptor1Name))
	Expect(descriptorStats.PerValueCount[descriptor1Name]).To(BeEquivalentTo(8))
	originStats := graphR.GetFlagStats(OriginFlagName, nil)
	Expect(originStats.TotalCount).To(BeEquivalentTo(8))
	Expect(originStats.PerValueCount).To(HaveKey(FromNB.String()))
	Expect(originStats.PerValueCount[FromNB.String()]).To(BeEquivalentTo(8))
	graphR.Release()

	// close scheduler
	err = scheduler.Close()
	Expect(err).To(BeNil())
}

func TestResyncNotRemovingSBValues(t *testing.T) {
	RegisterTestingT(t)

	// prepare KV Scheduler
	scheduler := NewPlugin(UseDeps(func(deps *Deps) {
		deps.HTTPHandlers = nil
	}))
	err := scheduler.Init()
	Expect(err).To(BeNil())

	// prepare mocks
	mockSB := test.NewMockSouthbound()
	// -> initial content:
	mockSB.SetValue(prefixA+baseValue1, test.NewStringValue(baseValue1),
		nil, FromSB, false)
	// -> descriptor1:
	descriptor1 := test.NewMockDescriptor(&KVDescriptor{
		Name:          descriptor1Name,
		KeySelector:   prefixSelector(prefixA),
		NBKeyPrefix:   prefixA,
		ValueTypeName: proto.MessageName(test.NewArrayValue()),
		DerivedValues: test.ArrayValueDerBuilder,
		Dependencies: func(key string, value proto.Message) []Dependency {
			if key == prefixA+baseValue2 {
				depKey := prefixA + baseValue1
				return []Dependency{
					{Label: depKey, Key: depKey},
				}
			}
			return nil
		},
		WithMetadata: true,
	}, mockSB, 0)

	// register descriptor with the scheduler
	scheduler.RegisterKVDescriptor(descriptor1)

	// get metadata map created for the descriptor
	metadataMap := scheduler.GetMetadataMap(descriptor1.Name)
	nameToInteger, withMetadataMap := metadataMap.(test.NameToInteger)
	Expect(withMetadataMap).To(BeTrue())

	// run resync transaction that should keep values not managed by NB untouched
	startTime := time.Now()
	schedulerTxn := scheduler.StartNBTransaction()
	schedulerTxn.SetValue(prefixA+baseValue2, test.NewLazyArrayValue("item1"))
	seqNum, err := schedulerTxn.Commit(WithResync(context.Background(), FullResync, true))
	stopTime := time.Now()
	Expect(seqNum).To(BeEquivalentTo(0))
	Expect(err).ShouldNot(HaveOccurred())

	// check the state of SB
	Expect(mockSB.GetKeysWithInvalidData()).To(BeEmpty())
	// -> base value 1
	value := mockSB.GetValue(prefixA + baseValue1)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue(baseValue1))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromSB))
	// -> base value 2
	value = mockSB.GetValue(prefixA + baseValue2)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item1"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(0))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 2
	value = mockSB.GetValue(prefixA + baseValue2 + "/item1")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item1"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	Expect(mockSB.GetValues(nil)).To(HaveLen(3))

	// check metadata
	metadata, exists := nameToInteger.LookupByName(baseValue1)
	Expect(exists).To(BeFalse())
	Expect(metadata).To(BeNil())
	metadata, exists = nameToInteger.LookupByName(baseValue2)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(0))

	// check operations executed in SB
	opHistory := mockSB.PopHistoryOfOps()
	Expect(opHistory).To(HaveLen(3))
	operation := opHistory[0]
	Expect(operation.OpType).To(Equal(test.MockDump))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	checkValuesForCorrelation(operation.CorrelateDump, []KVWithMetadata{
		{
			Key:      prefixA + baseValue2,
			Value:    test.NewArrayValue("item1"),
			Metadata: nil,
			Origin:   FromNB,
		},
	})
	operation = opHistory[1]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue2))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[2]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue2 + "/item1"))
	Expect(operation.Err).To(BeNil())

	// check transaction operations
	txnHistory := scheduler.GetTransactionHistory(startTime, time.Now())
	Expect(txnHistory).To(HaveLen(1))
	txn := txnHistory[0]
	Expect(txn.PreRecord).To(BeFalse())
	Expect(txn.Start.After(startTime)).To(BeTrue())
	Expect(txn.Start.Before(txn.Stop)).To(BeTrue())
	Expect(txn.Stop.Before(stopTime)).To(BeTrue())
	Expect(txn.SeqNum).To(BeEquivalentTo(0))
	Expect(txn.TxnType).To(BeEquivalentTo(NBTransaction))
	Expect(txn.ResyncType).To(BeEquivalentTo(FullResync))
	Expect(txn.Description).To(BeEmpty())
	checkRecordedValues(txn.Values, []RecordedKVPair{
		{Key: prefixA + baseValue1, Value: utils.RecordProtoMessage(test.NewStringValue(baseValue1)), Origin: FromSB},
		{Key: prefixA + baseValue2, Value: utils.RecordProtoMessage(test.NewArrayValue("item1")), Origin: FromNB},
	})
	Expect(txn.PreErrors).To(BeEmpty())

	txnOps := RecordedTxnOps{
		{
			Operation:  Add,
			Key:        prefixA + baseValue2,
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue2 + "/item1",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
	}
	checkTxnOperations(txn.Planned, txnOps)
	checkTxnOperations(txn.Executed, txnOps)

	// check flag stats
	graphR := scheduler.graph.Read()
	errorStats := graphR.GetFlagStats(ErrorFlagName, nil)
	Expect(errorStats.TotalCount).To(BeEquivalentTo(0))
	pendingStats := graphR.GetFlagStats(PendingFlagName, nil)
	Expect(pendingStats.TotalCount).To(BeEquivalentTo(0))
	derivedStats := graphR.GetFlagStats(DerivedFlagName, nil)
	Expect(derivedStats.TotalCount).To(BeEquivalentTo(1))
	lastUpdateStats := graphR.GetFlagStats(LastUpdateFlagName, nil)
	Expect(lastUpdateStats.TotalCount).To(BeEquivalentTo(3))
	lastChangeStats := graphR.GetFlagStats(LastChangeFlagName, nil)
	Expect(lastChangeStats.TotalCount).To(BeEquivalentTo(2))
	descriptorStats := graphR.GetFlagStats(DescriptorFlagName, nil)
	Expect(descriptorStats.TotalCount).To(BeEquivalentTo(3))
	Expect(descriptorStats.PerValueCount).To(HaveKey(descriptor1Name))
	Expect(descriptorStats.PerValueCount[descriptor1Name]).To(BeEquivalentTo(3))
	originStats := graphR.GetFlagStats(OriginFlagName, nil)
	Expect(originStats.TotalCount).To(BeEquivalentTo(3))
	Expect(originStats.PerValueCount).To(HaveKey(FromNB.String()))
	Expect(originStats.PerValueCount[FromNB.String()]).To(BeEquivalentTo(2))
	Expect(originStats.PerValueCount).To(HaveKey(FromSB.String()))
	Expect(originStats.PerValueCount[FromSB.String()]).To(BeEquivalentTo(1))
	graphR.Release()

	// close scheduler
	err = scheduler.Close()
	Expect(err).To(BeNil())
}

func TestResyncWithMultipleDescriptors(t *testing.T) {
	RegisterTestingT(t)

	// prepare KV Scheduler
	scheduler := NewPlugin(UseDeps(func(deps *Deps) {
		deps.HTTPHandlers = nil
	}))
	err := scheduler.Init()
	Expect(err).To(BeNil())

	// prepare mocks
	mockSB := test.NewMockSouthbound()
	// -> initial content:
	mockSB.SetValue(prefixA+baseValue1, test.NewArrayValue("item1"),
		&test.OnlyInteger{Integer: 0}, FromNB, false)
	mockSB.SetValue(prefixA+baseValue1+"/item1", test.NewStringValue("item1"),
		nil, FromNB, true)
	mockSB.SetValue(prefixB+baseValue2, test.NewArrayValue("item1"),
		&test.OnlyInteger{Integer: 0}, FromNB, false)
	mockSB.SetValue(prefixB+baseValue2+"/item1", test.NewStringValue("item1"),
		nil, FromNB, true)
	mockSB.SetValue(prefixC+baseValue3, test.NewArrayValue("item1"),
		&test.OnlyInteger{Integer: 0}, FromNB, false)
	mockSB.SetValue(prefixC+baseValue3+"/item1", test.NewStringValue("item1"),
		nil, FromNB, true)
	// -> descriptor1:
	descriptor1 := test.NewMockDescriptor(&KVDescriptor{
		Name:          descriptor1Name,
		NBKeyPrefix:   prefixA,
		KeySelector:   prefixSelector(prefixA),
		ValueTypeName: proto.MessageName(test.NewArrayValue()),
		DerivedValues: test.ArrayValueDerBuilder,
		WithMetadata:  true,
	}, mockSB, 1)
	// -> descriptor2:
	descriptor2 := test.NewMockDescriptor(&KVDescriptor{
		Name:          descriptor2Name,
		NBKeyPrefix:   prefixB,
		KeySelector:   prefixSelector(prefixB),
		ValueTypeName: proto.MessageName(test.NewArrayValue()),
		DerivedValues: test.ArrayValueDerBuilder,
		Dependencies: func(key string, value proto.Message) []Dependency {
			if key == prefixB+baseValue2+"/item1" {
				depKey := prefixA + baseValue1
				return []Dependency{
					{Label: depKey, Key: depKey},
				}
			}
			if key == prefixB+baseValue2+"/item2" {
				depKey := prefixA + baseValue1 + "/item1"
				return []Dependency{
					{Label: depKey, Key: depKey},
				}
			}
			return nil
		},
		WithMetadata:     true,
		DumpDependencies: []string{descriptor1Name},
	}, mockSB, 1)
	// -> descriptor3:
	descriptor3 := test.NewMockDescriptor(&KVDescriptor{
		Name:          descriptor3Name,
		NBKeyPrefix:   prefixC,
		KeySelector:   prefixSelector(prefixC),
		ValueTypeName: proto.MessageName(test.NewArrayValue()),
		DerivedValues: test.ArrayValueDerBuilder,
		ModifyWithRecreate: func(key string, oldValue, newValue proto.Message, metadata Metadata) bool {
			return key == prefixC+baseValue3
		},
		WithMetadata:     true,
		DumpDependencies: []string{descriptor2Name},
	}, mockSB, 1)

	// register all 3 descriptors with the scheduler
	scheduler.RegisterKVDescriptor(descriptor1)
	scheduler.RegisterKVDescriptor(descriptor2)
	scheduler.RegisterKVDescriptor(descriptor3)
	nbPrefixes := scheduler.GetRegisteredNBKeyPrefixes()
	Expect(nbPrefixes).To(HaveLen(3))
	Expect(nbPrefixes).To(ContainElement(prefixA))
	Expect(nbPrefixes).To(ContainElement(prefixB))
	Expect(nbPrefixes).To(ContainElement(prefixC))

	// get metadata map created for each descriptor
	metadataMap := scheduler.GetMetadataMap(descriptor1.Name)
	nameToInteger1, withMetadataMap := metadataMap.(test.NameToInteger)
	Expect(withMetadataMap).To(BeTrue())
	metadataMap = scheduler.GetMetadataMap(descriptor2.Name)
	nameToInteger2, withMetadataMap := metadataMap.(test.NameToInteger)
	Expect(withMetadataMap).To(BeTrue())
	metadataMap = scheduler.GetMetadataMap(descriptor3.Name)
	nameToInteger3, withMetadataMap := metadataMap.(test.NameToInteger)
	Expect(withMetadataMap).To(BeTrue())

	// run resync transaction with SB that already has some values added
	startTime := time.Now()
	schedulerTxn := scheduler.StartNBTransaction()
	schedulerTxn.SetValue(prefixB+baseValue2, test.NewLazyArrayValue("item1", "item2"))
	schedulerTxn.SetValue(prefixA+baseValue1, test.NewLazyArrayValue("item2"))
	schedulerTxn.SetValue(prefixC+baseValue3, test.NewLazyArrayValue("item1", "item2"))
	seqNum, err := schedulerTxn.Commit(WithResync(context.Background(), FullResync, true))
	stopTime := time.Now()
	Expect(seqNum).To(BeEquivalentTo(0))
	Expect(err).ShouldNot(HaveOccurred())

	// check the state of SB
	Expect(mockSB.GetKeysWithInvalidData()).To(BeEmpty())
	// -> base value 1
	value := mockSB.GetValue(prefixA + baseValue1)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item2"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(0))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 1 was removed
	value = mockSB.GetValue(prefixA + baseValue1 + "/item1")
	Expect(value).To(BeNil())
	// -> item2 derived from base value 1
	value = mockSB.GetValue(prefixA + baseValue1 + "/item2")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item2"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> base value 2
	value = mockSB.GetValue(prefixB + baseValue2)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item1", "item2"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(0))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 2
	value = mockSB.GetValue(prefixB + baseValue2 + "/item1")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item1"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item2 derived from base value 2 is pending
	value = mockSB.GetValue(prefixB + baseValue2 + "/item2")
	Expect(value).To(BeNil())
	// -> base value 3
	value = mockSB.GetValue(prefixC + baseValue3)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item1", "item2"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(1))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 3
	value = mockSB.GetValue(prefixC + baseValue3 + "/item1")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item1"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item2 derived from base value 3
	value = mockSB.GetValue(prefixC + baseValue3 + "/item2")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item2"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	Expect(mockSB.GetValues(nil)).To(HaveLen(7))

	// check metadata
	metadata, exists := nameToInteger1.LookupByName(baseValue1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(0))
	metadata, exists = nameToInteger2.LookupByName(baseValue2)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(0))
	metadata, exists = nameToInteger3.LookupByName(baseValue3)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(1))

	// check operations executed in SB
	opHistory := mockSB.PopHistoryOfOps()
	Expect(opHistory).To(HaveLen(13))
	operation := opHistory[0]
	Expect(operation.OpType).To(Equal(test.MockDump))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	checkValuesForCorrelation(operation.CorrelateDump, []KVWithMetadata{
		{
			Key:      prefixA + baseValue1,
			Value:    test.NewArrayValue("item2"),
			Metadata: nil,
			Origin:   FromNB,
		},
	})
	operation = opHistory[1]
	Expect(operation.OpType).To(Equal(test.MockDump))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor2Name))
	checkValuesForCorrelation(operation.CorrelateDump, []KVWithMetadata{
		{
			Key:      prefixB + baseValue2,
			Value:    test.NewArrayValue("item1", "item2"),
			Metadata: nil,
			Origin:   FromNB,
		},
	})
	operation = opHistory[2]
	Expect(operation.OpType).To(Equal(test.MockDump))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor3Name))
	checkValuesForCorrelation(operation.CorrelateDump, []KVWithMetadata{
		{
			Key:      prefixC + baseValue3,
			Value:    test.NewArrayValue("item1", "item2"),
			Metadata: nil,
			Origin:   FromNB,
		},
	})
	operation = opHistory[3]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor3Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixC + baseValue3 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[4]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor3Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixC + baseValue3))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[5]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor3Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixC + baseValue3))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[6]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor3Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixC + baseValue3 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[7]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor3Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixC + baseValue3 + "/item2"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[8]
	Expect(operation.OpType).To(Equal(test.MockDelete))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[9]
	Expect(operation.OpType).To(Equal(test.MockModify))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[10]
	Expect(operation.OpType).To(Equal(test.MockUpdate))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor2Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixB + baseValue2 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[11]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item2"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[12]
	Expect(operation.OpType).To(Equal(test.MockModify))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor2Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixB + baseValue2))
	Expect(operation.Err).To(BeNil())

	// check transaction operations
	txnHistory := scheduler.GetTransactionHistory(time.Time{}, time.Time{})
	Expect(txnHistory).To(HaveLen(1))
	txn := txnHistory[0]
	Expect(txn.PreRecord).To(BeFalse())
	Expect(txn.Start.After(startTime)).To(BeTrue())
	Expect(txn.Start.Before(txn.Stop)).To(BeTrue())
	Expect(txn.Stop.Before(stopTime)).To(BeTrue())
	Expect(txn.SeqNum).To(BeEquivalentTo(0))
	Expect(txn.TxnType).To(BeEquivalentTo(NBTransaction))
	Expect(txn.ResyncType).To(BeEquivalentTo(FullResync))
	Expect(txn.Description).To(BeEmpty())
	checkRecordedValues(txn.Values, []RecordedKVPair{
		{Key: prefixA + baseValue1, Value: utils.RecordProtoMessage(test.NewArrayValue("item2")), Origin: FromNB},
		{Key: prefixB + baseValue2, Value: utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")), Origin: FromNB},
		{Key: prefixC + baseValue3, Value: utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")), Origin: FromNB},
	})
	Expect(txn.PreErrors).To(BeEmpty())

	txnOps := RecordedTxnOps{
		{
			Operation:  Delete,
			Key:        prefixC + baseValue3 + "/item1",
			Derived:    true,
			PrevValue:  utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Delete,
			Key:        prefixC + baseValue3,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
			IsPending:  true,
		},
		{
			Operation:  Add,
			Key:        prefixC + baseValue3,
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
			WasPending: true,
		},
		{
			Operation:  Add,
			Key:        prefixC + baseValue3 + "/item1",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixC + baseValue3 + "/item2",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Delete,
			Key:        prefixA + baseValue1 + "/item1",
			Derived:    true,
			PrevValue:  utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Modify,
			Key:        prefixA + baseValue1,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue("item1")),
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Update,
			Key:        prefixB + baseValue2 + "/item1",
			Derived:    true,
			PrevValue:  utils.RecordProtoMessage(test.NewStringValue("item1")),
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue1 + "/item2",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Modify,
			Key:        prefixB + baseValue2,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue("item1")),
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixB + baseValue2 + "/item2",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
			IsPending:  true,
		},
	}
	checkTxnOperations(txn.Planned, txnOps)
	checkTxnOperations(txn.Executed, txnOps)

	// check flag stats
	graphR := scheduler.graph.Read()
	errorStats := graphR.GetFlagStats(ErrorFlagName, nil)
	Expect(errorStats.TotalCount).To(BeEquivalentTo(0))
	pendingStats := graphR.GetFlagStats(PendingFlagName, nil)
	Expect(pendingStats.TotalCount).To(BeEquivalentTo(1))
	derivedStats := graphR.GetFlagStats(DerivedFlagName, nil)
	Expect(derivedStats.TotalCount).To(BeEquivalentTo(5))
	lastUpdateStats := graphR.GetFlagStats(LastUpdateFlagName, nil)
	Expect(lastUpdateStats.TotalCount).To(BeEquivalentTo(8))
	lastChangeStats := graphR.GetFlagStats(LastChangeFlagName, nil)
	Expect(lastChangeStats.TotalCount).To(BeEquivalentTo(3))
	descriptorStats := graphR.GetFlagStats(DescriptorFlagName, nil)
	Expect(descriptorStats.TotalCount).To(BeEquivalentTo(8))
	Expect(descriptorStats.PerValueCount).To(HaveKey(descriptor1Name))
	Expect(descriptorStats.PerValueCount[descriptor1Name]).To(BeEquivalentTo(2))
	Expect(descriptorStats.PerValueCount).To(HaveKey(descriptor2Name))
	Expect(descriptorStats.PerValueCount[descriptor2Name]).To(BeEquivalentTo(3))
	Expect(descriptorStats.PerValueCount).To(HaveKey(descriptor3Name))
	Expect(descriptorStats.PerValueCount[descriptor3Name]).To(BeEquivalentTo(3))
	originStats := graphR.GetFlagStats(OriginFlagName, nil)
	Expect(originStats.TotalCount).To(BeEquivalentTo(8))
	Expect(originStats.PerValueCount).To(HaveKey(FromNB.String()))
	Expect(originStats.PerValueCount[FromNB.String()]).To(BeEquivalentTo(8))
	graphR.Release()

	// close scheduler
	err = scheduler.Close()
	Expect(err).To(BeNil())
}

func TestResyncWithRetry(t *testing.T) {
	RegisterTestingT(t)

	// prepare KV Scheduler
	scheduler := NewPlugin(UseDeps(func(deps *Deps) {
		deps.HTTPHandlers = nil
	}))
	err := scheduler.Init()
	Expect(err).To(BeNil())

	// prepare mocks
	mockSB := test.NewMockSouthbound()
	// -> initial content:
	mockSB.SetValue(prefixA+baseValue1, test.NewArrayValue(),
		&test.OnlyInteger{Integer: 0}, FromNB, false)
	// -> descriptor1:
	descriptor1 := test.NewMockDescriptor(&KVDescriptor{
		Name:          descriptor1Name,
		NBKeyPrefix:   prefixA,
		KeySelector:   prefixSelector(prefixA),
		ValueTypeName: proto.MessageName(test.NewArrayValue()),
		DerivedValues: test.ArrayValueDerBuilder,
		WithMetadata:  true,
	}, mockSB, 1)
	// -> planned error
	mockSB.PlanError(prefixA+baseValue1+"/item2", errors.New("failed to add value"),
		func() {
			mockSB.SetValue(prefixA+baseValue1, test.NewArrayValue("item1"),
				&test.OnlyInteger{Integer: 0}, FromNB, false)
		})

	// register descriptor with the scheduler
	scheduler.RegisterKVDescriptor(descriptor1)

	// subscribe to receive notifications about errors
	errorChan := make(chan KeyWithError, 5)
	scheduler.SubscribeForErrors(errorChan, prefixSelector(prefixA))

	// get metadata map created for the descriptor
	metadataMap := scheduler.GetMetadataMap(descriptor1.Name)
	nameToInteger, withMetadataMap := metadataMap.(test.NameToInteger)
	Expect(withMetadataMap).To(BeTrue())

	// run resync transaction that will fail for one value
	startTime := time.Now()
	resyncTxn := scheduler.StartNBTransaction()
	resyncTxn.SetValue(prefixA+baseValue1, test.NewLazyArrayValue("item1", "item2"))
	description := "testing resync with retry"
	ctx := context.Background()
	ctx = WithRetry(ctx, 3*time.Second, false)
	ctx = WithResync(ctx, FullResync, true)
	ctx = WithDescription(ctx, description)
	seqNum, err := resyncTxn.Commit(ctx)
	stopTime := time.Now()
	Expect(seqNum).To(BeEquivalentTo(0))
	Expect(err).ToNot(BeNil())
	txnErr := err.(*TransactionError)
	Expect(txnErr.GetTxnInitError()).ShouldNot(HaveOccurred())
	kvErrors := txnErr.GetKVErrors()
	Expect(kvErrors).To(HaveLen(1))
	Expect(kvErrors[0].TxnOperation).To(BeEquivalentTo(Add))
	Expect(kvErrors[0].Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item2"))
	Expect(kvErrors[0].Error.Error()).To(BeEquivalentTo("failed to add value"))

	// check the state of SB
	Expect(mockSB.GetKeysWithInvalidData()).To(BeEmpty())
	// -> base value 1
	value := mockSB.GetValue(prefixA + baseValue1)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item1"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(0))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 1
	value = mockSB.GetValue(prefixA + baseValue1 + "/item1")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item1"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item2 derived from base value 1 failed to get added
	value = mockSB.GetValue(prefixA + baseValue1 + "/item2")
	Expect(value).To(BeNil())
	Expect(mockSB.GetValues(nil)).To(HaveLen(2))

	// check metadata
	metadata, exists := nameToInteger.LookupByName(baseValue1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(0))

	// check operations executed in SB
	opHistory := mockSB.PopHistoryOfOps()
	Expect(opHistory).To(HaveLen(5))
	operation := opHistory[0]
	Expect(operation.OpType).To(Equal(test.MockDump))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	checkValuesForCorrelation(operation.CorrelateDump, []KVWithMetadata{
		{
			Key:      prefixA + baseValue1,
			Value:    test.NewArrayValue("item1", "item2"),
			Metadata: nil,
			Origin:   FromNB,
		},
	})
	operation = opHistory[1]
	Expect(operation.OpType).To(Equal(test.MockModify))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[2]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item1"))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[3]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item2"))
	Expect(operation.Err).ToNot(BeNil())
	Expect(operation.Err.Error()).To(BeEquivalentTo("failed to add value"))
	operation = opHistory[4] // refresh failed value
	Expect(operation.OpType).To(Equal(test.MockDump))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	checkValuesForCorrelation(operation.CorrelateDump, []KVWithMetadata{
		{
			Key:      prefixA + baseValue1,
			Value:    test.NewArrayValue("item1", "item2"),
			Metadata: &test.OnlyInteger{Integer: 0},
			Origin:   FromNB,
		},
	})

	// check transaction operations
	txnHistory := scheduler.GetTransactionHistory(time.Time{}, time.Time{})
	Expect(txnHistory).To(HaveLen(1))
	txn := txnHistory[0]
	Expect(txn.PreRecord).To(BeFalse())
	Expect(txn.Start.After(startTime)).To(BeTrue())
	Expect(txn.Start.Before(txn.Stop)).To(BeTrue())
	Expect(txn.Stop.Before(stopTime)).To(BeTrue())
	Expect(txn.SeqNum).To(BeEquivalentTo(0))
	Expect(txn.TxnType).To(BeEquivalentTo(NBTransaction))
	Expect(txn.ResyncType).To(BeEquivalentTo(FullResync))
	Expect(txn.Description).To(Equal(description))
	checkRecordedValues(txn.Values, []RecordedKVPair{
		{Key: prefixA + baseValue1, Value: utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")), Origin: FromNB},
	})
	Expect(txn.PreErrors).To(BeEmpty())

	txnOps := RecordedTxnOps{
		{
			Operation:  Modify,
			Key:        prefixA + baseValue1,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue()),
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue1 + "/item1",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item1")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue1 + "/item2",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
		},
	}
	checkTxnOperations(txn.Planned, txnOps)
	txnOps[2].IsPending = true
	txnOps[2].NewErr = errors.New("failed to add value")
	checkTxnOperations(txn.Executed, txnOps)

	// check flag stats
	graphR := scheduler.graph.Read()
	errorStats := graphR.GetFlagStats(ErrorFlagName, nil)
	Expect(errorStats.TotalCount).To(BeEquivalentTo(1))
	pendingStats := graphR.GetFlagStats(PendingFlagName, nil)
	Expect(pendingStats.TotalCount).To(BeEquivalentTo(1))
	derivedStats := graphR.GetFlagStats(DerivedFlagName, nil)
	Expect(derivedStats.TotalCount).To(BeEquivalentTo(2))
	lastUpdateStats := graphR.GetFlagStats(LastUpdateFlagName, nil)
	Expect(lastUpdateStats.TotalCount).To(BeEquivalentTo(3))
	lastChangeStats := graphR.GetFlagStats(LastChangeFlagName, nil)
	Expect(lastChangeStats.TotalCount).To(BeEquivalentTo(1))
	descriptorStats := graphR.GetFlagStats(DescriptorFlagName, nil)
	Expect(descriptorStats.TotalCount).To(BeEquivalentTo(3))
	Expect(descriptorStats.PerValueCount).To(HaveKey(descriptor1Name))
	Expect(descriptorStats.PerValueCount[descriptor1Name]).To(BeEquivalentTo(3))
	originStats := graphR.GetFlagStats(OriginFlagName, nil)
	Expect(originStats.TotalCount).To(BeEquivalentTo(3))
	Expect(originStats.PerValueCount).To(HaveKey(FromNB.String()))
	Expect(originStats.PerValueCount[FromNB.String()]).To(BeEquivalentTo(3))
	graphR.Release()

	// check error updates received through the channel
	var errorNotif KeyWithError
	Eventually(errorChan, time.Second).Should(Receive(&errorNotif))
	Expect(errorNotif.Key).To(Equal(prefixA + baseValue1 + "/item2"))
	Expect(errorNotif.TxnOperation).To(BeEquivalentTo(Add))
	Expect(errorNotif.Error).ToNot(BeNil())
	Expect(errorNotif.Error.Error()).To(BeEquivalentTo("failed to add value"))

	// eventually the value should get "fixed"
	Eventually(errorChan, 5*time.Second).Should(Receive(&errorNotif))
	Expect(errorNotif.Key).To(Equal(prefixA + baseValue1 + "/item2"))
	Expect(errorNotif.Error).To(BeNil())

	// check the state of SB after retry
	Expect(mockSB.GetKeysWithInvalidData()).To(BeEmpty())
	// -> base value 1
	value = mockSB.GetValue(prefixA + baseValue1)
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewArrayValue("item1", "item2"))).To(BeTrue())
	Expect(value.Metadata).ToNot(BeNil())
	Expect(value.Metadata.(test.MetaWithInteger).GetInteger()).To(BeEquivalentTo(0))
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	// -> item1 derived from base value 1
	value = mockSB.GetValue(prefixA + baseValue1 + "/item1")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item1"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	Expect(mockSB.GetValues(nil)).To(HaveLen(3))
	// -> item2 derived from base value 1 was re-added
	value = mockSB.GetValue(prefixA + baseValue1 + "/item2")
	Expect(value).ToNot(BeNil())
	Expect(proto.Equal(value.Value, test.NewStringValue("item2"))).To(BeTrue())
	Expect(value.Metadata).To(BeNil())
	Expect(value.Origin).To(BeEquivalentTo(FromNB))
	Expect(mockSB.GetValues(nil)).To(HaveLen(3))

	// check metadata
	metadata, exists = nameToInteger.LookupByName(baseValue1)
	Expect(exists).To(BeTrue())
	Expect(metadata.GetInteger()).To(BeEquivalentTo(0))

	// check operations executed in SB during retry
	opHistory = mockSB.PopHistoryOfOps()
	Expect(opHistory).To(HaveLen(2))
	operation = opHistory[0]
	Expect(operation.OpType).To(Equal(test.MockModify))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1))
	Expect(operation.Err).To(BeNil())
	operation = opHistory[1]
	Expect(operation.OpType).To(Equal(test.MockAdd))
	Expect(operation.Descriptor).To(BeEquivalentTo(descriptor1Name))
	Expect(operation.Key).To(BeEquivalentTo(prefixA + baseValue1 + "/item2"))
	Expect(operation.Err).To(BeNil())

	// check retry transaction operations
	txnHistory = scheduler.GetTransactionHistory(time.Time{}, time.Now())
	Expect(txnHistory).To(HaveLen(2))
	txn = txnHistory[1]
	Expect(txn.PreRecord).To(BeFalse())
	Expect(txn.Start.After(stopTime)).To(BeTrue())
	Expect(txn.Start.Before(txn.Stop)).To(BeTrue())
	Expect(txn.Stop.Before(time.Now())).To(BeTrue())
	Expect(txn.SeqNum).To(BeEquivalentTo(1))
	Expect(txn.TxnType).To(BeEquivalentTo(RetryFailedOps))
	Expect(txn.ResyncType).To(BeEquivalentTo(NotResync))
	Expect(txn.Description).To(BeEmpty())
	checkRecordedValues(txn.Values, []RecordedKVPair{
		{Key: prefixA + baseValue1, Value: utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")), Origin: FromNB},
	})
	Expect(txn.PreErrors).To(BeEmpty())

	txnOps = RecordedTxnOps{
		{
			Operation:  Modify,
			Key:        prefixA + baseValue1,
			PrevValue:  utils.RecordProtoMessage(test.NewArrayValue("item1")),
			NewValue:   utils.RecordProtoMessage(test.NewArrayValue("item1", "item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
			IsRetry:    true,
		},
		{
			Operation:  Add,
			Key:        prefixA + baseValue1 + "/item2",
			Derived:    true,
			NewValue:   utils.RecordProtoMessage(test.NewStringValue("item2")),
			PrevOrigin: FromNB,
			NewOrigin:  FromNB,
			PrevErr:    errors.New("failed to add value"),
			IsRetry:    true,
		},
	}
	checkTxnOperations(txn.Planned, txnOps)
	checkTxnOperations(txn.Executed, txnOps)

	// check flag stats
	graphR = scheduler.graph.Read()
	errorStats = graphR.GetFlagStats(ErrorFlagName, nil)
	Expect(errorStats.TotalCount).To(BeEquivalentTo(1))
	pendingStats = graphR.GetFlagStats(PendingFlagName, nil)
	Expect(pendingStats.TotalCount).To(BeEquivalentTo(1))
	derivedStats = graphR.GetFlagStats(DerivedFlagName, nil)
	Expect(derivedStats.TotalCount).To(BeEquivalentTo(4))
	lastUpdateStats = graphR.GetFlagStats(LastUpdateFlagName, nil)
	Expect(lastUpdateStats.TotalCount).To(BeEquivalentTo(6))
	lastChangeStats = graphR.GetFlagStats(LastChangeFlagName, nil)
	Expect(lastChangeStats.TotalCount).To(BeEquivalentTo(2))
	descriptorStats = graphR.GetFlagStats(DescriptorFlagName, nil)
	Expect(descriptorStats.TotalCount).To(BeEquivalentTo(6))
	Expect(descriptorStats.PerValueCount).To(HaveKey(descriptor1Name))
	Expect(descriptorStats.PerValueCount[descriptor1Name]).To(BeEquivalentTo(6))
	originStats = graphR.GetFlagStats(OriginFlagName, nil)
	Expect(originStats.TotalCount).To(BeEquivalentTo(6))
	Expect(originStats.PerValueCount).To(HaveKey(FromNB.String()))
	Expect(originStats.PerValueCount[FromNB.String()]).To(BeEquivalentTo(6))
	graphR.Release()

	// close scheduler
	err = scheduler.Close()
	Expect(err).To(BeNil())
}
*/

/* when graph dump is needed:
graphR := scheduler.graph.Read()
graphDump := graphR.Dump()
fmt.Print(graphDump)
graphR.Release()
*/
