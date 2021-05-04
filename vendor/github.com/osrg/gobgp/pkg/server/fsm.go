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

package server

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/eapache/channels"
	"github.com/osrg/gobgp/internal/pkg/config"
	"github.com/osrg/gobgp/internal/pkg/table"
	"github.com/osrg/gobgp/pkg/packet/bgp"
	"github.com/osrg/gobgp/pkg/packet/bmp"

	log "github.com/sirupsen/logrus"
)

type peerDownReason int

const (
	peerDownByLocal peerDownReason = iota
	peerDownByLocalWithoutNotification
	peerDownByRemote
	peerDownByRemoteWithoutNotification
)

type fsmStateReasonType uint8

const (
	fsmDying fsmStateReasonType = iota
	fsmAdminDown
	fsmReadFailed
	fsmWriteFailed
	fsmNotificationSent
	fsmNotificationRecv
	fsmHoldTimerExpired
	fsmIdleTimerExpired
	fsmRestartTimerExpired
	fsmGracefulRestart
	fsmInvalidMsg
	fsmNewConnection
	fsmOpenMsgReceived
	fsmOpenMsgNegotiated
	fsmHardReset
)

type fsmStateReason struct {
	Type            fsmStateReasonType
	peerDownReason  peerDownReason
	BGPNotification *bgp.BGPMessage
	Data            []byte
}

func newfsmStateReason(typ fsmStateReasonType, notif *bgp.BGPMessage, data []byte) *fsmStateReason {
	var reasonCode peerDownReason
	switch typ {
	case fsmDying, fsmInvalidMsg, fsmNotificationSent, fsmHoldTimerExpired, fsmIdleTimerExpired, fsmRestartTimerExpired:
		reasonCode = peerDownByLocal
	case fsmAdminDown:
		reasonCode = peerDownByLocalWithoutNotification
	case fsmNotificationRecv, fsmGracefulRestart, fsmHardReset:
		reasonCode = peerDownByRemote
	case fsmReadFailed, fsmWriteFailed:
		reasonCode = peerDownByRemoteWithoutNotification
	}
	return &fsmStateReason{
		Type:            typ,
		peerDownReason:  reasonCode,
		BGPNotification: notif,
		Data:            data,
	}
}

func (r fsmStateReason) String() string {
	switch r.Type {
	case fsmDying:
		return "dying"
	case fsmAdminDown:
		return "admin-down"
	case fsmReadFailed:
		return "read-failed"
	case fsmWriteFailed:
		return "write-failed"
	case fsmNotificationSent:
		body := r.BGPNotification.Body.(*bgp.BGPNotification)
		return fmt.Sprintf("notification-sent %s", bgp.NewNotificationErrorCode(body.ErrorCode, body.ErrorSubcode).String())
	case fsmNotificationRecv:
		body := r.BGPNotification.Body.(*bgp.BGPNotification)
		return fmt.Sprintf("notification-received %s", bgp.NewNotificationErrorCode(body.ErrorCode, body.ErrorSubcode).String())
	case fsmHoldTimerExpired:
		return "hold-timer-expired"
	case fsmIdleTimerExpired:
		return "idle-hold-timer-expired"
	case fsmRestartTimerExpired:
		return "restart-timer-expired"
	case fsmGracefulRestart:
		return "graceful-restart"
	case fsmInvalidMsg:
		return "invalid-msg"
	case fsmNewConnection:
		return "new-connection"
	case fsmOpenMsgReceived:
		return "open-msg-received"
	case fsmOpenMsgNegotiated:
		return "open-msg-negotiated"
	case fsmHardReset:
		return "hard-reset"
	default:
		return "unknown"
	}
}

type fsmMsgType int

const (
	_ fsmMsgType = iota
	fsmMsgStateChange
	fsmMsgBGPMessage
	fsmMsgRouteRefresh
)

type fsmMsg struct {
	MsgType     fsmMsgType
	MsgSrc      string
	MsgData     interface{}
	StateReason *fsmStateReason
	PathList    []*table.Path
	timestamp   time.Time
	payload     []byte
	Version     uint
}

type fsmOutgoingMsg struct {
	Paths        []*table.Path
	Notification *bgp.BGPMessage
	StayIdle     bool
}

const (
	holdtimeOpensent = 240
	holdtimeIdle     = 5
)

type adminState int

const (
	adminStateUp adminState = iota
	adminStateDown
	adminStatePfxCt
)

func (s adminState) String() string {
	switch s {
	case adminStateUp:
		return "adminStateUp"
	case adminStateDown:
		return "adminStateDown"
	case adminStatePfxCt:
		return "adminStatePfxCt"
	default:
		return "Unknown"
	}
}

type adminStateOperation struct {
	State         adminState
	Communication []byte
}

var fsmVersion uint

type fsm struct {
	gConf                *config.Global
	pConf                *config.Neighbor
	lock                 sync.RWMutex
	state                bgp.FSMState
	reason               *fsmStateReason
	conn                 net.Conn
	connCh               chan net.Conn
	idleHoldTime         float64
	opensentHoldTime     float64
	adminState           adminState
	adminStateCh         chan adminStateOperation
	h                    *fsmHandler
	rfMap                map[bgp.RouteFamily]bgp.BGPAddPathMode
	capMap               map[bgp.BGPCapabilityCode][]bgp.ParameterCapabilityInterface
	recvOpen             *bgp.BGPMessage
	peerInfo             *table.PeerInfo
	policy               *table.RoutingPolicy
	gracefulRestartTimer *time.Timer
	twoByteAsTrans       bool
	version              uint
	marshallingOptions   *bgp.MarshallingOption
}

func (fsm *fsm) bgpMessageStateUpdate(MessageType uint8, isIn bool) {
	fsm.lock.Lock()
	defer fsm.lock.Unlock()
	state := &fsm.pConf.State.Messages
	timer := &fsm.pConf.Timers
	if isIn {
		state.Received.Total++
	} else {
		state.Sent.Total++
	}
	switch MessageType {
	case bgp.BGP_MSG_OPEN:
		if isIn {
			state.Received.Open++
		} else {
			state.Sent.Open++
		}
	case bgp.BGP_MSG_UPDATE:
		if isIn {
			state.Received.Update++
			timer.State.UpdateRecvTime = time.Now().Unix()
		} else {
			state.Sent.Update++
		}
	case bgp.BGP_MSG_NOTIFICATION:
		if isIn {
			state.Received.Notification++
		} else {
			state.Sent.Notification++
		}
	case bgp.BGP_MSG_KEEPALIVE:
		if isIn {
			state.Received.Keepalive++
		} else {
			state.Sent.Keepalive++
		}
	case bgp.BGP_MSG_ROUTE_REFRESH:
		if isIn {
			state.Received.Refresh++
		} else {
			state.Sent.Refresh++
		}
	default:
		if isIn {
			state.Received.Discarded++
		} else {
			state.Sent.Discarded++
		}
	}
}

func (fsm *fsm) bmpStatsUpdate(statType uint16, increment int) {
	fsm.lock.Lock()
	defer fsm.lock.Unlock()
	stats := &fsm.pConf.State.Messages.Received
	switch statType {
	// TODO
	// Support other stat types.
	case bmp.BMP_STAT_TYPE_WITHDRAW_UPDATE:
		stats.WithdrawUpdate += uint32(increment)
	case bmp.BMP_STAT_TYPE_WITHDRAW_PREFIX:
		stats.WithdrawPrefix += uint32(increment)
	}
}

func newFSM(gConf *config.Global, pConf *config.Neighbor, policy *table.RoutingPolicy) *fsm {
	adminState := adminStateUp
	if pConf.Config.AdminDown {
		adminState = adminStateDown
	}
	pConf.State.SessionState = config.IntToSessionStateMap[int(bgp.BGP_FSM_IDLE)]
	pConf.Timers.State.Downtime = time.Now().Unix()
	fsmVersion++
	fsm := &fsm{
		gConf:                gConf,
		pConf:                pConf,
		state:                bgp.BGP_FSM_IDLE,
		connCh:               make(chan net.Conn, 1),
		opensentHoldTime:     float64(holdtimeOpensent),
		adminState:           adminState,
		adminStateCh:         make(chan adminStateOperation, 1),
		rfMap:                make(map[bgp.RouteFamily]bgp.BGPAddPathMode),
		capMap:               make(map[bgp.BGPCapabilityCode][]bgp.ParameterCapabilityInterface),
		peerInfo:             table.NewPeerInfo(gConf, pConf),
		policy:               policy,
		gracefulRestartTimer: time.NewTimer(time.Hour),
		version:              fsmVersion,
	}
	fsm.gracefulRestartTimer.Stop()
	return fsm
}

