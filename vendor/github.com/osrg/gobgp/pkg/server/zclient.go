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

package server

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/osrg/gobgp/internal/pkg/table"
	"github.com/osrg/gobgp/internal/pkg/zebra"
	"github.com/osrg/gobgp/pkg/packet/bgp"

	log "github.com/sirupsen/logrus"
)

// nexthopStateCache stores a map of nexthop IP to metric value. Especially,
// the metric value of math.MaxUint32 means the nexthop is unreachable.
type nexthopStateCache map[string]uint32

func (m nexthopStateCache) applyToPathList(paths []*table.Path) []*table.Path {
	updated := make([]*table.Path, 0, len(paths))
	for _, path := range paths {
		if path == nil || path.IsWithdraw {
			continue
		}
		metric, ok := m[path.GetNexthop().String()]
		if !ok {
			continue
		}
		isNexthopInvalid := metric == math.MaxUint32
		med, err := path.GetMed()
		if err == nil && med == metric && path.IsNexthopInvalid == isNexthopInvalid {
			// If the nexthop state of the given path is already up to date,
			// skips this path.
			continue
		}
		newPath := path.Clone(false)
		if isNexthopInvalid {
			newPath.IsNexthopInvalid = true
		} else {
			newPath.IsNexthopInvalid = false
			newPath.SetMed(int64(metric), true)
		}
		updated = append(updated, newPath)
	}
	return updated
}

func (m nexthopStateCache) updateByNexthopUpdate(body *zebra.NexthopUpdateBody) (updated bool) {
	if len(body.Nexthops) == 0 {
		// If NEXTHOP_UPDATE message does not contain any nexthop, the given
		// nexthop is unreachable.
		if _, ok := m[body.Prefix.Prefix.String()]; !ok {
			// Zebra will send an empty NEXTHOP_UPDATE message as the fist
			// response for the NEXTHOP_REGISTER message. Here ignores it.
			return false
		}
		m[body.Prefix.Prefix.String()] = math.MaxUint32 // means unreachable
	} else {
		m[body.Prefix.Prefix.String()] = body.Metric
	}
	return true
}

func (m nexthopStateCache) filterPathToRegister(paths []*table.Path) []*table.Path {
	filteredPaths := make([]*table.Path, 0, len(paths))
	for _, path := range paths {
		// Here filters out:
		// - Nil path
		// - Withdrawn path
		// - External path (advertised from Zebra) in order avoid sending back
		// - Unspecified nexthop address
		// - Already registered nexthop
		if path == nil || path.IsWithdraw || path.IsFromExternal() {
			continue
		} else if nexthop := path.GetNexthop(); nexthop.IsUnspecified() {
			continue
		} else if _, ok := m[nexthop.String()]; ok {
			continue
		}
		filteredPaths = append(filteredPaths, path)
	}
	return filteredPaths
}

func filterOutExternalPath(paths []*table.Path) []*table.Path {
	filteredPaths := make([]*table.Path, 0, len(paths))
	for _, path := range paths {
		// Here filters out:
		// - Nil path
		// - External path (advertised from Zebra) in order avoid sending back
		// - Unreachable path because invalidated by Zebra
		if path == nil || path.IsFromExternal() || path.IsNexthopInvalid {
			continue
		}
		filteredPaths = append(filteredPaths, path)
	}
	return filteredPaths
}

