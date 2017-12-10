// Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
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

package server

import (
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	radix "github.com/armon/go-radix"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
)

func strToASParam(str string) *bgp.PathAttributeAsPath {
	toList := func(asstr, sep string) []uint32 {
		as := make([]uint32, 0)
		l := strings.Split(asstr, sep)
		for _, s := range l {
			v, _ := strconv.ParseUint(s, 10, 32)
			as = append(as, uint32(v))
		}
		return as
	}
	var atype uint8
	var as []uint32
	if strings.HasPrefix(str, "{") {
		atype = bgp.BGP_ASPATH_ATTR_TYPE_SET
		as = toList(str[1:len(str)-1], ",")
	} else if strings.HasPrefix(str, "(") {
		atype = bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SET
		as = toList(str[1:len(str)-1], " ")
	} else {
		atype = bgp.BGP_ASPATH_ATTR_TYPE_SEQ
		as = toList(str, " ")
	}

	return bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(atype, as)})
}

func validateOne(tree *radix.Tree, cidr, aspathStr string) config.RpkiValidationResultType {
	r := ValidatePath(65500, tree, cidr, strToASParam(aspathStr))
	return r.Status
}

func TestValidate0(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("192.168.0.0").To4(), 24, 32, 100, ""))
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("192.168.0.0").To4(), 24, 24, 200, ""))

	var r config.RpkiValidationResultType

	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "192.168.0.0/24", "100")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)

	r = validateOne(tree, "192.168.0.0/24", "100 200")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)

	r = validateOne(tree, "192.168.0.0/24", "300")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_INVALID)

	r = validateOne(tree, "192.168.0.0/25", "100")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)

	r = validateOne(tree, "192.168.0.0/25", "200")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_INVALID)

	r = validateOne(tree, "192.168.0.0/25", "300")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_INVALID)
}

func TestValidate1(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 16, 65000, ""))

	var r config.RpkiValidationResultType

	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/16", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)

	r = validateOne(tree, "10.0.0.0/16", "65001")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_INVALID)
}

func TestValidate2(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)

	var r config.RpkiValidationResultType

	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/16", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND)

	r = validateOne(tree, "10.0.0.0/16", "65001")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND)
}

func TestValidate3(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 16, 65000, ""))

	var r config.RpkiValidationResultType

	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/8", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND)

	r = validateOne(tree, "10.0.0.0/17", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_INVALID)

	manager, _ = NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 24, 65000, ""))

	tree = manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/17", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)
}

func TestValidate4(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 16, 65000, ""))
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 16, 65001, ""))

	var r config.RpkiValidationResultType
	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/16", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)

	r = validateOne(tree, "10.0.0.0/16", "65001")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)
}

func TestValidate5(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 17, 17, 65000, ""))
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.128.0").To4(), 17, 17, 65000, ""))

	var r config.RpkiValidationResultType
	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/16", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND)
}

func TestValidate6(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 8, 32, 0, ""))

	var r config.RpkiValidationResultType
	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/7", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND)

	r = validateOne(tree, "10.0.0.0/8", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_INVALID)

	r = validateOne(tree, "10.0.0.0/24", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_INVALID)
}

func TestValidate7(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 24, 65000, ""))

	var r config.RpkiValidationResultType
	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/24", "{65000}")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND)

	r = validateOne(tree, "10.0.0.0/24", "{65001}")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND)

	r = validateOne(tree, "10.0.0.0/24", "{65000,65001}")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND)
}

func TestValidate8(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 24, 0, ""))
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 24, 65000, ""))

	var r config.RpkiValidationResultType
	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/24", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)

	r = validateOne(tree, "10.0.0.0/24", "65001")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_INVALID)
}

func TestValidate9(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 24, 24, 65000, ""))
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 24, 65001, ""))

	var r config.RpkiValidationResultType
	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/24", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)

	r = validateOne(tree, "10.0.0.0/24", "65001")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)
}

func TestValidate10(t *testing.T) {
	assert := assert.New(t)

	manager, _ := NewROAManager(0)
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 24, 24, 0, ""))
	manager.addROA(table.NewROA(bgp.AFI_IP, net.ParseIP("10.0.0.0").To4(), 16, 24, 65001, ""))

	var r config.RpkiValidationResultType
	tree := manager.Roas[bgp.RF_IPv4_UC]
	r = validateOne(tree, "10.0.0.0/24", "65000")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_INVALID)

	r = validateOne(tree, "10.0.0.0/24", "65001")
	assert.Equal(r, config.RPKI_VALIDATION_RESULT_TYPE_VALID)
}