func (fsm *fsm) StateChange(nextState bgp.FSMState) {
	fsm.lock.Lock()
	defer fsm.lock.Unlock()

	log.WithFields(log.Fields{
		"Topic":  "Peer",
		"Key":    fsm.pConf.State.NeighborAddress,
		"old":    fsm.state.String(),
		"new":    nextState.String(),
		"reason": fsm.reason,
	}).Debug("state changed")
	fsm.state = nextState
	switch nextState {
	case bgp.BGP_FSM_ESTABLISHED:
		fsm.pConf.Timers.State.Uptime = time.Now().Unix()
		fsm.pConf.State.EstablishedCount++
		// reset the state set by the previous session
		fsm.twoByteAsTrans = false
		if _, y := fsm.capMap[bgp.BGP_CAP_FOUR_OCTET_AS_NUMBER]; !y {
			fsm.twoByteAsTrans = true
			break
		}
		y := func() bool {
			for _, c := range capabilitiesFromConfig(fsm.pConf) {
				switch c.(type) {
				case *bgp.CapFourOctetASNumber:
					return true
				}
			}
			return false
		}()
		if !y {
			fsm.twoByteAsTrans = true
		}
	default:
		fsm.pConf.Timers.State.Downtime = time.Now().Unix()
	}
}

func hostport(addr net.Addr) (string, uint16) {
	if addr != nil {
		host, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			return "", 0
		}
		p, _ := strconv.ParseUint(port, 10, 16)
		return host, uint16(p)
	}
	return "", 0
}

func (fsm *fsm) RemoteHostPort() (string, uint16) {
	return hostport(fsm.conn.RemoteAddr())

}

func (fsm *fsm) LocalHostPort() (string, uint16) {
	return hostport(fsm.conn.LocalAddr())
}

func (fsm *fsm) sendNotificationFromErrorMsg(e *bgp.MessageError) (*bgp.BGPMessage, error) {
	fsm.lock.RLock()
	established := fsm.h != nil && fsm.h.conn != nil
	fsm.lock.RUnlock()

	if established {
		m := bgp.NewBGPNotificationMessage(e.TypeCode, e.SubTypeCode, e.Data)
		b, _ := m.Serialize()
		_, err := fsm.h.conn.Write(b)
		if err == nil {
			fsm.bgpMessageStateUpdate(m.Header.Type, false)
			fsm.h.sentNotification = m
		}
		fsm.h.conn.Close()
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   fsm.pConf.State.NeighborAddress,
			"Data":  e,
		}).Warn("sent notification")
		return m, nil
	}
	return nil, fmt.Errorf("can't send notification to %s since TCP connection is not established", fsm.pConf.State.NeighborAddress)
}

func (fsm *fsm) sendNotification(code, subType uint8, data []byte, msg string) (*bgp.BGPMessage, error) {
	e := bgp.NewMessageError(code, subType, data, msg)
	return fsm.sendNotificationFromErrorMsg(e.(*bgp.MessageError))
}

type fsmHandler struct {
	fsm              *fsm
	conn             net.Conn
	msgCh            *channels.InfiniteChannel
	stateReasonCh    chan fsmStateReason
	incoming         *channels.InfiniteChannel
	stateCh          chan *fsmMsg
	outgoing         *channels.InfiniteChannel
	holdTimerResetCh chan bool
	sentNotification *bgp.BGPMessage
	ctx              context.Context
	ctxCancel        context.CancelFunc
	wg               *sync.WaitGroup
}

func newFSMHandler(fsm *fsm, incoming *channels.InfiniteChannel, stateCh chan *fsmMsg, outgoing *channels.InfiniteChannel) *fsmHandler {
	ctx, cancel := context.WithCancel(context.Background())
	h := &fsmHandler{
		fsm:              fsm,
		stateReasonCh:    make(chan fsmStateReason, 2),
		incoming:         incoming,
		stateCh:          stateCh,
		outgoing:         outgoing,
		holdTimerResetCh: make(chan bool, 2),
		wg:               &sync.WaitGroup{},
		ctx:              ctx,
		ctxCancel:        cancel,
	}
	h.wg.Add(1)
	go h.loop(ctx, h.wg)
	return h
}

func (h *fsmHandler) idle(ctx context.Context) (bgp.FSMState, *fsmStateReason) {
	fsm := h.fsm

	fsm.lock.RLock()
	idleHoldTimer := time.NewTimer(time.Second * time.Duration(fsm.idleHoldTime))
	fsm.lock.RUnlock()

	for {
		select {
		case <-ctx.Done():
			return -1, newfsmStateReason(fsmDying, nil, nil)
		case <-fsm.gracefulRestartTimer.C:
			fsm.lock.RLock()
			restarting := fsm.pConf.GracefulRestart.State.PeerRestarting
			fsm.lock.RUnlock()

			if restarting {
				fsm.lock.RLock()
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Warn("graceful restart timer expired")
				fsm.lock.RUnlock()
				return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmRestartTimerExpired, nil, nil)
			}
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			conn.Close()
			fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("Closed an accepted connection")
			fsm.lock.RUnlock()

		case <-idleHoldTimer.C:
			fsm.lock.RLock()
			adminStateUp := fsm.adminState == adminStateUp
			fsm.lock.RUnlock()

			if adminStateUp {
				fsm.lock.Lock()
				log.WithFields(log.Fields{
					"Topic":    "Peer",
					"Key":      fsm.pConf.State.NeighborAddress,
					"Duration": fsm.idleHoldTime,
				}).Debug("IdleHoldTimer expired")
				fsm.idleHoldTime = holdtimeIdle
				fsm.lock.Unlock()
				return bgp.BGP_FSM_ACTIVE, newfsmStateReason(fsmIdleTimerExpired, nil, nil)

			} else {
				log.WithFields(log.Fields{"Topic": "Peer"}).Debug("IdleHoldTimer expired, but stay at idle because the admin state is DOWN")
			}

		case stateOp := <-fsm.adminStateCh:
			err := h.changeadminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case adminStateDown:
					// stop idle hold timer
					idleHoldTimer.Stop()

				case adminStateUp:
					// restart idle hold timer
					fsm.lock.RLock()
					idleHoldTimer.Reset(time.Second * time.Duration(fsm.idleHoldTime))
					fsm.lock.RUnlock()
				}
			}
		}
	}
}

func (h *fsmHandler) connectLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	fsm := h.fsm

	tick, addr, port, password, ttl, ttlMin, localAddress := func() (int, string, int, string, uint8, uint8, string) {
		fsm.lock.RLock()
		defer fsm.lock.RUnlock()

		tick := int(fsm.pConf.Timers.Config.ConnectRetry)
		if tick < minConnectRetry {
			tick = minConnectRetry
		}

		addr := fsm.pConf.State.NeighborAddress
		port := int(bgp.BGP_PORT)
		if fsm.pConf.Transport.Config.RemotePort != 0 {
			port = int(fsm.pConf.Transport.Config.RemotePort)
		}
		password := fsm.pConf.Config.AuthPassword
		ttl := uint8(0)
		ttlMin := uint8(0)

		if fsm.pConf.TtlSecurity.Config.Enabled {
			ttl = 255
			ttlMin = fsm.pConf.TtlSecurity.Config.TtlMin
		} else if fsm.pConf.Config.PeerAs != 0 && fsm.pConf.Config.PeerType == config.PEER_TYPE_EXTERNAL {
			ttl = 1
			if fsm.pConf.EbgpMultihop.Config.Enabled {
				ttl = fsm.pConf.EbgpMultihop.Config.MultihopTtl
			}
		}
		return tick, addr, port, password, ttl, ttlMin, fsm.pConf.Transport.Config.LocalAddress
	}()

	for {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		timer := time.NewTimer(time.Duration(r.Intn(tick)+tick) * time.Second)
		select {
		case <-ctx.Done():
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   addr,
			}).Debug("stop connect loop")
			timer.Stop()
			return
		case <-timer.C:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   addr,
			}).Debug("try to connect")
		}

		laddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(localAddress, "0"))
		if err != nil {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   addr,
			}).Warnf("failed to resolve local address: %s", err)
		}

		if err == nil {
			d := net.Dialer{
				LocalAddr: laddr,
				Timeout:   time.Duration(tick-1) * time.Second,
				Control: func(network, address string, c syscall.RawConn) error {
					return dialerControl(network, address, c, ttl, ttlMin, password)
				},
			}

			conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(addr, fmt.Sprintf("%d", port)))
			select {
			case <-ctx.Done():
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   addr,
				}).Debug("stop connect loop")
				return
			default:
			}

			if err == nil {
				select {
				case fsm.connCh <- conn:
					return
				default:
					conn.Close()
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   addr,
					}).Warn("active conn is closed to avoid being blocked")
				}
			} else {
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   addr,
				}).Debugf("failed to connect: %s", err)
			}
		}
	}
}

