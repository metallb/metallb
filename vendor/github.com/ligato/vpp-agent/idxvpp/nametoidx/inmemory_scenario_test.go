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

package nametoidx

import (
	"testing"

	"strconv"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/onsi/gomega"
)

const (
	idx1 = 1
	idx2 = 2
	idx3 = 3
)

var (
	eth0 MappingName = "eth0"
	eth1 MappingName = "eth1"
	eth2 MappingName = "eth2"
)

func InMemory(reloaded bool) (idxvpp.NameToIdxRW, error) {
	return NewNameToIdx(logrus.DefaultLogger(), "test", nil), nil
}

func Test01UnregisteredMapsToNothing(t *testing.T) {
	Given(t).NameToIdx(InMemory, nil).
		When().Name(eth1).IsUnRegistered().
		Then().Name(eth1).MapsToNothing().
		And().Notification(eth1, Write).IsNotExpected()
}

func Test02RegisteredReturnsIdx(t *testing.T) {
	Given(t).NameToIdx(InMemory, nil).
		When().Name(eth1).IsRegistered(idx1).
		Then().Name(eth1).MapsTo(idx1).
		And().Notification(eth1, Write).IsExpectedFor(idx1)
}

func Test03RegFirstThenUnreg(t *testing.T) {
	Given(t).NameToIdx(InMemory, map[MappingName]MappingIdx{eth1: idx1}).
		When().Name(eth1).IsUnRegistered().
		Then().Name(eth1).MapsToNothing().
		And().Notification(eth1, Del).IsExpectedFor(idx1)
}

func Test03Eth0RegPlusEth1Unreg(t *testing.T) {
	Given(t).NameToIdx(InMemory, map[MappingName]MappingIdx{eth0: idx1, eth1: idx2}).
		When().Name(eth1).IsUnRegistered().
		Then().Name(eth1).MapsToNothing().
		And().Notification(eth1, Del).IsExpectedFor(idx2).
		And().Name(eth0).MapsTo(idx1).
		And().Notification(eth0, Write).IsNotExpected() //because watch is registered after given keyword
}

func Test04RegTwiceSameNameWithDifferentIdx(t *testing.T) {
	Given(t).NameToIdx(InMemory, nil).
		When().Name(eth1).IsRegistered(idx1).
		Then().Name(eth1).MapsTo(idx1). //Notif eth1, idx1
		And().Notification(eth1, Write).IsExpectedFor(idx1).
		When().Name(eth1).IsRegistered(idx2).
		Then().Name(eth1).MapsTo(idx2). //Notif eth1, idx1
		And().Notification(eth1, Write).IsExpectedFor(idx2)
}

const (
	flagMetaKey = "flag"
	valsMetaKey = "vals"
)

type metaInformation struct {
	flag bool
	vals []string
}

func createIdx(meta interface{}) map[string][]string {
	typed, ok := meta.(*metaInformation)
	if !ok {
		return nil
	}

	return map[string][]string{
		flagMetaKey: {strconv.FormatBool(typed.flag)},
		valsMetaKey: typed.vals,
	}
}

