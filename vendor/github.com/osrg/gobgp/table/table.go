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
	"net"
	"sort"
	"strings"
	"unsafe"

	"github.com/armon/go-radix"
	"github.com/osrg/gobgp/packet/bgp"
	log "github.com/sirupsen/logrus"
)

type LookupOption uint8

const (
	LOOKUP_EXACT LookupOption = iota
	LOOKUP_LONGER
	LOOKUP_SHORTER
)

type LookupPrefix struct {
	Prefix string
	LookupOption
}

type TableSelectOption struct {
	ID             string
	AS             uint32
	LookupPrefixes []*LookupPrefix
	VRF            *Vrf
	adj            bool
	Best           bool
	MultiPath      bool
}

type Table struct {
	routeFamily  bgp.RouteFamily
	destinations map[string]*Destination
}

func NewTable(rf bgp.RouteFamily, dsts ...*Destination) *Table {
	t := &Table{
		routeFamily:  rf,
		destinations: make(map[string]*Destination),
	}
	for _, dst := range dsts {
		t.setDestination(dst)
	}
	return t
}

func (t *Table) GetRoutefamily() bgp.RouteFamily {
	return t.routeFamily
}

func (t *Table) deletePathsByVrf(vrf *Vrf) []*Path {
	pathList := make([]*Path, 0)
	for _, dest := range t.destinations {
		for _, p := range dest.knownPathList {
			var rd bgp.RouteDistinguisherInterface
			nlri := p.GetNlri()
			switch nlri.(type) {
			case *bgp.LabeledVPNIPAddrPrefix:
				rd = nlri.(*bgp.LabeledVPNIPAddrPrefix).RD
			case *bgp.LabeledVPNIPv6AddrPrefix:
				rd = nlri.(*bgp.LabeledVPNIPv6AddrPrefix).RD
			case *bgp.EVPNNLRI:
				rd = nlri.(*bgp.EVPNNLRI).RD()
			default:
				return pathList
			}
			if p.IsLocal() && vrf.Rd.String() == rd.String() {
				pathList = append(pathList, p.Clone(true))
				break
			}
		}
	}
	return pathList
}

func (t *Table) deleteRTCPathsByVrf(vrf *Vrf, vrfs map[string]*Vrf) []*Path {
	pathList := make([]*Path, 0)
	if t.routeFamily != bgp.RF_RTC_UC {
		return pathList
	}
	for _, target := range vrf.ImportRt {
		lhs := target.String()
		for _, dest := range t.destinations {
			nlri := dest.GetNlri().(*bgp.RouteTargetMembershipNLRI)
			rhs := nlri.RouteTarget.String()
			if lhs == rhs && isLastTargetUser(vrfs, target) {
				for _, p := range dest.knownPathList {
					if p.IsLocal() {
						pathList = append(pathList, p.Clone(true))
						break
					}
				}
			}
		}
	}
	return pathList
}

func (t *Table) deleteDestByNlri(nlri bgp.AddrPrefixInterface) *Destination {
	if dst := t.GetDestination(nlri); dst != nil {
		t.deleteDest(dst)
		return dst
	}
	return nil
}

func (t *Table) deleteDest(dest *Destination) {
	destinations := t.GetDestinations()
	delete(destinations, t.tableKey(dest.GetNlri()))
	if len(destinations) == 0 {
		t.destinations = make(map[string]*Destination)
	}
}

func (t *Table) validatePath(path *Path) {
	if path == nil {
		log.WithFields(log.Fields{
			"Topic": "Table",
			"Key":   t.routeFamily,
		}).Error("path is nil")
	}
	if path.GetRouteFamily() != t.routeFamily {
		log.WithFields(log.Fields{
			"Topic":      "Table",
			"Key":        t.routeFamily,
			"Prefix":     path.GetNlri().String(),
			"ReceivedRf": path.GetRouteFamily().String(),
		}).Error("Invalid path. RouteFamily mismatch")
	}
	if attr := path.getPathAttr(bgp.BGP_ATTR_TYPE_AS_PATH); attr != nil {
		pathParam := attr.(*bgp.PathAttributeAsPath).Value
		for _, as := range pathParam {
			_, y := as.(*bgp.As4PathParam)
			if !y {
				log.WithFields(log.Fields{
					"Topic": "Table",
					"Key":   t.routeFamily,
					"As":    as,
				}).Fatal("AsPathParam must be converted to As4PathParam")
			}
		}
	}
	if attr := path.getPathAttr(bgp.BGP_ATTR_TYPE_AS4_PATH); attr != nil {
		log.WithFields(log.Fields{
			"Topic": "Table",
			"Key":   t.routeFamily,
		}).Fatal("AS4_PATH must be converted to AS_PATH")
	}
	if path.GetNlri() == nil {
		log.WithFields(log.Fields{
			"Topic": "Table",
			"Key":   t.routeFamily,
		}).Fatal("path's nlri is nil")
	}
}

