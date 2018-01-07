// Copyright (C) 2014 Nippon Telegraph and Telephone Corporation.
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
	"reflect"
	"testing"
	"time"

	"github.com/osrg/gobgp/packet/bgp"
	"github.com/stretchr/testify/assert"
)

// before:
//  as-path  : 65000, 4000, 400000, 300000, 40001
// expected result:
//  as-path  : 65000, 4000, 23456, 23456, 40001
//  as4-path : 65000, 4000, 400000, 300000, 40001
func TestAsPathAs2Trans1(t *testing.T) {
	as := []uint32{65000, 4000, 400000, 300000, 40001}
	params := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as)}
	aspath := bgp.NewPathAttributeAsPath(params)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs2ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 2)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS), 5)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[0], uint16(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[1], uint16(4000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[2], uint16(bgp.AS_TRANS))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[3], uint16(bgp.AS_TRANS))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[4], uint16(40001))
	assert.Equal(t, len(msg.PathAttributes[1].(*bgp.PathAttributeAs4Path).Value), 1)
	assert.Equal(t, len(msg.PathAttributes[1].(*bgp.PathAttributeAs4Path).Value[0].AS), 5)
	assert.Equal(t, msg.PathAttributes[1].(*bgp.PathAttributeAs4Path).Value[0].AS[0], uint32(65000))
	assert.Equal(t, msg.PathAttributes[1].(*bgp.PathAttributeAs4Path).Value[0].AS[1], uint32(4000))
	assert.Equal(t, msg.PathAttributes[1].(*bgp.PathAttributeAs4Path).Value[0].AS[2], uint32(400000))
	assert.Equal(t, msg.PathAttributes[1].(*bgp.PathAttributeAs4Path).Value[0].AS[3], uint32(300000))
	assert.Equal(t, msg.PathAttributes[1].(*bgp.PathAttributeAs4Path).Value[0].AS[4], uint32(40001))
}

// before:
//  as-path  : 65000, 4000, 40000, 30000, 40001
// expected result:
//  as-path  : 65000, 4000, 40000, 30000, 40001
func TestAsPathAs2Trans2(t *testing.T) {
	as := []uint32{65000, 4000, 40000, 30000, 40001}
	params := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as)}
	aspath := bgp.NewPathAttributeAsPath(params)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs2ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS), 5)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[0], uint16(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[1], uint16(4000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[2], uint16(40000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[3], uint16(30000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.AsPathParam).AS[4], uint16(40001))
}

// before:
//  as-path  : 65000, 4000, 23456, 23456, 40001
//  as4-path : 400000, 300000, 40001
// expected result:
//  as-path  : 65000, 4000, 400000, 300000, 40001
func TestAsPathAs4Trans1(t *testing.T) {
	as := []uint16{65000, 4000, bgp.AS_TRANS, bgp.AS_TRANS, 40001}
	params := []bgp.AsPathParamInterface{bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as)}
	aspath := bgp.NewPathAttributeAsPath(params)

	as4 := []uint32{400000, 300000, 40001}
	param4s := []*bgp.As4PathParam{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as4)}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS), 5)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[0], uint32(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[1], uint32(4000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[2], uint32(400000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[3], uint32(300000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[4], uint32(40001))
}

// before:
//  as-path  : 65000, 4000, {10, 20, 30}, 23456, 23456, 40001
//  as4-path : 400000, 300000, 40001
// expected result:
//  as-path  : 65000, 4000, {10, 20, 30}, 400000, 300000, 40001
func TestAsPathAs4Trans2(t *testing.T) {
	as1 := []uint16{65000, 4000}
	param1 := bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as1)
	as2 := []uint16{10, 20, 30}
	param2 := bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SET, as2)
	as3 := []uint16{bgp.AS_TRANS, bgp.AS_TRANS, 40001}
	param3 := bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as3)
	params := []bgp.AsPathParamInterface{param1, param2, param3}
	aspath := bgp.NewPathAttributeAsPath(params)

	as4 := []uint32{400000, 300000, 40001}
	param4s := []*bgp.As4PathParam{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as4)}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 3)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS), 2)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[0], uint32(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[1], uint32(4000))
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS), 3)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS[0], uint32(10))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS[1], uint32(20))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS[2], uint32(30))
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[2].(*bgp.As4PathParam).AS), 3)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[2].(*bgp.As4PathParam).AS[0], uint32(400000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[2].(*bgp.As4PathParam).AS[1], uint32(300000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[2].(*bgp.As4PathParam).AS[2], uint32(40001))
}

