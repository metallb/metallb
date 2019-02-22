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

package test

// A set of node flags used for testing of the Graph.

// TestingFlag as a base for flags in the UTs.
type TestingFlag struct {
	Name, Value string
}

// GetName returns the assigned name.
func (flag *TestingFlag) GetName() string {
	return flag.Name
}

// GetValue returns the assigned value.
func (flag *TestingFlag) GetValue() string {
	return flag.Value
}

// Color is a property to be assigned to nodes for testing purposes.
type Color int

const (
	// Red color.
	Red Color = iota
	// Blue color.
	Blue
	// Green color.
	Green
)

// String converts color to string.
func (color Color) String() string {
	switch color {
	case Red:
		return "red"
	case Blue:
		return "blue"
	case Green:
		return "green"
	}
	return "unknown"
}

// ColorFlagName is the name of the color flag.
const ColorFlagName = "color"

// ColorFlagImpl implements flag used in UTs to associate "color" with nodes.
type ColorFlagImpl struct {
	TestingFlag
	Color Color
}

// ColorFlag returns a new instance of color flag for testing.
func ColorFlag(color Color) *ColorFlagImpl {
	return &ColorFlagImpl{
		TestingFlag: TestingFlag{
			Name:  ColorFlagName,
			Value: color.String(),
		},
		Color: color,
	}
}

// AnyColorFlag can be used to match nodes with any color assigned.
func AnyColorFlag() *ColorFlagImpl {
	return &ColorFlagImpl{
		TestingFlag: TestingFlag{
			Name:  ColorFlagName,
			Value: "",
		},
	}
}

// AbstractFlagName is the name of the abstract flag.
const AbstractFlagName = "is-abstract"

// AbstractFlagImpl is used in UTs to mark "abstract" key-value pairs.
type AbstractFlagImpl struct {
	TestingFlag
}

// AbstractFlag returns a new instance of AbstractFlag for testing.
func AbstractFlag() *AbstractFlagImpl {
	return &AbstractFlagImpl{
		TestingFlag: TestingFlag{
			Name: AbstractFlagName,
			// empty value -> it is a boolean flag
		},
	}
}

// TemporaryFlagName is the name of the temporary flag.
const TemporaryFlagName = "is-temporary"

// TemporaryFlagImpl is used in UTs to mark "temporary" key-value pairs.
type TemporaryFlagImpl struct {
	TestingFlag
}

// TemporaryFlag returns a new instance of TemporaryFlag for testing.
func TemporaryFlag() *TemporaryFlagImpl {
	return &TemporaryFlagImpl{
		TestingFlag: TestingFlag{
			Name: TemporaryFlagName,
			// empty value -> it is a boolean flag
		},
	}
}