func (h *fsmHandler) active(ctx context.Context) (bgp.FSMState, *fsmStateReason) {
	c, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go h.connectLoop(c, &wg)

	defer func() {
		cancel()
		wg.Wait()
	}()

	fsm := h.fsm
	for {
		select {
		case <-ctx.Done():
			return -1, newfsmStateReason(fsmDying, nil, nil)
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			fsm.lock.Lock()
			fsm.conn = conn
			fsm.lock.Unlock()
			ttl := 0
			ttlMin := 0

			fsm.lock.RLock()
			if fsm.pConf.TtlSecurity.Config.Enabled {
				ttl = 255
				ttlMin = int(fsm.pConf.TtlSecurity.Config.TtlMin)
			} else if fsm.pConf.Config.PeerAs != 0 && fsm.pConf.Config.PeerType == config.PEER_TYPE_EXTERNAL {
				if fsm.pConf.EbgpMultihop.Config.Enabled {
					ttl = int(fsm.pConf.EbgpMultihop.Config.MultihopTtl)
				} else if fsm.pConf.Transport.Config.Ttl != 0 {
					ttl = int(fsm.pConf.Transport.Config.Ttl)
				} else {
					ttl = 1
				}
			} else if fsm.pConf.Transport.Config.Ttl != 0 {
				ttl = int(fsm.pConf.Transport.Config.Ttl)
			}
			if ttl != 0 {
				if err := setTCPTTLSockopt(conn.(*net.TCPConn), ttl); err != nil {
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   fsm.pConf.Config.NeighborAddress,
						"State": fsm.state.String(),
					}).Warnf("cannot set TTL(=%d) for peer: %s", ttl, err)
				}
			}
			if ttlMin != 0 {
				if err := setTCPMinTTLSockopt(conn.(*net.TCPConn), ttlMin); err != nil {
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   fsm.pConf.Config.NeighborAddress,
						"State": fsm.state.String(),
					}).Warnf("cannot set minimal TTL(=%d) for peer: %s", ttl, err)
				}
			}
			fsm.lock.RUnlock()
			// we don't implement delayed open timer so move to opensent right
			// away.
			return bgp.BGP_FSM_OPENSENT, newfsmStateReason(fsmNewConnection, nil, nil)
		case <-fsm.gracefulRestartTimer.C:
			fsm.lock.RLock()
			restarting := fsm.pConf.GracefulRestart.State.PeerRestarting
			fsm.lock.RUnlock()
			if restarting {
				fsm.lock.RLock()
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Warn("graceful restart timer expired")
				fsm.lock.RUnlock()
				return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmRestartTimerExpired, nil, nil)
			}
		case err := <-h.stateReasonCh:
			return bgp.BGP_FSM_IDLE, &err
		case stateOp := <-fsm.adminStateCh:
			err := h.changeadminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case adminStateDown:
					return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmAdminDown, nil, nil)
				case adminStateUp:
					log.WithFields(log.Fields{
						"Topic":      "Peer",
						"Key":        fsm.pConf.State.NeighborAddress,
						"State":      fsm.state.String(),
						"adminState": stateOp.State.String(),
					}).Panic("code logic bug")
				}
			}
		}
	}
}

func capAddPathFromConfig(pConf *config.Neighbor) bgp.ParameterCapabilityInterface {
	tuples := make([]*bgp.CapAddPathTuple, 0, len(pConf.AfiSafis))
	for _, af := range pConf.AfiSafis {
		var mode bgp.BGPAddPathMode
		if af.AddPaths.State.Receive {
			mode |= bgp.BGP_ADD_PATH_RECEIVE
		}
		if af.AddPaths.State.SendMax > 0 {
			mode |= bgp.BGP_ADD_PATH_SEND
		}
		if mode > 0 {
			tuples = append(tuples, bgp.NewCapAddPathTuple(af.State.Family, mode))
		}
	}
	if len(tuples) == 0 {
		return nil
	}
	return bgp.NewCapAddPath(tuples)
}

func capabilitiesFromConfig(pConf *config.Neighbor) []bgp.ParameterCapabilityInterface {
	caps := make([]bgp.ParameterCapabilityInterface, 0, 4)
	caps = append(caps, bgp.NewCapRouteRefresh())
	for _, af := range pConf.AfiSafis {
		caps = append(caps, bgp.NewCapMultiProtocol(af.State.Family))
	}
	caps = append(caps, bgp.NewCapFourOctetASNumber(pConf.Config.LocalAs))

	if c := pConf.GracefulRestart.Config; c.Enabled {
		tuples := []*bgp.CapGracefulRestartTuple{}
		ltuples := []*bgp.CapLongLivedGracefulRestartTuple{}

		// RFC 4724 4.1
		// To re-establish the session with its peer, the Restarting Speaker
		// MUST set the "Restart State" bit in the Graceful Restart Capability
		// of the OPEN message.
		restarting := pConf.GracefulRestart.State.LocalRestarting

		if !c.HelperOnly {
			for i, rf := range pConf.AfiSafis {
				if m := rf.MpGracefulRestart.Config; m.Enabled {
					// When restarting, always flag forwaring bit.
					// This can be a lie, depending on how gobgpd is used.
					// For a route-server use-case, since a route-server
					// itself doesn't forward packets, and the dataplane
					// is a l2 switch which continues to work with no
					// relation to bgpd, this behavior is ok.
					// TODO consideration of other use-cases
					tuples = append(tuples, bgp.NewCapGracefulRestartTuple(rf.State.Family, restarting))
					pConf.AfiSafis[i].MpGracefulRestart.State.Advertised = true
				}
				if m := rf.LongLivedGracefulRestart.Config; m.Enabled {
					ltuples = append(ltuples, bgp.NewCapLongLivedGracefulRestartTuple(rf.State.Family, restarting, m.RestartTime))
				}
			}
		}
		restartTime := c.RestartTime
		notification := c.NotificationEnabled
		caps = append(caps, bgp.NewCapGracefulRestart(restarting, notification, restartTime, tuples))
		if c.LongLivedEnabled {
			caps = append(caps, bgp.NewCapLongLivedGracefulRestart(ltuples))
		}
	}

	// unnumbered BGP
	if pConf.Config.NeighborInterface != "" {
		tuples := []*bgp.CapExtendedNexthopTuple{}
		families, _ := config.AfiSafis(pConf.AfiSafis).ToRfList()
		for _, family := range families {
			if family == bgp.RF_IPv6_UC {
				continue
			}
			tuple := bgp.NewCapExtendedNexthopTuple(family, bgp.AFI_IP6)
			tuples = append(tuples, tuple)
		}
		caps = append(caps, bgp.NewCapExtendedNexthop(tuples))
	}

	// ADD-PATH Capability
	if c := capAddPathFromConfig(pConf); c != nil {
		caps = append(caps, capAddPathFromConfig(pConf))
	}

	return caps
}

