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
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/eapache/channels"
	log "github.com/sirupsen/logrus"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
)

type TCPListener struct {
	l  *net.TCPListener
	ch chan struct{}
}

func (l *TCPListener) Close() error {
	if err := l.l.Close(); err != nil {
		return err
	}
	t := time.NewTicker(time.Second)
	select {
	case <-l.ch:
	case <-t.C:
		return fmt.Errorf("close timeout")
	}
	return nil
}

// avoid mapped IPv6 address
func NewTCPListener(address string, port uint32, ch chan *net.TCPConn) (*TCPListener, error) {
	proto := "tcp4"
	if ip := net.ParseIP(address); ip == nil {
		return nil, fmt.Errorf("can't listen on %s", address)
	} else if ip.To4() == nil {
		proto = "tcp6"
	}
	addr, err := net.ResolveTCPAddr(proto, net.JoinHostPort(address, strconv.Itoa(int(port))))
	if err != nil {
		return nil, err
	}

	l, err := net.ListenTCP(proto, addr)
	if err != nil {
		return nil, err
	}
	// Note: Set TTL=255 for incoming connection listener in order to accept
	// connection in case for the neighbor has TTL Security settings.
	if err := SetListenTcpTTLSockopt(l, 255); err != nil {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   addr,
		}).Warnf("cannot set TTL(=%d) for TCPListener: %s", 255, err)
	}

	closeCh := make(chan struct{})
	go func() error {
		for {
			conn, err := l.AcceptTCP()
			if err != nil {
				close(closeCh)
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Error": err,
				}).Warn("Failed to AcceptTCP")
				return err
			}
			ch <- conn
		}
	}()
	return &TCPListener{
		l:  l,
		ch: closeCh,
	}, nil
}

type BgpServer struct {
	bgpConfig     config.Bgp
	fsmincomingCh *channels.InfiniteChannel
	fsmStateCh    chan *FsmMsg
	acceptCh      chan *net.TCPConn

	mgmtCh       chan *mgmtOp
	policy       *table.RoutingPolicy
	listeners    []*TCPListener
	neighborMap  map[string]*Peer
	peerGroupMap map[string]*PeerGroup
	globalRib    *table.TableManager
	rsRib        *table.TableManager
	roaManager   *roaManager
	shutdown     bool
	watcherMap   map[WatchEventType][]*Watcher
	zclient      *zebraClient
	bmpManager   *bmpClientManager
	mrtManager   *mrtManager
}

func NewBgpServer() *BgpServer {
	roaManager, _ := NewROAManager(0)
	s := &BgpServer{
		neighborMap:  make(map[string]*Peer),
		peerGroupMap: make(map[string]*PeerGroup),
		policy:       table.NewRoutingPolicy(),
		roaManager:   roaManager,
		mgmtCh:       make(chan *mgmtOp, 1),
		watcherMap:   make(map[WatchEventType][]*Watcher),
	}
	s.bmpManager = newBmpClientManager(s)
	s.mrtManager = newMrtManager(s)
	return s
}

func (server *BgpServer) Listeners(addr string) []*net.TCPListener {
	list := make([]*net.TCPListener, 0, len(server.listeners))
	rhs := net.ParseIP(addr).To4() != nil
	for _, l := range server.listeners {
		host, _, _ := net.SplitHostPort(l.l.Addr().String())
		lhs := net.ParseIP(host).To4() != nil
		if lhs == rhs {
			list = append(list, l.l)
		}
	}
	return list
}

func (s *BgpServer) active() error {
	if s.bgpConfig.Global.Config.As == 0 {
		return fmt.Errorf("bgp server hasn't started yet")
	}
	return nil
}

type mgmtOp struct {
	f           func() error
	errCh       chan error
	checkActive bool // check BGP global setting is configured before calling f()
}

func (server *BgpServer) handleMGMTOp(op *mgmtOp) {
	if op.checkActive {
		if err := server.active(); err != nil {
			op.errCh <- err
			return
		}
	}
	op.errCh <- op.f()
}

func (s *BgpServer) mgmtOperation(f func() error, checkActive bool) (err error) {
	ch := make(chan error)
	defer func() { err = <-ch }()
	s.mgmtCh <- &mgmtOp{
		f:           f,
		errCh:       ch,
		checkActive: checkActive,
	}
	return
}

func (server *BgpServer) Serve() {
	server.listeners = make([]*TCPListener, 0, 2)
	server.fsmincomingCh = channels.NewInfiniteChannel()
	server.fsmStateCh = make(chan *FsmMsg, 4096)

	handleFsmMsg := func(e *FsmMsg) {
		peer, found := server.neighborMap[e.MsgSrc]
		if !found {
			log.WithFields(log.Fields{
				"Topic": "Peer",
			}).Warnf("Cant't find the neighbor %s", e.MsgSrc)
			return
		}
		if e.Version != peer.fsm.version {
			log.WithFields(log.Fields{
				"Topic": "Peer",
			}).Debug("FSM version inconsistent")
			return
		}
		server.handleFSMMessage(peer, e)
	}

	for {
		passConn := func(conn *net.TCPConn) {
			host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
			ipaddr, _ := net.ResolveIPAddr("ip", host)
			remoteAddr := ipaddr.String()
			peer, found := server.neighborMap[remoteAddr]
			if found {
				if peer.fsm.adminState != ADMIN_STATE_UP {
					log.WithFields(log.Fields{
						"Topic":       "Peer",
						"Remote Addr": remoteAddr,
						"Admin State": peer.fsm.adminState,
					}).Debug("New connection for non admin-state-up peer")
					conn.Close()
					return
				}
				localAddrValid := func(laddr string) bool {
					if laddr == "0.0.0.0" || laddr == "::" {
						return true
					}
					l := conn.LocalAddr()
					if l == nil {
						// already closed
						return false
					}

					host, _, _ := net.SplitHostPort(l.String())
					if host != laddr {
						log.WithFields(log.Fields{
							"Topic":           "Peer",
							"Key":             remoteAddr,
							"Configured addr": laddr,
							"Addr":            host,
						}).Info("Mismatched local address")
						return false
					}
					return true
				}(peer.fsm.pConf.Transport.Config.LocalAddress)
				if localAddrValid == false {
					conn.Close()
					return
				}
				log.WithFields(log.Fields{
					"Topic": "Peer",
				}).Debugf("Accepted a new passive connection from:%s", remoteAddr)
				peer.PassConn(conn)
			} else if pg := server.matchLongestDynamicNeighborPrefix(remoteAddr); pg != nil {
				log.WithFields(log.Fields{
					"Topic": "Peer",
				}).Debugf("Accepted a new dynamic neighbor from:%s", remoteAddr)
				peer := newDynamicPeer(&server.bgpConfig.Global, remoteAddr, pg.Conf, server.globalRib, server.policy)
				if peer == nil {
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   remoteAddr,
					}).Infof("Can't create new Dynamic Peer")
					conn.Close()
					return
				}
				server.policy.Reset(nil, map[string]config.ApplyPolicy{peer.ID(): peer.fsm.pConf.ApplyPolicy})
				server.neighborMap[remoteAddr] = peer
				peer.startFSMHandler(server.fsmincomingCh, server.fsmStateCh)
				server.broadcastPeerState(peer, bgp.BGP_FSM_ACTIVE)
				peer.PassConn(conn)
			} else {
				log.WithFields(log.Fields{
					"Topic": "Peer",
				}).Infof("Can't find configuration for a new passive connection from:%s", remoteAddr)
				conn.Close()
			}
		}

		select {
		case op := <-server.mgmtCh:
			server.handleMGMTOp(op)
		case conn := <-server.acceptCh:
			passConn(conn)
		default:
		}

		for {
			select {
			case e := <-server.fsmStateCh:
				handleFsmMsg(e)
			default:
				goto CONT
			}
		}
	CONT:

		select {
		case op := <-server.mgmtCh:
			server.handleMGMTOp(op)
		case rmsg := <-server.roaManager.ReceiveROA():
			server.roaManager.HandleROAEvent(rmsg)
		case conn := <-server.acceptCh:
			passConn(conn)
		case e, ok := <-server.fsmincomingCh.Out():
			if !ok {
				continue
			}
			handleFsmMsg(e.(*FsmMsg))
		case e := <-server.fsmStateCh:
			handleFsmMsg(e)
		}
	}
}

func (server *BgpServer) matchLongestDynamicNeighborPrefix(a string) *PeerGroup {
	ipAddr := net.ParseIP(a)
	longestMask := net.CIDRMask(0, 32).String()
	var longestPG *PeerGroup
	for _, pg := range server.peerGroupMap {
		for _, d := range pg.dynamicNeighbors {
			_, netAddr, _ := net.ParseCIDR(d.Config.Prefix)
			if netAddr.Contains(ipAddr) {
				if netAddr.Mask.String() > longestMask {
					longestMask = netAddr.Mask.String()
					longestPG = pg
				}
			}
		}
	}
	return longestPG
}

func sendFsmOutgoingMsg(peer *Peer, paths []*table.Path, notification *bgp.BGPMessage, stayIdle bool) {
	peer.outgoing.In() <- &FsmOutgoingMsg{
		Paths:        paths,
		Notification: notification,
		StayIdle:     stayIdle,
	}
}

func isASLoop(peer *Peer, path *table.Path) bool {
	for _, as := range path.GetAsList() {
		if as == peer.AS() {
			return true
		}
	}
	return false
}

