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

package gobgpapi

import (
	"net"
	"testing"

	"github.com/osrg/gobgp/packet/bgp"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/stretchr/testify/assert"
)

func Test_OriginAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &OriginAttribute{
		Origin: 0, // IGP
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewOriginAttributeFromNative(n)
	assert.Equal(input.Origin, output.Origin)
}

func Test_AsPathAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &AsPathAttribute{
		Segments: []*AsSegment{
			{
				Type:    1, // SET
				Numbers: []uint32{100, 200},
			},
			{
				Type:    2, // SEQ
				Numbers: []uint32{300, 400},
			},
		},
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewAsPathAttributeFromNative(n)
	assert.Equal(2, len(output.Segments))
	assert.Equal(input.Segments, output.Segments)
}

func Test_NextHopAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &NextHopAttribute{
		NextHop: "192.168.0.1",
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewNextHopAttributeFromNative(n)
	assert.Equal(input.NextHop, output.NextHop)
}

func Test_MultiExitDiscAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &MultiExitDiscAttribute{
		Med: 100,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMultiExitDiscAttributeFromNative(n)
	assert.Equal(input.Med, output.Med)
}

func Test_LocalPrefAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &LocalPrefAttribute{
		LocalPref: 100,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewLocalPrefAttributeFromNative(n)
	assert.Equal(input.LocalPref, output.LocalPref)
}

func Test_AtomicAggregateAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &AtomicAggregateAttribute{}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewAtomicAggregateAttributeFromNative(n)
	// AtomicAggregateAttribute has no value
	assert.NotNil(output)
}

func Test_AggregatorAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &AggregatorAttribute{
		As:      65000,
		Address: "1.1.1.1",
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewAggregatorAttributeFromNative(n)
	assert.Equal(input.As, output.As)
	assert.Equal(input.Address, output.Address)
}

func Test_CommunitiesAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &CommunitiesAttribute{
		Communities: []uint32{100, 200},
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewCommunitiesAttributeFromNative(n)
	assert.Equal(input.Communities, output.Communities)
}

func Test_OriginatorIdAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &OriginatorIdAttribute{
		Id: "1.1.1.1",
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewOriginatorIdAttributeFromNative(n)
	assert.Equal(input.Id, output.Id)
}

func Test_ClusterListAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &ClusterListAttribute{
		Ids: []string{"1.1.1.1", "2.2.2.2"},
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewClusterListAttributeFromNative(n)
	assert.Equal(input.Ids, output.Ids)
}