func (t *Table) getOrCreateDest(nlri bgp.AddrPrefixInterface) *Destination {
	dest := t.GetDestination(nlri)
	// If destination for given prefix does not exist we create it.
	if dest == nil {
		log.WithFields(log.Fields{
			"Topic": "Table",
			"Nlri":  nlri,
		}).Debugf("create Destination")
		dest = NewDestination(nlri, 64)
		t.setDestination(dest)
	}
	return dest
}

func (t *Table) GetSortedDestinations() []*Destination {
	results := make([]*Destination, 0, len(t.GetDestinations()))
	switch t.routeFamily {
	case bgp.RF_IPv4_UC, bgp.RF_IPv6_UC:
		r := radix.New()
		for _, dst := range t.GetDestinations() {
			r.Insert(AddrToRadixkey(dst.nlri), dst)
		}
		r.Walk(func(s string, v interface{}) bool {
			results = append(results, v.(*Destination))
			return false
		})
	default:
		for _, dst := range t.GetDestinations() {
			results = append(results, dst)
		}
		sort.Sort(destinations(results))
	}
	return results
}

func (t *Table) GetDestinations() map[string]*Destination {
	return t.destinations
}
func (t *Table) setDestinations(destinations map[string]*Destination) {
	t.destinations = destinations
}
func (t *Table) GetDestination(nlri bgp.AddrPrefixInterface) *Destination {
	dest, ok := t.destinations[t.tableKey(nlri)]
	if ok {
		return dest
	} else {
		return nil
	}
}

func (t *Table) GetLongerPrefixDestinations(key string) ([]*Destination, error) {
	results := make([]*Destination, 0, len(t.GetDestinations()))
	switch t.routeFamily {
	case bgp.RF_IPv4_UC, bgp.RF_IPv6_UC, bgp.RF_IPv4_MPLS, bgp.RF_IPv6_MPLS:
		_, prefix, err := net.ParseCIDR(key)
		if err != nil {
			return nil, err
		}
		k := CidrToRadixkey(prefix.String())
		r := radix.New()
		for _, dst := range t.GetDestinations() {
			r.Insert(AddrToRadixkey(dst.nlri), dst)
		}
		r.WalkPrefix(k, func(s string, v interface{}) bool {
			results = append(results, v.(*Destination))
			return false
		})
	default:
		for _, dst := range t.GetDestinations() {
			results = append(results, dst)
		}
	}
	return results, nil
}

func (t *Table) GetEvpnDestinationsWithRouteType(typ string) ([]*Destination, error) {
	var routeType uint8
	switch strings.ToLower(typ) {
	case "a-d":
		routeType = bgp.EVPN_ROUTE_TYPE_ETHERNET_AUTO_DISCOVERY
	case "macadv":
		routeType = bgp.EVPN_ROUTE_TYPE_MAC_IP_ADVERTISEMENT
	case "multicast":
		routeType = bgp.EVPN_INCLUSIVE_MULTICAST_ETHERNET_TAG
	case "esi":
		routeType = bgp.EVPN_ETHERNET_SEGMENT_ROUTE
	case "prefix":
		routeType = bgp.EVPN_IP_PREFIX
	default:
		return nil, fmt.Errorf("unsupported evpn route type: %s", typ)
	}
	destinations := t.GetDestinations()
	results := make([]*Destination, 0, len(destinations))
	switch t.routeFamily {
	case bgp.RF_EVPN:
		for _, dst := range destinations {
			if nlri, ok := dst.nlri.(*bgp.EVPNNLRI); !ok {
				return nil, fmt.Errorf("invalid evpn nlri type detected: %T", dst.nlri)
			} else if nlri.RouteType == routeType {
				results = append(results, dst)
			}
		}
	default:
		for _, dst := range destinations {
			results = append(results, dst)
		}
	}
	return results, nil
}

func (t *Table) setDestination(dst *Destination) {
	t.destinations[t.tableKey(dst.nlri)] = dst
}

func (t *Table) tableKey(nlri bgp.AddrPrefixInterface) string {
	switch T := nlri.(type) {
	case *bgp.IPAddrPrefix:
		b := make([]byte, 5)
		copy(b, T.Prefix.To4())
		b[4] = T.Length
		return *(*string)(unsafe.Pointer(&b))
	case *bgp.IPv6AddrPrefix:
		b := make([]byte, 17)
		copy(b, T.Prefix.To16())
		b[16] = T.Length
		return *(*string)(unsafe.Pointer(&b))
	}
	return nlri.String()
}

