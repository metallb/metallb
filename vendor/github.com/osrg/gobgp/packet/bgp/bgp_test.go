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

package bgp

import (
	"bytes"
	"encoding/binary"
	"net"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func keepalive() *BGPMessage {
	return NewBGPKeepAliveMessage()
}

func notification() *BGPMessage {
	return NewBGPNotificationMessage(1, 2, nil)
}

func refresh() *BGPMessage {
	return NewBGPRouteRefreshMessage(1, 2, 10)
}

func Test_Message(t *testing.T) {
	l := []*BGPMessage{keepalive(), notification(), refresh(), NewTestBGPOpenMessage(), NewTestBGPUpdateMessage()}
	for _, m1 := range l {
		buf1, _ := m1.Serialize()
		t.Log("LEN =", len(buf1))
		m2, err := ParseBGPMessage(buf1)
		if err != nil {
			t.Error(err)
		}
		// FIXME: shouldn't but workaround for some structs.
		buf2, _ := m2.Serialize()

		if reflect.DeepEqual(m1, m2) == true {
			t.Log("OK")
		} else {
			t.Error("Something wrong")
			t.Error(len(buf1), m1, buf1)
			t.Error(len(buf2), m2, buf2)
		}
	}
}

func Test_IPAddrPrefixString(t *testing.T) {
	ipv4 := NewIPAddrPrefix(24, "129.6.10.0")
	assert.Equal(t, "129.6.10.0/24", ipv4.String())
	ipv6 := NewIPv6AddrPrefix(18, "3343:faba:3903::1")
	assert.Equal(t, "3343:faba:3903::1/18", ipv6.String())
	ipv6 = NewIPv6AddrPrefix(18, "3343:faba:3903::0")
	assert.Equal(t, "3343:faba:3903::/18", ipv6.String())
}

func Test_RouteTargetMembershipNLRIString(t *testing.T) {
	assert := assert.New(t)

	// TwoOctetAsSpecificExtended
	buf := make([]byte, 13)
	buf[0] = 12
	binary.BigEndian.PutUint32(buf[1:5], 65546)
	buf[5] = byte(EC_TYPE_TRANSITIVE_TWO_OCTET_AS_SPECIFIC) // typehigh
	binary.BigEndian.PutUint16(buf[7:9], 65000)
	binary.BigEndian.PutUint32(buf[9:], 65546)
	r := &RouteTargetMembershipNLRI{}
	err := r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:65000:65546", r.String())
	buf, err = r.Serialize()
	assert.Equal(nil, err)
	err = r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:65000:65546", r.String())

	// IPv4AddressSpecificExtended
	buf = make([]byte, 13)
	binary.BigEndian.PutUint32(buf[1:5], 65546)
	buf[5] = byte(EC_TYPE_TRANSITIVE_IP4_SPECIFIC) // typehigh
	ip := net.ParseIP("10.0.0.1").To4()
	copy(buf[7:11], []byte(ip))
	binary.BigEndian.PutUint16(buf[11:], 65000)
	r = &RouteTargetMembershipNLRI{}
	err = r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:10.0.0.1:65000", r.String())
	buf, err = r.Serialize()
	assert.Equal(nil, err)
	err = r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:10.0.0.1:65000", r.String())

	// FourOctetAsSpecificExtended
	buf = make([]byte, 13)
	binary.BigEndian.PutUint32(buf[1:5], 65546)
	buf[5] = byte(EC_TYPE_TRANSITIVE_FOUR_OCTET_AS_SPECIFIC) // typehigh
	buf[6] = byte(EC_SUBTYPE_ROUTE_TARGET)                   // subtype
	binary.BigEndian.PutUint32(buf[7:], 65546)
	binary.BigEndian.PutUint16(buf[11:], 65000)
	r = &RouteTargetMembershipNLRI{}
	err = r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:1.10:65000", r.String())
	buf, err = r.Serialize()
	assert.Equal(nil, err)
	err = r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:1.10:65000", r.String())

	// OpaqueExtended
	buf = make([]byte, 13)
	binary.BigEndian.PutUint32(buf[1:5], 65546)
	buf[5] = byte(EC_TYPE_TRANSITIVE_OPAQUE) // typehigh
	binary.BigEndian.PutUint32(buf[9:], 1000000)
	r = &RouteTargetMembershipNLRI{}
	err = r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:1000000", r.String())
	buf, err = r.Serialize()
	assert.Equal(nil, err)
	err = r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:1000000", r.String())

	// Unknown
	buf = make([]byte, 13)
	binary.BigEndian.PutUint32(buf[1:5], 65546)
	buf[5] = 0x04 // typehigh
	binary.BigEndian.PutUint32(buf[9:], 1000000)
	r = &RouteTargetMembershipNLRI{}
	err = r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:1000000", r.String())
	buf, err = r.Serialize()
	assert.Equal(nil, err)
	err = r.DecodeFromBytes(buf)
	assert.Equal(nil, err)
	assert.Equal("65546:1000000", r.String())

}

func Test_MalformedUpdateMsg(t *testing.T) {
	assert := assert.New(t)

	// Invalid AGGREGATOR
	bufin := []byte{
		0x00, 0x00, // Withdraws(0)
		0x00, 0x16, // Attrs Len(22)
		0xc0, 0x07, 0x05, // Flag, Type(7), Length(5)
		0x00, 0x00, 0x00, 0x64, // aggregator - invalid length
		0x00,
		0x40, 0x01, 0x01, 0x00, // Attr(ORIGIN)
		0x40, 0x03, 0x04, 0xc0, // Attr(NEXT_HOP)
		0xa8, 0x01, 0x64,
		0x40, 0x02, 0x00, // Attr(AS_PATH)
	}

	u := &BGPUpdate{}
	err := u.DecodeFromBytes(bufin)
	assert.Error(err)
	assert.Equal(ERROR_HANDLING_ATTRIBUTE_DISCARD, err.(*MessageError).ErrorHandling)

	// Invalid MP_REACH_NLRI
	bufin = []byte{
		0x00, 0x00, // Withdraws(0)
		0x00, 0x27, // Attrs Len(39)
		0x80, 0x0e, 0x1d, // Flag, Type(14), Length(29)
		0x00, 0x02, 0x01, // afi(2), safi(1)
		0x0f, 0x00, 0x00, 0x00, // nexthop - invalid nexthop length
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0xff,
		0xff, 0x0a, 0x00, 0x00,
		0x00,                   // SNPA(0)
		0x40, 0x20, 0x01, 0x0d, // NLRI
		0xb8, 0x00, 0x01, 0x00,
		0x00,
		0x40, 0x01, 0x01, 0x00, // Attr(ORIGIN)
		0x40, 0x02, 0x00, // Attr(AS_PATH)
	}

	err = u.DecodeFromBytes(bufin)
	assert.Error(err)
	assert.Equal(ERROR_HANDLING_AFISAFI_DISABLE, err.(*MessageError).ErrorHandling)

	// Invalid flag
	bufin = []byte{
		0x00, 0x00, // Withdraws(0)
		0x00, 0x0e, // Attrs Len(14)
		0xc0, 0x01, 0x01, 0x00, // Attr(ORIGIN) - invalid flag
		0x40, 0x03, 0x04, 0xc0, // Attr(NEXT_HOP)
		0xa8, 0x01, 0x64,
		0x40, 0x02, 0x00, // Attr(AS_PATH)
	}

	err = u.DecodeFromBytes(bufin)
	assert.Error(err)
	assert.Equal(ERROR_HANDLING_TREAT_AS_WITHDRAW, err.(*MessageError).ErrorHandling)

	// Invalid AGGREGATOR and MULTI_EXIT_DESC
	bufin = []byte{
		0x00, 0x00, // Withdraws(0)
		0x00, 0x1e, // Attrs Len(30)
		0xc0, 0x07, 0x05, 0x00, // Attr(AGGREGATOR) - invalid length
		0x00, 0x00, 0x64, 0x00,
		0x80, 0x04, 0x05, 0x00, // Attr(MULTI_EXIT_DESC)  - invalid length
		0x00, 0x00, 0x00, 0x64,
		0x40, 0x01, 0x01, 0x00, // Attr(ORIGIN)
		0x40, 0x02, 0x00, // Attr(AS_PATH)
		0x40, 0x03, 0x04, 0xc0, // Attr(NEXT_HOP)
		0xa8, 0x01, 0x64,
		0x20, 0xc8, 0xc8, 0xc8, // NLRI
		0xc8,
	}

	err = u.DecodeFromBytes(bufin)
	assert.Error(err)
	assert.Equal(ERROR_HANDLING_TREAT_AS_WITHDRAW, err.(*MessageError).ErrorHandling)
}

func Test_RFC5512(t *testing.T) {
	assert := assert.New(t)

	buf := make([]byte, 8)
	buf[0] = byte(EC_TYPE_TRANSITIVE_OPAQUE)
	buf[1] = byte(EC_SUBTYPE_COLOR)
	binary.BigEndian.PutUint32(buf[4:], 1000000)
	ec, err := ParseExtended(buf)
	assert.Equal(nil, err)
	assert.Equal("1000000", ec.String())
	buf, err = ec.Serialize()
	assert.Equal(nil, err)
	assert.Equal([]byte{0x3, 0xb, 0x0, 0x0, 0x0, 0xf, 0x42, 0x40}, buf)

	buf = make([]byte, 8)
	buf[0] = byte(EC_TYPE_TRANSITIVE_OPAQUE)
	buf[1] = byte(EC_SUBTYPE_ENCAPSULATION)
	binary.BigEndian.PutUint16(buf[6:], uint16(TUNNEL_TYPE_VXLAN))
	ec, err = ParseExtended(buf)
	assert.Equal(nil, err)
	assert.Equal("VXLAN", ec.String())
	buf, err = ec.Serialize()
	assert.Equal(nil, err)
	assert.Equal([]byte{0x3, 0xc, 0x0, 0x0, 0x0, 0x0, 0x0, 0x8}, buf)

	subTlv := &TunnelEncapSubTLV{
		Type:  ENCAP_SUBTLV_TYPE_COLOR,
		Value: &TunnelEncapSubTLVColor{10},
	}

	tlv := &TunnelEncapTLV{
		Type:  TUNNEL_TYPE_VXLAN,
		Value: []*TunnelEncapSubTLV{subTlv},
	}

	attr := NewPathAttributeTunnelEncap([]*TunnelEncapTLV{tlv})

	buf1, err := attr.Serialize()
	assert.Equal(nil, err)

	p, err := GetPathAttribute(buf1)
	assert.Equal(nil, err)

	err = p.DecodeFromBytes(buf1)
	assert.Equal(nil, err)

	buf2, err := p.Serialize()
	assert.Equal(nil, err)
	assert.Equal(buf1, buf2)

	n1 := NewEncapNLRI("10.0.0.1")
	buf1, err = n1.Serialize()
	assert.Equal(nil, err)

	n2 := NewEncapNLRI("")
	err = n2.DecodeFromBytes(buf1)
	assert.Equal(nil, err)
	assert.Equal("10.0.0.1", n2.String())

	n3 := NewEncapv6NLRI("2001::1")
	buf1, err = n3.Serialize()
	assert.Equal(nil, err)

	n4 := NewEncapv6NLRI("")
	err = n4.DecodeFromBytes(buf1)
	assert.Equal(nil, err)
	assert.Equal("2001::1", n4.String())
}

func Test_ASLen(t *testing.T) {
	assert := assert.New(t)

	aspath := AsPathParam{
		Num: 2,
		AS:  []uint16{65000, 65001},
	}
	aspath.Type = BGP_ASPATH_ATTR_TYPE_SEQ
	assert.Equal(2, aspath.ASLen())

	aspath.Type = BGP_ASPATH_ATTR_TYPE_SET
	assert.Equal(1, aspath.ASLen())

	aspath.Type = BGP_ASPATH_ATTR_TYPE_CONFED_SEQ
	assert.Equal(0, aspath.ASLen())

	aspath.Type = BGP_ASPATH_ATTR_TYPE_CONFED_SET
	assert.Equal(0, aspath.ASLen())

	as4path := As4PathParam{
		Num: 2,
		AS:  []uint32{65000, 65001},
	}
	as4path.Type = BGP_ASPATH_ATTR_TYPE_SEQ
	assert.Equal(2, as4path.ASLen())

	as4path.Type = BGP_ASPATH_ATTR_TYPE_SET
	assert.Equal(1, as4path.ASLen())

	as4path.Type = BGP_ASPATH_ATTR_TYPE_CONFED_SEQ
	assert.Equal(0, as4path.ASLen())

	as4path.Type = BGP_ASPATH_ATTR_TYPE_CONFED_SET
	assert.Equal(0, as4path.ASLen())

}

func Test_MPLSLabelStack(t *testing.T) {
	assert := assert.New(t)
	mpls := NewMPLSLabelStack()
	buf, err := mpls.Serialize()
	assert.Nil(err)
	assert.Equal(true, bytes.Equal(buf, []byte{0, 0, 1}))

	mpls = &MPLSLabelStack{}
	assert.Nil(mpls.DecodeFromBytes(buf))
	assert.Equal(1, len(mpls.Labels))
	assert.Equal(uint32(0), mpls.Labels[0])

	mpls = NewMPLSLabelStack(WITHDRAW_LABEL)
	buf, err = mpls.Serialize()
	assert.Nil(err)
	assert.Equal(true, bytes.Equal(buf, []byte{128, 0, 0}))

	mpls = &MPLSLabelStack{}
	assert.Nil(mpls.DecodeFromBytes(buf))
	assert.Equal(1, len(mpls.Labels))
	assert.Equal(WITHDRAW_LABEL, mpls.Labels[0])
}

func Test_FlowSpecNlri(t *testing.T) {
	assert := assert.New(t)
	cmp := make([]FlowSpecComponentInterface, 0)
	cmp = append(cmp, NewFlowSpecDestinationPrefix(NewIPAddrPrefix(24, "10.0.0.0")))
	cmp = append(cmp, NewFlowSpecSourcePrefix(NewIPAddrPrefix(24, "10.0.0.0")))
	item1 := NewFlowSpecComponentItem(DEC_NUM_OP_EQ, TCP)
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_IP_PROTO, []*FlowSpecComponentItem{item1}))
	item2 := NewFlowSpecComponentItem(DEC_NUM_OP_GT_EQ, 20)
	item3 := NewFlowSpecComponentItem(DEC_NUM_OP_AND|DEC_NUM_OP_LT_EQ, 30)
	item4 := NewFlowSpecComponentItem(DEC_NUM_OP_GT_EQ, 10)
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_PORT, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_DST_PORT, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_SRC_PORT, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_ICMP_TYPE, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_ICMP_CODE, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_PKT_LEN, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_DSCP, []*FlowSpecComponentItem{item2, item3, item4}))
	isFragment := uint64(0x02)
	lastFragment := uint64(0x08)
	item5 := NewFlowSpecComponentItem(BITMASK_FLAG_OP_MATCH, isFragment)
	item6 := NewFlowSpecComponentItem(BITMASK_FLAG_OP_AND, lastFragment)
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_FRAGMENT, []*FlowSpecComponentItem{item5, item6}))
	item7 := NewFlowSpecComponentItem(0, TCP_FLAG_ACK)
	item8 := NewFlowSpecComponentItem(BITMASK_FLAG_OP_AND|BITMASK_FLAG_OP_NOT, TCP_FLAG_URGENT)
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_TCP_FLAG, []*FlowSpecComponentItem{item7, item8}))
	n1 := NewFlowSpecIPv4Unicast(cmp)
	buf1, err := n1.Serialize()
	assert.Nil(err)
	n2, err := NewPrefixFromRouteFamily(RouteFamilyToAfiSafi(RF_FS_IPv4_UC))
	assert.Nil(err)
	err = n2.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ := n2.Serialize()
	if reflect.DeepEqual(n1, n2) == true {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), n1, buf1)
		t.Error(len(buf2), n2, buf2)
		t.Log(bytes.Equal(buf1, buf2))
	}
}

