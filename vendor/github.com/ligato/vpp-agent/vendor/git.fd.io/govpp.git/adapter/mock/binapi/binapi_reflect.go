// Copyright (c) 2017 Cisco and/or its affiliates.
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

// Package binapi is a helper package for generic handling of VPP binary
// API messages in the mock adapter and integration tests.
package binapi

import (
	"reflect"
)

const swIfIndexName = "SwIfIndex"
const retvalName = "Retval"
const replySuffix = "_reply"

// findFieldOfType finds the field specified by its name in provided message defined as reflect.Type data type.
func findFieldOfType(reply reflect.Type, fieldName string) (reflect.StructField, bool) {
	for reply.Kind() == reflect.Ptr {
		reply = reply.Elem()
	}
	if reply.Kind() == reflect.Struct {
		field, found := reply.FieldByName(fieldName)
		return field, found
	}
	return reflect.StructField{}, false
}

// findFieldOfValue finds the field specified by its name in provided message defined as reflect.Value data type.
func findFieldOfValue(reply reflect.Value, fieldName string) (reflect.Value, bool) {
	if reply.Kind() == reflect.Struct {
		field := reply.FieldByName(fieldName)
		return field, field.IsValid()
	} else if reply.Kind() == reflect.Ptr && reply.Elem().Kind() == reflect.Struct {
		field := reply.Elem().FieldByName(fieldName)
		return field, field.IsValid()
	}
	return reflect.Value{}, false
}

// HasSwIfIdx checks whether provided message has the swIfIndex field.
func HasSwIfIdx(msg reflect.Type) bool {
	_, found := findFieldOfType(msg, swIfIndexName)
	return found
}

// SetSwIfIdx sets the swIfIndex field of provided message to provided value.
func SetSwIfIdx(msg reflect.Value, swIfIndex uint32) {
	if field, found := findFieldOfValue(msg, swIfIndexName); found {
		field.Set(reflect.ValueOf(swIfIndex))
	}
}

// SetRetval sets the retval field of provided message to provided value.
func SetRetval(msg reflect.Value, retVal int32) {
	if field, found := findFieldOfValue(msg, retvalName); found {
		field.Set(reflect.ValueOf(retVal))
	}
}

// ReplyNameFor returns reply message name to the given request message name.
func ReplyNameFor(requestName string) (string, bool) {
	return requestName + replySuffix, true
}