func filterpath(peer *Peer, path, old *table.Path) *table.Path {
	if path == nil {
		return nil
	}
	if _, ok := peer.fsm.rfMap[path.GetRouteFamily()]; !ok {
		return nil
	}

	//iBGP handling
	if peer.isIBGPPeer() {
		ignore := false
		//RFC4684 Constrained Route Distribution
		if _, y := peer.fsm.rfMap[bgp.RF_RTC_UC]; y && path.GetRouteFamily() != bgp.RF_RTC_UC {
			ignore = true
			for _, ext := range path.GetExtCommunities() {
				for _, path := range peer.adjRibIn.PathList([]bgp.RouteFamily{bgp.RF_RTC_UC}, true) {
					rt := path.GetNlri().(*bgp.RouteTargetMembershipNLRI).RouteTarget
					if rt == nil {
						ignore = false
					} else if ext.String() == rt.String() {
						ignore = false
						break
					}
				}
				if !ignore {
					break
				}
			}
		}

		if !path.IsLocal() {
			ignore = true
			info := path.GetSource()
			//if the path comes from eBGP peer
			if info.AS != peer.AS() {
				ignore = false
			}
			// RFC4456 8. Avoiding Routing Information Loops
			// A router that recognizes the ORIGINATOR_ID attribute SHOULD
			// ignore a route received with its BGP Identifier as the ORIGINATOR_ID.
			if id := path.GetOriginatorID(); peer.fsm.gConf.Config.RouterId == id.String() {
				log.WithFields(log.Fields{
					"Topic":        "Peer",
					"Key":          peer.ID(),
					"OriginatorID": id,
					"Data":         path,
				}).Debug("Originator ID is mine, ignore")
				return nil
			}
			if info.RouteReflectorClient {
				ignore = false
			}
			if peer.isRouteReflectorClient() {
				// RFC4456 8. Avoiding Routing Information Loops
				// If the local CLUSTER_ID is found in the CLUSTER_LIST,
				// the advertisement received SHOULD be ignored.
				for _, clusterId := range path.GetClusterList() {
					if clusterId.Equal(peer.fsm.peerInfo.RouteReflectorClusterID) {
						log.WithFields(log.Fields{
							"Topic":     "Peer",
							"Key":       peer.ID(),
							"ClusterID": clusterId,
							"Data":      path,
						}).Debug("cluster list path attribute has local cluster id, ignore")
						return nil
					}
				}
				ignore = false
			}
		}

		if ignore {
			if !path.IsWithdraw && old != nil {
				oldSource := old.GetSource()
				if old.IsLocal() || oldSource.Address.String() != peer.ID() && oldSource.AS != peer.AS() {
					// In this case, we suppose this peer has the same prefix
					// received from another iBGP peer.
					// So we withdraw the old best which was injected locally
					// (from CLI or gRPC for example) in order to avoid the
					// old best left on peers.
					// Also, we withdraw the eBGP route which is the old best.
					// When we got the new best from iBGP, we don't advertise
					// the new best and need to withdraw the old best.
					return old.Clone(true)
				}
			}
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   peer.ID(),
				"Data":  path,
			}).Debug("From same AS, ignore.")
			return nil
		}
	}

	if peer.ID() == path.GetSource().Address.String() {
		// Note: multiple paths having the same prefix could exist the
		// withdrawals list in the case of Route Server setup with
		// import policies modifying paths. In such case, gobgp sends
		// duplicated update messages; withdraw messages for the same
		// prefix.
		if !peer.isRouteServerClient() {
			// Say, peer A and B advertized same prefix P, and
			// best path calculation chose a path from B as best.
			// When B withdraws prefix P, best path calculation chooses
			// the path from A as best.
			// For peers other than A, this path should be advertised
			// (as implicit withdrawal). However for A, we should advertise
			// the withdrawal path.
			// Thing is same when peer A and we advertized prefix P (as local
			// route), then, we withdraws the prefix.
			if !path.IsWithdraw && old != nil && old.GetSource().Address.String() != peer.ID() {
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

	if !peer.isRouteServerClient() && isASLoop(peer, path) {
		return nil
	}
	return path
}

func clonePathList(pathList []*table.Path) []*table.Path {
	l := make([]*table.Path, 0, len(pathList))
	for _, p := range pathList {
		if p != nil {
			l = append(l, p.Clone(p.IsWithdraw))
		}
	}
	return l
}

func (server *BgpServer) notifyBestWatcher(best []*table.Path, multipath [][]*table.Path) {
	clonedM := make([][]*table.Path, len(multipath))
	for i, pathList := range multipath {
		clonedM[i] = clonePathList(pathList)
	}
	clonedB := clonePathList(best)
	for _, p := range clonedB {
		switch p.GetRouteFamily() {
		case bgp.RF_IPv4_VPN, bgp.RF_IPv6_VPN:
			for _, vrf := range server.globalRib.Vrfs {
				if vrf.Id != 0 && table.CanImportToVrf(vrf, p) {
					p.VrfIds = append(p.VrfIds, uint16(vrf.Id))
				}
			}
		}
	}
	server.notifyWatcher(WATCH_EVENT_TYPE_BEST_PATH, &WatchEventBestPath{PathList: clonedB, MultiPathList: clonedM})
}

func (server *BgpServer) notifyPrePolicyUpdateWatcher(peer *Peer, pathList []*table.Path, msg *bgp.BGPMessage, timestamp time.Time, payload []byte) {
	if !server.isWatched(WATCH_EVENT_TYPE_PRE_UPDATE) || peer == nil {
		return
	}
	cloned := clonePathList(pathList)
	if len(cloned) == 0 {
		return
	}
	_, y := peer.fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]
	l, _ := peer.fsm.LocalHostPort()
	ev := &WatchEventUpdate{
		Message:      msg,
		PeerAS:       peer.fsm.peerInfo.AS,
		LocalAS:      peer.fsm.peerInfo.LocalAS,
		PeerAddress:  peer.fsm.peerInfo.Address,
		LocalAddress: net.ParseIP(l),
		PeerID:       peer.fsm.peerInfo.ID,
		FourBytesAs:  y,
		Timestamp:    timestamp,
		Payload:      payload,
		PostPolicy:   false,
		PathList:     cloned,
		Neighbor:     peer.ToConfig(false),
	}
	server.notifyWatcher(WATCH_EVENT_TYPE_PRE_UPDATE, ev)
}

func (server *BgpServer) notifyPostPolicyUpdateWatcher(peer *Peer, pathList []*table.Path) {
	if !server.isWatched(WATCH_EVENT_TYPE_POST_UPDATE) || peer == nil {
		return
	}
	cloned := clonePathList(pathList)
	if len(cloned) == 0 {
		return
	}
	_, y := peer.fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]
	l, _ := peer.fsm.LocalHostPort()
	ev := &WatchEventUpdate{
		PeerAS:       peer.fsm.peerInfo.AS,
		LocalAS:      peer.fsm.peerInfo.LocalAS,
		PeerAddress:  peer.fsm.peerInfo.Address,
		LocalAddress: net.ParseIP(l),
		PeerID:       peer.fsm.peerInfo.ID,
		FourBytesAs:  y,
		Timestamp:    cloned[0].GetTimestamp(),
		PostPolicy:   true,
		PathList:     cloned,
		Neighbor:     peer.ToConfig(false),
	}
	server.notifyWatcher(WATCH_EVENT_TYPE_POST_UPDATE, ev)
}

func dstsToPaths(id string, dsts []*table.Destination, addpath bool) ([]*table.Path, []*table.Path, [][]*table.Path) {
	bestList := make([]*table.Path, 0, len(dsts))
	oldList := make([]*table.Path, 0, len(dsts))
	mpathList := make([][]*table.Path, 0, len(dsts))

	for _, dst := range dsts {
		if addpath {
			bestList = append(bestList, dst.GetAddPathChanges(id)...)
		} else {
			best, old, mpath := dst.GetChanges(id, false)
			bestList = append(bestList, best)
			oldList = append(oldList, old)
			if mpath != nil {
				mpathList = append(mpathList, mpath)
			}
		}
	}
	if addpath {
		oldList = nil
	}
	return bestList, oldList, mpathList
}

func (server *BgpServer) dropPeerAllRoutes(peer *Peer, families []bgp.RouteFamily) {
	var gBestList, bestList []*table.Path
	var mpathList [][]*table.Path
	families = peer.toGlobalFamilies(families)
	rib := server.globalRib
	if peer.isRouteServerClient() {
		rib = server.rsRib
	}
	for _, rf := range families {
		dsts := rib.DeletePathsByPeer(peer.fsm.peerInfo, rf)
		if !peer.isRouteServerClient() {
			gBestList, _, mpathList = dstsToPaths(table.GLOBAL_RIB_NAME, dsts, false)
			server.notifyBestWatcher(gBestList, mpathList)
		}

		for _, targetPeer := range server.neighborMap {
			if peer.isRouteServerClient() != targetPeer.isRouteServerClient() || targetPeer == peer {
				continue
			}
			if targetPeer.isAddPathSendEnabled(rf) {
				bestList, _, _ = dstsToPaths(targetPeer.TableID(), dsts, true)
			} else if targetPeer.isRouteServerClient() {
				bestList, _, _ = dstsToPaths(targetPeer.TableID(), dsts, false)
			} else {
				bestList = gBestList
			}
			if paths := targetPeer.processOutgoingPaths(bestList, nil); len(paths) > 0 {
				sendFsmOutgoingMsg(targetPeer, paths, nil, false)
			}
		}
	}
}

func createWatchEventPeerState(peer *Peer) *WatchEventPeerState {
	_, rport := peer.fsm.RemoteHostPort()
	laddr, lport := peer.fsm.LocalHostPort()
	sentOpen := buildopen(peer.fsm.gConf, peer.fsm.pConf)
	recvOpen := peer.fsm.recvOpen
	return &WatchEventPeerState{
		PeerAS:        peer.fsm.peerInfo.AS,
		LocalAS:       peer.fsm.peerInfo.LocalAS,
		PeerAddress:   peer.fsm.peerInfo.Address,
		LocalAddress:  net.ParseIP(laddr),
		PeerPort:      rport,
		LocalPort:     lport,
		PeerID:        peer.fsm.peerInfo.ID,
		SentOpen:      sentOpen,
		RecvOpen:      recvOpen,
		State:         peer.fsm.state,
		AdminState:    peer.fsm.adminState,
		Timestamp:     time.Now(),
		PeerInterface: peer.fsm.pConf.Config.NeighborInterface,
	}
}

func (server *BgpServer) broadcastPeerState(peer *Peer, oldState bgp.FSMState) {
	newState := peer.fsm.state
	if oldState == bgp.BGP_FSM_ESTABLISHED || newState == bgp.BGP_FSM_ESTABLISHED {
		server.notifyWatcher(WATCH_EVENT_TYPE_PEER_STATE, createWatchEventPeerState(peer))
	}
}

func (server *BgpServer) notifyMessageWatcher(peer *Peer, timestamp time.Time, msg *bgp.BGPMessage, isSent bool) {
	// validation should be done in the caller of this function
	_, y := peer.fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]
	l, _ := peer.fsm.LocalHostPort()
	ev := &WatchEventMessage{
		Message:      msg,
		PeerAS:       peer.fsm.peerInfo.AS,
		LocalAS:      peer.fsm.peerInfo.LocalAS,
		PeerAddress:  peer.fsm.peerInfo.Address,
		LocalAddress: net.ParseIP(l),
		PeerID:       peer.fsm.peerInfo.ID,
		FourBytesAs:  y,
		Timestamp:    timestamp,
		IsSent:       isSent,
	}
	if !isSent {
		server.notifyWatcher(WATCH_EVENT_TYPE_RECV_MSG, ev)
	}
}

func (server *BgpServer) notifyRecvMessageWatcher(peer *Peer, timestamp time.Time, msg *bgp.BGPMessage) {
	if peer == nil || !server.isWatched(WATCH_EVENT_TYPE_RECV_MSG) {
		return
	}
	server.notifyMessageWatcher(peer, timestamp, msg, false)
}

func (server *BgpServer) RSimportPaths(peer *Peer, pathList []*table.Path) []*table.Path {
	moded := make([]*table.Path, 0, len(pathList)/2)
	for _, before := range pathList {
		if isASLoop(peer, before) {
			before.Filter(peer.ID(), table.POLICY_DIRECTION_IMPORT)
			continue
		}
		after := server.policy.ApplyPolicy(peer.TableID(), table.POLICY_DIRECTION_IMPORT, before, nil)
		if after == nil {
			before.Filter(peer.ID(), table.POLICY_DIRECTION_IMPORT)
		} else if after != before {
			before.Filter(peer.ID(), table.POLICY_DIRECTION_IMPORT)
			for _, n := range server.neighborMap {
				if n == peer {
					continue
				}
				after.Filter(n.ID(), table.POLICY_DIRECTION_IMPORT)
			}
			moded = append(moded, after)
		}
	}
	return moded
}