func Test_FlowSpecExtended(t *testing.T) {
	assert := assert.New(t)
	exts := make([]ExtendedCommunityInterface, 0)
	exts = append(exts, NewTrafficRateExtended(100, 9600.0))
	exts = append(exts, NewTrafficActionExtended(true, false))
	exts = append(exts, NewRedirectTwoOctetAsSpecificExtended(1000, 1000))
	exts = append(exts, NewRedirectIPv4AddressSpecificExtended("10.0.0.1", 1000))
	exts = append(exts, NewRedirectFourOctetAsSpecificExtended(10000000, 1000))
	exts = append(exts, NewTrafficRemarkExtended(10))
	m1 := NewPathAttributeExtendedCommunities(exts)
	buf1, err := m1.Serialize()
	assert.Nil(err)
	m2 := NewPathAttributeExtendedCommunities(nil)
	err = m2.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ := m2.Serialize()
	if reflect.DeepEqual(m1, m2) == true {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), m1, buf1)
		t.Error(len(buf2), m2, buf2)
	}
}

func Test_IP6FlowSpecExtended(t *testing.T) {
	assert := assert.New(t)
	exts := make([]ExtendedCommunityInterface, 0)
	exts = append(exts, NewRedirectIPv6AddressSpecificExtended("2001:db8::68", 1000))
	m1 := NewPathAttributeIP6ExtendedCommunities(exts)
	buf1, err := m1.Serialize()
	assert.Nil(err)
	m2 := NewPathAttributeIP6ExtendedCommunities(nil)
	err = m2.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ := m2.Serialize()
	if reflect.DeepEqual(m1, m2) == true {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), m1, buf1)
		t.Error(len(buf2), m2, buf2)
	}
}

