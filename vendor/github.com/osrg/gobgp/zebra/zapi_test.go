// Copyright (C) 2014, 2015 Nippon Telegraph and Telephone Corporation.
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

package zebra

import (
	"encoding/binary"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func Test_Header(t *testing.T) {
	assert := assert.New(t)

	//DecodeFromBytes
	buf := make([]byte, 6)
	binary.BigEndian.PutUint16(buf[0:], 10)
	buf[2] = HEADER_MARKER
	buf[3] = 2
	binary.BigEndian.PutUint16(buf[4:], uint16(IPV4_ROUTE_ADD))
	h := &Header{}
	err := h.DecodeFromBytes(buf)
	assert.Equal(nil, err)

	//Serialize
	buf, err = h.Serialize()
	assert.Equal(nil, err)
	h2 := &Header{}
	err = h2.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal(h, h2)

	// header_size mismatch
	buf = make([]byte, HeaderSize(2)-1)
	binary.BigEndian.PutUint16(buf[0:], 10)
	buf[2] = 0xff
	buf[3] = 0x02
	h3 := &Header{}
	err = h3.DecodeFromBytes(buf)
	assert.NotEqual(nil, err)
}

func Test_InterfaceUpdateBody(t *testing.T) {
	assert := assert.New(t)

	//DecodeFromBytes
	buf := make([]byte, INTERFACE_NAMSIZ+49)
	pos := INTERFACE_NAMSIZ
	binary.BigEndian.PutUint32(buf[pos:], 1)
	pos += 4
	buf[pos] = byte(INTERFACE_ACTIVE)
	pos += 1
	binary.BigEndian.PutUint64(buf[pos:], 1)
	pos += 8 // flags
	binary.BigEndian.PutUint32(buf[pos:], 1)
	pos += 4 // metric
	binary.BigEndian.PutUint32(buf[pos:], 1500)
	pos += 4 // MTU
	binary.BigEndian.PutUint32(buf[pos:], 1500)
	pos += 4 // MTU6
	binary.BigEndian.PutUint32(buf[pos:], 200)
	pos += 4 // bandwidth
	binary.BigEndian.PutUint32(buf[pos:], 6)
	pos += 4 // hwaddr_len
	mac, _ := net.ParseMAC("01:23:45:67:89:ab")
	copy(buf[pos:pos+6], []byte(mac))
	pos += 4
	b := &InterfaceUpdateBody{}
	err := b.DecodeFromBytes(buf, 2)
	assert.Equal(nil, err)
	assert.Equal("01:23:45:67:89:ab", b.HardwareAddr.String())

	buf = make([]byte, INTERFACE_NAMSIZ+28)
	b = &InterfaceUpdateBody{}
	err = b.DecodeFromBytes(buf, 2)
	assert.NotEqual(nil, err)
}

func Test_InterfaceAddressUpdateBody(t *testing.T) {
	assert := assert.New(t)

	//DecodeFromBytes
	buf := make([]byte, 15)
	pos := 0
	binary.BigEndian.PutUint32(buf[pos:], 0) // index
	pos += 4
	buf[pos] = 0x01 // flags
	pos += 1
	buf[pos] = 0x2 // family
	pos += 1
	ip := net.ParseIP("192.168.100.1").To4() // prefix
	copy(buf[pos:pos+4], []byte(ip))
	pos += 4
	buf[pos] = byte(24) // prefix len
	pos += 1
	dst := net.ParseIP("192.168.100.255").To4() // destination
	copy(buf[pos:pos+4], []byte(dst))

	b := &InterfaceAddressUpdateBody{}
	err := b.DecodeFromBytes(buf, 2)
	require.NoError(t, err)

	assert.Equal(uint32(0), b.Index)
	assert.Equal(INTERFACE_ADDRESS_FLAG(1), b.Flags)
	assert.Equal("192.168.100.1", b.Prefix.String())
	assert.Equal(uint8(24), b.Length)
	assert.Equal("192.168.100.255", b.Destination.String())

	// af invalid
	buf[5] = 0x4
	pos += 1
	b = &InterfaceAddressUpdateBody{}
	err = b.DecodeFromBytes(buf, 2)
	assert.NotEqual(nil, err)
}

func Test_RouterIDUpdateBody(t *testing.T) {
	assert := assert.New(t)

	//DecodeFromBytes
	buf := make([]byte, 6)
	pos := 0
	buf[pos] = 0x2
	pos += 1
	ip := net.ParseIP("192.168.100.1").To4()
	copy(buf[pos:pos+4], []byte(ip))
	pos += 4
	buf[pos] = byte(32)

	b := &RouterIDUpdateBody{}
	err := b.DecodeFromBytes(buf, 2)
	assert.Equal(nil, err)
	assert.Equal("192.168.100.1", b.Prefix.String())
	assert.Equal(uint8(32), b.Length)

	// af invalid
	buf[0] = 0x4
	pos += 1
	b = &RouterIDUpdateBody{}
	err = b.DecodeFromBytes(buf, 2)
	assert.NotEqual(nil, err)
}

func Test_IPRouteBody_IPv4(t *testing.T) {
	assert := assert.New(t)

	//DecodeFromBytes IPV4_ROUTE
	buf := make([]byte, 26)
	buf[0] = byte(ROUTE_CONNECT)
	buf[1] = byte(FLAG_SELECTED)
	buf[2] = byte(MESSAGE_NEXTHOP | MESSAGE_DISTANCE | MESSAGE_METRIC | MESSAGE_MTU)
	buf[3] = 24
	ip := net.ParseIP("192.168.100.0").To4()
	copy(buf[4:7], []byte(ip))

	buf[7] = 1
	nexthop := net.ParseIP("0.0.0.0").To4()
	copy(buf[8:12], []byte(nexthop))

	buf[12] = 1
	binary.BigEndian.PutUint32(buf[13:], 1)
	buf[17] = 0 // distance
	binary.BigEndian.PutUint32(buf[18:], 1)
	binary.BigEndian.PutUint32(buf[22:], 1)
	r := &IPRouteBody{Api: IPV4_ROUTE_ADD}
	err := r.DecodeFromBytes(buf, 2)

	assert.Equal(nil, err)
	assert.Equal("192.168.100.0", r.Prefix.String())
	assert.Equal(uint8(0x18), r.PrefixLength)
	assert.Equal(MESSAGE_NEXTHOP|MESSAGE_DISTANCE|MESSAGE_METRIC|MESSAGE_MTU, r.Message)
	assert.Equal("0.0.0.0", r.Nexthops[0].String())
	assert.Equal(uint32(1), r.Ifindexs[0])
	assert.Equal(uint8(0), r.Distance)
	assert.Equal(uint32(1), r.Metric)
	assert.Equal(uint32(1), r.Mtu)

	//Serialize
	buf, err = r.Serialize(2)
	assert.Equal(nil, err)
	assert.Equal([]byte{0x2, 0x10, 0x1d}, buf[0:3])
	assert.Equal([]byte{0x0, 0x1}, buf[3:5])
	assert.Equal(byte(24), buf[5])
	ip = net.ParseIP("192.168.100.0").To4()
	assert.Equal([]byte(ip)[0:3], buf[6:9])
	assert.Equal(byte(NEXTHOP_IPV4), buf[10])
	assert.Equal(byte(NEXTHOP_IFINDEX), buf[15])
	assert.Equal(byte(0x0), buf[20])

	bi := make([]byte, 4)
	binary.BigEndian.PutUint32(bi, 1)
	assert.Equal(bi, buf[21:25])
	assert.Equal(bi, buf[25:])

	// length invalid
	buf = make([]byte, 18)
	buf[0] = byte(ROUTE_CONNECT)
	buf[1] = byte(FLAG_SELECTED)
	buf[2] = byte(MESSAGE_NEXTHOP | MESSAGE_DISTANCE | MESSAGE_METRIC)
	buf[3] = 24
	ip = net.ParseIP("192.168.100.0").To4()
	copy(buf[4:7], []byte(ip))
	buf[7] = 1
	nexthop = net.ParseIP("0.0.0.0").To4()
	copy(buf[8:12], []byte(nexthop))
	buf[12] = 1
	binary.BigEndian.PutUint32(buf[13:], 1)

	r = &IPRouteBody{Api: IPV4_ROUTE_ADD}
	err = r.DecodeFromBytes(buf, 2)
	assert.Equal("message length invalid", err.Error())

	// no nexthop
	buf = make([]byte, 12)
	buf[0] = byte(ROUTE_CONNECT)
	buf[1] = byte(FLAG_SELECTED)
	buf[2] = byte(MESSAGE_DISTANCE | MESSAGE_METRIC)
	buf[3] = 24
	ip = net.ParseIP("192.168.100.0").To4()
	copy(buf[4:7], []byte(ip))
	buf[7] = 1
	binary.BigEndian.PutUint32(buf[8:], 0)
	r = &IPRouteBody{Api: IPV4_ROUTE_ADD}
	err = r.DecodeFromBytes(buf, 2)
	assert.Equal(nil, err)

}

func Test_IPRouteBody_IPv6(t *testing.T) {
	assert := assert.New(t)

	//DecodeFromBytes IPV6_ROUTE
	buf := make([]byte, 43)
	buf[0] = byte(ROUTE_CONNECT)
	buf[1] = byte(FLAG_SELECTED)
	buf[2] = byte(MESSAGE_NEXTHOP | MESSAGE_DISTANCE | MESSAGE_METRIC | MESSAGE_MTU)
	buf[3] = 64
	ip := net.ParseIP("2001:db8:0:f101::").To16()
	copy(buf[4:12], []byte(ip))

	buf[12] = 1
	nexthop := net.ParseIP("::").To16()
	copy(buf[13:29], []byte(nexthop))
	// ifindex
	buf[29] = 1
	binary.BigEndian.PutUint32(buf[30:], 1)

	buf[34] = 0 // distance
	binary.BigEndian.PutUint32(buf[35:], 1)
	binary.BigEndian.PutUint32(buf[39:], 1)
	r := &IPRouteBody{Api: IPV6_ROUTE_ADD}
	err := r.DecodeFromBytes(buf, 2)

	assert.Equal(nil, err)
	assert.Equal("2001:db8:0:f101::", r.Prefix.String())
	assert.Equal(uint8(64), r.PrefixLength)
	assert.Equal(MESSAGE_NEXTHOP|MESSAGE_DISTANCE|MESSAGE_METRIC|MESSAGE_MTU, r.Message)
	assert.Equal("::", r.Nexthops[0].String())
	assert.Equal(uint32(1), r.Ifindexs[0])
	assert.Equal(uint8(0), r.Distance)
	assert.Equal(uint32(1), r.Metric)
	assert.Equal(uint32(1), r.Mtu)

	//Serialize
	buf, err = r.Serialize(2)
	assert.Equal(nil, err)
	assert.Equal([]byte{0x2, 0x10, 0x1d}, buf[0:3])
	assert.Equal([]byte{0x0, 0x1}, buf[3:5])
	assert.Equal(byte(64), buf[5])
	ip = net.ParseIP("2001:db8:0:f101::").To16()
	assert.Equal([]byte(ip)[0:8], buf[6:14])
	assert.Equal(byte(2), buf[14])
	assert.Equal(byte(NEXTHOP_IPV6), buf[15])
	ip = net.ParseIP("::").To16()
	assert.Equal([]byte(ip), buf[16:32])
	assert.Equal(byte(NEXTHOP_IFINDEX), buf[32])
	bi := make([]byte, 4)
	binary.BigEndian.PutUint32(bi, 1)
	assert.Equal(bi, buf[33:37])

	//distance
	assert.Equal(byte(0), buf[37])
	bi = make([]byte, 4)
	binary.BigEndian.PutUint32(bi, 1)
	assert.Equal(bi, buf[38:42])
	assert.Equal(bi, buf[42:])

	// length invalid
	buf = make([]byte, 50)
	buf[0] = byte(ROUTE_CONNECT)
	buf[1] = byte(FLAG_SELECTED)
	buf[2] = byte(MESSAGE_NEXTHOP | MESSAGE_DISTANCE | MESSAGE_METRIC)
	buf[3] = 24
	ip = net.ParseIP("2001:db8:0:f101::").To4()
	copy(buf[4:12], []byte(ip))
	buf[13] = 1
	nexthop = net.ParseIP("::").To16()
	copy(buf[14:30], []byte(nexthop))
	buf[31] = 1
	binary.BigEndian.PutUint32(buf[32:], 1)

	r = &IPRouteBody{Api: IPV6_ROUTE_ADD}
	err = r.DecodeFromBytes(buf, 2)
	assert.Equal("message length invalid", err.Error())

	// no nexthop
	buf = make([]byte, 11)
	buf[0] = byte(ROUTE_CONNECT)
	buf[1] = byte(FLAG_SELECTED)
	buf[2] = byte(MESSAGE_DISTANCE | MESSAGE_METRIC)
	buf[3] = 16
	ip = net.ParseIP("2501::").To16()
	copy(buf[4:6], []byte(ip))
	buf[6] = 1
	binary.BigEndian.PutUint32(buf[7:], 0)
	r = &IPRouteBody{Api: IPV6_ROUTE_ADD}
	err = r.DecodeFromBytes(buf, 2)
	assert.Equal(nil, err)
}

func Test_NexthopLookupBody(t *testing.T) {
	assert := assert.New(t)

	//ipv4
	//DecodeFromBytes
	pos := 0
	buf := make([]byte, 18)
	ip := net.ParseIP("192.168.50.0").To4()
	copy(buf[0:4], []byte(ip))
	pos += 4
	binary.BigEndian.PutUint32(buf[pos:], 10)
	pos += 4
	buf[pos] = byte(1)
	pos += 1
	buf[pos] = byte(4)
	pos += 1
	ip = net.ParseIP("172.16.1.101").To4()
	copy(buf[pos:pos+4], []byte(ip))
	pos += 4
	binary.BigEndian.PutUint32(buf[pos:], 3)

	b := &NexthopLookupBody{Api: IPV4_NEXTHOP_LOOKUP}
	err := b.DecodeFromBytes(buf, 2)
	assert.Equal(nil, err)
	assert.Equal("192.168.50.0", b.Addr.String())
	assert.Equal(uint32(10), b.Metric)
	assert.Equal(uint32(3), b.Nexthops[0].Ifindex)
	assert.Equal(NEXTHOP_FLAG(4), b.Nexthops[0].Type)
	assert.Equal("172.16.1.101", b.Nexthops[0].Addr.String())

	//Serialize
	buf, err = b.Serialize(2)
	ip = net.ParseIP("192.168.50.0").To4()
	assert.Equal(nil, err)
	assert.Equal([]byte(ip)[0:4], buf[0:4])

	// length invalid
	buf = make([]byte, 3)
	b = &NexthopLookupBody{Api: IPV4_NEXTHOP_LOOKUP}
	err = b.DecodeFromBytes(buf, 2)
	assert.NotEqual(nil, err)

	//ipv6
	//DecodeFromBytes
	pos = 0
	buf = make([]byte, 46)
	ip = net.ParseIP("2001:db8:0:f101::").To16()
	copy(buf[0:16], []byte(ip))
	pos += 16
	binary.BigEndian.PutUint32(buf[pos:], 10)
	pos += 4
	buf[pos] = byte(1)
	pos += 1
	buf[pos] = byte(4)
	pos += 1
	ip = net.ParseIP("2001:db8:0:1111::1").To16()
	copy(buf[pos:pos+16], []byte(ip))
	pos += 16
	binary.BigEndian.PutUint32(buf[pos:], 3)

	b = &NexthopLookupBody{Api: IPV6_NEXTHOP_LOOKUP}
	err = b.DecodeFromBytes(buf, 2)
	assert.Equal(nil, err)
	assert.Equal("2001:db8:0:f101::", b.Addr.String())
	assert.Equal(uint32(10), b.Metric)
	assert.Equal(uint32(3), b.Nexthops[0].Ifindex)
	assert.Equal(NEXTHOP_FLAG(4), b.Nexthops[0].Type)
	assert.Equal("2001:db8:0:1111::1", b.Nexthops[0].Addr.String())

	//Serialize
	buf, err = b.Serialize(2)
	ip = net.ParseIP("2001:db8:0:f101::").To16()
	assert.Equal(nil, err)
	assert.Equal([]byte(ip)[0:16], buf[0:16])

	// length invalid
	buf = make([]byte, 15)
	b = &NexthopLookupBody{Api: IPV6_NEXTHOP_LOOKUP}
	err = b.DecodeFromBytes(buf, 2)
	assert.NotEqual(nil, err)
}

func Test_ImportLookupBody(t *testing.T) {
	assert := assert.New(t)

	//DecodeFromBytes
	pos := 0
	buf := make([]byte, 18)
	ip := net.ParseIP("192.168.50.0").To4()
	copy(buf[0:4], []byte(ip))
	pos += 4
	binary.BigEndian.PutUint32(buf[pos:], 10)
	pos += 4
	buf[pos] = byte(1)
	pos += 1
	buf[pos] = byte(4)
	pos += 1
	ip = net.ParseIP("172.16.1.101").To4()
	copy(buf[pos:pos+4], []byte(ip))
	pos += 4
	binary.BigEndian.PutUint32(buf[pos:], 3)

	b := &ImportLookupBody{Api: IPV4_IMPORT_LOOKUP}
	err := b.DecodeFromBytes(buf, 2)
	assert.Equal(nil, err)
	assert.Equal("192.168.50.0", b.Addr.String())
	assert.Equal(uint32(10), b.Metric)
	assert.Equal(uint32(3), b.Nexthops[0].Ifindex)
	assert.Equal(NEXTHOP_FLAG(4), b.Nexthops[0].Type)
	assert.Equal("172.16.1.101", b.Nexthops[0].Addr.String())

	//Serialize
	b.PrefixLength = uint8(24)
	buf, err = b.Serialize(2)
	ip = net.ParseIP("192.168.50.0").To4()
	assert.Equal(nil, err)
	assert.Equal(uint8(24), buf[0])
	assert.Equal([]byte(ip)[0:4], buf[1:5])

	// length invalid
	buf = make([]byte, 3)
	b = &ImportLookupBody{Api: IPV4_IMPORT_LOOKUP}
	err = b.DecodeFromBytes(buf, 2)
	assert.NotEqual(nil, err)
}

func Test_NexthopRegisterBody(t *testing.T) {
	assert := assert.New(t)

	// Input binary
	bufIn := []byte{
		0x01, 0x00, 0x02, 0x20, // connected(1 byte)=1, afi(2 bytes)=AF_INET, prefix_len(1 byte)=32
		0xc0, 0xa8, 0x01, 0x01, // prefix(4 bytes)="192.168.1.1"
		0x00, 0x00, 0x0a, 0x80, // connected(1 byte)=0, afi(2 bytes)=AF_INET6, prefix_len(1 byte)=128
		0x20, 0x01, 0x0d, 0xb8, // prefix(16 bytes)="2001:db8:1:1::1"
		0x00, 0x01, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
	}
	binary.BigEndian.PutUint16(bufIn[1:], syscall.AF_INET)
	binary.BigEndian.PutUint16(bufIn[9:], syscall.AF_INET6)

	// Test DecodeFromBytes()
	b := &NexthopRegisterBody{Api: NEXTHOP_REGISTER}
	err := b.DecodeFromBytes(bufIn, 3)
	assert.Nil(err)

	// Test decoded values
	assert.Equal(uint8(1), b.Nexthops[0].Connected)
	assert.Equal(uint16(syscall.AF_INET), b.Nexthops[0].Family)
	assert.Equal(net.ParseIP("192.168.1.1").To4(), b.Nexthops[0].Prefix)
	assert.Equal(uint8(0), b.Nexthops[1].Connected)
	assert.Equal(uint16(syscall.AF_INET6), b.Nexthops[1].Family)
	assert.Equal(net.ParseIP("2001:db8:1:1::1").To16(), b.Nexthops[1].Prefix)

	// Test Serialize()
	bufOut, err := b.Serialize(3)
	assert.Nil(err)

	// Test serialised value
	assert.Equal(bufIn, bufOut)
}

func Test_NexthopUpdateBody(t *testing.T) {
	assert := assert.New(t)

	// Input binary
	bufIn := []byte{
		0x00, 0x02, 0x20, // afi(2 bytes)=AF_INET, prefix_len(1 byte)=32
		0xc0, 0xa8, 0x01, 0x01, // prefix(4 bytes)="192.168.1.1"
		0x00, 0x00, 0x00, 0x01, // metric(4 bytes)=1
		0x01,                   // nexthops(1 byte)=1
		0x04,                   // nexthop_type(1 byte)=NEXTHOP_IPV4_IFINDEX
		0xc0, 0xa8, 0x01, 0x01, // nexthop_ip(4 bytes)="192.168.0.1"
		0x00, 0x00, 0x00, 0x02, // nexthop_ifindex(4 byte)=2
	}

	// Test DecodeFromBytes()
	b := &NexthopUpdateBody{Api: NEXTHOP_UPDATE}
	err := b.DecodeFromBytes(bufIn, 2)
	assert.Nil(err)

	// Test decoded values
	assert.Equal(uint16(syscall.AF_INET), b.Family)
	assert.Equal(net.ParseIP("192.168.1.1").To4(), b.Prefix)
	assert.Equal(uint32(1), b.Metric)
	nexthop := &Nexthop{
		Type:    NEXTHOP_FLAG(NEXTHOP_IPV4_IFINDEX),
		Addr:    net.ParseIP("192.168.1.1").To4(),
		Ifindex: uint32(2),
	}
	assert.Equal(1, len(b.Nexthops))
	assert.Equal(nexthop, b.Nexthops[0])
}