func buildopen(gConf *config.Global, pConf *config.Neighbor) *bgp.BGPMessage {
	caps := capabilitiesFromConfig(pConf)
	opt := bgp.NewOptionParameterCapability(caps)
	holdTime := uint16(pConf.Timers.Config.HoldTime)
	as := pConf.Config.LocalAs
	if as > (1<<16)-1 {
		as = bgp.AS_TRANS
	}
	return bgp.NewBGPOpenMessage(uint16(as), holdTime, gConf.Config.RouterId,
		[]bgp.OptionParameterInterface{opt})
}

func readAll(conn net.Conn, length int) ([]byte, error) {
	buf := make([]byte, length)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func getPathAttrFromBGPUpdate(m *bgp.BGPUpdate, typ bgp.BGPAttrType) bgp.PathAttributeInterface {
	for _, a := range m.PathAttributes {
		if a.GetType() == typ {
			return a
		}
	}
	return nil
}

func hasOwnASLoop(ownAS uint32, limit int, asPath *bgp.PathAttributeAsPath) bool {
	cnt := 0
	for _, param := range asPath.Value {
		for _, as := range param.GetAS() {
			if as == ownAS {
				cnt++
				if cnt > limit {
					return true
				}
			}
		}
	}
	return false
}

func extractRouteFamily(p *bgp.PathAttributeInterface) *bgp.RouteFamily {
	attr := *p

	var afi uint16
	var safi uint8

	switch a := attr.(type) {
	case *bgp.PathAttributeMpReachNLRI:
		afi = a.AFI
		safi = a.SAFI
	case *bgp.PathAttributeMpUnreachNLRI:
		afi = a.AFI
		safi = a.SAFI
	default:
		return nil
	}

	rf := bgp.AfiSafiToRouteFamily(afi, safi)
	return &rf
}

func (h *fsmHandler) afiSafiDisable(rf bgp.RouteFamily) string {
	h.fsm.lock.Lock()
	defer h.fsm.lock.Unlock()

	n := bgp.AddressFamilyNameMap[rf]

	for i, a := range h.fsm.pConf.AfiSafis {
		if string(a.Config.AfiSafiName) == n {
			h.fsm.pConf.AfiSafis[i].State.Enabled = false
			break
		}
	}
	newList := make([]bgp.ParameterCapabilityInterface, 0)
	for _, c := range h.fsm.capMap[bgp.BGP_CAP_MULTIPROTOCOL] {
		if c.(*bgp.CapMultiProtocol).CapValue == rf {
			continue
		}
		newList = append(newList, c)
	}
	h.fsm.capMap[bgp.BGP_CAP_MULTIPROTOCOL] = newList
	return n
}

func (h *fsmHandler) handlingError(m *bgp.BGPMessage, e error, useRevisedError bool) bgp.ErrorHandling {
	handling := bgp.ERROR_HANDLING_NONE
	if m.Header.Type == bgp.BGP_MSG_UPDATE && useRevisedError {
		factor := e.(*bgp.MessageError)
		handling = factor.ErrorHandling
		switch handling {
		case bgp.ERROR_HANDLING_ATTRIBUTE_DISCARD:
			h.fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   h.fsm.pConf.State.NeighborAddress,
				"State": h.fsm.state.String(),
				"error": e,
			}).Warn("Some attributes were discarded")
			h.fsm.lock.RUnlock()
		case bgp.ERROR_HANDLING_TREAT_AS_WITHDRAW:
			m.Body = bgp.TreatAsWithdraw(m.Body.(*bgp.BGPUpdate))
			h.fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   h.fsm.pConf.State.NeighborAddress,
				"State": h.fsm.state.String(),
				"error": e,
			}).Warn("the received Update message was treated as withdraw")
			h.fsm.lock.RUnlock()
		case bgp.ERROR_HANDLING_AFISAFI_DISABLE:
			rf := extractRouteFamily(factor.ErrorAttribute)
			if rf == nil {
				h.fsm.lock.RLock()
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   h.fsm.pConf.State.NeighborAddress,
					"State": h.fsm.state.String(),
				}).Warn("Error occurred during AFI/SAFI disabling")
				h.fsm.lock.RUnlock()
			} else {
				n := h.afiSafiDisable(*rf)
				h.fsm.lock.RLock()
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   h.fsm.pConf.State.NeighborAddress,
					"State": h.fsm.state.String(),
					"error": e,
				}).Warnf("Capability %s was disabled", n)
				h.fsm.lock.RUnlock()
			}
		}
	} else {
		handling = bgp.ERROR_HANDLING_SESSION_RESET
	}
	return handling
}

