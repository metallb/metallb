// Copyright (C) 2014-2016 Nippon Telegraph and Telephone Corporation.
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
	"time"

	"github.com/eapache/channels"
	log "github.com/sirupsen/logrus"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
)

const (
	FLOP_THRESHOLD    = time.Second * 30
	MIN_CONNECT_RETRY = 10
)

type PeerGroup struct {
	Conf             *config.PeerGroup
	members          map[string]config.Neighbor
	dynamicNeighbors map[string]*config.DynamicNeighbor
}

func NewPeerGroup(c *config.PeerGroup) *PeerGroup {
	return &PeerGroup{
		Conf:             c,
		members:          make(map[string]config.Neighbor),
		dynamicNeighbors: make(map[string]*config.DynamicNeighbor),
	}
}

func (pg *PeerGroup) AddMember(c config.Neighbor) {
	pg.members[c.State.NeighborAddress] = c
}

func (pg *PeerGroup) DeleteMember(c config.Neighbor) {
	delete(pg.members, c.State.NeighborAddress)
}

func (pg *PeerGroup) AddDynamicNeighbor(c *config.DynamicNeighbor) {
	pg.dynamicNeighbors[c.Config.Prefix] = c
}

func newDynamicPeer(g *config.Global, neighborAddress string, pg *config.PeerGroup, loc *table.TableManager, policy *table.RoutingPolicy) *Peer {
	conf := config.Neighbor{
		Config: config.NeighborConfig{
			PeerGroup: pg.Config.PeerGroupName,
		},
		State: config.NeighborState{
			NeighborAddress: neighborAddress,
		},
		Transport: config.Transport{
			Config: config.TransportConfig{
				PassiveMode: true,
			},
		},
	}
	if err := config.OverwriteNeighborConfigWithPeerGroup(&conf, pg); err != nil {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   neighborAddress,
		}).Debugf("Can't overwrite neighbor config: %s", err)
		return nil
	}
	if err := config.SetDefaultNeighborConfigValues(&conf, pg, g); err != nil {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   neighborAddress,
		}).Debugf("Can't set default config: %s", err)
		return nil
	}
	peer := NewPeer(g, &conf, loc, policy)
	peer.fsm.state = bgp.BGP_FSM_ACTIVE
	return peer
}

type Peer struct {
	tableId           string
	fsm               *FSM
	adjRibIn          *table.AdjRib
	outgoing          *channels.InfiniteChannel
	policy            *table.RoutingPolicy
	localRib          *table.TableManager
	prefixLimitWarned map[bgp.RouteFamily]bool
	llgrEndChs        []chan struct{}
}

func NewPeer(g *config.Global, conf *config.Neighbor, loc *table.TableManager, policy *table.RoutingPolicy) *Peer {
	peer := &Peer{
		outgoing:          channels.NewInfiniteChannel(),
		localRib:          loc,
		policy:            policy,
		fsm:               NewFSM(g, conf, policy),
		prefixLimitWarned: make(map[bgp.RouteFamily]bool),
	}
	if peer.isRouteServerClient() {
		peer.tableId = conf.State.NeighborAddress
	} else {
		peer.tableId = table.GLOBAL_RIB_NAME
	}
	rfs, _ := config.AfiSafis(conf.AfiSafis).ToRfList()
	peer.adjRibIn = table.NewAdjRib(rfs)
	return peer
}

func (peer *Peer) AS() uint32 {
	return peer.fsm.pConf.State.PeerAs
}

func (peer *Peer) ID() string {
	return peer.fsm.pConf.State.NeighborAddress
}

func (peer *Peer) TableID() string {
	return peer.tableId
}

func (peer *Peer) isIBGPPeer() bool {
	return peer.fsm.pConf.State.PeerAs == peer.fsm.gConf.Config.As
}

func (peer *Peer) isRouteServerClient() bool {
	return peer.fsm.pConf.RouteServer.Config.RouteServerClient
}

func (peer *Peer) isRouteReflectorClient() bool {
	return peer.fsm.pConf.RouteReflector.Config.RouteReflectorClient
}

func (peer *Peer) isGracefulRestartEnabled() bool {
	return peer.fsm.pConf.GracefulRestart.State.Enabled
}

func (peer *Peer) getAddPathMode(family bgp.RouteFamily) bgp.BGPAddPathMode {
	if mode, y := peer.fsm.rfMap[family]; y {
		return mode
	}
	return bgp.BGP_ADD_PATH_NONE
}

