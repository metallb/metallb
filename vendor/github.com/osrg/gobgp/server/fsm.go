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
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/eapache/channels"
	log "github.com/sirupsen/logrus"
	"gopkg.in/tomb.v2"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/packet/bmp"
	"github.com/osrg/gobgp/table"
)

type FsmStateReason string

const (
	FSM_DYING                   = "dying"
	FSM_ADMIN_DOWN              = "admin-down"
	FSM_READ_FAILED             = "read-failed"
	FSM_WRITE_FAILED            = "write-failed"
	FSM_NOTIFICATION_SENT       = "notification-sent"
	FSM_NOTIFICATION_RECV       = "notification-received"
	FSM_HOLD_TIMER_EXPIRED      = "hold-timer-expired"
	FSM_IDLE_HOLD_TIMER_EXPIRED = "idle-hold-timer-expired"
	FSM_RESTART_TIMER_EXPIRED   = "restart-timer-expired"
	FSM_GRACEFUL_RESTART        = "graceful-restart"
	FSM_INVALID_MSG             = "invalid-msg"
	FSM_NEW_CONNECTION          = "new-connection"
	FSM_OPEN_MSG_RECEIVED       = "open-msg-received"
	FSM_OPEN_MSG_NEGOTIATED     = "open-msg-negotiated"
	FSM_HARD_RESET              = "hard-reset"
)

type FsmMsgType int

const (
	_ FsmMsgType = iota
	FSM_MSG_STATE_CHANGE
	FSM_MSG_BGP_MESSAGE
	FSM_MSG_ROUTE_REFRESH
)

type FsmMsg struct {
	MsgType   FsmMsgType
	MsgSrc    string
	MsgData   interface{}
	PathList  []*table.Path
	timestamp time.Time
	payload   []byte
	Version   uint
}

type FsmOutgoingMsg struct {
	Paths        []*table.Path
	Notification *bgp.BGPMessage
	StayIdle     bool
}

const (
	HOLDTIME_OPENSENT = 240
	HOLDTIME_IDLE     = 5
)

type AdminState int

const (
	ADMIN_STATE_UP AdminState = iota
	ADMIN_STATE_DOWN
	ADMIN_STATE_PFX_CT
)

func (s AdminState) String() string {
	switch s {
	case ADMIN_STATE_UP:
		return "ADMIN_STATE_UP"
	case ADMIN_STATE_DOWN:
		return "ADMIN_STATE_DOWN"
	case ADMIN_STATE_PFX_CT:
		return "ADMIN_STATE_PFX_CT"
	default:
		return "Unknown"
	}
}

type AdminStateOperation struct {
	State         AdminState
	Communication []byte
}

var fsmVersion uint

