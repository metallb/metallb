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

//go:generate protoc --proto_path=./model --gogo_out=./model values.proto

package test

import (
	"github.com/gogo/protobuf/proto"

	"github.com/ligato/cn-infra/datasync"
	. "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	. "github.com/ligato/vpp-agent/plugins/kvscheduler/internal/test/model"
)

// LazyArrayValue implements datasync.LazyValue for ArrayValue.
type LazyArrayValue struct {
	items []string
}

// LazyStringValue implements datasync.LazyValue for StringValue.
type LazyStringValue struct {
	value string
}

// GetValue unmarshalls ArrayValue into the given proto.Message.
func (lav *LazyArrayValue) GetValue(value proto.Message) error {
	av := NewArrayValue(lav.items...)
	tmp, err := proto.Marshal(av)
	if err != nil {
		return err
	}
	return proto.Unmarshal(tmp, value)
}

// GetValue unmarshalls StringValue into the given proto.Message.
func (lsv *LazyStringValue) GetValue(value proto.Message) error {
	sv := NewStringValue(lsv.value)
	tmp, err := proto.Marshal(sv)
	if err != nil {
		return err
	}
	return proto.Unmarshal(tmp, value)
}

// NewStringValue creates a new instance of StringValue.
func NewStringValue(str string) proto.Message {
	return &StringValue{Value: str}
}

// NewArrayValue creates a new instance of ArrayValue.
func NewArrayValue(items ...string) proto.Message {
	return &ArrayValue{Items: items}
}

// NewLazyStringValue creates a new instance of lazy (marshalled) StringValue.
func NewLazyStringValue(str string) datasync.LazyValue {
	return &LazyStringValue{value: str}
}

// NewLazyArrayValue creates a new instance of lazy (marshalled) ArrayValue.
func NewLazyArrayValue(items ...string) datasync.LazyValue {
	return &LazyArrayValue{
		items: items,
	}
}

// StringValueComparator is (a custom) KVDescriptor.ValueComparator for string values.
func StringValueComparator(key string, v1, v2 proto.Message) bool {
	sv1, isStringVal1 := v1.(*StringValue)
	sv2, isStringVal2 := v2.(*StringValue)
	if !isStringVal1 || !isStringVal2 {
		return false
	}
	return sv1.Value == sv2.Value
}

// ArrayValueDerBuilder can be used to derive one StringValue for every item
// in the array.
func ArrayValueDerBuilder(key string, value proto.Message) []KeyValuePair {
	var derivedVals []KeyValuePair
	arrayVal, isArrayVal := value.(*ArrayValue)
	if isArrayVal {
		for _, item := range arrayVal.Items {
			derivedVals = append(derivedVals, KeyValuePair{
				Key:   key + "/" + item,
				Value: NewStringValue(item),
			})
		}
	}
	return derivedVals
}
