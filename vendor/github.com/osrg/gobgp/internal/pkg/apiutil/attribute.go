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

package apiutil

import (
	"errors"
	"fmt"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/pkg/packet/bgp"
	log "github.com/sirupsen/logrus"
)

func UnmarshalAttribute(an *any.Any) (bgp.PathAttributeInterface, error) {
	var value ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(an, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal route distinguisher: %s", err)
	}
	switch a := value.Message.(type) {
	case *api.OriginAttribute:
		return bgp.NewPathAttributeOrigin(uint8(a.Origin)), nil
	case *api.AsPathAttribute:
		params := make([]bgp.AsPathParamInterface, 0, len(a.Segments))
		for _, segment := range a.Segments {
			params = append(params, bgp.NewAs4PathParam(uint8(segment.Type), segment.Numbers))
		}
		return bgp.NewPathAttributeAsPath(params), nil
	case *api.NextHopAttribute:
		nexthop := net.ParseIP(a.NextHop).To4()
		if nexthop == nil {
			return nil, fmt.Errorf("invalid nexthop address: %s", a.NextHop)
		}
		return bgp.NewPathAttributeNextHop(a.NextHop), nil
	case *api.MultiExitDiscAttribute:
		return bgp.NewPathAttributeMultiExitDisc(a.Med), nil
	case *api.LocalPrefAttribute:
		return bgp.NewPathAttributeLocalPref(a.LocalPref), nil
	case *api.AtomicAggregateAttribute:
		return bgp.NewPathAttributeAtomicAggregate(), nil
	case *api.AggregatorAttribute:
		if net.ParseIP(a.Address).To4() == nil {
			return nil, fmt.Errorf("invalid aggregator address: %s", a.Address)
		}
		return bgp.NewPathAttributeAggregator(a.As, a.Address), nil
	case *api.CommunitiesAttribute:
		return bgp.NewPathAttributeCommunities(a.Communities), nil
	case *api.OriginatorIdAttribute:
		if net.ParseIP(a.Id).To4() == nil {
			return nil, fmt.Errorf("invalid originator id: %s", a.Id)
		}
		return bgp.NewPathAttributeOriginatorId(a.Id), nil
	case *api.ClusterListAttribute:
		for _, id := range a.Ids {
			if net.ParseIP(id).To4() == nil {
				return nil, fmt.Errorf("invalid cluster list: %s", a.Ids)
			}
		}
		return bgp.NewPathAttributeClusterList(a.Ids), nil
	}
	return nil, errors.New("unexpected object")
}

func NewOriginAttributeFromNative(a *bgp.PathAttributeOrigin) *api.OriginAttribute {
	return &api.OriginAttribute{
		Origin: uint32(a.Value),
	}
}

func NewAsPathAttributeFromNative(a *bgp.PathAttributeAsPath) *api.AsPathAttribute {
	segments := make([]*api.AsSegment, 0, len(a.Value))
	for _, param := range a.Value {
		segments = append(segments, &api.AsSegment{
			Type:    uint32(param.GetType()),
			Numbers: param.GetAS(),
		})
	}
	return &api.AsPathAttribute{
		Segments: segments,
	}
}

func NewNextHopAttributeFromNative(a *bgp.PathAttributeNextHop) *api.NextHopAttribute {
	return &api.NextHopAttribute{
		NextHop: a.Value.String(),
	}
}

func NewMultiExitDiscAttributeFromNative(a *bgp.PathAttributeMultiExitDisc) *api.MultiExitDiscAttribute {
	return &api.MultiExitDiscAttribute{
		Med: a.Value,
	}
}

func NewLocalPrefAttributeFromNative(a *bgp.PathAttributeLocalPref) *api.LocalPrefAttribute {
	return &api.LocalPrefAttribute{
		LocalPref: a.Value,
	}
}

func NewAtomicAggregateAttributeFromNative(a *bgp.PathAttributeAtomicAggregate) *api.AtomicAggregateAttribute {
	return &api.AtomicAggregateAttribute{}
}

func NewAggregatorAttributeFromNative(a *bgp.PathAttributeAggregator) *api.AggregatorAttribute {
	return &api.AggregatorAttribute{
		As:      a.Value.AS,
		Address: a.Value.Address.String(),
	}
}

func NewCommunitiesAttributeFromNative(a *bgp.PathAttributeCommunities) *api.CommunitiesAttribute {
	return &api.CommunitiesAttribute{
		Communities: a.Value,
	}
}

func NewOriginatorIdAttributeFromNative(a *bgp.PathAttributeOriginatorId) *api.OriginatorIdAttribute {
	return &api.OriginatorIdAttribute{
		Id: a.Value.String(),
	}
}

func NewClusterListAttributeFromNative(a *bgp.PathAttributeClusterList) *api.ClusterListAttribute {
	ids := make([]string, 0, len(a.Value))
	for _, id := range a.Value {
		ids = append(ids, id.String())
	}
	return &api.ClusterListAttribute{
		Ids: ids,
	}
}

func MarshalRD(rd bgp.RouteDistinguisherInterface) *any.Any {
	var r proto.Message
	switch v := rd.(type) {
	case *bgp.RouteDistinguisherTwoOctetAS:
		r = &api.RouteDistinguisherTwoOctetAS{
			Admin:    uint32(v.Admin),
			Assigned: v.Assigned,
		}
	case *bgp.RouteDistinguisherIPAddressAS:
		r = &api.RouteDistinguisherIPAddress{
			Admin:    v.Admin.String(),
			Assigned: uint32(v.Assigned),
		}
	case *bgp.RouteDistinguisherFourOctetAS:
		r = &api.RouteDistinguisherFourOctetAS{
			Admin:    v.Admin,
			Assigned: uint32(v.Assigned),
		}
	default:
		log.WithFields(log.Fields{
			"Topic": "protobuf",
			"RD":    rd,
		}).Warn("invalid rd type to marshal")
		return nil
	}
	a, _ := ptypes.MarshalAny(r)
	return a
}

func UnmarshalRD(a *any.Any) (bgp.RouteDistinguisherInterface, error) {
	var value ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(a, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal route distinguisher: %s", err)
	}
	switch v := value.Message.(type) {
	case *api.RouteDistinguisherTwoOctetAS:
		return bgp.NewRouteDistinguisherTwoOctetAS(uint16(v.Admin), v.Assigned), nil
	case *api.RouteDistinguisherIPAddress:
		rd := bgp.NewRouteDistinguisherIPAddressAS(v.Admin, uint16(v.Assigned))
		if rd == nil {
			return nil, fmt.Errorf("invalid address for route distinguisher: %s", v.Admin)
		}
		return rd, nil
	case *api.RouteDistinguisherFourOctetAS:
		return bgp.NewRouteDistinguisherFourOctetAS(v.Admin, uint16(v.Assigned)), nil
	}
	return nil, fmt.Errorf("invalid route distinguisher type: %s", a.TypeUrl)
}