type FSM struct {
	t                    tomb.Tomb
	gConf                *config.Global
	pConf                *config.Neighbor
	state                bgp.FSMState
	reason               FsmStateReason
	conn                 net.Conn
	connCh               chan net.Conn
	idleHoldTime         float64
	opensentHoldTime     float64
	adminState           AdminState
	adminStateCh         chan AdminStateOperation
	getActiveCh          chan struct{}
	h                    *FSMHandler
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

func (fsm *FSM) bgpMessageStateUpdate(MessageType uint8, isIn bool) {
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

func (fsm *FSM) bmpStatsUpdate(statType uint16, increment int) {
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

func NewFSM(gConf *config.Global, pConf *config.Neighbor, policy *table.RoutingPolicy) *FSM {
	adminState := ADMIN_STATE_UP
	if pConf.Config.AdminDown {
		adminState = ADMIN_STATE_DOWN
	}
	pConf.State.SessionState = config.IntToSessionStateMap[int(bgp.BGP_FSM_IDLE)]
	pConf.Timers.State.Downtime = time.Now().Unix()
	fsmVersion++
	fsm := &FSM{
		gConf:                gConf,
		pConf:                pConf,
		state:                bgp.BGP_FSM_IDLE,
		connCh:               make(chan net.Conn, 1),
		opensentHoldTime:     float64(HOLDTIME_OPENSENT),
		adminState:           adminState,
		adminStateCh:         make(chan AdminStateOperation, 1),
		getActiveCh:          make(chan struct{}),
		rfMap:                make(map[bgp.RouteFamily]bgp.BGPAddPathMode),
		capMap:               make(map[bgp.BGPCapabilityCode][]bgp.ParameterCapabilityInterface),
		peerInfo:             table.NewPeerInfo(gConf, pConf),
		policy:               policy,
		gracefulRestartTimer: time.NewTimer(time.Hour),
		version:              fsmVersion,
	}
	fsm.gracefulRestartTimer.Stop()
	fsm.t.Go(fsm.connectLoop)
	return fsm
}

func (fsm *FSM) StateChange(nextState bgp.FSMState) {
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
	case bgp.BGP_FSM_ACTIVE:
		if !fsm.pConf.Transport.Config.PassiveMode {
			fsm.getActiveCh <- struct{}{}
		}
		fallthrough
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

func (fsm *FSM) RemoteHostPort() (string, uint16) {
	return hostport(fsm.conn.RemoteAddr())

}

func (fsm *FSM) LocalHostPort() (string, uint16) {
	return hostport(fsm.conn.LocalAddr())
}

func (fsm *FSM) sendNotificationFromErrorMsg(e *bgp.MessageError) error {
	if fsm.h != nil && fsm.h.conn != nil {
		m := bgp.NewBGPNotificationMessage(e.TypeCode, e.SubTypeCode, e.Data)
		b, _ := m.Serialize()
		_, err := fsm.h.conn.Write(b)
		if err == nil {
			fsm.bgpMessageStateUpdate(m.Header.Type, false)
			fsm.h.sentNotification = bgp.NewNotificationErrorCode(e.TypeCode, e.SubTypeCode).String()
		}
		fsm.h.conn.Close()
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   fsm.pConf.State.NeighborAddress,
			"Data":  e,
		}).Warn("sent notification")
		return nil
	}
	return fmt.Errorf("can't send notification to %s since TCP connection is not established", fsm.pConf.State.NeighborAddress)
}

func (fsm *FSM) sendNotification(code, subType uint8, data []byte, msg string) error {
	e := bgp.NewMessageError(code, subType, data, msg)
	return fsm.sendNotificationFromErrorMsg(e.(*bgp.MessageError))
}

func (fsm *FSM) connectLoop() error {
	tick := int(fsm.pConf.Timers.Config.ConnectRetry)
	if tick < MIN_CONNECT_RETRY {
		tick = MIN_CONNECT_RETRY
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	timer := time.NewTimer(time.Duration(tick) * time.Second)
	timer.Stop()

	connect := func() {
		addr := fsm.pConf.State.NeighborAddress
		port := int(bgp.BGP_PORT)
		if fsm.pConf.Transport.Config.RemotePort != 0 {
			port = int(fsm.pConf.Transport.Config.RemotePort)
		}
		laddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(fsm.pConf.Transport.Config.LocalAddress, "0"))
		if err != nil {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
			}).Warn("failed to resolve local address: %s", err)
			return
		}
		var conn net.Conn
		d := TCPDialer{
			Dialer: net.Dialer{
				LocalAddr: laddr,
				Timeout:   time.Duration(MIN_CONNECT_RETRY-1) * time.Second,
			},
			AuthPassword: fsm.pConf.Config.AuthPassword,
		}
		if fsm.pConf.TtlSecurity.Config.Enabled {
			d.Ttl = 255
			d.TtlMin = fsm.pConf.TtlSecurity.Config.TtlMin
		} else if fsm.pConf.Config.PeerAs != 0 && fsm.pConf.Config.PeerType == config.PEER_TYPE_EXTERNAL {
			d.Ttl = 1
			if fsm.pConf.EbgpMultihop.Config.Enabled {
				d.Ttl = fsm.pConf.EbgpMultihop.Config.MultihopTtl
			}
		}
		conn, err = d.DialTCP(addr, port)
		if err == nil {
			select {
			case fsm.connCh <- conn:
				return
			default:
				conn.Close()
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
				}).Warn("active conn is closed to avoid being blocked")
			}
		} else {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
			}).Debugf("failed to connect: %s", err)
		}

		if fsm.state == bgp.BGP_FSM_ACTIVE && !fsm.pConf.GracefulRestart.State.PeerRestarting {
			timer.Reset(time.Duration(tick) * time.Second)
		}
	}

	for {
		select {
		case <-fsm.t.Dying():
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
			}).Debug("stop connect loop")
			return nil
		case <-timer.C:
			if fsm.state == bgp.BGP_FSM_ACTIVE && !fsm.pConf.GracefulRestart.State.PeerRestarting {
				go connect()
			}
		case <-fsm.getActiveCh:
			timer.Reset(time.Duration(r.Intn(MIN_CONNECT_RETRY)+MIN_CONNECT_RETRY) * time.Second)
		}
	}
}

type FSMHandler struct {
	t                tomb.Tomb
	fsm              *FSM
	conn             net.Conn
	msgCh            *channels.InfiniteChannel
	errorCh          chan FsmStateReason
	incoming         *channels.InfiniteChannel
	stateCh          chan *FsmMsg
	outgoing         *channels.InfiniteChannel
	holdTimerResetCh chan bool
	sentNotification string
}

func NewFSMHandler(fsm *FSM, incoming *channels.InfiniteChannel, stateCh chan *FsmMsg, outgoing *channels.InfiniteChannel) *FSMHandler {
	h := &FSMHandler{
		fsm:              fsm,
		errorCh:          make(chan FsmStateReason, 2),
		incoming:         incoming,
		stateCh:          stateCh,
		outgoing:         outgoing,
		holdTimerResetCh: make(chan bool, 2),
	}
	fsm.t.Go(h.loop)
	return h
}