func Test_FlowSpecNlriv6(t *testing.T) {
	assert := assert.New(t)
	cmp := make([]FlowSpecComponentInterface, 0)
	cmp = append(cmp, NewFlowSpecDestinationPrefix6(NewIPv6AddrPrefix(64, "2001::"), 12))
	cmp = append(cmp, NewFlowSpecSourcePrefix6(NewIPv6AddrPrefix(64, "2001::"), 12))
	item1 := NewFlowSpecComponentItem(DEC_NUM_OP_EQ, TCP)
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_IP_PROTO, []*FlowSpecComponentItem{item1}))
	item2 := NewFlowSpecComponentItem(DEC_NUM_OP_GT_EQ, 20)
	item3 := NewFlowSpecComponentItem(DEC_NUM_OP_AND|DEC_NUM_OP_LT_EQ, 30)
	item4 := NewFlowSpecComponentItem(DEC_NUM_OP_EQ, 10)
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_PORT, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_DST_PORT, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_SRC_PORT, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_ICMP_TYPE, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_ICMP_CODE, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_PKT_LEN, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_DSCP, []*FlowSpecComponentItem{item2, item3, item4}))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_LABEL, []*FlowSpecComponentItem{item2, item3, item4}))
	isFragment := uint64(0x02)
	item5 := NewFlowSpecComponentItem(BITMASK_FLAG_OP_MATCH, isFragment)
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_FRAGMENT, []*FlowSpecComponentItem{item5}))
	item6 := NewFlowSpecComponentItem(0, TCP_FLAG_ACK)
	item7 := NewFlowSpecComponentItem(BITMASK_FLAG_OP_AND|BITMASK_FLAG_OP_NOT, TCP_FLAG_URGENT)
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_TCP_FLAG, []*FlowSpecComponentItem{item6, item7}))
	n1 := NewFlowSpecIPv6Unicast(cmp)
	buf1, err := n1.Serialize()
	assert.Nil(err)
	n2, err := NewPrefixFromRouteFamily(RouteFamilyToAfiSafi(RF_FS_IPv6_UC))
	assert.Nil(err)
	err = n2.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ := n2.Serialize()
	if reflect.DeepEqual(n1, n2) == true {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), n1, buf1)
		t.Error(len(buf2), n2, buf2)
		t.Log(bytes.Equal(buf1, buf2))
	}
}