func (h *fsmHandler) recvMessageWithError() (*fsmMsg, error) {
	sendToStateReasonCh := func(typ fsmStateReasonType, notif *bgp.BGPMessage) {
		// probably doesn't happen but be cautious
		select {
		case h.stateReasonCh <- *newfsmStateReason(typ, notif, nil):
		default:
		}
	}

	headerBuf, err := readAll(h.conn, bgp.BGP_HEADER_LENGTH)
	if err != nil {
		sendToStateReasonCh(fsmReadFailed, nil)
		return nil, err
	}

	hd := &bgp.BGPHeader{}
	err = hd.DecodeFromBytes(headerBuf)
	if err != nil {
		h.fsm.bgpMessageStateUpdate(0, true)
		h.fsm.lock.RLock()
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   h.fsm.pConf.State.NeighborAddress,
			"State": h.fsm.state.String(),
			"error": err,
		}).Warn("Session will be reset due to malformed BGP Header")
		fmsg := &fsmMsg{
			MsgType: fsmMsgBGPMessage,
			MsgSrc:  h.fsm.pConf.State.NeighborAddress,
			MsgData: err,
			Version: h.fsm.version,
		}
		h.fsm.lock.RUnlock()
		return fmsg, err
	}

	bodyBuf, err := readAll(h.conn, int(hd.Len)-bgp.BGP_HEADER_LENGTH)
	if err != nil {
		sendToStateReasonCh(fsmReadFailed, nil)
		return nil, err
	}

	now := time.Now()
	handling := bgp.ERROR_HANDLING_NONE

	h.fsm.lock.RLock()
	useRevisedError := h.fsm.pConf.ErrorHandling.Config.TreatAsWithdraw
	options := h.fsm.marshallingOptions
	h.fsm.lock.RUnlock()

	m, err := bgp.ParseBGPBody(hd, bodyBuf, options)
	if err != nil {
		handling = h.handlingError(m, err, useRevisedError)
		h.fsm.bgpMessageStateUpdate(0, true)
	} else {
		h.fsm.bgpMessageStateUpdate(m.Header.Type, true)
		err = bgp.ValidateBGPMessage(m)
	}
	h.fsm.lock.RLock()
	fmsg := &fsmMsg{
		MsgType:   fsmMsgBGPMessage,
		MsgSrc:    h.fsm.pConf.State.NeighborAddress,
		timestamp: now,
		Version:   h.fsm.version,
	}
	h.fsm.lock.RUnlock()

	switch handling {
	case bgp.ERROR_HANDLING_AFISAFI_DISABLE:
		fmsg.MsgData = m
		return fmsg, nil
	case bgp.ERROR_HANDLING_SESSION_RESET:
		h.fsm.lock.RLock()
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   h.fsm.pConf.State.NeighborAddress,
			"State": h.fsm.state.String(),
			"error": err,
		}).Warn("Session will be reset due to malformed BGP message")
		h.fsm.lock.RUnlock()
		fmsg.MsgData = err
		return fmsg, err
	default:
		fmsg.MsgData = m

		h.fsm.lock.RLock()
		establishedState := h.fsm.state == bgp.BGP_FSM_ESTABLISHED
		h.fsm.lock.RUnlock()

		if establishedState {
			switch m.Header.Type {
			case bgp.BGP_MSG_ROUTE_REFRESH:
				fmsg.MsgType = fsmMsgRouteRefresh
			case bgp.BGP_MSG_UPDATE:
				body := m.Body.(*bgp.BGPUpdate)
				isEBGP := h.fsm.pConf.IsEBGPPeer(h.fsm.gConf)
				isConfed := h.fsm.pConf.IsConfederationMember(h.fsm.gConf)

				fmsg.payload = make([]byte, len(headerBuf)+len(bodyBuf))
				copy(fmsg.payload, headerBuf)
				copy(fmsg.payload[len(headerBuf):], bodyBuf)

				h.fsm.lock.RLock()
				rfMap := h.fsm.rfMap
				h.fsm.lock.RUnlock()
				ok, err := bgp.ValidateUpdateMsg(body, rfMap, isEBGP, isConfed)
				if !ok {
					handling = h.handlingError(m, err, useRevisedError)
				}
				if handling == bgp.ERROR_HANDLING_SESSION_RESET {
					h.fsm.lock.RLock()
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   h.fsm.pConf.State.NeighborAddress,
						"State": h.fsm.state.String(),
						"error": err,
					}).Warn("Session will be reset due to malformed BGP update message")
					h.fsm.lock.RUnlock()
					fmsg.MsgData = err
					return fmsg, err
				}

				if routes := len(body.WithdrawnRoutes); routes > 0 {
					h.fsm.bmpStatsUpdate(bmp.BMP_STAT_TYPE_WITHDRAW_UPDATE, 1)
					h.fsm.bmpStatsUpdate(bmp.BMP_STAT_TYPE_WITHDRAW_PREFIX, routes)
				} else if attr := getPathAttrFromBGPUpdate(body, bgp.BGP_ATTR_TYPE_MP_UNREACH_NLRI); attr != nil {
					mpUnreach := attr.(*bgp.PathAttributeMpUnreachNLRI)
					if routes = len(mpUnreach.Value); routes > 0 {
						h.fsm.bmpStatsUpdate(bmp.BMP_STAT_TYPE_WITHDRAW_UPDATE, 1)
						h.fsm.bmpStatsUpdate(bmp.BMP_STAT_TYPE_WITHDRAW_PREFIX, routes)
					}
				}

				table.UpdatePathAttrs4ByteAs(body)
				if err = table.UpdatePathAggregator4ByteAs(body); err != nil {
					fmsg.MsgData = err
					return fmsg, err
				}

				h.fsm.lock.RLock()
				peerInfo := h.fsm.peerInfo
				h.fsm.lock.RUnlock()
				fmsg.PathList = table.ProcessMessage(m, peerInfo, fmsg.timestamp)
				fallthrough
			case bgp.BGP_MSG_KEEPALIVE:
				// if the length of h.holdTimerResetCh
				// isn't zero, the timer will be reset
				// soon anyway.
				select {
				case h.holdTimerResetCh <- true:
				default:
				}
				if m.Header.Type == bgp.BGP_MSG_KEEPALIVE {
					return nil, nil
				}
			case bgp.BGP_MSG_NOTIFICATION:
				body := m.Body.(*bgp.BGPNotification)
				if body.ErrorCode == bgp.BGP_ERROR_CEASE && (body.ErrorSubcode == bgp.BGP_ERROR_SUB_ADMINISTRATIVE_SHUTDOWN || body.ErrorSubcode == bgp.BGP_ERROR_SUB_ADMINISTRATIVE_RESET) {
					communication, rest := decodeAdministrativeCommunication(body.Data)
					h.fsm.lock.RLock()
					log.WithFields(log.Fields{
						"Topic":               "Peer",
						"Key":                 h.fsm.pConf.State.NeighborAddress,
						"Code":                body.ErrorCode,
						"Subcode":             body.ErrorSubcode,
						"Communicated-Reason": communication,
						"Data":                rest,
					}).Warn("received notification")
					h.fsm.lock.RUnlock()
				} else {
					h.fsm.lock.RLock()
					log.WithFields(log.Fields{
						"Topic":   "Peer",
						"Key":     h.fsm.pConf.State.NeighborAddress,
						"Code":    body.ErrorCode,
						"Subcode": body.ErrorSubcode,
						"Data":    body.Data,
					}).Warn("received notification")
					h.fsm.lock.RUnlock()
				}

				h.fsm.lock.RLock()
				s := h.fsm.pConf.GracefulRestart.State
				hardReset := s.Enabled && s.NotificationEnabled && body.ErrorCode == bgp.BGP_ERROR_CEASE && body.ErrorSubcode == bgp.BGP_ERROR_SUB_HARD_RESET
				h.fsm.lock.RUnlock()
				if hardReset {
					sendToStateReasonCh(fsmHardReset, m)
				} else {
					sendToStateReasonCh(fsmNotificationRecv, m)
				}
				return nil, nil
			}
		}
	}
	return fmsg, nil
}

func (h *fsmHandler) recvMessage(ctx context.Context, wg *sync.WaitGroup) error {
	defer func() {
		h.msgCh.Close()
		wg.Done()
	}()
	fmsg, _ := h.recvMessageWithError()
	if fmsg != nil {
		h.msgCh.In() <- fmsg
	}
	return nil
}

func open2Cap(open *bgp.BGPOpen, n *config.Neighbor) (map[bgp.BGPCapabilityCode][]bgp.ParameterCapabilityInterface, map[bgp.RouteFamily]bgp.BGPAddPathMode) {
	capMap := make(map[bgp.BGPCapabilityCode][]bgp.ParameterCapabilityInterface)
	for _, p := range open.OptParams {
		if paramCap, y := p.(*bgp.OptionParameterCapability); y {
			for _, c := range paramCap.Capability {
				m, ok := capMap[c.Code()]
				if !ok {
					m = make([]bgp.ParameterCapabilityInterface, 0, 1)
				}
				capMap[c.Code()] = append(m, c)
			}
		}
	}

	// squash add path cap
	if caps, y := capMap[bgp.BGP_CAP_ADD_PATH]; y {
		items := make([]*bgp.CapAddPathTuple, 0, len(caps))
		for _, c := range caps {
			items = append(items, c.(*bgp.CapAddPath).Tuples...)
		}
		capMap[bgp.BGP_CAP_ADD_PATH] = []bgp.ParameterCapabilityInterface{bgp.NewCapAddPath(items)}
	}

	// remote open message may not include multi-protocol capability
	if _, y := capMap[bgp.BGP_CAP_MULTIPROTOCOL]; !y {
		capMap[bgp.BGP_CAP_MULTIPROTOCOL] = []bgp.ParameterCapabilityInterface{bgp.NewCapMultiProtocol(bgp.RF_IPv4_UC)}
	}

	local := n.CreateRfMap()
	remote := make(map[bgp.RouteFamily]bgp.BGPAddPathMode)
	for _, c := range capMap[bgp.BGP_CAP_MULTIPROTOCOL] {
		family := c.(*bgp.CapMultiProtocol).CapValue
		remote[family] = bgp.BGP_ADD_PATH_NONE
		for _, a := range capMap[bgp.BGP_CAP_ADD_PATH] {
			for _, i := range a.(*bgp.CapAddPath).Tuples {
				if i.RouteFamily == family {
					remote[family] = i.Mode
				}
			}
		}
	}
	negotiated := make(map[bgp.RouteFamily]bgp.BGPAddPathMode)
	for family, mode := range local {
		if m, y := remote[family]; y {
			n := bgp.BGP_ADD_PATH_NONE
			if mode&bgp.BGP_ADD_PATH_SEND > 0 && m&bgp.BGP_ADD_PATH_RECEIVE > 0 {
				n |= bgp.BGP_ADD_PATH_SEND
			}
			if mode&bgp.BGP_ADD_PATH_RECEIVE > 0 && m&bgp.BGP_ADD_PATH_SEND > 0 {
				n |= bgp.BGP_ADD_PATH_RECEIVE
			}
			negotiated[family] = n
		}
	}
	return capMap, negotiated
}

