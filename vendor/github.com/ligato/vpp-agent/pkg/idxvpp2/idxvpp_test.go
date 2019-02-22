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

package idxvpp2

import (
	"testing"

	"strconv"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/onsi/gomega"
)

const (
	idx1 = 1
	idx2 = 2
	idx3 = 3
)

var (
	eth0 = "eth0"
	eth1 = "eth1"
	eth2 = "eth2"
)

func IndexFactory() (NameToIndexRW, error) {
	return NewNameToIndex(logrus.DefaultLogger(), "test", nil), nil
}

func Test01UnregisteredMapsToNothing(t *testing.T) {
	Given(t).NameToIdx(IndexFactory, nil).
		When().Name(eth1).IsDeleted().
		Then().Name(eth1).MapsToNothing().
		And().Notification(eth1, Write).IsNotExpected()
}

func Test02RegisteredReturnsIdx(t *testing.T) {
	Given(t).NameToIdx(IndexFactory, nil).
		When().Name(eth1).IsAdded(idx1).
		Then().Name(eth1).MapsTo(idx1).
		And().Notification(eth1, Write).IsExpectedFor(idx1)
}

func Test03RegFirstThenUnreg(t *testing.T) {
	Given(t).NameToIdx(IndexFactory, map[string]uint32{eth1: idx1}).
		When().Name(eth1).IsDeleted().
		Then().Name(eth1).MapsToNothing().
		And().Notification(eth1, Del).IsExpectedFor(idx1)
}

func Test03Eth0RegPlusEth1Unreg(t *testing.T) {
	Given(t).NameToIdx(IndexFactory, map[string]uint32{eth0: idx1, eth1: idx2}).
		When().Name(eth1).IsDeleted().
		Then().Name(eth1).MapsToNothing().
		And().Notification(eth1, Del).IsExpectedFor(idx2).
		And().Name(eth0).MapsTo(idx1).
		And().Notification(eth0, Write).IsNotExpected() //because watch is registered after given keyword
}

func Test04RegTwiceSameNameWithDifferentIdx(t *testing.T) {
	Given(t).NameToIdx(IndexFactory, nil).
		When().Name(eth1).IsAdded(idx1).
		Then().Name(eth1).MapsTo(idx1). //Notif eth1, idx1
		And().Notification(eth1, Write).IsExpectedFor(idx1).
		When().Name(eth1).IsAdded(idx2).
		Then().Name(eth1).MapsTo(idx2). //Notif eth1, idx1
		And().Notification(eth1, Write).IsExpectedFor(idx2)
}

const (
	flagKey = "flag"
	valsKey = "vals"
)

type Item struct {
	index uint32
	flag  bool
	vals  []string
}

func (item *Item) GetIndex() uint32 {
	return item.index
}

func createIdx(item interface{}) map[string][]string {
	typed, ok := item.(*Item)
	if !ok {
		return nil
	}

	return map[string][]string{
		flagKey: {strconv.FormatBool(typed.flag)},
		valsKey: typed.vals,
	}
}

