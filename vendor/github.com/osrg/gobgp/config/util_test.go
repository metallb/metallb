// Copyright (C) 2017 Nippon Telegraph and Telephone Corporation.
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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectConfigFileType(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("toml", detectConfigFileType("bgpd.conf", "toml"))
	assert.Equal("toml", detectConfigFileType("bgpd.toml", "xxx"))
	assert.Equal("yaml", detectConfigFileType("bgpd.yaml", "xxx"))
	assert.Equal("yaml", detectConfigFileType("bgpd.yml", "xxx"))
	assert.Equal("json", detectConfigFileType("bgpd.json", "xxx"))
}