func (h *fsmHandler) opensent(ctx context.Context) (bgp.FSMState, *fsmStateReason) {
	fsm := h.fsm

	fsm.lock.RLock()
	m := buildopen(fsm.gConf, fsm.pConf)
	fsm.lock.RUnlock()

	b, _ := m.Serialize()
	fsm.conn.Write(b)
	fsm.bgpMessageStateUpdate(m.Header.Type, false)

	h.msgCh = channels.NewInfiniteChannel()

	fsm.lock.RLock()
	h.conn = fsm.conn
	fsm.lock.RUnlock()

	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go h.recvMessage(ctx, &wg)

	// RFC 4271 P.60
	// sets its HoldTimer to a large value
	// A HoldTimer value of 4 minutes is suggested as a "large value"
	// for the HoldTimer
	fsm.lock.RLock()
	holdTimer := time.NewTimer(time.Second * time.Duration(fsm.opensentHoldTime))
	fsm.lock.RUnlock()

	for {
		select {
		case <-ctx.Done():
			h.conn.Close()
			return -1, newfsmStateReason(fsmDying, nil, nil)
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			conn.Close()
			fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("Closed an accepted connection")
			fsm.lock.RUnlock()
		case <-fsm.gracefulRestartTimer.C:
			fsm.lock.RLock()
			restarting := fsm.pConf.GracefulRestart.State.PeerRestarting
			fsm.lock.RUnlock()
			if restarting {
				fsm.lock.RLock()
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Warn("graceful restart timer expired")
				fsm.lock.RUnlock()
				h.conn.Close()
				return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmRestartTimerExpired, nil, nil)
			}
		case i, ok := <-h.msgCh.Out():
			if !ok {
				continue
			}
			e := i.(*fsmMsg)
			switch e.MsgData.(type) {
			case *bgp.BGPMessage:
				m := e.MsgData.(*bgp.BGPMessage)
				if m.Header.Type == bgp.BGP_MSG_OPEN {
					fsm.lock.Lock()
					fsm.recvOpen = m
					fsm.lock.Unlock()

					body := m.Body.(*bgp.BGPOpen)

					fsm.lock.RLock()
					fsmPeerAS := fsm.pConf.Config.PeerAs
					fsm.lock.RUnlock()
					peerAs, err := bgp.ValidateOpenMsg(body, fsmPeerAS)
					if err != nil {
						m, _ := fsm.sendNotificationFromErrorMsg(err.(*bgp.MessageError))
						return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmInvalidMsg, m, nil)
					}

					// ASN negotiation was skipped
					fsm.lock.RLock()
					asnNegotiationSkipped := fsm.pConf.Config.PeerAs == 0
					fsm.lock.RUnlock()
					if asnNegotiationSkipped {
						fsm.lock.Lock()
						typ := config.PEER_TYPE_EXTERNAL
						if fsm.peerInfo.LocalAS == peerAs {
							typ = config.PEER_TYPE_INTERNAL
						}
						fsm.pConf.State.PeerType = typ
						log.WithFields(log.Fields{
							"Topic": "Peer",
							"Key":   fsm.pConf.State.NeighborAddress,
							"State": fsm.state.String(),
						}).Infof("skipped asn negotiation: peer-as: %d, peer-type: %s", peerAs, typ)
						fsm.lock.Unlock()
					} else {
						fsm.lock.Lock()
						fsm.pConf.State.PeerType = fsm.pConf.Config.PeerType
						fsm.lock.Unlock()
					}
					fsm.lock.Lock()
					fsm.pConf.State.PeerAs = peerAs
					fsm.peerInfo.AS = peerAs
					fsm.peerInfo.ID = body.ID
					fsm.capMap, fsm.rfMap = open2Cap(body, fsm.pConf)

					if _, y := fsm.capMap[bgp.BGP_CAP_ADD_PATH]; y {
						fsm.marshallingOptions = &bgp.MarshallingOption{
							AddPath: fsm.rfMap,
						}
					} else {
						fsm.marshallingOptions = nil
					}

					// calculate HoldTime
					// RFC 4271 P.13
					// a BGP speaker MUST calculate the value of the Hold Timer
					// by using the smaller of its configured Hold Time and the Hold Time
					// received in the OPEN message.
					holdTime := float64(body.HoldTime)
					myHoldTime := fsm.pConf.Timers.Config.HoldTime
					if holdTime > myHoldTime {
						fsm.pConf.Timers.State.NegotiatedHoldTime = myHoldTime
					} else {
						fsm.pConf.Timers.State.NegotiatedHoldTime = holdTime
					}

					keepalive := fsm.pConf.Timers.Config.KeepaliveInterval
					if n := fsm.pConf.Timers.State.NegotiatedHoldTime; n < myHoldTime {
						keepalive = n / 3
					}
					fsm.pConf.Timers.State.KeepaliveInterval = keepalive

					gr, ok := fsm.capMap[bgp.BGP_CAP_GRACEFUL_RESTART]
					if fsm.pConf.GracefulRestart.Config.Enabled && ok {
						state := &fsm.pConf.GracefulRestart.State
						state.Enabled = true
						cap := gr[len(gr)-1].(*bgp.CapGracefulRestart)
						state.PeerRestartTime = uint16(cap.Time)

						for _, t := range cap.Tuples {
							n := bgp.AddressFamilyNameMap[bgp.AfiSafiToRouteFamily(t.AFI, t.SAFI)]
							for i, a := range fsm.pConf.AfiSafis {
								if string(a.Config.AfiSafiName) == n {
									fsm.pConf.AfiSafis[i].MpGracefulRestart.State.Enabled = true
									fsm.pConf.AfiSafis[i].MpGracefulRestart.State.Received = true
									break
								}
							}
						}

						// RFC 4724 4.1
						// To re-establish the session with its peer, the Restarting Speaker
						// MUST set the "Restart State" bit in the Graceful Restart Capability
						// of the OPEN message.
						if fsm.pConf.GracefulRestart.State.PeerRestarting && cap.Flags&0x08 == 0 {
							log.WithFields(log.Fields{
								"Topic": "Peer",
								"Key":   fsm.pConf.State.NeighborAddress,
								"State": fsm.state.String(),
							}).Warn("restart flag is not set")
							// send notification?
							h.conn.Close()
							fsm.lock.Unlock()
							return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmInvalidMsg, nil, nil)
						}

						// RFC 4724 3
						// The most significant bit is defined as the Restart State (R)
						// bit, ...(snip)... When set (value 1), this bit
						// indicates that the BGP speaker has restarted, and its peer MUST
						// NOT wait for the End-of-RIB marker from the speaker before
						// advertising routing information to the speaker.
						if fsm.pConf.GracefulRestart.State.LocalRestarting && cap.Flags&0x08 != 0 {
							log.WithFields(log.Fields{
								"Topic": "Peer",
								"Key":   fsm.pConf.State.NeighborAddress,
								"State": fsm.state.String(),
							}).Debug("peer has restarted, skipping wait for EOR")
							for i := range fsm.pConf.AfiSafis {
								fsm.pConf.AfiSafis[i].MpGracefulRestart.State.EndOfRibReceived = true
							}
						}
						if fsm.pConf.GracefulRestart.Config.NotificationEnabled && cap.Flags&0x04 > 0 {
							fsm.pConf.GracefulRestart.State.NotificationEnabled = true
						}
					}
					llgr, ok2 := fsm.capMap[bgp.BGP_CAP_LONG_LIVED_GRACEFUL_RESTART]
					if fsm.pConf.GracefulRestart.Config.LongLivedEnabled && ok && ok2 {
						fsm.pConf.GracefulRestart.State.LongLivedEnabled = true
						cap := llgr[len(llgr)-1].(*bgp.CapLongLivedGracefulRestart)
						for _, t := range cap.Tuples {
							n := bgp.AddressFamilyNameMap[bgp.AfiSafiToRouteFamily(t.AFI, t.SAFI)]
							for i, a := range fsm.pConf.AfiSafis {
								if string(a.Config.AfiSafiName) == n {
									fsm.pConf.AfiSafis[i].LongLivedGracefulRestart.State.Enabled = true
									fsm.pConf.AfiSafis[i].LongLivedGracefulRestart.State.Received = true
									fsm.pConf.AfiSafis[i].LongLivedGracefulRestart.State.PeerRestartTime = t.RestartTime
									break
								}
							}
						}
					}

					fsm.lock.Unlock()
					msg := bgp.NewBGPKeepAliveMessage()
					b, _ := msg.Serialize()
					fsm.conn.Write(b)
					fsm.bgpMessageStateUpdate(msg.Header.Type, false)
					return bgp.BGP_FSM_OPENCONFIRM, newfsmStateReason(fsmOpenMsgReceived, nil, nil)
				} else {
					// send notification?
					h.conn.Close()
					return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmInvalidMsg, nil, nil)
				}
			case *bgp.MessageError:
				m, _ := fsm.sendNotificationFromErrorMsg(e.MsgData.(*bgp.MessageError))
				return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmInvalidMsg, m, nil)
			default:
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
					"Data":  e.MsgData,
				}).Panic("unknown msg type")
			}
		case err := <-h.stateReasonCh:
			h.conn.Close()
			return bgp.BGP_FSM_IDLE, &err
		case <-holdTimer.C:
			m, _ := fsm.sendNotification(bgp.BGP_ERROR_HOLD_TIMER_EXPIRED, 0, nil, "hold timer expired")
			return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmHoldTimerExpired, m, nil)
		case stateOp := <-fsm.adminStateCh:
			err := h.changeadminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case adminStateDown:
					h.conn.Close()
					return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmAdminDown, m, nil)
				case adminStateUp:
					log.WithFields(log.Fields{
						"Topic":      "Peer",
						"Key":        fsm.pConf.State.NeighborAddress,
						"State":      fsm.state.String(),
						"adminState": stateOp.State.String(),
					}).Panic("code logic bug")
				}
			}
		}
	}
}