func (server *BgpServer) propagateUpdate(peer *Peer, pathList []*table.Path) {
	var dsts []*table.Destination

	var gBestList, gOldList []*table.Path
	var mpathList [][]*table.Path

	rib := server.globalRib

	if peer != nil && peer.fsm.pConf.Config.Vrf != "" {
		vrf := server.globalRib.Vrfs[peer.fsm.pConf.Config.Vrf]
		for idx, path := range pathList {
			pathList[idx] = path.ToGlobal(vrf)
		}
	}

	if peer != nil && peer.isRouteServerClient() {
		rib = server.rsRib
		for _, path := range pathList {
			path.Filter(peer.ID(), table.POLICY_DIRECTION_IMPORT)
		}
		moded := make([]*table.Path, 0)
		for _, targetPeer := range server.neighborMap {
			if !targetPeer.isRouteServerClient() || peer == targetPeer {
				continue
			}
			moded = append(moded, server.RSimportPaths(targetPeer, pathList)...)
		}
		dsts = rib.ProcessPaths(append(pathList, moded...))
	} else {
		for idx, path := range pathList {
			var options *table.PolicyOptions
			if peer != nil {
				options = &table.PolicyOptions{
					Info: peer.fsm.peerInfo,
				}
			} else {
				options = nil
			}
			if p := server.policy.ApplyPolicy(table.GLOBAL_RIB_NAME, table.POLICY_DIRECTION_IMPORT, path, options); p != nil {
				path = p
			} else {
				path = path.Clone(true)
			}
			pathList[idx] = path
			// RFC4684 Constrained Route Distribution 6. Operation
			//
			// When a BGP speaker receives a BGP UPDATE that advertises or withdraws
			// a given Route Target membership NLRI, it should examine the RIB-OUTs
			// of VPN NLRIs and re-evaluate the advertisement status of routes that
			// match the Route Target in question.
			//
			// A BGP speaker should generate the minimum set of BGP VPN route
			// updates (advertisements and/or withdraws) necessary to transition
			// between the previous and current state of the route distribution
			// graph that is derived from Route Target membership information.
			if peer != nil && path != nil && path.GetRouteFamily() == bgp.RF_RTC_UC {
				rt := path.GetNlri().(*bgp.RouteTargetMembershipNLRI).RouteTarget
				fs := make([]bgp.RouteFamily, 0, len(peer.configuredRFlist()))
				for _, f := range peer.configuredRFlist() {
					if f != bgp.RF_RTC_UC {
						fs = append(fs, f)
					}
				}
				var candidates []*table.Path
				if path.IsWithdraw {
					candidates, _ = peer.getBestFromLocal(peer.configuredRFlist())
				} else {
					candidates = rib.GetBestPathList(peer.TableID(), fs)
				}
				paths := make([]*table.Path, 0, len(candidates))
				for _, p := range candidates {
					for _, ext := range p.GetExtCommunities() {
						if rt == nil || ext.String() == rt.String() {
							if path.IsWithdraw {
								p = p.Clone(true)
							}
							paths = append(paths, p)
							break
						}
					}
				}
				if path.IsWithdraw {
					paths = peer.processOutgoingPaths(nil, paths)
				} else {
					paths = peer.processOutgoingPaths(paths, nil)
				}
				sendFsmOutgoingMsg(peer, paths, nil, false)
			}
		}
		server.notifyPostPolicyUpdateWatcher(peer, pathList)
		dsts = rib.ProcessPaths(pathList)

		gBestList, gOldList, mpathList = dstsToPaths(table.GLOBAL_RIB_NAME, dsts, false)
		server.notifyBestWatcher(gBestList, mpathList)
	}

	server.propagateUpdateToNeighbors(peer, dsts, gBestList, gOldList)
}

func (server *BgpServer) propagateUpdateToNeighbors(peer *Peer, dsts []*table.Destination, gBestList, gOldList []*table.Path) {
	families := make(map[bgp.RouteFamily][]*table.Destination)
	for _, dst := range dsts {
		family := dst.Family()
		if families[family] == nil {
			families[family] = make([]*table.Destination, 0, len(dsts))
		}
		families[family] = append(families[family], dst)
	}

	var bestList, oldList []*table.Path
	for family, l := range families {
		for _, targetPeer := range server.neighborMap {
			if (peer == nil && targetPeer.isRouteServerClient()) || (peer != nil && peer.isRouteServerClient() != targetPeer.isRouteServerClient()) {
				continue
			}
			if targetPeer.isAddPathSendEnabled(family) {
				bestList, oldList, _ = dstsToPaths(targetPeer.TableID(), l, true)
			} else if targetPeer.isRouteServerClient() {
				bestList, oldList, _ = dstsToPaths(targetPeer.TableID(), l, false)
			} else {
				bestList = gBestList
				oldList = gOldList
			}
			if paths := targetPeer.processOutgoingPaths(bestList, oldList); len(paths) > 0 {
				sendFsmOutgoingMsg(targetPeer, paths, nil, false)
			}
		}
	}
}

func (server *BgpServer) handleFSMMessage(peer *Peer, e *FsmMsg) {
	switch e.MsgType {
	case FSM_MSG_STATE_CHANGE:
		nextState := e.MsgData.(bgp.FSMState)
		oldState := bgp.FSMState(peer.fsm.pConf.State.SessionState.ToInt())
		peer.fsm.pConf.State.SessionState = config.IntToSessionStateMap[int(nextState)]
		peer.fsm.StateChange(nextState)

		if oldState == bgp.BGP_FSM_ESTABLISHED {
			t := time.Now()
			if t.Sub(time.Unix(peer.fsm.pConf.Timers.State.Uptime, 0)) < FLOP_THRESHOLD {
				peer.fsm.pConf.State.Flops++
			}
			var drop []bgp.RouteFamily
			if peer.fsm.reason == FSM_GRACEFUL_RESTART {
				peer.fsm.pConf.GracefulRestart.State.PeerRestarting = true
				var p []bgp.RouteFamily
				p, drop = peer.forwardingPreservedFamilies()
				peer.StaleAll(p)
			} else {
				drop = peer.configuredRFlist()
			}
			peer.prefixLimitWarned = make(map[bgp.RouteFamily]bool)
			peer.DropAll(drop)
			server.dropPeerAllRoutes(peer, drop)
			if peer.fsm.pConf.Config.PeerAs == 0 {
				peer.fsm.pConf.State.PeerAs = 0
				peer.fsm.peerInfo.AS = 0
			}
			if peer.isDynamicNeighbor() {
				peer.stopPeerRestarting()
				go peer.stopFSM()
				delete(server.neighborMap, peer.fsm.pConf.State.NeighborAddress)
			}
		} else if peer.fsm.pConf.GracefulRestart.State.PeerRestarting && nextState == bgp.BGP_FSM_IDLE {
			if peer.fsm.pConf.GracefulRestart.State.LongLivedEnabled {
				llgr, no_llgr := peer.llgrFamilies()

				peer.DropAll(no_llgr)
				server.dropPeerAllRoutes(peer, no_llgr)

				// attach LLGR_STALE community to paths in peer's adj-rib-in
				// paths with NO_LLGR are deleted
				pathList := peer.markLLGRStale(llgr)

				// calculate again
				// wheh path with LLGR_STALE chosen as best,
				// peer which doesn't support LLGR will drop the path
				// if it is in adj-rib-out, do withdrawal
				server.propagateUpdate(peer, pathList)

				for _, f := range llgr {
					endCh := make(chan struct{})
					peer.llgrEndChs = append(peer.llgrEndChs, endCh)
					go func(family bgp.RouteFamily, endCh chan struct{}) {
						t := peer.llgrRestartTime(family)
						timer := time.NewTimer(time.Second * time.Duration(t))

						log.WithFields(log.Fields{
							"Topic":  "Peer",
							"Key":    peer.ID(),
							"Family": family,
						}).Debugf("start LLGR restart timer (%d sec) for %s", t, family)

						select {
						case <-timer.C:
							server.mgmtOperation(func() error {
								log.WithFields(log.Fields{
									"Topic":  "Peer",
									"Key":    peer.ID(),
									"Family": family,
								}).Debugf("LLGR restart timer (%d sec) for %s expired", t, family)
								peer.DropAll([]bgp.RouteFamily{family})
								server.dropPeerAllRoutes(peer, []bgp.RouteFamily{family})

								// when all llgr restart timer expired, stop PeerRestarting
								if peer.llgrRestartTimerExpired(family) {
									peer.stopPeerRestarting()
								}
								return nil
							}, false)
						case <-endCh:
							log.WithFields(log.Fields{
								"Topic":  "Peer",
								"Key":    peer.ID(),
								"Family": family,
							}).Debugf("stop LLGR restart timer (%d sec) for %s", t, family)
						}
					}(f, endCh)
				}
			} else {
				// RFC 4724 4.2
				// If the session does not get re-established within the "Restart Time"
				// that the peer advertised previously, the Receiving Speaker MUST
				// delete all the stale routes from the peer that it is retaining.
				peer.fsm.pConf.GracefulRestart.State.PeerRestarting = false
				peer.DropAll(peer.configuredRFlist())
				server.dropPeerAllRoutes(peer, peer.configuredRFlist())
			}
		}

		cleanInfiniteChannel(peer.outgoing)
		peer.outgoing = channels.NewInfiniteChannel()
		if nextState == bgp.BGP_FSM_ESTABLISHED {
			// update for export policy
			laddr, _ := peer.fsm.LocalHostPort()
			// may include zone info
			peer.fsm.pConf.Transport.State.LocalAddress = laddr
			// exclude zone info
			ipaddr, _ := net.ResolveIPAddr("ip", laddr)
			peer.fsm.peerInfo.LocalAddress = ipaddr.IP
			deferralExpiredFunc := func(family bgp.RouteFamily) func() {
				return func() {
					server.mgmtOperation(func() error {
						server.softResetOut(peer.fsm.pConf.State.NeighborAddress, family, true)
						return nil
					}, false)
				}
			}
			if !peer.fsm.pConf.GracefulRestart.State.LocalRestarting {
				// When graceful-restart cap (which means intention
				// of sending EOR) and route-target address family are negotiated,
				// send route-target NLRIs first, and wait to send others
				// till receiving EOR of route-target address family.
				// This prevents sending uninterested routes to peers.
				//
				// However, when the peer is graceful restarting, give up
				// waiting sending non-route-target NLRIs since the peer won't send
				// any routes (and EORs) before we send ours (or deferral-timer expires).
				var pathList []*table.Path
				_, y := peer.fsm.rfMap[bgp.RF_RTC_UC]
				if c := peer.fsm.pConf.GetAfiSafi(bgp.RF_RTC_UC); y && !peer.fsm.pConf.GracefulRestart.State.PeerRestarting && c.RouteTargetMembership.Config.DeferralTime > 0 {
					pathList, _ = peer.getBestFromLocal([]bgp.RouteFamily{bgp.RF_RTC_UC})
					t := c.RouteTargetMembership.Config.DeferralTime
					for _, f := range peer.configuredRFlist() {
						if f != bgp.RF_RTC_UC {
							time.AfterFunc(time.Second*time.Duration(t), deferralExpiredFunc(f))
						}
					}
				} else {
					pathList, _ = peer.getBestFromLocal(peer.configuredRFlist())
				}

				if len(pathList) > 0 {
					sendFsmOutgoingMsg(peer, pathList, nil, false)
				}
			} else {
				// RFC 4724 4.1
				// Once the session between the Restarting Speaker and the Receiving
				// Speaker is re-established, the Restarting Speaker will receive and
				// process BGP messages from its peers.  However, it MUST defer route
				// selection for an address family until it either (a) ...snip...
				// or (b) the Selection_Deferral_Timer referred to below has expired.
				deferral := peer.fsm.pConf.GracefulRestart.Config.DeferralTime
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   peer.ID(),
				}).Debugf("Now syncing, suppress sending updates. start deferral timer(%d)", deferral)
				time.AfterFunc(time.Second*time.Duration(deferral), deferralExpiredFunc(bgp.RouteFamily(0)))
			}
		} else {
			if server.shutdown && nextState == bgp.BGP_FSM_IDLE {
				die := true
				for _, p := range server.neighborMap {
					if p.fsm.state != bgp.BGP_FSM_IDLE {
						die = false
						break
					}
				}
				if die {
					os.Exit(0)
				}
			}
			peer.fsm.pConf.Timers.State.Downtime = time.Now().Unix()
		}
		// clear counter
		if peer.fsm.adminState == ADMIN_STATE_DOWN {
			peer.fsm.pConf.State = config.NeighborState{}
			peer.fsm.pConf.State.NeighborAddress = peer.fsm.pConf.Config.NeighborAddress
			peer.fsm.pConf.State.PeerAs = peer.fsm.pConf.Config.PeerAs
			peer.fsm.pConf.Timers.State = config.TimersState{}
		}
		peer.startFSMHandler(server.fsmincomingCh, server.fsmStateCh)
		server.broadcastPeerState(peer, oldState)
	case FSM_MSG_ROUTE_REFRESH:
		if peer.fsm.state != bgp.BGP_FSM_ESTABLISHED || e.timestamp.Unix() < peer.fsm.pConf.Timers.State.Uptime {
			return
		}
		if paths := peer.handleRouteRefresh(e); len(paths) > 0 {
			sendFsmOutgoingMsg(peer, paths, nil, false)
			return
		}
	case FSM_MSG_BGP_MESSAGE:
		switch m := e.MsgData.(type) {
		case *bgp.MessageError:
			sendFsmOutgoingMsg(peer, nil, bgp.NewBGPNotificationMessage(m.TypeCode, m.SubTypeCode, m.Data), false)
			return
		case *bgp.BGPMessage:
			server.notifyRecvMessageWatcher(peer, e.timestamp, m)
			if peer.fsm.state != bgp.BGP_FSM_ESTABLISHED || e.timestamp.Unix() < peer.fsm.pConf.Timers.State.Uptime {
				return
			}
			server.roaManager.validate(e.PathList)
			pathList, eor, notification := peer.handleUpdate(e)
			if notification != nil {
				sendFsmOutgoingMsg(peer, nil, notification, true)
				return
			}
			if m.Header.Type == bgp.BGP_MSG_UPDATE {
				server.notifyPrePolicyUpdateWatcher(peer, pathList, m, e.timestamp, e.payload)
			}

			if len(pathList) > 0 {
				server.propagateUpdate(peer, pathList)
			}

			if len(eor) > 0 {
				rtc := false
				for _, f := range eor {
					if f == bgp.RF_RTC_UC {
						rtc = true
					}
					for i, a := range peer.fsm.pConf.AfiSafis {
						if a.State.Family == f {
							peer.fsm.pConf.AfiSafis[i].MpGracefulRestart.State.EndOfRibReceived = true
						}
					}
				}

				// RFC 4724 4.1
				// Once the session between the Restarting Speaker and the Receiving
				// Speaker is re-established, ...snip... it MUST defer route
				// selection for an address family until it either (a) receives the
				// End-of-RIB marker from all its peers (excluding the ones with the
				// "Restart State" bit set in the received capability and excluding the
				// ones that do not advertise the graceful restart capability) or ...snip...
				if peer.fsm.pConf.GracefulRestart.State.LocalRestarting {
					allEnd := func() bool {
						for _, p := range server.neighborMap {
							if !p.recvedAllEOR() {
								return false
							}
						}
						return true
					}()
					if allEnd {
						for _, p := range server.neighborMap {
							p.fsm.pConf.GracefulRestart.State.LocalRestarting = false
							if !p.isGracefulRestartEnabled() {
								continue
							}
							paths, _ := p.getBestFromLocal(p.configuredRFlist())
							if len(paths) > 0 {
								sendFsmOutgoingMsg(p, paths, nil, false)
							}
						}
						log.WithFields(log.Fields{
							"Topic": "Server",
						}).Info("sync finished")

					}

					// we don't delay non-route-target NLRIs when local-restarting
					rtc = false
				}
				if peer.fsm.pConf.GracefulRestart.State.PeerRestarting {
					if peer.recvedAllEOR() {
						peer.stopPeerRestarting()
						pathList := peer.adjRibIn.DropStale(peer.configuredRFlist())
						log.WithFields(log.Fields{
							"Topic": "Peer",
							"Key":   peer.fsm.pConf.State.NeighborAddress,
						}).Debugf("withdraw %d stale routes", len(pathList))
						server.propagateUpdate(peer, pathList)
					}

					// we don't delay non-route-target NLRIs when peer is restarting
					rtc = false
				}

				// received EOR of route-target address family
				// outbound filter is now ready, let's flash non-route-target NLRIs
				if c := peer.fsm.pConf.GetAfiSafi(bgp.RF_RTC_UC); rtc && c != nil && c.RouteTargetMembership.Config.DeferralTime > 0 {
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   peer.ID(),
					}).Debug("received route-target eor. flash non-route-target NLRIs")
					families := make([]bgp.RouteFamily, 0, len(peer.configuredRFlist()))
					for _, f := range peer.configuredRFlist() {
						if f != bgp.RF_RTC_UC {
							families = append(families, f)
						}
					}
					if paths, _ := peer.getBestFromLocal(families); len(paths) > 0 {
						sendFsmOutgoingMsg(peer, paths, nil, false)
					}
				}
			}
		default:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   peer.fsm.pConf.State.NeighborAddress,
				"Data":  e.MsgData,
			}).Panic("unknown msg type")
		}
	}
	return
}