func (h *FSMHandler) idle() (bgp.FSMState, FsmStateReason) {
	fsm := h.fsm

	idleHoldTimer := time.NewTimer(time.Second * time.Duration(fsm.idleHoldTime))
	for {
		select {
		case <-h.t.Dying():
			return -1, FSM_DYING
		case <-fsm.gracefulRestartTimer.C:
			if fsm.pConf.GracefulRestart.State.PeerRestarting {
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Warn("graceful restart timer expired")
				return bgp.BGP_FSM_IDLE, FSM_RESTART_TIMER_EXPIRED
			}
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			conn.Close()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("Closed an accepted connection")
		case <-idleHoldTimer.C:

			if fsm.adminState == ADMIN_STATE_UP {
				log.WithFields(log.Fields{
					"Topic":    "Peer",
					"Key":      fsm.pConf.State.NeighborAddress,
					"Duration": fsm.idleHoldTime,
				}).Debug("IdleHoldTimer expired")
				fsm.idleHoldTime = HOLDTIME_IDLE
				return bgp.BGP_FSM_ACTIVE, FSM_IDLE_HOLD_TIMER_EXPIRED

			} else {
				log.WithFields(log.Fields{"Topic": "Peer"}).Debug("IdleHoldTimer expired, but stay at idle because the admin state is DOWN")
			}

		case stateOp := <-fsm.adminStateCh:
			err := h.changeAdminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case ADMIN_STATE_DOWN:
					// stop idle hold timer
					idleHoldTimer.Stop()

				case ADMIN_STATE_UP:
					// restart idle hold timer
					idleHoldTimer.Reset(time.Second * time.Duration(fsm.idleHoldTime))
				}
			}
		}
	}
}

func (h *FSMHandler) active() (bgp.FSMState, FsmStateReason) {
	fsm := h.fsm
	for {
		select {
		case <-h.t.Dying():
			return -1, FSM_DYING
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			fsm.conn = conn
			ttl := 0
			ttlMin := 0
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
				if err := SetTcpTTLSockopt(conn.(*net.TCPConn), ttl); err != nil {
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   fsm.pConf.Config.NeighborAddress,
						"State": fsm.state.String(),
					}).Warnf("cannot set TTL(=%d) for peer: %s", ttl, err)
				}
			}
			if ttlMin != 0 {
				if err := SetTcpMinTTLSockopt(conn.(*net.TCPConn), ttlMin); err != nil {
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   fsm.pConf.Config.NeighborAddress,
						"State": fsm.state.String(),
					}).Warnf("cannot set minimal TTL(=%d) for peer: %s", ttl, err)
				}
			}
			// we don't implement delayed open timer so move to opensent right
			// away.
			return bgp.BGP_FSM_OPENSENT, FSM_NEW_CONNECTION
		case <-fsm.gracefulRestartTimer.C:
			if fsm.pConf.GracefulRestart.State.PeerRestarting {
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Warn("graceful restart timer expired")
				return bgp.BGP_FSM_IDLE, FSM_RESTART_TIMER_EXPIRED
			}
		case err := <-h.errorCh:
			return bgp.BGP_FSM_IDLE, err
		case stateOp := <-fsm.adminStateCh:
			err := h.changeAdminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case ADMIN_STATE_DOWN:
					return bgp.BGP_FSM_IDLE, FSM_ADMIN_DOWN
				case ADMIN_STATE_UP:
					log.WithFields(log.Fields{
						"Topic":      "Peer",
						"Key":        fsm.pConf.State.NeighborAddress,
						"State":      fsm.state.String(),
						"AdminState": stateOp.State.String(),
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

func hasOwnASLoop(ownAS uint32, limit int, aspath *bgp.PathAttributeAsPath) bool {
	cnt := 0
	for _, i := range aspath.Value {
		for _, as := range i.(*bgp.As4PathParam).AS {
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

func (h *FSMHandler) afiSafiDisable(rf bgp.RouteFamily) string {
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

func (h *FSMHandler) handlingError(m *bgp.BGPMessage, e error, useRevisedError bool) bgp.ErrorHandling {
	handling := bgp.ERROR_HANDLING_NONE
	if m.Header.Type == bgp.BGP_MSG_UPDATE && useRevisedError {
		factor := e.(*bgp.MessageError)
		handling = factor.ErrorHandling
		switch handling {
		case bgp.ERROR_HANDLING_ATTRIBUTE_DISCARD:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   h.fsm.pConf.State.NeighborAddress,
				"State": h.fsm.state.String(),
				"error": e,
			}).Warn("Some attributes were discarded")
		case bgp.ERROR_HANDLING_TREAT_AS_WITHDRAW:
			m.Body = bgp.TreatAsWithdraw(m.Body.(*bgp.BGPUpdate))
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   h.fsm.pConf.State.NeighborAddress,
				"State": h.fsm.state.String(),
				"error": e,
			}).Warn("the received Update message was treated as withdraw")
		case bgp.ERROR_HANDLING_AFISAFI_DISABLE:
			rf := extractRouteFamily(factor.ErrorAttribute)
			if rf == nil {
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   h.fsm.pConf.State.NeighborAddress,
					"State": h.fsm.state.String(),
				}).Warn("Error occurred during AFI/SAFI disabling")
			} else {
				n := h.afiSafiDisable(*rf)
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   h.fsm.pConf.State.NeighborAddress,
					"State": h.fsm.state.String(),
					"error": e,
				}).Warnf("Capability %s was disabled", n)
			}
		}
	} else {
		handling = bgp.ERROR_HANDLING_SESSION_RESET
	}
	return handling
}