func Test_Aigp(t *testing.T) {
	assert := assert.New(t)
	m := NewAigpTLVIgpMetric(1000)
	a1 := NewPathAttributeAigp([]AigpTLV{m})
	buf1, err := a1.Serialize()
	assert.Nil(err)
	a2 := NewPathAttributeAigp(nil)
	err = a2.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ := a2.Serialize()
	if reflect.DeepEqual(a1, a2) == true {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), a1, buf1)
		t.Error(len(buf2), a2, buf2)
		t.Log(bytes.Equal(buf1, buf2))
	}
}

func Test_FlowSpecNlriL2(t *testing.T) {
	assert := assert.New(t)
	mac, _ := net.ParseMAC("01:23:45:67:89:ab")
	cmp := make([]FlowSpecComponentInterface, 0)
	cmp = append(cmp, NewFlowSpecDestinationMac(mac))
	cmp = append(cmp, NewFlowSpecSourceMac(mac))
	item1 := NewFlowSpecComponentItem(DEC_NUM_OP_EQ, uint64(IPv4))
	cmp = append(cmp, NewFlowSpecComponent(FLOW_SPEC_TYPE_ETHERNET_TYPE, []*FlowSpecComponentItem{item1}))
	rd, _ := ParseRouteDistinguisher("100:100")
	n1 := NewFlowSpecL2VPN(rd, cmp)
	buf1, err := n1.Serialize()
	assert.Nil(err)
	n2, err := NewPrefixFromRouteFamily(RouteFamilyToAfiSafi(RF_FS_L2_VPN))
	assert.Nil(err)
	err = n2.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ := n2.Serialize()
	if reflect.DeepEqual(n1, n2) == true {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), n1, buf1)
		t.Error(len(buf2), n2, buf2)
		t.Log(bytes.Equal(buf1, buf2))
	}
}

func Test_NotificationErrorCode(t *testing.T) {
	// boundary check
	t.Log(NewNotificationErrorCode(BGP_ERROR_MESSAGE_HEADER_ERROR, BGP_ERROR_SUB_BAD_MESSAGE_TYPE).String())
	t.Log(NewNotificationErrorCode(BGP_ERROR_MESSAGE_HEADER_ERROR, BGP_ERROR_SUB_BAD_MESSAGE_TYPE+1).String())
	t.Log(NewNotificationErrorCode(BGP_ERROR_MESSAGE_HEADER_ERROR, 0).String())
	t.Log(NewNotificationErrorCode(0, BGP_ERROR_SUB_BAD_MESSAGE_TYPE).String())
	t.Log(NewNotificationErrorCode(BGP_ERROR_ROUTE_REFRESH_MESSAGE_ERROR+1, 0).String())
}

func Test_FlowSpecNlriVPN(t *testing.T) {
	assert := assert.New(t)
	cmp := make([]FlowSpecComponentInterface, 0)
	cmp = append(cmp, NewFlowSpecDestinationPrefix(NewIPAddrPrefix(24, "10.0.0.0")))
	cmp = append(cmp, NewFlowSpecSourcePrefix(NewIPAddrPrefix(24, "10.0.0.0")))
	rd, _ := ParseRouteDistinguisher("100:100")
	n1 := NewFlowSpecIPv4VPN(rd, cmp)
	buf1, err := n1.Serialize()
	assert.Nil(err)
	n2, err := NewPrefixFromRouteFamily(RouteFamilyToAfiSafi(RF_FS_IPv4_VPN))
	assert.Nil(err)
	err = n2.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ := n2.Serialize()
	if reflect.DeepEqual(n1, n2) == true {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), n1, buf1)
		t.Error(len(buf2), n2, buf2)
		t.Log(bytes.Equal(buf1, buf2))
	}
}

func Test_EVPNIPPrefixRoute(t *testing.T) {
	assert := assert.New(t)
	rd, _ := ParseRouteDistinguisher("100:100")
	r := &EVPNIPPrefixRoute{
		RD: rd,
		ESI: EthernetSegmentIdentifier{
			Type:  ESI_ARBITRARY,
			Value: make([]byte, 9),
		},
		ETag:           10,
		IPPrefixLength: 24,
		IPPrefix:       net.IP{10, 10, 10, 0},
		GWIPAddress:    net.IP{10, 10, 10, 10},
		Label:          1000,
	}
	n1 := NewEVPNNLRI(EVPN_IP_PREFIX, 0, r)
	buf1, err := n1.Serialize()
	assert.Nil(err)
	n2, err := NewPrefixFromRouteFamily(RouteFamilyToAfiSafi(RF_EVPN))
	assert.Nil(err)
	err = n2.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ := n2.Serialize()
	t.Log(n1.RouteTypeData.(*EVPNIPPrefixRoute).ESI.Value, n2.(*EVPNNLRI).RouteTypeData.(*EVPNIPPrefixRoute).ESI.Value)
	t.Log(reflect.DeepEqual(n1.RouteTypeData.(*EVPNIPPrefixRoute).ESI.Value, n2.(*EVPNNLRI).RouteTypeData.(*EVPNIPPrefixRoute).ESI.Value))
	if reflect.DeepEqual(n1, n2) {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), n1, buf1)
		t.Error(len(buf2), n2, buf2)
		t.Log(bytes.Equal(buf1, buf2))
	}
}

