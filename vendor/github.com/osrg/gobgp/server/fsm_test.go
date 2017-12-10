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
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/eapache/channels"
	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type MockConnection struct {
	*testing.T
	net.Conn
	recvCh    chan chan byte
	sendBuf   [][]byte
	currentCh chan byte
	isClosed  bool
	wait      int
	mtx       sync.Mutex
}

func NewMockConnection(t *testing.T) *MockConnection {
	m := &MockConnection{
		T:        t,
		recvCh:   make(chan chan byte, 128),
		sendBuf:  make([][]byte, 0),
		isClosed: false,
	}
	return m
}

func (m *MockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *MockConnection) setData(data []byte) int {
	dataChan := make(chan byte, 4096)
	for _, b := range data {
		dataChan <- b
	}
	m.recvCh <- dataChan
	return len(dataChan)
}

func (m *MockConnection) Read(buf []byte) (int, error) {
	m.mtx.Lock()
	closed := m.isClosed
	m.mtx.Unlock()
	if closed {
		return 0, errors.New("already closed")
	}

	if m.currentCh == nil {
		m.currentCh = <-m.recvCh
	}

	length := 0
	rest := len(buf)
	for i := 0; i < rest; i++ {
		if len(m.currentCh) > 0 {
			val := <-m.currentCh
			buf[i] = val
			length++
		} else {
			m.currentCh = nil
			break
		}
	}

	m.Logf("%d bytes read from peer", length)
	return length, nil
}

func (m *MockConnection) Write(buf []byte) (int, error) {
	time.Sleep(time.Duration(m.wait) * time.Millisecond)
	m.sendBuf = append(m.sendBuf, buf)
	msg, _ := bgp.ParseBGPMessage(buf)
	m.Logf("%d bytes written by gobgp  message type : %s",
		len(buf), showMessageType(msg.Header.Type))
	return len(buf), nil
}

func showMessageType(t uint8) string {
	switch t {
	case bgp.BGP_MSG_KEEPALIVE:
		return "BGP_MSG_KEEPALIVE"
	case bgp.BGP_MSG_NOTIFICATION:
		return "BGP_MSG_NOTIFICATION"
	case bgp.BGP_MSG_OPEN:
		return "BGP_MSG_OPEN"
	case bgp.BGP_MSG_UPDATE:
		return "BGP_MSG_UPDATE"
	case bgp.BGP_MSG_ROUTE_REFRESH:
		return "BGP_MSG_ROUTE_REFRESH"
	}
	return strconv.Itoa(int(t))
}

func (m *MockConnection) Close() error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if !m.isClosed {
		close(m.recvCh)
		m.isClosed = true
	}
	return nil
}

func (m *MockConnection) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.ParseIP("10.10.10.10"),
		Port: bgp.BGP_PORT}
}

func TestReadAll(t *testing.T) {
	assert := assert.New(t)
	m := NewMockConnection(t)
	msg := open()
	expected1, _ := msg.Header.Serialize()
	expected2, _ := msg.Body.Serialize()

	pushBytes := func() {
		m.Log("push 5 bytes")
		m.setData(expected1[0:5])
		m.Log("push rest")
		m.setData(expected1[5:])
		m.Log("push bytes at once")
		m.setData(expected2)
	}

	go pushBytes()

	var actual1 []byte
	actual1, _ = readAll(m, bgp.BGP_HEADER_LENGTH)
	m.Log(actual1)
	assert.Equal(expected1, actual1)

	var actual2 []byte
	actual2, _ = readAll(m, len(expected2))
	m.Log(actual2)
	assert.Equal(expected2, actual2)
}

func TestFSMHandlerOpensent_HoldTimerExpired(t *testing.T) {
	assert := assert.New(t)
	m := NewMockConnection(t)

	p, h := makePeerAndHandler()

	// push mock connection
	p.fsm.conn = m
	p.fsm.h = h

	// set keepalive ticker
	p.fsm.pConf.Timers.State.NegotiatedHoldTime = 3

	// set holdtime
	p.fsm.opensentHoldTime = 2

	state, _ := h.opensent()

	assert.Equal(bgp.BGP_FSM_IDLE, state)
	lastMsg := m.sendBuf[len(m.sendBuf)-1]
	sent, _ := bgp.ParseBGPMessage(lastMsg)
	assert.Equal(uint8(bgp.BGP_MSG_NOTIFICATION), sent.Header.Type)
	assert.Equal(uint8(bgp.BGP_ERROR_HOLD_TIMER_EXPIRED), sent.Body.(*bgp.BGPNotification).ErrorCode)

}

