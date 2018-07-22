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
	nlri, err := d.Paths[0].GetNativeNlri()
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
	if len(p.Nlri) > 0 {
		return getNLRI(bgp.RouteFamily(p.Family), p.Nlri)
	}
	return UnmarshalNLRI(bgp.RouteFamily(p.Family), p.AnyNlri)
}

func (p *Path) GetNativePathAttributes() ([]bgp.PathAttributeInterface, error) {
	pattrsLen := len(p.Pattrs)
	if pattrsLen > 0 {
		pattrs := make([]bgp.PathAttributeInterface, 0, pattrsLen)
		for _, attr := range p.Pattrs {
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
	return UnmarshalPathAttributes(p.AnyPattrs)
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
		nlri, err = p.GetNativeNlri()
		if err != nil {
			return nil, err
		}
	}
	pattr, err := p.GetNativePathAttributes()
	if err != nil {
		return nil, err
	}
	t := time.Unix(p.Age, 0)
	nlri.SetPathIdentifier(p.Identifier)
	nlri.SetPathLocalIdentifier(p.LocalIdentifier)
	path := table.NewPath(info, nlri, p.IsWithdraw, pattr, t, false)

	// p.ValidationDetail.* are already validated
	matched, _ := NewROAListFromApiStructList(p.ValidationDetail.Matched)
	unmatchedAs, _ := NewROAListFromApiStructList(p.ValidationDetail.UnmatchedAs)
	unmatchedLength, _ := NewROAListFromApiStructList(p.ValidationDetail.UnmatchedLength)

	path.SetValidation(&table.Validation{
		Status:          config.IntToRpkiValidationResultTypeMap[int(p.Validation)],
		Reason:          table.IntToRpkiValidationReasonTypeMap[int(p.ValidationDetail.Reason)],
		Matched:         matched,
		UnmatchedAs:     unmatchedAs,
		UnmatchedLength: unmatchedLength,
	})
	path.MarkStale(p.Stale)
	path.IsNexthopInvalid = p.IsNexthopInvalid
	return path, nil
}

func NewROAListFromApiStructList(l []*Roa) ([]*table.ROA, error) {
	roas := make([]*table.ROA, 0, len(l))
	for _, r := range l {
		ip := net.ParseIP(r.Prefix)
		family := bgp.RF_IPv4_UC
		if ip == nil {
			return nil, fmt.Errorf("invalid prefix %s", r.Prefix)
		} else {
			if ip.To4() == nil {
				family = bgp.RF_IPv6_UC
			}
		}
		afi, _ := bgp.RouteFamilyToAfiSafi(family)
		roa := table.NewROA(int(afi), []byte(ip), uint8(r.Prefixlen), uint8(r.Maxlen), r.As, net.JoinHostPort(r.Conf.Address, r.Conf.RemotePort))
		roas = append(roas, roa)
	}
	return roas, nil
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