func newIPRouteBody(dst []*table.Path) (body *zebra.IPRouteBody, isWithdraw bool) {
	paths := filterOutExternalPath(dst)
	if len(paths) == 0 {
		return nil, false
	}
	path := paths[0]

	l := strings.SplitN(path.GetNlri().String(), "/", 2)
	var prefix net.IP
	//nexthops := make([]net.IP, 0, len(paths))
	var nexthop zebra.Nexthop
	nexthops := make([]zebra.Nexthop, 0, len(paths))
	switch path.GetRouteFamily() {
	case bgp.RF_IPv4_UC, bgp.RF_IPv4_VPN:
		if path.GetRouteFamily() == bgp.RF_IPv4_UC {
			prefix = path.GetNlri().(*bgp.IPAddrPrefix).IPAddrPrefixDefault.Prefix.To4()
		} else {
			prefix = path.GetNlri().(*bgp.LabeledVPNIPAddrPrefix).IPAddrPrefixDefault.Prefix.To4()
		}
		for _, p := range paths {
			nexthop.Gate = p.GetNexthop().To4()
			nexthops = append(nexthops, nexthop)
		}
	case bgp.RF_IPv6_UC, bgp.RF_IPv6_VPN:
		if path.GetRouteFamily() == bgp.RF_IPv6_UC {
			prefix = path.GetNlri().(*bgp.IPv6AddrPrefix).IPAddrPrefixDefault.Prefix.To16()
		} else {
			prefix = path.GetNlri().(*bgp.LabeledVPNIPv6AddrPrefix).IPAddrPrefixDefault.Prefix.To16()
		}
		for _, p := range paths {
			nexthop.Gate = p.GetNexthop().To16()
			nexthops = append(nexthops, nexthop)
		}
	default:
		return nil, false
	}
	msgFlags := zebra.MESSAGE_NEXTHOP
	plen, _ := strconv.ParseUint(l[1], 10, 8)
	med, err := path.GetMed()
	if err == nil {
		msgFlags |= zebra.MESSAGE_METRIC
	}
	var flags zebra.FLAG
	info := path.GetSource()
	if info.AS == info.LocalAS {
		flags = zebra.FLAG_IBGP | zebra.FLAG_INTERNAL
	} else if info.MultihopTtl > 0 {
		flags = zebra.FLAG_INTERNAL
	}
	return &zebra.IPRouteBody{
		Type:    zebra.ROUTE_BGP,
		Flags:   flags,
		SAFI:    zebra.SAFI_UNICAST,
		Message: msgFlags,
		Prefix: zebra.Prefix{
			Prefix:    prefix,
			PrefixLen: uint8(plen),
		},
		Nexthops: nexthops,
		Metric:   med,
	}, path.IsWithdraw
}

func newNexthopRegisterBody(paths []*table.Path, nexthopCache nexthopStateCache) *zebra.NexthopRegisterBody {
	paths = nexthopCache.filterPathToRegister(paths)
	if len(paths) == 0 {
		return nil
	}
	path := paths[0]

	family := path.GetRouteFamily()
	nexthops := make([]*zebra.RegisteredNexthop, 0, len(paths))
	for _, p := range paths {
		nexthop := p.GetNexthop()
		var nh *zebra.RegisteredNexthop
		switch family {
		case bgp.RF_IPv4_UC, bgp.RF_IPv4_VPN:
			nh = &zebra.RegisteredNexthop{
				Family: syscall.AF_INET,
				Prefix: nexthop.To4(),
			}
		case bgp.RF_IPv6_UC, bgp.RF_IPv6_VPN:
			nh = &zebra.RegisteredNexthop{
				Family: syscall.AF_INET6,
				Prefix: nexthop.To16(),
			}
		default:
			continue
		}
		nexthops = append(nexthops, nh)
	}

	// If no nexthop needs to be registered or unregistered, skips to send
	// message.
	if len(nexthops) == 0 {
		return nil
	}

	return &zebra.NexthopRegisterBody{
		Nexthops: nexthops,
	}
}

func newNexthopUnregisterBody(family uint16, prefix net.IP) *zebra.NexthopRegisterBody {
	return &zebra.NexthopRegisterBody{
		Nexthops: []*zebra.RegisteredNexthop{{
			Family: family,
			Prefix: prefix,
		}},
	}
}