func (peer *Peer) isAddPathReceiveEnabled(family bgp.RouteFamily) bool {
	return (peer.getAddPathMode(family) & bgp.BGP_ADD_PATH_RECEIVE) > 0
}

func (peer *Peer) isAddPathSendEnabled(family bgp.RouteFamily) bool {
	return (peer.getAddPathMode(family) & bgp.BGP_ADD_PATH_SEND) > 0
}

func (peer *Peer) isDynamicNeighbor() bool {
	return peer.fsm.pConf.Config.NeighborAddress == "" && peer.fsm.pConf.Config.NeighborInterface == ""
}

func (peer *Peer) recvedAllEOR() bool {
	for _, a := range peer.fsm.pConf.AfiSafis {
		if s := a.MpGracefulRestart.State; s.Enabled && !s.EndOfRibReceived {
			return false
		}
	}
	return true
}

func (peer *Peer) configuredRFlist() []bgp.RouteFamily {
	rfs, _ := config.AfiSafis(peer.fsm.pConf.AfiSafis).ToRfList()
	return rfs
}

func (peer *Peer) negotiatedRFList() []bgp.RouteFamily {
	l := make([]bgp.RouteFamily, 0, len(peer.fsm.rfMap))
	for family, _ := range peer.fsm.rfMap {
		l = append(l, family)
	}
	return l
}

func (peer *Peer) toGlobalFamilies(families []bgp.RouteFamily) []bgp.RouteFamily {
	if peer.fsm.pConf.Config.Vrf != "" {
		fs := make([]bgp.RouteFamily, 0, len(families))
		for _, f := range families {
			switch f {
			case bgp.RF_IPv4_UC:
				fs = append(fs, bgp.RF_IPv4_VPN)
			case bgp.RF_IPv6_UC:
				fs = append(fs, bgp.RF_IPv6_VPN)
			default:
				log.WithFields(log.Fields{
					"Topic":  "Peer",
					"Key":    peer.ID(),
					"Family": f,
					"VRF":    peer.fsm.pConf.Config.Vrf,
				}).Warn("invalid family configured for neighbor with vrf")
			}
		}
		families = fs
	}
	return families
}

func classifyFamilies(all, part []bgp.RouteFamily) ([]bgp.RouteFamily, []bgp.RouteFamily) {
	a := []bgp.RouteFamily{}
	b := []bgp.RouteFamily{}
	for _, f := range all {
		p := true
		for _, g := range part {
			if f == g {
				p = false
				a = append(a, f)
				break
			}
		}
		if p {
			b = append(b, f)
		}
	}
	return a, b
}

func (peer *Peer) forwardingPreservedFamilies() ([]bgp.RouteFamily, []bgp.RouteFamily) {
	list := []bgp.RouteFamily{}
	for _, a := range peer.fsm.pConf.AfiSafis {
		if s := a.MpGracefulRestart.State; s.Enabled && s.Received {
			list = append(list, a.State.Family)
		}
	}
	return classifyFamilies(peer.configuredRFlist(), list)
}

func (peer *Peer) llgrFamilies() ([]bgp.RouteFamily, []bgp.RouteFamily) {
	list := []bgp.RouteFamily{}
	for _, a := range peer.fsm.pConf.AfiSafis {
		if a.LongLivedGracefulRestart.State.Enabled {
			list = append(list, a.State.Family)
		}
	}
	return classifyFamilies(peer.configuredRFlist(), list)
}

func (peer *Peer) isLLGREnabledFamily(family bgp.RouteFamily) bool {
	if !peer.fsm.pConf.GracefulRestart.Config.LongLivedEnabled {
		return false
	}
	fs, _ := peer.llgrFamilies()
	for _, f := range fs {
		if f == family {
			return true
		}
	}
	return false
}

func (peer *Peer) llgrRestartTime(family bgp.RouteFamily) uint32 {
	for _, a := range peer.fsm.pConf.AfiSafis {
		if a.State.Family == family {
			return a.LongLivedGracefulRestart.State.PeerRestartTime
		}
	}
	return 0
}

func (peer *Peer) llgrRestartTimerExpired(family bgp.RouteFamily) bool {
	all := true
	for _, a := range peer.fsm.pConf.AfiSafis {
		if a.State.Family == family {
			a.LongLivedGracefulRestart.State.PeerRestartTimerExpired = true
		}
		s := a.LongLivedGracefulRestart.State
		if s.Received && !s.PeerRestartTimerExpired {
			all = false
		}
	}
	return all
}