func (s *BgpServer) StartCollector(c *config.CollectorConfig) error {
	return s.mgmtOperation(func() error {
		_, err := NewCollector(s, c.Url, c.DbName, c.TableDumpInterval)
		return err
	}, false)
}

func (s *BgpServer) StartZebraClient(c *config.ZebraConfig) error {
	return s.mgmtOperation(func() error {
		if s.zclient != nil {
			return fmt.Errorf("already connected to Zebra")
		}
		protos := make([]string, 0, len(c.RedistributeRouteTypeList))
		for _, p := range c.RedistributeRouteTypeList {
			protos = append(protos, string(p))
		}
		var err error
		s.zclient, err = newZebraClient(s, c.Url, protos, c.Version, c.NexthopTriggerEnable, c.NexthopTriggerDelay)
		return err
	}, false)
}

func (s *BgpServer) AddBmp(c *config.BmpServerConfig) error {
	return s.mgmtOperation(func() error {
		return s.bmpManager.addServer(c)
	}, true)
}

func (s *BgpServer) DeleteBmp(c *config.BmpServerConfig) error {
	return s.mgmtOperation(func() error {
		return s.bmpManager.deleteServer(c)
	}, true)
}

func (s *BgpServer) Shutdown() {
	s.mgmtOperation(func() error {
		s.shutdown = true
		stateOp := AdminStateOperation{ADMIN_STATE_DOWN, nil}
		for _, p := range s.neighborMap {
			p.fsm.adminStateCh <- stateOp
		}
		// the main goroutine waits for peers' goroutines to stop but if no peer is configured, needs to die immediately.
		if len(s.neighborMap) == 0 {
			os.Exit(0)
		}
		// TODO: call fsmincomingCh.Close()
		return nil
	}, false)
}

func (s *BgpServer) UpdatePolicy(policy config.RoutingPolicy) error {
	return s.mgmtOperation(func() error {
		ap := make(map[string]config.ApplyPolicy, len(s.neighborMap)+1)
		ap[table.GLOBAL_RIB_NAME] = s.bgpConfig.Global.ApplyPolicy
		for _, peer := range s.neighborMap {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   peer.fsm.pConf.State.NeighborAddress,
			}).Info("call set policy")
			ap[peer.ID()] = peer.fsm.pConf.ApplyPolicy
		}
		return s.policy.Reset(&policy, ap)
	}, false)
}

// EVPN MAC MOBILITY HANDLING
//
// We don't have multihoming function now, so ignore
// ESI comparison.
//
// RFC7432 15. MAC Mobility
//
// A PE detecting a locally attached MAC address for which it had
// previously received a MAC/IP Advertisement route with the same zero
// Ethernet segment identifier (single-homed scenarios) advertises it
// with a MAC Mobility extended community attribute with the sequence
// number set properly.  In the case of single-homed scenarios, there
// is no need for ESI comparison.

func getMacMobilityExtendedCommunity(etag uint32, mac net.HardwareAddr, evpnPaths []*table.Path) *bgp.MacMobilityExtended {
	seqs := make([]struct {
		seq     int
		isLocal bool
	}, 0)

	for _, path := range evpnPaths {
		nlri := path.GetNlri().(*bgp.EVPNNLRI)
		target, ok := nlri.RouteTypeData.(*bgp.EVPNMacIPAdvertisementRoute)
		if !ok {
			continue
		}
		if target.ETag == etag && bytes.Equal(target.MacAddress, mac) {
			found := false
			for _, ec := range path.GetExtCommunities() {
				if t, st := ec.GetTypes(); t == bgp.EC_TYPE_EVPN && st == bgp.EC_SUBTYPE_MAC_MOBILITY {
					seqs = append(seqs, struct {
						seq     int
						isLocal bool
					}{int(ec.(*bgp.MacMobilityExtended).Sequence), path.IsLocal()})
					found = true
					break
				}
			}

			if !found {
				seqs = append(seqs, struct {
					seq     int
					isLocal bool
				}{-1, path.IsLocal()})
			}
		}
	}

	if len(seqs) > 0 {
		newSeq := -2
		var isLocal bool
		for _, seq := range seqs {
			if seq.seq > newSeq {
				newSeq = seq.seq
				isLocal = seq.isLocal
			}
		}

		if !isLocal {
			newSeq += 1
		}

		if newSeq != -1 {
			return &bgp.MacMobilityExtended{
				Sequence: uint32(newSeq),
			}
		}
	}
	return nil
}

func (server *BgpServer) fixupApiPath(vrfId string, pathList []*table.Path) error {
	pi := &table.PeerInfo{
		AS:      server.bgpConfig.Global.Config.As,
		LocalID: net.ParseIP(server.bgpConfig.Global.Config.RouterId).To4(),
	}

	for _, path := range pathList {
		if path.GetSource() == nil {
			path.SetSource(pi)
		}

		if vrfId != "" {
			vrf := server.globalRib.Vrfs[vrfId]
			if vrf == nil {
				return fmt.Errorf("vrf %s not found", vrfId)
			}
			if err := vrf.ToGlobalPath(path); err != nil {
				return err
			}
		}

		// Address Family specific Handling
		switch nlri := path.GetNlri().(type) {
		case *bgp.EVPNNLRI:
			switch r := nlri.RouteTypeData.(type) {
			case *bgp.EVPNMacIPAdvertisementRoute:
				// MAC Mobility Extended Community
				paths := server.globalRib.GetBestPathList(table.GLOBAL_RIB_NAME, []bgp.RouteFamily{bgp.RF_EVPN})
				if m := getMacMobilityExtendedCommunity(r.ETag, r.MacAddress, paths); m != nil {
					path.SetExtCommunities([]bgp.ExtendedCommunityInterface{m}, false)
				}
			case *bgp.EVPNEthernetSegmentRoute:
				// RFC7432: BGP MPLS-Based Ethernet VPN
				// 7.6. ES-Import Route Target
				// The value is derived automatically for the ESI Types 1, 2,
				// and 3, by encoding the high-order 6-octet portion of the 9-octet ESI
				// Value, which corresponds to a MAC address, in the ES-Import Route
				// Target.
				// Note: If the given path already has the ES-Import Route Target,
				// skips deriving a new one.
				found := false
				for _, extComm := range path.GetExtCommunities() {
					if _, found = extComm.(*bgp.ESImportRouteTarget); found {
						break
					}
				}
				if !found {
					switch r.ESI.Type {
					case bgp.ESI_LACP, bgp.ESI_MSTP, bgp.ESI_MAC:
						mac := net.HardwareAddr(r.ESI.Value[0:6])
						rt := &bgp.ESImportRouteTarget{ESImport: mac}
						path.SetExtCommunities([]bgp.ExtendedCommunityInterface{rt}, false)
					}
				}
			}
		}
	}
	return nil
}

func (s *BgpServer) AddPath(vrfId string, pathList []*table.Path) (uuidBytes []byte, err error) {
	err = s.mgmtOperation(func() error {
		if err := s.fixupApiPath(vrfId, pathList); err != nil {
			return err
		}
		if len(pathList) == 1 {
			pathList[0].AssignNewUUID()
			uuidBytes = pathList[0].UUID().Bytes()
		}
		s.propagateUpdate(nil, pathList)
		return nil
	}, true)
	return
}