func newPathFromIPRouteMessage(m *zebra.Message, version uint8) *table.Path {
	header := m.Header
	body := m.Body.(*zebra.IPRouteBody)
	family := body.RouteFamily(version)
	isWithdraw := body.IsWithdraw(version)

	var nlri bgp.AddrPrefixInterface
	pattr := make([]bgp.PathAttributeInterface, 0)
	origin := bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP)
	pattr = append(pattr, origin)

	log.WithFields(log.Fields{
		"Topic":        "Zebra",
		"RouteType":    body.Type.String(),
		"Flag":         body.Flags.String(),
		"Message":      body.Message,
		"Family":       body.Prefix.Family,
		"Prefix":       body.Prefix.Prefix,
		"PrefixLength": body.Prefix.PrefixLen,
		"Nexthop":      body.Nexthops,
		"Metric":       body.Metric,
		"Distance":     body.Distance,
		"Mtu":          body.Mtu,
		"api":          header.Command.String(),
	}).Debugf("create path from ip route message.")

	switch family {
	case bgp.RF_IPv4_UC:
		nlri = bgp.NewIPAddrPrefix(body.Prefix.PrefixLen, body.Prefix.Prefix.String())
		if len(body.Nexthops) > 0 {
			pattr = append(pattr, bgp.NewPathAttributeNextHop(body.Nexthops[0].Gate.String()))
		}
	case bgp.RF_IPv6_UC:
		nlri = bgp.NewIPv6AddrPrefix(body.Prefix.PrefixLen, body.Prefix.Prefix.String())
		nexthop := ""
		if len(body.Nexthops) > 0 {
			nexthop = body.Nexthops[0].Gate.String()
		}
		pattr = append(pattr, bgp.NewPathAttributeMpReachNLRI(nexthop, []bgp.AddrPrefixInterface{nlri}))
	default:
		log.WithFields(log.Fields{
			"Topic": "Zebra",
		}).Errorf("unsupport address family: %s", family)
		return nil
	}

	med := bgp.NewPathAttributeMultiExitDisc(body.Metric)
	pattr = append(pattr, med)

	path := table.NewPath(nil, nlri, isWithdraw, pattr, time.Now(), false)
	path.SetIsFromExternal(true)
	return path
}

type zebraClient struct {
	client       *zebra.Client
	server       *BgpServer
	nexthopCache nexthopStateCache
	dead         chan struct{}
}

func (z *zebraClient) getPathListWithNexthopUpdate(body *zebra.NexthopUpdateBody) []*table.Path {
	rib := &table.TableManager{
		Tables: make(map[bgp.RouteFamily]*table.Table),
	}

	var rfList []bgp.RouteFamily
	switch body.Prefix.Family {
	case syscall.AF_INET:
		rfList = []bgp.RouteFamily{bgp.RF_IPv4_UC, bgp.RF_IPv4_VPN}
	case syscall.AF_INET6:
		rfList = []bgp.RouteFamily{bgp.RF_IPv6_UC, bgp.RF_IPv6_VPN}
	}

	for _, rf := range rfList {
		tbl, _, err := z.server.getRib("", rf, nil)
		if err != nil {
			log.WithFields(log.Fields{
				"Topic":  "Zebra",
				"Family": rf.String(),
				"Error":  err,
			}).Error("failed to get global rib")
			continue
		}
		rib.Tables[rf] = tbl
	}

	return rib.GetPathListWithNexthop(table.GLOBAL_RIB_NAME, rfList, body.Prefix.Prefix)
}

func (z *zebraClient) updatePathByNexthopCache(paths []*table.Path) {
	paths = z.nexthopCache.applyToPathList(paths)
	if len(paths) > 0 {
		if err := z.server.updatePath("", paths); err != nil {
			log.WithFields(log.Fields{
				"Topic":    "Zebra",
				"PathList": paths,
			}).Error("failed to update nexthop reachability")
		}
	}
}

