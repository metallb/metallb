// Copyright (C) 2016 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package table

import (
	"fmt"
	"net"
)

type IPPrefix struct {
	Prefix net.IP
	Length uint8
}

func (p *IPPrefix) String() string {
	return fmt.Sprintf("%s/%d", p.Prefix, p.Length)
}

type ROA struct {
	Family int
	Prefix *IPPrefix
	MaxLen uint8
	AS     uint32
	Src    string
}

func NewROA(family int, prefixByte []byte, prefixLen uint8, maxLen uint8, as uint32, src string) *ROA {
	p := make([]byte, len(prefixByte))
	copy(p, prefixByte)
	return &ROA{
		Family: family,
		Prefix: &IPPrefix{
			Prefix: p,
			Length: prefixLen,
		},
		MaxLen: maxLen,
		AS:     as,
		Src:    src,
	}
}

func (r *ROA) Equal(roa *ROA) bool {
	if r.MaxLen == roa.MaxLen && r.Src == roa.Src && r.AS == roa.AS {
		return true
	}
	return false
}