// before:
//  as-path  : 65000, 4000, {10, 20, 30}, 23456, 23456, 40001
//  as4-path : 3000, 400000, 300000, 40001
// expected result:
//  as-path  : 65000, 4000, 3000, 400000, 300000, 40001
func TestAsPathAs4Trans3(t *testing.T) {
	as1 := []uint16{65000, 4000}
	param1 := bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as1)
	as2 := []uint16{10, 20, 30}
	param2 := bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SET, as2)
	as3 := []uint16{bgp.AS_TRANS, bgp.AS_TRANS, 40001}
	param3 := bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as3)
	params := []bgp.AsPathParamInterface{param1, param2, param3}
	aspath := bgp.NewPathAttributeAsPath(params)

	as4 := []uint32{3000, 400000, 300000, 40001}
	param4s := []*bgp.As4PathParam{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as4)}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS), 6)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[0], uint32(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[1], uint32(4000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[2], uint32(3000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[3], uint32(400000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[4], uint32(300000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[5], uint32(40001))
}

// before:
//  as-path  : 65000, 4000, 23456, 23456, 40001
//  as4-path : 400000, 300000, 40001, {10, 20, 30}
// expected result:
//  as-path  : 65000, 400000, 300000, 40001, {10, 20, 30}
func TestAsPathAs4Trans4(t *testing.T) {
	as := []uint16{65000, 4000, bgp.AS_TRANS, bgp.AS_TRANS, 40001}
	params := []bgp.AsPathParamInterface{bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as)}
	aspath := bgp.NewPathAttributeAsPath(params)

	as4 := []uint32{400000, 300000, 40001}
	as4param1 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as4)
	as5 := []uint32{10, 20, 30}
	as4param2 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SET, as5)
	param4s := []*bgp.As4PathParam{as4param1, as4param2}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 2)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS), 4)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[0], uint32(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[1], uint32(400000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[2], uint32(300000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[3], uint32(40001))
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS), 3)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS[0], uint32(10))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS[1], uint32(20))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS[2], uint32(30))
}

// before:
//  as-path  : 65000, 4000, 23456, 23456, 40001
//  as4-path : {10, 20, 30} 400000, 300000, 40001
// expected result:
//  as-path  : 65000, {10, 20, 30}, 400000, 300000, 40001
func TestAsPathAs4Trans5(t *testing.T) {
	as := []uint16{65000, 4000, bgp.AS_TRANS, bgp.AS_TRANS, 40001}
	params := []bgp.AsPathParamInterface{bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as)}
	aspath := bgp.NewPathAttributeAsPath(params)

	as4 := []uint32{400000, 300000, 40001}
	as4param1 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as4)
	as5 := []uint32{10, 20, 30}
	as4param2 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SET, as5)
	param4s := []*bgp.As4PathParam{as4param2, as4param1}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 3)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS), 1)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[0], uint32(65000))
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS), 3)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS[0], uint32(10))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS[1], uint32(20))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[1].(*bgp.As4PathParam).AS[2], uint32(30))
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[2].(*bgp.As4PathParam).AS), 3)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[2].(*bgp.As4PathParam).AS[0], uint32(400000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[2].(*bgp.As4PathParam).AS[1], uint32(300000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[2].(*bgp.As4PathParam).AS[2], uint32(40001))
}

// before:
//  as-path  : 65000, 4000, 23456, 23456, 40001
//  as4-path : 100000, 65000, 4000, 400000, 300000, 40001
// expected result:
//  as-path  : 65000, 4000, 23456, 23456, 40001
func TestAsPathAs4TransInvalid1(t *testing.T) {
	as := []uint16{65000, 4000, bgp.AS_TRANS, bgp.AS_TRANS, 40001}
	params := []bgp.AsPathParamInterface{bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as)}
	aspath := bgp.NewPathAttributeAsPath(params)

	as4 := []uint32{100000, 65000, 4000, 400000, 300000, 40001}
	as4param1 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as4)
	param4s := []*bgp.As4PathParam{as4param1}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS), 5)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[0], uint32(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[1], uint32(4000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[2], uint32(bgp.AS_TRANS))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[3], uint32(bgp.AS_TRANS))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[4], uint32(40001))
}

