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
	"strconv"
	"sync"
	"time"

	"github.com/eapache/channels"
	uuid "github.com/satori/go.uuid"
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
	shutdownWG   *sync.WaitGroup
	watcherMap   map[WatchEventType][]*Watcher
	zclient      *zebraClient
	bmpManager   *bmpClientManager
	mrtManager   *mrtManager
	uuidMap      map[uuid.UUID]string
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
		uuidMap:      make(map[uuid.UUID]string),
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
			}).Warnf("Can't find the neighbor %s", e.MsgSrc)
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

				if !localAddrValid {
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
				rib := server.globalRib
				if pg.Conf.RouteServer.Config.RouteServerClient {
					rib = server.rsRib
				}
				peer := newDynamicPeer(&server.bgpConfig.Global, remoteAddr, pg.Conf, rib, server.policy)
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
				server.broadcastPeerState(peer, bgp.BGP_FSM_ACTIVE, nil)
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

	//RFC4684 Constrained Route Distribution
	if _, y := peer.fsm.rfMap[bgp.RF_RTC_UC]; y && path.GetRouteFamily() != bgp.RF_RTC_UC {
		ignore := true
		for _, ext := range path.GetExtCommunities() {
			for _, p := range peer.adjRibIn.PathList([]bgp.RouteFamily{bgp.RF_RTC_UC}, true) {
				rt := p.GetNlri().(*bgp.RouteTargetMembershipNLRI).RouteTarget
				// Note: nil RT means the default route target
				if rt == nil || ext.String() == rt.String() {
					ignore = false
					break
				}
			}
			if !ignore {
				break
			}
		}
		if ignore {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   peer.ID(),
				"Data":  path,
			}).Debug("Filtered by Route Target Constraint, ignore")
			return nil
		}
	}

	//iBGP handling
	if peer.isIBGPPeer() {
		ignore := false
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
				for _, clusterID := range path.GetClusterList() {
					if clusterID.Equal(peer.fsm.peerInfo.RouteReflectorClusterID) {
						log.WithFields(log.Fields{
							"Topic":     "Peer",
							"Key":       peer.ID(),
							"ClusterID": clusterID,
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

	if path = peer.filterPathFromSourcePeer(path, old); path == nil {
		return nil
	}

	if !peer.isRouteServerClient() && isASLoop(peer, path) {
		return nil
	}
	return path
}

func (s *BgpServer) filterpath(peer *Peer, path, old *table.Path) *table.Path {
	// Special handling for RTM NLRI.
	if path != nil && path.GetRouteFamily() == bgp.RF_RTC_UC && !path.IsWithdraw {
		// If the given "path" is locally generated and the same with "old", we
		// assumes "path" was already sent before. This assumption avoids the
		// infinite UPDATE loop between Route Reflector and its clients.
		if path.IsLocal() && path == old {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   peer.fsm.pConf.State.NeighborAddress,
				"Path":  path,
			}).Debug("given rtm nlri is already sent, skipping to advertise")
			return nil
		}

		if old != nil && old.IsLocal() {
			// We assumes VRF with the specific RT is deleted.
			path = old.Clone(true)
		} else if peer.isRouteReflectorClient() {
			// We need to send the path even if the peer is originator of the
			// path in order to signal that the client should distribute route
			// with the given RT.
		} else {
			// We send a path even if it is not the best path. See comments in
			// (*Destination) GetChanges().
			dst := peer.localRib.GetDestination(path)
			path = nil
			for _, p := range dst.GetKnownPathList(peer.TableID(), peer.AS()) {
				srcPeer := p.GetSource()
				if peer.ID() != srcPeer.Address.String() {
					if srcPeer.RouteReflectorClient {
						// The path from a RR client is preferred than others
						// for the case that RR and non RR client peering
						// (e.g., peering of different RR clusters).
						path = p
						break
					} else if path == nil {
						path = p
					}
				}
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

	options := &table.PolicyOptions{
		Info:       peer.fsm.peerInfo,
		OldNextHop: path.GetNexthop(),
	}
	path = table.UpdatePathAttrs(peer.fsm.gConf, peer.fsm.pConf, peer.fsm.peerInfo, path)

	if v := s.roaManager.validate(path); v != nil {
		options.ValidationResult = v
	}

	path = peer.policy.ApplyPolicy(peer.TableID(), table.POLICY_DIRECTION_EXPORT, path, options)
	// When 'path' is filtered (path == nil), check 'old' has been sent to this peer.
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
	if table.SelectionOptions.DisableBestPathSelection {
		// Note: If best path selection disabled, no best path to notify.
		return
	}
	clonedM := make([][]*table.Path, len(multipath))
	for i, pathList := range multipath {
		clonedM[i] = clonePathList(pathList)
	}
	clonedB := clonePathList(best)
	m := make(map[string]uint16)
	for _, p := range clonedB {
		switch p.GetRouteFamily() {
		case bgp.RF_IPv4_VPN, bgp.RF_IPv6_VPN:
			for _, vrf := range server.globalRib.Vrfs {
				if vrf.Id != 0 && table.CanImportToVrf(vrf, p) {
					m[p.GetNlri().String()] = uint16(vrf.Id)
				}
			}
		}
	}
	w := &WatchEventBestPath{PathList: clonedB, MultiPathList: clonedM}
	if len(m) > 0 {
		w.Vrf = m
	}
	server.notifyWatcher(WATCH_EVENT_TYPE_BEST_PATH, w)
}

func (s *BgpServer) ToConfig(peer *Peer, getAdvertised bool) *config.Neighbor {
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
			pathList, filtered := s.getBestFromLocal(peer, rfList)
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
		Neighbor:     server.ToConfig(peer, false),
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
		Neighbor:     server.ToConfig(peer, false),
	}
	server.notifyWatcher(WATCH_EVENT_TYPE_POST_UPDATE, ev)
}

func newWatchEventPeerState(peer *Peer, m *FsmMsg) *WatchEventPeerState {
	_, rport := peer.fsm.RemoteHostPort()
	laddr, lport := peer.fsm.LocalHostPort()
	sentOpen := buildopen(peer.fsm.gConf, peer.fsm.pConf)
	recvOpen := peer.fsm.recvOpen
	e := &WatchEventPeerState{
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

	if m != nil {
		e.StateReason = m.StateReason
	}
	return e
}

func (server *BgpServer) broadcastPeerState(peer *Peer, oldState bgp.FSMState, e *FsmMsg) {
	newState := peer.fsm.state
	if oldState == bgp.BGP_FSM_ESTABLISHED || newState == bgp.BGP_FSM_ESTABLISHED {
		server.notifyWatcher(WATCH_EVENT_TYPE_PEER_STATE, newWatchEventPeerState(peer, e))
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

func (s *BgpServer) getBestFromLocal(peer *Peer, rfList []bgp.RouteFamily) ([]*table.Path, []*table.Path) {
	pathList := []*table.Path{}
	filtered := []*table.Path{}
	for _, family := range peer.toGlobalFamilies(rfList) {
		pl := func() []*table.Path {
			if peer.isAddPathSendEnabled(family) {
				return peer.localRib.GetPathList(peer.TableID(), peer.AS(), []bgp.RouteFamily{family})
			}
			return peer.localRib.GetBestPathList(peer.TableID(), peer.AS(), []bgp.RouteFamily{family})
		}()
		for _, path := range pl {
			if p := s.filterpath(peer, path, nil); p != nil {
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

func (s *BgpServer) processOutgoingPaths(peer *Peer, paths, olds []*table.Path) []*table.Path {
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
		if p := s.filterpath(peer, path, old); p != nil {
			outgoing = append(outgoing, p)
		}
	}
	return outgoing
}

func (s *BgpServer) handleRouteRefresh(peer *Peer, e *FsmMsg) []*table.Path {
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
	accepted, filtered := s.getBestFromLocal(peer, rfList)
	for _, path := range filtered {
		path.IsWithdraw = true
		accepted = append(accepted, path)
	}
	return accepted
}

func (server *BgpServer) propagateUpdate(peer *Peer, pathList []*table.Path) {
	rs := peer != nil && peer.isRouteServerClient()
	vrf := !rs && peer != nil && peer.fsm.pConf.Config.Vrf != ""

	tableId := table.GLOBAL_RIB_NAME
	rib := server.globalRib
	if rs {
		tableId = peer.TableID()
		rib = server.rsRib
	}

	for _, path := range pathList {
		if vrf {
			path = path.ToGlobal(rib.Vrfs[peer.fsm.pConf.Config.Vrf])
		}

		policyOptions := &table.PolicyOptions{}

		if !rs && peer != nil {
			policyOptions.Info = peer.fsm.peerInfo
		}
		if v := server.roaManager.validate(path); v != nil {
			policyOptions.ValidationResult = v
		}

		if p := server.policy.ApplyPolicy(tableId, table.POLICY_DIRECTION_IMPORT, path, policyOptions); p != nil {
			path = p
		} else {
			path = path.Clone(true)
		}

		if !rs {
			server.notifyPostPolicyUpdateWatcher(peer, []*table.Path{path})

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
				fs := make([]bgp.RouteFamily, 0, len(peer.negotiatedRFList()))
				for _, f := range peer.negotiatedRFList() {
					if f != bgp.RF_RTC_UC {
						fs = append(fs, f)
					}
				}
				var candidates []*table.Path
				if path.IsWithdraw {
					// Note: The paths to be withdrawn are filtered because the
					// given RT on RTM NLRI is already removed from adj-RIB-in.
					_, candidates = server.getBestFromLocal(peer, fs)
				} else {
					candidates = server.globalRib.GetBestPathList(peer.TableID(), 0, fs)
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
					// Skips filtering because the paths are already filtered
					// and the withdrawal does not need the path attributes.
				} else {
					paths = server.processOutgoingPaths(peer, paths, nil)
				}
				sendFsmOutgoingMsg(peer, paths, nil, false)
			}
		}

		if dsts := rib.Update(path); len(dsts) > 0 {
			server.propagateUpdateToNeighbors(peer, path, dsts, true)
		}
	}
}

func (server *BgpServer) dropPeerAllRoutes(peer *Peer, families []bgp.RouteFamily) {
	rib := server.globalRib
	if peer.isRouteServerClient() {
		rib = server.rsRib
	}
	for _, family := range peer.toGlobalFamilies(families) {
		for _, path := range rib.GetPathListByPeer(peer.fsm.peerInfo, family) {
			p := path.Clone(true)
			if dsts := rib.Update(p); len(dsts) > 0 {
				server.propagateUpdateToNeighbors(peer, p, dsts, false)
			}
		}
	}
}

func dstsToPaths(id string, as uint32, dsts []*table.Update) ([]*table.Path, []*table.Path, [][]*table.Path) {
	bestList := make([]*table.Path, 0, len(dsts))
	oldList := make([]*table.Path, 0, len(dsts))
	mpathList := make([][]*table.Path, 0, len(dsts))

	for _, dst := range dsts {
		best, old, mpath := dst.GetChanges(id, as, false)
		bestList = append(bestList, best)
		oldList = append(oldList, old)
		if mpath != nil {
			mpathList = append(mpathList, mpath)
		}
	}
	return bestList, oldList, mpathList
}

func (server *BgpServer) propagateUpdateToNeighbors(source *Peer, newPath *table.Path, dsts []*table.Update, needOld bool) {
	if table.SelectionOptions.DisableBestPathSelection {
		return
	}
	var gBestList, gOldList, bestList, oldList []*table.Path
	var mpathList [][]*table.Path
	if source == nil || !source.isRouteServerClient() {
		gBestList, gOldList, mpathList = dstsToPaths(table.GLOBAL_RIB_NAME, 0, dsts)
		server.notifyBestWatcher(gBestList, mpathList)
	}
	family := newPath.GetRouteFamily()
	for _, targetPeer := range server.neighborMap {
		if (source == nil && targetPeer.isRouteServerClient()) || (source != nil && source.isRouteServerClient() != targetPeer.isRouteServerClient()) {
			continue
		}
		f := func() bgp.RouteFamily {
			if targetPeer.fsm.pConf.Config.Vrf != "" {
				switch family {
				case bgp.RF_IPv4_VPN:
					return bgp.RF_IPv4_UC
				case bgp.RF_IPv6_VPN:
					return bgp.RF_IPv6_UC
				}
			}
			return family
		}()
		if targetPeer.isAddPathSendEnabled(f) {
			if newPath.IsWithdraw {
				bestList = func() []*table.Path {
					l := make([]*table.Path, 0, len(dsts))
					for _, d := range dsts {
						l = append(l, d.GetWithdrawnPath()...)
					}
					return l
				}()
			} else {
				bestList = []*table.Path{newPath}
				if newPath.GetRouteFamily() == bgp.RF_RTC_UC {
					// we assumes that new "path" nlri was already sent before. This assumption avoids the
					// infinite UPDATE loop between Route Reflector and its clients.
					for _, old := range dsts[0].OldKnownPathList {
						if old.IsLocal() {
							bestList = []*table.Path{}
							break
						}
					}
				}
			}
			oldList = nil
		} else if targetPeer.isRouteServerClient() {
			bestList, oldList, _ = dstsToPaths(targetPeer.TableID(), targetPeer.AS(), dsts)
		} else {
			bestList = gBestList
			oldList = gOldList
		}
		if !needOld {
			oldList = nil
		}
		if paths := server.processOutgoingPaths(targetPeer, bestList, oldList); len(paths) > 0 {
			sendFsmOutgoingMsg(targetPeer, paths, nil, false)
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

		// PeerDown
		if oldState == bgp.BGP_FSM_ESTABLISHED {
			t := time.Now()
			if t.Sub(time.Unix(peer.fsm.pConf.Timers.State.Uptime, 0)) < FLOP_THRESHOLD {
				peer.fsm.pConf.State.Flops++
			}
			var drop []bgp.RouteFamily
			if peer.fsm.reason.Type == FSM_GRACEFUL_RESTART {
				peer.fsm.pConf.GracefulRestart.State.PeerRestarting = true
				var p []bgp.RouteFamily
				p, drop = peer.forwardingPreservedFamilies()
				server.propagateUpdate(peer, peer.StaleAll(p))
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
				server.broadcastPeerState(peer, oldState, e)
				return
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
					pathList, _ = server.getBestFromLocal(peer, []bgp.RouteFamily{bgp.RF_RTC_UC})
					t := c.RouteTargetMembership.Config.DeferralTime
					for _, f := range peer.negotiatedRFList() {
						if f != bgp.RF_RTC_UC {
							time.AfterFunc(time.Second*time.Duration(t), deferralExpiredFunc(f))
						}
					}
				} else {
					pathList, _ = server.getBestFromLocal(peer, peer.negotiatedRFList())
				}

				if len(pathList) > 0 {
					sendFsmOutgoingMsg(peer, pathList, nil, false)
				}
			} else {
				// RFC 4724 4.1
				// Once the session between the Restarting Speaker and the Receiving
				// Speaker is re-established, ...snip... it MUST defer route
				// selection for an address family until it either (a) receives the
				// End-of-RIB marker from all its peers (excluding the ones with the
				// "Restart State" bit set in the received capability and excluding the
				// ones that do not advertise the graceful restart capability) or (b)
				// the Selection_Deferral_Timer referred to below has expired.
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
						paths, _ := server.getBestFromLocal(p, p.configuredRFlist())
						if len(paths) > 0 {
							sendFsmOutgoingMsg(p, paths, nil, false)
						}
					}
					log.WithFields(log.Fields{
						"Topic": "Server",
					}).Info("sync finished")
				} else {
					deferral := peer.fsm.pConf.GracefulRestart.Config.DeferralTime
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   peer.ID(),
					}).Debugf("Now syncing, suppress sending updates. start deferral timer(%d)", deferral)
					time.AfterFunc(time.Second*time.Duration(deferral), deferralExpiredFunc(bgp.RouteFamily(0)))
				}
			}
		} else {
			if server.shutdownWG != nil && nextState == bgp.BGP_FSM_IDLE {
				die := true
				for _, p := range server.neighborMap {
					if p.fsm.state != bgp.BGP_FSM_IDLE {
						die = false
						break
					}
				}
				if die {
					server.shutdownWG.Done()
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
		server.broadcastPeerState(peer, oldState, e)
	case FSM_MSG_ROUTE_REFRESH:
		if peer.fsm.state != bgp.BGP_FSM_ESTABLISHED || e.timestamp.Unix() < peer.fsm.pConf.Timers.State.Uptime {
			return
		}
		if paths := server.handleRouteRefresh(peer, e); len(paths) > 0 {
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
							paths, _ := server.getBestFromLocal(p, p.negotiatedRFList())
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
					families := make([]bgp.RouteFamily, 0, len(peer.negotiatedRFList()))
					for _, f := range peer.negotiatedRFList() {
						if f != bgp.RF_RTC_UC {
							families = append(families, f)
						}
					}
					if paths, _ := server.getBestFromLocal(peer, families); len(paths) > 0 {
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
}

func (s *BgpServer) AddCollector(c *config.CollectorConfig) error {
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
		s.shutdownWG = new(sync.WaitGroup)
		s.shutdownWG.Add(1)
		stateOp := AdminStateOperation{
			State:         ADMIN_STATE_DOWN,
			Communication: nil,
		}
		for _, p := range s.neighborMap {
			p.fsm.adminStateCh <- stateOp
		}
		// TODO: call fsmincomingCh.Close()
		return nil
	}, false)

	// Waits for all goroutines per peer to stop.
	// Note: This should not be wrapped with s.mgmtOperation() in order to
	// avoid the deadlock in the main goroutine of BgpServer.
	if s.shutdownWG != nil {
		s.shutdownWG.Wait()
		s.shutdownWG = nil
	}
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
		if !path.IsWithdraw {
			if _, err := path.GetOrigin(); err != nil {
				return err
			}
		}

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
				paths := server.globalRib.GetBestPathList(table.GLOBAL_RIB_NAME, 0, []bgp.RouteFamily{bgp.RF_EVPN})
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

func pathTokey(path *table.Path) string {
	return fmt.Sprintf("%d:%s", path.GetNlri().PathIdentifier(), path.GetNlri().String())
}

func (s *BgpServer) AddPath(vrfId string, pathList []*table.Path) (uuidBytes []byte, err error) {
	err = s.mgmtOperation(func() error {
		if err := s.fixupApiPath(vrfId, pathList); err != nil {
			return err
		}
		if len(pathList) == 1 {
			path := pathList[0]
			id, _ := uuid.NewV4()
			s.uuidMap[id] = pathTokey(path)
			uuidBytes = id.Bytes()
		}
		s.propagateUpdate(nil, pathList)
		return nil
	}, true)
	return
}

func (s *BgpServer) DeletePath(uuidBytes []byte, f bgp.RouteFamily, vrfId string, pathList []*table.Path) error {
	return s.mgmtOperation(func() error {
		deletePathList := make([]*table.Path, 0)
		if len(uuidBytes) > 0 {
			// Delete locally generated path which has the given UUID
			path := func() *table.Path {
				id, _ := uuid.FromBytes(uuidBytes)
				if key, ok := s.uuidMap[id]; !ok {
					return nil
				} else {
					for _, path := range s.globalRib.GetPathList(table.GLOBAL_RIB_NAME, 0, s.globalRib.GetRFlist()) {
						if path.IsLocal() && key == pathTokey(path) {
							delete(s.uuidMap, id)
							return path
						}
					}
				}
				return nil
			}()
			if path == nil {
				return fmt.Errorf("Can't find a specified path")
			}
			deletePathList = append(deletePathList, path.Clone(true))
		} else if len(pathList) == 0 {
			// Delete all locally generated paths
			families := s.globalRib.GetRFlist()
			if f != 0 {
				families = []bgp.RouteFamily{f}
			}
			for _, path := range s.globalRib.GetPathList(table.GLOBAL_RIB_NAME, 0, families) {
				if path.IsLocal() {
					deletePathList = append(deletePathList, path.Clone(true))
				}
			}
			s.uuidMap = make(map[uuid.UUID]string)
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
		s.propagateUpdate(nil, pathList)
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
		if pathList, err := s.globalRib.AddVrf(name, id, rd, im, ex, pi); err != nil {
			return err
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

func familiesForSoftreset(peer *Peer, family bgp.RouteFamily) []bgp.RouteFamily {
	if family == bgp.RouteFamily(0) {
		configured := peer.configuredRFlist()
		families := make([]bgp.RouteFamily, 0, len(configured))
		for _, f := range configured {
			if f != bgp.RF_RTC_UC {
				families = append(families, f)
			}
		}
		return families
	}
	return []bgp.RouteFamily{family}
}

func (s *BgpServer) softResetIn(addr string, family bgp.RouteFamily) error {
	peers, err := s.addrToPeers(addr)
	if err != nil {
		return err
	}
	for _, peer := range peers {
		families := familiesForSoftreset(peer, family)

		pathList := make([]*table.Path, 0, peer.adjRibIn.Count(families))
		for _, path := range peer.adjRibIn.PathList(families, false) {
			// RFC4271 9.1.2 Phase 2: Route Selection
			//
			// If the AS_PATH attribute of a BGP route contains an AS loop, the BGP
			// route should be excluded from the Phase 2 decision function.
			isLooped := false
			if aspath := path.GetAsPath(); aspath != nil {
				isLooped = hasOwnASLoop(peer.fsm.peerInfo.LocalAS, int(peer.fsm.pConf.AsPathOptions.Config.AllowOwnAs), aspath)
			}
			if path.IsAsLooped() != isLooped {
				// can't modify the existing one. needs to create one
				path = path.Clone(false)
				path.SetAsLooped(isLooped)
				// update accepted counter
				peer.adjRibIn.Update([]*table.Path{path})
			}
			if !path.IsAsLooped() {
				pathList = append(pathList, path)
			}
		}
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
		families := familiesForSoftreset(peer, family)

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

		pathList, filtered := s.getBestFromLocal(peer, families)
		if len(pathList) > 0 {
			sendFsmOutgoingMsg(peer, pathList, nil, false)
		}
		if !deferral && len(filtered) > 0 {
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

func (s *BgpServer) validateTable(r *table.Table) (v []*table.Validation) {
	if s.roaManager.enabled() {
		v = make([]*table.Validation, 0, len(r.GetDestinations()))
		for _, d := range r.GetDestinations() {
			for _, p := range d.GetAllKnownPathList() {
				v = append(v, s.roaManager.validate(p))
			}
		}
	}
	return
}

func (s *BgpServer) GetRib(addr string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (rib *table.Table, v []*table.Validation, err error) {
	err = s.mgmtOperation(func() error {
		m := s.globalRib
		id := table.GLOBAL_RIB_NAME
		as := uint32(0)
		if len(addr) > 0 {
			peer, ok := s.neighborMap[addr]
			if !ok {
				return fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
			}
			if !peer.isRouteServerClient() {
				return fmt.Errorf("Neighbor %v doesn't have local rib", addr)
			}
			id = peer.ID()
			as = peer.AS()
			m = s.rsRib
		}
		af := bgp.RouteFamily(family)
		tbl, ok := m.Tables[af]
		if !ok {
			return fmt.Errorf("address family: %s not supported", af)
		}
		rib, err = tbl.Select(table.TableSelectOption{ID: id, AS: as, LookupPrefixes: prefixes})
		v = s.validateTable(rib)
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

func (s *BgpServer) GetAdjRib(addr string, family bgp.RouteFamily, in bool, prefixes []*table.LookupPrefix) (rib *table.Table, v []*table.Validation, err error) {
	err = s.mgmtOperation(func() error {
		peer, ok := s.neighborMap[addr]
		if !ok {
			return fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
		}
		id := peer.ID()
		as := peer.AS()

		var adjRib *table.AdjRib
		if in {
			adjRib = peer.adjRibIn
		} else {
			adjRib = table.NewAdjRib(peer.configuredRFlist())
			accepted, _ := s.getBestFromLocal(peer, peer.configuredRFlist())
			adjRib.Update(accepted)
		}
		rib, err = adjRib.Select(family, false, table.TableSelectOption{ID: id, AS: as, LookupPrefixes: prefixes})
		v = s.validateTable(rib)
		return err
	}, true)
	return
}

func (s *BgpServer) GetRibInfo(addr string, family bgp.RouteFamily) (info *table.TableInfo, err error) {
	err = s.mgmtOperation(func() error {
		m := s.globalRib
		id := table.GLOBAL_RIB_NAME
		as := uint32(0)
		if len(addr) > 0 {
			peer, ok := s.neighborMap[addr]
			if !ok {
				return fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
			}
			if !peer.isRouteServerClient() {
				return fmt.Errorf("Neighbor %v doesn't have local rib", addr)
			}
			id = peer.ID()
			as = peer.AS()
			m = s.rsRib
		}
		info, err = m.TableInfo(id, as, family)
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
			adjRib = table.NewAdjRib(peer.configuredRFlist())
			accepted, _ := s.getBestFromLocal(peer, peer.configuredRFlist())
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
			l = append(l, s.ToConfig(peer, getAdvertised))
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
	server.neighborMap[addr] = peer
	if name := c.Config.PeerGroup; name != "" {
		server.peerGroupMap[name].AddMember(*c)
	}
	peer.startFSMHandler(server.fsmincomingCh, server.fsmStateCh)
	server.broadcastPeerState(peer, bgp.BGP_FSM_IDLE, nil)
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

	if original.NeedsResendOpenMessage(c) {
		sub := uint8(bgp.BGP_ERROR_SUB_OTHER_CONFIGURATION_CHANGE)
		if original.Config.AdminDown != c.Config.AdminDown {
			sub = bgp.BGP_ERROR_SUB_ADMINISTRATIVE_SHUTDOWN
			state := "Admin Down"

			if !c.Config.AdminDown {
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
	Init         bool
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
	StateReason   *FsmStateReason
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
	Vrf           map[string]uint16
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
			as := uint32(0)
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
				as = peer.AS()
				rib = w.s.rsRib
			}

			pathList := func() map[string][]*table.Path {
				pathList := make(map[string][]*table.Path)
				for _, t := range rib.Tables {
					for _, dst := range t.GetSortedDestinations() {
						if paths := dst.GetKnownPathList(id, as); len(paths) > 0 {
							pathList[dst.GetNlri().String()] = clonePathList(paths)
						}
					}
				}
				return pathList
			}()
			l := make([]*config.Neighbor, 0, len(w.s.neighborMap))
			for _, peer := range w.s.neighborMap {
				l = append(l, w.s.ToConfig(peer, false))
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
	for ev := range w.ch.Out() {
		w.realCh <- ev.(WatchEvent)
	}
	close(w.realCh)
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
				w.notify(newWatchEventPeerState(peer, nil))
			}
		}
		if w.opts.initBest && s.active() == nil {
			w.notify(&WatchEventBestPath{
				PathList:      s.globalRib.GetBestPathList(table.GLOBAL_RIB_NAME, 0, nil),
				MultiPathList: s.globalRib.GetBestMultiPathList(table.GLOBAL_RIB_NAME, nil),
			})
		}
		if w.opts.initUpdate {
			for _, peer := range s.neighborMap {
				if peer.fsm.state != bgp.BGP_FSM_ESTABLISHED {
					continue
				}
				configNeighbor := w.s.ToConfig(peer, false)
				for _, rf := range peer.configuredRFlist() {
					_, y := peer.fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]
					l, _ := peer.fsm.LocalHostPort()
					w.notify(&WatchEventUpdate{
						PeerAS:       peer.fsm.peerInfo.AS,
						LocalAS:      peer.fsm.peerInfo.LocalAS,
						PeerAddress:  peer.fsm.peerInfo.Address,
						LocalAddress: net.ParseIP(l),
						PeerID:       peer.fsm.peerInfo.ID,
						FourBytesAs:  y,
						Init:         true,
						PostPolicy:   false,
						Neighbor:     configNeighbor,
						PathList:     peer.adjRibIn.PathList([]bgp.RouteFamily{rf}, false),
					})

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
						Init:         true,
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
				for _, path := range s.globalRib.GetPathList(table.GLOBAL_RIB_NAME, 0, []bgp.RouteFamily{rf}) {
					pathsByPeer[path.GetSource()] = append(pathsByPeer[path.GetSource()], path)
				}
				for peerInfo, paths := range pathsByPeer {
					// create copy which can be access to without mutex
					var configNeighbor *config.Neighbor
					if peer, ok := s.neighborMap[peerInfo.Address.String()]; ok {
						configNeighbor = w.s.ToConfig(peer, false)
					}

					w.notify(&WatchEventUpdate{
						PeerAS:      peerInfo.AS,
						PeerAddress: peerInfo.Address,
						PeerID:      peerInfo.ID,
						PostPolicy:  true,
						Neighbor:    configNeighbor,
						PathList:    paths,
						Init:        true,
					})

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
						Init:        true,
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
