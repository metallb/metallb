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
	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
	log "github.com/sirupsen/logrus"
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
		members:          make(map[string]config.Neighbor, 0),
		dynamicNeighbors: make(map[string]*config.DynamicNeighbor, 0),
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
	peer.adjRibIn = table.NewAdjRib(peer.ID(), rfs)
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
			if c == bgp.COMMUNITY_NO_LLGR {
				doStale = false
				p = p.Clone(true)
				break
			}
		}
		if doStale {
			p = p.Clone(false)
			p.SetCommunities([]uint32{bgp.COMMUNITY_LLGR_STALE}, false)
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

func (peer *Peer) getAccepted(rfList []bgp.RouteFamily) []*table.Path {
	return peer.adjRibIn.PathList(rfList, true)
}

func (peer *Peer) filterpath(path, old *table.Path) *table.Path {
	// special handling for RTC nlri
	// see comments in (*Destination).Calculate()
	if path != nil && path.GetRouteFamily() == bgp.RF_RTC_UC && !path.IsWithdraw {

		// If we already sent the same nlri, send unnecessary
		// update. Fix this after the API change between table
		// and server packages.

		dst := peer.localRib.GetDestination(path)
		path = nil
		// we send a path even if it is not a best path
		for _, p := range dst.GetKnownPathList(peer.TableID()) {
			// just take care not to send back it
			if peer.ID() != p.GetSource().Address.String() {
				path = p
				break
			}
		}
	}

	// only allow vpnv4 and vpnv6 paths to be advertised to VRFed neighbors.
	// also check we can import this path using table.CanImportToVrf()
	// if we can, make it local path by calling (*Path).ToLocal()
	if path != nil && peer.fsm.pConf.Config.Vrf != "" {
		if f := path.GetRouteFamily(); f != bgp.RF_IPv4_VPN && f != bgp.RF_IPv6_VPN {
			return nil
		}
		vrf := peer.localRib.Vrfs[peer.fsm.pConf.Config.Vrf]
		if table.CanImportToVrf(vrf, path) {
			path = path.ToLocal()
		} else {
			return nil
		}
	}

	// replace-peer-as handling
	if path != nil && !path.IsWithdraw && peer.fsm.pConf.AsPathOptions.State.ReplacePeerAs {
		path = path.ReplaceAS(peer.fsm.pConf.Config.LocalAs, peer.fsm.pConf.Config.PeerAs)
	}

	if path = filterpath(peer, path, old); path == nil {
		return nil
	}

	path = table.UpdatePathAttrs(peer.fsm.gConf, peer.fsm.pConf, peer.fsm.peerInfo, path)

	options := &table.PolicyOptions{
		Info: peer.fsm.peerInfo,
	}
	path = peer.policy.ApplyPolicy(peer.TableID(), table.POLICY_DIRECTION_EXPORT, path, options)
	// When 'path' is filetered (path == nil), check 'old' has been sent to this peer.
	// If it has, send withdrawal to the peer.
	if path == nil && old != nil {
		o := peer.policy.ApplyPolicy(peer.TableID(), table.POLICY_DIRECTION_EXPORT, old, options)
		if o != nil {
			path = old.Clone(true)
		}
	}

	// draft-uttaro-idr-bgp-persistence-02
	// 4.3.  Processing LLGR_STALE Routes
	//
	// The route SHOULD NOT be advertised to any neighbor from which the
	// Long-lived Graceful Restart Capability has not been received.  The
	// exception is described in the Optional Partial Deployment
	// Procedure section (Section 4.7).  Note that this requirement
	// implies that such routes should be withdrawn from any such neighbor.
	if path != nil && !path.IsWithdraw && !peer.isLLGREnabledFamily(path.GetRouteFamily()) && path.IsLLGRStale() {
		// we send unnecessary withdrawn even if we didn't
		// sent the route.
		path = path.Clone(true)
	}

	// remove local-pref attribute
	// we should do this after applying export policy since policy may
	// set local-preference
	if path != nil && !peer.isIBGPPeer() && !peer.isRouteServerClient() {
		path.RemoveLocalPref()
	}
	return path
}

func (peer *Peer) getBestFromLocal(rfList []bgp.RouteFamily) ([]*table.Path, []*table.Path) {
	pathList := []*table.Path{}
	filtered := []*table.Path{}
	for _, family := range peer.toGlobalFamilies(rfList) {
		pl := func() []*table.Path {
			if peer.isAddPathSendEnabled(family) {
				return peer.localRib.GetPathList(peer.TableID(), []bgp.RouteFamily{family})
			}
			return peer.localRib.GetBestPathList(peer.TableID(), []bgp.RouteFamily{family})
		}()
		for _, path := range pl {
			if p := peer.filterpath(path, nil); p != nil {
				pathList = append(pathList, p)
			} else {
				filtered = append(filtered, path)
			}
		}
	}
	if peer.isGracefulRestartEnabled() {
		for _, family := range rfList {
			pathList = append(pathList, table.NewEOR(family))
		}
	}
	return pathList, filtered
}

func (peer *Peer) processOutgoingPaths(paths, olds []*table.Path) []*table.Path {
	if peer.fsm.state != bgp.BGP_FSM_ESTABLISHED {
		return nil
	}
	if peer.fsm.pConf.GracefulRestart.State.LocalRestarting {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   peer.fsm.pConf.State.NeighborAddress,
		}).Debug("now syncing, suppress sending updates")
		return nil
	}

	outgoing := make([]*table.Path, 0, len(paths))

	for idx, path := range paths {
		var old *table.Path
		if olds != nil {
			old = olds[idx]
		}
		if p := peer.filterpath(path, old); p != nil {
			outgoing = append(outgoing, p)
		}
	}
	return outgoing
}