func (peer *Peer) markLLGRStale(fs []bgp.RouteFamily) []*table.Path {
	paths := peer.adjRibIn.PathList(fs, true)
	for i, p := range paths {
		doStale := true
		for _, c := range p.GetCommunities() {
			if c == uint32(bgp.COMMUNITY_NO_LLGR) {
				doStale = false
				p = p.Clone(true)
				break
			}
		}
		if doStale {
			p = p.Clone(false)
			p.SetCommunities([]uint32{uint32(bgp.COMMUNITY_LLGR_STALE)}, false)
		}
		paths[i] = p
	}
	return paths
}

func (peer *Peer) stopPeerRestarting() {
	peer.fsm.pConf.GracefulRestart.State.PeerRestarting = false
	for _, ch := range peer.llgrEndChs {
		close(ch)
	}
	peer.llgrEndChs = make([]chan struct{}, 0)

}

func (peer *Peer) filterPathFromSourcePeer(path, old *table.Path) *table.Path {
	if peer.ID() != path.GetSource().Address.String() {
		return path
	}

	// Note: Multiple paths having the same prefix could exist the withdrawals
	// list in the case of Route Server setup with import policies modifying
	// paths. In such case, gobgp sends duplicated update messages; withdraw
	// messages for the same prefix.
	if !peer.isRouteServerClient() {
		if peer.isRouteReflectorClient() && path.GetRouteFamily() == bgp.RF_RTC_UC {
			// When the peer is a Route Reflector client and the given path
			// contains the Route Tartget Membership NLRI, the path should not
			// be withdrawn in order to signal the client to distribute routes
			// with the specific RT to Route Reflector.
			return path
		} else if !path.IsWithdraw && old != nil && old.GetSource().Address.String() != peer.ID() {
			// Say, peer A and B advertized same prefix P, and best path
			// calculation chose a path from B as best. When B withdraws prefix
			// P, best path calculation chooses the path from A as best. For
			// peers other than A, this path should be advertised (as implicit
			// withdrawal). However for A, we should advertise the withdrawal
			// path. Thing is same when peer A and we advertized prefix P (as
			// local route), then, we withdraws the prefix.
			return old.Clone(true)
		}
	}
	log.WithFields(log.Fields{
		"Topic": "Peer",
		"Key":   peer.ID(),
		"Data":  path,
	}).Debug("From me, ignore.")
	return nil
}

func (peer *Peer) doPrefixLimit(k bgp.RouteFamily, c *config.PrefixLimitConfig) *bgp.BGPMessage {
	if maxPrefixes := int(c.MaxPrefixes); maxPrefixes > 0 {
		count := peer.adjRibIn.Count([]bgp.RouteFamily{k})
		pct := int(c.ShutdownThresholdPct)
		if pct > 0 && !peer.prefixLimitWarned[k] && count > (maxPrefixes*pct/100) {
			peer.prefixLimitWarned[k] = true
			log.WithFields(log.Fields{
				"Topic":         "Peer",
				"Key":           peer.ID(),
				"AddressFamily": k.String(),
			}).Warnf("prefix limit %d%% reached", pct)
		}
		if count > maxPrefixes {
			log.WithFields(log.Fields{
				"Topic":         "Peer",
				"Key":           peer.ID(),
				"AddressFamily": k.String(),
			}).Warnf("prefix limit reached")
			return bgp.NewBGPNotificationMessage(bgp.BGP_ERROR_CEASE, bgp.BGP_ERROR_SUB_MAXIMUM_NUMBER_OF_PREFIXES_REACHED, nil)
		}
	}
	return nil

}

func (peer *Peer) updatePrefixLimitConfig(c []config.AfiSafi) error {
	x := peer.fsm.pConf.AfiSafis
	y := c
	if len(x) != len(y) {
		return fmt.Errorf("changing supported afi-safi is not allowed")
	}
	m := make(map[bgp.RouteFamily]config.PrefixLimitConfig)
	for _, e := range x {
		m[e.State.Family] = e.PrefixLimit.Config
	}
	for _, e := range y {
		if p, ok := m[e.State.Family]; !ok {
			return fmt.Errorf("changing supported afi-safi is not allowed")
		} else if !p.Equal(&e.PrefixLimit.Config) {
			log.WithFields(log.Fields{
				"Topic":                   "Peer",
				"Key":                     peer.ID(),
				"AddressFamily":           e.Config.AfiSafiName,
				"OldMaxPrefixes":          p.MaxPrefixes,
				"NewMaxPrefixes":          e.PrefixLimit.Config.MaxPrefixes,
				"OldShutdownThresholdPct": p.ShutdownThresholdPct,
				"NewShutdownThresholdPct": e.PrefixLimit.Config.ShutdownThresholdPct,
			}).Warnf("update prefix limit configuration")
			peer.prefixLimitWarned[e.State.Family] = false
			if msg := peer.doPrefixLimit(e.State.Family, &e.PrefixLimit.Config); msg != nil {
				sendFsmOutgoingMsg(peer, nil, msg, true)
			}
		}
	}
	peer.fsm.pConf.AfiSafis = c
	return nil
}