func (h *FSMHandler) recvMessageWithError() (*FsmMsg, error) {
	sendToErrorCh := func(reason FsmStateReason) {
		// probably doesn't happen but be cautious
		select {
		case h.errorCh <- reason:
		default:
		}
	}

	headerBuf, err := readAll(h.conn, bgp.BGP_HEADER_LENGTH)
	if err != nil {
		sendToErrorCh(FSM_READ_FAILED)
		return nil, err
	}

	hd := &bgp.BGPHeader{}
	err = hd.DecodeFromBytes(headerBuf)
	if err != nil {
		h.fsm.bgpMessageStateUpdate(0, true)
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   h.fsm.pConf.State.NeighborAddress,
			"State": h.fsm.state.String(),
			"error": err,
		}).Warn("Session will be reset due to malformed BGP Header")
		fmsg := &FsmMsg{
			MsgType: FSM_MSG_BGP_MESSAGE,
			MsgSrc:  h.fsm.pConf.State.NeighborAddress,
			MsgData: err,
			Version: h.fsm.version,
		}
		return fmsg, err
	}

	bodyBuf, err := readAll(h.conn, int(hd.Len)-bgp.BGP_HEADER_LENGTH)
	if err != nil {
		sendToErrorCh(FSM_READ_FAILED)
		return nil, err
	}

	now := time.Now()
	useRevisedError := h.fsm.pConf.ErrorHandling.Config.TreatAsWithdraw
	handling := bgp.ERROR_HANDLING_NONE

	m, err := bgp.ParseBGPBody(hd, bodyBuf, h.fsm.marshallingOptions)
	if err != nil {
		handling = h.handlingError(m, err, useRevisedError)
		h.fsm.bgpMessageStateUpdate(0, true)
	} else {
		h.fsm.bgpMessageStateUpdate(m.Header.Type, true)
		err = bgp.ValidateBGPMessage(m)
	}
	fmsg := &FsmMsg{
		MsgType:   FSM_MSG_BGP_MESSAGE,
		MsgSrc:    h.fsm.pConf.State.NeighborAddress,
		timestamp: now,
		Version:   h.fsm.version,
	}

	switch handling {
	case bgp.ERROR_HANDLING_AFISAFI_DISABLE:
		fmsg.MsgData = m
		return fmsg, nil
	case bgp.ERROR_HANDLING_SESSION_RESET:
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   h.fsm.pConf.State.NeighborAddress,
			"State": h.fsm.state.String(),
			"error": err,
		}).Warn("Session will be reset due to malformed BGP message")
		fmsg.MsgData = err
		return fmsg, err
	default:
		fmsg.MsgData = m
		if h.fsm.state == bgp.BGP_FSM_ESTABLISHED {
			switch m.Header.Type {
			case bgp.BGP_MSG_ROUTE_REFRESH:
				fmsg.MsgType = FSM_MSG_ROUTE_REFRESH
			case bgp.BGP_MSG_UPDATE:
				body := m.Body.(*bgp.BGPUpdate)
				isEBGP := h.fsm.pConf.IsEBGPPeer(h.fsm.gConf)
				isConfed := h.fsm.pConf.IsConfederationMember(h.fsm.gConf)

				fmsg.payload = make([]byte, len(headerBuf)+len(bodyBuf))
				copy(fmsg.payload, headerBuf)
				copy(fmsg.payload[len(headerBuf):], bodyBuf)

				ok, err := bgp.ValidateUpdateMsg(body, h.fsm.rfMap, isEBGP, isConfed)
				if !ok {
					handling = h.handlingError(m, err, useRevisedError)
				}
				if handling == bgp.ERROR_HANDLING_SESSION_RESET {
					log.WithFields(log.Fields{
						"Topic": "Peer",
						"Key":   h.fsm.pConf.State.NeighborAddress,
						"State": h.fsm.state.String(),
						"error": err,
					}).Warn("Session will be reset due to malformed BGP update message")
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

				// RFC4271 9.1.2 Phase 2: Route Selection
				//
				// If the AS_PATH attribute of a BGP route contains an AS loop, the BGP
				// route should be excluded from the Phase 2 decision function.
				var asLoop bool
				if attr := getPathAttrFromBGPUpdate(body, bgp.BGP_ATTR_TYPE_AS_PATH); attr != nil {
					asLoop = hasOwnASLoop(h.fsm.peerInfo.LocalAS, int(h.fsm.pConf.AsPathOptions.Config.AllowOwnAs), attr.(*bgp.PathAttributeAsPath))
				}

				fmsg.PathList = table.ProcessMessage(m, h.fsm.peerInfo, fmsg.timestamp)
				id := h.fsm.pConf.State.NeighborAddress
				for _, path := range fmsg.PathList {
					if path.IsEOR() {
						continue
					}
					if asLoop || (h.fsm.policy.ApplyPolicy(id, table.POLICY_DIRECTION_IN, path, nil) == nil) {
						path.Filter(id, table.POLICY_DIRECTION_IN)
					}
				}
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
					log.WithFields(log.Fields{
						"Topic":               "Peer",
						"Key":                 h.fsm.pConf.State.NeighborAddress,
						"Code":                body.ErrorCode,
						"Subcode":             body.ErrorSubcode,
						"Communicated-Reason": communication,
						"Data":                rest,
					}).Warn("received notification")
				} else {
					log.WithFields(log.Fields{
						"Topic":   "Peer",
						"Key":     h.fsm.pConf.State.NeighborAddress,
						"Code":    body.ErrorCode,
						"Subcode": body.ErrorSubcode,
						"Data":    body.Data,
					}).Warn("received notification")
				}

				if s := h.fsm.pConf.GracefulRestart.State; s.Enabled && s.NotificationEnabled && body.ErrorCode == bgp.BGP_ERROR_CEASE && body.ErrorSubcode == bgp.BGP_ERROR_SUB_HARD_RESET {
					sendToErrorCh(FSM_HARD_RESET)
				} else {
					sendToErrorCh(FsmStateReason(fmt.Sprintf("%s %s", FSM_NOTIFICATION_RECV, bgp.NewNotificationErrorCode(body.ErrorCode, body.ErrorSubcode).String())))
				}
				return nil, nil
			}
		}
	}
	return fmsg, nil
}