func (peer *Peer) handleRouteRefresh(e *FsmMsg) []*table.Path {
	m := e.MsgData.(*bgp.BGPMessage)
	rr := m.Body.(*bgp.BGPRouteRefresh)
	rf := bgp.AfiSafiToRouteFamily(rr.AFI, rr.SAFI)
	if _, ok := peer.fsm.rfMap[rf]; !ok {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   peer.ID(),
			"Data":  rf,
		}).Warn("Route family isn't supported")
		return nil
	}
	if _, ok := peer.fsm.capMap[bgp.BGP_CAP_ROUTE_REFRESH]; !ok {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   peer.ID(),
		}).Warn("ROUTE_REFRESH received but the capability wasn't advertised")
		return nil
	}
	rfList := []bgp.RouteFamily{rf}
	accepted, filtered := peer.getBestFromLocal(rfList)
	for _, path := range filtered {
		path.IsWithdraw = true
		accepted = append(accepted, path)
	}
	return accepted
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
		peer.adjRibIn.Update(e.PathList)
		for _, af := range peer.fsm.pConf.AfiSafis {
			if msg := peer.doPrefixLimit(af.State.Family, &af.PrefixLimit.Config); msg != nil {
				return nil, nil, msg
			}
		}
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
			if path.Filtered(peer.ID()) != table.POLICY_DIRECTION_IN {
				paths = append(paths, path)
			} else {
				paths = append(paths, path.Clone(true))
			}
		}
		return paths, eor, nil
	}
	return nil, nil, nil
}

func (peer *Peer) startFSMHandler(incoming *channels.InfiniteChannel, stateCh chan *FsmMsg) {
	peer.fsm.h = NewFSMHandler(peer.fsm, incoming, stateCh, peer.outgoing)
}

func (peer *Peer) StaleAll(rfList []bgp.RouteFamily) {
	peer.adjRibIn.StaleAll(rfList)
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

func (peer *Peer) ToConfig(getAdvertised bool) *config.Neighbor {
	// create copy which can be access to without mutex
	conf := *peer.fsm.pConf

	conf.AfiSafis = make([]config.AfiSafi, len(peer.fsm.pConf.AfiSafis))
	for i, af := range peer.fsm.pConf.AfiSafis {
		conf.AfiSafis[i] = af
		conf.AfiSafis[i].AddPaths.State.Receive = peer.isAddPathReceiveEnabled(af.State.Family)
		if peer.isAddPathSendEnabled(af.State.Family) {
			conf.AfiSafis[i].AddPaths.State.SendMax = af.AddPaths.State.SendMax
		} else {
			conf.AfiSafis[i].AddPaths.State.SendMax = 0
		}
	}

	remoteCap := make([]bgp.ParameterCapabilityInterface, 0, len(peer.fsm.capMap))
	for _, caps := range peer.fsm.capMap {
		for _, m := range caps {
			// need to copy all values here
			buf, _ := m.Serialize()
			c, _ := bgp.DecodeCapability(buf)
			remoteCap = append(remoteCap, c)
		}
	}
	conf.State.RemoteCapabilityList = remoteCap
	conf.State.LocalCapabilityList = capabilitiesFromConfig(peer.fsm.pConf)

	conf.State.SessionState = config.IntToSessionStateMap[int(peer.fsm.state)]
	conf.State.AdminState = config.IntToAdminStateMap[int(peer.fsm.adminState)]

	if peer.fsm.state == bgp.BGP_FSM_ESTABLISHED {
		rfList := peer.configuredRFlist()
		if getAdvertised {
			pathList, filtered := peer.getBestFromLocal(rfList)
			conf.State.AdjTable.Advertised = uint32(len(pathList))
			conf.State.AdjTable.Filtered = uint32(len(filtered))
		} else {
			conf.State.AdjTable.Advertised = 0
		}
		conf.State.AdjTable.Received = uint32(peer.adjRibIn.Count(rfList))
		conf.State.AdjTable.Accepted = uint32(peer.adjRibIn.Accepted(rfList))

		conf.Transport.State.LocalAddress, conf.Transport.State.LocalPort = peer.fsm.LocalHostPort()
		_, conf.Transport.State.RemotePort = peer.fsm.RemoteHostPort()
		buf, _ := peer.fsm.recvOpen.Serialize()
		// need to copy all values here
		conf.State.ReceivedOpenMessage, _ = bgp.ParseBGPMessage(buf)
		conf.State.RemoteRouterId = peer.fsm.peerInfo.ID.To4().String()
	}
	return &conf
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
