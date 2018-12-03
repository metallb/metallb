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

package utils_test

import (
	"testing"

	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/onsi/gomega"
)

// Test01GetPrefix tests whether prefix can be obtained using GetPrefix() method.
func Test01GetPrefix(t *testing.T) {
	gomega.RegisterTestingT(t)

	prefixStack := getPrefixStack()
	result := prefixStack.GetPrefix()
	gomega.Expect(result).To(gomega.BeEquivalentTo("-"))
}

// Test02SetLast sets correct flag setup for last entry in prefix stack.
func Test02SetLast(t *testing.T) {
	gomega.RegisterTestingT(t)

	prefixStack := getPrefixStack()

	gomega.Expect(prefixStack.Entries[0].Last).To(gomega.BeFalse())
	prefixStack.SetLast()
	gomega.Expect(prefixStack.Entries[0].Last).To(gomega.BeTrue())
}

// Test03Push tests correct functionality of pushing prefix stack entries.
func Test03Push(t *testing.T) {
	gomega.RegisterTestingT(t)

	prefixStack := getPrefixStack()

	gomega.Expect(len(prefixStack.Entries)).To(gomega.BeEquivalentTo(1))
	prefixStack.Push()
	gomega.Expect(len(prefixStack.Entries)).To(gomega.BeEquivalentTo(2))
}

// Test04Pop tests correct functionality of popping prefix stack entries.
func Test04Pop(t *testing.T) {
	gomega.RegisterTestingT(t)

	prefixStack := getPrefixStack()
	prefixStack.Push()
	prefixStack.Push()

	gomega.Expect(len(prefixStack.Entries)).To(gomega.BeEquivalentTo(3))
	prefixStack.Pop()
	gomega.Expect(len(prefixStack.Entries)).To(gomega.BeEquivalentTo(2))
}

// Test05PfxStack_GetPreamble tests correct format of returned icon with preamble.
func Test05PfxStack_GetPreamble(t *testing.T) {
	gomega.RegisterTestingT(t)

	prefixStack := getPrefixStack()

	result := prefixStack.GetPreamble("icon")
	// Added three spaces (as defined in 'getPrefixStack')
	gomega.Expect(result).To(gomega.BeEquivalentTo("   icon"))
}

func getPrefixStack() *utils.PfxStack {
	pfxEntry := utils.PfxStackEntry{
		Preamble: "-",
		Last:     false,
	}

	return &utils.PfxStack{
		Entries:    []utils.PfxStackEntry{pfxEntry},
		Spaces:     3,
		FirstDash:  "├─",
		MiddleDash: "│ ",
		LastDash:   "└─",
	}
}