func (h *FSMHandler) recvMessage() error {
	defer h.msgCh.Close()
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
			for _, i := range c.(*bgp.CapAddPath).Tuples {
				items = append(items, i)
			}
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

func (h *FSMHandler) opensent() (bgp.FSMState, FsmStateReason) {
	fsm := h.fsm
	m := buildopen(fsm.gConf, fsm.pConf)
	b, _ := m.Serialize()
	fsm.conn.Write(b)
	fsm.bgpMessageStateUpdate(m.Header.Type, false)

	h.msgCh = channels.NewInfiniteChannel()
	h.conn = fsm.conn

	h.t.Go(h.recvMessage)

	// RFC 4271 P.60
	// sets its HoldTimer to a large value
	// A HoldTimer value of 4 minutes is suggested as a "large value"
	// for the HoldTimer
	holdTimer := time.NewTimer(time.Second * time.Duration(fsm.opensentHoldTime))

	for {
		select {
		case <-h.t.Dying():
			h.conn.Close()
			return -1, FSM_DYING
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			conn.Close()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("Closed an accepted connection")
		case <-fsm.gracefulRestartTimer.C:
			if fsm.pConf.GracefulRestart.State.PeerRestarting {
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Warn("graceful restart timer expired")
				h.conn.Close()
				return bgp.BGP_FSM_IDLE, FSM_RESTART_TIMER_EXPIRED
			}
		case i, ok := <-h.msgCh.Out():
			if !ok {
				continue
			}
			e := i.(*FsmMsg)
			switch e.MsgData.(type) {
			case *bgp.BGPMessage:
				m := e.MsgData.(*bgp.BGPMessage)
				if m.Header.Type == bgp.BGP_MSG_OPEN {
					fsm.recvOpen = m
					body := m.Body.(*bgp.BGPOpen)
					peerAs, err := bgp.ValidateOpenMsg(body, fsm.pConf.Config.PeerAs)
					if err != nil {
						fsm.sendNotificationFromErrorMsg(err.(*bgp.MessageError))
						return bgp.BGP_FSM_IDLE, FSM_INVALID_MSG
					}

					// ASN negotiation was skipped
					if fsm.pConf.Config.PeerAs == 0 {
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
					} else {
						fsm.pConf.State.PeerType = fsm.pConf.Config.PeerType
					}
					fsm.pConf.State.PeerAs = peerAs
					fsm.peerInfo.AS = peerAs
					fsm.peerInfo.ID = body.ID
					fsm.capMap, fsm.rfMap = open2Cap(body, fsm.pConf)

					if _, y := fsm.capMap[bgp.BGP_CAP_ADD_PATH]; y {
						fsm.marshallingOptions = &bgp.MarshallingOption{
							AddPath: fsm.rfMap,
						}
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
							return bgp.BGP_FSM_IDLE, FSM_INVALID_MSG
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

					msg := bgp.NewBGPKeepAliveMessage()
					b, _ := msg.Serialize()
					fsm.conn.Write(b)
					fsm.bgpMessageStateUpdate(msg.Header.Type, false)
					return bgp.BGP_FSM_OPENCONFIRM, FSM_OPEN_MSG_RECEIVED
				} else {
					// send notification?
					h.conn.Close()
					return bgp.BGP_FSM_IDLE, FSM_INVALID_MSG
				}
			case *bgp.MessageError:
				fsm.sendNotificationFromErrorMsg(e.MsgData.(*bgp.MessageError))
				return bgp.BGP_FSM_IDLE, FSM_INVALID_MSG
			default:
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
					"Data":  e.MsgData,
				}).Panic("unknown msg type")
			}
		case err := <-h.errorCh:
			h.conn.Close()
			return bgp.BGP_FSM_IDLE, err
		case <-holdTimer.C:
			fsm.sendNotification(bgp.BGP_ERROR_HOLD_TIMER_EXPIRED, 0, nil, "hold timer expired")
			h.t.Kill(nil)
			return bgp.BGP_FSM_IDLE, FSM_HOLD_TIMER_EXPIRED
		case stateOp := <-fsm.adminStateCh:
			err := h.changeAdminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case ADMIN_STATE_DOWN:
					h.conn.Close()
					return bgp.BGP_FSM_IDLE, FSM_ADMIN_DOWN
				case ADMIN_STATE_UP:
					log.WithFields(log.Fields{
						"Topic":      "Peer",
						"Key":        fsm.pConf.State.NeighborAddress,
						"State":      fsm.state.String(),
						"AdminState": stateOp.State.String(),
					}).Panic("code logic bug")
				}
			}
		}
	}
}

