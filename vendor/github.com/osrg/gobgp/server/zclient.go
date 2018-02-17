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
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
	"github.com/osrg/gobgp/zebra"
)

type pathList []*table.Path

type nexthopTrackingManager struct {
	dead              chan struct{}
	nexthopCache      map[string]struct{}
	server            *BgpServer
	delay             int
	isScheduled       bool
	scheduledPathList map[string]pathList
	trigger           chan struct{}
	pathListCh        chan pathList
}

func newNexthopTrackingManager(server *BgpServer, delay int) *nexthopTrackingManager {
	return &nexthopTrackingManager{
		dead:              make(chan struct{}),
		nexthopCache:      make(map[string]struct{}),
		server:            server,
		delay:             delay,
		scheduledPathList: make(map[string]pathList, 0),
		trigger:           make(chan struct{}),
		pathListCh:        make(chan pathList),
	}
}

func (m *nexthopTrackingManager) stop() {
	close(m.pathListCh)
	close(m.trigger)
	close(m.dead)
}

func (m *nexthopTrackingManager) isRegisteredNexthop(nexthop net.IP) bool {
	key := nexthop.String()
	_, ok := m.nexthopCache[key]
	return ok
}

func (m *nexthopTrackingManager) registerNexthop(nexthop net.IP) bool {
	key := nexthop.String()
	if _, ok := m.nexthopCache[key]; ok {
		return false
	}
	m.nexthopCache[key] = struct{}{}
	return true
}

func (m *nexthopTrackingManager) unregisterNexthop(nexthop net.IP) {
	key := nexthop.String()
	delete(m.nexthopCache, key)
}

func (m *nexthopTrackingManager) appendPathList(paths pathList) {
	if len(paths) == 0 {
		return
	}
	path := paths[0]

	m.scheduledPathList[path.GetNexthop().String()] = paths
}

func (m *nexthopTrackingManager) calculateDelay(penalty int) int {
	if penalty <= 950 {
		return m.delay
	}

	delay := 8
	for penalty > 950 {
		delay += 8
		penalty /= 2
	}
	return delay
}

func (m *nexthopTrackingManager) triggerUpdatePathAfter(delay int) {
	time.Sleep(time.Duration(delay) * time.Second)

	m.trigger <- struct{}{}
}

func (m *nexthopTrackingManager) loop() {
	t := time.NewTicker(8 * time.Second)
	defer t.Stop()

	penalty := 0

	for {
		select {
		case <-m.dead:
			return

		case <-t.C:
			penalty /= 2

		case paths := <-m.pathListCh:
			penalty += 500
			log.WithFields(log.Fields{
				"Topic": "Zebra",
				"Event": "Nexthop Tracking",
			}).Debugf("penalty 500 charged: penalty: %d", penalty)

			m.appendPathList(paths)

			isScheduled := m.isScheduled
			if isScheduled {
				log.WithFields(log.Fields{
					"Topic": "Zebra",
					"Event": "Nexthop Tracking",
				}).Debug("nexthop tracking event already scheduled")
				continue
			} else {
				m.isScheduled = true
			}

			delay := m.calculateDelay(penalty)
			go m.triggerUpdatePathAfter(delay)
			log.WithFields(log.Fields{
				"Topic": "Zebra",
				"Event": "Nexthop Tracking",
			}).Debugf("nexthop tracking event scheduled in %d secs", delay)

		case <-m.trigger:
			paths := make(pathList, 0)
			for _, pList := range m.scheduledPathList {
				for _, p := range pList {
					paths = append(paths, p)
				}
			}
			log.WithFields(log.Fields{
				"Topic": "Zebra",
				"Event": "Nexthop Tracking",
			}).Debugf("update nexthop reachability: %s", paths)

			if err := m.server.UpdatePath("", paths); err != nil {
				log.WithFields(log.Fields{
					"Topic": "Zebra",
					"Event": "Nexthop Tracking",
				}).Error("failed to update nexthop reachability")
			}

			m.isScheduled = false
			m.scheduledPathList = make(map[string]pathList, 0)
		}
	}
}

func (m *nexthopTrackingManager) scheduleUpdate(paths pathList) {
	if len(paths) == 0 {
		return
	}
	m.pathListCh <- paths
}

func (m *nexthopTrackingManager) filterPathToRegister(paths pathList) pathList {
	filteredPaths := make(pathList, 0, len(paths))
	for _, path := range paths {
		if path == nil || path.IsFromExternal() {
			continue
		}
		// NEXTHOP_UNREGISTER message will be sent when GoBGP received
		// NEXTHOP_UPDATE message and there is no path bound for the updated
		// nexthop.
		// Here filters out withdraw paths and paths whose nexthop is:
		// - already invalidated
		// - already registered
		// - unspecified address
		if path.IsWithdraw || path.IsNexthopInvalid {
			continue
		}
		nexthop := path.GetNexthop()
		if m.isRegisteredNexthop(nexthop) || nexthop.IsUnspecified() {
			continue
		}
		filteredPaths = append(filteredPaths, path)
	}
	return filteredPaths
}