func Test_CapExtendedNexthop(t *testing.T) {
	assert := assert.New(t)
	tuple := NewCapExtendedNexthopTuple(RF_IPv4_UC, AFI_IP6)
	n1 := NewCapExtendedNexthop([]*CapExtendedNexthopTuple{tuple})
	buf1, err := n1.Serialize()
	assert.Nil(err)
	n2, err := DecodeCapability(buf1)
	assert.Nil(err)
	buf2, _ := n2.Serialize()
	if reflect.DeepEqual(n1, n2) {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), n1, buf1)
		t.Error(len(buf2), n2, buf2)
		t.Log(bytes.Equal(buf1, buf2))
	}
}

func Test_AddPath(t *testing.T) {
	assert := assert.New(t)
	opt := &MarshallingOption{AddPath: map[RouteFamily]BGPAddPathMode{RF_IPv4_UC: BGP_ADD_PATH_BOTH}}
	{
		n1 := NewIPAddrPrefix(24, "10.10.10.0")
		assert.Equal(n1.PathIdentifier(), uint32(0))
		n1.SetPathLocalIdentifier(10)
		assert.Equal(n1.PathLocalIdentifier(), uint32(10))
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := &IPAddrPrefix{}
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(10))
	}
	{
		n1 := NewIPv6AddrPrefix(64, "2001::")
		n1.SetPathIdentifier(10)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewIPv6AddrPrefix(0, "")
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(0))
	}
	opt = &MarshallingOption{AddPath: map[RouteFamily]BGPAddPathMode{RF_IPv4_UC: BGP_ADD_PATH_BOTH, RF_IPv6_UC: BGP_ADD_PATH_BOTH}}
	{
		n1 := NewIPv6AddrPrefix(64, "2001::")
		n1.SetPathLocalIdentifier(10)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewIPv6AddrPrefix(0, "")
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(10))
	}
	opt = &MarshallingOption{AddPath: map[RouteFamily]BGPAddPathMode{RF_IPv4_VPN: BGP_ADD_PATH_BOTH, RF_IPv6_VPN: BGP_ADD_PATH_BOTH}}
	{
		rd, _ := ParseRouteDistinguisher("100:100")
		labels := NewMPLSLabelStack(100, 200)
		n1 := NewLabeledVPNIPAddrPrefix(24, "10.10.10.0", *labels, rd)
		n1.SetPathLocalIdentifier(20)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewLabeledVPNIPAddrPrefix(0, "", MPLSLabelStack{}, nil)
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(20))
	}
	{
		rd, _ := ParseRouteDistinguisher("100:100")
		labels := NewMPLSLabelStack(100, 200)
		n1 := NewLabeledVPNIPv6AddrPrefix(64, "2001::", *labels, rd)
		n1.SetPathLocalIdentifier(20)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewLabeledVPNIPAddrPrefix(0, "", MPLSLabelStack{}, nil)
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(20))
	}
	opt = &MarshallingOption{AddPath: map[RouteFamily]BGPAddPathMode{RF_IPv4_MPLS: BGP_ADD_PATH_BOTH, RF_IPv6_MPLS: BGP_ADD_PATH_BOTH}}
	{
		labels := NewMPLSLabelStack(100, 200)
		n1 := NewLabeledIPAddrPrefix(24, "10.10.10.0", *labels)
		n1.SetPathLocalIdentifier(20)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewLabeledIPAddrPrefix(0, "", MPLSLabelStack{})
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(20))
	}
	{
		labels := NewMPLSLabelStack(100, 200)
		n1 := NewLabeledIPv6AddrPrefix(64, "2001::", *labels)
		n1.SetPathLocalIdentifier(20)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewLabeledIPAddrPrefix(0, "", MPLSLabelStack{})
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(20))
	}
	opt = &MarshallingOption{AddPath: map[RouteFamily]BGPAddPathMode{RF_RTC_UC: BGP_ADD_PATH_BOTH}}
	{
		rt, _ := ParseRouteTarget("100:100")
		n1 := NewRouteTargetMembershipNLRI(65000, rt)
		n1.SetPathLocalIdentifier(30)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewRouteTargetMembershipNLRI(0, nil)
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(30))
	}
	opt = &MarshallingOption{AddPath: map[RouteFamily]BGPAddPathMode{RF_EVPN: BGP_ADD_PATH_BOTH}}
	{
		n1 := NewEVPNNLRI(EVPN_ROUTE_TYPE_ETHERNET_AUTO_DISCOVERY, 0,
			&EVPNEthernetAutoDiscoveryRoute{NewRouteDistinguisherFourOctetAS(5, 6),
				EthernetSegmentIdentifier{ESI_ARBITRARY, make([]byte, 9)}, 2, 2})
		n1.SetPathLocalIdentifier(40)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewEVPNNLRI(0, 0, nil)
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(40))
	}
	opt = &MarshallingOption{AddPath: map[RouteFamily]BGPAddPathMode{RF_IPv4_ENCAP: BGP_ADD_PATH_BOTH}}
	{
		n1 := NewEncapNLRI("10.10.10.0")
		n1.SetPathLocalIdentifier(50)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewEncapNLRI("")
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(50))
	}
	opt = &MarshallingOption{AddPath: map[RouteFamily]BGPAddPathMode{RF_FS_IPv4_UC: BGP_ADD_PATH_BOTH}}
	{
		n1 := NewFlowSpecIPv4Unicast([]FlowSpecComponentInterface{NewFlowSpecDestinationPrefix(NewIPAddrPrefix(24, "10.0.0.0"))})
		n1.SetPathLocalIdentifier(60)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := NewFlowSpecIPv4Unicast(nil)
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(60))
	}
	opt = &MarshallingOption{AddPath: map[RouteFamily]BGPAddPathMode{RF_OPAQUE: BGP_ADD_PATH_BOTH}}
	{
		n1 := NewOpaqueNLRI([]byte("key"), []byte("value"))
		n1.SetPathLocalIdentifier(70)
		bits, err := n1.Serialize(opt)
		assert.Nil(err)
		n2 := &OpaqueNLRI{}
		err = n2.DecodeFromBytes(bits, opt)
		assert.Nil(err)
		assert.Equal(n2.PathIdentifier(), uint32(70))
	}

}