func keepaliveTicker(fsm *FSM) *time.Ticker {
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

func (h *FSMHandler) openconfirm() (bgp.FSMState, FsmStateReason) {
	fsm := h.fsm
	ticker := keepaliveTicker(fsm)
	h.msgCh = channels.NewInfiniteChannel()
	h.conn = fsm.conn

	h.t.Go(h.recvMessage)

	var holdTimer *time.Timer
	if fsm.pConf.Timers.State.NegotiatedHoldTime == 0 {
		holdTimer = &time.Timer{}
	} else {
		// RFC 4271 P.65
		// sets the HoldTimer according to the negotiated value
		holdTimer = time.NewTimer(time.Second * time.Duration(fsm.pConf.Timers.State.NegotiatedHoldTime))
	}

	for {
		select {
		case <-h.t.Dying():
			h.conn.Close()
			return -1, FSM_DYING
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			conn.Close()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("Closed an accepted connection")
		case <-fsm.gracefulRestartTimer.C:
			if fsm.pConf.GracefulRestart.State.PeerRestarting {
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Warn("graceful restart timer expired")
				h.conn.Close()
				return bgp.BGP_FSM_IDLE, FSM_RESTART_TIMER_EXPIRED
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
			e := i.(*FsmMsg)
			switch e.MsgData.(type) {
			case *bgp.BGPMessage:
				m := e.MsgData.(*bgp.BGPMessage)
				if m.Header.Type == bgp.BGP_MSG_KEEPALIVE {
					return bgp.BGP_FSM_ESTABLISHED, FSM_OPEN_MSG_NEGOTIATED
				}
				// send notification ?
				h.conn.Close()
				return bgp.BGP_FSM_IDLE, FSM_INVALID_MSG
			case *bgp.MessageError:
				fsm.sendNotificationFromErrorMsg(e.MsgData.(*bgp.MessageError))
				return bgp.BGP_FSM_IDLE, FSM_INVALID_MSG
			default:
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
					"Data":  e.MsgData,
				}).Panic("unknown msg type")
			}
		case err := <-h.errorCh:
			h.conn.Close()
			return bgp.BGP_FSM_IDLE, err
		case <-holdTimer.C:
			fsm.sendNotification(bgp.BGP_ERROR_HOLD_TIMER_EXPIRED, 0, nil, "hold timer expired")
			h.t.Kill(nil)
			return bgp.BGP_FSM_IDLE, FSM_HOLD_TIMER_EXPIRED
		case stateOp := <-fsm.adminStateCh:
			err := h.changeAdminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case ADMIN_STATE_DOWN:
					h.conn.Close()
					return bgp.BGP_FSM_IDLE, FSM_ADMIN_DOWN
				case ADMIN_STATE_UP:
					log.WithFields(log.Fields{
						"Topic":      "Peer",
						"Key":        fsm.pConf.State.NeighborAddress,
						"State":      fsm.state.String(),
						"AdminState": stateOp.State.String(),
					}).Panic("code logic bug")
				}
			}
		}
	}
}