func keepaliveTicker(fsm *fsm) *time.Ticker {
	fsm.lock.RLock()
	defer fsm.lock.RUnlock()

	negotiatedTime := fsm.pConf.Timers.State.NegotiatedHoldTime
	if negotiatedTime == 0 {
		return &time.Ticker{}
	}
	sec := time.Second * time.Duration(fsm.pConf.Timers.State.KeepaliveInterval)
	if sec == 0 {
		sec = time.Second
	}
	return time.NewTicker(sec)
}

func (h *fsmHandler) openconfirm(ctx context.Context) (bgp.FSMState, *fsmStateReason) {
	fsm := h.fsm
	ticker := keepaliveTicker(fsm)
	h.msgCh = channels.NewInfiniteChannel()
	fsm.lock.RLock()
	h.conn = fsm.conn

	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Add(1)
	go h.recvMessage(ctx, &wg)

	var holdTimer *time.Timer
	if fsm.pConf.Timers.State.NegotiatedHoldTime == 0 {
		holdTimer = &time.Timer{}
	} else {
		// RFC 4271 P.65
		// sets the HoldTimer according to the negotiated value
		holdTimer = time.NewTimer(time.Second * time.Duration(fsm.pConf.Timers.State.NegotiatedHoldTime))
	}
	fsm.lock.RUnlock()

	for {
		select {
		case <-ctx.Done():
			h.conn.Close()
			return -1, newfsmStateReason(fsmDying, nil, nil)
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			conn.Close()
			fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("Closed an accepted connection")
			fsm.lock.RUnlock()
		case <-fsm.gracefulRestartTimer.C:
			fsm.lock.RLock()
			restarting := fsm.pConf.GracefulRestart.State.PeerRestarting
			fsm.lock.RUnlock()
			if restarting {
				fsm.lock.RLock()
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Warn("graceful restart timer expired")
				fsm.lock.RUnlock()
				h.conn.Close()
				return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmRestartTimerExpired, nil, nil)
			}
		case <-ticker.C:
			m := bgp.NewBGPKeepAliveMessage()
			b, _ := m.Serialize()
			// TODO: check error
			fsm.conn.Write(b)
			fsm.bgpMessageStateUpdate(m.Header.Type, false)
		case i, ok := <-h.msgCh.Out():
			if !ok {
				continue
			}
			e := i.(*fsmMsg)
			switch e.MsgData.(type) {
			case *bgp.BGPMessage:
				m := e.MsgData.(*bgp.BGPMessage)
				if m.Header.Type == bgp.BGP_MSG_KEEPALIVE {
					return bgp.BGP_FSM_ESTABLISHED, newfsmStateReason(fsmOpenMsgNegotiated, nil, nil)
				}
				// send notification ?
				h.conn.Close()
				return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmInvalidMsg, nil, nil)
			case *bgp.MessageError:
				m, _ := fsm.sendNotificationFromErrorMsg(e.MsgData.(*bgp.MessageError))
				return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmInvalidMsg, m, nil)
			default:
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
					"Data":  e.MsgData,
				}).Panic("unknown msg type")
			}
		case err := <-h.stateReasonCh:
			h.conn.Close()
			return bgp.BGP_FSM_IDLE, &err
		case <-holdTimer.C:
			m, _ := fsm.sendNotification(bgp.BGP_ERROR_HOLD_TIMER_EXPIRED, 0, nil, "hold timer expired")
			return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmHoldTimerExpired, m, nil)
		case stateOp := <-fsm.adminStateCh:
			err := h.changeadminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case adminStateDown:
					h.conn.Close()
					return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmAdminDown, nil, nil)
				case adminStateUp:
					log.WithFields(log.Fields{
						"Topic":      "Peer",
						"Key":        fsm.pConf.State.NeighborAddress,
						"State":      fsm.state.String(),
						"adminState": stateOp.State.String(),
					}).Panic("code logic bug")
				}
			}
		}
	}
}

func (h *fsmHandler) sendMessageloop(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()
	conn := h.conn
	fsm := h.fsm
	ticker := keepaliveTicker(fsm)
	send := func(m *bgp.BGPMessage) error {
		fsm.lock.RLock()
		if fsm.twoByteAsTrans && m.Header.Type == bgp.BGP_MSG_UPDATE {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
				"Data":  m,
			}).Debug("update for 2byte AS peer")
			table.UpdatePathAttrs2ByteAs(m.Body.(*bgp.BGPUpdate))
			table.UpdatePathAggregator2ByteAs(m.Body.(*bgp.BGPUpdate))
		}
		b, err := m.Serialize(h.fsm.marshallingOptions)
		fsm.lock.RUnlock()
		if err != nil {
			fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
				"Data":  err,
			}).Warn("failed to serialize")
			fsm.lock.RUnlock()
			fsm.bgpMessageStateUpdate(0, false)
			return nil
		}
		fsm.lock.RLock()
		err = conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(fsm.pConf.Timers.State.NegotiatedHoldTime)))
		fsm.lock.RUnlock()
		if err != nil {
			h.stateReasonCh <- *newfsmStateReason(fsmWriteFailed, nil, nil)
			conn.Close()
			return fmt.Errorf("failed to set write deadline")
		}
		_, err = conn.Write(b)
		if err != nil {
			fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
				"Data":  err,
			}).Warn("failed to send")
			fsm.lock.RUnlock()
			h.stateReasonCh <- *newfsmStateReason(fsmWriteFailed, nil, nil)
			conn.Close()
			return fmt.Errorf("closed")
		}
		fsm.bgpMessageStateUpdate(m.Header.Type, false)

		switch m.Header.Type {
		case bgp.BGP_MSG_NOTIFICATION:
			body := m.Body.(*bgp.BGPNotification)
			if body.ErrorCode == bgp.BGP_ERROR_CEASE && (body.ErrorSubcode == bgp.BGP_ERROR_SUB_ADMINISTRATIVE_SHUTDOWN || body.ErrorSubcode == bgp.BGP_ERROR_SUB_ADMINISTRATIVE_RESET) {
				communication, rest := decodeAdministrativeCommunication(body.Data)
				fsm.lock.RLock()
				log.WithFields(log.Fields{
					"Topic":               "Peer",
					"Key":                 fsm.pConf.State.NeighborAddress,
					"State":               fsm.state.String(),
					"Code":                body.ErrorCode,
					"Subcode":             body.ErrorSubcode,
					"Communicated-Reason": communication,
					"Data":                rest,
				}).Warn("sent notification")
				fsm.lock.RUnlock()
			} else {
				fsm.lock.RLock()
				log.WithFields(log.Fields{
					"Topic":   "Peer",
					"Key":     fsm.pConf.State.NeighborAddress,
					"State":   fsm.state.String(),
					"Code":    body.ErrorCode,
					"Subcode": body.ErrorSubcode,
					"Data":    body.Data,
				}).Warn("sent notification")
				fsm.lock.RUnlock()
			}
			h.stateReasonCh <- *newfsmStateReason(fsmNotificationSent, m, nil)
			conn.Close()
			return fmt.Errorf("closed")
		case bgp.BGP_MSG_UPDATE:
			update := m.Body.(*bgp.BGPUpdate)
			fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic":       "Peer",
				"Key":         fsm.pConf.State.NeighborAddress,
				"State":       fsm.state.String(),
				"nlri":        update.NLRI,
				"withdrawals": update.WithdrawnRoutes,
				"attributes":  update.PathAttributes,
			}).Debug("sent update")
			fsm.lock.RUnlock()
		default:
			fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
				"data":  m,
			}).Debug("sent")
			fsm.lock.RUnlock()
		}
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case o := <-h.outgoing.Out():
			switch m := o.(type) {
			case *fsmOutgoingMsg:
				h.fsm.lock.RLock()
				options := h.fsm.marshallingOptions
				h.fsm.lock.RUnlock()
				for _, msg := range table.CreateUpdateMsgFromPaths(m.Paths, options) {
					if err := send(msg); err != nil {
						return nil
					}
				}
				if m.Notification != nil {
					if m.StayIdle {
						// current user is only prefix-limit
						// fix me if this is not the case
						h.changeadminState(adminStatePfxCt)
					}
					if err := send(m.Notification); err != nil {
						return nil
					}
				}
			default:
				return nil
			}
		case <-ticker.C:
			if err := send(bgp.NewBGPKeepAliveMessage()); err != nil {
				return nil
			}
		}
	}
}

