// Copyright (C) 2018 Nippon Telegraph and Telephone Corporation.
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

// +build race

package bgp

import (
	"testing"
	"time"
)

// Test_RaceCondition detects data races when serialization.
// Currently tests only attributes contained in UPDATE message.
func Test_RaceCondition(t *testing.T) {
	m := NewTestBGPUpdateMessage()
	updateBody := m.Body.(*BGPUpdate)

	go func(body *BGPUpdate) {
		for _, v := range body.WithdrawnRoutes {
			v.Serialize()
		}
		for _, v := range body.PathAttributes {
			v.Serialize()
		}
		for _, v := range body.NLRI {
			v.Serialize()
		}
	}(updateBody)

	time.Sleep(time.Second)

	updateBody.Serialize()
}