func (h *FSMHandler) sendMessageloop() error {
	conn := h.conn
	fsm := h.fsm
	ticker := keepaliveTicker(fsm)
	send := func(m *bgp.BGPMessage) error {
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
		if err != nil {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
				"Data":  err,
			}).Warn("failed to serialize")
			fsm.bgpMessageStateUpdate(0, false)
			return nil
		}
		if err := conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(fsm.pConf.Timers.State.NegotiatedHoldTime))); err != nil {
			h.errorCh <- FSM_WRITE_FAILED
			conn.Close()
			return fmt.Errorf("failed to set write deadline")
		}
		_, err = conn.Write(b)
		if err != nil {
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
				"Data":  err,
			}).Warn("failed to send")
			h.errorCh <- FSM_WRITE_FAILED
			conn.Close()
			return fmt.Errorf("closed")
		}
		fsm.bgpMessageStateUpdate(m.Header.Type, false)

		switch m.Header.Type {
		case bgp.BGP_MSG_NOTIFICATION:
			body := m.Body.(*bgp.BGPNotification)
			if body.ErrorCode == bgp.BGP_ERROR_CEASE && (body.ErrorSubcode == bgp.BGP_ERROR_SUB_ADMINISTRATIVE_SHUTDOWN || body.ErrorSubcode == bgp.BGP_ERROR_SUB_ADMINISTRATIVE_RESET) {
				communication, rest := decodeAdministrativeCommunication(body.Data)
				log.WithFields(log.Fields{
					"Topic":               "Peer",
					"Key":                 fsm.pConf.State.NeighborAddress,
					"State":               fsm.state.String(),
					"Code":                body.ErrorCode,
					"Subcode":             body.ErrorSubcode,
					"Communicated-Reason": communication,
					"Data":                rest,
				}).Warn("sent notification")
			} else {
				log.WithFields(log.Fields{
					"Topic":   "Peer",
					"Key":     fsm.pConf.State.NeighborAddress,
					"State":   fsm.state.String(),
					"Code":    body.ErrorCode,
					"Subcode": body.ErrorSubcode,
					"Data":    body.Data,
				}).Warn("sent notification")
			}
			h.errorCh <- FsmStateReason(fmt.Sprintf("%s %s", FSM_NOTIFICATION_SENT, bgp.NewNotificationErrorCode(body.ErrorCode, body.ErrorSubcode).String()))
			conn.Close()
			return fmt.Errorf("closed")
		case bgp.BGP_MSG_UPDATE:
			update := m.Body.(*bgp.BGPUpdate)
			log.WithFields(log.Fields{
				"Topic":       "Peer",
				"Key":         fsm.pConf.State.NeighborAddress,
				"State":       fsm.state.String(),
				"nlri":        update.NLRI,
				"withdrawals": update.WithdrawnRoutes,
				"attributes":  update.PathAttributes,
			}).Debug("sent update")
		default:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
				"data":  m,
			}).Debug("sent")
		}
		return nil
	}

	for {
		select {
		case <-h.t.Dying():
			return nil
		case o := <-h.outgoing.Out():
			m := o.(*FsmOutgoingMsg)
			for _, msg := range table.CreateUpdateMsgFromPaths(m.Paths, h.fsm.marshallingOptions) {
				if err := send(msg); err != nil {
					return nil
				}
			}
			if m.Notification != nil {
				if m.StayIdle {
					// current user is only prefix-limit
					// fix me if this is not the case
					h.changeAdminState(ADMIN_STATE_PFX_CT)
				}
				if err := send(m.Notification); err != nil {
					return nil
				}
			}
		case <-ticker.C:
			if err := send(bgp.NewBGPKeepAliveMessage()); err != nil {
				return nil
			}
		}
	}
}