// before:
//  as-path  : 65000, 4000, 23456, 23456, 40001
//  as4-path : 300000, 40001
// expected result:
//  as-path  : 65000, 4000, 23456, 300000, 40001
func TestAsPathAs4TransInvalid2(t *testing.T) {
	as := []uint16{65000, 4000, bgp.AS_TRANS, bgp.AS_TRANS, 40001}
	params := []bgp.AsPathParamInterface{bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as)}
	aspath := bgp.NewPathAttributeAsPath(params)

	as4 := []uint32{300000, 40001}
	as4param1 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as4)
	param4s := []*bgp.As4PathParam{as4param1}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS), 5)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[0], uint32(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[1], uint32(4000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[2], uint32(bgp.AS_TRANS))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[3], uint32(300000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[4], uint32(40001))
}

// before:
//  as-path  : 65000, 4000, 23456, 23456, 40001
//  as4-path : nil
// expected result:
//  as-path  : 65000, 4000, 23456, 23456, 40001
func TestAsPathAs4TransInvalid3(t *testing.T) {
	as := []uint16{65000, 4000, bgp.AS_TRANS, bgp.AS_TRANS, 40001}
	params := []bgp.AsPathParamInterface{bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as)}
	aspath := bgp.NewPathAttributeAsPath(params)

	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS), 5)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[0], uint32(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[1], uint32(4000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[2], uint32(bgp.AS_TRANS))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[3], uint32(bgp.AS_TRANS))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[4], uint32(40001))
}

// before:
//  as-path  : 65000, 4000, 23456, 23456, 40001
//  as4-path : empty
// expected result:
//  as-path  : 65000, 4000, 23456, 23456, 40001
func TestAsPathAs4TransInvalid4(t *testing.T) {
	as := []uint16{65000, 4000, bgp.AS_TRANS, bgp.AS_TRANS, 40001}
	params := []bgp.AsPathParamInterface{bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as)}
	aspath := bgp.NewPathAttributeAsPath(params)

	as4 := []uint32{}
	as4param1 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as4)
	param4s := []*bgp.As4PathParam{as4param1}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	assert.Equal(t, len(msg.PathAttributes), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value), 1)
	assert.Equal(t, len(msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS), 5)
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[0], uint32(65000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[1], uint32(4000))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[2], uint32(bgp.AS_TRANS))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[3], uint32(bgp.AS_TRANS))
	assert.Equal(t, msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value[0].(*bgp.As4PathParam).AS[4], uint32(40001))
}

func TestASPathAs4TransMultipleParams(t *testing.T) {
	as1 := []uint16{17676, 2914, 174, 50607}
	as2 := []uint16{bgp.AS_TRANS, bgp.AS_TRANS}
	params := []bgp.AsPathParamInterface{bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as1), bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as2)}
	aspath := bgp.NewPathAttributeAsPath(params)

	as41 := []uint32{2914, 174, 50607}
	as42 := []uint32{198035, 198035}
	as4param1 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as41)
	as4param2 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as42)
	param4s := []*bgp.As4PathParam{as4param1, as4param2}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	for _, param := range msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value {
		p := param.(*bgp.As4PathParam)
		assert.Equal(t, p.Num, uint8(len(p.AS)))
	}
}

func TestASPathAs4TransMultipleLargeParams(t *testing.T) {
	as1 := make([]uint16, 0, 255)
	for i := 0; i < 255-5; i++ {
		as1 = append(as1, uint16(i+1))
	}
	as1 = append(as1, []uint16{17676, 2914, 174, 50607}...)
	as2 := []uint16{bgp.AS_TRANS, bgp.AS_TRANS}
	params := []bgp.AsPathParamInterface{bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as1), bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as2)}
	aspath := bgp.NewPathAttributeAsPath(params)

	as41 := []uint32{2914, 174, 50607}
	as42 := []uint32{198035, 198035}
	as4param1 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as41)
	as4param2 := bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, as42)
	param4s := []*bgp.As4PathParam{as4param1, as4param2}
	as4path := bgp.NewPathAttributeAs4Path(param4s)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{aspath, as4path}, nil).Body.(*bgp.BGPUpdate)
	UpdatePathAttrs4ByteAs(msg)
	for _, param := range msg.PathAttributes[0].(*bgp.PathAttributeAsPath).Value {
		p := param.(*bgp.As4PathParam)
		assert.Equal(t, p.Num, uint8(len(p.AS)))
	}
}

