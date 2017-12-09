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
	"fmt"
	"net"
	"time"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
)

type ToNativeOption struct {
	LocalAS                 uint32
	LocalID                 net.IP
	RouteReflectorClient    bool
	RouteReflectorClusterID net.IP
	NLRI                    bgp.AddrPrefixInterface
}

func (t *Table) ToNativeTable(option ...ToNativeOption) (*table.Table, error) {
	dsts := make([]*table.Destination, 0, len(t.Destinations))
	for _, d := range t.Destinations {
		dst, err := d.ToNativeDestination(option...)
		if err != nil {
			return nil, err
		}
		dsts = append(dsts, dst)
	}
	return table.NewTable(bgp.RouteFamily(t.Family), dsts...), nil
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

func (d *Destination) ToNativeDestination(option ...ToNativeOption) (*table.Destination, error) {
	if len(d.Paths) == 0 {
		return nil, fmt.Errorf("no path in destination")
	}
	nlri, err := getNLRI(bgp.RouteFamily(d.Paths[0].Family), d.Paths[0].Nlri)
	if err != nil {
		return nil, err
	}
	option = append(option, ToNativeOption{
		NLRI: nlri,
	})
	paths := make([]*table.Path, 0, len(d.Paths))
	for _, p := range d.Paths {
		var path *table.Path
		var err error
		if p.Identifier > 0 {
			path, err = p.ToNativePath()
		} else {
			path, err = p.ToNativePath(option...)
		}
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return table.NewDestination(nlri, 0, paths...), nil
}

func (p *Path) GetNativeNlri() (bgp.AddrPrefixInterface, error) {
	return getNLRI(bgp.RouteFamily(p.Family), p.Nlri)
}

func (p *Path) ToNativePath(option ...ToNativeOption) (*table.Path, error) {
	info := &table.PeerInfo{
		AS:      p.SourceAsn,
		ID:      net.ParseIP(p.SourceId),
		Address: net.ParseIP(p.NeighborIp),
	}
	var nlri bgp.AddrPrefixInterface
	for _, o := range option {
		info.LocalAS = o.LocalAS
		info.LocalID = o.LocalID
		info.RouteReflectorClient = o.RouteReflectorClient
		info.RouteReflectorClusterID = o.RouteReflectorClusterID
		nlri = o.NLRI
	}
	if nlri == nil {
		var err error
		nlri, err = getNLRI(bgp.RouteFamily(p.Family), p.Nlri)
		if err != nil {
			return nil, err
		}
	}
	pattr := make([]bgp.PathAttributeInterface, 0, len(p.Pattrs))
	for _, attr := range p.Pattrs {
		p, err := bgp.GetPathAttribute(attr)
		if err != nil {
			return nil, err
		}
		err = p.DecodeFromBytes(attr)
		if err != nil {
			return nil, err
		}
		pattr = append(pattr, p)
	}
	t := time.Unix(p.Age, 0)
	nlri.SetPathIdentifier(p.Identifier)
	nlri.SetPathLocalIdentifier(p.LocalIdentifier)
	path := table.NewPath(info, nlri, p.IsWithdraw, pattr, t, false)
	path.SetValidation(&table.Validation{
		Status:          config.IntToRpkiValidationResultTypeMap[int(p.Validation)],
		Reason:          table.IntToRpkiValidationReasonTypeMap[int(p.ValidationDetail.Reason)],
		Matched:         NewROAListFromApiStructList(p.ValidationDetail.Matched),
		UnmatchedAs:     NewROAListFromApiStructList(p.ValidationDetail.UnmatchedAs),
		UnmatchedLength: NewROAListFromApiStructList(p.ValidationDetail.UnmatchedLength),
	})
	path.MarkStale(p.Stale)
	path.SetUUID(p.Uuid)
	if p.Filtered {
		path.Filter("", table.POLICY_DIRECTION_IN)
	}
	path.IsNexthopInvalid = p.IsNexthopInvalid
	return path, nil
}

func NewROAListFromApiStructList(l []*Roa) []*table.ROA {
	roas := make([]*table.ROA, 0, len(l))
	for _, r := range l {
		ip := net.ParseIP(r.Prefix)
		rf := func(prefix string) bgp.RouteFamily {
			a, _, _ := net.ParseCIDR(prefix)
			if a.To4() != nil {
				return bgp.RF_IPv4_UC
			} else {
				return bgp.RF_IPv6_UC
			}
		}(r.Prefix)
		afi, _ := bgp.RouteFamilyToAfiSafi(rf)
		roa := table.NewROA(int(afi), []byte(ip), uint8(r.Prefixlen), uint8(r.Maxlen), r.As, net.JoinHostPort(r.Conf.Address, r.Conf.RemotePort))
		roas = append(roas, roa)
	}
	return roas
}

func extractFamilyFromConfigAfiSafi(c *config.AfiSafi) uint32 {
	if c == nil {
		return 0
	}
	// If address family value is already stored in AfiSafiState structure,
	// we prefer to use this value.
	if c.State.Family != 0 {
		return uint32(c.State.Family)
	}
	// In case that Neighbor structure came from CLI or gRPC, address family
	// value in AfiSafiState structure can be omitted.
	// Here extracts value from AfiSafiName field in AfiSafiConfig structure.
	if rf, err := bgp.GetRouteFamily(string(c.Config.AfiSafiName)); err == nil {
		return uint32(rf)
	}
	// Ignores invalid address family name
	return 0
}
