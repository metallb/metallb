//  Copyright (c) 2019 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package util

import (
	"reflect"

	"github.com/gogo/protobuf/proto"
)

func ExtractProtos(from ...interface{}) (protos []proto.Message) {
	for _, v := range from {
		if reflect.ValueOf(v).IsNil() {
			continue
		}
		val := reflect.ValueOf(v).Elem()
		typ := val.Type()
		if typ.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < typ.NumField(); i++ {
			field := val.Field(i)
			if field.Kind() == reflect.Slice {
				for idx := 0; idx < field.Len(); idx++ {
					elem := field.Index(idx)
					if msg, ok := elem.Interface().(proto.Message); ok {
						protos = append(protos, msg)
					}
				}
			} else {
				if msg, ok := field.Interface().(proto.Message); ok {
					protos = append(protos, msg)
				}
			}
		}
	}
	return
}

func PlaceProtos(protos map[string]proto.Message, dsts ...interface{}) {
	for _, prot := range protos {
		protTyp := reflect.TypeOf(prot)
		for _, dst := range dsts {
			dstVal := reflect.ValueOf(dst).Elem()
			dstTyp := dstVal.Type()
			if dstTyp.Kind() != reflect.Struct {
				return
			}
			for i := 0; i < dstTyp.NumField(); i++ {
				field := dstVal.Field(i)
				if field.Kind() == reflect.Slice {
					if protTyp.AssignableTo(field.Type().Elem()) {
						field.Set(reflect.Append(field, reflect.ValueOf(prot)))
					}
				} else {
					if field.Type() == protTyp {
						field.Set(reflect.ValueOf(prot))
					}
				}
			}
		}
	}
	return
}