func TestAggregator4BytesASes(t *testing.T) {
	getAggr := func(msg *bgp.BGPUpdate) *bgp.PathAttributeAggregator {
		for _, attr := range msg.PathAttributes {
			switch attr.(type) {
			case *bgp.PathAttributeAggregator:
				return attr.(*bgp.PathAttributeAggregator)
			}
		}
		return nil
	}

	getAggr4 := func(msg *bgp.BGPUpdate) *bgp.PathAttributeAs4Aggregator {
		for _, attr := range msg.PathAttributes {
			switch attr.(type) {
			case *bgp.PathAttributeAs4Aggregator:
				return attr.(*bgp.PathAttributeAs4Aggregator)
			}
		}
		return nil
	}

	addr := "192.168.0.1"
	as4 := uint32(100000)
	as := uint32(1000)
	msg := bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{bgp.NewPathAttributeAggregator(as4, addr)}, nil).Body.(*bgp.BGPUpdate)

	// 4byte capable to 4byte capable for 4 bytes AS
	assert.Equal(t, UpdatePathAggregator4ByteAs(msg), nil)
	assert.Equal(t, getAggr(msg).Value.AS, as4)
	assert.Equal(t, getAggr(msg).Value.Address.String(), addr)

	// 4byte capable to 2byte capable for 4 bytes AS
	UpdatePathAggregator2ByteAs(msg)
	assert.Equal(t, getAggr(msg).Value.AS, uint32(bgp.AS_TRANS))
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint16)
	assert.Equal(t, getAggr4(msg).Value.AS, as4)
	assert.Equal(t, getAggr4(msg).Value.Address.String(), addr)

	msg = bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{bgp.NewPathAttributeAggregator(uint16(bgp.AS_TRANS), addr), bgp.NewPathAttributeAs4Aggregator(as4, addr)}, nil).Body.(*bgp.BGPUpdate)
	assert.Equal(t, getAggr(msg).Value.AS, uint32(bgp.AS_TRANS))
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint16)

	// non 4byte capable to 4byte capable for 4 bytes AS
	assert.Equal(t, UpdatePathAggregator4ByteAs(msg), nil)
	assert.Equal(t, getAggr(msg).Value.AS, as4)
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint32)
	assert.Equal(t, getAggr(msg).Value.Address.String(), addr)
	assert.Equal(t, getAggr4(msg), (*bgp.PathAttributeAs4Aggregator)(nil))

	// non 4byte capable to non 4byte capable for 4 bytes AS
	UpdatePathAggregator2ByteAs(msg)
	assert.Equal(t, getAggr(msg).Value.AS, uint32(bgp.AS_TRANS))
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint16)
	assert.Equal(t, getAggr4(msg).Value.AS, as4)
	assert.Equal(t, getAggr4(msg).Value.Address.String(), addr)

	msg = bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{bgp.NewPathAttributeAggregator(uint32(as), addr)}, nil).Body.(*bgp.BGPUpdate)
	// 4byte capable to 4byte capable for 2 bytes AS
	assert.Equal(t, getAggr(msg).Value.AS, as)
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint32)
	assert.Equal(t, UpdatePathAggregator4ByteAs(msg), nil)
	assert.Equal(t, getAggr(msg).Value.AS, as)
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint32)

	// 4byte capable to non 4byte capable for 2 bytes AS
	UpdatePathAggregator2ByteAs(msg)
	assert.Equal(t, getAggr4(msg), (*bgp.PathAttributeAs4Aggregator)(nil))
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint16)
	assert.Equal(t, getAggr(msg).Value.AS, as)

	msg = bgp.NewBGPUpdateMessage(nil, []bgp.PathAttributeInterface{bgp.NewPathAttributeAggregator(uint16(as), addr)}, nil).Body.(*bgp.BGPUpdate)
	// non 4byte capable to 4byte capable for 2 bytes AS
	assert.Equal(t, getAggr(msg).Value.AS, as)
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint16)
	assert.Equal(t, UpdatePathAggregator4ByteAs(msg), nil)

	assert.Equal(t, getAggr(msg).Value.AS, as)
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint32)

	// non 4byte capable to non 4byte capable for 2 bytes AS
	UpdatePathAggregator2ByteAs(msg)
	assert.Equal(t, getAggr(msg).Value.AS, as)
	assert.Equal(t, getAggr(msg).Value.Askind, reflect.Uint16)
	assert.Equal(t, getAggr4(msg), (*bgp.PathAttributeAs4Aggregator)(nil))
}