func (s *BgpServer) DeletePath(uuid []byte, f bgp.RouteFamily, vrfId string, pathList []*table.Path) error {
	return s.mgmtOperation(func() error {
		deletePathList := make([]*table.Path, 0)
		if len(uuid) > 0 {
			path := func() *table.Path {
				for _, path := range s.globalRib.GetPathList(table.GLOBAL_RIB_NAME, s.globalRib.GetRFlist()) {
					if len(path.UUID()) > 0 && bytes.Equal(path.UUID().Bytes(), uuid) {
						return path
					}
				}
				return nil
			}()
			if path != nil {
				deletePathList = append(deletePathList, path.Clone(true))
			} else {
				return fmt.Errorf("Can't find a specified path")
			}
		} else if len(pathList) == 0 {
			// delete all paths
			families := s.globalRib.GetRFlist()
			if f != 0 {
				families = []bgp.RouteFamily{f}
			}
			for _, path := range s.globalRib.GetPathList(table.GLOBAL_RIB_NAME, families) {
				deletePathList = append(deletePathList, path.Clone(true))
			}
		} else {
			if err := s.fixupApiPath(vrfId, pathList); err != nil {
				return err
			}
			deletePathList = pathList
		}
		s.propagateUpdate(nil, deletePathList)
		return nil
	}, true)
}

func (s *BgpServer) UpdatePath(vrfId string, pathList []*table.Path) error {
	err := s.mgmtOperation(func() error {
		if err := s.fixupApiPath(vrfId, pathList); err != nil {
			return err
		}
		dsts := s.globalRib.ProcessPaths(pathList)
		gBestList, gOldList, gMPathList := dstsToPaths(table.GLOBAL_RIB_NAME, dsts, false)
		s.notifyBestWatcher(gBestList, gMPathList)
		s.propagateUpdateToNeighbors(nil, dsts, gBestList, gOldList)
		return nil
	}, true)
	return err
}

func (s *BgpServer) Start(c *config.Global) error {
	return s.mgmtOperation(func() error {
		if err := config.SetDefaultGlobalConfigValues(c); err != nil {
			return err
		}

		if c.Config.Port > 0 {
			acceptCh := make(chan *net.TCPConn, 4096)
			for _, addr := range c.Config.LocalAddressList {
				l, err := NewTCPListener(addr, uint32(c.Config.Port), acceptCh)
				if err != nil {
					return err
				}
				s.listeners = append(s.listeners, l)
			}
			s.acceptCh = acceptCh
		}

		rfs, _ := config.AfiSafis(c.AfiSafis).ToRfList()
		s.globalRib = table.NewTableManager(rfs)
		s.rsRib = table.NewTableManager(rfs)

		if err := s.policy.Reset(&config.RoutingPolicy{}, map[string]config.ApplyPolicy{}); err != nil {
			return err
		}
		s.bgpConfig.Global = *c
		// update route selection options
		table.SelectionOptions = c.RouteSelectionOptions.Config
		table.UseMultiplePaths = c.UseMultiplePaths.Config

		s.roaManager.SetAS(s.bgpConfig.Global.Config.As)
		return nil
	}, false)
}

func (s *BgpServer) GetVrf() (l []*table.Vrf) {
	s.mgmtOperation(func() error {
		l = make([]*table.Vrf, 0, len(s.globalRib.Vrfs))
		for _, vrf := range s.globalRib.Vrfs {
			l = append(l, vrf.Clone())
		}
		return nil
	}, true)
	return l
}

func (s *BgpServer) AddVrf(name string, id uint32, rd bgp.RouteDistinguisherInterface, im, ex []bgp.ExtendedCommunityInterface) error {
	return s.mgmtOperation(func() error {
		pi := &table.PeerInfo{
			AS:      s.bgpConfig.Global.Config.As,
			LocalID: net.ParseIP(s.bgpConfig.Global.Config.RouterId).To4(),
		}
		if pathList, e := s.globalRib.AddVrf(name, id, rd, im, ex, pi); e != nil {
			return e
		} else if len(pathList) > 0 {
			s.propagateUpdate(nil, pathList)
		}
		return nil
	}, true)
}

func (s *BgpServer) DeleteVrf(name string) error {
	return s.mgmtOperation(func() error {
		for _, n := range s.neighborMap {
			if n.fsm.pConf.Config.Vrf == name {
				return fmt.Errorf("failed to delete VRF %s: neighbor %s is in use", name, n.ID())
			}
		}
		pathList, err := s.globalRib.DeleteVrf(name)
		if err != nil {
			return err
		}
		if len(pathList) > 0 {
			s.propagateUpdate(nil, pathList)
		}
		return nil
	}, true)
}

func (s *BgpServer) Stop() error {
	return s.mgmtOperation(func() error {
		for k, _ := range s.neighborMap {
			if err := s.deleteNeighbor(&config.Neighbor{Config: config.NeighborConfig{
				NeighborAddress: k}}, bgp.BGP_ERROR_CEASE, bgp.BGP_ERROR_SUB_PEER_DECONFIGURED); err != nil {
				return err
			}
		}
		for _, l := range s.listeners {
			l.Close()
		}
		s.bgpConfig.Global = config.Global{}
		return nil
	}, true)
}

func (s *BgpServer) softResetIn(addr string, family bgp.RouteFamily) error {
	peers, err := s.addrToPeers(addr)
	if err != nil {
		return err
	}
	for _, peer := range peers {
		pathList := []*table.Path{}
		families := []bgp.RouteFamily{family}
		if family == bgp.RouteFamily(0) {
			families = peer.configuredRFlist()
		}
		for _, path := range peer.adjRibIn.PathList(families, false) {
			exResult := path.Filtered(peer.ID())
			path.Filter(peer.ID(), table.POLICY_DIRECTION_NONE)

			// RFC4271 9.1.2 Phase 2: Route Selection
			//
			// If the AS_PATH attribute of a BGP route contains an AS loop, the BGP
			// route should be excluded from the Phase 2 decision function.
			var asLoop bool
			if aspath := path.GetAsPath(); aspath != nil {
				asLoop = hasOwnASLoop(peer.fsm.peerInfo.LocalAS, int(peer.fsm.pConf.AsPathOptions.Config.AllowOwnAs), aspath)
			}

			if !asLoop && s.policy.ApplyPolicy(peer.ID(), table.POLICY_DIRECTION_IN, path, nil) != nil {
				pathList = append(pathList, path.Clone(false))
				// this path still in rib's
				// knownPathList. We can't
				// drop
				// table.POLICY_DIRECTION_IMPORT
				// flag here. Otherwise, this
				// path could be the old best
				// path.
				if peer.isRouteServerClient() {
					path.Filter(peer.ID(), table.POLICY_DIRECTION_IMPORT)
				}
			} else {
				path.Filter(peer.ID(), table.POLICY_DIRECTION_IN)
				if exResult != table.POLICY_DIRECTION_IN {
					pathList = append(pathList, path.Clone(true))
				}
			}
		}
		peer.adjRibIn.RefreshAcceptedNumber(families)
		s.propagateUpdate(peer, pathList)
	}
	return err
}

func (s *BgpServer) softResetOut(addr string, family bgp.RouteFamily, deferral bool) error {
	peers, err := s.addrToPeers(addr)
	if err != nil {
		return err
	}
	for _, peer := range peers {
		if peer.fsm.state != bgp.BGP_FSM_ESTABLISHED {
			continue
		}

		families := []bgp.RouteFamily{family}
		if family == bgp.RouteFamily(0) {
			families = peer.configuredRFlist()
		}

		if deferral {
			_, y := peer.fsm.rfMap[bgp.RF_RTC_UC]
			if peer.fsm.pConf.GracefulRestart.State.LocalRestarting {
				peer.fsm.pConf.GracefulRestart.State.LocalRestarting = false
				log.WithFields(log.Fields{
					"Topic":    "Peer",
					"Key":      peer.ID(),
					"Families": families,
				}).Debug("deferral timer expired")
			} else if c := peer.fsm.pConf.GetAfiSafi(bgp.RF_RTC_UC); y && !c.MpGracefulRestart.State.EndOfRibReceived {
				log.WithFields(log.Fields{
					"Topic":    "Peer",
					"Key":      peer.ID(),
					"Families": families,
				}).Debug("route-target deferral timer expired")
			} else {
				continue
			}
		}

		pathList, filtered := peer.getBestFromLocal(families)
		if len(pathList) > 0 {
			sendFsmOutgoingMsg(peer, pathList, nil, false)
		}
		if deferral == false && len(filtered) > 0 {
			withdrawnList := make([]*table.Path, 0, len(filtered))
			for _, p := range filtered {
				withdrawnList = append(withdrawnList, p.Clone(true))
			}
			sendFsmOutgoingMsg(peer, withdrawnList, nil, false)
		}
	}
	return nil
}

func (s *BgpServer) SoftResetIn(addr string, family bgp.RouteFamily) error {
	return s.mgmtOperation(func() error {
		log.WithFields(log.Fields{
			"Topic": "Operation",
			"Key":   addr,
		}).Info("Neighbor soft reset in")
		return s.softResetIn(addr, family)
	}, true)
}

func (s *BgpServer) SoftResetOut(addr string, family bgp.RouteFamily) error {
	return s.mgmtOperation(func() error {
		log.WithFields(log.Fields{
			"Topic": "Operation",
			"Key":   addr,
		}).Info("Neighbor soft reset out")
		return s.softResetOut(addr, family, false)
	}, true)
}

func (s *BgpServer) SoftReset(addr string, family bgp.RouteFamily) error {
	return s.mgmtOperation(func() error {
		log.WithFields(log.Fields{
			"Topic": "Operation",
			"Key":   addr,
		}).Info("Neighbor soft reset")
		err := s.softResetIn(addr, family)
		if err != nil {
			return err
		}
		return s.softResetOut(addr, family, false)
	}, true)
}