func Test_MpReachNLRIAttribute_IPv4_UC(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 2)
	a, err := ptypes.MarshalAny(&IPAddressPrefix{
		PrefixLen: 24,
		Prefix:    "192.168.101.0",
	})
	assert.Nil(err)
	nlris = append(nlris, a)
	a, err = ptypes.MarshalAny(&IPAddressPrefix{
		PrefixLen: 24,
		Prefix:    "192.168.201.0",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_IPv4_UC),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(2, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_IPv6_UC(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 2)
	a, err := ptypes.MarshalAny(&IPAddressPrefix{
		PrefixLen: 64,
		Prefix:    "2001:db8:1::",
	})
	assert.Nil(err)
	nlris = append(nlris, a)
	a, err = ptypes.MarshalAny(&IPAddressPrefix{
		PrefixLen: 64,
		Prefix:    "2001:db8:2::",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_IPv6_UC),
		NextHops: []string{"2001:db8::1", "2001:db8::2"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(2, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_IPv4_MPLS(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 2)
	a, err := ptypes.MarshalAny(&LabeledIPAddressPrefix{
		Labels:    []uint32{100},
		PrefixLen: 24,
		Prefix:    "192.168.101.0",
	})
	assert.Nil(err)
	nlris = append(nlris, a)
	a, err = ptypes.MarshalAny(&LabeledIPAddressPrefix{
		Labels:    []uint32{200},
		PrefixLen: 24,
		Prefix:    "192.168.201.0",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_IPv4_MPLS),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(2, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_IPv6_MPLS(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 2)
	a, err := ptypes.MarshalAny(&LabeledIPAddressPrefix{
		Labels:    []uint32{100},
		PrefixLen: 64,
		Prefix:    "2001:db8:1::",
	})
	assert.Nil(err)
	nlris = append(nlris, a)
	a, err = ptypes.MarshalAny(&LabeledIPAddressPrefix{
		Labels:    []uint32{200},
		PrefixLen: 64,
		Prefix:    "2001:db8:2::",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_IPv6_MPLS),
		NextHops: []string{"2001:db8::1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(2, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_IPv4_ENCAP(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 2)
	a, err := ptypes.MarshalAny(&EncapsulationNLRI{
		Address: "192.168.101.1",
	})
	assert.Nil(err)
	nlris = append(nlris, a)
	a, err = ptypes.MarshalAny(&EncapsulationNLRI{
		Address: "192.168.201.1",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_IPv4_ENCAP),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(2, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_IPv6_ENCAP(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 2)
	a, err := ptypes.MarshalAny(&EncapsulationNLRI{
		Address: "2001:db8:1::1",
	})
	assert.Nil(err)
	nlris = append(nlris, a)
	a, err = ptypes.MarshalAny(&EncapsulationNLRI{
		Address: "2001:db8:2::1",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_IPv6_ENCAP),
		NextHops: []string{"2001:db8::1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(2, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_EVPN_AD_Route(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 1)
	rd, err := ptypes.MarshalAny(&RouteDistinguisherTwoOctetAS{
		Admin:    65000,
		Assigned: 100,
	})
	assert.Nil(err)
	esi := &EthernetSegmentIdentifier{
		Type:  0,
		Value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	a, err := ptypes.MarshalAny(&EVPNEthernetAutoDiscoveryRoute{
		Rd:          rd,
		Esi:         esi,
		EthernetTag: 100,
		Label:       200,
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_EVPN),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_EVPN_MAC_IP_Route(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 1)
	rd, err := ptypes.MarshalAny(&RouteDistinguisherIPAddress{
		Admin:    "1.1.1.1",
		Assigned: 100,
	})
	assert.Nil(err)
	esi := &EthernetSegmentIdentifier{
		Type:  0,
		Value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	a, err := ptypes.MarshalAny(&EVPNMACIPAdvertisementRoute{
		Rd:          rd,
		Esi:         esi,
		EthernetTag: 100,
		MacAddress:  "aa:bb:cc:dd:ee:ff",
		IpAddress:   "192.168.101.1",
		Labels:      []uint32{200},
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_EVPN),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_EVPN_MC_Route(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 1)
	rd, err := ptypes.MarshalAny(&RouteDistinguisherFourOctetAS{
		Admin:    65000,
		Assigned: 100,
	})
	assert.Nil(err)
	a, err := ptypes.MarshalAny(&EVPNInclusiveMulticastEthernetTagRoute{
		Rd:          rd,
		EthernetTag: 100,
		IpAddress:   "192.168.101.1",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_EVPN),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_EVPN_ES_Route(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 1)
	rd, err := ptypes.MarshalAny(&RouteDistinguisherIPAddress{
		Admin:    "1.1.1.1",
		Assigned: 100,
	})
	assert.Nil(err)
	esi := &EthernetSegmentIdentifier{
		Type:  0,
		Value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	a, err := ptypes.MarshalAny(&EVPNEthernetSegmentRoute{
		Rd:        rd,
		Esi:       esi,
		IpAddress: "192.168.101.1",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_EVPN),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_EVPN_Prefix_Route(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 1)
	rd, err := ptypes.MarshalAny(&RouteDistinguisherIPAddress{
		Admin:    "1.1.1.1",
		Assigned: 100,
	})
	assert.Nil(err)
	esi := &EthernetSegmentIdentifier{
		Type:  0,
		Value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	a, err := ptypes.MarshalAny(&EVPNIPPrefixRoute{
		Rd:          rd,
		Esi:         esi,
		EthernetTag: 100,
		IpPrefixLen: 24,
		IpPrefix:    "192.168.101.0",
		Label:       200,
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_EVPN),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_IPv4_VPN(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 1)
	rd, err := ptypes.MarshalAny(&RouteDistinguisherIPAddress{
		Admin:    "1.1.1.1",
		Assigned: 100,
	})
	assert.Nil(err)
	a, err := ptypes.MarshalAny(&LabeledVPNIPAddressPrefix{
		Labels:    []uint32{100, 200},
		Rd:        rd,
		PrefixLen: 24,
		Prefix:    "192.168.101.0",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_IPv4_VPN),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_IPv6_VPN(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 1)
	rd, err := ptypes.MarshalAny(&RouteDistinguisherIPAddress{
		Admin:    "1.1.1.1",
		Assigned: 100,
	})
	assert.Nil(err)
	a, err := ptypes.MarshalAny(&LabeledVPNIPAddressPrefix{
		Labels:    []uint32{100, 200},
		Rd:        rd,
		PrefixLen: 64,
		Prefix:    "2001:db8:1::",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_IPv6_VPN),
		NextHops: []string{"2001:db8::1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_RTC_UC(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 1)
	rt, err := ptypes.MarshalAny(&IPv4AddressSpecificExtended{
		IsTransitive: true,
		SubType:      0x02, // Route Target
		Address:      "1.1.1.1",
		LocalAdmin:   100,
	})
	assert.Nil(err)
	a, err := ptypes.MarshalAny(&RouteTargetMembershipNLRI{
		As: 65000,
		Rt: rt,
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family:   uint32(bgp.RF_RTC_UC),
		NextHops: []string{"192.168.1.1"},
		Nlris:    nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_FS_IPv4_UC(t *testing.T) {
	assert := assert.New(t)

	rules := make([]*any.Any, 0, 3)
	rule, err := ptypes.MarshalAny(&FlowSpecIPPrefix{
		Type:      1, // Destination Prefix
		PrefixLen: 24,
		Prefix:    "192.168.101.0",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecIPPrefix{
		Type:      2, // Source Prefix
		PrefixLen: 24,
		Prefix:    "192.168.201.0",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecComponent{
		Type: 3, // IP Protocol
		Items: []*FlowSpecComponentItem{
			{
				Op:    0x80 | 0x01, // End, EQ
				Value: 6,           // TCP
			},
		},
	})
	assert.Nil(err)
	rules = append(rules, rule)

	nlris := make([]*any.Any, 0, 1)
	a, err := ptypes.MarshalAny(&FlowSpecNLRI{
		Rules: rules,
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family: uint32(bgp.RF_FS_IPv4_UC),
		// NextHops: // No nexthop required
		Nlris: nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_FS_IPv4_VPN(t *testing.T) {
	assert := assert.New(t)

	rd, err := ptypes.MarshalAny(&RouteDistinguisherIPAddress{
		Admin:    "1.1.1.1",
		Assigned: 100,
	})
	assert.Nil(err)

	rules := make([]*any.Any, 0, 3)
	rule, err := ptypes.MarshalAny(&FlowSpecIPPrefix{
		Type:      1, // Destination Prefix
		PrefixLen: 24,
		Prefix:    "192.168.101.0",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecIPPrefix{
		Type:      2, // Source Prefix
		PrefixLen: 24,
		Prefix:    "192.168.201.0",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecComponent{
		Type: 3, // IP Protocol
		Items: []*FlowSpecComponentItem{
			{
				Op:    0x80 | 0x01, // End, EQ
				Value: 6,           // TCP
			},
		},
	})
	assert.Nil(err)
	rules = append(rules, rule)

	nlris := make([]*any.Any, 0, 1)
	a, err := ptypes.MarshalAny(&VPNFlowSpecNLRI{
		Rd:    rd,
		Rules: rules,
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family: uint32(bgp.RF_FS_IPv4_VPN),
		// NextHops: // No nexthop required
		Nlris: nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_FS_IPv6_UC(t *testing.T) {
	assert := assert.New(t)

	rules := make([]*any.Any, 0, 3)
	rule, err := ptypes.MarshalAny(&FlowSpecIPPrefix{
		Type:      1, // Destination Prefix
		PrefixLen: 64,
		Prefix:    "2001:db8:1::",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecIPPrefix{
		Type:      2, // Source Prefix
		PrefixLen: 64,
		Prefix:    "2001:db8:2::",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecComponent{
		Type: 3, // Next Header
		Items: []*FlowSpecComponentItem{
			{
				Op:    0x80 | 0x01, // End, EQ
				Value: 6,           // TCP
			},
		},
	})
	assert.Nil(err)
	rules = append(rules, rule)

	nlris := make([]*any.Any, 0, 1)
	a, err := ptypes.MarshalAny(&FlowSpecNLRI{
		Rules: rules,
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family: uint32(bgp.RF_FS_IPv6_UC),
		// NextHops: // No nexthop required
		Nlris: nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_FS_IPv6_VPN(t *testing.T) {
	assert := assert.New(t)

	rd, err := ptypes.MarshalAny(&RouteDistinguisherIPAddress{
		Admin:    "1.1.1.1",
		Assigned: 100,
	})
	assert.Nil(err)

	rules := make([]*any.Any, 0, 3)
	rule, err := ptypes.MarshalAny(&FlowSpecIPPrefix{
		Type:      1, // Destination Prefix
		PrefixLen: 64,
		Prefix:    "2001:db8:1::",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecIPPrefix{
		Type:      2, // Source Prefix
		PrefixLen: 64,
		Prefix:    "2001:db8:2::",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecComponent{
		Type: 3, // Next Header
		Items: []*FlowSpecComponentItem{
			{
				Op:    0x80 | 0x01, // End, EQ
				Value: 6,           // TCP
			},
		},
	})
	assert.Nil(err)
	rules = append(rules, rule)

	nlris := make([]*any.Any, 0, 1)
	a, err := ptypes.MarshalAny(&VPNFlowSpecNLRI{
		Rd:    rd,
		Rules: rules,
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family: uint32(bgp.RF_FS_IPv6_VPN),
		// NextHops: // No nexthop required
		Nlris: nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpReachNLRIAttribute_FS_L2_VPN(t *testing.T) {
	assert := assert.New(t)

	rd, err := ptypes.MarshalAny(&RouteDistinguisherIPAddress{
		Admin:    "1.1.1.1",
		Assigned: 100,
	})
	assert.Nil(err)

	rules := make([]*any.Any, 0, 3)
	rule, err := ptypes.MarshalAny(&FlowSpecMAC{
		Type:    15, // Source MAC
		Address: "aa:bb:cc:11:22:33",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecMAC{
		Type:    16, // Destination MAC
		Address: "dd:ee:ff:11:22:33",
	})
	assert.Nil(err)
	rules = append(rules, rule)
	rule, err = ptypes.MarshalAny(&FlowSpecComponent{
		Type: 21, // VLAN ID
		Items: []*FlowSpecComponentItem{
			{
				Op:    0x80 | 0x01, // End, EQ
				Value: 100,
			},
		},
	})
	assert.Nil(err)
	rules = append(rules, rule)

	nlris := make([]*any.Any, 0, 1)
	a, err := ptypes.MarshalAny(&VPNFlowSpecNLRI{
		Rd:    rd,
		Rules: rules,
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpReachNLRIAttribute{
		Family: uint32(bgp.RF_FS_L2_VPN),
		// NextHops: // No nexthop required
		Nlris: nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpReachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(input.NextHops, output.NextHops)
	assert.Equal(1, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_MpUnreachNLRIAttribute_IPv4_UC(t *testing.T) {
	assert := assert.New(t)

	nlris := make([]*any.Any, 0, 2)
	a, err := ptypes.MarshalAny(&IPAddressPrefix{
		PrefixLen: 24,
		Prefix:    "192.168.101.0",
	})
	assert.Nil(err)
	nlris = append(nlris, a)
	a, err = ptypes.MarshalAny(&IPAddressPrefix{
		PrefixLen: 24,
		Prefix:    "192.168.201.0",
	})
	assert.Nil(err)
	nlris = append(nlris, a)

	input := &MpUnreachNLRIAttribute{
		Family: uint32(bgp.RF_IPv4_UC),
		Nlris:  nlris,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewMpUnreachNLRIAttributeFromNative(n)
	assert.Equal(input.Family, output.Family)
	assert.Equal(2, len(output.Nlris))
	for idx, inputNLRI := range input.Nlris {
		outputNLRI := output.Nlris[idx]
		assert.Equal(inputNLRI.TypeUrl, outputNLRI.TypeUrl)
		assert.Equal(inputNLRI.Value, outputNLRI.Value)
	}
}

func Test_ExtendedCommunitiesAttribute(t *testing.T) {
	assert := assert.New(t)

	communities := make([]*any.Any, 0, 19)
	a, err := ptypes.MarshalAny(&TwoOctetAsSpecificExtended{
		IsTransitive: true,
		SubType:      0x02, // ROUTE_TARGET
		As:           65001,
		LocalAdmin:   100,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&IPv4AddressSpecificExtended{
		IsTransitive: true,
		SubType:      0x02, // ROUTE_TARGET
		Address:      "2.2.2.2",
		LocalAdmin:   200,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&FourOctetAsSpecificExtended{
		IsTransitive: true,
		SubType:      0x02, // ROUTE_TARGET
		As:           65003,
		LocalAdmin:   300,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&ValidationExtended{
		State: 0, // VALID
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&ColorExtended{
		Color: 400,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&EncapExtended{
		TunnelType: 8, // VXLAN
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&DefaultGatewayExtended{
		// No value
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&OpaqueExtended{
		IsTransitive: true,
		Value:        []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77},
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&ESILabelExtended{
		IsSingleActive: true,
		Label:          500,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&ESImportRouteTarget{
		EsImport: "aa:bb:cc:dd:ee:ff",
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&MacMobilityExtended{
		IsSticky:    true,
		SequenceNum: 1,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&RouterMacExtended{
		Mac: "ff:ee:dd:cc:bb:aa",
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&TrafficRateExtended{
		As:   65004,
		Rate: 100.0,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&TrafficActionExtended{
		Terminal: true,
		Sample:   false,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&RedirectTwoOctetAsSpecificExtended{
		As:         65005,
		LocalAdmin: 500,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&RedirectIPv4AddressSpecificExtended{
		Address:    "6.6.6.6",
		LocalAdmin: 600,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&RedirectFourOctetAsSpecificExtended{
		As:         65007,
		LocalAdmin: 700,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&TrafficRemarkExtended{
		Dscp: 0x0a, // AF11
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&UnknownExtended{
		Type:  0xff, // Max of uint8
		Value: []byte{1, 2, 3, 4, 5, 6, 7},
	})
	assert.Nil(err)
	communities = append(communities, a)

	input := &ExtendedCommunitiesAttribute{
		Communities: communities,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewExtendedCommunitiesAttributeFromNative(n)
	assert.Equal(19, len(output.Communities))
	for idx, inputCommunity := range input.Communities {
		outputCommunity := output.Communities[idx]
		assert.Equal(inputCommunity.TypeUrl, outputCommunity.TypeUrl)
		assert.Equal(inputCommunity.Value, outputCommunity.Value)
	}
}

func Test_As4PathAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &As4PathAttribute{
		Segments: []*AsSegment{
			{
				Type:    1, // SET
				Numbers: []uint32{100, 200},
			},
			{
				Type:    2, // SEQ
				Numbers: []uint32{300, 400},
			},
		},
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewAs4PathAttributeFromNative(n)
	assert.Equal(2, len(output.Segments))
	assert.Equal(input.Segments, output.Segments)
}

func Test_As4AggregatorAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &As4AggregatorAttribute{
		As:      65000,
		Address: "1.1.1.1",
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewAs4AggregatorAttributeFromNative(n)
	assert.Equal(input.As, output.As)
	assert.Equal(input.Address, output.Address)
}

func Test_PmsiTunnelAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &PmsiTunnelAttribute{
		Flags: 0x01, // IsLeafInfoRequired = true
		Type:  6,    // INGRESS_REPL
		Label: 100,
		Id:    net.ParseIP("1.1.1.1").To4(), // IngressReplTunnelID with IPv4
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewPmsiTunnelAttributeFromNative(n)
	assert.Equal(input.Flags, output.Flags)
	assert.Equal(input.Type, output.Type)
	assert.Equal(input.Label, output.Label)
	assert.Equal(input.Id, output.Id)
}

func Test_TunnelEncapAttribute(t *testing.T) {
	assert := assert.New(t)

	subTlvs := make([]*any.Any, 0, 4)
	a, err := ptypes.MarshalAny(&TunnelEncapSubTLVEncapsulation{
		Key:    100,
		Cookie: []byte{0x11, 0x22, 0x33, 0x44},
	})
	assert.Nil(err)
	subTlvs = append(subTlvs, a)
	a, err = ptypes.MarshalAny(&TunnelEncapSubTLVProtocol{
		Protocol: 200,
	})
	assert.Nil(err)
	subTlvs = append(subTlvs, a)
	a, err = ptypes.MarshalAny(&TunnelEncapSubTLVColor{
		Color: 300,
	})
	assert.Nil(err)
	subTlvs = append(subTlvs, a)
	a, err = ptypes.MarshalAny(&TunnelEncapSubTLVUnknown{
		Type:  0xff, // Max of uint8
		Value: []byte{0x55, 0x66, 0x77, 0x88},
	})
	assert.Nil(err)
	subTlvs = append(subTlvs, a)

	input := &TunnelEncapAttribute{
		Tlvs: []*TunnelEncapTLV{
			{
				Type: 8, // VXLAN
				Tlvs: subTlvs,
			},
		},
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewTunnelEncapAttributeFromNative(n)
	assert.Equal(1, len(output.Tlvs))
	assert.Equal(input.Tlvs[0].Type, output.Tlvs[0].Type)
	assert.Equal(len(output.Tlvs[0].Tlvs), len(output.Tlvs[0].Tlvs))
	for idx, inputSubTlv := range input.Tlvs[0].Tlvs {
		outputSubTlv := output.Tlvs[0].Tlvs[idx]
		assert.Equal(inputSubTlv.TypeUrl, outputSubTlv.TypeUrl)
		assert.Equal(inputSubTlv.Value, outputSubTlv.Value)
	}
}

func Test_IP6ExtendedCommunitiesAttribute(t *testing.T) {
	assert := assert.New(t)

	communities := make([]*any.Any, 0, 2)
	a, err := ptypes.MarshalAny(&IPv6AddressSpecificExtended{
		IsTransitive: true,
		SubType:      0xff, // Max of uint8
		Address:      "2001:db8:1::1",
		LocalAdmin:   100,
	})
	assert.Nil(err)
	communities = append(communities, a)
	a, err = ptypes.MarshalAny(&RedirectIPv6AddressSpecificExtended{
		Address:    "2001:db8:2::1",
		LocalAdmin: 200,
	})
	assert.Nil(err)
	communities = append(communities, a)

	input := &IP6ExtendedCommunitiesAttribute{
		Communities: communities,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewIP6ExtendedCommunitiesAttributeFromNative(n)
	assert.Equal(2, len(output.Communities))
	for idx, inputCommunity := range input.Communities {
		outputCommunity := output.Communities[idx]
		assert.Equal(inputCommunity.TypeUrl, outputCommunity.TypeUrl)
		assert.Equal(inputCommunity.Value, outputCommunity.Value)
	}
}

func Test_AigpAttribute(t *testing.T) {
	assert := assert.New(t)

	tlvs := make([]*any.Any, 0, 2)
	a, err := ptypes.MarshalAny(&AigpTLVIGPMetric{
		Metric: 50,
	})
	assert.Nil(err)
	tlvs = append(tlvs, a)
	a, err = ptypes.MarshalAny(&AigpTLVUnknown{
		Type:  0xff, // Max of uint8
		Value: []byte{0x11, 0x22, 0x33, 0x44},
	})
	assert.Nil(err)
	tlvs = append(tlvs, a)

	input := &AigpAttribute{
		Tlvs: tlvs,
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewAigpAttributeFromNative(n)
	assert.Equal(2, len(output.Tlvs))
	for idx, inputTlv := range input.Tlvs {
		outputTlv := output.Tlvs[idx]
		assert.Equal(inputTlv.TypeUrl, outputTlv.TypeUrl)
		assert.Equal(inputTlv.Value, outputTlv.Value)
	}
}

func Test_LargeCommunitiesAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &LargeCommunitiesAttribute{
		Communities: []*LargeCommunity{
			{
				GlobalAdmin: 65001,
				LocalData1:  100,
				LocalData2:  200,
			},
			{
				GlobalAdmin: 65002,
				LocalData1:  300,
				LocalData2:  400,
			},
		},
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewLargeCommunitiesAttributeFromNative(n)
	assert.Equal(2, len(output.Communities))
	assert.Equal(input.Communities, output.Communities)
}

func Test_UnknownAttribute(t *testing.T) {
	assert := assert.New(t)

	input := &UnknownAttribute{
		Flags: (1 << 6) | (1 << 7), // OPTIONAL and TRANSITIVE
		Type:  0xff,
		Value: []byte{0x11, 0x22, 0x33, 0x44},
	}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewUnknownAttributeFromNative(n)
	assert.Equal(input.Flags, output.Flags)
	assert.Equal(input.Type, output.Type)
	assert.Equal(input.Value, output.Value)
}
