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

package sql

import (
	"reflect"

	"github.com/ligato/cn-infra/utils/safeclose"
)

// SliceIt reads everything from the ValIterator and stores it to pointerToASlice.
// It closes the iterator (since nothing left in the iterator).
func SliceIt(pointerToASlice interface{}, it ValIterator) error {
	/* TODO defer func() {
		if exp := recover(); exp != nil && it != nil {
			logger.Error(exp)
			exp = safeclose.Close(it)
			if exp != nil {
				logger.Error(exp)
			}
		}
	}()*/

	sl := reflect.ValueOf(pointerToASlice)
	if sl.Kind() == reflect.Ptr {
		sl = sl.Elem()
	} else {
		panic("must be pointer")
	}

	if sl.Kind() != reflect.Slice {
		panic("must be slice")
	}

	sliceType := sl.Type()

	sliceElemType := sliceType.Elem()
	sliceElemPtr := sliceElemType.Kind() == reflect.Ptr
	if sliceElemPtr {
		sliceElemType = sliceElemType.Elem()
	}
	for {
		row := reflect.New(sliceElemType)
		if stop := it.GetNext(row.Interface()); stop {
			break
		}

		if sliceElemPtr {
			sl.Set(reflect.Append(sl, row))
		} else {
			sl.Set(reflect.Append(sl, row.Elem()))
		}
	}

	return safeclose.Close(it)
}