func TestIndexedMetadata(t *testing.T) {
	gomega.RegisterTestingT(t)
	idxm := NewNameToIdx(logrus.DefaultLogger(), "title", createIdx)

	res := idxm.LookupNameByMetadata(flagMetaKey, "true")
	gomega.Expect(res).To(gomega.BeNil())

	meta1 := &metaInformation{
		flag: true,
		vals: []string{"abc", "def", "xyz"},
	}
	meta2 := &metaInformation{
		flag: false,
		vals: []string{"abc", "klm", "opq"},
	}
	meta3 := &metaInformation{
		flag: true,
		vals: []string{"jkl"},
	}

	idxm.RegisterName(string(eth0), idx1, meta1)
	idxm.RegisterName(string(eth1), idx2, meta2)
	idxm.RegisterName(string(eth2), idx3, meta3)

	res = idxm.LookupNameByMetadata(flagMetaKey, "false")
	gomega.Expect(res).NotTo(gomega.BeNil())
	gomega.Expect(res[0]).To(gomega.BeEquivalentTo(eth1))

	res = idxm.LookupNameByMetadata(flagMetaKey, "true")
	gomega.Expect(len(res)).To(gomega.BeEquivalentTo(2))
	gomega.Expect(res).To(gomega.ContainElement(string(eth0)))
	gomega.Expect(res).To(gomega.ContainElement(string(eth2)))

	res = idxm.LookupNameByMetadata(valsMetaKey, "abc")
	gomega.Expect(len(res)).To(gomega.BeEquivalentTo(2))
	gomega.Expect(res).To(gomega.ContainElement(string(eth0)))
	gomega.Expect(res).To(gomega.ContainElement(string(eth1)))

	res = idxm.LookupNameByMetadata(valsMetaKey, "jkl")
	gomega.Expect(len(res)).To(gomega.BeEquivalentTo(1))
	gomega.Expect(res[0]).To(gomega.BeEquivalentTo(eth2))

	idxm.UnregisterName(string(eth0))
	res = idxm.LookupNameByMetadata(flagMetaKey, "true")
	gomega.Expect(len(res)).To(gomega.BeEquivalentTo(1))
	gomega.Expect(res[0]).To(gomega.BeEquivalentTo(eth2))

}

func TestOldIndexRemove(t *testing.T) {
	gomega.RegisterTestingT(t)
	idxm := NewNameToIdx(logrus.DefaultLogger(), "title", nil)

	idxm.RegisterName(string(eth0), idx1, nil)

	idx, _, found := idxm.LookupIdx(string(eth0))
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(idx).To(gomega.BeEquivalentTo(idx1))

	name, _, found := idxm.LookupName(idx1)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(name).To(gomega.BeEquivalentTo(string(name)))

	idxm.RegisterName(string(eth0), idx2, nil)

	idx, _, found = idxm.LookupIdx(string(eth0))
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(idx).To(gomega.BeEquivalentTo(idx2))

	name, _, found = idxm.LookupName(idx2)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(name).To(gomega.BeEquivalentTo(string(name)))

	name, _, found = idxm.LookupName(idx1)
	gomega.Expect(found).To(gomega.BeFalse())
	gomega.Expect(name).To(gomega.BeEquivalentTo(""))
}

func TestUpdateMetadata(t *testing.T) {
	gomega.RegisterTestingT(t)
	idxm := NewNameToIdx(logrus.DefaultLogger(), "title", nil)

	idxm.RegisterName(string(eth0), idx1, nil)

	idx, meta, found := idxm.LookupIdx(string(eth0))
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(idx).To(gomega.BeEquivalentTo(idx1))
	gomega.Expect(meta).To(gomega.BeNil())

	success := idxm.UpdateMetadata(string(eth0), "dummy-meta")
	gomega.Expect(success).To(gomega.BeTrue())

	idx, meta, found = idxm.LookupIdx(string(eth0))
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(idx).To(gomega.BeEquivalentTo(idx1))
	gomega.Expect(meta).ToNot(gomega.BeNil())
}

func TestClearMapping(t *testing.T) {
	gomega.RegisterTestingT(t)
	idxm := NewNameToIdx(logrus.DefaultLogger(), "title", nil)

	idxm.RegisterName(string(eth0), idx1, nil)
	idxm.RegisterName(string(eth1), idx2, nil)
	idxm.RegisterName(string(eth2), idx3, nil)

	idx, _, found := idxm.LookupIdx(string(eth0))
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(idx).To(gomega.BeEquivalentTo(idx1))

	idx, _, found = idxm.LookupIdx(string(eth1))
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(idx).To(gomega.BeEquivalentTo(idx2))

	idx, _, found = idxm.LookupIdx(string(eth2))
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(idx).To(gomega.BeEquivalentTo(idx3))

	idxm.Clear()

	_, _, found = idxm.LookupIdx(string(eth0))
	gomega.Expect(found).To(gomega.BeFalse())

	_, _, found = idxm.LookupIdx(string(eth1))
	gomega.Expect(found).To(gomega.BeFalse())

	_, _, found = idxm.LookupIdx(string(eth2))
	gomega.Expect(found).To(gomega.BeFalse())
}