func TestBMP(t *testing.T) {
	aspath1 := []bgp.AsPathParamInterface{
		bgp.NewAs4PathParam(2, []uint32{1000000}),
		bgp.NewAs4PathParam(1, []uint32{1000001, 1002}),
		bgp.NewAs4PathParam(2, []uint32{1003, 100004}),
	}
	mp_nlri := []bgp.AddrPrefixInterface{bgp.NewIPv6AddrPrefix(100,
		"fe80:1234:1234:5667:8967:af12:8912:1023")}

	p := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(3),
		bgp.NewPathAttributeAsPath(aspath1),
		bgp.NewPathAttributeMpUnreachNLRI(mp_nlri),
	}
	w := []*bgp.IPAddrPrefix{}
	n := []*bgp.IPAddrPrefix{}

	msg := bgp.NewBGPUpdateMessage(w, p, n)
	pList := ProcessMessage(msg, peerR1(), time.Now())
	CreateUpdateMsgFromPaths(pList)
}

func unreachIndex(msgs []*bgp.BGPMessage) int {
	for i, _ := range msgs {
		for _, a := range msgs[i].Body.(*bgp.BGPUpdate).PathAttributes {
			if a.GetType() == bgp.BGP_ATTR_TYPE_MP_UNREACH_NLRI {
				return i
			}
		}
	}
	// should not be here
	return -1
}

func TestMixedMPReachMPUnreach(t *testing.T) {
	aspath1 := []bgp.AsPathParamInterface{
		bgp.NewAs4PathParam(2, []uint32{100}),
	}
	nlri1 := []bgp.AddrPrefixInterface{bgp.NewIPv6AddrPrefix(32, "2222::")}
	nlri2 := []bgp.AddrPrefixInterface{bgp.NewIPv6AddrPrefix(32, "1111::")}

	p := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(0),
		bgp.NewPathAttributeAsPath(aspath1),
		bgp.NewPathAttributeMpReachNLRI("1::1", nlri1),
		bgp.NewPathAttributeMpUnreachNLRI(nlri2),
	}
	msg := bgp.NewBGPUpdateMessage(nil, p, nil)
	pList := ProcessMessage(msg, peerR1(), time.Now())
	assert.Equal(t, len(pList), 2)
	assert.Equal(t, pList[0].IsWithdraw, false)
	assert.Equal(t, pList[1].IsWithdraw, true)
	msgs := CreateUpdateMsgFromPaths(pList)
	assert.Equal(t, len(msgs), 2)

	uIndex := unreachIndex(msgs)
	rIndex := 0
	if uIndex == 0 {
		rIndex = 1
	}
	assert.Equal(t, len(msgs[uIndex].Body.(*bgp.BGPUpdate).PathAttributes), 1)
	assert.Equal(t, len(msgs[rIndex].Body.(*bgp.BGPUpdate).PathAttributes), 3)
}

func TestMixedNLRIAndMPUnreach(t *testing.T) {
	aspath1 := []bgp.AsPathParamInterface{
		bgp.NewAs4PathParam(2, []uint32{100}),
	}
	nlri1 := []*bgp.IPAddrPrefix{bgp.NewIPAddrPrefix(24, "10.0.0.0")}
	nlri2 := []bgp.AddrPrefixInterface{bgp.NewIPv6AddrPrefix(32, "1111::")}

	p := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(0),
		bgp.NewPathAttributeAsPath(aspath1),
		bgp.NewPathAttributeNextHop("1.1.1.1"),
		bgp.NewPathAttributeMpUnreachNLRI(nlri2),
	}
	msg := bgp.NewBGPUpdateMessage(nil, p, nlri1)
	pList := ProcessMessage(msg, peerR1(), time.Now())

	assert.Equal(t, len(pList), 2)
	assert.Equal(t, pList[0].IsWithdraw, false)
	assert.Equal(t, pList[1].IsWithdraw, true)
	msgs := CreateUpdateMsgFromPaths(pList)
	assert.Equal(t, len(msgs), 2)

	uIndex := unreachIndex(msgs)
	rIndex := 0
	if uIndex == 0 {
		rIndex = 1
	}
	assert.Equal(t, len(msgs[uIndex].Body.(*bgp.BGPUpdate).PathAttributes), 1)
	assert.Equal(t, len(msgs[rIndex].Body.(*bgp.BGPUpdate).PathAttributes), 3)
}

