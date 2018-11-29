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

package apiutil

import (
	"encoding/json"
	"net"
	"time"

	"github.com/golang/protobuf/ptypes"
	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/pkg/packet/bgp"
)

// workaround. This for the json format compatibility. Once we update senario tests, we can remove this.
type Path struct {
	Nlri       bgp.AddrPrefixInterface      `json:"nlri"`
	Age        int64                        `json:"age"`
	Best       bool                         `json:"best"`
	Attrs      []bgp.PathAttributeInterface `json:"attrs"`
	Stale      bool                         `json:"stale"`
	Withdrawal bool                         `json:"withdrawal,omitempty"`
	SourceID   net.IP                       `json:"source-id,omitempty"`
	NeighborIP net.IP                       `json:"neighbor-ip,omitempty"`
}

type Destination struct {
	Paths []*Path
}

func (d *Destination) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Paths)
}

func NewDestination(dst *api.Destination) *Destination {
	l := make([]*Path, 0, len(dst.Paths))
	for _, p := range dst.Paths {
		nlri, _ := GetNativeNlri(p)
		attrs, _ := GetNativePathAttributes(p)
		t, _ := ptypes.Timestamp(p.Age)
		l = append(l, &Path{
			Nlri:       nlri,
			Age:        t.Unix(),
			Best:       p.Best,
			Attrs:      attrs,
			Stale:      p.Stale,
			Withdrawal: p.IsWithdraw,
			SourceID:   net.ParseIP(p.SourceId),
			NeighborIP: net.ParseIP(p.NeighborIp),
		})
	}
	return &Destination{Paths: l}
}

func NewPath(nlri bgp.AddrPrefixInterface, isWithdraw bool, attrs []bgp.PathAttributeInterface, age time.Time) *api.Path {
	t, _ := ptypes.TimestampProto(age)
	return &api.Path{
		Nlri:       MarshalNLRI(nlri),
		Pattrs:     MarshalPathAttributes(attrs),
		Age:        t,
		IsWithdraw: isWithdraw,
		Family:     ToApiFamily(nlri.AFI(), nlri.SAFI()),
		Identifier: nlri.PathIdentifier(),
	}
}

func getNLRI(family bgp.RouteFamily, buf []byte) (bgp.AddrPrefixInterface, error) {
	afi, safi := bgp.RouteFamilyToAfiSafi(family)
	nlri, err := bgp.NewPrefixFromRouteFamily(afi, safi)
	if err != nil {
		return nil, err
	}
	if err := nlri.DecodeFromBytes(buf); err != nil {
		return nil, err
	}
	return nlri, nil
}

func GetNativeNlri(p *api.Path) (bgp.AddrPrefixInterface, error) {
	if len(p.NlriBinary) > 0 {
		return getNLRI(ToRouteFamily(p.Family), p.NlriBinary)
	}
	return UnmarshalNLRI(ToRouteFamily(p.Family), p.Nlri)
}

func GetNativePathAttributes(p *api.Path) ([]bgp.PathAttributeInterface, error) {
	pattrsLen := len(p.PattrsBinary)
	if pattrsLen > 0 {
		pattrs := make([]bgp.PathAttributeInterface, 0, pattrsLen)
		for _, attr := range p.PattrsBinary {
			a, err := bgp.GetPathAttribute(attr)
			if err != nil {
				return nil, err
			}
			err = a.DecodeFromBytes(attr)
			if err != nil {
				return nil, err
			}
			pattrs = append(pattrs, a)
		}
		return pattrs, nil
	}
	return UnmarshalPathAttributes(p.Pattrs)
}

func ToRouteFamily(f *api.Family) bgp.RouteFamily {
	return bgp.AfiSafiToRouteFamily(uint16(f.Afi), uint8(f.Safi))
}

func ToApiFamily(afi uint16, safi uint8) *api.Family {
	return &api.Family{
		Afi:  api.Family_Afi(afi),
		Safi: api.Family_Safi(safi),
	}
}