func filterOutExternalPath(paths pathList) pathList {
	filteredPaths := make(pathList, 0, len(paths))
	for _, path := range paths {
		if path == nil || path.IsFromExternal() {
			continue
		}
		filteredPaths = append(filteredPaths, path)
	}
	return filteredPaths
}

func newIPRouteBody(dst pathList) (body *zebra.IPRouteBody, isWithdraw bool) {
	paths := filterOutExternalPath(dst)
	if len(paths) == 0 {
		return nil, false
	}
	path := paths[0]

	l := strings.SplitN(path.GetNlri().String(), "/", 2)
	var prefix net.IP
	nexthops := make([]net.IP, 0, len(paths))
	switch path.GetRouteFamily() {
	case bgp.RF_IPv4_UC, bgp.RF_IPv4_VPN:
		if path.GetRouteFamily() == bgp.RF_IPv4_UC {
			prefix = path.GetNlri().(*bgp.IPAddrPrefix).IPAddrPrefixDefault.Prefix.To4()
		} else {
			prefix = path.GetNlri().(*bgp.LabeledVPNIPAddrPrefix).IPAddrPrefixDefault.Prefix.To4()
		}
		for _, p := range paths {
			nexthops = append(nexthops, p.GetNexthop().To4())
		}
	case bgp.RF_IPv6_UC, bgp.RF_IPv6_VPN:
		if path.GetRouteFamily() == bgp.RF_IPv6_UC {
			prefix = path.GetNlri().(*bgp.IPv6AddrPrefix).IPAddrPrefixDefault.Prefix.To16()
		} else {
			prefix = path.GetNlri().(*bgp.LabeledVPNIPv6AddrPrefix).IPAddrPrefixDefault.Prefix.To16()
		}
		for _, p := range paths {
			nexthops = append(nexthops, p.GetNexthop().To16())
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
		Type:         zebra.ROUTE_BGP,
		Flags:        flags,
		SAFI:         zebra.SAFI_UNICAST,
		Message:      msgFlags,
		Prefix:       prefix,
		PrefixLength: uint8(plen),
		Nexthops:     nexthops,
		Metric:       med,
	}, path.IsWithdraw
}

func newNexthopRegisterBody(dst pathList, nhtManager *nexthopTrackingManager) (body *zebra.NexthopRegisterBody, isWithdraw bool) {
	if nhtManager == nil {
		return nil, false
	}

	paths := nhtManager.filterPathToRegister(dst)
	if len(paths) == 0 {
		return nil, false
	}
	path := paths[0]

	if path.IsWithdraw == true {
		// NEXTHOP_UNREGISTER message will be sent when GoBGP received
		// NEXTHOP_UPDATE message and there is no path bound for the updated
		// nexthop. So there is nothing to do here.
		return nil, true
	}

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
		nhtManager.registerNexthop(nexthop)
	}

	// If no nexthop needs to be registered or unregistered,
	// skips to send message.
	if len(nexthops) == 0 {
		return nil, path.IsWithdraw
	}

	return &zebra.NexthopRegisterBody{
		Nexthops: nexthops,
	}, path.IsWithdraw
}