func Test_CompareFlowSpecNLRI(t *testing.T) {
	assert := assert.New(t)
	cmp, err := ParseFlowSpecComponents(RF_FS_IPv4_UC, "destination 10.0.0.2/32 source 10.0.0.1/32 destination-port ==3128 protocol tcp")
	assert.Nil(err)
	// Note: Use NewFlowSpecIPv4Unicast() for the consistent ordered rules.
	n1 := NewFlowSpecIPv4Unicast(cmp).FlowSpecNLRI
	cmp, err = ParseFlowSpecComponents(RF_FS_IPv4_UC, "source 10.0.0.0/24 destination-port ==3128 protocol tcp")
	assert.Nil(err)
	n2 := NewFlowSpecIPv4Unicast(cmp).FlowSpecNLRI
	r, err := CompareFlowSpecNLRI(&n1, &n2)
	assert.Nil(err)
	assert.True(r > 0)
	cmp, err = ParseFlowSpecComponents(RF_FS_IPv4_UC, "source 10.0.0.9/32 port ==80 ==8080 destination-port >8080&<8080 ==3128 source-port >1024 protocol ==udp ==tcp")
	n3 := NewFlowSpecIPv4Unicast(cmp).FlowSpecNLRI
	assert.Nil(err)
	cmp, err = ParseFlowSpecComponents(RF_FS_IPv4_UC, "destination 192.168.0.2/32")
	n4 := NewFlowSpecIPv4Unicast(cmp).FlowSpecNLRI
	assert.Nil(err)
	r, err = CompareFlowSpecNLRI(&n3, &n4)
	assert.Nil(err)
	assert.True(r < 0)
}

func Test_MpReachNLRIWithIPv4MappedIPv6Prefix(t *testing.T) {
	assert := assert.New(t)
	n1 := NewIPv6AddrPrefix(120, "::ffff:10.0.0.1")
	buf1, err := n1.Serialize()
	assert.Nil(err)
	n2, err := NewPrefixFromRouteFamily(RouteFamilyToAfiSafi(RF_IPv6_UC))
	assert.Nil(err)
	err = n2.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ := n2.Serialize()
	if reflect.DeepEqual(n1, n2) {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), n1, buf1)
		t.Error(len(buf2), n2, buf2)
		t.Log(bytes.Equal(buf1, buf2))
	}

	label := NewMPLSLabelStack(2)

	n3 := NewLabeledIPv6AddrPrefix(120, "::ffff:10.0.0.1", *label)
	buf1, err = n3.Serialize()
	assert.Nil(err)
	n4, err := NewPrefixFromRouteFamily(RouteFamilyToAfiSafi(RF_IPv6_MPLS))
	assert.Nil(err)
	err = n4.DecodeFromBytes(buf1)
	assert.Nil(err)
	buf2, _ = n3.Serialize()
	t.Log(n3, n4)
	if reflect.DeepEqual(n3, n4) {
		t.Log("OK")
	} else {
		t.Error("Something wrong")
		t.Error(len(buf1), n3, buf1)
		t.Error(len(buf2), n4, buf2)
		t.Log(bytes.Equal(buf1, buf2))
	}
}

func Test_MpReachNLRIWithIPv6PrefixWithIPv4Peering(t *testing.T) {
	assert := assert.New(t)
	bufin := []byte{
		0x80, 0x0e, 0x1e, // flags(1), type(1), length(1)
		0x00, 0x02, 0x01, 0x10, // afi(2), safi(1), nexthoplen(1)
		0x00, 0x00, 0x00, 0x00, // nexthop(16)
		0x00, 0x00, 0x00, 0x00, // = "::ffff:172.20.0.1"
		0x00, 0x00, 0xff, 0xff,
		0xac, 0x14, 0x00, 0x01,
		0x00,                   // reserved(1)
		0x40, 0x20, 0x01, 0x0d, // nlri(9)
		0xb8, 0x00, 0x01, 0x00, // = "2001:db8:1:1::/64"
		0x01,
	}
	// Test DecodeFromBytes()
	p := &PathAttributeMpReachNLRI{}
	err := p.DecodeFromBytes(bufin)
	assert.Nil(err)
	// Test decoded values
	assert.Equal(BGPAttrFlag(0x80), p.Flags)
	assert.Equal(BGPAttrType(0xe), p.Type)
	assert.Equal(uint16(0x1e), p.Length)
	assert.Equal(uint16(AFI_IP6), p.AFI)
	assert.Equal(uint8(SAFI_UNICAST), p.SAFI)
	assert.Equal(net.ParseIP("::ffff:172.20.0.1"), p.Nexthop)
	assert.Equal(net.ParseIP(""), p.LinkLocalNexthop)
	value := []AddrPrefixInterface{
		NewIPv6AddrPrefix(64, "2001:db8:1:1::"),
	}
	assert.Equal(value, p.Value)
	// Set NextHop as IPv4 address (because IPv4 peering)
	p.Nexthop = net.ParseIP("172.20.0.1")
	// Test Serialize()
	bufout, err := p.Serialize()
	assert.Nil(err)
	// Test serialised value
	assert.Equal(bufin, bufout)
}

func Test_MpReachNLRIWithIPv6(t *testing.T) {
	assert := assert.New(t)
	bufin := []byte{
		0x90, 0x0e, 0x00, 0x1e, // flags(1), type(1), length(2),
		0x00, 0x02, 0x01, 0x10, // afi(2), safi(1), nexthoplen(1)
		0x20, 0x01, 0x0d, 0xb8, // nexthop(16)
		0x00, 0x01, 0x00, 0x00, // = "2001:db8:1::1"
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
		0x00,                   // reserved(1)
		0x40, 0x20, 0x01, 0x0d, // nlri(9)
		0xb8, 0x00, 0x53, 0x00, // = "2001:db8:53::/64"
		0x00,
	}
	// Test DecodeFromBytes()
	p := &PathAttributeMpReachNLRI{}
	err := p.DecodeFromBytes(bufin)
	assert.Nil(err)
	// Test decoded values
	assert.Equal(BGPAttrFlag(0x90), p.Flags)
	assert.Equal(BGPAttrType(0xe), p.Type)
	assert.Equal(uint16(0x1e), p.Length)
	assert.Equal(uint16(AFI_IP6), p.AFI)
	assert.Equal(uint8(SAFI_UNICAST), p.SAFI)
	assert.Equal(net.ParseIP("2001:db8:1::1"), p.Nexthop)
	value := []AddrPrefixInterface{
		NewIPv6AddrPrefix(64, "2001:db8:53::"),
	}
	assert.Equal(value, p.Value)
}