func NewEthernetSegmentIdentifierFromNative(a *bgp.EthernetSegmentIdentifier) *api.EthernetSegmentIdentifier {
	return &api.EthernetSegmentIdentifier{
		Type:  uint32(a.Type),
		Value: a.Value,
	}
}

func unmarshalESI(a *api.EthernetSegmentIdentifier) (*bgp.EthernetSegmentIdentifier, error) {
	return &bgp.EthernetSegmentIdentifier{
		Type:  bgp.ESIType(a.Type),
		Value: a.Value,
	}, nil
}

func MarshalFlowSpecRules(values []bgp.FlowSpecComponentInterface) []*any.Any {
	rules := make([]*any.Any, 0, len(values))
	for _, value := range values {
		var rule proto.Message
		switch v := value.(type) {
		case *bgp.FlowSpecDestinationPrefix:
			rule = &api.FlowSpecIPPrefix{
				Type:      uint32(bgp.FLOW_SPEC_TYPE_DST_PREFIX),
				PrefixLen: uint32(v.Prefix.(*bgp.IPAddrPrefix).Length),
				Prefix:    v.Prefix.(*bgp.IPAddrPrefix).Prefix.String(),
			}
		case *bgp.FlowSpecSourcePrefix:
			rule = &api.FlowSpecIPPrefix{
				Type:      uint32(bgp.FLOW_SPEC_TYPE_SRC_PREFIX),
				PrefixLen: uint32(v.Prefix.(*bgp.IPAddrPrefix).Length),
				Prefix:    v.Prefix.(*bgp.IPAddrPrefix).Prefix.String(),
			}
		case *bgp.FlowSpecDestinationPrefix6:
			rule = &api.FlowSpecIPPrefix{
				Type:      uint32(bgp.FLOW_SPEC_TYPE_DST_PREFIX),
				PrefixLen: uint32(v.Prefix.(*bgp.IPv6AddrPrefix).Length),
				Prefix:    v.Prefix.(*bgp.IPv6AddrPrefix).Prefix.String(),
				Offset:    uint32(v.Offset),
			}
		case *bgp.FlowSpecSourcePrefix6:
			rule = &api.FlowSpecIPPrefix{
				Type:      uint32(bgp.FLOW_SPEC_TYPE_SRC_PREFIX),
				PrefixLen: uint32(v.Prefix.(*bgp.IPv6AddrPrefix).Length),
				Prefix:    v.Prefix.(*bgp.IPv6AddrPrefix).Prefix.String(),
				Offset:    uint32(v.Offset),
			}
		case *bgp.FlowSpecSourceMac:
			rule = &api.FlowSpecMAC{
				Type:    uint32(bgp.FLOW_SPEC_TYPE_SRC_MAC),
				Address: v.Mac.String(),
			}
		case *bgp.FlowSpecDestinationMac:
			rule = &api.FlowSpecMAC{
				Type:    uint32(bgp.FLOW_SPEC_TYPE_DST_MAC),
				Address: v.Mac.String(),
			}
		case *bgp.FlowSpecComponent:
			items := make([]*api.FlowSpecComponentItem, 0, len(v.Items))
			for _, i := range v.Items {
				items = append(items, &api.FlowSpecComponentItem{
					Op:    uint32(i.Op),
					Value: i.Value,
				})
			}
			rule = &api.FlowSpecComponent{
				Type:  uint32(v.Type()),
				Items: items,
			}
		}
		a, _ := ptypes.MarshalAny(rule)
		rules = append(rules, a)
	}
	return rules
}

