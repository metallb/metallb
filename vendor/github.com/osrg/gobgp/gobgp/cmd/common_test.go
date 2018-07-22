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

package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ExtractReserved(t *testing.T) {
	assert := assert.New(t)
	args := strings.Split("10 rt 100:100 med 10 nexthop 10.0.0.1 aigp metric 10 local-pref 100", " ")
	keys := map[string]int{
		"rt":         PARAM_LIST,
		"med":        PARAM_SINGLE,
		"nexthop":    PARAM_SINGLE,
		"aigp":       PARAM_LIST,
		"local-pref": PARAM_SINGLE}
	m, _ := extractReserved(args, keys)
	assert.True(len(m["rt"]) == 1)
	assert.True(len(m["med"]) == 1)
	assert.True(len(m["nexthop"]) == 1)
	assert.True(len(m["aigp"]) == 2)
	assert.True(len(m["local-pref"]) == 1)
}