func createPathFromIPRouteMessage(m *zebra.Message) *table.Path {
	header := m.Header
	body := m.Body.(*zebra.IPRouteBody)
	family := body.RouteFamily()
	isWithdraw := body.IsWithdraw()

	var nlri bgp.AddrPrefixInterface
	pattr := make([]bgp.PathAttributeInterface, 0)
	origin := bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP)
	pattr = append(pattr, origin)

	log.WithFields(log.Fields{
		"Topic":        "Zebra",
		"RouteType":    body.Type.String(),
		"Flag":         body.Flags.String(),
		"Message":      body.Message,
		"Prefix":       body.Prefix,
		"PrefixLength": body.PrefixLength,
		"Nexthop":      body.Nexthops,
		"IfIndex":      body.Ifindexs,
		"Metric":       body.Metric,
		"Distance":     body.Distance,
		"Mtu":          body.Mtu,
		"api":          header.Command.String(),
	}).Debugf("create path from ip route message.")

	switch family {
	case bgp.RF_IPv4_UC:
		nlri = bgp.NewIPAddrPrefix(body.PrefixLength, body.Prefix.String())
		if len(body.Nexthops) > 0 {
			pattr = append(pattr, bgp.NewPathAttributeNextHop(body.Nexthops[0].String()))
		}
	case bgp.RF_IPv6_UC:
		nlri = bgp.NewIPv6AddrPrefix(body.PrefixLength, body.Prefix.String())
		nexthop := ""
		if len(body.Nexthops) > 0 {
			nexthop = body.Nexthops[0].String()
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

func createPathListFromNexthopUpdateMessage(m *zebra.Message, manager *table.TableManager, nhtManager *nexthopTrackingManager) (pathList, *zebra.NexthopRegisterBody, error) {
	body := m.Body.(*zebra.NexthopUpdateBody)
	isNexthopInvalid := len(body.Nexthops) == 0

	var rfList []bgp.RouteFamily
	switch body.Family {
	case uint16(syscall.AF_INET):
		rfList = []bgp.RouteFamily{bgp.RF_IPv4_UC, bgp.RF_IPv4_VPN}
	case uint16(syscall.AF_INET6):
		rfList = []bgp.RouteFamily{bgp.RF_IPv6_UC, bgp.RF_IPv6_VPN}
	default:
		return nil, nil, fmt.Errorf("invalid address family: %d", body.Family)
	}

	paths := manager.GetPathListWithNexthop(table.GLOBAL_RIB_NAME, rfList, body.Prefix)
	pathsLen := len(paths)

	// If there is no path bound for the updated nexthop, send
	// NEXTHOP_UNREGISTER message.
	var nexthopUnregisterBody *zebra.NexthopRegisterBody
	if pathsLen == 0 {
		nexthopUnregisterBody = &zebra.NexthopRegisterBody{
			Nexthops: []*zebra.RegisteredNexthop{{
				Family: body.Family,
				Prefix: body.Prefix,
			}},
		}
		nhtManager.unregisterNexthop(body.Prefix)
	}

	updatedPathList := make(pathList, 0, pathsLen)
	for _, path := range paths {
		newPath := path.Clone(false)
		if isNexthopInvalid {
			// If NEXTHOP_UPDATE message does NOT contain any nexthop,
			// invalidates the nexthop reachability.
			newPath.IsNexthopInvalid = true
		} else {
			// If NEXTHOP_UPDATE message contains valid nexthops,
			// copies Metric into MED.
			newPath.IsNexthopInvalid = false
			newPath.SetMed(int64(body.Metric), true)
		}
		updatedPathList = append(updatedPathList, newPath)
	}

	return updatedPathList, nexthopUnregisterBody, nil
}

type zebraClient struct {
	client     *zebra.Client
	server     *BgpServer
	dead       chan struct{}
	nhtManager *nexthopTrackingManager
}

func (z *zebraClient) stop() {
	close(z.dead)
}

func (z *zebraClient) loop() {
	w := z.server.Watch([]WatchOption{
		WatchBestPath(true),
		WatchPostUpdate(true),
	}...)
	defer w.Stop()

	if z.nhtManager != nil {
		go z.nhtManager.loop()
		defer z.nhtManager.stop()
	}

	for {
		select {
		case <-z.dead:
			return
		case msg := <-z.client.Receive():
			switch body := msg.Body.(type) {
			case *zebra.IPRouteBody:
				if p := createPathFromIPRouteMessage(msg); p != nil {
					if _, err := z.server.AddPath("", pathList{p}); err != nil {
						log.Errorf("failed to add path from zebra: %s", p)
					}
				}
			case *zebra.NexthopUpdateBody:
				if z.nhtManager != nil {
					if paths, b, err := createPathListFromNexthopUpdateMessage(msg, z.server.globalRib, z.nhtManager); err != nil {
						log.Errorf("failed to create updated path list related to nexthop %s", body.Prefix.String())
					} else {
						z.nhtManager.scheduleUpdate(paths)
						if b != nil {
							z.client.SendNexthopRegister(msg.Header.VrfId, b, true)
						}
					}
				}
			}
		case ev := <-w.Event():
			switch msg := ev.(type) {
			case *WatchEventBestPath:
				if table.UseMultiplePaths.Enabled {
					for _, dst := range msg.MultiPathList {
						if body, isWithdraw := newIPRouteBody(dst); body != nil {
							z.client.SendIPRoute(0, body, isWithdraw)
						}
						if body, isWithdraw := newNexthopRegisterBody(dst, z.nhtManager); body != nil {
							z.client.SendNexthopRegister(0, body, isWithdraw)
						}
					}
				} else {
					for _, path := range msg.PathList {
						if len(path.VrfIds) == 0 {
							path.VrfIds = []uint16{0}
						}
						for _, i := range path.VrfIds {
							if body, isWithdraw := newIPRouteBody(pathList{path}); body != nil {
								z.client.SendIPRoute(i, body, isWithdraw)
							}
							if body, isWithdraw := newNexthopRegisterBody(pathList{path}, z.nhtManager); body != nil {
								z.client.SendNexthopRegister(i, body, isWithdraw)
							}
						}
					}
				}
			case *WatchEventUpdate:
				if body, isWithdraw := newNexthopRegisterBody(msg.PathList, z.nhtManager); body != nil {
					vrfId := uint16(0)
					for _, vrf := range z.server.GetVrf() {
						if vrf.Name == msg.Neighbor.Config.Vrf {
							vrfId = uint16(vrf.Id)
						}
					}
					z.client.SendNexthopRegister(vrfId, body, isWithdraw)
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
	for _, ver := range []uint8{version, 2, 3, 4} {
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
		t, err := zebra.RouteTypeFromString(typ)
		if err != nil {
			return nil, err
		}
		cli.SendRedistribute(t, zebra.VRF_DEFAULT)
	}
	var nhtManager *nexthopTrackingManager = nil
	if nhtEnable {
		nhtManager = newNexthopTrackingManager(s, int(nhtDelay))
	}
	w := &zebraClient{
		dead:       make(chan struct{}),
		client:     cli,
		server:     s,
		nhtManager: nhtManager,
	}
	go w.loop()
	return w, nil
}