func (s *BgpServer) GetRib(addr string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (rib *table.Table, err error) {
	err = s.mgmtOperation(func() error {
		m := s.globalRib
		id := table.GLOBAL_RIB_NAME
		if len(addr) > 0 {
			peer, ok := s.neighborMap[addr]
			if !ok {
				return fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
			}
			if !peer.isRouteServerClient() {
				return fmt.Errorf("Neighbor %v doesn't have local rib", addr)
			}
			id = peer.ID()
			m = s.rsRib
		}
		af := bgp.RouteFamily(family)
		tbl, ok := m.Tables[af]
		if !ok {
			return fmt.Errorf("address family: %s not supported", af)
		}
		rib, err = tbl.Select(table.TableSelectOption{ID: id, LookupPrefixes: prefixes})
		return err
	}, true)
	return
}

func (s *BgpServer) GetVrfRib(name string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (rib *table.Table, err error) {
	err = s.mgmtOperation(func() error {
		m := s.globalRib
		vrfs := m.Vrfs
		if _, ok := vrfs[name]; !ok {
			return fmt.Errorf("vrf %s not found", name)
		}
		var af bgp.RouteFamily
		switch family {
		case bgp.RF_IPv4_UC:
			af = bgp.RF_IPv4_VPN
		case bgp.RF_IPv6_UC:
			af = bgp.RF_IPv6_VPN
		case bgp.RF_EVPN:
			af = bgp.RF_EVPN
		}
		tbl, ok := m.Tables[af]
		if !ok {
			return fmt.Errorf("address family: %s not supported", af)
		}
		rib, err = tbl.Select(table.TableSelectOption{VRF: vrfs[name], LookupPrefixes: prefixes})
		return err
	}, true)
	return
}

func (s *BgpServer) GetAdjRib(addr string, family bgp.RouteFamily, in bool, prefixes []*table.LookupPrefix) (rib *table.Table, err error) {
	err = s.mgmtOperation(func() error {
		peer, ok := s.neighborMap[addr]
		if !ok {
			return fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
		}
		id := peer.ID()

		var adjRib *table.AdjRib
		if in {
			adjRib = peer.adjRibIn
		} else {
			adjRib = table.NewAdjRib(id, peer.configuredRFlist())
			accepted, _ := peer.getBestFromLocal(peer.configuredRFlist())
			adjRib.Update(accepted)
		}
		rib, err = adjRib.Select(family, false, table.TableSelectOption{ID: id, LookupPrefixes: prefixes})
		return err
	}, true)
	return
}

func (s *BgpServer) GetRibInfo(addr string, family bgp.RouteFamily) (info *table.TableInfo, err error) {
	err = s.mgmtOperation(func() error {
		m := s.globalRib
		id := table.GLOBAL_RIB_NAME
		if len(addr) > 0 {
			peer, ok := s.neighborMap[addr]
			if !ok {
				return fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
			}
			if !peer.isRouteServerClient() {
				return fmt.Errorf("Neighbor %v doesn't have local rib", addr)
			}
			id = peer.ID()
			m = s.rsRib
		}
		info, err = m.TableInfo(id, family)
		return err
	}, true)
	return
}

func (s *BgpServer) GetAdjRibInfo(addr string, family bgp.RouteFamily, in bool) (info *table.TableInfo, err error) {
	err = s.mgmtOperation(func() error {
		peer, ok := s.neighborMap[addr]
		if !ok {
			return fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
		}

		var adjRib *table.AdjRib
		if in {
			adjRib = peer.adjRibIn
		} else {
			adjRib = table.NewAdjRib(peer.ID(), peer.configuredRFlist())
			accepted, _ := peer.getBestFromLocal(peer.configuredRFlist())
			adjRib.Update(accepted)
		}
		info, err = adjRib.TableInfo(family)
		return err
	}, true)
	return
}

func (s *BgpServer) GetServer() (c *config.Global) {
	s.mgmtOperation(func() error {
		g := s.bgpConfig.Global
		c = &g
		return nil
	}, false)
	return c
}

func (s *BgpServer) GetNeighbor(address string, getAdvertised bool) (l []*config.Neighbor) {
	s.mgmtOperation(func() error {
		l = make([]*config.Neighbor, 0, len(s.neighborMap))
		for k, peer := range s.neighborMap {
			if address != "" && address != k && address != peer.fsm.pConf.Config.NeighborInterface {
				continue
			}
			l = append(l, peer.ToConfig(getAdvertised))
		}
		return nil
	}, false)
	return l
}

func (server *BgpServer) addPeerGroup(c *config.PeerGroup) error {
	name := c.Config.PeerGroupName
	if _, y := server.peerGroupMap[name]; y {
		return fmt.Errorf("Can't overwrite the existing peer-group: %s", name)
	}

	log.WithFields(log.Fields{
		"Topic": "Peer",
		"Name":  name,
	}).Info("Add a peer group configuration")

	server.peerGroupMap[c.Config.PeerGroupName] = NewPeerGroup(c)

	return nil
}

func (server *BgpServer) addNeighbor(c *config.Neighbor) error {
	addr, err := c.ExtractNeighborAddress()
	if err != nil {
		return err
	}

	if _, y := server.neighborMap[addr]; y {
		return fmt.Errorf("Can't overwrite the existing peer: %s", addr)
	}

	var pgConf *config.PeerGroup
	if c.Config.PeerGroup != "" {
		pg, ok := server.peerGroupMap[c.Config.PeerGroup]
		if !ok {
			return fmt.Errorf("no such peer-group: %s", c.Config.PeerGroup)
		}
		pgConf = pg.Conf
	}

	if err := config.SetDefaultNeighborConfigValues(c, pgConf, &server.bgpConfig.Global); err != nil {
		return err
	}

	if vrf := c.Config.Vrf; vrf != "" {
		if c.RouteServer.Config.RouteServerClient {
			return fmt.Errorf("route server client can't be enslaved to VRF")
		}
		families, _ := config.AfiSafis(c.AfiSafis).ToRfList()
		for _, f := range families {
			if f != bgp.RF_IPv4_UC && f != bgp.RF_IPv6_UC {
				return fmt.Errorf("%s is not supported for VRF enslaved neighbor", f)
			}
		}
		_, y := server.globalRib.Vrfs[vrf]
		if !y {
			return fmt.Errorf("VRF not found: %s", vrf)
		}
	}

	if c.RouteServer.Config.RouteServerClient && c.RouteReflector.Config.RouteReflectorClient {
		return fmt.Errorf("can't be both route-server-client and route-reflector-client")
	}

	if server.bgpConfig.Global.Config.Port > 0 {
		for _, l := range server.Listeners(addr) {
			if c.Config.AuthPassword != "" {
				if err := SetTcpMD5SigSockopt(l, addr, c.Config.AuthPassword); err != nil {
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   addr,
					}).Warnf("failed to set md5: %s", err)
				}
			}
		}
	}
	log.WithFields(log.Fields{
		"Topic": "Peer",
	}).Infof("Add a peer configuration for:%s", addr)

	rib := server.globalRib
	if c.RouteServer.Config.RouteServerClient {
		rib = server.rsRib
	}
	peer := NewPeer(&server.bgpConfig.Global, c, rib, server.policy)
	server.policy.Reset(nil, map[string]config.ApplyPolicy{peer.ID(): c.ApplyPolicy})
	if peer.isRouteServerClient() {
		pathList := make([]*table.Path, 0)
		rfList := peer.configuredRFlist()
		for _, p := range server.neighborMap {
			if !p.isRouteServerClient() {
				continue
			}
			pathList = append(pathList, p.getAccepted(rfList)...)
		}
		moded := server.RSimportPaths(peer, pathList)
		if len(moded) > 0 {
			server.rsRib.ProcessPaths(moded)
		}
	}
	server.neighborMap[addr] = peer
	if name := c.Config.PeerGroup; name != "" {
		server.peerGroupMap[name].AddMember(*c)
	}
	peer.startFSMHandler(server.fsmincomingCh, server.fsmStateCh)
	server.broadcastPeerState(peer, bgp.BGP_FSM_IDLE)
	return nil
}

func (s *BgpServer) AddPeerGroup(c *config.PeerGroup) error {
	return s.mgmtOperation(func() error {
		return s.addPeerGroup(c)
	}, true)
}

func (s *BgpServer) AddNeighbor(c *config.Neighbor) error {
	return s.mgmtOperation(func() error {
		return s.addNeighbor(c)
	}, true)
}

func (s *BgpServer) AddDynamicNeighbor(c *config.DynamicNeighbor) error {
	return s.mgmtOperation(func() error {
		s.peerGroupMap[c.Config.PeerGroup].AddDynamicNeighbor(c)
		return nil
	}, true)
}

func (server *BgpServer) deletePeerGroup(pg *config.PeerGroup) error {
	name := pg.Config.PeerGroupName

	if _, y := server.peerGroupMap[name]; !y {
		return fmt.Errorf("Can't delete a peer-group %s which does not exist", name)
	}

	log.WithFields(log.Fields{
		"Topic": "Peer",
		"Name":  name,
	}).Info("Delete a peer group configuration")

	delete(server.peerGroupMap, name)
	return nil
}

func (server *BgpServer) deleteNeighbor(c *config.Neighbor, code, subcode uint8) error {
	if c.Config.PeerGroup != "" {
		_, y := server.peerGroupMap[c.Config.PeerGroup]
		if y {
			server.peerGroupMap[c.Config.PeerGroup].DeleteMember(*c)
		}
	}

	addr, err := c.ExtractNeighborAddress()
	if err != nil {
		return err
	}

	if intf := c.Config.NeighborInterface; intf != "" {
		var err error
		addr, err = config.GetIPv6LinkLocalNeighborAddress(intf)
		if err != nil {
			return err
		}
	}
	n, y := server.neighborMap[addr]
	if !y {
		return fmt.Errorf("Can't delete a peer configuration for %s", addr)
	}
	for _, l := range server.Listeners(addr) {
		if err := SetTcpMD5SigSockopt(l, addr, ""); err != nil {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   addr,
			}).Warnf("failed to unset md5: %s", err)
		}
	}
	log.WithFields(log.Fields{
		"Topic": "Peer",
	}).Infof("Delete a peer configuration for:%s", addr)

	n.fsm.sendNotification(code, subcode, nil, "")
	n.stopPeerRestarting()

	go n.stopFSM()
	delete(server.neighborMap, addr)
	server.dropPeerAllRoutes(n, n.configuredRFlist())
	return nil
}

func (s *BgpServer) DeletePeerGroup(c *config.PeerGroup) error {
	return s.mgmtOperation(func() error {
		name := c.Config.PeerGroupName
		for _, n := range s.neighborMap {
			if n.fsm.pConf.Config.PeerGroup == name {
				return fmt.Errorf("failed to delete peer-group %s: neighbor %s is in use", name, n.ID())
			}
		}
		return s.deletePeerGroup(c)
	}, true)
}

func (s *BgpServer) DeleteNeighbor(c *config.Neighbor) error {
	return s.mgmtOperation(func() error {
		return s.deleteNeighbor(c, bgp.BGP_ERROR_CEASE, bgp.BGP_ERROR_SUB_PEER_DECONFIGURED)
	}, true)
}

func (s *BgpServer) updatePeerGroup(pg *config.PeerGroup) (needsSoftResetIn bool, err error) {
	name := pg.Config.PeerGroupName

	_, ok := s.peerGroupMap[name]
	if !ok {
		return false, fmt.Errorf("Peer-group %s doesn't exist.", name)
	}
	s.peerGroupMap[name].Conf = pg

	for _, n := range s.peerGroupMap[name].members {
		c := n
		u, err := s.updateNeighbor(&c)
		if err != nil {
			return needsSoftResetIn, err
		}
		needsSoftResetIn = needsSoftResetIn || u
	}
	return needsSoftResetIn, nil
}

func (s *BgpServer) UpdatePeerGroup(pg *config.PeerGroup) (needsSoftResetIn bool, err error) {
	err = s.mgmtOperation(func() error {
		needsSoftResetIn, err = s.updatePeerGroup(pg)
		return err
	}, true)
	return needsSoftResetIn, err
}

func (s *BgpServer) updateNeighbor(c *config.Neighbor) (needsSoftResetIn bool, err error) {
	if c.Config.PeerGroup != "" {
		if pg, ok := s.peerGroupMap[c.Config.PeerGroup]; ok {
			if err := config.SetDefaultNeighborConfigValues(c, pg.Conf, &s.bgpConfig.Global); err != nil {
				return needsSoftResetIn, err
			}
		} else {
			return needsSoftResetIn, fmt.Errorf("no such peer-group: %s", c.Config.PeerGroup)
		}
	}

	addr, err := c.ExtractNeighborAddress()
	if err != nil {
		return needsSoftResetIn, err
	}

	peer, ok := s.neighborMap[addr]
	if !ok {
		return needsSoftResetIn, fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
	}

	if !peer.fsm.pConf.ApplyPolicy.Equal(&c.ApplyPolicy) {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   addr,
		}).Info("Update ApplyPolicy")
		s.policy.Reset(nil, map[string]config.ApplyPolicy{peer.ID(): c.ApplyPolicy})
		peer.fsm.pConf.ApplyPolicy = c.ApplyPolicy
		needsSoftResetIn = true
	}
	original := peer.fsm.pConf

	if !original.AsPathOptions.Config.Equal(&c.AsPathOptions.Config) {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   peer.ID(),
		}).Info("Update aspath options")
		peer.fsm.pConf.AsPathOptions = c.AsPathOptions
		needsSoftResetIn = true
	}

	if !original.Config.Equal(&c.Config) || !original.Transport.Config.Equal(&c.Transport.Config) || config.CheckAfiSafisChange(original.AfiSafis, c.AfiSafis) {
		sub := uint8(bgp.BGP_ERROR_SUB_OTHER_CONFIGURATION_CHANGE)
		if original.Config.AdminDown != c.Config.AdminDown {
			sub = bgp.BGP_ERROR_SUB_ADMINISTRATIVE_SHUTDOWN
			state := "Admin Down"
			if c.Config.AdminDown == false {
				state = "Admin Up"
			}
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   peer.ID(),
				"State": state,
			}).Info("Update admin-state configuration")
		} else if original.Config.PeerAs != c.Config.PeerAs {
			sub = bgp.BGP_ERROR_SUB_PEER_DECONFIGURED
		}
		if err = s.deleteNeighbor(peer.fsm.pConf, bgp.BGP_ERROR_CEASE, sub); err != nil {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   addr,
			}).Error(err)
			return needsSoftResetIn, err
		}
		err = s.addNeighbor(c)
		if err != nil {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   addr,
			}).Error(err)
		}
		return needsSoftResetIn, err
	}

	if !original.Timers.Config.Equal(&c.Timers.Config) {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   peer.ID(),
		}).Info("Update timer configuration")
		peer.fsm.pConf.Timers.Config = c.Timers.Config
	}

	err = peer.updatePrefixLimitConfig(c.AfiSafis)
	if err != nil {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   addr,
		}).Error(err)
		// rollback to original state
		peer.fsm.pConf = original
	}
	return needsSoftResetIn, err
}

