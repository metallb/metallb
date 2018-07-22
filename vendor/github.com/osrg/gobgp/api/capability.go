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

package gobgpapi

import (
	"fmt"

	proto "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"

	"github.com/osrg/gobgp/packet/bgp"
)

func NewMultiProtocolCapability(a *bgp.CapMultiProtocol) *MultiProtocolCapability {
	return &MultiProtocolCapability{
		Family: Family(a.CapValue),
	}
}

func (a *MultiProtocolCapability) ToNative() (*bgp.CapMultiProtocol, error) {
	return bgp.NewCapMultiProtocol(bgp.RouteFamily(a.Family)), nil
}

func NewRouteRefreshCapability(a *bgp.CapRouteRefresh) *RouteRefreshCapability {
	return &RouteRefreshCapability{}
}

func (a *RouteRefreshCapability) ToNative() (*bgp.CapRouteRefresh, error) {
	return bgp.NewCapRouteRefresh(), nil
}

func NewCarryingLabelInfoCapability(a *bgp.CapCarryingLabelInfo) *CarryingLabelInfoCapability {
	return &CarryingLabelInfoCapability{}
}

func (a *CarryingLabelInfoCapability) ToNative() (*bgp.CapCarryingLabelInfo, error) {
	return bgp.NewCapCarryingLabelInfo(), nil
}

func NewExtendedNexthopCapability(a *bgp.CapExtendedNexthop) *ExtendedNexthopCapability {
	tuples := make([]*ExtendedNexthopCapabilityTuple, 0, len(a.Tuples))
	for _, t := range a.Tuples {
		tuples = append(tuples, &ExtendedNexthopCapabilityTuple{
			NlriFamily:    Family(bgp.AfiSafiToRouteFamily(t.NLRIAFI, uint8(t.NLRISAFI))),
			NexthopFamily: Family(bgp.AfiSafiToRouteFamily(t.NexthopAFI, bgp.SAFI_UNICAST)),
		})
	}
	return &ExtendedNexthopCapability{
		Tuples: tuples,
	}
}

func (a *ExtendedNexthopCapability) ToNative() (*bgp.CapExtendedNexthop, error) {
	tuples := make([]*bgp.CapExtendedNexthopTuple, 0, len(a.Tuples))
	for _, t := range a.Tuples {
		var nhAfi uint16
		switch t.NexthopFamily {
		case Family_IPv4:
			nhAfi = bgp.AFI_IP
		case Family_IPv6:
			nhAfi = bgp.AFI_IP6
		default:
			return nil, fmt.Errorf("invalid address family for nexthop afi in extended nexthop capability: %s", t.NexthopFamily)
		}
		tuples = append(tuples, bgp.NewCapExtendedNexthopTuple(bgp.RouteFamily(t.NlriFamily), nhAfi))
	}
	return bgp.NewCapExtendedNexthop(tuples), nil
}

func NewGracefulRestartCapability(a *bgp.CapGracefulRestart) *GracefulRestartCapability {
	tuples := make([]*GracefulRestartCapabilityTuple, 0, len(a.Tuples))
	for _, t := range a.Tuples {
		tuples = append(tuples, &GracefulRestartCapabilityTuple{
			Family: Family(bgp.AfiSafiToRouteFamily(t.AFI, uint8(t.SAFI))),
			Flags:  uint32(t.Flags),
		})
	}
	return &GracefulRestartCapability{
		Flags:  uint32(a.Flags),
		Time:   uint32(a.Time),
		Tuples: tuples,
	}
}

func (a *GracefulRestartCapability) ToNative() (*bgp.CapGracefulRestart, error) {
	tuples := make([]*bgp.CapGracefulRestartTuple, 0, len(a.Tuples))
	for _, t := range a.Tuples {
		var forward bool
		if t.Flags&0x80 > 0 {
			forward = true
		}
		tuples = append(tuples, bgp.NewCapGracefulRestartTuple(bgp.RouteFamily(t.Family), forward))
	}
	var restarting bool
	if a.Flags&0x08 > 0 {
		restarting = true
	}
	var notification bool
	if a.Flags&0x04 > 0 {
		notification = true
	}
	return bgp.NewCapGracefulRestart(restarting, notification, uint16(a.Time), tuples), nil
}

func NewFourOctetASNumberCapability(a *bgp.CapFourOctetASNumber) *FourOctetASNumberCapability {
	return &FourOctetASNumberCapability{
		As: a.CapValue,
	}
}

func (a *FourOctetASNumberCapability) ToNative() (*bgp.CapFourOctetASNumber, error) {
	return bgp.NewCapFourOctetASNumber(a.As), nil
}

func NewAddPathCapability(a *bgp.CapAddPath) *AddPathCapability {
	tuples := make([]*AddPathCapabilityTuple, 0, len(a.Tuples))
	for _, t := range a.Tuples {
		tuples = append(tuples, &AddPathCapabilityTuple{
			Family: Family(t.RouteFamily),
			Mode:   AddPathMode(t.Mode),
		})
	}
	return &AddPathCapability{
		Tuples: tuples,
	}
}

func (a *AddPathCapability) ToNative() (*bgp.CapAddPath, error) {
	tuples := make([]*bgp.CapAddPathTuple, 0, len(a.Tuples))
	for _, t := range a.Tuples {
		tuples = append(tuples, bgp.NewCapAddPathTuple(bgp.RouteFamily(t.Family), bgp.BGPAddPathMode(t.Mode)))
	}
	return bgp.NewCapAddPath(tuples), nil
}

