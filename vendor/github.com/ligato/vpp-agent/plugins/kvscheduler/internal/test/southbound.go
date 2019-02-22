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

package test

import (
	"sync"

	"github.com/gogo/protobuf/proto"

	. "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
)

// MockSouthbound is used in UTs to simulate the state of the southbound for the scheduler.
type MockSouthbound struct {
	sync.Mutex

	values         map[string]*KVWithMetadata // key -> value
	plannedErrors  map[string][]plannedError  // key -> planned error
	derivedKeys    map[string]struct{}
	opHistory      []MockOperation // from the oldest to the latest
	invalidKeyData map[string]struct{}
}

// MockOpType is used to remember the type of a simulated operation.
type MockOpType int

const (
	// MockAdd is a mock Add operation.
	MockAdd MockOpType = iota
	// MockModify is a mock Modify operation.
	MockModify
	// MockDelete is a mock Delete operation.
	MockDelete
	// MockDump is a mock Dump operation.
	MockDump
)

// MockOperation is used in UTs to remember executed descriptor operations.
type MockOperation struct {
	OpType        MockOpType
	Descriptor    string
	Key           string
	Value         proto.Message
	Err           error
	CorrelateDump []KVWithMetadata
}

// plannedError is used to simulate error situation.
type plannedError struct {
	err         error
	afterErrClb func() // update values after error via SetValue()
}

// NewMockSouthbound creates a new instance of SB mock.
func NewMockSouthbound() *MockSouthbound {
	return &MockSouthbound{
		values:         make(map[string]*KVWithMetadata),
		plannedErrors:  make(map[string][]plannedError),
		derivedKeys:    make(map[string]struct{}),
		invalidKeyData: make(map[string]struct{}),
	}
}

// GetKeysWithInvalidData returns a set of keys for which invalid data were provided
// in one of the descriptor's operations.
func (ms *MockSouthbound) GetKeysWithInvalidData() map[string]struct{} {
	return ms.invalidKeyData
}

// PlanError is used to simulate error situation for the next operation over the given key.
func (ms *MockSouthbound) PlanError(key string, err error, afterErrClb func()) {
	ms.Lock()
	defer ms.Unlock()

	if _, has := ms.plannedErrors[key]; !has {
		ms.plannedErrors[key] = []plannedError{}
	}
	ms.plannedErrors[key] = append(ms.plannedErrors[key], plannedError{err: err, afterErrClb: afterErrClb})
}

// SetValue is used in UTs to prepare the state of SB for the next Dump.
func (ms *MockSouthbound) SetValue(key string, value proto.Message, metadata Metadata, origin ValueOrigin, isDerived bool) {
	ms.Lock()
	defer ms.Unlock()

	ms.setValueUnsafe(key, value, metadata, origin, isDerived)
}

// GetValue can be used in UTs to query the state of simulated SB.
func (ms *MockSouthbound) GetValue(key string) *KVWithMetadata {
	ms.Lock()
	defer ms.Unlock()

	if _, hasValue := ms.values[key]; !hasValue {
		return nil
	}
	return ms.values[key]
}

// GetValues can be used in UTs to query the state of simulated SB.
func (ms *MockSouthbound) GetValues(selector KeySelector) []*KVWithMetadata {
	ms.Lock()
	defer ms.Unlock()

	var values []*KVWithMetadata
	for _, kv := range ms.values {
		if selector != nil && !selector(kv.Key) {
			continue
		}
		values = append(values, kv)
	}

	return values
}

// PopHistoryOfOps returns and simultaneously clears the history of executed descriptor operations.
func (ms *MockSouthbound) PopHistoryOfOps() []MockOperation {
	ms.Lock()
	defer ms.Unlock()

	history := ms.opHistory
	ms.opHistory = []MockOperation{}
	return history
}

// setValueUnsafe changes the value under given key without acquiring the lock.
func (ms *MockSouthbound) setValueUnsafe(key string, value proto.Message, metadata Metadata, origin ValueOrigin, isDerived bool) {
	if value == nil {
		delete(ms.values, key)
	} else {
		ms.values[key] = &KVWithMetadata{Key: key, Value: value, Metadata: metadata, Origin: origin}
	}
	if isDerived {
		ms.derivedKeys[key] = struct{}{}
	}
}

// registerDerivedKey is used to remember that the given key points to a derived value.
// Used by MockDescriptor.
func (ms *MockSouthbound) registerDerivedKey(key string) {
	ms.Lock()
	defer ms.Unlock()
	ms.derivedKeys[key] = struct{}{}
}

// isKeyDerived returns true if the given key belongs to a derived value.
func (ms *MockSouthbound) isKeyDerived(key string) bool {
	_, isDerived := ms.derivedKeys[key]
	return isDerived
}

// registerKeyWithInvalidData is used to remember that for the given key invalid input
// data were provided.
func (ms *MockSouthbound) registerKeyWithInvalidData(key string) {
	//panic(key)
	ms.invalidKeyData[key] = struct{}{}
}

// dump returns non-derived values under the given selector.
// Used by MockDescriptor.
func (ms *MockSouthbound) dump(descriptor string, correlate []KVWithMetadata, selector KeySelector) ([]KVWithMetadata, error) {
	ms.Lock()
	defer ms.Unlock()

	var dump []KVWithMetadata
	for _, kv := range ms.values {
		if ms.isKeyDerived(kv.Key) || !selector(kv.Key) {
			continue
		}
		dump = append(dump, KVWithMetadata{
			Key:      kv.Key,
			Value:    kv.Value,
			Metadata: kv.Metadata,
			Origin:   kv.Origin,
		})
	}

	ms.opHistory = append(ms.opHistory, MockOperation{
		OpType:        MockDump,
		Descriptor:    descriptor,
		CorrelateDump: correlate,
	})
	return dump, nil
}

// executeChange is used by MockDescriptor to simulate execution of a operation in SB.
func (ms *MockSouthbound) executeChange(descriptor string, opType MockOpType, key string, value proto.Message, metadata Metadata) error {
	ms.Lock()

	operation := MockOperation{OpType: opType, Descriptor: descriptor, Key: key, Value: value}

	plannedErrors, hasErrors := ms.plannedErrors[key]
	if hasErrors {
		// simulate error situation
		ms.plannedErrors[key] = plannedErrors[1:]
		if len(ms.plannedErrors[key]) == 0 {
			delete(ms.plannedErrors, key)
		}
		err := plannedErrors[0].err
		clb := plannedErrors[0].afterErrClb
		operation.Err = err
		ms.opHistory = append(ms.opHistory, operation)
		ms.Unlock()

		if clb != nil {
			clb()
		}

		return err
	}

	// the simulated operation has succeeded
	ms.setValueUnsafe(key, value, metadata, FromNB, ms.isKeyDerived(key))
	ms.opHistory = append(ms.opHistory, operation)
	ms.Unlock()
	return nil
}