func (s *BgpServer) UpdateNeighbor(c *config.Neighbor) (needsSoftResetIn bool, err error) {
	err = s.mgmtOperation(func() error {
		needsSoftResetIn, err = s.updateNeighbor(c)
		return err
	}, true)
	return needsSoftResetIn, err
}

func (s *BgpServer) addrToPeers(addr string) (l []*Peer, err error) {
	if len(addr) == 0 {
		for _, p := range s.neighborMap {
			l = append(l, p)
		}
		return l, nil
	}
	peer, found := s.neighborMap[addr]
	if !found {
		return l, fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
	}
	return []*Peer{peer}, nil
}

func (s *BgpServer) resetNeighbor(op, addr string, subcode uint8, data []byte) error {
	log.WithFields(log.Fields{
		"Topic": "Operation",
		"Key":   addr,
	}).Info(op)

	peers, err := s.addrToPeers(addr)
	if err == nil {
		m := bgp.NewBGPNotificationMessage(bgp.BGP_ERROR_CEASE, subcode, data)
		for _, peer := range peers {
			sendFsmOutgoingMsg(peer, nil, m, false)
		}
	}
	return err
}

func (s *BgpServer) ShutdownNeighbor(addr, communication string) error {
	return s.mgmtOperation(func() error {
		return s.resetNeighbor("Neighbor shutdown", addr, bgp.BGP_ERROR_SUB_ADMINISTRATIVE_SHUTDOWN, newAdministrativeCommunication(communication))
	}, true)
}

func (s *BgpServer) ResetNeighbor(addr, communication string) error {
	return s.mgmtOperation(func() error {
		err := s.resetNeighbor("Neighbor reset", addr, bgp.BGP_ERROR_SUB_ADMINISTRATIVE_RESET, newAdministrativeCommunication(communication))
		if err != nil {
			return err
		}
		peers, _ := s.addrToPeers(addr)
		for _, peer := range peers {
			peer.fsm.idleHoldTime = peer.fsm.pConf.Timers.Config.IdleHoldTimeAfterReset
		}
		return nil
	}, true)
}

func (s *BgpServer) setAdminState(addr, communication string, enable bool) error {
	peers, err := s.addrToPeers(addr)
	if err != nil {
		return err
	}
	for _, peer := range peers {
		f := func(stateOp *AdminStateOperation, message string) {
			select {
			case peer.fsm.adminStateCh <- *stateOp:
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   peer.fsm.pConf.State.NeighborAddress,
				}).Debug(message)
			default:
				log.Warning("previous request is still remaining. : ", peer.fsm.pConf.State.NeighborAddress)
			}
		}
		if enable {
			f(&AdminStateOperation{ADMIN_STATE_UP, nil}, "ADMIN_STATE_UP requested")
		} else {
			f(&AdminStateOperation{ADMIN_STATE_DOWN, newAdministrativeCommunication(communication)}, "ADMIN_STATE_DOWN requested")
		}
	}
	return nil
}

func (s *BgpServer) EnableNeighbor(addr string) error {
	return s.mgmtOperation(func() error {
		return s.setAdminState(addr, "", true)
	}, true)
}

func (s *BgpServer) DisableNeighbor(addr, communication string) error {
	return s.mgmtOperation(func() error {
		return s.setAdminState(addr, communication, false)
	}, true)
}

func (s *BgpServer) GetDefinedSet(typ table.DefinedType, name string) (sets *config.DefinedSets, err error) {
	err = s.mgmtOperation(func() error {
		sets, err = s.policy.GetDefinedSet(typ, name)
		return nil
	}, false)
	return sets, err
}

func (s *BgpServer) AddDefinedSet(a table.DefinedSet) error {
	return s.mgmtOperation(func() error {
		return s.policy.AddDefinedSet(a)
	}, false)
}

func (s *BgpServer) DeleteDefinedSet(a table.DefinedSet, all bool) error {
	return s.mgmtOperation(func() error {
		return s.policy.DeleteDefinedSet(a, all)
	}, false)
}

func (s *BgpServer) ReplaceDefinedSet(a table.DefinedSet) error {
	return s.mgmtOperation(func() error {
		return s.policy.ReplaceDefinedSet(a)
	}, false)
}

func (s *BgpServer) GetStatement() (l []*config.Statement) {
	s.mgmtOperation(func() error {
		l = s.policy.GetStatement()
		return nil
	}, false)
	return l
}

func (s *BgpServer) AddStatement(st *table.Statement) error {
	return s.mgmtOperation(func() error {
		return s.policy.AddStatement(st)
	}, false)
}

func (s *BgpServer) DeleteStatement(st *table.Statement, all bool) error {
	return s.mgmtOperation(func() error {
		return s.policy.DeleteStatement(st, all)
	}, false)
}

func (s *BgpServer) ReplaceStatement(st *table.Statement) error {
	return s.mgmtOperation(func() error {
		return s.policy.ReplaceStatement(st)
	}, false)
}

func (s *BgpServer) GetPolicy() (l []*config.PolicyDefinition) {
	s.mgmtOperation(func() error {
		l = s.policy.GetAllPolicy()
		return nil
	}, false)
	return l
}

func (s *BgpServer) AddPolicy(x *table.Policy, refer bool) error {
	return s.mgmtOperation(func() error {
		return s.policy.AddPolicy(x, refer)
	}, false)
}

func (s *BgpServer) DeletePolicy(x *table.Policy, all, preserve bool) error {
	return s.mgmtOperation(func() error {
		l := make([]string, 0, len(s.neighborMap)+1)
		for _, peer := range s.neighborMap {
			l = append(l, peer.ID())
		}
		l = append(l, table.GLOBAL_RIB_NAME)

		return s.policy.DeletePolicy(x, all, preserve, l)
	}, false)
}

func (s *BgpServer) ReplacePolicy(x *table.Policy, refer, preserve bool) error {
	return s.mgmtOperation(func() error {
		return s.policy.ReplacePolicy(x, refer, preserve)
	}, false)
}

func (server *BgpServer) toPolicyInfo(name string, dir table.PolicyDirection) (string, error) {
	if name == "" {
		switch dir {
		case table.POLICY_DIRECTION_IMPORT, table.POLICY_DIRECTION_EXPORT:
			return table.GLOBAL_RIB_NAME, nil
		}
		return "", fmt.Errorf("invalid policy type")
	} else {
		peer, ok := server.neighborMap[name]
		if !ok {
			return "", fmt.Errorf("not found peer %s", name)
		}
		if !peer.isRouteServerClient() {
			return "", fmt.Errorf("non-rs-client peer %s doesn't have per peer policy", name)
		}
		return peer.ID(), nil
	}
}

func (s *BgpServer) GetPolicyAssignment(name string, dir table.PolicyDirection) (rt table.RouteType, l []*config.PolicyDefinition, err error) {
	err = s.mgmtOperation(func() error {
		var id string
		id, err = s.toPolicyInfo(name, dir)
		if err != nil {
			rt = table.ROUTE_TYPE_NONE
			return err
		}
		rt, l, err = s.policy.GetPolicyAssignment(id, dir)
		return nil
	}, false)
	return rt, l, err
}

func (s *BgpServer) AddPolicyAssignment(name string, dir table.PolicyDirection, policies []*config.PolicyDefinition, def table.RouteType) error {
	return s.mgmtOperation(func() error {
		id, err := s.toPolicyInfo(name, dir)
		if err != nil {
			return err
		}
		return s.policy.AddPolicyAssignment(id, dir, policies, def)
	}, false)
}

func (s *BgpServer) DeletePolicyAssignment(name string, dir table.PolicyDirection, policies []*config.PolicyDefinition, all bool) error {
	return s.mgmtOperation(func() error {
		id, err := s.toPolicyInfo(name, dir)
		if err != nil {
			return err
		}
		return s.policy.DeletePolicyAssignment(id, dir, policies, all)
	}, false)
}

func (s *BgpServer) ReplacePolicyAssignment(name string, dir table.PolicyDirection, policies []*config.PolicyDefinition, def table.RouteType) error {
	return s.mgmtOperation(func() error {
		id, err := s.toPolicyInfo(name, dir)
		if err != nil {
			return err
		}
		return s.policy.ReplacePolicyAssignment(id, dir, policies, def)
	}, false)
}

func (s *BgpServer) EnableMrt(c *config.MrtConfig) error {
	return s.mgmtOperation(func() error {
		return s.mrtManager.enable(c)
	}, false)
}

func (s *BgpServer) DisableMrt(c *config.MrtConfig) error {
	return s.mgmtOperation(func() error {
		return s.mrtManager.disable(c)
	}, false)
}

func (s *BgpServer) ValidateRib(prefix string) error {
	return s.mgmtOperation(func() error {
		for _, rf := range s.globalRib.GetRFlist() {
			if t, ok := s.globalRib.Tables[rf]; ok {
				dsts := t.GetDestinations()
				if prefix != "" {
					_, p, _ := net.ParseCIDR(prefix)
					if dst := t.GetDestination(p.String()); dst != nil {
						dsts = map[string]*table.Destination{p.String(): dst}
					}
				}
				for _, dst := range dsts {
					s.roaManager.validate(dst.GetAllKnownPathList())
				}
			}
		}
		return nil
	}, true)
}

func (s *BgpServer) GetRpki() (l []*config.RpkiServer, err error) {
	err = s.mgmtOperation(func() error {
		l = s.roaManager.GetServers()
		return nil
	}, false)
	return l, err
}

func (s *BgpServer) GetRoa(family bgp.RouteFamily) (l []*table.ROA, err error) {
	s.mgmtOperation(func() error {
		l, err = s.roaManager.GetRoa(family)
		return nil
	}, false)
	return l, err
}

func (s *BgpServer) AddRpki(c *config.RpkiServerConfig) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.AddServer(net.JoinHostPort(c.Address, strconv.Itoa(int(c.Port))), c.RecordLifetime)
	}, false)
}

func (s *BgpServer) DeleteRpki(c *config.RpkiServerConfig) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.DeleteServer(c.Address)
	}, false)
}

func (s *BgpServer) EnableRpki(c *config.RpkiServerConfig) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.Enable(c.Address)
	}, false)
}

func (s *BgpServer) DisableRpki(c *config.RpkiServerConfig) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.Disable(c.Address)
	}, false)
}

func (s *BgpServer) ResetRpki(c *config.RpkiServerConfig) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.Reset(c.Address)
	}, false)
}

func (s *BgpServer) SoftResetRpki(c *config.RpkiServerConfig) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.SoftReset(c.Address)
	}, false)
}

type WatchEventType string