func Test_MpUnreachNLRIWithIPv6(t *testing.T) {
	assert := assert.New(t)
	bufin := []byte{
		0x90, 0x0f, 0x00, 0x0c, // flags(1), type(1), length(2),
		0x00, 0x02, 0x01, // afi(2), safi(1),
		0x40, 0x20, 0x01, 0x0d, // nlri(9)
		0xb8, 0x00, 0x53, 0x00, // = "2001:db8:53::/64"
		0x00,
	}
	// Test DecodeFromBytes()
	p := &PathAttributeMpUnreachNLRI{}
	err := p.DecodeFromBytes(bufin)
	assert.Nil(err)
	// Test decoded values
	assert.Equal(BGPAttrFlag(0x90), p.Flags)
	assert.Equal(BGPAttrType(0xf), p.Type)
	assert.Equal(uint16(0x0c), p.Length)
	assert.Equal(uint16(AFI_IP6), p.AFI)
	assert.Equal(uint8(SAFI_UNICAST), p.SAFI)
	value := []AddrPrefixInterface{
		NewIPv6AddrPrefix(64, "2001:db8:53::"),
	}
	assert.Equal(value, p.Value)
}

func Test_MpReachNLRIWithIPv6PrefixWithLinkLocalNexthop(t *testing.T) {
	assert := assert.New(t)
	bufin := []byte{
		0x80, 0x0e, 0x2c, // flags(1), type(1), length(1)
		0x00, 0x02, 0x01, 0x20, // afi(2), safi(1), nexthoplen(1)
		0x20, 0x01, 0x0d, 0xb8, // nexthop(32)
		0x00, 0x01, 0x00, 0x00, // = "2001:db8:1::1"
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
		0xfe, 0x80, 0x00, 0x00, // + "fe80::1" (link local)
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
		0x00,                   // reserved(1)
		0x30, 0x20, 0x10, 0x0a, // nlri(7)
		0xb8, 0x00, 0x01, // = "2010:ab8:1::/48"
	}
	// Test DecodeFromBytes()
	p := &PathAttributeMpReachNLRI{}
	err := p.DecodeFromBytes(bufin)
	assert.Nil(err)
	// Test decoded values
	assert.Equal(BGPAttrFlag(0x80), p.Flags)
	assert.Equal(BGPAttrType(0xe), p.Type)
	assert.Equal(uint16(0x2c), p.Length)
	assert.Equal(uint16(AFI_IP6), p.AFI)
	assert.Equal(uint8(SAFI_UNICAST), p.SAFI)
	assert.Equal(net.ParseIP("2001:db8:1::1"), p.Nexthop)
	assert.Equal(net.ParseIP("fe80::1"), p.LinkLocalNexthop)
	value := []AddrPrefixInterface{
		NewIPv6AddrPrefix(48, "2010:ab8:1::"),
	}
	assert.Equal(value, p.Value)
	// Test Serialize()
	bufout, err := p.Serialize()
	assert.Nil(err)
	// Test serialised value
	assert.Equal(bufin, bufout)
}

func Test_MpReachNLRIWithVPNv4Prefix(t *testing.T) {
	assert := assert.New(t)
	bufin := []byte{
		0x80, 0x0e, 0x20, // flags(1), type(1), length(1)
		0x00, 0x01, 0x80, 0x0c, // afi(2), safi(1), nexthoplen(1)
		0x00, 0x00, 0x00, 0x00, // nexthop(12)
		0x00, 0x00, 0x00, 0x00, // = (rd:"0:0",) "172.20.0.1"
		0xac, 0x14, 0x00, 0x01,
		0x00,                   // reserved(1)
		0x70, 0x00, 0x01, 0x01, // nlri(15)
		0x00, 0x00, 0xfd, 0xe8, // = label:16, rd:"65000:100", prefix:"10.1.1.0/24"
		0x00, 0x00, 0x00, 0x64,
		0x0a, 0x01, 0x01,
	}
	// Test DecodeFromBytes()
	p := &PathAttributeMpReachNLRI{}
	err := p.DecodeFromBytes(bufin)
	assert.Nil(err)
	// Test decoded values
	assert.Equal(BGPAttrFlag(0x80), p.Flags)
	assert.Equal(BGPAttrType(0xe), p.Type)
	assert.Equal(uint16(0x20), p.Length)
	assert.Equal(uint16(AFI_IP), p.AFI)
	assert.Equal(uint8(SAFI_MPLS_VPN), p.SAFI)
	assert.Equal(net.ParseIP("172.20.0.1").To4(), p.Nexthop)
	assert.Equal(net.ParseIP(""), p.LinkLocalNexthop)
	value := []AddrPrefixInterface{
		NewLabeledVPNIPAddrPrefix(24, "10.1.1.0", *NewMPLSLabelStack(16),
			NewRouteDistinguisherTwoOctetAS(65000, 100)),
	}
	assert.Equal(value, p.Value)
	// Test Serialize()
	bufout, err := p.Serialize()
	assert.Nil(err)
	// Test serialised value
	assert.Equal(bufin, bufout)
}