func UnmarshalFlowSpecRules(values []*any.Any) ([]bgp.FlowSpecComponentInterface, error) {
	rules := make([]bgp.FlowSpecComponentInterface, 0, len(values))
	for _, an := range values {
		var rule bgp.FlowSpecComponentInterface
		var value ptypes.DynamicAny
		if err := ptypes.UnmarshalAny(an, &value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal flow spec component: %s", err)
		}
		switch v := value.Message.(type) {
		case *api.FlowSpecIPPrefix:
			typ := bgp.BGPFlowSpecType(v.Type)
			isIPv4 := net.ParseIP(v.Prefix).To4() != nil
			switch {
			case typ == bgp.FLOW_SPEC_TYPE_DST_PREFIX && isIPv4:
				rule = bgp.NewFlowSpecDestinationPrefix(bgp.NewIPAddrPrefix(uint8(v.PrefixLen), v.Prefix))
			case typ == bgp.FLOW_SPEC_TYPE_SRC_PREFIX && isIPv4:
				rule = bgp.NewFlowSpecSourcePrefix(bgp.NewIPAddrPrefix(uint8(v.PrefixLen), v.Prefix))
			case typ == bgp.FLOW_SPEC_TYPE_DST_PREFIX && !isIPv4:
				rule = bgp.NewFlowSpecDestinationPrefix6(bgp.NewIPv6AddrPrefix(uint8(v.PrefixLen), v.Prefix), uint8(v.Offset))
			case typ == bgp.FLOW_SPEC_TYPE_SRC_PREFIX && !isIPv4:
				rule = bgp.NewFlowSpecSourcePrefix6(bgp.NewIPv6AddrPrefix(uint8(v.PrefixLen), v.Prefix), uint8(v.Offset))
			}
		case *api.FlowSpecMAC:
			typ := bgp.BGPFlowSpecType(v.Type)
			mac, err := net.ParseMAC(v.Address)
			if err != nil {
				return nil, fmt.Errorf("invalid mac address for %s flow spec component: %s", typ.String(), v.Address)
			}
			switch typ {
			case bgp.FLOW_SPEC_TYPE_SRC_MAC:
				rule = bgp.NewFlowSpecSourceMac(mac)
			case bgp.FLOW_SPEC_TYPE_DST_MAC:
				rule = bgp.NewFlowSpecDestinationMac(mac)
			}
		case *api.FlowSpecComponent:
			items := make([]*bgp.FlowSpecComponentItem, 0, len(v.Items))
			for _, item := range v.Items {
				items = append(items, bgp.NewFlowSpecComponentItem(uint8(item.Op), item.Value))
			}
			rule = bgp.NewFlowSpecComponent(bgp.BGPFlowSpecType(v.Type), items)
		}
		if rule == nil {
			return nil, fmt.Errorf("invalid flow spec component: %v", value.Message)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func MarshalNLRI(value bgp.AddrPrefixInterface) *any.Any {
	var nlri proto.Message

	switch v := value.(type) {
	case *bgp.IPAddrPrefix:
		nlri = &api.IPAddressPrefix{
			PrefixLen: uint32(v.Length),
			Prefix:    v.Prefix.String(),
		}
	case *bgp.IPv6AddrPrefix:
		nlri = &api.IPAddressPrefix{
			PrefixLen: uint32(v.Length),
			Prefix:    v.Prefix.String(),
		}
	case *bgp.LabeledIPAddrPrefix:
		nlri = &api.LabeledIPAddressPrefix{
			Labels:    v.Labels.Labels,
			PrefixLen: uint32(v.IPPrefixLen()),
			Prefix:    v.Prefix.String(),
		}
	case *bgp.LabeledIPv6AddrPrefix:
		nlri = &api.LabeledIPAddressPrefix{
			Labels:    v.Labels.Labels,
			PrefixLen: uint32(v.IPPrefixLen()),
			Prefix:    v.Prefix.String(),
		}
	case *bgp.EncapNLRI:
		nlri = &api.EncapsulationNLRI{
			Address: v.String(),
		}
	case *bgp.Encapv6NLRI:
		nlri = &api.EncapsulationNLRI{
			Address: v.String(),
		}
	case *bgp.EVPNNLRI:
		switch r := v.RouteTypeData.(type) {
		case *bgp.EVPNEthernetAutoDiscoveryRoute:
			nlri = &api.EVPNEthernetAutoDiscoveryRoute{
				Rd:          MarshalRD(r.RD),
				Esi:         NewEthernetSegmentIdentifierFromNative(&r.ESI),
				EthernetTag: r.ETag,
				Label:       r.Label,
			}
		case *bgp.EVPNMacIPAdvertisementRoute:
			nlri = &api.EVPNMACIPAdvertisementRoute{
				Rd:          MarshalRD(r.RD),
				Esi:         NewEthernetSegmentIdentifierFromNative(&r.ESI),
				EthernetTag: r.ETag,
				MacAddress:  r.MacAddress.String(),
				IpAddress:   r.IPAddress.String(),
				Labels:      r.Labels,
			}
		case *bgp.EVPNMulticastEthernetTagRoute:
			nlri = &api.EVPNInclusiveMulticastEthernetTagRoute{
				Rd:          MarshalRD(r.RD),
				EthernetTag: r.ETag,
				IpAddress:   r.IPAddress.String(),
			}
		case *bgp.EVPNEthernetSegmentRoute:
			nlri = &api.EVPNEthernetSegmentRoute{
				Rd:        MarshalRD(r.RD),
				Esi:       NewEthernetSegmentIdentifierFromNative(&r.ESI),
				IpAddress: r.IPAddress.String(),
			}
		case *bgp.EVPNIPPrefixRoute:
			nlri = &api.EVPNIPPrefixRoute{
				Rd:          MarshalRD(r.RD),
				Esi:         NewEthernetSegmentIdentifierFromNative(&r.ESI),
				EthernetTag: r.ETag,
				IpPrefix:    r.IPPrefix.String(),
				IpPrefixLen: uint32(r.IPPrefixLength),
				Label:       r.Label,
				GwAddress:   r.GWIPAddress.String(),
			}
		}
	case *bgp.LabeledVPNIPAddrPrefix:
		nlri = &api.LabeledVPNIPAddressPrefix{
			Labels:    v.Labels.Labels,
			Rd:        MarshalRD(v.RD),
			PrefixLen: uint32(v.IPPrefixLen()),
			Prefix:    v.Prefix.String(),
		}
	case *bgp.LabeledVPNIPv6AddrPrefix:
		nlri = &api.LabeledVPNIPAddressPrefix{
			Labels:    v.Labels.Labels,
			Rd:        MarshalRD(v.RD),
			PrefixLen: uint32(v.IPPrefixLen()),
			Prefix:    v.Prefix.String(),
		}
	case *bgp.RouteTargetMembershipNLRI:
		nlri = &api.RouteTargetMembershipNLRI{
			As: v.AS,
			Rt: MarshalRT(v.RouteTarget),
		}
	case *bgp.FlowSpecIPv4Unicast:
		nlri = &api.FlowSpecNLRI{
			Rules: MarshalFlowSpecRules(v.Value),
		}
	case *bgp.FlowSpecIPv6Unicast:
		nlri = &api.FlowSpecNLRI{
			Rules: MarshalFlowSpecRules(v.Value),
		}
	case *bgp.FlowSpecIPv4VPN:
		nlri = &api.VPNFlowSpecNLRI{
			Rd:    MarshalRD(v.RD()),
			Rules: MarshalFlowSpecRules(v.Value),
		}
	case *bgp.FlowSpecIPv6VPN:
		nlri = &api.VPNFlowSpecNLRI{
			Rd:    MarshalRD(v.RD()),
			Rules: MarshalFlowSpecRules(v.Value),
		}
	case *bgp.FlowSpecL2VPN:
		nlri = &api.VPNFlowSpecNLRI{
			Rd:    MarshalRD(v.RD()),
			Rules: MarshalFlowSpecRules(v.Value),
		}
	}

	an, _ := ptypes.MarshalAny(nlri)
	return an
}

func MarshalNLRIs(values []bgp.AddrPrefixInterface) []*any.Any {
	nlris := make([]*any.Any, 0, len(values))
	for _, value := range values {
		nlris = append(nlris, MarshalNLRI(value))
	}
	return nlris
}

func UnmarshalNLRI(rf bgp.RouteFamily, an *any.Any) (bgp.AddrPrefixInterface, error) {
	var nlri bgp.AddrPrefixInterface

	var value ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(an, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nlri: %s", err)
	}

	switch v := value.Message.(type) {
	case *api.IPAddressPrefix:
		switch rf {
		case bgp.RF_IPv4_UC:
			nlri = bgp.NewIPAddrPrefix(uint8(v.PrefixLen), v.Prefix)
		case bgp.RF_IPv6_UC:
			nlri = bgp.NewIPv6AddrPrefix(uint8(v.PrefixLen), v.Prefix)
		}
	case *api.LabeledIPAddressPrefix:
		switch rf {
		case bgp.RF_IPv4_MPLS:
			nlri = bgp.NewLabeledIPAddrPrefix(uint8(v.PrefixLen), v.Prefix, *bgp.NewMPLSLabelStack(v.Labels...))
		case bgp.RF_IPv6_MPLS:
			nlri = bgp.NewLabeledIPv6AddrPrefix(uint8(v.PrefixLen), v.Prefix, *bgp.NewMPLSLabelStack(v.Labels...))
		}
	case *api.EncapsulationNLRI:
		switch rf {
		case bgp.RF_IPv4_ENCAP:
			nlri = bgp.NewEncapNLRI(v.Address)
		case bgp.RF_IPv6_ENCAP:
			nlri = bgp.NewEncapv6NLRI(v.Address)
		}
	case *api.EVPNEthernetAutoDiscoveryRoute:
		if rf == bgp.RF_EVPN {
			rd, err := UnmarshalRD(v.Rd)
			if err != nil {
				return nil, err
			}
			esi, err := unmarshalESI(v.Esi)
			if err != nil {
				return nil, err
			}
			nlri = bgp.NewEVPNEthernetAutoDiscoveryRoute(rd, *esi, v.EthernetTag, v.Label)
		}
	case *api.EVPNMACIPAdvertisementRoute:
		if rf == bgp.RF_EVPN {
			rd, err := UnmarshalRD(v.Rd)
			if err != nil {
				return nil, err
			}
			esi, err := unmarshalESI(v.Esi)
			if err != nil {
				return nil, err
			}
			nlri = bgp.NewEVPNMacIPAdvertisementRoute(rd, *esi, v.EthernetTag, v.MacAddress, v.IpAddress, v.Labels)
		}
	case *api.EVPNInclusiveMulticastEthernetTagRoute:
		if rf == bgp.RF_EVPN {
			rd, err := UnmarshalRD(v.Rd)
			if err != nil {
				return nil, err
			}
			nlri = bgp.NewEVPNMulticastEthernetTagRoute(rd, v.EthernetTag, v.IpAddress)
		}
	case *api.EVPNEthernetSegmentRoute:
		if rf == bgp.RF_EVPN {
			rd, err := UnmarshalRD(v.Rd)
			if err != nil {
				return nil, err
			}
			esi, err := unmarshalESI(v.Esi)
			if err != nil {
				return nil, err
			}
			nlri = bgp.NewEVPNEthernetSegmentRoute(rd, *esi, v.IpAddress)
		}
	case *api.EVPNIPPrefixRoute:
		if rf == bgp.RF_EVPN {
			rd, err := UnmarshalRD(v.Rd)
			if err != nil {
				return nil, err
			}
			esi, err := unmarshalESI(v.Esi)
			if err != nil {
				return nil, err
			}
			nlri = bgp.NewEVPNIPPrefixRoute(rd, *esi, v.EthernetTag, uint8(v.IpPrefixLen), v.IpPrefix, v.GwAddress, v.Label)
		}
	case *api.LabeledVPNIPAddressPrefix:
		rd, err := UnmarshalRD(v.Rd)
		if err != nil {
			return nil, err
		}
		switch rf {
		case bgp.RF_IPv4_VPN:
			nlri = bgp.NewLabeledVPNIPAddrPrefix(uint8(v.PrefixLen), v.Prefix, *bgp.NewMPLSLabelStack(v.Labels...), rd)
		case bgp.RF_IPv6_VPN:
			nlri = bgp.NewLabeledVPNIPv6AddrPrefix(uint8(v.PrefixLen), v.Prefix, *bgp.NewMPLSLabelStack(v.Labels...), rd)
		}
	case *api.RouteTargetMembershipNLRI:
		rt, err := UnmarshalRT(v.Rt)
		if err != nil {
			return nil, err
		}
		nlri = bgp.NewRouteTargetMembershipNLRI(v.As, rt)
	case *api.FlowSpecNLRI:
		rules, err := UnmarshalFlowSpecRules(v.Rules)
		if err != nil {
			return nil, err
		}
		switch rf {
		case bgp.RF_FS_IPv4_UC:
			nlri = bgp.NewFlowSpecIPv4Unicast(rules)
		case bgp.RF_FS_IPv6_UC:
			nlri = bgp.NewFlowSpecIPv6Unicast(rules)
		}
	case *api.VPNFlowSpecNLRI:
		rd, err := UnmarshalRD(v.Rd)
		if err != nil {
			return nil, err
		}
		rules, err := UnmarshalFlowSpecRules(v.Rules)
		if err != nil {
			return nil, err
		}
		switch rf {
		case bgp.RF_FS_IPv4_VPN:
			nlri = bgp.NewFlowSpecIPv4VPN(rd, rules)
		case bgp.RF_FS_IPv6_VPN:
			nlri = bgp.NewFlowSpecIPv6VPN(rd, rules)
		case bgp.RF_FS_L2_VPN:
			nlri = bgp.NewFlowSpecL2VPN(rd, rules)
		}
	}

	if nlri == nil {
		return nil, fmt.Errorf("invalid nlri for %s family: %s", rf.String(), value.Message)
	}

	return nlri, nil
}

func UnmarshalNLRIs(rf bgp.RouteFamily, values []*any.Any) ([]bgp.AddrPrefixInterface, error) {
	nlris := make([]bgp.AddrPrefixInterface, 0, len(values))
	for _, an := range values {
		nlri, err := UnmarshalNLRI(rf, an)
		if err != nil {
			return nil, err
		}
		nlris = append(nlris, nlri)
	}
	return nlris, nil
}

func NewMpReachNLRIAttributeFromNative(a *bgp.PathAttributeMpReachNLRI) *api.MpReachNLRIAttribute {
	var nexthops []string
	if a.SAFI == bgp.SAFI_FLOW_SPEC_UNICAST || a.SAFI == bgp.SAFI_FLOW_SPEC_VPN {
		nexthops = nil
	} else {
		nexthops = []string{a.Nexthop.String()}
		if a.LinkLocalNexthop != nil {
			nexthops = append(nexthops, a.LinkLocalNexthop.String())
		}
	}
	return &api.MpReachNLRIAttribute{
		Family:   ToApiFamily(a.AFI, a.SAFI),
		NextHops: nexthops,
		Nlris:    MarshalNLRIs(a.Value),
	}
}

func NewMpUnreachNLRIAttributeFromNative(a *bgp.PathAttributeMpUnreachNLRI) *api.MpUnreachNLRIAttribute {
	return &api.MpUnreachNLRIAttribute{
		Family: ToApiFamily(a.AFI, a.SAFI),
		Nlris:  MarshalNLRIs(a.Value),
	}
}

func MarshalRT(rt bgp.ExtendedCommunityInterface) *any.Any {
	var r proto.Message
	switch v := rt.(type) {
	case *bgp.TwoOctetAsSpecificExtended:
		r = &api.TwoOctetAsSpecificExtended{
			IsTransitive: true,
			SubType:      uint32(bgp.EC_SUBTYPE_ROUTE_TARGET),
			As:           uint32(v.AS),
			LocalAdmin:   uint32(v.LocalAdmin),
		}
	case *bgp.IPv4AddressSpecificExtended:
		r = &api.IPv4AddressSpecificExtended{
			IsTransitive: true,
			SubType:      uint32(bgp.EC_SUBTYPE_ROUTE_TARGET),
			Address:      v.IPv4.String(),
			LocalAdmin:   uint32(v.LocalAdmin),
		}
	case *bgp.FourOctetAsSpecificExtended:
		r = &api.FourOctetAsSpecificExtended{
			IsTransitive: true,
			SubType:      uint32(bgp.EC_SUBTYPE_ROUTE_TARGET),
			As:           uint32(v.AS),
			LocalAdmin:   uint32(v.LocalAdmin),
		}
	default:
		log.WithFields(log.Fields{
			"Topic": "protobuf",
			"RT":    rt,
		}).Warn("invalid rt type to marshal")
		return nil
	}
	a, _ := ptypes.MarshalAny(r)
	return a
}

func MarshalRTs(values []bgp.ExtendedCommunityInterface) []*any.Any {
	rts := make([]*any.Any, 0, len(values))
	for _, rt := range values {
		rts = append(rts, MarshalRT(rt))
	}
	return rts
}

func UnmarshalRT(a *any.Any) (bgp.ExtendedCommunityInterface, error) {
	var value ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(a, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal route target: %s", err)
	}
	switch v := value.Message.(type) {
	case *api.TwoOctetAsSpecificExtended:
		return bgp.NewTwoOctetAsSpecificExtended(bgp.ExtendedCommunityAttrSubType(v.SubType), uint16(v.As), v.LocalAdmin, v.IsTransitive), nil
	case *api.IPv4AddressSpecificExtended:
		rt := bgp.NewIPv4AddressSpecificExtended(bgp.ExtendedCommunityAttrSubType(v.SubType), v.Address, uint16(v.LocalAdmin), v.IsTransitive)
		if rt == nil {
			return nil, fmt.Errorf("invalid address for ipv4 address specific route target: %s", v.Address)
		}
		return rt, nil
	case *api.FourOctetAsSpecificExtended:
		return bgp.NewFourOctetAsSpecificExtended(bgp.ExtendedCommunityAttrSubType(v.SubType), v.As, uint16(v.LocalAdmin), v.IsTransitive), nil
	}
	return nil, fmt.Errorf("invalid route target type: %s", a.TypeUrl)
}

func UnmarshalRTs(values []*any.Any) ([]bgp.ExtendedCommunityInterface, error) {
	rts := make([]bgp.ExtendedCommunityInterface, 0, len(values))
	for _, an := range values {
		rt, err := UnmarshalRT(an)
		if err != nil {
			return nil, err
		}
		rts = append(rts, rt)
	}
	return rts, nil
}

func NewExtendedCommunitiesAttributeFromNative(a *bgp.PathAttributeExtendedCommunities) *api.ExtendedCommunitiesAttribute {
	communities := make([]*any.Any, 0, len(a.Value))
	for _, value := range a.Value {
		var community proto.Message
		switch v := value.(type) {
		case *bgp.TwoOctetAsSpecificExtended:
			community = &api.TwoOctetAsSpecificExtended{
				IsTransitive: v.IsTransitive,
				SubType:      uint32(v.SubType),
				As:           uint32(v.AS),
				LocalAdmin:   uint32(v.LocalAdmin),
			}
		case *bgp.IPv4AddressSpecificExtended:
			community = &api.IPv4AddressSpecificExtended{
				IsTransitive: v.IsTransitive,
				SubType:      uint32(v.SubType),
				Address:      v.IPv4.String(),
				LocalAdmin:   uint32(v.LocalAdmin),
			}
		case *bgp.FourOctetAsSpecificExtended:
			community = &api.FourOctetAsSpecificExtended{
				IsTransitive: v.IsTransitive,
				SubType:      uint32(v.SubType),
				As:           uint32(v.AS),
				LocalAdmin:   uint32(v.LocalAdmin),
			}
		case *bgp.ValidationExtended:
			community = &api.ValidationExtended{
				State: uint32(v.State),
			}
		case *bgp.ColorExtended:
			community = &api.ColorExtended{
				Color: v.Color,
			}
		case *bgp.EncapExtended:
			community = &api.EncapExtended{
				TunnelType: uint32(v.TunnelType),
			}
		case *bgp.DefaultGatewayExtended:
			community = &api.DefaultGatewayExtended{}
		case *bgp.OpaqueExtended:
			community = &api.OpaqueExtended{
				IsTransitive: v.IsTransitive,
				Value:        v.Value,
			}
		case *bgp.ESILabelExtended:
			community = &api.ESILabelExtended{
				IsSingleActive: v.IsSingleActive,
				Label:          v.Label,
			}
		case *bgp.ESImportRouteTarget:
			community = &api.ESImportRouteTarget{
				EsImport: v.ESImport.String(),
			}
		case *bgp.MacMobilityExtended:
			community = &api.MacMobilityExtended{
				IsSticky:    v.IsSticky,
				SequenceNum: v.Sequence,
			}
		case *bgp.RouterMacExtended:
			community = &api.RouterMacExtended{
				Mac: v.Mac.String(),
			}
		case *bgp.TrafficRateExtended:
			community = &api.TrafficRateExtended{
				As:   uint32(v.AS),
				Rate: v.Rate,
			}
		case *bgp.TrafficActionExtended:
			community = &api.TrafficActionExtended{
				Terminal: v.Terminal,
				Sample:   v.Sample,
			}
		case *bgp.RedirectTwoOctetAsSpecificExtended:
			community = &api.RedirectTwoOctetAsSpecificExtended{
				As:         uint32(v.AS),
				LocalAdmin: v.LocalAdmin,
			}
		case *bgp.RedirectIPv4AddressSpecificExtended:
			community = &api.RedirectIPv4AddressSpecificExtended{
				Address:    v.IPv4.String(),
				LocalAdmin: uint32(v.LocalAdmin),
			}
		case *bgp.RedirectFourOctetAsSpecificExtended:
			community = &api.RedirectFourOctetAsSpecificExtended{
				As:         v.AS,
				LocalAdmin: uint32(v.LocalAdmin),
			}
		case *bgp.TrafficRemarkExtended:
			community = &api.TrafficRemarkExtended{
				Dscp: uint32(v.DSCP),
			}
		case *bgp.UnknownExtended:
			community = &api.UnknownExtended{
				Type:  uint32(v.Type),
				Value: v.Value,
			}
		default:
			log.WithFields(log.Fields{
				"Topic":     "protobuf",
				"Community": value,
			}).Warn("unsupported extended community")
			return nil
		}
		an, _ := ptypes.MarshalAny(community)
		communities = append(communities, an)
	}
	return &api.ExtendedCommunitiesAttribute{
		Communities: communities,
	}
}

func unmarshalExComm(a *api.ExtendedCommunitiesAttribute) (*bgp.PathAttributeExtendedCommunities, error) {
	communities := make([]bgp.ExtendedCommunityInterface, 0, len(a.Communities))
	for _, an := range a.Communities {
		var community bgp.ExtendedCommunityInterface
		var value ptypes.DynamicAny
		if err := ptypes.UnmarshalAny(an, &value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal extended community: %s", err)
		}
		switch v := value.Message.(type) {
		case *api.TwoOctetAsSpecificExtended:
			community = bgp.NewTwoOctetAsSpecificExtended(bgp.ExtendedCommunityAttrSubType(v.SubType), uint16(v.As), v.LocalAdmin, v.IsTransitive)
		case *api.IPv4AddressSpecificExtended:
			community = bgp.NewIPv4AddressSpecificExtended(bgp.ExtendedCommunityAttrSubType(v.SubType), v.Address, uint16(v.LocalAdmin), v.IsTransitive)
		case *api.FourOctetAsSpecificExtended:
			community = bgp.NewFourOctetAsSpecificExtended(bgp.ExtendedCommunityAttrSubType(v.SubType), v.As, uint16(v.LocalAdmin), v.IsTransitive)
		case *api.ValidationExtended:
			community = bgp.NewValidationExtended(bgp.ValidationState(v.State))
		case *api.ColorExtended:
			community = bgp.NewColorExtended(v.Color)
		case *api.EncapExtended:
			community = bgp.NewEncapExtended(bgp.TunnelType(v.TunnelType))
		case *api.DefaultGatewayExtended:
			community = bgp.NewDefaultGatewayExtended()
		case *api.OpaqueExtended:
			community = bgp.NewOpaqueExtended(v.IsTransitive, v.Value)
		case *api.ESILabelExtended:
			community = bgp.NewESILabelExtended(v.Label, v.IsSingleActive)
		case *api.ESImportRouteTarget:
			community = bgp.NewESImportRouteTarget(v.EsImport)
		case *api.MacMobilityExtended:
			community = bgp.NewMacMobilityExtended(v.SequenceNum, v.IsSticky)
		case *api.RouterMacExtended:
			community = bgp.NewRoutersMacExtended(v.Mac)
		case *api.TrafficRateExtended:
			community = bgp.NewTrafficRateExtended(uint16(v.As), v.Rate)
		case *api.TrafficActionExtended:
			community = bgp.NewTrafficActionExtended(v.Terminal, v.Sample)
		case *api.RedirectTwoOctetAsSpecificExtended:
			community = bgp.NewRedirectTwoOctetAsSpecificExtended(uint16(v.As), v.LocalAdmin)
		case *api.RedirectIPv4AddressSpecificExtended:
			community = bgp.NewRedirectIPv4AddressSpecificExtended(v.Address, uint16(v.LocalAdmin))
		case *api.RedirectFourOctetAsSpecificExtended:
			community = bgp.NewRedirectFourOctetAsSpecificExtended(v.As, uint16(v.LocalAdmin))
		case *api.TrafficRemarkExtended:
			community = bgp.NewTrafficRemarkExtended(uint8(v.Dscp))
		case *api.UnknownExtended:
			community = bgp.NewUnknownExtended(bgp.ExtendedCommunityAttrType(v.Type), v.Value)
		}
		if community == nil {
			return nil, fmt.Errorf("invalid extended community: %v", value.Message)
		}
		communities = append(communities, community)
	}
	return bgp.NewPathAttributeExtendedCommunities(communities), nil
}

func NewAs4PathAttributeFromNative(a *bgp.PathAttributeAs4Path) *api.As4PathAttribute {
	segments := make([]*api.AsSegment, 0, len(a.Value))
	for _, param := range a.Value {
		segments = append(segments, &api.AsSegment{
			Type:    uint32(param.Type),
			Numbers: param.AS,
		})
	}
	return &api.As4PathAttribute{
		Segments: segments,
	}
}

func NewAs4AggregatorAttributeFromNative(a *bgp.PathAttributeAs4Aggregator) *api.As4AggregatorAttribute {
	return &api.As4AggregatorAttribute{
		As:      a.Value.AS,
		Address: a.Value.Address.String(),
	}
}

func NewPmsiTunnelAttributeFromNative(a *bgp.PathAttributePmsiTunnel) *api.PmsiTunnelAttribute {
	var flags uint32
	if a.IsLeafInfoRequired {
		flags |= 0x01
	}
	id, _ := a.TunnelID.Serialize()
	return &api.PmsiTunnelAttribute{
		Flags: flags,
		Type:  uint32(a.TunnelType),
		Label: a.Label,
		Id:    id,
	}
}

func NewTunnelEncapAttributeFromNative(a *bgp.PathAttributeTunnelEncap) *api.TunnelEncapAttribute {
	tlvs := make([]*api.TunnelEncapTLV, 0, len(a.Value))
	for _, v := range a.Value {
		subTlvs := make([]*any.Any, 0, len(v.Value))
		for _, s := range v.Value {
			var subTlv proto.Message
			switch sv := s.(type) {
			case *bgp.TunnelEncapSubTLVEncapsulation:
				subTlv = &api.TunnelEncapSubTLVEncapsulation{
					Key:    sv.Key,
					Cookie: sv.Cookie,
				}
			case *bgp.TunnelEncapSubTLVProtocol:
				subTlv = &api.TunnelEncapSubTLVProtocol{
					Protocol: uint32(sv.Protocol),
				}
			case *bgp.TunnelEncapSubTLVColor:
				subTlv = &api.TunnelEncapSubTLVColor{
					Color: sv.Color,
				}
			case *bgp.TunnelEncapSubTLVUnknown:
				subTlv = &api.TunnelEncapSubTLVUnknown{
					Type:  uint32(sv.Type),
					Value: sv.Value,
				}
			}
			an, _ := ptypes.MarshalAny(subTlv)
			subTlvs = append(subTlvs, an)
		}
		tlvs = append(tlvs, &api.TunnelEncapTLV{
			Type: uint32(v.Type),
			Tlvs: subTlvs,
		})
	}
	return &api.TunnelEncapAttribute{
		Tlvs: tlvs,
	}
}

func NewIP6ExtendedCommunitiesAttributeFromNative(a *bgp.PathAttributeIP6ExtendedCommunities) *api.IP6ExtendedCommunitiesAttribute {
	communities := make([]*any.Any, 0, len(a.Value))
	for _, value := range a.Value {
		var community proto.Message
		switch v := value.(type) {
		case *bgp.IPv6AddressSpecificExtended:
			community = &api.IPv6AddressSpecificExtended{
				IsTransitive: v.IsTransitive,
				SubType:      uint32(v.SubType),
				Address:      v.IPv6.String(),
				LocalAdmin:   uint32(v.LocalAdmin),
			}
		case *bgp.RedirectIPv6AddressSpecificExtended:
			community = &api.RedirectIPv6AddressSpecificExtended{
				Address:    v.IPv6.String(),
				LocalAdmin: uint32(v.LocalAdmin),
			}
		default:
			log.WithFields(log.Fields{
				"Topic":     "protobuf",
				"Attribute": value,
			}).Warn("invalid ipv6 extended community")
			return nil
		}
		an, _ := ptypes.MarshalAny(community)
		communities = append(communities, an)
	}
	return &api.IP6ExtendedCommunitiesAttribute{
		Communities: communities,
	}
}

func NewAigpAttributeFromNative(a *bgp.PathAttributeAigp) *api.AigpAttribute {
	tlvs := make([]*any.Any, 0, len(a.Values))
	for _, value := range a.Values {
		var tlv proto.Message
		switch v := value.(type) {
		case *bgp.AigpTLVIgpMetric:
			tlv = &api.AigpTLVIGPMetric{
				Metric: v.Metric,
			}
		case *bgp.AigpTLVDefault:
			tlv = &api.AigpTLVUnknown{
				Type:  uint32(v.Type()),
				Value: v.Value,
			}
		}
		an, _ := ptypes.MarshalAny(tlv)
		tlvs = append(tlvs, an)
	}
	return &api.AigpAttribute{
		Tlvs: tlvs,
	}
}

func NewLargeCommunitiesAttributeFromNative(a *bgp.PathAttributeLargeCommunities) *api.LargeCommunitiesAttribute {
	communities := make([]*api.LargeCommunity, 0, len(a.Values))
	for _, v := range a.Values {
		communities = append(communities, &api.LargeCommunity{
			GlobalAdmin: v.ASN,
			LocalData1:  v.LocalData1,
			LocalData2:  v.LocalData2,
		})
	}
	return &api.LargeCommunitiesAttribute{
		Communities: communities,
	}
}

func NewUnknownAttributeFromNative(a *bgp.PathAttributeUnknown) *api.UnknownAttribute {
	return &api.UnknownAttribute{
		Flags: uint32(a.Flags),
		Type:  uint32(a.Type),
		Value: a.Value,
	}
}

func MarshalPathAttributes(attrList []bgp.PathAttributeInterface) []*any.Any {
	anyList := make([]*any.Any, 0, len(attrList))
	for _, attr := range attrList {
		switch a := attr.(type) {
		case *bgp.PathAttributeOrigin:
			n, _ := ptypes.MarshalAny(NewOriginAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeAsPath:
			n, _ := ptypes.MarshalAny(NewAsPathAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeNextHop:
			n, _ := ptypes.MarshalAny(NewNextHopAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeMultiExitDisc:
			n, _ := ptypes.MarshalAny(NewMultiExitDiscAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeLocalPref:
			n, _ := ptypes.MarshalAny(NewLocalPrefAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeAtomicAggregate:
			n, _ := ptypes.MarshalAny(NewAtomicAggregateAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeAggregator:
			n, _ := ptypes.MarshalAny(NewAggregatorAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeCommunities:
			n, _ := ptypes.MarshalAny(NewCommunitiesAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeOriginatorId:
			n, _ := ptypes.MarshalAny(NewOriginatorIdAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeClusterList:
			n, _ := ptypes.MarshalAny(NewClusterListAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeMpReachNLRI:
			n, _ := ptypes.MarshalAny(NewMpReachNLRIAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeMpUnreachNLRI:
			n, _ := ptypes.MarshalAny(NewMpUnreachNLRIAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeExtendedCommunities:
			n, _ := ptypes.MarshalAny(NewExtendedCommunitiesAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeAs4Path:
			n, _ := ptypes.MarshalAny(NewAs4PathAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeAs4Aggregator:
			n, _ := ptypes.MarshalAny(NewAs4AggregatorAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributePmsiTunnel:
			n, _ := ptypes.MarshalAny(NewPmsiTunnelAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeTunnelEncap:
			n, _ := ptypes.MarshalAny(NewTunnelEncapAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeIP6ExtendedCommunities:
			n, _ := ptypes.MarshalAny(NewIP6ExtendedCommunitiesAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeAigp:
			n, _ := ptypes.MarshalAny(NewAigpAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeLargeCommunities:
			n, _ := ptypes.MarshalAny(NewLargeCommunitiesAttributeFromNative(a))
			anyList = append(anyList, n)
		case *bgp.PathAttributeUnknown:
			n, _ := ptypes.MarshalAny(NewUnknownAttributeFromNative(a))
			anyList = append(anyList, n)
		}
	}
	return anyList
}

func UnmarshalPathAttributes(values []*any.Any) ([]bgp.PathAttributeInterface, error) {
	attrList := make([]bgp.PathAttributeInterface, 0, len(values))
	typeMap := make(map[bgp.BGPAttrType]struct{})
	for _, an := range values {
		attr, err := unmarshalAttribute(an)
		if err != nil {
			return nil, err
		}
		if _, ok := typeMap[attr.GetType()]; ok {
			return nil, fmt.Errorf("duplicated path attribute type: %d", attr.GetType())
		}
		typeMap[attr.GetType()] = struct{}{}
		attrList = append(attrList, attr)
	}
	return attrList, nil
}

func unmarshalAttribute(an *any.Any) (bgp.PathAttributeInterface, error) {
	var value ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(an, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal route distinguisher: %s", err)
	}
	switch a := value.Message.(type) {
	case *api.OriginAttribute:
		return bgp.NewPathAttributeOrigin(uint8(a.Origin)), nil
	case *api.AsPathAttribute:
		params := make([]bgp.AsPathParamInterface, 0, len(a.Segments))
		for _, segment := range a.Segments {
			params = append(params, bgp.NewAs4PathParam(uint8(segment.Type), segment.Numbers))
		}
		return bgp.NewPathAttributeAsPath(params), nil
	case *api.NextHopAttribute:
		nexthop := net.ParseIP(a.NextHop).To4()
		if nexthop == nil {
			return nil, fmt.Errorf("invalid nexthop address: %s", a.NextHop)
		}
		return bgp.NewPathAttributeNextHop(a.NextHop), nil
	case *api.MultiExitDiscAttribute:
		return bgp.NewPathAttributeMultiExitDisc(a.Med), nil
	case *api.LocalPrefAttribute:
		return bgp.NewPathAttributeLocalPref(a.LocalPref), nil
	case *api.AtomicAggregateAttribute:
		return bgp.NewPathAttributeAtomicAggregate(), nil
	case *api.AggregatorAttribute:
		if net.ParseIP(a.Address).To4() == nil {
			return nil, fmt.Errorf("invalid aggregator address: %s", a.Address)
		}
		return bgp.NewPathAttributeAggregator(a.As, a.Address), nil
	case *api.CommunitiesAttribute:
		return bgp.NewPathAttributeCommunities(a.Communities), nil
	case *api.OriginatorIdAttribute:
		if net.ParseIP(a.Id).To4() == nil {
			return nil, fmt.Errorf("invalid originator id: %s", a.Id)
		}
		return bgp.NewPathAttributeOriginatorId(a.Id), nil
	case *api.ClusterListAttribute:
		for _, id := range a.Ids {
			if net.ParseIP(id).To4() == nil {
				return nil, fmt.Errorf("invalid cluster list: %s", a.Ids)
			}
		}
		return bgp.NewPathAttributeClusterList(a.Ids), nil
	case *api.MpReachNLRIAttribute:
		rf := ToRouteFamily(a.Family)
		nlris, err := UnmarshalNLRIs(rf, a.Nlris)
		if err != nil {
			return nil, err
		}
		afi, safi := bgp.RouteFamilyToAfiSafi(rf)
		nexthop := "0.0.0.0"
		var linkLocalNexthop net.IP
		if afi == bgp.AFI_IP6 {
			nexthop = "::"
			if len(a.NextHops) > 1 {
				linkLocalNexthop = net.ParseIP(a.NextHops[1]).To16()
				if linkLocalNexthop == nil {
					return nil, fmt.Errorf("invalid nexthop: %s", a.NextHops[1])
				}
			}
		}
		if safi == bgp.SAFI_FLOW_SPEC_UNICAST || safi == bgp.SAFI_FLOW_SPEC_VPN {
			nexthop = ""
		} else if len(a.NextHops) > 0 {
			nexthop = a.NextHops[0]
			if net.ParseIP(nexthop) == nil {
				return nil, fmt.Errorf("invalid nexthop: %s", nexthop)
			}
		}
		attr := bgp.NewPathAttributeMpReachNLRI(nexthop, nlris)
		attr.LinkLocalNexthop = linkLocalNexthop
		return attr, nil
	case *api.MpUnreachNLRIAttribute:
		rf := ToRouteFamily(a.Family)
		nlris, err := UnmarshalNLRIs(rf, a.Nlris)
		if err != nil {
			return nil, err
		}
		return bgp.NewPathAttributeMpUnreachNLRI(nlris), nil
	case *api.ExtendedCommunitiesAttribute:
		return unmarshalExComm(a)
	case *api.As4PathAttribute:
		params := make([]*bgp.As4PathParam, 0, len(a.Segments))
		for _, segment := range a.Segments {
			params = append(params, bgp.NewAs4PathParam(uint8(segment.Type), segment.Numbers))
		}
		return bgp.NewPathAttributeAs4Path(params), nil
	case *api.As4AggregatorAttribute:
		if net.ParseIP(a.Address).To4() == nil {
			return nil, fmt.Errorf("invalid as4 aggregator address: %s", a.Address)
		}
		return bgp.NewPathAttributeAs4Aggregator(a.As, a.Address), nil
	case *api.PmsiTunnelAttribute:
		typ := bgp.PmsiTunnelType(a.Type)
		var isLeafInfoRequired bool
		if a.Flags&0x01 > 0 {
			isLeafInfoRequired = true
		}
		var id bgp.PmsiTunnelIDInterface
		switch typ {
		case bgp.PMSI_TUNNEL_TYPE_INGRESS_REPL:
			ip := net.IP(a.Id)
			if ip.To4() == nil && ip.To16() == nil {
				return nil, fmt.Errorf("invalid pmsi tunnel identifier: %s", a.Id)
			}
			id = bgp.NewIngressReplTunnelID(ip.String())
		default:
			id = bgp.NewDefaultPmsiTunnelID(a.Id)
		}
		return bgp.NewPathAttributePmsiTunnel(typ, isLeafInfoRequired, a.Label, id), nil
	case *api.TunnelEncapAttribute:
		tlvs := make([]*bgp.TunnelEncapTLV, 0, len(a.Tlvs))
		for _, tlv := range a.Tlvs {
			subTlvs := make([]bgp.TunnelEncapSubTLVInterface, 0, len(tlv.Tlvs))
			for _, an := range tlv.Tlvs {
				var subTlv bgp.TunnelEncapSubTLVInterface
				var subValue ptypes.DynamicAny
				if err := ptypes.UnmarshalAny(an, &subValue); err != nil {
					return nil, fmt.Errorf("failed to unmarshal tunnel encapsulation attribute sub tlv: %s", err)
				}
				switch sv := subValue.Message.(type) {
				case *api.TunnelEncapSubTLVEncapsulation:
					subTlv = bgp.NewTunnelEncapSubTLVEncapsulation(sv.Key, sv.Cookie)
				case *api.TunnelEncapSubTLVProtocol:
					subTlv = bgp.NewTunnelEncapSubTLVProtocol(uint16(sv.Protocol))
				case *api.TunnelEncapSubTLVColor:
					subTlv = bgp.NewTunnelEncapSubTLVColor(sv.Color)
				case *api.TunnelEncapSubTLVUnknown:
					subTlv = bgp.NewTunnelEncapSubTLVUnknown(bgp.EncapSubTLVType(sv.Type), sv.Value)
				default:
					return nil, fmt.Errorf("invalid tunnel encapsulation attribute sub tlv: %v", subValue.Message)
				}
				subTlvs = append(subTlvs, subTlv)
			}
			tlvs = append(tlvs, bgp.NewTunnelEncapTLV(bgp.TunnelType(tlv.Type), subTlvs))
		}
		return bgp.NewPathAttributeTunnelEncap(tlvs), nil
	case *api.IP6ExtendedCommunitiesAttribute:
		communities := make([]bgp.ExtendedCommunityInterface, 0, len(a.Communities))
		for _, an := range a.Communities {
			var community bgp.ExtendedCommunityInterface
			var value ptypes.DynamicAny
			if err := ptypes.UnmarshalAny(an, &value); err != nil {
				return nil, fmt.Errorf("failed to unmarshal ipv6 extended community: %s", err)
			}
			switch v := value.Message.(type) {
			case *api.IPv6AddressSpecificExtended:
				community = bgp.NewIPv6AddressSpecificExtended(bgp.ExtendedCommunityAttrSubType(v.SubType), v.Address, uint16(v.LocalAdmin), v.IsTransitive)
			case *api.RedirectIPv6AddressSpecificExtended:
				community = bgp.NewRedirectIPv6AddressSpecificExtended(v.Address, uint16(v.LocalAdmin))
			}
			if community == nil {
				return nil, fmt.Errorf("invalid ipv6 extended community: %v", value.Message)
			}
			communities = append(communities, community)
		}
		return bgp.NewPathAttributeIP6ExtendedCommunities(communities), nil

	case *api.AigpAttribute:
		tlvs := make([]bgp.AigpTLVInterface, 0, len(a.Tlvs))
		for _, an := range a.Tlvs {
			var tlv bgp.AigpTLVInterface
			var value ptypes.DynamicAny
			if err := ptypes.UnmarshalAny(an, &value); err != nil {
				return nil, fmt.Errorf("failed to unmarshal aigp attribute tlv: %s", err)
			}
			switch v := value.Message.(type) {
			case *api.AigpTLVIGPMetric:
				tlv = bgp.NewAigpTLVIgpMetric(v.Metric)
			case *api.AigpTLVUnknown:
				tlv = bgp.NewAigpTLVDefault(bgp.AigpTLVType(v.Type), v.Value)
			}
			if tlv == nil {
				return nil, fmt.Errorf("invalid aigp attribute tlv: %v", value.Message)
			}
			tlvs = append(tlvs, tlv)
		}
		return bgp.NewPathAttributeAigp(tlvs), nil

	case *api.LargeCommunitiesAttribute:
		communities := make([]*bgp.LargeCommunity, 0, len(a.Communities))
		for _, c := range a.Communities {
			communities = append(communities, bgp.NewLargeCommunity(c.GlobalAdmin, c.LocalData1, c.LocalData2))
		}
		return bgp.NewPathAttributeLargeCommunities(communities), nil

	case *api.UnknownAttribute:
		return bgp.NewPathAttributeUnknown(bgp.BGPAttrFlag(a.Flags), bgp.BGPAttrType(a.Type), a.Value), nil
	}
	return nil, errors.New("unknown path attribute")
}
