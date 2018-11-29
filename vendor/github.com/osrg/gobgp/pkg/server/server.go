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
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/eapache/channels"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/internal/pkg/apiutil"
	"github.com/osrg/gobgp/internal/pkg/config"
	"github.com/osrg/gobgp/internal/pkg/table"
	"github.com/osrg/gobgp/internal/pkg/zebra"
	"github.com/osrg/gobgp/pkg/packet/bgp"
)

type tcpListener struct {
	l  *net.TCPListener
	ch chan struct{}
}

func (l *tcpListener) Close() error {
	if err := l.l.Close(); err != nil {
		return err
	}
	<-l.ch
	return nil
}

// avoid mapped IPv6 address
func newTCPListener(address string, port uint32, ch chan *net.TCPConn) (*tcpListener, error) {
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
	if err := setListenTCPTTLSockopt(l, 255); err != nil {
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
	return &tcpListener{
		l:  l,
		ch: closeCh,
	}, nil
}

type options struct {
	grpcAddress string
	grpcOption  []grpc.ServerOption
}

type ServerOption func(*options)

func GrpcListenAddress(addr string) ServerOption {
	return func(o *options) {
		o.grpcAddress = addr
	}
}

func GrpcOption(opt []grpc.ServerOption) ServerOption {
	return func(o *options) {
		o.grpcOption = opt
	}
}

type BgpServer struct {
	bgpConfig     config.Bgp
	fsmincomingCh *channels.InfiniteChannel
	fsmStateCh    chan *fsmMsg
	acceptCh      chan *net.TCPConn

	mgmtCh       chan *mgmtOp
	policy       *table.RoutingPolicy
	listeners    []*tcpListener
	neighborMap  map[string]*peer
	peerGroupMap map[string]*peerGroup
	globalRib    *table.TableManager
	rsRib        *table.TableManager
	roaManager   *roaManager
	shutdownWG   *sync.WaitGroup
	watcherMap   map[watchEventType][]*watcher
	zclient      *zebraClient
	bmpManager   *bmpClientManager
	mrtManager   *mrtManager
	uuidMap      map[uuid.UUID]string
}

func NewBgpServer(opt ...ServerOption) *BgpServer {
	opts := options{}
	for _, o := range opt {
		o(&opts)
	}

	roaManager, _ := newROAManager(0)
	s := &BgpServer{
		neighborMap:  make(map[string]*peer),
		peerGroupMap: make(map[string]*peerGroup),
		policy:       table.NewRoutingPolicy(),
		roaManager:   roaManager,
		mgmtCh:       make(chan *mgmtOp, 1),
		watcherMap:   make(map[watchEventType][]*watcher),
		uuidMap:      make(map[uuid.UUID]string),
	}
	s.bmpManager = newBmpClientManager(s)
	s.mrtManager = newMrtManager(s)
	if len(opts.grpcAddress) != 0 {
		grpc.EnableTracing = false
		api := newAPIserver(s, grpc.NewServer(opts.grpcOption...), opts.grpcAddress)
		go func() {
			if err := api.serve(); err != nil {
				log.Fatalf("failed to listen grpc port: %s", err)
			}
		}()

	}
	return s
}

func (s *BgpServer) listListeners(addr string) []*net.TCPListener {
	list := make([]*net.TCPListener, 0, len(s.listeners))
	rhs := net.ParseIP(addr).To4() != nil
	for _, l := range s.listeners {
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

func (s *BgpServer) handleMGMTOp(op *mgmtOp) {
	if op.checkActive {
		if err := s.active(); err != nil {
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

func (s *BgpServer) Serve() {
	s.listeners = make([]*tcpListener, 0, 2)
	s.fsmincomingCh = channels.NewInfiniteChannel()
	s.fsmStateCh = make(chan *fsmMsg, 4096)

	handlefsmMsg := func(e *fsmMsg) {
		peer, found := s.neighborMap[e.MsgSrc]
		if !found {
			log.WithFields(log.Fields{
				"Topic": "Peer",
			}).Warnf("Can't find the neighbor %s", e.MsgSrc)
			return
		}
		peer.fsm.lock.RLock()
		versionMismatch := e.Version != peer.fsm.version
		peer.fsm.lock.RUnlock()
		if versionMismatch {
			log.WithFields(log.Fields{
				"Topic": "Peer",
			}).Debug("FSM version inconsistent")
			return
		}
		s.handleFSMMessage(peer, e)
	}

	for {
		passConn := func(conn *net.TCPConn) {
			host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
			ipaddr, _ := net.ResolveIPAddr("ip", host)
			remoteAddr := ipaddr.String()
			peer, found := s.neighborMap[remoteAddr]
			if found {
				peer.fsm.lock.RLock()
				adminStateNotUp := peer.fsm.adminState != adminStateUp
				peer.fsm.lock.RUnlock()
				if adminStateNotUp {
					peer.fsm.lock.RLock()
					log.WithFields(log.Fields{
						"Topic":       "Peer",
						"Remote Addr": remoteAddr,
						"Admin State": peer.fsm.adminState,
					}).Debug("New connection for non admin-state-up peer")
					peer.fsm.lock.RUnlock()
					conn.Close()
					return
				}
				peer.fsm.lock.RLock()
				localAddr := peer.fsm.pConf.Transport.Config.LocalAddress
				peer.fsm.lock.RUnlock()
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
				}(localAddr)

				if !localAddrValid {
					conn.Close()
					return
				}

				log.WithFields(log.Fields{
					"Topic": "Peer",
				}).Debugf("Accepted a new passive connection from:%s", remoteAddr)
				peer.PassConn(conn)
			} else if pg := s.matchLongestDynamicNeighborPrefix(remoteAddr); pg != nil {
				log.WithFields(log.Fields{
					"Topic": "Peer",
				}).Debugf("Accepted a new dynamic neighbor from:%s", remoteAddr)
				rib := s.globalRib
				if pg.Conf.RouteServer.Config.RouteServerClient {
					rib = s.rsRib
				}
				peer := newDynamicPeer(&s.bgpConfig.Global, remoteAddr, pg.Conf, rib, s.policy)
				if peer == nil {
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   remoteAddr,
					}).Infof("Can't create new Dynamic Peer")
					conn.Close()
					return
				}
				peer.fsm.lock.RLock()
				policy := peer.fsm.pConf.ApplyPolicy
				peer.fsm.lock.RUnlock()
				s.policy.Reset(nil, map[string]config.ApplyPolicy{peer.ID(): policy})
				s.neighborMap[remoteAddr] = peer
				peer.startFSMHandler(s.fsmincomingCh, s.fsmStateCh)
				s.broadcastPeerState(peer, bgp.BGP_FSM_ACTIVE, nil)
				peer.PassConn(conn)
			} else {
				log.WithFields(log.Fields{
					"Topic": "Peer",
				}).Infof("Can't find configuration for a new passive connection from:%s", remoteAddr)
				conn.Close()
			}
		}

		select {
		case op := <-s.mgmtCh:
			s.handleMGMTOp(op)
		case conn := <-s.acceptCh:
			passConn(conn)
		default:
		}

		for {
			select {
			case e := <-s.fsmStateCh:
				handlefsmMsg(e)
			default:
				goto CONT
			}
		}
	CONT:

		select {
		case op := <-s.mgmtCh:
			s.handleMGMTOp(op)
		case rmsg := <-s.roaManager.ReceiveROA():
			s.roaManager.HandleROAEvent(rmsg)
		case conn := <-s.acceptCh:
			passConn(conn)
		case e, ok := <-s.fsmincomingCh.Out():
			if !ok {
				continue
			}
			handlefsmMsg(e.(*fsmMsg))
		case e := <-s.fsmStateCh:
			handlefsmMsg(e)
		}
	}
}

func (s *BgpServer) matchLongestDynamicNeighborPrefix(a string) *peerGroup {
	ipAddr := net.ParseIP(a)
	longestMask := net.CIDRMask(0, 32).String()
	var longestPG *peerGroup
	for _, pg := range s.peerGroupMap {
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

func sendfsmOutgoingMsg(peer *peer, paths []*table.Path, notification *bgp.BGPMessage, stayIdle bool) {
	peer.outgoing.In() <- &fsmOutgoingMsg{
		Paths:        paths,
		Notification: notification,
		StayIdle:     stayIdle,
	}
}

func isASLoop(peer *peer, path *table.Path) bool {
	for _, as := range path.GetAsList() {
		if as == peer.AS() {
			return true
		}
	}
	return false
}

func filterpath(peer *peer, path, old *table.Path) *table.Path {
	if path == nil {
		return nil
	}

	peer.fsm.lock.RLock()
	_, ok := peer.fsm.rfMap[path.GetRouteFamily()]
	peer.fsm.lock.RUnlock()
	if !ok {
		return nil
	}

	//RFC4684 Constrained Route Distribution
	peer.fsm.lock.RLock()
	_, y := peer.fsm.rfMap[bgp.RF_RTC_UC]
	peer.fsm.lock.RUnlock()
	if y && path.GetRouteFamily() != bgp.RF_RTC_UC {
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
			if info.RouteReflectorClient {
				ignore = false
			}
			if peer.isRouteReflectorClient() {
				// RFC4456 8. Avoiding Routing Information Loops
				// If the local CLUSTER_ID is found in the CLUSTER_LIST,
				// the advertisement received SHOULD be ignored.
				for _, clusterID := range path.GetClusterList() {
					peer.fsm.lock.RLock()
					rrClusterID := peer.fsm.peerInfo.RouteReflectorClusterID
					peer.fsm.lock.RUnlock()
					if clusterID.Equal(rrClusterID) {
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

func (s *BgpServer) filterpath(peer *peer, path, old *table.Path) *table.Path {
	// Special handling for RTM NLRI.
	if path != nil && path.GetRouteFamily() == bgp.RF_RTC_UC && !path.IsWithdraw {
		// If the given "path" is locally generated and the same with "old", we
		// assumes "path" was already sent before. This assumption avoids the
		// infinite UPDATE loop between Route Reflector and its clients.
		if path.IsLocal() && path.Equal(old) {
			peer.fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   peer.fsm.pConf.State.NeighborAddress,
				"Path":  path,
			}).Debug("given rtm nlri is already sent, skipping to advertise")
			peer.fsm.lock.RUnlock()
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
	peer.fsm.lock.RLock()
	peerVrf := peer.fsm.pConf.Config.Vrf
	peer.fsm.lock.RUnlock()
	if path != nil && peerVrf != "" {
		if f := path.GetRouteFamily(); f != bgp.RF_IPv4_VPN && f != bgp.RF_IPv6_VPN {
			return nil
		}
		vrf := peer.localRib.Vrfs[peerVrf]
		if table.CanImportToVrf(vrf, path) {
			path = path.ToLocal()
		} else {
			return nil
		}
	}

	// replace-peer-as handling
	peer.fsm.lock.RLock()
	if path != nil && !path.IsWithdraw && peer.fsm.pConf.AsPathOptions.State.ReplacePeerAs {
		path = path.ReplaceAS(peer.fsm.pConf.Config.LocalAs, peer.fsm.pConf.Config.PeerAs)
	}
	peer.fsm.lock.RUnlock()

	if path = filterpath(peer, path, old); path == nil {
		return nil
	}

	peer.fsm.lock.RLock()
	options := &table.PolicyOptions{
		Info:       peer.fsm.peerInfo,
		OldNextHop: path.GetNexthop(),
	}
	path = table.UpdatePathAttrs(peer.fsm.gConf, peer.fsm.pConf, peer.fsm.peerInfo, path)

	if v := s.roaManager.validate(path); v != nil {
		options.ValidationResult = v
	}
	peer.fsm.lock.RUnlock()

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

func (s *BgpServer) notifyBestWatcher(best []*table.Path, multipath [][]*table.Path) {
	if table.SelectionOptions.DisableBestPathSelection {
		// Note: If best path selection disabled, no best path to notify.
		return
	}
	clonedM := make([][]*table.Path, len(multipath))
	for i, pathList := range multipath {
		clonedM[i] = clonePathList(pathList)
	}
	clonedB := clonePathList(best)
	m := make(map[string]uint32)
	for _, p := range clonedB {
		switch p.GetRouteFamily() {
		case bgp.RF_IPv4_VPN, bgp.RF_IPv6_VPN:
			for _, vrf := range s.globalRib.Vrfs {
				if vrf.Id != 0 && table.CanImportToVrf(vrf, p) {
					m[p.GetNlri().String()] = uint32(vrf.Id)
				}
			}
		}
	}
	w := &watchEventBestPath{PathList: clonedB, MultiPathList: clonedM}
	if len(m) > 0 {
		w.Vrf = m
	}
	s.notifyWatcher(watchEventTypeBestPath, w)
}

func (s *BgpServer) toConfig(peer *peer, getAdvertised bool) *config.Neighbor {
	// create copy which can be access to without mutex
	peer.fsm.lock.RLock()
	conf := *peer.fsm.pConf
	peerAfiSafis := peer.fsm.pConf.AfiSafis
	peerCapMap := peer.fsm.capMap
	peer.fsm.lock.RUnlock()

	conf.AfiSafis = make([]config.AfiSafi, len(peerAfiSafis))
	for i, af := range peerAfiSafis {
		conf.AfiSafis[i] = af
		conf.AfiSafis[i].AddPaths.State.Receive = peer.isAddPathReceiveEnabled(af.State.Family)
		if peer.isAddPathSendEnabled(af.State.Family) {
			conf.AfiSafis[i].AddPaths.State.SendMax = af.AddPaths.State.SendMax
		} else {
			conf.AfiSafis[i].AddPaths.State.SendMax = 0
		}
	}

	remoteCap := make([]bgp.ParameterCapabilityInterface, 0, len(peerCapMap))
	for _, caps := range peerCapMap {
		for _, m := range caps {
			// need to copy all values here
			buf, _ := m.Serialize()
			c, _ := bgp.DecodeCapability(buf)
			remoteCap = append(remoteCap, c)
		}
	}

	conf.State.RemoteCapabilityList = remoteCap

	peer.fsm.lock.RLock()
	conf.State.LocalCapabilityList = capabilitiesFromConfig(peer.fsm.pConf)
	conf.State.SessionState = config.IntToSessionStateMap[int(peer.fsm.state)]
	conf.State.AdminState = config.IntToAdminStateMap[int(peer.fsm.adminState)]
	state := peer.fsm.state
	peer.fsm.lock.RUnlock()

	if state == bgp.BGP_FSM_ESTABLISHED {
		peer.fsm.lock.RLock()
		conf.Transport.State.LocalAddress, conf.Transport.State.LocalPort = peer.fsm.LocalHostPort()
		_, conf.Transport.State.RemotePort = peer.fsm.RemoteHostPort()
		buf, _ := peer.fsm.recvOpen.Serialize()
		// need to copy all values here
		conf.State.ReceivedOpenMessage, _ = bgp.ParseBGPMessage(buf)
		conf.State.RemoteRouterId = peer.fsm.peerInfo.ID.To4().String()
		peer.fsm.lock.RUnlock()
	}
	return &conf
}

func (s *BgpServer) notifyPrePolicyUpdateWatcher(peer *peer, pathList []*table.Path, msg *bgp.BGPMessage, timestamp time.Time, payload []byte) {
	if !s.isWatched(watchEventTypePreUpdate) || peer == nil {
		return
	}

	cloned := clonePathList(pathList)
	if len(cloned) == 0 {
		return
	}
	n := s.toConfig(peer, false)
	peer.fsm.lock.RLock()
	_, y := peer.fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]
	l, _ := peer.fsm.LocalHostPort()
	ev := &watchEventUpdate{
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
		Neighbor:     n,
	}
	peer.fsm.lock.RUnlock()
	s.notifyWatcher(watchEventTypePreUpdate, ev)
}

func (s *BgpServer) notifyPostPolicyUpdateWatcher(peer *peer, pathList []*table.Path) {
	if !s.isWatched(watchEventTypePostUpdate) || peer == nil {
		return
	}

	cloned := clonePathList(pathList)
	if len(cloned) == 0 {
		return
	}
	n := s.toConfig(peer, false)
	peer.fsm.lock.RLock()
	_, y := peer.fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]
	l, _ := peer.fsm.LocalHostPort()
	ev := &watchEventUpdate{
		PeerAS:       peer.fsm.peerInfo.AS,
		LocalAS:      peer.fsm.peerInfo.LocalAS,
		PeerAddress:  peer.fsm.peerInfo.Address,
		LocalAddress: net.ParseIP(l),
		PeerID:       peer.fsm.peerInfo.ID,
		FourBytesAs:  y,
		Timestamp:    cloned[0].GetTimestamp(),
		PostPolicy:   true,
		PathList:     cloned,
		Neighbor:     n,
	}
	peer.fsm.lock.RUnlock()
	s.notifyWatcher(watchEventTypePostUpdate, ev)
}

func newWatchEventPeerState(peer *peer, m *fsmMsg) *watchEventPeerState {
	_, rport := peer.fsm.RemoteHostPort()
	laddr, lport := peer.fsm.LocalHostPort()
	sentOpen := buildopen(peer.fsm.gConf, peer.fsm.pConf)
	peer.fsm.lock.RLock()
	recvOpen := peer.fsm.recvOpen
	e := &watchEventPeerState{
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
	peer.fsm.lock.RUnlock()

	if m != nil {
		e.StateReason = m.StateReason
	}
	return e
}

func (s *BgpServer) broadcastPeerState(peer *peer, oldState bgp.FSMState, e *fsmMsg) {
	peer.fsm.lock.RLock()
	newState := peer.fsm.state
	peer.fsm.lock.RUnlock()
	if oldState == bgp.BGP_FSM_ESTABLISHED || newState == bgp.BGP_FSM_ESTABLISHED {
		s.notifyWatcher(watchEventTypePeerState, newWatchEventPeerState(peer, e))
	}
}

func (s *BgpServer) notifyMessageWatcher(peer *peer, timestamp time.Time, msg *bgp.BGPMessage, isSent bool) {
	// validation should be done in the caller of this function
	peer.fsm.lock.RLock()
	_, y := peer.fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]
	l, _ := peer.fsm.LocalHostPort()
	ev := &watchEventMessage{
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
	peer.fsm.lock.RUnlock()
	if !isSent {
		s.notifyWatcher(watchEventTypeRecvMsg, ev)
	}
}

func (s *BgpServer) notifyRecvMessageWatcher(peer *peer, timestamp time.Time, msg *bgp.BGPMessage) {
	if peer == nil || !s.isWatched(watchEventTypeRecvMsg) {
		return
	}
	s.notifyMessageWatcher(peer, timestamp, msg, false)
}

func (s *BgpServer) getBestFromLocal(peer *peer, rfList []bgp.RouteFamily) ([]*table.Path, []*table.Path) {
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

func (s *BgpServer) processOutgoingPaths(peer *peer, paths, olds []*table.Path) []*table.Path {
	peer.fsm.lock.RLock()
	notEstablished := peer.fsm.state != bgp.BGP_FSM_ESTABLISHED
	localRestarting := peer.fsm.pConf.GracefulRestart.State.LocalRestarting
	peer.fsm.lock.RUnlock()
	if notEstablished {
		return nil
	}
	if localRestarting {
		peer.fsm.lock.RLock()
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   peer.fsm.pConf.State.NeighborAddress,
		}).Debug("now syncing, suppress sending updates")
		peer.fsm.lock.RUnlock()
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

func (s *BgpServer) handleRouteRefresh(peer *peer, e *fsmMsg) []*table.Path {
	m := e.MsgData.(*bgp.BGPMessage)
	rr := m.Body.(*bgp.BGPRouteRefresh)
	rf := bgp.AfiSafiToRouteFamily(rr.AFI, rr.SAFI)

	peer.fsm.lock.RLock()
	_, ok := peer.fsm.rfMap[rf]
	peer.fsm.lock.RUnlock()
	if !ok {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   peer.ID(),
			"Data":  rf,
		}).Warn("Route family isn't supported")
		return nil
	}

	peer.fsm.lock.RLock()
	_, ok = peer.fsm.capMap[bgp.BGP_CAP_ROUTE_REFRESH]
	peer.fsm.lock.RUnlock()
	if !ok {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   peer.ID(),
		}).Warn("ROUTE_REFRESH received but the capability wasn't advertised")
		return nil
	}
	rfList := []bgp.RouteFamily{rf}
	accepted, _ := s.getBestFromLocal(peer, rfList)
	return accepted
}

func (s *BgpServer) propagateUpdate(peer *peer, pathList []*table.Path) {
	rs := peer != nil && peer.isRouteServerClient()
	vrf := false
	if peer != nil {
		peer.fsm.lock.RLock()
		vrf = !rs && peer.fsm.pConf.Config.Vrf != ""
		peer.fsm.lock.RUnlock()
	}

	tableId := table.GLOBAL_RIB_NAME
	rib := s.globalRib
	if rs {
		tableId = peer.TableID()
		rib = s.rsRib
	}

	for _, path := range pathList {
		if vrf {
			peer.fsm.lock.RLock()
			peerVrf := peer.fsm.pConf.Config.Vrf
			peer.fsm.lock.RUnlock()
			path = path.ToGlobal(rib.Vrfs[peerVrf])
		}

		policyOptions := &table.PolicyOptions{}

		if !rs && peer != nil {
			peer.fsm.lock.RLock()
			policyOptions.Info = peer.fsm.peerInfo
			peer.fsm.lock.RUnlock()
		}
		if v := s.roaManager.validate(path); v != nil {
			policyOptions.ValidationResult = v
		}

		if p := s.policy.ApplyPolicy(tableId, table.POLICY_DIRECTION_IMPORT, path, policyOptions); p != nil {
			path = p
		} else {
			path = path.Clone(true)
		}

		if !rs {
			s.notifyPostPolicyUpdateWatcher(peer, []*table.Path{path})

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
					_, candidates = s.getBestFromLocal(peer, fs)
				} else {
					// https://github.com/osrg/gobgp/issues/1777
					// Ignore duplicate Membership announcements
					membershipsForSource := s.globalRib.GetPathListWithSource(table.GLOBAL_RIB_NAME, []bgp.RouteFamily{bgp.RF_RTC_UC}, path.GetSource())
					found := false
					for _, membership := range membershipsForSource {
						if membership.GetNlri().(*bgp.RouteTargetMembershipNLRI).RouteTarget.String() == rt.String() {
							found = true
							break
						}
					}
					if !found {
						candidates = s.globalRib.GetBestPathList(peer.TableID(), 0, fs)
					}
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
					paths = s.processOutgoingPaths(peer, paths, nil)
				}
				sendfsmOutgoingMsg(peer, paths, nil, false)
			}
		}

		if dsts := rib.Update(path); len(dsts) > 0 {
			s.propagateUpdateToNeighbors(peer, path, dsts, true)
		}
	}
}

func (s *BgpServer) dropPeerAllRoutes(peer *peer, families []bgp.RouteFamily) {
	peer.fsm.lock.RLock()
	peerInfo := peer.fsm.peerInfo
	peer.fsm.lock.RUnlock()

	rib := s.globalRib
	if peer.isRouteServerClient() {
		rib = s.rsRib
	}
	for _, family := range peer.toGlobalFamilies(families) {
		for _, path := range rib.GetPathListByPeer(peerInfo, family) {
			p := path.Clone(true)
			if dsts := rib.Update(p); len(dsts) > 0 {
				s.propagateUpdateToNeighbors(peer, p, dsts, false)
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

func (s *BgpServer) propagateUpdateToNeighbors(source *peer, newPath *table.Path, dsts []*table.Update, needOld bool) {
	if table.SelectionOptions.DisableBestPathSelection {
		return
	}
	var gBestList, gOldList, bestList, oldList []*table.Path
	var mpathList [][]*table.Path
	if source == nil || !source.isRouteServerClient() {
		gBestList, gOldList, mpathList = dstsToPaths(table.GLOBAL_RIB_NAME, 0, dsts)
		s.notifyBestWatcher(gBestList, mpathList)
	}
	family := newPath.GetRouteFamily()
	for _, targetPeer := range s.neighborMap {
		if (source == nil && targetPeer.isRouteServerClient()) || (source != nil && source.isRouteServerClient() != targetPeer.isRouteServerClient()) {
			continue
		}
		f := func() bgp.RouteFamily {
			targetPeer.fsm.lock.RLock()
			peerVrf := targetPeer.fsm.pConf.Config.Vrf
			targetPeer.fsm.lock.RUnlock()
			if peerVrf != "" {
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
		if paths := s.processOutgoingPaths(targetPeer, bestList, oldList); len(paths) > 0 {
			sendfsmOutgoingMsg(targetPeer, paths, nil, false)
		}
	}
}

func (s *BgpServer) handleFSMMessage(peer *peer, e *fsmMsg) {
	switch e.MsgType {
	case fsmMsgStateChange:
		nextState := e.MsgData.(bgp.FSMState)
		peer.fsm.lock.Lock()
		oldState := bgp.FSMState(peer.fsm.pConf.State.SessionState.ToInt())
		peer.fsm.pConf.State.SessionState = config.IntToSessionStateMap[int(nextState)]
		peer.fsm.lock.Unlock()

		peer.fsm.StateChange(nextState)

		peer.fsm.lock.RLock()
		nextStateIdle := peer.fsm.pConf.GracefulRestart.State.PeerRestarting && nextState == bgp.BGP_FSM_IDLE
		peer.fsm.lock.RUnlock()

		// PeerDown
		if oldState == bgp.BGP_FSM_ESTABLISHED {
			t := time.Now()
			peer.fsm.lock.Lock()
			if t.Sub(time.Unix(peer.fsm.pConf.Timers.State.Uptime, 0)) < flopThreshold {
				peer.fsm.pConf.State.Flops++
			}
			graceful := peer.fsm.reason.Type == fsmGracefulRestart
			peer.fsm.lock.Unlock()
			var drop []bgp.RouteFamily
			if graceful {
				peer.fsm.lock.Lock()
				peer.fsm.pConf.GracefulRestart.State.PeerRestarting = true
				peer.fsm.lock.Unlock()
				var p []bgp.RouteFamily
				p, drop = peer.forwardingPreservedFamilies()
				s.propagateUpdate(peer, peer.StaleAll(p))
			} else {
				drop = peer.configuredRFlist()
			}
			peer.prefixLimitWarned = make(map[bgp.RouteFamily]bool)
			peer.DropAll(drop)
			s.dropPeerAllRoutes(peer, drop)

			peer.fsm.lock.Lock()
			if peer.fsm.pConf.Config.PeerAs == 0 {
				peer.fsm.pConf.State.PeerAs = 0
				peer.fsm.peerInfo.AS = 0
			}
			peer.fsm.lock.Unlock()

			if peer.isDynamicNeighbor() {
				peer.stopPeerRestarting()
				go peer.stopFSM()
				peer.fsm.lock.RLock()
				delete(s.neighborMap, peer.fsm.pConf.State.NeighborAddress)
				peer.fsm.lock.RUnlock()
				s.broadcastPeerState(peer, oldState, e)
				return
			}
		} else if nextStateIdle {
			peer.fsm.lock.RLock()
			longLivedEnabled := peer.fsm.pConf.GracefulRestart.State.LongLivedEnabled
			peer.fsm.lock.RUnlock()
			if longLivedEnabled {
				llgr, no_llgr := peer.llgrFamilies()

				peer.DropAll(no_llgr)
				s.dropPeerAllRoutes(peer, no_llgr)

				// attach LLGR_STALE community to paths in peer's adj-rib-in
				// paths with NO_LLGR are deleted
				pathList := peer.markLLGRStale(llgr)

				// calculate again
				// wheh path with LLGR_STALE chosen as best,
				// peer which doesn't support LLGR will drop the path
				// if it is in adj-rib-out, do withdrawal
				s.propagateUpdate(peer, pathList)

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
							s.mgmtOperation(func() error {
								log.WithFields(log.Fields{
									"Topic":  "Peer",
									"Key":    peer.ID(),
									"Family": family,
								}).Debugf("LLGR restart timer (%d sec) for %s expired", t, family)
								peer.DropAll([]bgp.RouteFamily{family})
								s.dropPeerAllRoutes(peer, []bgp.RouteFamily{family})

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
				peer.fsm.lock.Lock()
				peer.fsm.pConf.GracefulRestart.State.PeerRestarting = false
				peer.fsm.lock.Unlock()
				peer.DropAll(peer.configuredRFlist())
				s.dropPeerAllRoutes(peer, peer.configuredRFlist())
			}
		}

		cleanInfiniteChannel(peer.outgoing)
		peer.outgoing = channels.NewInfiniteChannel()
		if nextState == bgp.BGP_FSM_ESTABLISHED {
			// update for export policy
			laddr, _ := peer.fsm.LocalHostPort()
			// may include zone info
			peer.fsm.lock.Lock()
			peer.fsm.pConf.Transport.State.LocalAddress = laddr
			// exclude zone info
			ipaddr, _ := net.ResolveIPAddr("ip", laddr)
			peer.fsm.peerInfo.LocalAddress = ipaddr.IP
			neighborAddress := peer.fsm.pConf.State.NeighborAddress
			peer.fsm.lock.Unlock()
			deferralExpiredFunc := func(family bgp.RouteFamily) func() {
				return func() {
					s.mgmtOperation(func() error {
						s.softResetOut(neighborAddress, family, true)
						return nil
					}, false)
				}
			}
			peer.fsm.lock.RLock()
			notLocalRestarting := !peer.fsm.pConf.GracefulRestart.State.LocalRestarting
			peer.fsm.lock.RUnlock()
			if notLocalRestarting {
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
				peer.fsm.lock.RLock()
				_, y := peer.fsm.rfMap[bgp.RF_RTC_UC]
				c := peer.fsm.pConf.GetAfiSafi(bgp.RF_RTC_UC)
				notPeerRestarting := !peer.fsm.pConf.GracefulRestart.State.PeerRestarting
				peer.fsm.lock.RUnlock()
				if y && notPeerRestarting && c.RouteTargetMembership.Config.DeferralTime > 0 {
					pathList, _ = s.getBestFromLocal(peer, []bgp.RouteFamily{bgp.RF_RTC_UC})
					t := c.RouteTargetMembership.Config.DeferralTime
					for _, f := range peer.negotiatedRFList() {
						if f != bgp.RF_RTC_UC {
							time.AfterFunc(time.Second*time.Duration(t), deferralExpiredFunc(f))
						}
					}
				} else {
					pathList, _ = s.getBestFromLocal(peer, peer.negotiatedRFList())
				}

				if len(pathList) > 0 {
					sendfsmOutgoingMsg(peer, pathList, nil, false)
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
					for _, p := range s.neighborMap {
						if !p.recvedAllEOR() {
							return false
						}
					}
					return true
				}()
				if allEnd {
					for _, p := range s.neighborMap {
						p.fsm.lock.Lock()
						p.fsm.pConf.GracefulRestart.State.LocalRestarting = false
						p.fsm.lock.Unlock()
						if !p.isGracefulRestartEnabled() {
							continue
						}
						paths, _ := s.getBestFromLocal(p, p.configuredRFlist())
						if len(paths) > 0 {
							sendfsmOutgoingMsg(p, paths, nil, false)
						}
					}
					log.WithFields(log.Fields{
						"Topic": "Server",
					}).Info("sync finished")
				} else {
					peer.fsm.lock.RLock()
					deferral := peer.fsm.pConf.GracefulRestart.Config.DeferralTime
					peer.fsm.lock.RUnlock()
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   peer.ID(),
					}).Debugf("Now syncing, suppress sending updates. start deferral timer(%d)", deferral)
					time.AfterFunc(time.Second*time.Duration(deferral), deferralExpiredFunc(bgp.RouteFamily(0)))
				}
			}
		} else {
			if s.shutdownWG != nil && nextState == bgp.BGP_FSM_IDLE {
				die := true
				for _, p := range s.neighborMap {
					p.fsm.lock.RLock()
					stateNotIdle := p.fsm.state != bgp.BGP_FSM_IDLE
					p.fsm.lock.RUnlock()
					if stateNotIdle {
						die = false
						break
					}
				}
				if die {
					s.shutdownWG.Done()
				}
			}
			peer.fsm.lock.Lock()
			peer.fsm.pConf.Timers.State.Downtime = time.Now().Unix()
			peer.fsm.lock.Unlock()
		}
		// clear counter
		peer.fsm.lock.RLock()
		adminStateDown := peer.fsm.adminState == adminStateDown
		peer.fsm.lock.RUnlock()
		if adminStateDown {
			peer.fsm.lock.Lock()
			peer.fsm.pConf.State = config.NeighborState{}
			peer.fsm.pConf.State.NeighborAddress = peer.fsm.pConf.Config.NeighborAddress
			peer.fsm.pConf.State.PeerAs = peer.fsm.pConf.Config.PeerAs
			peer.fsm.pConf.Timers.State = config.TimersState{}
			peer.fsm.lock.Unlock()
		}
		peer.startFSMHandler(s.fsmincomingCh, s.fsmStateCh)
		s.broadcastPeerState(peer, oldState, e)
	case fsmMsgRouteRefresh:
		peer.fsm.lock.RLock()
		notEstablished := peer.fsm.state != bgp.BGP_FSM_ESTABLISHED
		beforeUptime := e.timestamp.Unix() < peer.fsm.pConf.Timers.State.Uptime
		peer.fsm.lock.RUnlock()
		if notEstablished || beforeUptime {
			return
		}
		if paths := s.handleRouteRefresh(peer, e); len(paths) > 0 {
			sendfsmOutgoingMsg(peer, paths, nil, false)
			return
		}
	case fsmMsgBGPMessage:
		switch m := e.MsgData.(type) {
		case *bgp.MessageError:
			sendfsmOutgoingMsg(peer, nil, bgp.NewBGPNotificationMessage(m.TypeCode, m.SubTypeCode, m.Data), false)
			return
		case *bgp.BGPMessage:
			s.notifyRecvMessageWatcher(peer, e.timestamp, m)
			peer.fsm.lock.RLock()
			notEstablished := peer.fsm.state != bgp.BGP_FSM_ESTABLISHED
			beforeUptime := e.timestamp.Unix() < peer.fsm.pConf.Timers.State.Uptime
			peer.fsm.lock.RUnlock()
			if notEstablished || beforeUptime {
				return
			}
			pathList, eor, notification := peer.handleUpdate(e)
			if notification != nil {
				sendfsmOutgoingMsg(peer, nil, notification, true)
				return
			}
			if m.Header.Type == bgp.BGP_MSG_UPDATE {
				s.notifyPrePolicyUpdateWatcher(peer, pathList, m, e.timestamp, e.payload)
			}

			if len(pathList) > 0 {
				s.propagateUpdate(peer, pathList)
			}

			peer.fsm.lock.RLock()
			peerAfiSafis := peer.fsm.pConf.AfiSafis
			peer.fsm.lock.RUnlock()
			if len(eor) > 0 {
				rtc := false
				for _, f := range eor {
					if f == bgp.RF_RTC_UC {
						rtc = true
					}
					for i, a := range peerAfiSafis {
						if a.State.Family == f {
							peer.fsm.lock.Lock()
							peer.fsm.pConf.AfiSafis[i].MpGracefulRestart.State.EndOfRibReceived = true
							peer.fsm.lock.Unlock()
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

				peer.fsm.lock.RLock()
				localRestarting := peer.fsm.pConf.GracefulRestart.State.LocalRestarting
				peer.fsm.lock.RUnlock()
				if localRestarting {
					allEnd := func() bool {
						for _, p := range s.neighborMap {
							if !p.recvedAllEOR() {
								return false
							}
						}
						return true
					}()
					if allEnd {
						for _, p := range s.neighborMap {
							p.fsm.lock.Lock()
							p.fsm.pConf.GracefulRestart.State.LocalRestarting = false
							p.fsm.lock.Unlock()
							if !p.isGracefulRestartEnabled() {
								continue
							}
							paths, _ := s.getBestFromLocal(p, p.negotiatedRFList())
							if len(paths) > 0 {
								sendfsmOutgoingMsg(p, paths, nil, false)
							}
						}
						log.WithFields(log.Fields{
							"Topic": "Server",
						}).Info("sync finished")

					}

					// we don't delay non-route-target NLRIs when local-restarting
					rtc = false
				}
				peer.fsm.lock.RLock()
				peerRestarting := peer.fsm.pConf.GracefulRestart.State.PeerRestarting
				peer.fsm.lock.RUnlock()
				if peerRestarting {
					if peer.recvedAllEOR() {
						peer.stopPeerRestarting()
						pathList := peer.adjRibIn.DropStale(peer.configuredRFlist())
						peer.fsm.lock.RLock()
						log.WithFields(log.Fields{
							"Topic": "Peer",
							"Key":   peer.fsm.pConf.State.NeighborAddress,
						}).Debugf("withdraw %d stale routes", len(pathList))
						peer.fsm.lock.RUnlock()
						s.propagateUpdate(peer, pathList)
					}

					// we don't delay non-route-target NLRIs when peer is restarting
					rtc = false
				}

				// received EOR of route-target address family
				// outbound filter is now ready, let's flash non-route-target NLRIs
				peer.fsm.lock.RLock()
				c := peer.fsm.pConf.GetAfiSafi(bgp.RF_RTC_UC)
				peer.fsm.lock.RUnlock()
				if rtc && c != nil && c.RouteTargetMembership.Config.DeferralTime > 0 {
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
					if paths, _ := s.getBestFromLocal(peer, families); len(paths) > 0 {
						sendfsmOutgoingMsg(peer, paths, nil, false)
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

func (s *BgpServer) EnableZebra(ctx context.Context, r *api.EnableZebraRequest) error {
	return s.mgmtOperation(func() error {
		if s.zclient != nil {
			return fmt.Errorf("already connected to Zebra")
		}

		for _, p := range r.RouteTypes {
			if _, err := zebra.RouteTypeFromString(p, uint8(r.Version)); err != nil {
				return err
			}
		}

		protos := make([]string, 0, len(r.RouteTypes))
		for _, p := range r.RouteTypes {
			protos = append(protos, string(p))
		}
		var err error
		s.zclient, err = newZebraClient(s, r.Url, protos, uint8(r.Version), r.NexthopTriggerEnable, uint8(r.NexthopTriggerDelay))
		return err
	}, false)
}

func (s *BgpServer) AddBmp(ctx context.Context, r *api.AddBmpRequest) error {
	return s.mgmtOperation(func() error {
		_, ok := api.AddBmpRequest_MonitoringPolicy_name[int32(r.Type)]
		if !ok {
			return fmt.Errorf("invalid bmp route monitoring policy: %v", r.Type)
		}
		return s.bmpManager.addServer(&config.BmpServerConfig{
			Address: r.Address,
			Port:    r.Port,
			RouteMonitoringPolicy: config.IntToBmpRouteMonitoringPolicyTypeMap[int(r.Type)],
			StatisticsTimeout:     uint16(r.StatisticsTimeout),
		})
	}, true)
}

func (s *BgpServer) DeleteBmp(ctx context.Context, r *api.DeleteBmpRequest) error {
	return s.mgmtOperation(func() error {
		return s.bmpManager.deleteServer(&config.BmpServerConfig{
			Address: r.Address,
			Port:    r.Port,
		})
	}, true)
}

func (s *BgpServer) StopBgp(ctx context.Context, r *api.StopBgpRequest) error {
	s.mgmtOperation(func() error {
		s.shutdownWG = new(sync.WaitGroup)
		s.shutdownWG.Add(1)

		for k := range s.neighborMap {
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
	}, false)

	// Waits for all goroutines per peer to stop.
	// Note: This should not be wrapped with s.mgmtOperation() in order to
	// avoid the deadlock in the main goroutine of BgpServer.
	// if s.shutdownWG != nil {
	// 	s.shutdownWG.Wait()
	// 	s.shutdownWG = nil
	// }
	return nil
}

func (s *BgpServer) SetPolicies(ctx context.Context, r *api.SetPoliciesRequest) error {
	rp, err := newRoutingPolicyFromApiStruct(r)
	if err != nil {
		return err
	}

	getConfig := func(id string) (*config.ApplyPolicy, error) {
		f := func(id string, dir table.PolicyDirection) (config.DefaultPolicyType, []string, error) {
			rt, policies, err := s.policy.GetPolicyAssignment(id, dir)
			if err != nil {
				return config.DEFAULT_POLICY_TYPE_REJECT_ROUTE, nil, err
			}
			names := make([]string, 0, len(policies))
			for _, p := range policies {
				names = append(names, p.Name)
			}
			t := config.DEFAULT_POLICY_TYPE_ACCEPT_ROUTE
			if rt == table.ROUTE_TYPE_REJECT {
				t = config.DEFAULT_POLICY_TYPE_REJECT_ROUTE
			}
			return t, names, nil
		}

		c := &config.ApplyPolicy{}
		rt, policies, err := f(id, table.POLICY_DIRECTION_IMPORT)
		if err != nil {
			return nil, err
		}
		c.Config.ImportPolicyList = policies
		c.Config.DefaultImportPolicy = rt
		rt, policies, err = f(id, table.POLICY_DIRECTION_EXPORT)
		if err != nil {
			return nil, err
		}
		c.Config.ExportPolicyList = policies
		c.Config.DefaultExportPolicy = rt
		return c, nil
	}

	return s.mgmtOperation(func() error {
		ap := make(map[string]config.ApplyPolicy, len(s.neighborMap)+1)
		a, err := getConfig(table.GLOBAL_RIB_NAME)
		if err != nil {
			return err
		}
		ap[table.GLOBAL_RIB_NAME] = *a
		for _, peer := range s.neighborMap {
			peer.fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   peer.fsm.pConf.State.NeighborAddress,
			}).Info("call set policy")
			peer.fsm.lock.RUnlock()
			a, err := getConfig(peer.ID())
			if err != nil {
				return err
			}
			ap[peer.ID()] = *a
		}
		return s.policy.Reset(rp, ap)
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

func (s *BgpServer) fixupApiPath(vrfId string, pathList []*table.Path) error {
	pi := &table.PeerInfo{
		AS:      s.bgpConfig.Global.Config.As,
		LocalID: net.ParseIP(s.bgpConfig.Global.Config.RouterId).To4(),
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
			vrf := s.globalRib.Vrfs[vrfId]
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
				paths := s.globalRib.GetBestPathList(table.GLOBAL_RIB_NAME, 0, []bgp.RouteFamily{bgp.RF_EVPN})
				if m := getMacMobilityExtendedCommunity(r.ETag, r.MacAddress, paths); m != nil {
					pm := getMacMobilityExtendedCommunity(r.ETag, r.MacAddress, []*table.Path{path})
					if pm == nil {
						path.SetExtCommunities([]bgp.ExtendedCommunityInterface{m}, false)
					} else if pm != nil && pm.Sequence < m.Sequence {
						return fmt.Errorf("Invalid MAC mobility sequence number")
					}
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

func (s *BgpServer) addPathList(vrfId string, pathList []*table.Path) error {
	err := s.fixupApiPath(vrfId, pathList)
	if err == nil {
		s.propagateUpdate(nil, pathList)
	}
	return err
}

func (s *BgpServer) AddPath(ctx context.Context, r *api.AddPathRequest) (*api.AddPathResponse, error) {
	var uuidBytes []byte
	err := s.mgmtOperation(func() error {
		pathList, err := api2PathList(r.Resource, []*api.Path{r.Path})
		if err != nil {
			return err
		}
		err = s.addPathList(r.VrfId, pathList)
		if err != nil {
			return err
		}
		path := pathList[0]
		id, _ := uuid.NewV4()
		s.uuidMap[id] = pathTokey(path)
		uuidBytes = id.Bytes()
		return nil
	}, true)
	return &api.AddPathResponse{Uuid: uuidBytes}, err
}

func (s *BgpServer) DeletePath(ctx context.Context, r *api.DeletePathRequest) error {
	return s.mgmtOperation(func() error {
		deletePathList := make([]*table.Path, 0)

		pathList, err := func() ([]*table.Path, error) {
			if r.Path != nil {
				r.Path.IsWithdraw = true
				return api2PathList(r.Resource, []*api.Path{r.Path})
			}
			return []*table.Path{}, nil
		}()
		if err != nil {
			return err
		}

		if len(r.Uuid) > 0 {
			// Delete locally generated path which has the given UUID
			path := func() *table.Path {
				id, _ := uuid.FromBytes(r.Uuid)
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
			if r.Family != nil {
				families = []bgp.RouteFamily{bgp.AfiSafiToRouteFamily(uint16(r.Family.Afi), uint8(r.Family.Safi))}

			}
			for _, path := range s.globalRib.GetPathList(table.GLOBAL_RIB_NAME, 0, families) {
				if path.IsLocal() {
					deletePathList = append(deletePathList, path.Clone(true))
				}
			}
			s.uuidMap = make(map[uuid.UUID]string)
		} else {
			if err := s.fixupApiPath(r.VrfId, pathList); err != nil {
				return err
			}
			deletePathList = pathList
		}
		s.propagateUpdate(nil, deletePathList)
		return nil
	}, true)
}

func (s *BgpServer) updatePath(vrfId string, pathList []*table.Path) error {
	err := s.mgmtOperation(func() error {
		if err := s.fixupApiPath(vrfId, pathList); err != nil {
			return err
		}
		s.propagateUpdate(nil, pathList)
		return nil
	}, true)
	return err
}

func (s *BgpServer) StartBgp(ctx context.Context, r *api.StartBgpRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Global == nil {
			return fmt.Errorf("invalid request")
		}
		g := r.Global
		if net.ParseIP(g.RouterId) == nil {
			return fmt.Errorf("invalid router-id format: %s", g.RouterId)
		}

		c := newGlobalFromAPIStruct(g)
		if err := config.SetDefaultGlobalConfigValues(c); err != nil {
			return err
		}

		if c.Config.Port > 0 {
			acceptCh := make(chan *net.TCPConn, 4096)
			for _, addr := range c.Config.LocalAddressList {
				l, err := newTCPListener(addr, uint32(c.Config.Port), acceptCh)
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

// TODO: delete this function
func (s *BgpServer) listVrf() (l []*table.Vrf) {
	s.mgmtOperation(func() error {
		l = make([]*table.Vrf, 0, len(s.globalRib.Vrfs))
		for _, vrf := range s.globalRib.Vrfs {
			l = append(l, vrf.Clone())
		}
		return nil
	}, true)
	return l
}

func (s *BgpServer) ListVrf(ctx context.Context, _ *api.ListVrfRequest, fn func(*api.Vrf)) error {
	toApi := func(v *table.Vrf) *api.Vrf {
		return &api.Vrf{
			Name:     v.Name,
			Rd:       apiutil.MarshalRD(v.Rd),
			Id:       v.Id,
			ImportRt: apiutil.MarshalRTs(v.ImportRt),
			ExportRt: apiutil.MarshalRTs(v.ExportRt),
		}
	}
	var l []*api.Vrf
	s.mgmtOperation(func() error {
		l = make([]*api.Vrf, 0, len(s.globalRib.Vrfs))
		for _, vrf := range s.globalRib.Vrfs {
			l = append(l, toApi(vrf.Clone()))
		}
		return nil
	}, true)
	for _, v := range l {
		select {
		case <-ctx.Done():
			return nil
		default:
			fn(v)
		}
	}
	return nil
}

func (s *BgpServer) AddVrf(ctx context.Context, r *api.AddVrfRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Vrf == nil {
			return fmt.Errorf("invalid request")
		}

		name := r.Vrf.Name
		id := r.Vrf.Id

		rd, err := apiutil.UnmarshalRD(r.Vrf.Rd)
		if err != nil {
			return err
		}
		im, err := apiutil.UnmarshalRTs(r.Vrf.ImportRt)
		if err != nil {
			return err
		}
		ex, err := apiutil.UnmarshalRTs(r.Vrf.ExportRt)
		if err != nil {
			return err
		}

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

func (s *BgpServer) DeleteVrf(ctx context.Context, r *api.DeleteVrfRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Name == "" {
			return fmt.Errorf("invalid request")
		}
		name := r.Name
		for _, n := range s.neighborMap {
			n.fsm.lock.RLock()
			peerVrf := n.fsm.pConf.Config.Vrf
			n.fsm.lock.RUnlock()
			if peerVrf == name {
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

func familiesForSoftreset(peer *peer, family bgp.RouteFamily) []bgp.RouteFamily {
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
				peer.fsm.lock.RLock()
				localAS := peer.fsm.peerInfo.LocalAS
				allowOwnAS := int(peer.fsm.pConf.AsPathOptions.Config.AllowOwnAs)
				peer.fsm.lock.RUnlock()
				isLooped = hasOwnASLoop(localAS, allowOwnAS, aspath)
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
		peer.fsm.lock.RLock()
		notEstablished := peer.fsm.state != bgp.BGP_FSM_ESTABLISHED
		peer.fsm.lock.RUnlock()
		if notEstablished {
			continue
		}
		families := familiesForSoftreset(peer, family)

		if deferral {
			if family == bgp.RouteFamily(0) {
				families = peer.configuredRFlist()
			}
			peer.fsm.lock.RLock()
			_, y := peer.fsm.rfMap[bgp.RF_RTC_UC]
			c := peer.fsm.pConf.GetAfiSafi(bgp.RF_RTC_UC)
			restarting := peer.fsm.pConf.GracefulRestart.State.LocalRestarting
			peer.fsm.lock.RUnlock()
			if restarting {
				peer.fsm.lock.Lock()
				peer.fsm.pConf.GracefulRestart.State.LocalRestarting = false
				peer.fsm.lock.Unlock()
				log.WithFields(log.Fields{
					"Topic":    "Peer",
					"Key":      peer.ID(),
					"Families": families,
				}).Debug("deferral timer expired")
			} else if y && !c.MpGracefulRestart.State.EndOfRibReceived {
				log.WithFields(log.Fields{
					"Topic":    "Peer",
					"Key":      peer.ID(),
					"Families": families,
				}).Debug("route-target deferral timer expired")
			} else {
				continue
			}
		}

		pathList, _ := s.getBestFromLocal(peer, families)
		if len(pathList) > 0 {
			if deferral {
				pathList = func() []*table.Path {
					l := make([]*table.Path, 0, len(pathList))
					for _, p := range pathList {
						if !p.IsWithdraw {
							l = append(l, p)
						}
					}
					return l
				}()
			}
			sendfsmOutgoingMsg(peer, pathList, nil, false)
		}
	}
	return nil
}

func (s *BgpServer) sResetIn(addr string, family bgp.RouteFamily) error {
	log.WithFields(log.Fields{
		"Topic": "Operation",
		"Key":   addr,
	}).Info("Neighbor soft reset in")
	return s.softResetIn(addr, family)
}

func (s *BgpServer) sResetOut(addr string, family bgp.RouteFamily) error {
	log.WithFields(log.Fields{
		"Topic": "Operation",
		"Key":   addr,
	}).Info("Neighbor soft reset out")
	return s.softResetOut(addr, family, false)
}

func (s *BgpServer) sReset(addr string, family bgp.RouteFamily) error {
	log.WithFields(log.Fields{
		"Topic": "Operation",
		"Key":   addr,
	}).Info("Neighbor soft reset")
	err := s.softResetIn(addr, family)
	if err != nil {
		return err
	}
	return s.softResetOut(addr, family, false)
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

func (s *BgpServer) getRib(addr string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (rib *table.Table, v []*table.Validation, err error) {
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

func (s *BgpServer) getVrfRib(name string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (rib *table.Table, err error) {
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

func (s *BgpServer) getAdjRib(addr string, family bgp.RouteFamily, in bool, prefixes []*table.LookupPrefix) (rib *table.Table, v []*table.Validation, err error) {
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

func (s *BgpServer) ListPath(ctx context.Context, r *api.ListPathRequest, fn func(*api.Destination)) error {
	var tbl *table.Table
	var v []*table.Validation

	f := func() []*table.LookupPrefix {
		l := make([]*table.LookupPrefix, 0, len(r.Prefixes))
		for _, p := range r.Prefixes {
			l = append(l, &table.LookupPrefix{
				Prefix:       p.Prefix,
				LookupOption: table.LookupOption(p.LookupOption),
			})
		}
		return l
	}

	in := false
	family := bgp.RouteFamily(0)
	if r.Family != nil {
		family = bgp.AfiSafiToRouteFamily(uint16(r.Family.Afi), uint8(r.Family.Safi))
	}
	var err error
	switch r.Type {
	case api.Resource_LOCAL, api.Resource_GLOBAL:
		tbl, v, err = s.getRib(r.Name, family, f())
	case api.Resource_ADJ_IN:
		in = true
		fallthrough
	case api.Resource_ADJ_OUT:
		tbl, v, err = s.getAdjRib(r.Name, family, in, f())
	case api.Resource_VRF:
		tbl, err = s.getVrfRib(r.Name, family, []*table.LookupPrefix{})
	default:
		return fmt.Errorf("unsupported resource type: %v", r.Type)
	}

	if err != nil {
		return err
	}

	idx := 0
	err = func() error {
		for _, dst := range tbl.GetDestinations() {
			d := api.Destination{
				Prefix: dst.GetNlri().String(),
				Paths:  make([]*api.Path, 0, len(dst.GetAllKnownPathList())),
			}
			for i, path := range dst.GetAllKnownPathList() {
				p := toPathApi(path, getValidation(v, idx))
				idx++
				if i == 0 && !table.SelectionOptions.DisableBestPathSelection {
					switch r.Type {
					case api.Resource_LOCAL, api.Resource_GLOBAL:
						p.Best = true
					}
				}
				d.Paths = append(d.Paths, p)
			}
			select {
			case <-ctx.Done():
				return nil
			default:
				fn(&d)
			}
		}
		return nil
	}()
	return err
}

func (s *BgpServer) getRibInfo(addr string, family bgp.RouteFamily) (info *table.TableInfo, err error) {
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

func (s *BgpServer) getAdjRibInfo(addr string, family bgp.RouteFamily, in bool) (info *table.TableInfo, err error) {
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

func (s *BgpServer) GetTable(ctx context.Context, r *api.GetTableRequest) (*api.GetTableResponse, error) {
	if r == nil || r.Name == "" {
		return nil, fmt.Errorf("invalid request")
	}
	family := bgp.RouteFamily(0)
	if r.Family != nil {
		family = bgp.AfiSafiToRouteFamily(uint16(r.Family.Afi), uint8(r.Family.Safi))
	}
	var in bool
	var err error
	var info *table.TableInfo
	switch r.Type {
	case api.Resource_GLOBAL, api.Resource_LOCAL:
		info, err = s.getRibInfo(r.Name, family)
	case api.Resource_ADJ_IN:
		in = true
		fallthrough
	case api.Resource_ADJ_OUT:
		info, err = s.getAdjRibInfo(r.Name, family, in)
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", r.Type)
	}

	if err != nil {
		return nil, err
	}

	return &api.GetTableResponse{
		NumDestination: uint64(info.NumDestination),
		NumPath:        uint64(info.NumPath),
		NumAccepted:    uint64(info.NumAccepted),
	}, nil
}

func (s *BgpServer) GetBgp(ctx context.Context, r *api.GetBgpRequest) (*api.GetBgpResponse, error) {
	var rsp *api.GetBgpResponse
	s.mgmtOperation(func() error {
		g := s.bgpConfig.Global
		rsp = &api.GetBgpResponse{
			Global: &api.Global{
				As:               g.Config.As,
				RouterId:         g.Config.RouterId,
				ListenPort:       g.Config.Port,
				ListenAddresses:  g.Config.LocalAddressList,
				UseMultiplePaths: g.UseMultiplePaths.Config.Enabled,
			},
		}
		return nil
	}, false)
	return rsp, nil
}

func (s *BgpServer) getNeighbor(address string, getAdvertised bool) []*config.Neighbor {
	var l []*config.Neighbor
	s.mgmtOperation(func() error {
		l = make([]*config.Neighbor, 0, len(s.neighborMap))
		for k, peer := range s.neighborMap {
			peer.fsm.lock.RLock()
			neighborIface := peer.fsm.pConf.Config.NeighborInterface
			peer.fsm.lock.RUnlock()
			if address != "" && address != k && address != neighborIface {
				continue
			}
			// FIXME: should remove toConfig() conversion
			l = append(l, s.toConfig(peer, getAdvertised))
		}
		return nil
	}, false)
	return l
}

func (s *BgpServer) ListPeer(ctx context.Context, r *api.ListPeerRequest, fn func(*api.Peer)) error {
	var l []*api.Peer
	s.mgmtOperation(func() error {
		address := r.Address
		getAdvertised := r.EnableAdvertised
		l = make([]*api.Peer, 0, len(s.neighborMap))
		for k, peer := range s.neighborMap {
			peer.fsm.lock.RLock()
			neighborIface := peer.fsm.pConf.Config.NeighborInterface
			peer.fsm.lock.RUnlock()
			if address != "" && address != k && address != neighborIface {
				continue
			}
			// FIXME: should remove toConfig() conversion
			p := config.NewPeerFromConfigStruct(s.toConfig(peer, getAdvertised))
			for _, family := range peer.configuredRFlist() {
				for i, afisafi := range p.AfiSafis {
					if afisafi.Config.Enabled != true {
						continue
					}
					afi, safi := bgp.RouteFamilyToAfiSafi(family)
					c := afisafi.Config
					if c.Family != nil && c.Family.Afi == api.Family_Afi(afi) && c.Family.Safi == api.Family_Safi(safi) {
						flist := []bgp.RouteFamily{family}
						received := uint32(peer.adjRibIn.Count(flist))
						accepted := uint32(peer.adjRibIn.Accepted(flist))
						advertised := uint32(0)
						if getAdvertised == true {
							pathList, _ := s.getBestFromLocal(peer, flist)
							advertised = uint32(len(pathList))
						}
						p.AfiSafis[i].State = &api.AfiSafiState{
							Family:     c.Family,
							Enabled:    true,
							Received:   received,
							Accepted:   accepted,
							Advertised: advertised,
						}
					}
				}
			}
			l = append(l, p)
		}
		return nil
	}, false)
	for _, p := range l {
		select {
		case <-ctx.Done():
			return nil
		default:
			fn(p)
		}
	}
	return nil
}

func (s *BgpServer) addPeerGroup(c *config.PeerGroup) error {
	name := c.Config.PeerGroupName
	if _, y := s.peerGroupMap[name]; y {
		return fmt.Errorf("Can't overwrite the existing peer-group: %s", name)
	}

	log.WithFields(log.Fields{
		"Topic": "Peer",
		"Name":  name,
	}).Info("Add a peer group configuration")

	s.peerGroupMap[c.Config.PeerGroupName] = newPeerGroup(c)

	return nil
}

func (s *BgpServer) addNeighbor(c *config.Neighbor) error {
	addr, err := c.ExtractNeighborAddress()
	if err != nil {
		return err
	}

	if _, y := s.neighborMap[addr]; y {
		return fmt.Errorf("Can't overwrite the existing peer: %s", addr)
	}

	var pgConf *config.PeerGroup
	if c.Config.PeerGroup != "" {
		pg, ok := s.peerGroupMap[c.Config.PeerGroup]
		if !ok {
			return fmt.Errorf("no such peer-group: %s", c.Config.PeerGroup)
		}
		pgConf = pg.Conf
	}

	if err := config.SetDefaultNeighborConfigValues(c, pgConf, &s.bgpConfig.Global); err != nil {
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
		_, y := s.globalRib.Vrfs[vrf]
		if !y {
			return fmt.Errorf("VRF not found: %s", vrf)
		}
	}

	if c.RouteServer.Config.RouteServerClient && c.RouteReflector.Config.RouteReflectorClient {
		return fmt.Errorf("can't be both route-server-client and route-reflector-client")
	}

	if s.bgpConfig.Global.Config.Port > 0 {
		for _, l := range s.listListeners(addr) {
			if c.Config.AuthPassword != "" {
				if err := setTCPMD5SigSockopt(l, addr, c.Config.AuthPassword); err != nil {
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

	rib := s.globalRib
	if c.RouteServer.Config.RouteServerClient {
		rib = s.rsRib
	}
	peer := newPeer(&s.bgpConfig.Global, c, rib, s.policy)
	s.policy.Reset(nil, map[string]config.ApplyPolicy{peer.ID(): c.ApplyPolicy})
	s.neighborMap[addr] = peer
	if name := c.Config.PeerGroup; name != "" {
		s.peerGroupMap[name].AddMember(*c)
	}
	peer.startFSMHandler(s.fsmincomingCh, s.fsmStateCh)
	s.broadcastPeerState(peer, bgp.BGP_FSM_IDLE, nil)
	return nil
}

func (s *BgpServer) AddPeerGroup(ctx context.Context, r *api.AddPeerGroupRequest) error {
	return s.mgmtOperation(func() error {
		c, err := newPeerGroupFromAPIStruct(r.PeerGroup)
		if err != nil {
			return err
		}
		return s.addPeerGroup(c)
	}, true)
}

func (s *BgpServer) AddPeer(ctx context.Context, r *api.AddPeerRequest) error {
	return s.mgmtOperation(func() error {
		c, err := newNeighborFromAPIStruct(r.Peer)
		if err != nil {
			return err
		}
		return s.addNeighbor(c)
	}, true)
}

func (s *BgpServer) AddDynamicNeighbor(ctx context.Context, r *api.AddDynamicNeighborRequest) error {
	return s.mgmtOperation(func() error {
		c := &config.DynamicNeighbor{Config: config.DynamicNeighborConfig{
			Prefix:    r.DynamicNeighbor.Prefix,
			PeerGroup: r.DynamicNeighbor.PeerGroup},
		}
		s.peerGroupMap[c.Config.PeerGroup].AddDynamicNeighbor(c)
		return nil
	}, true)
}

func (s *BgpServer) deletePeerGroup(name string) error {
	if _, y := s.peerGroupMap[name]; !y {
		return fmt.Errorf("Can't delete a peer-group %s which does not exist", name)
	}

	log.WithFields(log.Fields{
		"Topic": "Peer",
		"Name":  name,
	}).Info("Delete a peer group configuration")

	delete(s.peerGroupMap, name)
	return nil
}

func (s *BgpServer) deleteNeighbor(c *config.Neighbor, code, subcode uint8) error {
	if c.Config.PeerGroup != "" {
		_, y := s.peerGroupMap[c.Config.PeerGroup]
		if y {
			s.peerGroupMap[c.Config.PeerGroup].DeleteMember(*c)
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
	n, y := s.neighborMap[addr]
	if !y {
		return fmt.Errorf("Can't delete a peer configuration for %s", addr)
	}
	for _, l := range s.listListeners(addr) {
		if err := setTCPMD5SigSockopt(l, addr, ""); err != nil {
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
	delete(s.neighborMap, addr)
	s.dropPeerAllRoutes(n, n.configuredRFlist())
	return nil
}

func (s *BgpServer) DeletePeerGroup(ctx context.Context, r *api.DeletePeerGroupRequest) error {
	return s.mgmtOperation(func() error {
		name := r.Name
		for _, n := range s.neighborMap {
			n.fsm.lock.RLock()
			peerGroup := n.fsm.pConf.Config.PeerGroup
			n.fsm.lock.RUnlock()
			if peerGroup == name {
				return fmt.Errorf("failed to delete peer-group %s: neighbor %s is in use", name, n.ID())
			}
		}
		return s.deletePeerGroup(name)
	}, true)
}

func (s *BgpServer) DeletePeer(ctx context.Context, r *api.DeletePeerRequest) error {
	return s.mgmtOperation(func() error {
		c := &config.Neighbor{Config: config.NeighborConfig{
			NeighborAddress:   r.Address,
			NeighborInterface: r.Interface,
		}}
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

func (s *BgpServer) UpdatePeerGroup(ctx context.Context, r *api.UpdatePeerGroupRequest) (rsp *api.UpdatePeerGroupResponse, err error) {
	doSoftreset := false
	err = s.mgmtOperation(func() error {
		pg, err := newPeerGroupFromAPIStruct(r.PeerGroup)
		if err != nil {
			return err
		}
		doSoftreset, err = s.updatePeerGroup(pg)
		return err
	}, true)
	return &api.UpdatePeerGroupResponse{NeedsSoftResetIn: doSoftreset}, err
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

func (s *BgpServer) UpdatePeer(ctx context.Context, r *api.UpdatePeerRequest) (rsp *api.UpdatePeerResponse, err error) {
	doSoftReset := false
	err = s.mgmtOperation(func() error {
		c, err := newNeighborFromAPIStruct(r.Peer)
		if err != nil {
			return err
		}
		doSoftReset, err = s.updateNeighbor(c)
		return err
	}, true)
	return &api.UpdatePeerResponse{NeedsSoftResetIn: doSoftReset}, err
}

func (s *BgpServer) addrToPeers(addr string) (l []*peer, err error) {
	if len(addr) == 0 {
		for _, p := range s.neighborMap {
			l = append(l, p)
		}
		return l, nil
	}
	p, found := s.neighborMap[addr]
	if !found {
		return l, fmt.Errorf("Neighbor that has %v doesn't exist.", addr)
	}
	return []*peer{p}, nil
}

func (s *BgpServer) sendNotification(op, addr string, subcode uint8, data []byte) error {
	log.WithFields(log.Fields{
		"Topic": "Operation",
		"Key":   addr,
	}).Info(op)

	peers, err := s.addrToPeers(addr)
	if err == nil {
		m := bgp.NewBGPNotificationMessage(bgp.BGP_ERROR_CEASE, subcode, data)
		for _, peer := range peers {
			sendfsmOutgoingMsg(peer, nil, m, false)
		}
	}
	return err
}

func (s *BgpServer) ShutdownPeer(ctx context.Context, r *api.ShutdownPeerRequest) error {
	return s.mgmtOperation(func() error {
		return s.sendNotification("Neighbor shutdown", r.Address, bgp.BGP_ERROR_SUB_ADMINISTRATIVE_SHUTDOWN, newAdministrativeCommunication(r.Communication))
	}, true)
}

func (s *BgpServer) ResetPeer(ctx context.Context, r *api.ResetPeerRequest) error {
	return s.mgmtOperation(func() error {
		addr := r.Address
		comm := r.Communication
		if r.Soft {
			var err error
			if addr == "all" {
				addr = ""
			}
			family := bgp.RouteFamily(0)
			switch r.Direction {
			case api.ResetPeerRequest_IN:
				err = s.sResetIn(addr, family)
			case api.ResetPeerRequest_OUT:
				err = s.sResetOut(addr, family)
			case api.ResetPeerRequest_BOTH:
				err = s.sReset(addr, family)
			default:
				err = fmt.Errorf("unknown direction")
			}
			return err
		}

		err := s.sendNotification("Neighbor reset", addr, bgp.BGP_ERROR_SUB_ADMINISTRATIVE_RESET, newAdministrativeCommunication(comm))
		if err != nil {
			return err
		}
		peers, _ := s.addrToPeers(addr)
		for _, peer := range peers {
			peer.fsm.lock.Lock()
			peer.fsm.idleHoldTime = peer.fsm.pConf.Timers.Config.IdleHoldTimeAfterReset
			peer.fsm.lock.Unlock()
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
		f := func(stateOp *adminStateOperation, message string) {
			select {
			case peer.fsm.adminStateCh <- *stateOp:
				peer.fsm.lock.RLock()
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   peer.fsm.pConf.State.NeighborAddress,
				}).Debug(message)
				peer.fsm.lock.RUnlock()
			default:
				peer.fsm.lock.RLock()
				log.Warning("previous request is still remaining. : ", peer.fsm.pConf.State.NeighborAddress)
				peer.fsm.lock.RUnlock()
			}
		}
		if enable {
			f(&adminStateOperation{adminStateUp, nil}, "adminStateUp requested")
		} else {
			f(&adminStateOperation{adminStateDown, newAdministrativeCommunication(communication)}, "adminStateDown requested")
		}
	}
	return nil
}

func (s *BgpServer) EnablePeer(ctx context.Context, r *api.EnablePeerRequest) error {
	return s.mgmtOperation(func() error {
		return s.setAdminState(r.Address, "", true)
	}, true)
}

func (s *BgpServer) DisablePeer(ctx context.Context, r *api.DisablePeerRequest) error {
	return s.mgmtOperation(func() error {
		return s.setAdminState(r.Address, r.Communication, false)
	}, true)
}

func (s *BgpServer) ListDefinedSet(ctx context.Context, r *api.ListDefinedSetRequest, fn func(*api.DefinedSet)) error {
	var cd *config.DefinedSets
	var err error
	err = s.mgmtOperation(func() error {
		cd, err = s.policy.GetDefinedSet(table.DefinedType(r.Type), r.Name)
		return err
	}, false)

	if err != nil {
		return err
	}
	exec := func(d *api.DefinedSet) bool {
		select {
		case <-ctx.Done():
			return true
		default:
			fn(d)
		}
		return false
	}

	for _, cs := range cd.PrefixSets {
		ad := &api.DefinedSet{
			Type: api.DefinedType_PREFIX,
			Name: cs.PrefixSetName,
			Prefixes: func() []*api.Prefix {
				l := make([]*api.Prefix, 0, len(cs.PrefixList))
				for _, p := range cs.PrefixList {
					elems := _regexpPrefixMaskLengthRange.FindStringSubmatch(p.MasklengthRange)
					min, _ := strconv.ParseUint(elems[1], 10, 32)
					max, _ := strconv.ParseUint(elems[2], 10, 32)

					l = append(l, &api.Prefix{IpPrefix: p.IpPrefix, MaskLengthMin: uint32(min), MaskLengthMax: uint32(max)})
				}
				return l
			}(),
		}
		if exec(ad) {
			return nil
		}
	}
	for _, cs := range cd.NeighborSets {
		ad := &api.DefinedSet{
			Type: api.DefinedType_NEIGHBOR,
			Name: cs.NeighborSetName,
			List: cs.NeighborInfoList,
		}
		if exec(ad) {
			return nil
		}
	}
	for _, cs := range cd.BgpDefinedSets.CommunitySets {
		ad := &api.DefinedSet{
			Type: api.DefinedType_COMMUNITY,
			Name: cs.CommunitySetName,
			List: cs.CommunityList,
		}
		if exec(ad) {
			return nil
		}
	}
	for _, cs := range cd.BgpDefinedSets.ExtCommunitySets {
		ad := &api.DefinedSet{
			Type: api.DefinedType_EXT_COMMUNITY,
			Name: cs.ExtCommunitySetName,
			List: cs.ExtCommunityList,
		}
		if exec(ad) {
			return nil
		}
	}
	for _, cs := range cd.BgpDefinedSets.LargeCommunitySets {
		ad := &api.DefinedSet{
			Type: api.DefinedType_LARGE_COMMUNITY,
			Name: cs.LargeCommunitySetName,
			List: cs.LargeCommunityList,
		}
		if exec(ad) {
			return nil
		}
	}
	for _, cs := range cd.BgpDefinedSets.AsPathSets {
		ad := &api.DefinedSet{
			Type: api.DefinedType_AS_PATH,
			Name: cs.AsPathSetName,
			List: cs.AsPathList,
		}
		if exec(ad) {
			return nil
		}
	}
	return nil
}

func (s *BgpServer) AddDefinedSet(ctx context.Context, r *api.AddDefinedSetRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.DefinedSet == nil {
			return fmt.Errorf("invalid request")
		}
		set, err := newDefinedSetFromApiStruct(r.DefinedSet)
		if err != nil {
			return err
		}
		return s.policy.AddDefinedSet(set)
	}, false)
}

func (s *BgpServer) DeleteDefinedSet(ctx context.Context, r *api.DeleteDefinedSetRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.DefinedSet == nil {
			return fmt.Errorf("invalid request")
		}
		set, err := newDefinedSetFromApiStruct(r.DefinedSet)
		if err != nil {
			return err
		}
		return s.policy.DeleteDefinedSet(set, r.All)
	}, false)
}

func (s *BgpServer) ListStatement(ctx context.Context, r *api.ListStatementRequest, fn func(*api.Statement)) error {
	var l []*api.Statement
	s.mgmtOperation(func() error {
		s := s.policy.GetStatement(r.Name)
		l = make([]*api.Statement, len(s))
		for _, st := range s {
			l = append(l, toStatementApi(st))
		}
		return nil
	}, false)
	for _, s := range l {
		select {
		case <-ctx.Done():
			return nil
		default:
			fn(s)
		}
	}
	return nil
}

func (s *BgpServer) AddStatement(ctx context.Context, r *api.AddStatementRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Statement == nil {
			return fmt.Errorf("invalid request")
		}
		st, err := newStatementFromApiStruct(r.Statement)
		if err != nil {
			return err
		}
		return s.policy.AddStatement(st)
	}, false)
}

func (s *BgpServer) DeleteStatement(ctx context.Context, r *api.DeleteStatementRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Statement == nil {
			return fmt.Errorf("invalid request")
		}
		st, err := newStatementFromApiStruct(r.Statement)
		if err == nil {
			err = s.policy.DeleteStatement(st, r.All)
		}
		return err
	}, false)
}

func (s *BgpServer) ListPolicy(ctx context.Context, r *api.ListPolicyRequest, fn func(*api.Policy)) error {
	var l []*api.Policy
	s.mgmtOperation(func() error {
		pl := s.policy.GetPolicy(r.Name)
		l = make([]*api.Policy, 0, len(pl))
		for _, p := range pl {
			l = append(l, table.ToPolicyApi(p))
		}
		return nil
	}, false)
	for _, p := range l {
		select {
		case <-ctx.Done():
			return nil
		default:
			fn(p)
		}
	}
	return nil
}

func (s *BgpServer) AddPolicy(ctx context.Context, r *api.AddPolicyRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Policy == nil {
			return fmt.Errorf("invalid request")
		}
		p, err := newPolicyFromApiStruct(r.Policy)
		if err == nil {
			err = s.policy.AddPolicy(p, r.ReferExistingStatements)
		}
		return err
	}, false)
}

func (s *BgpServer) DeletePolicy(ctx context.Context, r *api.DeletePolicyRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Policy == nil {
			return fmt.Errorf("invalid request")
		}
		p, err := newPolicyFromApiStruct(r.Policy)
		if err != nil {
			return err
		}

		l := make([]string, 0, len(s.neighborMap)+1)
		for _, peer := range s.neighborMap {
			l = append(l, peer.ID())
		}
		l = append(l, table.GLOBAL_RIB_NAME)

		return s.policy.DeletePolicy(p, r.All, r.PreserveStatements, l)
	}, false)
}

func (s *BgpServer) toPolicyInfo(name string, dir api.PolicyDirection) (string, table.PolicyDirection, error) {
	if name == "" {
		return "", table.POLICY_DIRECTION_NONE, fmt.Errorf("empty table name")
	}

	if name == table.GLOBAL_RIB_NAME {
		name = table.GLOBAL_RIB_NAME
	} else {
		peer, ok := s.neighborMap[name]
		if !ok {
			return "", table.POLICY_DIRECTION_NONE, fmt.Errorf("not found peer %s", name)
		}
		if !peer.isRouteServerClient() {
			return "", table.POLICY_DIRECTION_NONE, fmt.Errorf("non-rs-client peer %s doesn't have per peer policy", name)
		}
		name = peer.ID()
	}
	switch dir {
	case api.PolicyDirection_IMPORT:
		return name, table.POLICY_DIRECTION_IMPORT, nil
	case api.PolicyDirection_EXPORT:
		return name, table.POLICY_DIRECTION_EXPORT, nil
	}
	return "", table.POLICY_DIRECTION_NONE, fmt.Errorf("invalid policy type")
}

func (s *BgpServer) ListPolicyAssignment(ctx context.Context, r *api.ListPolicyAssignmentRequest, fn func(*api.PolicyAssignment)) error {
	var a []*api.PolicyAssignment
	err := s.mgmtOperation(func() error {
		if r == nil {
			return fmt.Errorf("invalid request")
		}

		names := make([]string, 0, len(s.neighborMap)+1)
		if r.Name == "" {
			names = append(names, table.GLOBAL_RIB_NAME)
			for name := range s.neighborMap {
				names = append(names, name)
			}
		} else {
			names = append(names, r.Name)
		}
		dirs := make([]api.PolicyDirection, 0, 2)
		if r.Direction == api.PolicyDirection_UNKNOWN {
			dirs = []api.PolicyDirection{api.PolicyDirection_EXPORT, api.PolicyDirection_IMPORT}
		} else {
			dirs = append(dirs, r.Direction)
		}

		a = make([]*api.PolicyAssignment, 0, len(names))
		for _, name := range names {
			for _, dir := range dirs {
				id, dir, err := s.toPolicyInfo(name, dir)
				if err != nil {
					return err
				}
				rt, policies, err := s.policy.GetPolicyAssignment(id, dir)
				if err != nil {
					return err
				}
				if len(policies) == 0 {
					continue
				}
				t := &table.PolicyAssignment{
					Name:     name,
					Type:     dir,
					Default:  rt,
					Policies: policies,
				}
				a = append(a, table.NewAPIPolicyAssignmentFromTableStruct(t))
			}
		}
		return nil
	}, false)
	if err == nil {
		for _, p := range a {
			select {
			case <-ctx.Done():
				return nil
			default:
				fn(p)
			}
		}
	}
	return err
}

func (s *BgpServer) AddPolicyAssignment(ctx context.Context, r *api.AddPolicyAssignmentRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Assignment == nil {
			return fmt.Errorf("invalid request")
		}
		id, dir, err := s.toPolicyInfo(r.Assignment.Name, r.Assignment.Direction)
		if err != nil {
			return err
		}
		return s.policy.AddPolicyAssignment(id, dir, toPolicyDefinition(r.Assignment.Policies), defaultRouteType(r.Assignment.DefaultAction))
	}, false)
}

func (s *BgpServer) DeletePolicyAssignment(ctx context.Context, r *api.DeletePolicyAssignmentRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Assignment == nil {
			return fmt.Errorf("invalid request")
		}
		id, dir, err := s.toPolicyInfo(r.Assignment.Name, r.Assignment.Direction)
		if err != nil {
			return err
		}
		return s.policy.DeletePolicyAssignment(id, dir, toPolicyDefinition(r.Assignment.Policies), r.All)
	}, false)
}

func (s *BgpServer) SetPolicyAssignment(ctx context.Context, r *api.SetPolicyAssignmentRequest) error {
	return s.mgmtOperation(func() error {
		if r == nil || r.Assignment == nil {
			return fmt.Errorf("invalid request")
		}
		id, dir, err := s.toPolicyInfo(r.Assignment.Name, r.Assignment.Direction)
		if err != nil {
			return err
		}
		return s.policy.SetPolicyAssignment(id, dir, toPolicyDefinition(r.Assignment.Policies), defaultRouteType(r.Assignment.DefaultAction))
	}, false)
}

func (s *BgpServer) EnableMrt(ctx context.Context, r *api.EnableMrtRequest) error {
	return s.mgmtOperation(func() error {
		return s.mrtManager.enable(&config.MrtConfig{
			DumpInterval:     r.DumpInterval,
			RotationInterval: r.RotationInterval,
			DumpType:         config.IntToMrtTypeMap[int(r.DumpType)],
			FileName:         r.Filename,
		})
	}, false)
}

func (s *BgpServer) DisableMrt(ctx context.Context, r *api.DisableMrtRequest) error {
	return s.mgmtOperation(func() error {
		return s.mrtManager.disable(&config.MrtConfig{})
	}, false)
}

func (s *BgpServer) ListRpki(ctx context.Context, r *api.ListRpkiRequest, fn func(*api.Rpki)) error {
	var l []*api.Rpki
	err := s.mgmtOperation(func() error {
		for _, r := range s.roaManager.GetServers() {
			received := &r.State.RpkiMessages.RpkiReceived
			sent := &r.State.RpkiMessages.RpkiSent
			rpki := &api.Rpki{
				Conf: &api.RPKIConf{
					Address:    r.Config.Address,
					RemotePort: uint32(r.Config.Port),
				},
				State: &api.RPKIState{
					Uptime:        config.ProtoTimestamp(r.State.Uptime),
					Downtime:      config.ProtoTimestamp(r.State.Downtime),
					Up:            r.State.Up,
					RecordIpv4:    r.State.RecordsV4,
					RecordIpv6:    r.State.RecordsV6,
					PrefixIpv4:    r.State.PrefixesV4,
					PrefixIpv6:    r.State.PrefixesV6,
					Serial:        r.State.SerialNumber,
					ReceivedIpv4:  received.Ipv4Prefix,
					ReceivedIpv6:  received.Ipv6Prefix,
					SerialNotify:  received.SerialNotify,
					CacheReset:    received.CacheReset,
					CacheResponse: received.CacheResponse,
					EndOfData:     received.EndOfData,
					Error:         received.Error,
					SerialQuery:   sent.SerialQuery,
					ResetQuery:    sent.ResetQuery,
				},
			}
			l = append(l, rpki)
		}
		return nil
	}, false)
	if err == nil {
		for _, r := range l {
			select {
			case <-ctx.Done():
				return nil
			default:
				fn(r)
			}
		}
	}
	return err
}

func (s *BgpServer) ListRpkiTable(ctx context.Context, r *api.ListRpkiTableRequest, fn func(*api.Roa)) error {
	var l []*api.Roa
	err := s.mgmtOperation(func() error {
		family := bgp.RouteFamily(0)
		if r.Family != nil {
			family = bgp.AfiSafiToRouteFamily(uint16(r.Family.Afi), uint8(r.Family.Safi))
		}
		roas, err := s.roaManager.GetRoa(family)
		if err == nil {
			l = append(l, newRoaListFromTableStructList(roas)...)
		}
		return err
	}, false)
	if err == nil {
		for _, roa := range l {
			select {
			case <-ctx.Done():
				return nil
			default:
				fn(roa)
			}
		}
	}
	return err
}

func (s *BgpServer) AddRpki(ctx context.Context, r *api.AddRpkiRequest) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.AddServer(net.JoinHostPort(r.Address, strconv.Itoa(int(r.Port))), r.Lifetime)
	}, false)
}

func (s *BgpServer) DeleteRpki(ctx context.Context, r *api.DeleteRpkiRequest) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.DeleteServer(r.Address)
	}, false)
}

func (s *BgpServer) EnableRpki(ctx context.Context, r *api.EnableRpkiRequest) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.Enable(r.Address)
	}, false)
}

func (s *BgpServer) DisableRpki(ctx context.Context, r *api.DisableRpkiRequest) error {
	return s.mgmtOperation(func() error {
		return s.roaManager.Disable(r.Address)
	}, false)
}

func (s *BgpServer) ResetRpki(ctx context.Context, r *api.ResetRpkiRequest) error {
	return s.mgmtOperation(func() error {
		if r.Soft {
			return s.roaManager.SoftReset(r.Address)
		}
		return s.roaManager.Reset(r.Address)
	}, false)
}

func (s *BgpServer) MonitorTable(ctx context.Context, r *api.MonitorTableRequest, fn func(*api.Path)) error {
	if r == nil {
		return fmt.Errorf("nil request")
	}
	w, err := func() (*watcher, error) {
		switch r.Type {
		case api.Resource_GLOBAL:
			return s.watch(watchBestPath(r.Current)), nil
		case api.Resource_ADJ_IN:
			if r.PostPolicy {
				return s.watch(watchPostUpdate(r.Current)), nil
			}
			return s.watch(watchUpdate(r.Current)), nil
		default:
			return nil, fmt.Errorf("unsupported resource type: %v", r.Type)
		}
	}()
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			w.Stop()
		}()
		family := bgp.RouteFamily(0)
		if r.Family != nil {
			family = bgp.AfiSafiToRouteFamily(uint16(r.Family.Afi), uint8(r.Family.Safi))
		}

		for {
			select {
			case ev := <-w.Event():
				var pl []*table.Path
				switch msg := ev.(type) {
				case *watchEventBestPath:
					if len(msg.MultiPathList) > 0 {
						l := make([]*table.Path, 0)
						for _, p := range msg.MultiPathList {
							l = append(l, p...)
						}
						pl = l
					} else {
						pl = msg.PathList
					}
				case *watchEventUpdate:
					pl = msg.PathList
				}
				for _, path := range pl {
					if path == nil || (r.Family != nil && family != path.GetRouteFamily()) {
						continue
					}
					select {
					case <-ctx.Done():
						return
					default:
						fn(toPathApi(path, nil))
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *BgpServer) MonitorPeer(ctx context.Context, r *api.MonitorPeerRequest, fn func(*api.Peer)) error {
	if r == nil {
		return fmt.Errorf("nil request")
	}

	go func() {
		w := s.watch(watchPeerState(r.Current))
		defer func() {
			w.Stop()
		}()
		for {
			select {
			case m := <-w.Event():
				msg := m.(*watchEventPeerState)
				if len(r.Address) > 0 && r.Address != msg.PeerAddress.String() && r.Address != msg.PeerInterface {
					break
				}
				p := &api.Peer{
					Conf: &api.PeerConf{
						PeerAs:            msg.PeerAS,
						LocalAs:           msg.LocalAS,
						NeighborAddress:   msg.PeerAddress.String(),
						NeighborInterface: msg.PeerInterface,
					},
					State: &api.PeerState{
						PeerAs:          msg.PeerAS,
						LocalAs:         msg.LocalAS,
						NeighborAddress: msg.PeerAddress.String(),
						SessionState:    api.PeerState_SessionState(int(msg.State) + 1),
						AdminState:      api.PeerState_AdminState(msg.AdminState),
						RouterId:        msg.PeerID.String(),
					},
					Transport: &api.Transport{
						LocalAddress: msg.LocalAddress.String(),
						LocalPort:    uint32(msg.LocalPort),
						RemotePort:   uint32(msg.PeerPort),
					},
				}
				fn(p)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

type watchEventType string

const (
	watchEventTypeBestPath   watchEventType = "bestpath"
	watchEventTypePreUpdate  watchEventType = "preupdate"
	watchEventTypePostUpdate watchEventType = "postupdate"
	watchEventTypePeerState  watchEventType = "peerstate"
	watchEventTypeTable      watchEventType = "table"
	watchEventTypeRecvMsg    watchEventType = "receivedmessage"
)

type watchEvent interface {
}

type watchEventUpdate struct {
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

type watchEventPeerState struct {
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
	StateReason   *fsmStateReason
	AdminState    adminState
	Timestamp     time.Time
	PeerInterface string
}

type watchEventAdjIn struct {
	PathList []*table.Path
}

type watchEventTable struct {
	RouterID string
	PathList map[string][]*table.Path
	Neighbor []*config.Neighbor
}

type watchEventBestPath struct {
	PathList      []*table.Path
	MultiPathList [][]*table.Path
	Vrf           map[string]uint32
}

type watchEventMessage struct {
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

type watchOption func(*watchOptions)

func watchBestPath(current bool) watchOption {
	return func(o *watchOptions) {
		o.bestpath = true
		if current {
			o.initBest = true
		}
	}
}

func watchUpdate(current bool) watchOption {
	return func(o *watchOptions) {
		o.preUpdate = true
		if current {
			o.initUpdate = true
		}
	}
}

func watchPostUpdate(current bool) watchOption {
	return func(o *watchOptions) {
		o.postUpdate = true
		if current {
			o.initPostUpdate = true
		}
	}
}

func watchPeerState(current bool) watchOption {
	return func(o *watchOptions) {
		o.peerState = true
		if current {
			o.initPeerState = true
		}
	}
}

func watchTableName(name string) watchOption {
	return func(o *watchOptions) {
		o.tableName = name
	}
}

func watchMessage(isSent bool) watchOption {
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

type watcher struct {
	opts   watchOptions
	realCh chan watchEvent
	ch     *channels.InfiniteChannel
	s      *BgpServer
}

func (w *watcher) Event() <-chan watchEvent {
	return w.realCh
}

func (w *watcher) Generate(t watchEventType) error {
	return w.s.mgmtOperation(func() error {
		switch t {
		case watchEventTypePreUpdate:
			pathList := make([]*table.Path, 0)
			for _, peer := range w.s.neighborMap {
				pathList = append(pathList, peer.adjRibIn.PathList(peer.configuredRFlist(), false)...)
			}
			w.notify(&watchEventAdjIn{PathList: clonePathList(pathList)})
		case watchEventTypeTable:
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
					for _, dst := range t.GetDestinations() {
						if paths := dst.GetKnownPathList(id, as); len(paths) > 0 {
							pathList[dst.GetNlri().String()] = clonePathList(paths)
						}
					}
				}
				return pathList
			}()
			l := make([]*config.Neighbor, 0, len(w.s.neighborMap))
			for _, peer := range w.s.neighborMap {
				l = append(l, w.s.toConfig(peer, false))
			}
			w.notify(&watchEventTable{PathList: pathList, Neighbor: l})
		default:
			return fmt.Errorf("unsupported type %v", t)
		}
		return nil
	}, false)
}

func (w *watcher) notify(v watchEvent) {
	w.ch.In() <- v
}

func (w *watcher) loop() {
	for ev := range w.ch.Out() {
		w.realCh <- ev.(watchEvent)
	}
	close(w.realCh)
}

func (w *watcher) Stop() {
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

func (s *BgpServer) isWatched(typ watchEventType) bool {
	return len(s.watcherMap[typ]) != 0
}

func (s *BgpServer) notifyWatcher(typ watchEventType, ev watchEvent) {
	for _, w := range s.watcherMap[typ] {
		w.notify(ev)
	}
}

func (s *BgpServer) watch(opts ...watchOption) (w *watcher) {
	s.mgmtOperation(func() error {
		w = &watcher{
			s:      s,
			realCh: make(chan watchEvent, 8),
			ch:     channels.NewInfiniteChannel(),
		}

		for _, opt := range opts {
			opt(&w.opts)
		}

		register := func(t watchEventType, w *watcher) {
			s.watcherMap[t] = append(s.watcherMap[t], w)
		}

		if w.opts.bestpath {
			register(watchEventTypeBestPath, w)
		}
		if w.opts.preUpdate {
			register(watchEventTypePreUpdate, w)
		}
		if w.opts.postUpdate {
			register(watchEventTypePostUpdate, w)
		}
		if w.opts.peerState {
			register(watchEventTypePeerState, w)
		}
		if w.opts.initPeerState {
			for _, peer := range s.neighborMap {
				peer.fsm.lock.RLock()
				notEstablished := peer.fsm.state != bgp.BGP_FSM_ESTABLISHED
				peer.fsm.lock.RUnlock()
				if notEstablished {
					continue
				}
				w.notify(newWatchEventPeerState(peer, nil))
			}
		}
		if w.opts.initBest && s.active() == nil {
			w.notify(&watchEventBestPath{
				PathList:      s.globalRib.GetBestPathList(table.GLOBAL_RIB_NAME, 0, nil),
				MultiPathList: s.globalRib.GetBestMultiPathList(table.GLOBAL_RIB_NAME, nil),
			})
		}
		if w.opts.initUpdate {
			for _, peer := range s.neighborMap {
				peer.fsm.lock.RLock()
				notEstablished := peer.fsm.state != bgp.BGP_FSM_ESTABLISHED
				peer.fsm.lock.RUnlock()
				if notEstablished {
					continue
				}
				configNeighbor := w.s.toConfig(peer, false)
				for _, rf := range peer.configuredRFlist() {
					peer.fsm.lock.RLock()
					_, y := peer.fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]
					l, _ := peer.fsm.LocalHostPort()
					update := &watchEventUpdate{
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
					}
					peer.fsm.lock.RUnlock()
					w.notify(update)

					eor := bgp.NewEndOfRib(rf)
					eorBuf, _ := eor.Serialize()
					peer.fsm.lock.RLock()
					update = &watchEventUpdate{
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
					}
					peer.fsm.lock.RUnlock()
					w.notify(update)
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
						configNeighbor = w.s.toConfig(peer, false)
					}

					w.notify(&watchEventUpdate{
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
					w.notify(&watchEventUpdate{
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
			register(watchEventTypeRecvMsg, w)
		}

		go w.loop()
		return nil
	}, false)
	return w
}