func (h *FSMHandler) recvMessageloop() error {
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

func (h *FSMHandler) established() (bgp.FSMState, FsmStateReason) {
	fsm := h.fsm
	h.conn = fsm.conn
	h.t.Go(h.sendMessageloop)
	h.msgCh = h.incoming
	h.t.Go(h.recvMessageloop)

	var holdTimer *time.Timer
	if fsm.pConf.Timers.State.NegotiatedHoldTime == 0 {
		holdTimer = &time.Timer{}
	} else {
		holdTimer = time.NewTimer(time.Second * time.Duration(fsm.pConf.Timers.State.NegotiatedHoldTime))
	}

	fsm.gracefulRestartTimer.Stop()

	for {
		select {
		case <-h.t.Dying():
			return -1, FSM_DYING
		case conn, ok := <-fsm.connCh:
			if !ok {
				break
			}
			conn.Close()
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("Closed an accepted connection")
		case err := <-h.errorCh:
			h.conn.Close()
			h.t.Kill(nil)
			if s := fsm.pConf.GracefulRestart.State; s.Enabled && ((s.NotificationEnabled && strings.HasPrefix(string(err), FSM_NOTIFICATION_RECV)) || err == FSM_READ_FAILED || err == FSM_WRITE_FAILED) {
				err = FSM_GRACEFUL_RESTART
				log.WithFields(log.Fields{
					"Topic": "Peer",
					"Key":   fsm.pConf.State.NeighborAddress,
					"State": fsm.state.String(),
				}).Info("peer graceful restart")
				fsm.gracefulRestartTimer.Reset(time.Duration(fsm.pConf.GracefulRestart.State.PeerRestartTime) * time.Second)
			}
			return bgp.BGP_FSM_IDLE, err
		case <-holdTimer.C:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Warn("hold timer expired")
			m := bgp.NewBGPNotificationMessage(bgp.BGP_ERROR_HOLD_TIMER_EXPIRED, 0, nil)
			h.outgoing.In() <- &FsmOutgoingMsg{Notification: m}
			return bgp.BGP_FSM_IDLE, FSM_HOLD_TIMER_EXPIRED
		case <-h.holdTimerResetCh:
			if fsm.pConf.Timers.State.NegotiatedHoldTime != 0 {
				holdTimer.Reset(time.Second * time.Duration(fsm.pConf.Timers.State.NegotiatedHoldTime))
			}
		case stateOp := <-fsm.adminStateCh:
			err := h.changeAdminState(stateOp.State)
			if err == nil {
				switch stateOp.State {
				case ADMIN_STATE_DOWN:
					m := bgp.NewBGPNotificationMessage(bgp.BGP_ERROR_CEASE, bgp.BGP_ERROR_SUB_ADMINISTRATIVE_SHUTDOWN, stateOp.Communication)
					h.outgoing.In() <- &FsmOutgoingMsg{Notification: m}
				}
			}
		}
	}
}

func (h *FSMHandler) loop() error {
	fsm := h.fsm
	ch := make(chan bgp.FSMState)
	oldState := fsm.state

	f := func() error {
		nextState := bgp.FSMState(-1)
		var reason FsmStateReason
		switch fsm.state {
		case bgp.BGP_FSM_IDLE:
			nextState, reason = h.idle()
			// case bgp.BGP_FSM_CONNECT:
			// 	nextState = h.connect()
		case bgp.BGP_FSM_ACTIVE:
			nextState, reason = h.active()
		case bgp.BGP_FSM_OPENSENT:
			nextState, reason = h.opensent()
		case bgp.BGP_FSM_OPENCONFIRM:
			nextState, reason = h.openconfirm()
		case bgp.BGP_FSM_ESTABLISHED:
			nextState, reason = h.established()
		}
		fsm.reason = reason
		ch <- nextState
		return nil
	}

	h.t.Go(f)

	nextState := <-ch

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
		if fsm.h.sentNotification != "" {
			reason = FsmStateReason(fmt.Sprintf("%s %s", FSM_NOTIFICATION_SENT, fsm.h.sentNotification))
		}
		log.WithFields(log.Fields{
			"Topic":  "Peer",
			"Key":    fsm.pConf.State.NeighborAddress,
			"State":  fsm.state.String(),
			"Reason": reason,
		}).Info("Peer Down")
	}

	e := time.AfterFunc(time.Second*120, func() {
		log.WithFields(log.Fields{"Topic": "Peer"}).Fatalf("failed to free the fsm.h.t for %s %s %s", fsm.pConf.State.NeighborAddress, oldState, nextState)
	})
	h.t.Wait()
	e.Stop()

	// under zero means that tomb.Dying()
	if nextState >= bgp.BGP_FSM_IDLE {
		e := &FsmMsg{
			MsgType: FSM_MSG_STATE_CHANGE,
			MsgSrc:  fsm.pConf.State.NeighborAddress,
			MsgData: nextState,
			Version: h.fsm.version,
		}
		h.stateCh <- e
	}
	return nil
}

func (h *FSMHandler) changeAdminState(s AdminState) error {
	fsm := h.fsm
	if fsm.adminState != s {
		log.WithFields(log.Fields{
			"Topic":      "Peer",
			"Key":        fsm.pConf.State.NeighborAddress,
			"State":      fsm.state.String(),
			"AdminState": s.String(),
		}).Debug("admin state changed")

		fsm.adminState = s
		fsm.pConf.State.AdminDown = !fsm.pConf.State.AdminDown

		switch s {
		case ADMIN_STATE_UP:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Info("Administrative start")
		case ADMIN_STATE_DOWN:
			log.WithFields(log.Fields{
				"Topic": "Peer",
				"Key":   fsm.pConf.State.NeighborAddress,
				"State": fsm.state.String(),
			}).Info("Administrative shutdown")
		case ADMIN_STATE_PFX_CT:
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