func (peer *Peer) handleUpdate(e *FsmMsg) ([]*table.Path, []bgp.RouteFamily, *bgp.BGPMessage) {
	m := e.MsgData.(*bgp.BGPMessage)
	update := m.Body.(*bgp.BGPUpdate)
	log.WithFields(log.Fields{
		"Topic":       "Peer",
		"Key":         peer.fsm.pConf.State.NeighborAddress,
		"nlri":        update.NLRI,
		"withdrawals": update.WithdrawnRoutes,
		"attributes":  update.PathAttributes,
	}).Debug("received update")
	peer.fsm.pConf.Timers.State.UpdateRecvTime = time.Now().Unix()
	if len(e.PathList) > 0 {
		paths := make([]*table.Path, 0, len(e.PathList))
		eor := []bgp.RouteFamily{}
		for _, path := range e.PathList {
			if path.IsEOR() {
				family := path.GetRouteFamily()
				log.WithFields(log.Fields{
					"Topic":         "Peer",
					"Key":           peer.ID(),
					"AddressFamily": family,
				}).Debug("EOR received")
				eor = append(eor, family)
				continue
			}
			// RFC4271 9.1.2 Phase 2: Route Selection
			//
			// If the AS_PATH attribute of a BGP route contains an AS loop, the BGP
			// route should be excluded from the Phase 2 decision function.
			if aspath := path.GetAsPath(); aspath != nil {
				if hasOwnASLoop(peer.fsm.peerInfo.LocalAS, int(peer.fsm.pConf.AsPathOptions.Config.AllowOwnAs), aspath) {
					path.SetAsLooped(true)
					continue
				}
			}
			paths = append(paths, path)
		}
		peer.adjRibIn.Update(e.PathList)
		for _, af := range peer.fsm.pConf.AfiSafis {
			if msg := peer.doPrefixLimit(af.State.Family, &af.PrefixLimit.Config); msg != nil {
				return nil, nil, msg
			}
		}
		return paths, eor, nil
	}
	return nil, nil, nil
}

func (peer *Peer) startFSMHandler(incoming *channels.InfiniteChannel, stateCh chan *FsmMsg) {
	peer.fsm.h = NewFSMHandler(peer.fsm, incoming, stateCh, peer.outgoing)
}

func (peer *Peer) StaleAll(rfList []bgp.RouteFamily) []*table.Path {
	return peer.adjRibIn.StaleAll(rfList)
}

func (peer *Peer) PassConn(conn *net.TCPConn) {
	select {
	case peer.fsm.connCh <- conn:
	default:
		conn.Close()
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   peer.ID(),
		}).Warn("accepted conn is closed to avoid be blocked")
	}
}

func (peer *Peer) DropAll(rfList []bgp.RouteFamily) {
	peer.adjRibIn.Drop(rfList)
}

func (peer *Peer) stopFSM() error {
	failed := false
	addr := peer.fsm.pConf.State.NeighborAddress
	t1 := time.AfterFunc(time.Minute*5, func() {
		log.WithFields(log.Fields{
			"Topic": "Peer",
		}).Warnf("Failed to free the fsm.h.t for %s", addr)
		failed = true
	})
	peer.fsm.h.t.Kill(nil)
	peer.fsm.h.t.Wait()
	t1.Stop()
	if !failed {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   addr,
		}).Debug("freed fsm.h.t")
		cleanInfiniteChannel(peer.outgoing)
	}
	failed = false
	t2 := time.AfterFunc(time.Minute*5, func() {
		log.WithFields(log.Fields{
			"Topic": "Peer",
		}).Warnf("Failed to free the fsm.t for %s", addr)
		failed = true
	})
	peer.fsm.t.Kill(nil)
	peer.fsm.t.Wait()
	t2.Stop()
	if !failed {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   addr,
		}).Debug("freed fsm.t")
		return nil
	}
	return fmt.Errorf("Failed to free FSM for %s", addr)
}