func (t *Table) Bests(id string, as uint32) []*Path {
	paths := make([]*Path, 0, len(t.destinations))
	for _, dst := range t.destinations {
		path := dst.GetBestPath(id, as)
		if path != nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func (t *Table) MultiBests(id string) [][]*Path {
	paths := make([][]*Path, 0, len(t.destinations))
	for _, dst := range t.destinations {
		path := dst.GetMultiBestPath(id)
		if path != nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func (t *Table) GetKnownPathList(id string, as uint32) []*Path {
	paths := make([]*Path, 0, len(t.destinations))
	for _, dst := range t.destinations {
		paths = append(paths, dst.GetKnownPathList(id, as)...)
	}
	return paths
}

func (t *Table) Select(option ...TableSelectOption) (*Table, error) {
	id := GLOBAL_RIB_NAME
	var vrf *Vrf
	adj := false
	prefixes := make([]*LookupPrefix, 0, len(option))
	best := false
	mp := false
	as := uint32(0)
	for _, o := range option {
		if o.ID != "" {
			id = o.ID
		}
		if o.VRF != nil {
			vrf = o.VRF
		}
		adj = o.adj
		prefixes = append(prefixes, o.LookupPrefixes...)
		best = o.Best
		mp = o.MultiPath
		as = o.AS
	}
	dOption := DestinationSelectOption{ID: id, AS: as, VRF: vrf, adj: adj, Best: best, MultiPath: mp}
	r := &Table{
		routeFamily:  t.routeFamily,
		destinations: make(map[string]*Destination),
	}

	if len(prefixes) != 0 {
		switch t.routeFamily {
		case bgp.RF_IPv4_UC, bgp.RF_IPv6_UC:
			f := func(prefixStr string) bool {
				var nlri bgp.AddrPrefixInterface
				if t.routeFamily == bgp.RF_IPv4_UC {
					nlri, _ = bgp.NewPrefixFromRouteFamily(bgp.AFI_IP, bgp.SAFI_UNICAST, prefixStr)
				} else {
					nlri, _ = bgp.NewPrefixFromRouteFamily(bgp.AFI_IP6, bgp.SAFI_UNICAST, prefixStr)
				}
				if dst := t.GetDestination(nlri); dst != nil {
					if d := dst.Select(dOption); d != nil {
						r.setDestination(d)
						return true
					}
				}
				return false
			}

			for _, p := range prefixes {
				key := p.Prefix
				switch p.LookupOption {
				case LOOKUP_LONGER:
					ds, err := t.GetLongerPrefixDestinations(key)
					if err != nil {
						return nil, err
					}
					for _, dst := range ds {
						if d := dst.Select(dOption); d != nil {
							r.setDestination(d)
						}
					}
				case LOOKUP_SHORTER:
					addr, prefix, err := net.ParseCIDR(key)
					if err != nil {
						return nil, err
					}
					ones, _ := prefix.Mask.Size()
					for i := ones; i >= 0; i-- {
						_, prefix, _ := net.ParseCIDR(fmt.Sprintf("%s/%d", addr.String(), i))
						f(prefix.String())
					}
				default:
					if host := net.ParseIP(key); host != nil {
						masklen := 32
						if t.routeFamily == bgp.RF_IPv6_UC {
							masklen = 128
						}
						for i := masklen; i >= 0; i-- {
							_, prefix, err := net.ParseCIDR(fmt.Sprintf("%s/%d", key, i))
							if err != nil {
								return nil, err
							}
							if f(prefix.String()) {
								break
							}
						}
					} else {
						f(key)
					}
				}
			}
		case bgp.RF_EVPN:
			for _, p := range prefixes {
				// Uses LookupPrefix.Prefix as EVPN Route Type string
				ds, err := t.GetEvpnDestinationsWithRouteType(p.Prefix)
				if err != nil {
					return nil, err
				}
				for _, dst := range ds {
					if d := dst.Select(dOption); d != nil {
						r.setDestination(d)
					}
				}
			}
		default:
			return nil, fmt.Errorf("route filtering is not supported for this family")
		}
	} else {
		for _, dst := range t.GetDestinations() {
			if d := dst.Select(dOption); d != nil {
				r.setDestination(d)
			}
		}
	}
	return r, nil
}

type TableInfo struct {
	NumDestination int
	NumPath        int
	NumAccepted    int
}

func (t *Table) Info(id string, as uint32) *TableInfo {
	var numD, numP int
	for _, d := range t.destinations {
		ps := d.GetKnownPathList(id, as)
		if len(ps) > 0 {
			numD += 1
			numP += len(ps)
		}
	}
	return &TableInfo{
		NumDestination: numD,
		NumPath:        numP,
	}
}
