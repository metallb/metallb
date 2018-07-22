// Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
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

	"github.com/osrg/gobgp/packet/bgp"
)

type AdjRib struct {
	accepted map[bgp.RouteFamily]int
	table    map[bgp.RouteFamily]map[string]*Path
}

func NewAdjRib(rfList []bgp.RouteFamily) *AdjRib {
	table := make(map[bgp.RouteFamily]map[string]*Path)
	for _, rf := range rfList {
		table[rf] = make(map[string]*Path)
	}
	return &AdjRib{
		table:    table,
		accepted: make(map[bgp.RouteFamily]int),
	}
}

func (adj *AdjRib) Update(pathList []*Path) {
	for _, path := range pathList {
		if path == nil || path.IsEOR() {
			continue
		}
		rf := path.GetRouteFamily()
		key := fmt.Sprintf("%d:%s", path.GetNlri().PathIdentifier(), path.getPrefix())

		old, found := adj.table[rf][key]
		if path.IsWithdraw {
			if found {
				delete(adj.table[rf], key)
				if !old.IsAsLooped() {
					adj.accepted[rf]--
				}
			}
		} else {
			if found {
				if old.IsAsLooped() && !path.IsAsLooped() {
					adj.accepted[rf]++
				} else if !old.IsAsLooped() && path.IsAsLooped() {
					adj.accepted[rf]--
				}
			} else {
				if !path.IsAsLooped() {
					adj.accepted[rf]++
				}
			}
			if found && old.Equal(path) {
				path.setTimestamp(old.GetTimestamp())
			}
			adj.table[rf][key] = path
		}
	}
}

func (adj *AdjRib) PathList(rfList []bgp.RouteFamily, accepted bool) []*Path {
	pathList := make([]*Path, 0, adj.Count(rfList))
	for _, rf := range rfList {
		for _, rr := range adj.table[rf] {
			if accepted && rr.IsAsLooped() {
				continue
			}
			pathList = append(pathList, rr)
		}
	}
	return pathList
}

func (adj *AdjRib) Count(rfList []bgp.RouteFamily) int {
	count := 0
	for _, rf := range rfList {
		if table, ok := adj.table[rf]; ok {
			count += len(table)
		}
	}
	return count
}

func (adj *AdjRib) Accepted(rfList []bgp.RouteFamily) int {
	count := 0
	for _, rf := range rfList {
		if n, ok := adj.accepted[rf]; ok {
			count += n
		}
	}
	return count
}

func (adj *AdjRib) Drop(rfList []bgp.RouteFamily) {
	for _, rf := range rfList {
		if _, ok := adj.table[rf]; ok {
			adj.table[rf] = make(map[string]*Path)
			adj.accepted[rf] = 0
		}
	}
}

func (adj *AdjRib) DropStale(rfList []bgp.RouteFamily) []*Path {
	pathList := make([]*Path, 0, adj.Count(rfList))
	for _, rf := range rfList {
		if table, ok := adj.table[rf]; ok {
			for k, p := range table {
				if p.IsStale() {
					delete(table, k)
					if !p.IsAsLooped() {
						adj.accepted[rf]--
					}
					pathList = append(pathList, p.Clone(true))
				}
			}
		}
	}
	return pathList
}

func (adj *AdjRib) StaleAll(rfList []bgp.RouteFamily) []*Path {
	pathList := make([]*Path, 0)
	for _, rf := range rfList {
		if table, ok := adj.table[rf]; ok {
			l := make([]*Path, 0, len(table))
			for k, p := range table {
				n := p.Clone(false)
				n.MarkStale(true)
				table[k] = n
				l = append(l, n)
			}
			if len(l) > 0 {
				pathList = append(pathList, l...)
			}
		}
	}
	return pathList
}

func (adj *AdjRib) Select(family bgp.RouteFamily, accepted bool, option ...TableSelectOption) (*Table, error) {
	m := make(map[string][]*Path)
	pl := adj.PathList([]bgp.RouteFamily{family}, accepted)
	for _, path := range pl {
		key := path.GetNlri().String()
		if _, y := m[key]; y {
			m[key] = append(m[key], path)
		} else {
			m[key] = []*Path{path}
		}
	}
	d := make([]*Destination, 0, len(pl))
	for _, l := range m {
		d = append(d, NewDestination(l[0].GetNlri(), 0, l...))
	}
	tbl := NewTable(family, d...)
	option = append(option, TableSelectOption{adj: true})
	return tbl.Select(option...)
}

func (adj *AdjRib) TableInfo(family bgp.RouteFamily) (*TableInfo, error) {
	if _, ok := adj.table[family]; !ok {
		return nil, fmt.Errorf("%s unsupported", family)
	}
	c := adj.Count([]bgp.RouteFamily{family})
	a := adj.Accepted([]bgp.RouteFamily{family})
	return &TableInfo{
		NumDestination: c,
		NumPath:        c,
		NumAccepted:    a,
	}, nil
}
