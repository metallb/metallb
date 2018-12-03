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

package aclidx_test

import (
	"testing"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/aclplugin/aclidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
	. "github.com/onsi/gomega"
)

func aclIndexTestInitialization(t *testing.T) (idxvpp.NameToIdxRW, aclidx.ACLIndexRW) {
	RegisterTestingT(t)

	// initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "index_test", nil)
	index := aclidx.NewACLIndex(nameToIdx)
	names := nameToIdx.ListNames()

	// check if names were empty
	Expect(names).To(BeEmpty())

	return index.GetMapping(), index
}

var acldata = acl.AccessLists_Acl{
	AclName:    "acl1",
	Rules:      []*acl.AccessLists_Acl_Rule{{AclAction: acl.AclAction_PERMIT}},
	Interfaces: &acl.AccessLists_Acl_Interfaces{},
}

// Tests registering and unregistering name to index
func TestRegisterAndUnregisterName(t *testing.T) {
	mapping, index := aclIndexTestInitialization(t)

	// Register entry
	index.RegisterName("acl1", 0, &acldata)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement("acl1"))

	// Unregister entry
	index.UnregisterName("acl1")
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests index mapping clear
func TestClear(t *testing.T) {
	mapping, index := aclIndexTestInitialization(t)

	// Register entries
	index.RegisterName("acl1", 0, nil)
	index.RegisterName("acl2", 1, nil)
	index.RegisterName("acl3", 2, nil)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(3))

	// Clear
	index.Clear()
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

func TestLookupIndex(t *testing.T) {
	RegisterTestingT(t)

	_, aclIndex := aclIndexTestInitialization(t)

	aclIndex.RegisterName("acl", 0, &acldata)

	foundName, acl, exist := aclIndex.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal("acl"))
	Expect(acl.AclName).To(Equal("acl1"))
}

func TestLookupName(t *testing.T) {
	RegisterTestingT(t)

	_, aclIndex := aclIndexTestInitialization(t)

	aclIndex.RegisterName("acl", 0, &acldata)

	foundName, acl, exist := aclIndex.LookupIdx("acl")
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal(uint32(0)))
	Expect(acl.AclName).To(Equal("acl1"))
}

func TestWatchNameToIdx(t *testing.T) {
	RegisterTestingT(t)

	_, aclIndex := aclIndexTestInitialization(t)

	c := make(chan aclidx.IdxDto)
	aclIndex.WatchNameToIdx("testName", c)

	aclIndex.RegisterName("aclX", 0, &acldata)

	var dto aclidx.IdxDto
	Eventually(c).Should(Receive(&dto))
	Expect(dto.Name).To(Equal("aclX"))
	Expect(dto.NameToIdxDtoWithoutMeta.Idx).To(Equal(uint32(0)))
}