func (z *zebraClient) loop() {
	w := z.server.watch([]watchOption{
		watchBestPath(true),
		watchPostUpdate(true),
	}...)
	defer w.Stop()

	for {
		select {
		case <-z.dead:
			return
		case msg := <-z.client.Receive():
			switch body := msg.Body.(type) {
			case *zebra.IPRouteBody:
				if path := newPathFromIPRouteMessage(msg, z.client.Version); path != nil {
					if err := z.server.addPathList("", []*table.Path{path}); err != nil {
						log.WithFields(log.Fields{
							"Topic": "Zebra",
							"Path":  path,
							"Error": err,
						}).Error("failed to add path from zebra")
					}
				}
			case *zebra.NexthopUpdateBody:
				if updated := z.nexthopCache.updateByNexthopUpdate(body); !updated {
					continue
				}
				paths := z.getPathListWithNexthopUpdate(body)
				if len(paths) == 0 {
					// If there is no path bound for the given nexthop, send
					// NEXTHOP_UNREGISTER message.
					z.client.SendNexthopRegister(msg.Header.VrfId, newNexthopUnregisterBody(uint16(body.Prefix.Family), body.Prefix.Prefix), true)
					delete(z.nexthopCache, body.Prefix.Prefix.String())
				}
				z.updatePathByNexthopCache(paths)
			}
		case ev := <-w.Event():
			switch msg := ev.(type) {
			case *watchEventBestPath:
				if table.UseMultiplePaths.Enabled {
					for _, paths := range msg.MultiPathList {
						z.updatePathByNexthopCache(paths)
						if body, isWithdraw := newIPRouteBody(paths); body != nil {
							z.client.SendIPRoute(0, body, isWithdraw)
						}
						if body := newNexthopRegisterBody(paths, z.nexthopCache); body != nil {
							z.client.SendNexthopRegister(0, body, false)
						}
					}
				} else {
					z.updatePathByNexthopCache(msg.PathList)
					for _, path := range msg.PathList {
						vrfs := []uint32{0}
						if msg.Vrf != nil {
							if v, ok := msg.Vrf[path.GetNlri().String()]; ok {
								vrfs = append(vrfs, v)
							}
						}
						for _, i := range vrfs {
							if body, isWithdraw := newIPRouteBody([]*table.Path{path}); body != nil {
								z.client.SendIPRoute(i, body, isWithdraw)
							}
							if body := newNexthopRegisterBody([]*table.Path{path}, z.nexthopCache); body != nil {
								z.client.SendNexthopRegister(i, body, false)
							}
						}
					}
				}
			case *watchEventUpdate:
				if body := newNexthopRegisterBody(msg.PathList, z.nexthopCache); body != nil {
					vrfID := uint32(0)
					for _, vrf := range z.server.listVrf() {
						if vrf.Name == msg.Neighbor.Config.Vrf {
							vrfID = uint32(vrf.Id)
						}
					}
					z.client.SendNexthopRegister(vrfID, body, false)
				}
			}
		}
	}
}

func newZebraClient(s *BgpServer, url string, protos []string, version uint8, nhtEnable bool, nhtDelay uint8) (*zebraClient, error) {
	l := strings.SplitN(url, ":", 2)
	if len(l) != 2 {
		return nil, fmt.Errorf("unsupported url: %s", url)
	}
	var cli *zebra.Client
	var err error
	for _, ver := range []uint8{version, 2, 3, 4, 5, 6} {
		cli, err = zebra.NewClient(l[0], l[1], zebra.ROUTE_BGP, ver)
		if err == nil {
			break
		}
		// Retry with another Zebra message version
		log.WithFields(log.Fields{
			"Topic": "Zebra",
		}).Warnf("cannot connect to Zebra with message version %d. going to retry another version...", ver)
	}
	if cli == nil {
		return nil, err
	}
	// Note: HELLO/ROUTER_ID_ADD messages are automatically sent to negotiate
	// the Zebra message version in zebra.NewClient().
	// cli.SendHello()
	// cli.SendRouterIDAdd()
	cli.SendInterfaceAdd()
	for _, typ := range protos {
		t, err := zebra.RouteTypeFromString(typ, version)
		if err != nil {
			return nil, err
		}
		cli.SendRedistribute(t, zebra.VRF_DEFAULT)
	}
	w := &zebraClient{
		client:       cli,
		server:       s,
		nexthopCache: make(nexthopStateCache),
		dead:         make(chan struct{}),
	}
	go w.loop()
	return w, nil
}
