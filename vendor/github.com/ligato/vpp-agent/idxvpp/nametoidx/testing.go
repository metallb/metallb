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
	"github.com/ligato/vpp-agent/idxvpp"

	"testing"
	"time"

	. "github.com/onsi/gomega"
)

// Factory defines type of a function used to create new instances of a name-to-index mapping.
type Factory func(reloaded bool) (idxvpp.NameToIdxRW, error)

// MappingName defines type for names in the mappings.
type MappingName string

// MappingIdx defines type for indexes in the mappings.
type MappingIdx uint32

// GivenKW defines the initial state of a testing scenario.
type GivenKW struct {
	nameToIdxFactory Factory
	plug1NameIdx     idxvpp.NameToIdxRW
	plug1NameIdxChan chan idxvpp.NameToIdxDto
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
	name MappingName
}

// ThenName defines actions/changes expected from the registry for a given name.
type ThenName struct {
	then *Then
	name MappingName
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
func (given *GivenKW) NameToIdx(idxMapFactory Factory, reg map[MappingName]MappingIdx) *GivenKW {
	Expect(given.nameToIdxFactory).Should(BeNil())
	Expect(given.plug1NameIdx).Should(BeNil())
	var err error
	given.nameToIdxFactory = idxMapFactory
	given.plug1NameIdx, err = idxMapFactory(false)
	Expect(err).Should(BeNil())

	for name, idx := range reg {
		n := string(name)
		given.plug1NameIdx.RegisterName(n, uint32(idx), nil)
	}

	// Registration of given mappings is done before watch (therefore there will be no notifications).
	given.watchNameIdx()
	return given
}

func (given *GivenKW) watchNameIdx() {
	plug1NameIdxChan := make(chan idxvpp.NameToIdxDto, 1000)
	given.plug1NameIdx.Watch("plugin2", ToChan(plug1NameIdxChan))
	given.plug1NameIdxChan = make(chan idxvpp.NameToIdxDto, 1000)
	go func() {
		for {
			v := <-plug1NameIdxChan
			given.plug1NameIdxChan <- v
			v.Done() // We can mark event as processed by calling Done() because we want to have events buffered in given.plug1NameIdxChan (because of assertions).
		}
	}()

}

// Then starts a then-clause.
func (when *When) Then() *Then {
	return &Then{when: when}
}

// NameToIdxIsReloaded simulates a full registry reload.
func (when *When) NameToIdxIsReloaded() *When {
	Expect(when.given.nameToIdxFactory).ShouldNot(BeNil())
	Expect(when.given.plug1NameIdx).ShouldNot(BeNil())

	when.given.plug1NameIdx = nil

	var err error
	when.given.plug1NameIdx, err = when.given.nameToIdxFactory(true)
	Expect(err).Should(BeNil())
	return when
}

// Name associates when-clause with a given name in the registry.
func (when *When) Name(name MappingName) *WhenName {
	return &WhenName{when: when, name: name}
}

// IsUnRegistered un-registers a given name from the registry.
func (whenName *WhenName) IsUnRegistered() *WhenName {
	name := string(whenName.name)
	whenName.when.given.plug1NameIdx.UnregisterName(name)

	return whenName
}

// Then starts a then-clause.
func (whenName *WhenName) Then() *Then {
	return &Then{when: whenName.when}
}

// IsRegistered registers a given name-index pair into the registry.
func (whenName *WhenName) IsRegistered(idx MappingIdx) *WhenName {
	name := string(whenName.name)
	whenName.when.given.plug1NameIdx.RegisterName(name, uint32(idx), nil)
	return whenName
}

// And connects two when-clauses.
func (whenName *WhenName) And() *When {
	return whenName.when
}

// Name associates then-clause with a given name in the registry.
func (then *Then) Name(name MappingName) *ThenName {
	return &ThenName{then: then, name: name}
}

// MapsToNothing verifies that a given name really maps to nothing.
func (thenName *ThenName) MapsToNothing() *ThenName {
	name := string(thenName.name)
	_, _, exist := thenName.then.when.given.plug1NameIdx.LookupIdx(name)
	Expect(exist).Should(BeFalse())

	return thenName
}

//MapsTo asserts the response of LookupIdx, LookupName and message in the channel.
func (thenName *ThenName) MapsTo(expectedIdx MappingIdx) *ThenName {
	name := string(thenName.name)
	retIdx, _, exist := thenName.then.when.given.plug1NameIdx.LookupIdx(name)
	Expect(exist).Should(BeTrue())
	Expect(retIdx).Should(Equal(uint32(expectedIdx)))

	retName, _, exist := thenName.then.when.given.plug1NameIdx.LookupName(retIdx)
	Expect(exist).Should(BeTrue())
	Expect(retName).ShouldNot(BeNil())
	Expect(retName).Should(Equal(name))

	return thenName
}

// Name associates then-clause with a given name in the registry.
func (thenName *ThenName) Name(name MappingName) *ThenName {
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
	name MappingName
	del  DelWriteEnum
}

// DelWriteEnum defines type for the flag used to tell if a mapping was removed or not.
type DelWriteEnum bool

// Del defines the value of a notification flag used when a mapping was removed.
const Del DelWriteEnum = true

// Write defines the value of a notification flag used when a mapping was created.
const Write DelWriteEnum = false

// Notification starts a section of then-clause referring to a given notification.
func (then *Then) Notification(name MappingName, del DelWriteEnum) *ThenNotification {
	return &ThenNotification{then: then, name: name, del: del}
}

// IsNotExpected verifies that a given notification was indeed NOT received.
func (thenNotif *ThenNotification) IsNotExpected() *ThenNotification {
	_, exist := thenNotif.receiveChan()
	Expect(exist).Should(BeFalse())
	return thenNotif
}

// IsExpectedFor verifies that a given notification was really received.
func (thenNotif *ThenNotification) IsExpectedFor(idx MappingIdx) *ThenNotification {
	notif, exist := thenNotif.receiveChan()
	Expect(exist).Should(BeTrue())
	//Expect(notif.Idx).Should(BeEquivalentTo(uint32(int)))
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

func (thenNotif *ThenNotification) receiveChan() (*idxvpp.NameToIdxDto, bool) {
	ch := thenNotif.then.when.given.plug1NameIdxChan
	var x idxvpp.NameToIdxDto
	select {
	case x = <-ch:
		return &x, true
	case <-time.After(time.Second * 1):
		return nil, false
	}
}
