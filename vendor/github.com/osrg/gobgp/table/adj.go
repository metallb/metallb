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
	id       string
	accepted map[bgp.RouteFamily]int
	table    map[bgp.RouteFamily]map[string]*Path
}

func NewAdjRib(id string, rfList []bgp.RouteFamily) *AdjRib {
	table := make(map[bgp.RouteFamily]map[string]*Path)
	for _, rf := range rfList {
		table[rf] = make(map[string]*Path)
	}
	return &AdjRib{
		id:       id,
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
				if old.Filtered(adj.id) != POLICY_DIRECTION_IN {
					adj.accepted[rf]--
				}
			}
		} else {
			n := path.Filtered(adj.id)
			if found {
				o := old.Filtered(adj.id)
				if o == POLICY_DIRECTION_IN && n == POLICY_DIRECTION_NONE {
					adj.accepted[rf]++
				} else if o != POLICY_DIRECTION_IN && n == POLICY_DIRECTION_IN {
					adj.accepted[rf]--
				}
			} else {
				if n == POLICY_DIRECTION_NONE {
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

func (adj *AdjRib) RefreshAcceptedNumber(rfList []bgp.RouteFamily) {
	for _, rf := range rfList {
		adj.accepted[rf] = 0
		for _, p := range adj.table[rf] {
			if p.Filtered(adj.id) != POLICY_DIRECTION_IN {
				adj.accepted[rf]++
			}
		}
	}
}

func (adj *AdjRib) PathList(rfList []bgp.RouteFamily, accepted bool) []*Path {
	pathList := make([]*Path, 0, adj.Count(rfList))
	for _, rf := range rfList {
		for _, rr := range adj.table[rf] {
			if accepted && rr.Filtered(adj.id) == POLICY_DIRECTION_IN {
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
			for _, p := range table {
				if p.IsStale() {
					delete(table, p.getPrefix())
					if p.Filtered(adj.id) == POLICY_DIRECTION_NONE {
						adj.accepted[rf]--
					}
					pathList = append(pathList, p.Clone(true))
				}
			}
		}
	}
	return pathList
}

func (adj *AdjRib) StaleAll(rfList []bgp.RouteFamily) {
	for _, rf := range rfList {
		if table, ok := adj.table[rf]; ok {
			for _, p := range table {
				p.MarkStale(true)
			}
		}
	}
}

func (adj *AdjRib) Exists(path *Path) bool {
	if path == nil {
		return false
	}
	family := path.GetRouteFamily()
	table, ok := adj.table[family]
	if !ok {
		return false
	}
	_, ok = table[path.getPrefix()]
	return ok
}

func (adj *AdjRib) Select(family bgp.RouteFamily, accepted bool, option ...TableSelectOption) (*Table, error) {
	paths := adj.PathList([]bgp.RouteFamily{family}, accepted)
	dsts := make(map[string]*Destination, len(paths))
	for _, path := range paths {
		if d, y := dsts[path.GetNlri().String()]; y {
			d.knownPathList = append(d.knownPathList, path)
		} else {
			dst := NewDestination(path.GetNlri(), 0)
			dsts[path.GetNlri().String()] = dst
			dst.knownPathList = append(dst.knownPathList, path)
		}
	}
	tbl := &Table{
		routeFamily:  family,
		destinations: dsts,
	}
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
