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

package mem

import (
	"testing"
	"time"

	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/onsi/gomega"
	"strings"
)

func TestNewNamedMappingMem(t *testing.T) {
	gomega.RegisterTestingT(t)
	title := "Title"
	mapping := NewNamedMapping(logrus.DefaultLogger(), title, nil)
	returnedTitle := mapping.GetRegistryTitle()
	gomega.Expect(returnedTitle).To(gomega.BeEquivalentTo(title))

	names := mapping.ListAllNames()
	gomega.Expect(names).To(gomega.BeNil())
}

func TestUpdateMetadata(t *testing.T) {
	gomega.RegisterTestingT(t)
	mapping := NewNamedMapping(logrus.DefaultLogger(), "title", nil)

	success := mapping.Update("Name1", "value1")
	gomega.Expect(success).To(gomega.BeFalse())

	mapping.Put("Name1", "value1")
	meta, found := mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("value1"))

	success = mapping.Update("Name1", "value2")
	gomega.Expect(success).To(gomega.BeTrue())

	meta, found = mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("value2"))
}

func TestClear(t *testing.T) {
	gomega.RegisterTestingT(t)
	mapping := NewNamedMapping(logrus.DefaultLogger(), "title", nil)

	mapping.Put("Name1", "value1")
	mapping.Put("Name2", "value2")
	mapping.Put("Name3", "value3")

	_, found := mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeTrue())
	_, found = mapping.GetValue("Name2")
	gomega.Expect(found).To(gomega.BeTrue())
	_, found = mapping.GetValue("Name3")
	gomega.Expect(found).To(gomega.BeTrue())

	mapping.Clear()

	_, found = mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeFalse())
	_, found = mapping.GetValue("Name2")
	gomega.Expect(found).To(gomega.BeFalse())
	_, found = mapping.GetValue("Name3")
	gomega.Expect(found).To(gomega.BeFalse())
}

func TestCrudOps(t *testing.T) {
	gomega.RegisterTestingT(t)
	mapping := NewNamedMapping(logrus.DefaultLogger(), "title", nil)

	mapping.Put("Name1", "value1")
	meta, found := mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("value1"))

	mapping.Put("Name2", "value2")
	meta, found = mapping.GetValue("Name2")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("value2"))

	mapping.Put("Name3", "value3")
	meta, found = mapping.GetValue("Name3")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("value3"))

	names := mapping.ListAllNames()
	gomega.Expect(names).To(gomega.ContainElement("Name1"))
	gomega.Expect(names).To(gomega.ContainElement("Name2"))
	gomega.Expect(names).To(gomega.ContainElement("Name3"))

	meta, found = mapping.Delete("Name2")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("value2"))

	meta, found = mapping.GetValue("Name2")
	gomega.Expect(found).To(gomega.BeFalse())
	gomega.Expect(meta).To(gomega.BeNil())

	meta, found = mapping.Delete("Unknown")
	gomega.Expect(found).To(gomega.BeFalse())
	gomega.Expect(meta).To(gomega.BeNil())
}

