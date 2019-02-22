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
	"time"

	. "github.com/onsi/gomega"
)

// Factory defines type of a function used to create new instances of a NameToIndex mapping.
type Factory func() (NameToIndexRW, error)

// GivenKW defines the initial state of a testing scenario.
type GivenKW struct {
	nameToIndexFactory Factory
	nameToIndex        NameToIndexRW
	nameToIndexChan    chan NameToIndexDto
}

// When defines the actions/changes done to the tested registry.
type When struct {
	given *GivenKW
}

// Then defines the actions/changes expected from the tested registry.
type Then struct {
	when *When
}

// WhenName defines the actions/changes done to a registry for a given name.
type WhenName struct {
	when *When
	name string
}

// ThenName defines actions/changes expected from the registry for a given name.
type ThenName struct {
	then *Then
	name string
}

// Given prepares the initial state of a testing scenario.
func Given(t *testing.T) *GivenKW {
	RegisterTestingT(t)

	return &GivenKW{}
}

// When starts when-clause.
func (given *GivenKW) When() *When {
	return &When{given: given}
}

// NameToIdx sets up a given registry for the tested scenario.
func (given *GivenKW) NameToIdx(idxMapFactory Factory, reg map[string]uint32) *GivenKW {
	Expect(given.nameToIndexFactory).Should(BeNil())
	Expect(given.nameToIndex).Should(BeNil())
	var err error
	given.nameToIndexFactory = idxMapFactory
	given.nameToIndex, err = idxMapFactory()
	Expect(err).Should(BeNil())

	for name, idx := range reg {
		given.nameToIndex.Put(name, &OnlyIndex{idx})
	}

	// Registration of given mappings is done before watch (therefore there will be no notifications).
	given.watchNameIdx()
	return given
}

func (given *GivenKW) watchNameIdx() {
	given.nameToIndexChan = make(chan NameToIndexDto, 1000)
	given.nameToIndex.WatchItems("plugin2", given.nameToIndexChan)
}

// Then starts a then-clause.
func (when *When) Then() *Then {
	return &Then{when: when}
}

// Name associates when-clause with a given name in the registry.
func (when *When) Name(name string) *WhenName {
	return &WhenName{when: when, name: name}
}

// IsDeleted removes a given name from the registry.
func (whenName *WhenName) IsDeleted() *WhenName {
	name := string(whenName.name)
	whenName.when.given.nameToIndex.Delete(name)

	return whenName
}

// Then starts a then-clause.
func (whenName *WhenName) Then() *Then {
	return &Then{when: whenName.when}
}

// IsAdded adds a given name-index pair into the registry.
func (whenName *WhenName) IsAdded(idx uint32) *WhenName {
	name := string(whenName.name)
	whenName.when.given.nameToIndex.Put(name, &OnlyIndex{idx})
	return whenName
}

// And connects two when-clauses.
func (whenName *WhenName) And() *When {
	return whenName.when
}

// Name associates then-clause with a given name in the registry.
func (then *Then) Name(name string) *ThenName {
	return &ThenName{then: then, name: name}
}

// MapsToNothing verifies that a given name really maps to nothing.
func (thenName *ThenName) MapsToNothing() *ThenName {
	name := string(thenName.name)
	_, exist := thenName.then.when.given.nameToIndex.LookupByName(name)
	Expect(exist).Should(BeFalse())

	return thenName
}

//MapsTo asserts the response of LookupIdx, LookupName and message in the channel.
func (thenName *ThenName) MapsTo(expectedIdx uint32) *ThenName {
	name := string(thenName.name)
	item, exist := thenName.then.when.given.nameToIndex.LookupByName(name)
	Expect(exist).Should(BeTrue())
	Expect(item.GetIndex()).Should(Equal(uint32(expectedIdx)))

	retName, _, exist := thenName.then.when.given.nameToIndex.LookupByIndex(item.GetIndex())
	Expect(exist).Should(BeTrue())
	Expect(retName).ShouldNot(BeNil())
	Expect(retName).Should(Equal(name))

	return thenName
}

// Name associates then-clause with a given name in the registry.
func (thenName *ThenName) Name(name string) *ThenName {
	return &ThenName{then: thenName.then, name: name}
}

// And connects two then-clauses.
func (thenName *ThenName) And() *Then {
	return thenName.then
}

// When starts a when-clause.
func (thenName *ThenName) When() *When {
	return thenName.then.when
}

// ThenNotification defines notification parameters for a then-clause.
type ThenNotification struct {
	then *Then
	name string
	del  DelWriteEnum
}

// DelWriteEnum defines type for the flag used to tell if a mapping was removed or not.
type DelWriteEnum bool

// Del defines the value of a notification flag used when a mapping was removed.
const Del DelWriteEnum = true

// Write defines the value of a notification flag used when a mapping was created.
const Write DelWriteEnum = false

// Notification starts a section of then-clause referring to a given notification.
func (then *Then) Notification(name string, del DelWriteEnum) *ThenNotification {
	return &ThenNotification{then: then, name: name, del: del}
}

// IsNotExpected verifies that a given notification was indeed NOT received.
func (thenNotif *ThenNotification) IsNotExpected() *ThenNotification {
	_, exist := thenNotif.receiveChan()
	Expect(exist).Should(BeFalse())
	return thenNotif
}

// IsExpectedFor verifies that a given notification was really received.
func (thenNotif *ThenNotification) IsExpectedFor(idx uint32) *ThenNotification {
	notif, exist := thenNotif.receiveChan()
	Expect(exist).Should(BeTrue())
	Expect(notif.Item.GetIndex()).Should(BeEquivalentTo(uint32(idx)))
	Expect(notif.Del).Should(BeEquivalentTo(bool(thenNotif.del)))
	return thenNotif
}

// And connects two then-clauses.
func (thenNotif *ThenNotification) And() *Then {
	return thenNotif.then
}

// When starts a when-clause.
func (thenNotif *ThenNotification) When() *When {
	return thenNotif.then.when
}

func (thenNotif *ThenNotification) receiveChan() (*NameToIndexDto, bool) {
	ch := thenNotif.then.when.given.nameToIndexChan
	var x NameToIndexDto
	select {
	case x = <-ch:
		return &x, true
	case <-time.After(time.Second * 1):
		return nil, false
	}
}
