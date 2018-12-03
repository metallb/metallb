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

package utils

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// PfxStack is helper struct while creating tree output.
type PfxStack struct {
	Entries    []PfxStackEntry
	Spaces     int
	FirstDash  string
	MiddleDash string
	LastDash   string
}

// PfxStackEntry represents entry in prefix stack entry list.
type PfxStackEntry struct {
	Preamble string
	Last     bool
}

// GetPrefix returns prefix as a sum of preambles.
func (stack *PfxStack) GetPrefix() string {
	var pfx = ""
	for _, se := range stack.Entries {
		pfx = pfx + se.Preamble
	}
	return pfx
}

// SetLast sets the top element of the prefix stack to display
// the Last element of a list.
func (stack *PfxStack) SetLast() {
	stack.Entries[len(stack.Entries)-1].Preamble = stack.GetPreamble(stack.LastDash)
	stack.Entries[len(stack.Entries)-1].Last = true
}

// Push increases the current prefix stack level (i.e. it makes the prefix
// stack longer). If the list at the current level continues (i.e. the
// list element is not the Last element), the current prefix element
// is replaced with a vertical bar (MiddleDash) icon. If the current
// element is the Last element of a list, the current prefix element
// is replaced with a space (i.e. the vertical line in the tree will
// not continue).
func (stack *PfxStack) Push() {
	// Replace current entry at the top of the prefix stack with either
	// vertical bar or empty space.
	if len(stack.Entries) > 0 {
		if stack.Entries[len(stack.Entries)-1].Last {
			stack.Entries[len(stack.Entries)-1].Preamble =
				fmt.Sprintf("%s",
					strings.Repeat(" ", stack.Spaces+utf8.RuneCountInString(stack.LastDash)))

		} else {
			stack.Entries[len(stack.Entries)-1].Preamble = stack.GetPreamble(stack.MiddleDash)
		}

	}
	// Add new entry at the top of the prefix stack.
	stack.Entries = append(stack.Entries, PfxStackEntry{
		Preamble: stack.GetPreamble(stack.FirstDash),
		Last:     false})
}

// Pop increases the current prefix stack level (i.e. it make the
// prefix stack shorter). If the element at the top of the
// prefix stack is not the the Last element on a list after Pop, it's replaced
// with the list element (FirstDash) icon.
func (stack *PfxStack) Pop() {
	stack.Entries = stack.Entries[:len(stack.Entries)-1]
	if !stack.Entries[len(stack.Entries)-1].Last {
		stack.Entries[len(stack.Entries)-1].Preamble = stack.GetPreamble(stack.FirstDash)
	}
}

// GetPreamble creates the string for a prefix stack entry. The
// prefix itself is then created by joining all prefix Entries.
func (stack *PfxStack) GetPreamble(icon string) string {
	return fmt.Sprintf("%s%s", strings.Repeat(" ", stack.Spaces), icon)
}

func (stack *PfxStack) setTopPfxStackEntry(new string) string {
	prev := stack.Entries[len(stack.Entries)-1].Preamble
	stack.Entries[len(stack.Entries)-1].Preamble = new
	return prev
}

func (stack *PfxStack) getTopPfxStackEntry() string {
	return stack.Entries[len(stack.Entries)-1].Preamble
}