func TestFSMHandlerOpenconfirm_HoldTimerExpired(t *testing.T) {
	assert := assert.New(t)
	m := NewMockConnection(t)

	p, h := makePeerAndHandler()

	// push mock connection
	p.fsm.conn = m
	p.fsm.h = h

	// set up keepalive ticker
	p.fsm.pConf.Timers.Config.KeepaliveInterval = 1

	// set holdtime
	p.fsm.pConf.Timers.State.NegotiatedHoldTime = 2
	state, _ := h.openconfirm()

	assert.Equal(bgp.BGP_FSM_IDLE, state)
	lastMsg := m.sendBuf[len(m.sendBuf)-1]
	sent, _ := bgp.ParseBGPMessage(lastMsg)
	assert.Equal(uint8(bgp.BGP_MSG_NOTIFICATION), sent.Header.Type)
	assert.Equal(uint8(bgp.BGP_ERROR_HOLD_TIMER_EXPIRED), sent.Body.(*bgp.BGPNotification).ErrorCode)

}

func TestFSMHandlerEstablish_HoldTimerExpired(t *testing.T) {
	assert := assert.New(t)
	m := NewMockConnection(t)

	p, h := makePeerAndHandler()

	// push mock connection
	p.fsm.conn = m
	p.fsm.h = h

	// set keepalive ticker
	p.fsm.pConf.Timers.State.NegotiatedHoldTime = 3

	msg := keepalive()
	header, _ := msg.Header.Serialize()
	body, _ := msg.Body.Serialize()

	pushPackets := func() {
		// first keepalive from peer
		m.setData(header)
		m.setData(body)
	}

	// set holdtime
	p.fsm.pConf.Timers.Config.HoldTime = 2
	p.fsm.pConf.Timers.State.NegotiatedHoldTime = 2

	go pushPackets()
	state, _ := h.established()
	time.Sleep(time.Second * 1)
	assert.Equal(bgp.BGP_FSM_IDLE, state)
	m.mtx.Lock()
	lastMsg := m.sendBuf[len(m.sendBuf)-1]
	m.mtx.Unlock()
	sent, _ := bgp.ParseBGPMessage(lastMsg)
	assert.Equal(uint8(bgp.BGP_MSG_NOTIFICATION), sent.Header.Type)
	assert.Equal(uint8(bgp.BGP_ERROR_HOLD_TIMER_EXPIRED), sent.Body.(*bgp.BGPNotification).ErrorCode)
}

func TestFSMHandlerOpenconfirm_HoldtimeZero(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assert := assert.New(t)
	m := NewMockConnection(t)

	p, h := makePeerAndHandler()

	// push mock connection
	p.fsm.conn = m
	p.fsm.h = h

	// set up keepalive ticker
	p.fsm.pConf.Timers.Config.KeepaliveInterval = 1
	// set holdtime
	p.fsm.pConf.Timers.State.NegotiatedHoldTime = 0
	go h.openconfirm()

	time.Sleep(100 * time.Millisecond)

	assert.Equal(0, len(m.sendBuf))

}

func TestFSMHandlerEstablished_HoldtimeZero(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assert := assert.New(t)
	m := NewMockConnection(t)

	p, h := makePeerAndHandler()

	// push mock connection
	p.fsm.conn = m
	p.fsm.h = h

	// set holdtime
	p.fsm.pConf.Timers.State.NegotiatedHoldTime = 0

	go h.established()

	time.Sleep(100 * time.Millisecond)

	assert.Equal(0, len(m.sendBuf))
}

func TestCheckOwnASLoop(t *testing.T) {
	assert := assert.New(t)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{65100})}
	aspath := bgp.NewPathAttributeAsPath(aspathParam)
	assert.False(hasOwnASLoop(65100, 10, aspath))
	assert.True(hasOwnASLoop(65100, 0, aspath))
	assert.False(hasOwnASLoop(65200, 0, aspath))
}

func makePeerAndHandler() (*Peer, *FSMHandler) {
	p := &Peer{
		fsm:      NewFSM(&config.Global{}, &config.Neighbor{}, table.NewRoutingPolicy()),
		outgoing: channels.NewInfiniteChannel(),
	}

	h := &FSMHandler{
		fsm:      p.fsm,
		errorCh:  make(chan FsmStateReason, 2),
		incoming: channels.NewInfiniteChannel(),
		outgoing: p.outgoing,
	}

	return p, h

}

func open() *bgp.BGPMessage {
	p1 := bgp.NewOptionParameterCapability(
		[]bgp.ParameterCapabilityInterface{bgp.NewCapRouteRefresh()})
	p2 := bgp.NewOptionParameterCapability(
		[]bgp.ParameterCapabilityInterface{bgp.NewCapMultiProtocol(bgp.RF_IPv4_UC)})
	g := &bgp.CapGracefulRestartTuple{AFI: 4, SAFI: 2, Flags: 3}
	p3 := bgp.NewOptionParameterCapability(
		[]bgp.ParameterCapabilityInterface{bgp.NewCapGracefulRestart(true, true, 100,
			[]*bgp.CapGracefulRestartTuple{g})})
	p4 := bgp.NewOptionParameterCapability(
		[]bgp.ParameterCapabilityInterface{bgp.NewCapFourOctetASNumber(100000)})
	return bgp.NewBGPOpenMessage(11033, 303, "100.4.10.3",
		[]bgp.OptionParameterInterface{p1, p2, p3, p4})
}

func keepalive() *bgp.BGPMessage {
	return bgp.NewBGPKeepAliveMessage()
}
