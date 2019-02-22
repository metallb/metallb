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

package utils

import (
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestSingleton(t *testing.T) {
	RegisterTestingT(t)

	// constructor
	s := NewSingletonKeySet("key1")
	Expect(s.String()).To(BeEquivalentTo("{key1}"))
	Expect(s.Has("key1")).To(BeTrue())
	Expect(s.Has("key2")).To(BeFalse())
	Expect(s.String()).To(BeEquivalentTo("{key1}"))
	Expect(s.Length()).To(BeEquivalentTo(1))
	Expect(s.Iterate()).To(BeEquivalentTo([]string{"key1"}))

	// delete
	Expect(s.Del("key2")).To(BeFalse())
	Expect(s.Del("key1")).To(BeTrue())
	Expect(s.Has("key1")).To(BeFalse())
	Expect(s.String()).To(BeEquivalentTo("{}"))
	Expect(s.Length()).To(BeEquivalentTo(0))
	Expect(s.Iterate()).To(BeEquivalentTo([]string{}))

	// add
	Expect(s.Add("key1")).To(BeTrue())
	Expect(s.Add("key1")).To(BeFalse())
	Expect(s.Add("key2")).To(BeTrue())
	Expect(s.Has("key1")).To(BeFalse())
	Expect(s.Has("key2")).To(BeTrue())
	Expect(s.String()).To(BeEquivalentTo("{key2}"))
	Expect(s.Length()).To(BeEquivalentTo(1))
	Expect(s.Iterate()).To(BeEquivalentTo([]string{"key2"}))

	// copy-on-write
	s2 := s.CopyOnWrite()
	Expect(s2.Has("key1")).To(BeFalse())
	Expect(s2.Has("key2")).To(BeTrue())
	Expect(s2.String()).To(BeEquivalentTo("{key2}"))
	Expect(s2.Length()).To(BeEquivalentTo(1))
	Expect(s2.Iterate()).To(BeEquivalentTo([]string{"key2"}))
	Expect(s2.Del("key2")).To(BeTrue())
	Expect(s2.String()).To(BeEquivalentTo("{}"))
	Expect(s.String()).To(BeEquivalentTo("{key2}"))
	Expect(s.Length()).To(BeEquivalentTo(1))
	Expect(s2.Length()).To(BeEquivalentTo(0))

	// subtract
	Expect(s.Subtract(s2)).To(BeFalse())
	s2.Add("key2")
	Expect(s.Subtract(s2)).To(BeTrue())
	Expect(s.String()).To(BeEquivalentTo("{}"))
	s.Add("key1")
	Expect(s.Subtract(s2)).To(BeFalse())

	// intersect
	Expect(s.Intersect(s2)).To(BeTrue())
	Expect(s.String()).To(BeEquivalentTo("{}"))
	s.Add("key2")
	Expect(s.Intersect(s2)).To(BeFalse())
	Expect(s.String()).To(BeEquivalentTo("{key2}"))
}

func permutations(arr []string) [][]string {
	var helper func([]string, int)
	res := [][]string{}

	helper = func(arr []string, n int) {
		if n == 1 {
			tmp := make([]string, len(arr))
			copy(tmp, arr)
			res = append(res, tmp)
		} else {
			for i := 0; i < n; i++ {
				helper(arr, n-1)
				if n%2 == 1 {
					tmp := arr[i]
					arr[i] = arr[n-1]
					arr[n-1] = tmp
				} else {
					tmp := arr[0]
					arr[0] = arr[n-1]
					arr[n-1] = tmp
				}
			}
		}
	}
	helper(arr, len(arr))
	return res
}

func testKeySetToString(s KeySet, keys ...string) {
	str := s.String()
	validStr := false
	for _, permutation := range permutations(keys) {
		permStr := "{" + strings.Join(permutation, ", ") + "}"
		if permStr == str {
			validStr = true
			break
		}
	}
	Expect(validStr).To(BeTrue())
}

func testKeySetIterator(s KeySet, keys ...string) {
	iter := s.Iterate()
	validIter := false
	for _, permutation := range permutations(keys) {
		if reflect.DeepEqual(permutation, iter) {
			validIter = true
			break
		}
	}
	Expect(validIter).To(BeTrue())
}

func testKeySet(factory1, factory2 func(keys ...string) KeySet) {
	// constructor
	s1 := factory1()
	Expect(s1.Has("key1")).To(BeFalse())
	Expect(s1.Has("key2")).To(BeFalse())
	Expect(s1.String()).To(BeEquivalentTo("{}"))
	Expect(s1.Length()).To(BeEquivalentTo(0))
	Expect(s1.Iterate()).To(BeEquivalentTo([]string{}))
	s1 = factory1("key1", "key2", "key3")
	Expect(s1.Has("key1")).To(BeTrue())
	Expect(s1.Has("key2")).To(BeTrue())
	testKeySetToString(s1, "key1", "key2", "key3")
	Expect(s1.Length()).To(BeEquivalentTo(3))
	testKeySetIterator(s1, "key1", "key2", "key3")

	// delete
	Expect(s1.Del("key4")).To(BeFalse())
	Expect(s1.Del("key2")).To(BeTrue())
	Expect(s1.Has("key2")).To(BeFalse())
	Expect(s1.Has("key1")).To(BeTrue())
	Expect(s1.Has("key3")).To(BeTrue())
	Expect(s1.Length()).To(BeEquivalentTo(2))
	testKeySetToString(s1, "key1", "key3")
	testKeySetIterator(s1, "key1", "key3")
	Expect(s1.Del("key1")).To(BeTrue())
	Expect(s1.Del("key3")).To(BeTrue())
	Expect(s1.Del("key3")).To(BeFalse())
	Expect(s1.String()).To(BeEquivalentTo("{}"))
	Expect(s1.Length()).To(BeEquivalentTo(0))
	Expect(s1.Iterate()).To(BeEquivalentTo([]string{}))

	// add
	Expect(s1.Add("key2")).To(BeTrue())
	Expect(s1.Add("key2")).To(BeFalse())
	Expect(s1.Add("key1")).To(BeTrue())
	Expect(s1.Add("key3")).To(BeTrue())
	Expect(s1.Has("key1")).To(BeTrue())
	Expect(s1.Has("key2")).To(BeTrue())
	Expect(s1.Has("key3")).To(BeTrue())
	Expect(s1.Has("key4")).To(BeFalse())
	Expect(s1.Length()).To(BeEquivalentTo(3))
	testKeySetToString(s1, "key1", "key2", "key3")
	testKeySetIterator(s1, "key1", "key2", "key3")

	// copy-on-write
	s2 := s1.CopyOnWrite()
	Expect(s2.Has("key1")).To(BeTrue())
	Expect(s2.Has("key2")).To(BeTrue())
	Expect(s2.Has("key3")).To(BeTrue())
	Expect(s2.Has("key4")).To(BeFalse())
	Expect(s2.Length()).To(BeEquivalentTo(3))
	testKeySetToString(s2, "key1", "key2", "key3")
	testKeySetIterator(s2, "key1", "key2", "key3")
	Expect(s2.Add("key4")).To(BeTrue())
	Expect(s2.Has("key4")).To(BeTrue())
	Expect(s1.Has("key4")).To(BeFalse())
	Expect(s2.Del("key1")).To(BeTrue())
	Expect(s2.Has("key1")).To(BeFalse())
	Expect(s1.Has("key1")).To(BeTrue())
	Expect(s2.Length()).To(BeEquivalentTo(3))
	testKeySetToString(s2, "key2", "key3", "key4")
	testKeySetIterator(s2, "key2", "key3", "key4")
	Expect(s1.Length()).To(BeEquivalentTo(3))
	testKeySetToString(s1, "key1", "key2", "key3")
	testKeySetIterator(s1, "key1", "key2", "key3")

	// subtract
	s3 := factory2("key1", "key3")
	Expect(s1.Subtract(s3)).To(BeTrue())
	Expect(s1.Length()).To(BeEquivalentTo(1))
	testKeySetToString(s1, "key2")
	testKeySetIterator(s1, "key2")
	Expect(s1.Subtract(s3)).To(BeFalse())
	Expect(s1.Length()).To(BeEquivalentTo(1))
	testKeySetToString(s1, "key2")
	testKeySetIterator(s1, "key2")
	Expect(s2.Subtract(s3)).To(BeTrue())
	Expect(s2.Length()).To(BeEquivalentTo(2))
	testKeySetToString(s2, "key2", "key4")
	testKeySetIterator(s2, "key2", "key4")
	Expect(s2.Subtract(s3)).To(BeFalse())
	Expect(s2.Length()).To(BeEquivalentTo(2))
	testKeySetToString(s2, "key2", "key4")
	testKeySetIterator(s2, "key2", "key4")

	// intersect
	Expect(s1.Intersect(s2)).To(BeFalse())
	Expect(s1.Length()).To(BeEquivalentTo(1))
	testKeySetToString(s1, "key2")
	testKeySetIterator(s1, "key2")
	Expect(s2.Intersect(s1)).To(BeTrue())
	Expect(s2.Length()).To(BeEquivalentTo(1))
	testKeySetToString(s2, "key2")
	testKeySetIterator(s2, "key2")
}

func TestMapBasedKeySet(t *testing.T) {
	RegisterTestingT(t)

	testKeySet(NewMapBasedKeySet, NewMapBasedKeySet)
	testKeySet(NewMapBasedKeySet, NewSliceBasedKeySet)
}

func TestSliceBasedKeySet(t *testing.T) {
	RegisterTestingT(t)

	testKeySet(NewSliceBasedKeySet, NewSliceBasedKeySet)
	testKeySet(NewSliceBasedKeySet, NewMapBasedKeySet)
}