func Test_MpReachNLRIWithVPNv6Prefix(t *testing.T) {
	assert := assert.New(t)
	bufin := []byte{
		0x80, 0x0e, 0x39, // flags(1), type(1), length(1)
		0x00, 0x02, 0x80, 0x18, // afi(2), safi(1), nexthoplen(1)
		0x00, 0x00, 0x00, 0x00, // nexthop(24)
		0x00, 0x00, 0x00, 0x00, // = (rd:"0:0",) "2001:db8:1::1"
		0x20, 0x01, 0x0d, 0xb8,
		0x00, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
		0x00,                   // reserved(1)
		0xd4, 0x00, 0x01, 0x01, // nlri(28)
		0x00, 0x00, 0xfd, 0xe8, // = label:16, rd:"65000:100", prefix:"2001:1::/124"
		0x00, 0x00, 0x00, 0x64,
		0x20, 0x01, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	// Test DecodeFromBytes()
	p := &PathAttributeMpReachNLRI{}
	err := p.DecodeFromBytes(bufin)
	assert.Nil(err)
	// Test decoded values
	assert.Equal(BGPAttrFlag(0x80), p.Flags)
	assert.Equal(BGPAttrType(0xe), p.Type)
	assert.Equal(uint16(0x39), p.Length)
	assert.Equal(uint16(AFI_IP6), p.AFI)
	assert.Equal(uint8(SAFI_MPLS_VPN), p.SAFI)
	assert.Equal(net.ParseIP("2001:db8:1::1"), p.Nexthop)
	assert.Equal(net.ParseIP(""), p.LinkLocalNexthop)
	value := []AddrPrefixInterface{
		NewLabeledVPNIPv6AddrPrefix(124, "2001:1::", *NewMPLSLabelStack(16),
			NewRouteDistinguisherTwoOctetAS(65000, 100)),
	}
	assert.Equal(value, p.Value)
	// Test Serialize()
	bufout, err := p.Serialize()
	assert.Nil(err)
	// Test serialised value
	assert.Equal(bufin, bufout)
}

func Test_MpReachNLRIWithIPv4PrefixWithIPv6Nexthop(t *testing.T) {
	assert := assert.New(t)
	bufin := []byte{
		0x80, 0x0e, 0x19, // flags(1), type(1), length(1)
		0x00, 0x01, 0x01, 0x10, // afi(1), safi(1), nexthoplen(1)
		0x20, 0x01, 0x0d, 0xb8, // nexthop(32)
		0x00, 0x01, 0x00, 0x00, // = "2001:db8:1::1"
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
		0x00,                   // reserved(1)
		0x18, 0xc0, 0xa8, 0x0a, // nlri(7)
	}
	// Test DecodeFromBytes()
	p := &PathAttributeMpReachNLRI{}
	err := p.DecodeFromBytes(bufin)
	assert.Nil(err)
	// Test decoded values
	assert.Equal(BGPAttrFlag(0x80), p.Flags)
	assert.Equal(BGPAttrType(0xe), p.Type)
	assert.Equal(uint16(0x19), p.Length)
	assert.Equal(uint16(AFI_IP), p.AFI)
	assert.Equal(uint8(SAFI_UNICAST), p.SAFI)
	assert.Equal(net.ParseIP("2001:db8:1::1"), p.Nexthop)
	value := []AddrPrefixInterface{
		NewIPAddrPrefix(24, "192.168.10.0"),
	}
	assert.Equal(value, p.Value)
	// Test Serialize()
	bufout, err := p.Serialize()
	assert.Nil(err)
	// Test serialised value
	assert.Equal(bufin, bufout)
}

func Test_ParseRouteDistinguisher(t *testing.T) {
	assert := assert.New(t)

	rd, _ := ParseRouteDistinguisher("100:1000")
	rdType0, ok := rd.(*RouteDistinguisherTwoOctetAS)
	if !ok {
		t.Fatal("Type of RD interface is not RouteDistinguisherTwoOctetAS")
	}

	assert.Equal(uint16(100), rdType0.Admin)
	assert.Equal(uint32(1000), rdType0.Assigned)

	rd, _ = ParseRouteDistinguisher("10.0.0.0:100")
	rdType1, ok := rd.(*RouteDistinguisherIPAddressAS)
	if !ok {
		t.Fatal("Type of RD interface is not RouteDistinguisherIPAddressAS")
	}

	assert.Equal("10.0.0.0", rdType1.Admin.String())
	assert.Equal(uint16(100), rdType1.Assigned)

	rd, _ = ParseRouteDistinguisher("100.1000:10000")
	rdType2, ok := rd.(*RouteDistinguisherFourOctetAS)
	if !ok {
		t.Fatal("Type of RD interface is not RouteDistinguisherFourOctetAS")
	}

	assert.Equal(uint32((100<<16)|1000), rdType2.Admin)
	assert.Equal(uint16(10000), rdType2.Assigned)
}

func Test_ParseEthernetSegmentIdentifier(t *testing.T) {
	assert := assert.New(t)

	// "single-homed"
	esiZero := EthernetSegmentIdentifier{}
	args := make([]string, 0, 0)
	esi, err := ParseEthernetSegmentIdentifier(args)
	assert.Nil(err)
	assert.Equal(esiZero, esi)
	args = []string{"single-homed"}
	esi, err = ParseEthernetSegmentIdentifier(args)
	assert.Nil(err)
	assert.Equal(esiZero, esi)

	// ESI_ARBITRARY
	args = []string{"ARBITRARY", "11:22:33:44:55:66:77:88:99"} // omit "ESI_"
	esi, err = ParseEthernetSegmentIdentifier(args)
	assert.Nil(err)
	assert.Equal(EthernetSegmentIdentifier{
		Type:  ESI_ARBITRARY,
		Value: []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99},
	}, esi)

	// ESI_LACP
	args = []string{"lacp", "aa:bb:cc:dd:ee:ff", strconv.FormatInt(0x1122, 10)} // lower case
	esi, err = ParseEthernetSegmentIdentifier(args)
	assert.Nil(err)
	assert.Equal(EthernetSegmentIdentifier{
		Type:  ESI_LACP,
		Value: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22, 0x00},
	}, esi)

	// ESI_MSTP
	args = []string{"esi_mstp", "aa:bb:cc:dd:ee:ff", strconv.FormatInt(0x1122, 10)} // omit "ESI_" + lower case
	esi, err = ParseEthernetSegmentIdentifier(args)
	assert.Nil(err)
	assert.Equal(EthernetSegmentIdentifier{
		Type:  ESI_MSTP,
		Value: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22, 0x00},
	}, esi)

	// ESI_MAC
	args = []string{"ESI_MAC", "aa:bb:cc:dd:ee:ff", strconv.FormatInt(0x112233, 10)}
	esi, err = ParseEthernetSegmentIdentifier(args)
	assert.Nil(err)
	assert.Equal(EthernetSegmentIdentifier{
		Type:  ESI_MAC,
		Value: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22, 0x33},
	}, esi)

	// ESI_ROUTERID
	args = []string{"ESI_ROUTERID", "1.1.1.1", strconv.FormatInt(0x11223344, 10)}
	esi, err = ParseEthernetSegmentIdentifier(args)
	assert.Nil(err)
	assert.Equal(EthernetSegmentIdentifier{
		Type:  ESI_ROUTERID,
		Value: []byte{0x01, 0x01, 0x01, 0x01, 0x11, 0x22, 0x33, 0x44, 0x00},
	}, esi)

	// ESI_AS
	args = []string{"ESI_AS", strconv.FormatInt(0xaabbccdd, 10), strconv.FormatInt(0x11223344, 10)}
	esi, err = ParseEthernetSegmentIdentifier(args)
	assert.Nil(err)
	assert.Equal(EthernetSegmentIdentifier{
		Type:  ESI_AS,
		Value: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0x11, 0x22, 0x33, 0x44, 0x00},
	}, esi)

	// Other
	args = []string{"99", "11:22:33:44:55:66:77:88:99"}
	esi, err = ParseEthernetSegmentIdentifier(args)
	assert.Nil(err)
	assert.Equal(EthernetSegmentIdentifier{
		Type:  ESIType(99),
		Value: []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99},
	}, esi)
}