func TestIndexedMetadata(t *testing.T) {
	gomega.RegisterTestingT(t)
	idxm := NewNameToIndex(logrus.DefaultLogger(), "title", createIdx)

	res := idxm.ListNames(flagKey, "true")
	gomega.Expect(res).To(gomega.BeNil())

	item1 := &Item{
		index: idx1,
		flag:  true,
		vals:  []string{"abc", "def", "xyz"},
	}
	item2 := &Item{
		index: idx2,
		flag:  false,
		vals:  []string{"abc", "klm", "opq"},
	}
	item3 := &Item{
		index: idx3,
		flag:  true,
		vals:  []string{"jkl"},
	}

	idxm.Put(eth0, item1)
	idxm.Put(eth1, item2)
	idxm.Put(eth2, item3)

	res = idxm.ListNames(flagKey, "false")
	gomega.Expect(res).NotTo(gomega.BeNil())
	gomega.Expect(res[0]).To(gomega.BeEquivalentTo(eth1))

	res = idxm.ListNames(flagKey, "true")
	gomega.Expect(len(res)).To(gomega.BeEquivalentTo(2))
	gomega.Expect(res).To(gomega.ContainElement(string(eth0)))
	gomega.Expect(res).To(gomega.ContainElement(string(eth2)))

	res = idxm.ListNames(valsKey, "abc")
	gomega.Expect(len(res)).To(gomega.BeEquivalentTo(2))
	gomega.Expect(res).To(gomega.ContainElement(string(eth0)))
	gomega.Expect(res).To(gomega.ContainElement(string(eth1)))

	res = idxm.ListNames(valsKey, "jkl")
	gomega.Expect(len(res)).To(gomega.BeEquivalentTo(1))
	gomega.Expect(res[0]).To(gomega.BeEquivalentTo(eth2))

	idxm.Delete(eth0)
	res = idxm.ListNames(flagKey, "true")
	gomega.Expect(len(res)).To(gomega.BeEquivalentTo(1))
	gomega.Expect(res[0]).To(gomega.BeEquivalentTo(eth2))

}

func TestOldIndexRemove(t *testing.T) {
	gomega.RegisterTestingT(t)
	idxm := NewNameToIndex(logrus.DefaultLogger(), "title", nil)

	idxm.Put(eth0, &OnlyIndex{idx1})

	item, found := idxm.LookupByName(eth0)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(item.GetIndex()).To(gomega.BeEquivalentTo(idx1))

	name, _, found := idxm.LookupByIndex(idx1)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(name).To(gomega.BeEquivalentTo(eth0))

	idxm.Put(eth0, &OnlyIndex{idx2})

	item, found = idxm.LookupByName(eth0)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(item.GetIndex()).To(gomega.BeEquivalentTo(idx2))

	name, item, found = idxm.LookupByIndex(idx2)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(name).To(gomega.BeEquivalentTo(string(eth0)))
	gomega.Expect(item).ToNot(gomega.BeNil())

	name, item, found = idxm.LookupByIndex(idx1)
	gomega.Expect(found).To(gomega.BeFalse())
	gomega.Expect(name).To(gomega.BeEquivalentTo(""))
	gomega.Expect(item).To(gomega.BeNil())
}

func TestUpdateIndex(t *testing.T) {
	gomega.RegisterTestingT(t)
	idxm := NewNameToIndex(logrus.DefaultLogger(), "title", nil)

	idxm.Put(eth0, &OnlyIndex{idx1})

	item, found := idxm.LookupByName(eth0)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(item.GetIndex()).To(gomega.BeEquivalentTo(idx1))

	success := idxm.Update(eth0, &OnlyIndex{idx2})
	gomega.Expect(success).To(gomega.BeTrue())

	item, found = idxm.LookupByName(eth0)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(item.GetIndex()).To(gomega.BeEquivalentTo(idx2))
}

func TestClearMapping(t *testing.T) {
	gomega.RegisterTestingT(t)
	idxm := NewNameToIndex(logrus.DefaultLogger(), "title", nil)

	idxm.Put(eth0, &OnlyIndex{idx1})
	idxm.Put(eth1, &OnlyIndex{idx2})
	idxm.Put(eth2, &OnlyIndex{idx3})

	item, found := idxm.LookupByName(eth0)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(item.GetIndex()).To(gomega.BeEquivalentTo(idx1))

	item, found = idxm.LookupByName(eth1)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(item.GetIndex()).To(gomega.BeEquivalentTo(idx2))

	item, found = idxm.LookupByName(eth2)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(item.GetIndex()).To(gomega.BeEquivalentTo(idx3))

	idxm.Clear()

	_, found = idxm.LookupByName(eth0)
	gomega.Expect(found).To(gomega.BeFalse())

	_, found = idxm.LookupByName(eth1)
	gomega.Expect(found).To(gomega.BeFalse())

	_, found = idxm.LookupByName(eth2)
	gomega.Expect(found).To(gomega.BeFalse())
}