func NewEnhancedRouteRefreshCapability(a *bgp.CapEnhancedRouteRefresh) *EnhancedRouteRefreshCapability {
	return &EnhancedRouteRefreshCapability{}
}

func (a *EnhancedRouteRefreshCapability) ToNative() (*bgp.CapEnhancedRouteRefresh, error) {
	return bgp.NewCapEnhancedRouteRefresh(), nil
}

func NewLongLivedGracefulRestartCapability(a *bgp.CapLongLivedGracefulRestart) *LongLivedGracefulRestartCapability {
	tuples := make([]*LongLivedGracefulRestartCapabilityTuple, 0, len(a.Tuples))
	for _, t := range a.Tuples {
		tuples = append(tuples, &LongLivedGracefulRestartCapabilityTuple{
			Family: Family(bgp.AfiSafiToRouteFamily(t.AFI, uint8(t.SAFI))),
			Flags:  uint32(t.Flags),
			Time:   t.RestartTime,
		})
	}
	return &LongLivedGracefulRestartCapability{
		Tuples: tuples,
	}
}

func (a *LongLivedGracefulRestartCapability) ToNative() (*bgp.CapLongLivedGracefulRestart, error) {
	tuples := make([]*bgp.CapLongLivedGracefulRestartTuple, 0, len(a.Tuples))
	for _, t := range a.Tuples {
		var forward bool
		if t.Flags&0x80 > 0 {
			forward = true
		}
		tuples = append(tuples, bgp.NewCapLongLivedGracefulRestartTuple(bgp.RouteFamily(t.Family), forward, t.Time))
	}
	return bgp.NewCapLongLivedGracefulRestart(tuples), nil
}

func NewRouteRefreshCiscoCapability(a *bgp.CapRouteRefreshCisco) *RouteRefreshCiscoCapability {
	return &RouteRefreshCiscoCapability{}
}

func (a *RouteRefreshCiscoCapability) ToNative() (*bgp.CapRouteRefreshCisco, error) {
	return bgp.NewCapRouteRefreshCisco(), nil
}

func NewUnknownCapability(a *bgp.CapUnknown) *UnknownCapability {
	return &UnknownCapability{
		Code:  uint32(a.CapCode),
		Value: a.CapValue,
	}
}

func (a *UnknownCapability) ToNative() (*bgp.CapUnknown, error) {
	return bgp.NewCapUnknown(bgp.BGPCapabilityCode(a.Code), a.Value), nil
}

func MarshalCapability(value bgp.ParameterCapabilityInterface) (*any.Any, error) {
	var m proto.Message
	switch n := value.(type) {
	case *bgp.CapMultiProtocol:
		m = NewMultiProtocolCapability(n)
	case *bgp.CapRouteRefresh:
		m = NewRouteRefreshCapability(n)
	case *bgp.CapCarryingLabelInfo:
		m = NewCarryingLabelInfoCapability(n)
	case *bgp.CapExtendedNexthop:
		m = NewExtendedNexthopCapability(n)
	case *bgp.CapGracefulRestart:
		m = NewGracefulRestartCapability(n)
	case *bgp.CapFourOctetASNumber:
		m = NewFourOctetASNumberCapability(n)
	case *bgp.CapAddPath:
		m = NewAddPathCapability(n)
	case *bgp.CapEnhancedRouteRefresh:
		m = NewEnhancedRouteRefreshCapability(n)
	case *bgp.CapLongLivedGracefulRestart:
		m = NewLongLivedGracefulRestartCapability(n)
	case *bgp.CapRouteRefreshCisco:
		m = NewRouteRefreshCiscoCapability(n)
	case *bgp.CapUnknown:
		m = NewUnknownCapability(n)
	default:
		return nil, fmt.Errorf("invalid capability type to marshal: %+v", value)
	}
	return ptypes.MarshalAny(m)
}

func MarshalCapabilities(values []bgp.ParameterCapabilityInterface) ([]*any.Any, error) {
	caps := make([]*any.Any, 0, len(values))
	for _, value := range values {
		a, err := MarshalCapability(value)
		if err != nil {
			return nil, err
		}
		caps = append(caps, a)
	}
	return caps, nil
}

func UnmarshalCapability(a *any.Any) (bgp.ParameterCapabilityInterface, error) {
	var value ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(a, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal capability: %s", err)
	}
	switch v := value.Message.(type) {
	case *MultiProtocolCapability:
		return v.ToNative()
	case *RouteRefreshCapability:
		return v.ToNative()
	case *CarryingLabelInfoCapability:
		return v.ToNative()
	case *ExtendedNexthopCapability:
		return v.ToNative()
	case *GracefulRestartCapability:
		return v.ToNative()
	case *FourOctetASNumberCapability:
		return v.ToNative()
	case *AddPathCapability:
		return v.ToNative()
	case *EnhancedRouteRefreshCapability:
		return v.ToNative()
	case *LongLivedGracefulRestartCapability:
		return v.ToNative()
	case *RouteRefreshCiscoCapability:
		return v.ToNative()
	case *UnknownCapability:
		return v.ToNative()
	}
	return nil, fmt.Errorf("invalid capability type to unmarshal: %s", a.TypeUrl)
}

func UnmarshalCapabilities(values []*any.Any) ([]bgp.ParameterCapabilityInterface, error) {
	caps := make([]bgp.ParameterCapabilityInterface, 0, len(values))
	for _, value := range values {
		c, err := UnmarshalCapability(value)
		if err != nil {
			return nil, err
		}
		caps = append(caps, c)
	}
	return caps, nil
}