const (
	WATCH_EVENT_TYPE_BEST_PATH   WatchEventType = "bestpath"
	WATCH_EVENT_TYPE_PRE_UPDATE  WatchEventType = "preupdate"
	WATCH_EVENT_TYPE_POST_UPDATE WatchEventType = "postupdate"
	WATCH_EVENT_TYPE_PEER_STATE  WatchEventType = "peerstate"
	WATCH_EVENT_TYPE_TABLE       WatchEventType = "table"
	WATCH_EVENT_TYPE_RECV_MSG    WatchEventType = "receivedmessage"
)

type WatchEvent interface {
}

type WatchEventUpdate struct {
	Message      *bgp.BGPMessage
	PeerAS       uint32
	LocalAS      uint32
	PeerAddress  net.IP
	LocalAddress net.IP
	PeerID       net.IP
	FourBytesAs  bool
	Timestamp    time.Time
	Payload      []byte
	PostPolicy   bool
	PathList     []*table.Path
	Neighbor     *config.Neighbor
}

type WatchEventPeerState struct {
	PeerAS        uint32
	LocalAS       uint32
	PeerAddress   net.IP
	LocalAddress  net.IP
	PeerPort      uint16
	LocalPort     uint16
	PeerID        net.IP
	SentOpen      *bgp.BGPMessage
	RecvOpen      *bgp.BGPMessage
	State         bgp.FSMState
	AdminState    AdminState
	Timestamp     time.Time
	PeerInterface string
}

type WatchEventAdjIn struct {
	PathList []*table.Path
}

type WatchEventTable struct {
	RouterId string
	PathList map[string][]*table.Path
	Neighbor []*config.Neighbor
}

type WatchEventBestPath struct {
	PathList      []*table.Path
	MultiPathList [][]*table.Path
}

type WatchEventMessage struct {
	Message      *bgp.BGPMessage
	PeerAS       uint32
	LocalAS      uint32
	PeerAddress  net.IP
	LocalAddress net.IP
	PeerID       net.IP
	FourBytesAs  bool
	Timestamp    time.Time
	IsSent       bool
}

type watchOptions struct {
	bestpath       bool
	preUpdate      bool
	postUpdate     bool
	peerState      bool
	initBest       bool
	initUpdate     bool
	initPostUpdate bool
	initPeerState  bool
	tableName      string
	recvMessage    bool
	sentMessage    bool
}

type WatchOption func(*watchOptions)

func WatchBestPath(current bool) WatchOption {
	return func(o *watchOptions) {
		o.bestpath = true
		if current {
			o.initBest = true
		}
	}
}

func WatchUpdate(current bool) WatchOption {
	return func(o *watchOptions) {
		o.preUpdate = true
		if current {
			o.initUpdate = true
		}
	}
}

func WatchPostUpdate(current bool) WatchOption {
	return func(o *watchOptions) {
		o.postUpdate = true
		if current {
			o.initPostUpdate = true
		}
	}
}

func WatchPeerState(current bool) WatchOption {
	return func(o *watchOptions) {
		o.peerState = true
		if current {
			o.initPeerState = true
		}
	}
}

func WatchTableName(name string) WatchOption {
	return func(o *watchOptions) {
		o.tableName = name
	}
}

func WatchMessage(isSent bool) WatchOption {
	return func(o *watchOptions) {
		if isSent {
			log.WithFields(log.Fields{
				"Topic": "Server",
			}).Warn("watch event for sent messages is not implemented yet")
			// o.sentMessage = true
		} else {
			o.recvMessage = true
		}
	}
}

type Watcher struct {
	opts   watchOptions
	realCh chan WatchEvent
	ch     *channels.InfiniteChannel
	s      *BgpServer
}

func (w *Watcher) Event() <-chan WatchEvent {
	return w.realCh
}

func (w *Watcher) Generate(t WatchEventType) error {
	return w.s.mgmtOperation(func() error {
		switch t {
		case WATCH_EVENT_TYPE_PRE_UPDATE:
			pathList := make([]*table.Path, 0)
			for _, peer := range w.s.neighborMap {
				pathList = append(pathList, peer.adjRibIn.PathList(peer.configuredRFlist(), false)...)
			}
			w.notify(&WatchEventAdjIn{PathList: clonePathList(pathList)})
		case WATCH_EVENT_TYPE_TABLE:
			rib := w.s.globalRib
			id := table.GLOBAL_RIB_NAME
			if len(w.opts.tableName) > 0 {
				peer, ok := w.s.neighborMap[w.opts.tableName]
				if !ok {
					return fmt.Errorf("Neighbor that has %v doesn't exist.", w.opts.tableName)
				}
				if !peer.isRouteServerClient() {
					return fmt.Errorf("Neighbor %v doesn't have local rib", w.opts.tableName)
				}
				id = peer.ID()
				rib = w.s.rsRib
			}

			pathList := func() map[string][]*table.Path {
				pathList := make(map[string][]*table.Path)
				for _, t := range rib.Tables {
					for _, dst := range t.GetSortedDestinations() {
						if paths := dst.GetKnownPathList(id); len(paths) > 0 {
							pathList[dst.GetNlri().String()] = clonePathList(paths)
						}
					}
				}
				return pathList
			}()
			l := make([]*config.Neighbor, 0, len(w.s.neighborMap))
			for _, peer := range w.s.neighborMap {
				l = append(l, peer.ToConfig(false))
			}
			w.notify(&WatchEventTable{PathList: pathList, Neighbor: l})
		default:
			return fmt.Errorf("unsupported type %v", t)
		}
		return nil
	}, false)
}

func (w *Watcher) notify(v WatchEvent) {
	w.ch.In() <- v
}

func (w *Watcher) loop() {
	for {
		select {
		case ev, ok := <-w.ch.Out():
			if !ok {
				close(w.realCh)
				return
			}
			w.realCh <- ev.(WatchEvent)
		}
	}
}

func (w *Watcher) Stop() {
	w.s.mgmtOperation(func() error {
		for k, l := range w.s.watcherMap {
			for i, v := range l {
				if w == v {
					w.s.watcherMap[k] = append(l[:i], l[i+1:]...)
					break
				}
			}
		}

		cleanInfiniteChannel(w.ch)
		// the loop function goroutine might be blocked for
		// writing to realCh. make sure it finishes.
		for range w.realCh {
		}
		return nil
	}, false)
}

func (s *BgpServer) isWatched(typ WatchEventType) bool {
	return len(s.watcherMap[typ]) != 0
}

func (s *BgpServer) notifyWatcher(typ WatchEventType, ev WatchEvent) {
	for _, w := range s.watcherMap[typ] {
		w.notify(ev)
	}
}

func (s *BgpServer) Watch(opts ...WatchOption) (w *Watcher) {
	s.mgmtOperation(func() error {
		w = &Watcher{
			s:      s,
			realCh: make(chan WatchEvent, 8),
			ch:     channels.NewInfiniteChannel(),
		}

		for _, opt := range opts {
			opt(&w.opts)
		}

		register := func(t WatchEventType, w *Watcher) {
			s.watcherMap[t] = append(s.watcherMap[t], w)
		}

		if w.opts.bestpath {
			register(WATCH_EVENT_TYPE_BEST_PATH, w)
		}
		if w.opts.preUpdate {
			register(WATCH_EVENT_TYPE_PRE_UPDATE, w)
		}
		if w.opts.postUpdate {
			register(WATCH_EVENT_TYPE_POST_UPDATE, w)
		}
		if w.opts.peerState {
			register(WATCH_EVENT_TYPE_PEER_STATE, w)
		}
		if w.opts.initPeerState {
			for _, peer := range s.neighborMap {
				if peer.fsm.state != bgp.BGP_FSM_ESTABLISHED {
					continue
				}
				w.notify(createWatchEventPeerState(peer))
			}
		}
		if w.opts.initBest && s.active() == nil {
			w.notify(&WatchEventBestPath{
				PathList:      s.globalRib.GetBestPathList(table.GLOBAL_RIB_NAME, nil),
				MultiPathList: s.globalRib.GetBestMultiPathList(table.GLOBAL_RIB_NAME, nil),
			})
		}
		if w.opts.initUpdate {
			for _, peer := range s.neighborMap {
				if peer.fsm.state != bgp.BGP_FSM_ESTABLISHED {
					continue
				}
				configNeighbor := peer.ToConfig(false)
				for _, rf := range peer.configuredRFlist() {
					_, y := peer.fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]
					l, _ := peer.fsm.LocalHostPort()
					for _, path := range peer.adjRibIn.PathList([]bgp.RouteFamily{rf}, false) {
						msgs := table.CreateUpdateMsgFromPaths([]*table.Path{path})
						buf, _ := msgs[0].Serialize()
						w.notify(&WatchEventUpdate{
							Message:      msgs[0],
							PeerAS:       peer.fsm.peerInfo.AS,
							LocalAS:      peer.fsm.peerInfo.LocalAS,
							PeerAddress:  peer.fsm.peerInfo.Address,
							LocalAddress: net.ParseIP(l),
							PeerID:       peer.fsm.peerInfo.ID,
							FourBytesAs:  y,
							Timestamp:    path.GetTimestamp(),
							Payload:      buf,
							PostPolicy:   false,
							Neighbor:     configNeighbor,
						})
					}
					eor := bgp.NewEndOfRib(rf)
					eorBuf, _ := eor.Serialize()
					w.notify(&WatchEventUpdate{
						Message:      eor,
						PeerAS:       peer.fsm.peerInfo.AS,
						LocalAS:      peer.fsm.peerInfo.LocalAS,
						PeerAddress:  peer.fsm.peerInfo.Address,
						LocalAddress: net.ParseIP(l),
						PeerID:       peer.fsm.peerInfo.ID,
						FourBytesAs:  y,
						Timestamp:    time.Now(),
						Payload:      eorBuf,
						PostPolicy:   false,
						Neighbor:     configNeighbor,
					})
				}
			}
		}
		if w.opts.initPostUpdate && s.active() == nil {
			for _, rf := range s.globalRib.GetRFlist() {
				if len(s.globalRib.Tables[rf].GetDestinations()) == 0 {
					continue
				}
				pathsByPeer := make(map[*table.PeerInfo][]*table.Path)
				for _, path := range s.globalRib.GetPathList(table.GLOBAL_RIB_NAME, []bgp.RouteFamily{rf}) {
					pathsByPeer[path.GetSource()] = append(pathsByPeer[path.GetSource()], path)
				}
				for peerInfo, paths := range pathsByPeer {
					// create copy which can be access to without mutex
					var configNeighbor *config.Neighbor
					if peer, ok := s.neighborMap[peerInfo.Address.String()]; ok {
						configNeighbor = peer.ToConfig(false)
					}
					for _, path := range paths {
						msgs := table.CreateUpdateMsgFromPaths([]*table.Path{path})
						buf, _ := msgs[0].Serialize()
						w.notify(&WatchEventUpdate{
							Message:     msgs[0],
							PeerAS:      peerInfo.AS,
							PeerAddress: peerInfo.Address,
							PeerID:      peerInfo.ID,
							Timestamp:   path.GetTimestamp(),
							Payload:     buf,
							PostPolicy:  true,
							Neighbor:    configNeighbor,
						})
					}
					eor := bgp.NewEndOfRib(rf)
					eorBuf, _ := eor.Serialize()
					w.notify(&WatchEventUpdate{
						Message:     eor,
						PeerAS:      peerInfo.AS,
						PeerAddress: peerInfo.Address,
						PeerID:      peerInfo.ID,
						Timestamp:   time.Now(),
						Payload:     eorBuf,
						PostPolicy:  true,
						Neighbor:    configNeighbor,
					})
				}
			}
		}
		if w.opts.recvMessage {
			register(WATCH_EVENT_TYPE_RECV_MSG, w)
		}

		go w.loop()
		return nil
	}, false)
	return w
}