func TestSecondaryIndexes(t *testing.T) {
	gomega.RegisterTestingT(t)
	const secondaryIx = "secondary"
	mapping := NewNamedMapping(logrus.DefaultLogger(), "title", func(meta interface{}) map[string][]string {
		res := map[string][]string{}
		if str, ok := meta.(string); ok {
			res[secondaryIx] = []string{str, strings.ToLower(str), strings.ToUpper(str)}
		}
		return res
	})

	mapping.Put("Name1", "Value")
	meta, found := mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("Value"))
	fields := map[string][]string{secondaryIx: {"Value", "value", "VALUE"}}
	gomega.Expect(mapping.ListFields("Name1")).To(gomega.BeEquivalentTo(fields))

	mapping.Put("Name2", "Value")
	meta, found = mapping.GetValue("Name2")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("Value"))
	gomega.Expect(mapping.ListFields("Name2")).To(gomega.BeEquivalentTo(fields))

	mapping.Put("Name3", "Different")
	meta, found = mapping.GetValue("Name3")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("Different"))
	fields = map[string][]string{secondaryIx: {"Different", "different", "DIFFERENT"}}
	gomega.Expect(mapping.ListFields("Name3")).To(gomega.BeEquivalentTo(fields))

	names := mapping.ListNames(secondaryIx, "Value")
	gomega.Expect(names).To(gomega.ContainElement("Name1"))
	gomega.Expect(names).To(gomega.ContainElement("Name2"))
	gomega.Expect(names).To(gomega.HaveLen(2))

	names = mapping.ListNames(secondaryIx, "different")
	gomega.Expect(names).To(gomega.ContainElement("Name3"))
	gomega.Expect(names).To(gomega.HaveLen(1))

	names = mapping.ListNames(secondaryIx, "Unknown")
	gomega.Expect(names).To(gomega.BeNil())
	names = mapping.ListNames("Unknown index", "value")
	gomega.Expect(names).To(gomega.BeNil())

	mapping.Put("Name2", "Different")
	gomega.Expect(mapping.ListFields("Name2")).To(gomega.BeEquivalentTo(fields))
	names = mapping.ListNames(secondaryIx, "DIFFERENT")
	gomega.Expect(names).To(gomega.ContainElement("Name2"))
	gomega.Expect(names).To(gomega.ContainElement("Name3"))
	gomega.Expect(names).To(gomega.HaveLen(2))
	names = mapping.ListNames(secondaryIx, "value")
	gomega.Expect(names).To(gomega.ContainElement("Name1"))
	gomega.Expect(names).To(gomega.HaveLen(1))
}

func TestNotifications(t *testing.T) {
	gomega.RegisterTestingT(t)
	mapping := NewNamedMapping(logrus.DefaultLogger(), "title", nil)

	ch := make(chan idxmap.NamedMappingGenericEvent, 10)
	err := mapping.Watch("subscriber", idxmap.ToChan(ch))
	gomega.Expect(err).To(gomega.BeNil())

	mapping.Put("Name1", "value")
	meta, found := mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("value"))

	select {
	case notif := <-ch:
		gomega.Expect(notif.RegistryTitle).To(gomega.BeEquivalentTo("title"))
		gomega.Expect(notif.Del).To(gomega.BeFalse())
		gomega.Expect(notif.Update).To(gomega.BeFalse())
		gomega.Expect(notif.Name).To(gomega.BeEquivalentTo("Name1"))
		gomega.Expect(notif.Value).To(gomega.BeEquivalentTo("value"))
	case <-time.After(time.Second):
		t.FailNow()
	}

	mapping.Put("Name1", "modified")
	meta, found = mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("modified"))

	select {
	case notif := <-ch:
		gomega.Expect(notif.RegistryTitle).To(gomega.BeEquivalentTo("title"))
		gomega.Expect(notif.Del).To(gomega.BeFalse())
		gomega.Expect(notif.Update).To(gomega.BeFalse())
		gomega.Expect(notif.Name).To(gomega.BeEquivalentTo("Name1"))
		gomega.Expect(notif.Value).To(gomega.BeEquivalentTo("modified"))
	case <-time.After(time.Second):
		t.FailNow()
	}

	mapping.Update("Name1", "updated")
	meta, found = mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(meta).To(gomega.BeEquivalentTo("updated"))

	select {
	case notif := <-ch:
		gomega.Expect(notif.RegistryTitle).To(gomega.BeEquivalentTo("title"))
		gomega.Expect(notif.Del).To(gomega.BeFalse())
		gomega.Expect(notif.Update).To(gomega.BeTrue())
		gomega.Expect(notif.Name).To(gomega.BeEquivalentTo("Name1"))
		gomega.Expect(notif.Value).To(gomega.BeEquivalentTo("updated"))
	case <-time.After(time.Second):
		t.FailNow()
	}

	mapping.Delete("Name1")
	meta, found = mapping.GetValue("Name1")
	gomega.Expect(found).To(gomega.BeFalse())
	gomega.Expect(meta).To(gomega.BeNil())

	select {
	case notif := <-ch:
		gomega.Expect(notif.RegistryTitle).To(gomega.BeEquivalentTo("title"))
		gomega.Expect(notif.Del).To(gomega.BeTrue())
		gomega.Expect(notif.Update).To(gomega.BeFalse())
		gomega.Expect(notif.Name).To(gomega.BeEquivalentTo("Name1"))
		gomega.Expect(notif.Value).To(gomega.BeEquivalentTo("updated"))
	case <-time.After(time.Second):
		t.FailNow()
	}

	close(ch)
}
