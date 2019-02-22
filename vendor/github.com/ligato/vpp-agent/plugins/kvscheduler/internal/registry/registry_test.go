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

package registry

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	. "github.com/ligato/vpp-agent/plugins/kvscheduler/internal/test"
)

const (
	descriptor1Name = "descriptor1"
	descriptor2Name = "descriptor2"
	descriptor3Name = "descriptor3"
	descriptor4Name = "descriptor4"

	prefixA = "/prefixA/"
	prefixB = "/prefixB/"
	prefixC = "/prefixC/"

	randomKey    = "randomKey"
	randomSuffix = "randomSuffix"
)

func keySelector(keys ...string) func(key string) bool {
	return func(key string) bool {
		for _, k := range keys {
			if key == k {
				return true
			}
		}
		return false
	}
}

func prefixSelector(prefix string) func(key string) bool {
	return func(key string) bool {
		return strings.HasPrefix(key, prefix)
	}
}

func TestRegistry(t *testing.T) {
	RegisterTestingT(t)

	descriptor1 := NewMockDescriptor(
		&KVDescriptor{
			Name:             descriptor1Name,
			KeySelector:      prefixSelector(prefixA),
			DumpDependencies: []string{descriptor2Name},
		}, nil, 0)

	descriptor2 := NewMockDescriptor(
		&KVDescriptor{
			Name:             descriptor2Name,
			KeySelector:      prefixSelector(prefixB),
			DumpDependencies: []string{descriptor3Name},
		}, nil, 0)

	descriptor3 := NewMockDescriptor(
		&KVDescriptor{
			Name:             descriptor3Name,
			KeySelector:      prefixSelector(prefixC),
			DumpDependencies: []string{descriptor4Name},
		}, nil, 0)

	descriptor4 := NewMockDescriptor(
		&KVDescriptor{
			Name:        descriptor4Name,
			KeySelector: keySelector(randomKey),
		}, nil, 0)

	registry := NewRegistry()

	registry.RegisterDescriptor(descriptor3)
	registry.RegisterDescriptor(descriptor2)
	registry.RegisterDescriptor(descriptor1)
	registry.RegisterDescriptor(descriptor4)

	// test that descriptors are ordered by dependencies
	allDescriptors := registry.GetAllDescriptors()
	Expect(allDescriptors).To(HaveLen(4))
	Expect(allDescriptors[0].Name).To(BeEquivalentTo(descriptor4Name))
	Expect(allDescriptors[1].Name).To(BeEquivalentTo(descriptor3Name))
	Expect(allDescriptors[2].Name).To(BeEquivalentTo(descriptor2Name))
	Expect(allDescriptors[3].Name).To(BeEquivalentTo(descriptor1Name))

	// test GetDescriptor() method
	descriptor := registry.GetDescriptor(descriptor1Name)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor1Name))
	Expect(descriptor.KeySelector(prefixA + randomSuffix)).To(BeTrue())
	Expect(descriptor.KeySelector(prefixB + randomSuffix)).To(BeFalse())
	descriptor = registry.GetDescriptor(descriptor2Name)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor2Name))
	Expect(descriptor.KeySelector(prefixA + randomSuffix)).To(BeFalse())
	Expect(descriptor.KeySelector(prefixB + randomSuffix)).To(BeTrue())
	descriptor = registry.GetDescriptor(descriptor3Name)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor3Name))
	Expect(descriptor.KeySelector(prefixA + randomSuffix)).To(BeFalse())
	Expect(descriptor.KeySelector(prefixC + randomSuffix)).To(BeTrue())
	descriptor = registry.GetDescriptor(descriptor4Name)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor4Name))
	Expect(descriptor.KeySelector(prefixA + randomSuffix)).To(BeFalse())
	Expect(descriptor.KeySelector(randomKey)).To(BeTrue())

	// basic GetDescriptorForKey tests
	descriptor = registry.GetDescriptorForKey(prefixA + randomSuffix)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor1Name))
	descriptor = registry.GetDescriptorForKey(prefixB + randomSuffix)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor2Name))
	descriptor = registry.GetDescriptorForKey(prefixC + randomSuffix)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor3Name))
	descriptor = registry.GetDescriptorForKey(randomKey)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor4Name))

	// repeated lookups will take result from the cache
	descriptor = registry.GetDescriptorForKey(prefixA + randomSuffix)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor1Name))
	descriptor = registry.GetDescriptorForKey(prefixB + randomSuffix)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor2Name))
	descriptor = registry.GetDescriptorForKey(prefixC + randomSuffix)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor3Name))
	descriptor = registry.GetDescriptorForKey(randomKey)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor4Name))

	// fill up the cache
	for i := 0; i < maxKeyCacheSize; i++ {
		if i%2 == 0 {
			descriptor = registry.GetDescriptorForKey(fmt.Sprintf("%s%d", prefixA, i))
			Expect(descriptor).ToNot(BeNil())
			Expect(descriptor.Name).To(BeEquivalentTo(descriptor1Name))
		} else {
			descriptor = registry.GetDescriptorForKey(fmt.Sprintf("%s%d", prefixB, i))
			Expect(descriptor).ToNot(BeNil())
			Expect(descriptor.Name).To(BeEquivalentTo(descriptor2Name))
		}
	}

	// results for these lookups were already removed from the cache and thus will have to be repeated
	descriptor = registry.GetDescriptorForKey(prefixA + randomSuffix)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor1Name))
	descriptor = registry.GetDescriptorForKey(prefixB + randomSuffix)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor2Name))
	descriptor = registry.GetDescriptorForKey(prefixC + randomSuffix)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor3Name))
	descriptor = registry.GetDescriptorForKey(randomKey)
	Expect(descriptor).ToNot(BeNil())
	Expect(descriptor.Name).To(BeEquivalentTo(descriptor4Name))
}