func (h *fsmHandler) recvMessageloop(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()
	for {
		fmsg, err := h.recvMessageWithError()
		if fmsg != nil {
			h.msgCh.In() <- fmsg
		}
		if err != nil {
			return nil
		}
	}
}

func (h *fsmHandler) established(ctx context.Context) (bgp.FSMState, *fsmStateReason) {
	var wg sync.WaitGroup
	fsm := h.fsm
	fsm.lock.Lock()
	h.conn = fsm.conn
	fsm.lock.Unlock()

	defer wg.Wait()
	wg.Add(2)

	go h.sendMessageloop(ctx, &wg)
	h.msgCh = h.incoming
	go h.recvMessageloop(ctx, &wg)

	var holdTimer *time.Timer
	if fsm.pConf.Timers.State.NegotiatedHoldTime == 0 {
		holdTimer = &time.Timer{}
	} else {
		fsm.lock.RLock()
		holdTimer = time.NewTimer(time.Second * time.Duration(fsm.pConf.Timers.State.NegotiatedHoldTime))
		fsm.lock.RUnlock()
	}

	fsm.gracefulRestartTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			h.conn.Close()
			return -1, newfsmStateReason(fsmDying, nil, nil)
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			conn.Close()
			fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("Closed an accepted connection")
			fsm.lock.RUnlock()
		case err := <-h.stateReasonCh:
			h.conn.Close()
			// if recv goroutine hit an error and sent to
			// stateReasonCh, then tx goroutine might take
			// long until it exits because it waits for
			// ctx.Done() or keepalive timer. So let kill
			// it now.
			h.outgoing.In() <- err
			fsm.lock.RLock()
			if s := fsm.pConf.GracefulRestart.State; s.Enabled &&
				(s.NotificationEnabled && err.Type == fsmNotificationRecv ||
					err.Type == fsmReadFailed ||
					err.Type == fsmWriteFailed) {
				err = *newfsmStateReason(fsmGracefulRestart, nil, nil)
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Info("peer graceful restart")
				fsm.gracefulRestartTimer.Reset(time.Duration(fsm.pConf.GracefulRestart.State.PeerRestartTime) * time.Second)
			}
			fsm.lock.RUnlock()
			return bgp.BGP_FSM_IDLE, &err
		case <-holdTimer.C:
			fsm.lock.RLock()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("hold timer expired")
			fsm.lock.RUnlock()
			m := bgp.NewBGPNotificationMessage(bgp.BGP_ERROR_HOLD_TIMER_EXPIRED, 0, nil)
			h.outgoing.In() <- &fsmOutgoingMsg{Notification: m}
			return bgp.BGP_FSM_IDLE, newfsmStateReason(fsmHoldTimerExpired, m, nil)
		case <-h.holdTimerResetCh:
			fsm.lock.RLock()
			if fsm.pConf.Timers.State.NegotiatedHoldTime != 0 {
				holdTimer.Reset(time.Second * time.Duration(fsm.pConf.Timers.State.NegotiatedHoldTime))
			}
			fsm.lock.RUnlock()
		case stateOp := <-fsm.adminStateCh:
			err := h.changeadminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case adminStateDown:
					m := bgp.NewBGPNotificationMessage(bgp.BGP_ERROR_CEASE, bgp.BGP_ERROR_SUB_ADMINISTRATIVE_SHUTDOWN, stateOp.Communication)
					h.outgoing.In() <- &fsmOutgoingMsg{Notification: m}
				}
			}
		}
	}
}

func (h *fsmHandler) loop(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()

	fsm := h.fsm
	fsm.lock.RLock()
	oldState := fsm.state
	fsm.lock.RUnlock()

	var reason *fsmStateReason
	nextState := bgp.FSMState(-1)
	fsm.lock.RLock()
	fsmState := fsm.state
	fsm.lock.RUnlock()

	switch fsmState {
	case bgp.BGP_FSM_IDLE:
		nextState, reason = h.idle(ctx)
		// case bgp.BGP_FSM_CONNECT:
		// 	nextState = h.connect()
	case bgp.BGP_FSM_ACTIVE:
		nextState, reason = h.active(ctx)
	case bgp.BGP_FSM_OPENSENT:
		nextState, reason = h.opensent(ctx)
	case bgp.BGP_FSM_OPENCONFIRM:
		nextState, reason = h.openconfirm(ctx)
	case bgp.BGP_FSM_ESTABLISHED:
		nextState, reason = h.established(ctx)
	}

	fsm.lock.RLock()
	fsm.reason = reason

	if nextState == bgp.BGP_FSM_ESTABLISHED && oldState == bgp.BGP_FSM_OPENCONFIRM {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   fsm.pConf.State.NeighborAddress,
			"State": fsm.state.String(),
		}).Info("Peer Up")
	}

	if oldState == bgp.BGP_FSM_ESTABLISHED {
		// The main goroutine sent the notificaiton due to
		// deconfiguration or something.
		reason := fsm.reason
		if fsm.h.sentNotification != nil {
			reason.Type = fsmNotificationSent
			reason.peerDownReason = peerDownByLocal
			reason.BGPNotification = fsm.h.sentNotification
		}
		log.WithFields(log.Fields{
			"Topic":  "Peer",
			"Key":    fsm.pConf.State.NeighborAddress,
			"State":  fsm.state.String(),
			"Reason": reason.String(),
		}).Info("Peer Down")
	}
	fsm.lock.RUnlock()

	// under zero means that the context was canceled.
	if nextState >= bgp.BGP_FSM_IDLE {
		fsm.lock.RLock()
		h.stateCh <- &fsmMsg{
			MsgType:     fsmMsgStateChange,
			MsgSrc:      fsm.pConf.State.NeighborAddress,
			MsgData:     nextState,
			StateReason: reason,
			Version:     h.fsm.version,
		}
		fsm.lock.RUnlock()
	}
	return nil
}

func (h *fsmHandler) changeadminState(s adminState) error {
	h.fsm.lock.Lock()
	defer h.fsm.lock.Unlock()

	fsm := h.fsm
	if fsm.adminState != s {
		log.WithFields(log.Fields{
			"Topic":      "Peer",
			"Key":        fsm.pConf.State.NeighborAddress,
			"State":      fsm.state.String(),
			"adminState": s.String(),
		}).Debug("admin state changed")

		fsm.adminState = s
		fsm.pConf.State.AdminDown = !fsm.pConf.State.AdminDown

		switch s {
		case adminStateUp:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Info("Administrative start")
		case adminStateDown:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Info("Administrative shutdown")
		case adminStatePfxCt:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Info("Administrative shutdown(Prefix limit reached)")
		}

	} else {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   fsm.pConf.State.NeighborAddress,
			"State": fsm.state.String(),
		}).Warn("cannot change to the same state")

		return fmt.Errorf("cannot change to the same state.")
	}
	return nil
}