func TestMergeV4NLRIs(t *testing.T) {
	aspath1 := []bgp.AsPathParamInterface{
		bgp.NewAs4PathParam(2, []uint32{100}),
	}
	attrs := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(0),
		bgp.NewPathAttributeAsPath(aspath1),
		bgp.NewPathAttributeNextHop("1.1.1.1"),
	}

	nr := 1024
	paths := make([]*Path, 0, nr)
	addrs := make([]string, 0, nr)
	for i := 0; i < nr; i++ {
		addrs = append(addrs, fmt.Sprintf("1.1.%d.%d", i>>8&0xff, i&0xff))
		nlri := []*bgp.IPAddrPrefix{bgp.NewIPAddrPrefix(32, addrs[i])}
		msg := bgp.NewBGPUpdateMessage(nil, attrs, nlri)
		paths = append(paths, ProcessMessage(msg, peerR1(), time.Now())...)
	}
	msgs := CreateUpdateMsgFromPaths(paths)
	assert.Equal(t, len(msgs), 2)

	l := make([]*bgp.IPAddrPrefix, 0, nr)
	for _, msg := range msgs {
		u := msg.Body.(*bgp.BGPUpdate)
		assert.Equal(t, len(u.PathAttributes), 3)
		for _, n := range u.NLRI {
			l = append(l, n)
		}
	}

	assert.Equal(t, len(l), nr)
	for i, addr := range addrs {
		assert.Equal(t, addr, l[i].Prefix.String())
	}
	for _, msg := range msgs {
		d, _ := msg.Serialize()
		assert.True(t, len(d) < bgp.BGP_MAX_MESSAGE_LENGTH)
	}
}

func TestNotMergeV4NLRIs(t *testing.T) {
	paths := make([]*Path, 0, 2)

	aspath1 := []bgp.AsPathParamInterface{
		bgp.NewAs4PathParam(2, []uint32{100}),
	}
	attrs1 := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(0),
		bgp.NewPathAttributeAsPath(aspath1),
		bgp.NewPathAttributeNextHop("1.1.1.1"),
	}
	nlri1 := []*bgp.IPAddrPrefix{bgp.NewIPAddrPrefix(32, "1.1.1.1")}
	paths = append(paths, ProcessMessage(bgp.NewBGPUpdateMessage(nil, attrs1, nlri1), peerR1(), time.Now())...)

	attrs2 := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(0),
		bgp.NewPathAttributeAsPath(aspath1),
		bgp.NewPathAttributeNextHop("2.2.2.2"),
	}
	nlri2 := []*bgp.IPAddrPrefix{bgp.NewIPAddrPrefix(32, "2.2.2.2")}
	paths = append(paths, ProcessMessage(bgp.NewBGPUpdateMessage(nil, attrs2, nlri2), peerR1(), time.Now())...)

	assert.NotEmpty(t, paths[0].GetHash(), paths[1].GetHash())

	msgs := CreateUpdateMsgFromPaths(paths)
	assert.Equal(t, len(msgs), 2)

	paths[1].SetHash(paths[0].GetHash())
	msgs = CreateUpdateMsgFromPaths(paths)
	assert.Equal(t, len(msgs), 2)
}

func TestMergeV4Withdraw(t *testing.T) {
	nr := 1024
	paths := make([]*Path, 0, nr)
	addrs := make([]string, 0, nr)
	for i := 0; i < nr; i++ {
		addrs = append(addrs, fmt.Sprintf("1.1.%d.%d", i>>8&0xff, i&0xff))
		nlri := []*bgp.IPAddrPrefix{bgp.NewIPAddrPrefix(32, addrs[i])}
		// use different attribute for each nlri
		aspath1 := []bgp.AsPathParamInterface{
			bgp.NewAs4PathParam(2, []uint32{uint32(i)}),
		}
		attrs := []bgp.PathAttributeInterface{
			bgp.NewPathAttributeOrigin(0),
			bgp.NewPathAttributeAsPath(aspath1),
			bgp.NewPathAttributeNextHop("1.1.1.1"),
		}
		msg := bgp.NewBGPUpdateMessage(nlri, attrs, nil)
		paths = append(paths, ProcessMessage(msg, peerR1(), time.Now())...)
	}
	msgs := CreateUpdateMsgFromPaths(paths)
	assert.Equal(t, len(msgs), 2)

	l := make([]*bgp.IPAddrPrefix, 0, nr)
	for _, msg := range msgs {
		u := msg.Body.(*bgp.BGPUpdate)
		assert.Equal(t, len(u.PathAttributes), 0)
		for _, n := range u.WithdrawnRoutes {
			l = append(l, n)
		}
	}
	assert.Equal(t, len(l), nr)
	for i, addr := range addrs {
		assert.Equal(t, addr, l[i].Prefix.String())
	}

	for _, msg := range msgs {
		d, _ := msg.Serialize()
		assert.True(t, len(d) < bgp.BGP_MAX_MESSAGE_LENGTH)
	}
}
