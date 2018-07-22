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

package server

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/osrg/gobgp/table"
	"github.com/osrg/gobgp/zebra"
)

func Test_newPathFromIPRouteMessage(t *testing.T) {
	assert := assert.New(t)

	// IPv4 Route Add
	m := &zebra.Message{}
	h := &zebra.Header{
		Len:     zebra.HeaderSize(2),
		Marker:  zebra.HEADER_MARKER,
		Version: 2,
		Command: zebra.IPV4_ROUTE_ADD,
	}
	b := &zebra.IPRouteBody{
		Type:         zebra.ROUTE_TYPE(zebra.ROUTE_STATIC),
		Flags:        zebra.FLAG(zebra.FLAG_SELECTED),
		Message:      zebra.MESSAGE_NEXTHOP | zebra.MESSAGE_DISTANCE | zebra.MESSAGE_METRIC | zebra.MESSAGE_MTU,
		SAFI:         zebra.SAFI(zebra.SAFI_UNICAST),
		Prefix:       net.ParseIP("192.168.100.0"),
		PrefixLength: uint8(24),
		Nexthops:     []net.IP{net.ParseIP("0.0.0.0")},
		Ifindexs:     []uint32{1},
		Distance:     uint8(0),
		Metric:       uint32(100),
		Mtu:          uint32(0),
		Api:          zebra.API_TYPE(zebra.IPV4_ROUTE_ADD),
	}
	m.Header = *h
	m.Body = b

	path := newPathFromIPRouteMessage(m)
	pp := table.NewPath(nil, path.GetNlri(), path.IsWithdraw, path.GetPathAttrs(), time.Now(), false)
	pp.SetIsFromExternal(path.IsFromExternal())
	assert.Equal("0.0.0.0", pp.GetNexthop().String())
	assert.Equal("192.168.100.0/24", pp.GetNlri().String())
	assert.True(pp.IsFromExternal())
	assert.False(pp.IsWithdraw)

	// IPv4 Route Delete
	h.Command = zebra.IPV4_ROUTE_DELETE
	b.Api = zebra.IPV4_ROUTE_DELETE
	m.Header = *h
	m.Body = b

	path = newPathFromIPRouteMessage(m)
	pp = table.NewPath(nil, path.GetNlri(), path.IsWithdraw, path.GetPathAttrs(), time.Now(), false)
	pp.SetIsFromExternal(path.IsFromExternal())
	assert.Equal("0.0.0.0", pp.GetNexthop().String())
	assert.Equal("192.168.100.0/24", pp.GetNlri().String())
	med, _ := pp.GetMed()
	assert.Equal(uint32(100), med)
	assert.True(pp.IsFromExternal())
	assert.True(pp.IsWithdraw)

	// IPv6 Route Add
	h.Command = zebra.IPV6_ROUTE_ADD
	b.Api = zebra.IPV6_ROUTE_ADD
	b.Prefix = net.ParseIP("2001:db8:0:f101::")
	b.PrefixLength = uint8(64)
	b.Nexthops = []net.IP{net.ParseIP("::")}
	m.Header = *h
	m.Body = b

	path = newPathFromIPRouteMessage(m)
	pp = table.NewPath(nil, path.GetNlri(), path.IsWithdraw, path.GetPathAttrs(), time.Now(), false)
	pp.SetIsFromExternal(path.IsFromExternal())
	assert.Equal("::", pp.GetNexthop().String())
	assert.Equal("2001:db8:0:f101::/64", pp.GetNlri().String())
	med, _ = pp.GetMed()
	assert.Equal(uint32(100), med)
	assert.True(pp.IsFromExternal())
	assert.False(pp.IsWithdraw)

	// IPv6 Route Delete
	h.Command = zebra.IPV6_ROUTE_DELETE
	b.Api = zebra.IPV6_ROUTE_DELETE
	m.Header = *h
	m.Body = b

	path = newPathFromIPRouteMessage(m)
	pp = table.NewPath(nil, path.GetNlri(), path.IsWithdraw, path.GetPathAttrs(), time.Now(), false)
	pp.SetIsFromExternal(path.IsFromExternal())
	assert.Equal("::", pp.GetNexthop().String())
	assert.Equal("2001:db8:0:f101::/64", pp.GetNlri().String())
	assert.True(pp.IsFromExternal())
	assert.True(pp.IsWithdraw)
}
