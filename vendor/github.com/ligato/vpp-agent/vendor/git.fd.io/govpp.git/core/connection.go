// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	logger "github.com/sirupsen/logrus"

	"git.fd.io/govpp.git/adapter"
	"git.fd.io/govpp.git/api"
	"git.fd.io/govpp.git/codec"
)

var (
	RequestChanBufSize      = 100 // default size of the request channel buffer
	ReplyChanBufSize        = 100 // default size of the reply channel buffer
	NotificationChanBufSize = 100 // default size of the notification channel buffer
)

var (
	HealthCheckProbeInterval = time.Second * 1        // default health check probe interval
	HealthCheckReplyTimeout  = time.Millisecond * 100 // timeout for reply to a health check probe
	HealthCheckThreshold     = 1                      // number of failed health checks until the error is reported
	DefaultReplyTimeout      = time.Second * 1        // default timeout for replies from VPP
)

// ConnectionState represents the current state of the connection to VPP.
type ConnectionState int

const (
	// Connected represents state in which the connection has been successfully established.
	Connected ConnectionState = iota

	// Disconnected represents state in which the connection has been dropped.
	Disconnected
)

// ConnectionEvent is a notification about change in the VPP connection state.
type ConnectionEvent struct {
	// Timestamp holds the time when the event has been created.
	Timestamp time.Time

	// State holds the new state of the connection at the time when the event has been created.
	State ConnectionState

	// Error holds error if any encountered.
	Error error
}

var (
	connLock sync.RWMutex // lock for the global connection
	conn     *Connection  // global handle to the Connection (used in the message receive callback)
)

// Connection represents a shared memory connection to VPP via vppAdapter.
type Connection struct {
	vpp       adapter.VppAdapter // VPP adapter
	connected uint32             // non-zero if the adapter is connected to VPP

	codec  *codec.MsgCodec        // message codec
	msgIDs map[string]uint16      // map of message IDs indexed by message name + CRC
	msgMap map[uint16]api.Message // map of messages indexed by message ID

	maxChannelID uint32              // maximum used channel ID (the real limit is 2^15, 32-bit is used for atomic operations)
	channelsLock sync.RWMutex        // lock for the channels map
	channels     map[uint16]*Channel // map of all API channels indexed by the channel ID

	subscriptionsLock sync.RWMutex                  // lock for the subscriptions map
	subscriptions     map[uint16][]*subscriptionCtx // map od all notification subscriptions indexed by message ID

	pingReqID   uint16 // ID if the ControlPing message
	pingReplyID uint16 // ID of the ControlPingReply message

	lastReplyLock sync.Mutex // lock for the last reply
	lastReply     time.Time  // time of the last received reply from VPP
}

func newConnection(vpp adapter.VppAdapter) *Connection {
	c := &Connection{
		vpp:           vpp,
		codec:         &codec.MsgCodec{},
		msgIDs:        make(map[string]uint16),
		msgMap:        make(map[uint16]api.Message),
		channels:      make(map[uint16]*Channel),
		subscriptions: make(map[uint16][]*subscriptionCtx),
	}
	vpp.SetMsgCallback(c.msgCallback)
	return c
}

// Connect connects to VPP using specified VPP adapter and returns the connection handle.
// This call blocks until VPP is connected, or an error occurs. Only one connection attempt will be performed.
func Connect(vppAdapter adapter.VppAdapter) (*Connection, error) {
	// create new connection handle
	c, err := createConnection(vppAdapter)
	if err != nil {
		return nil, err
	}

	// blocking attempt to connect to VPP
	if err := c.connectVPP(); err != nil {
		return nil, err
	}

	return c, nil
}

// AsyncConnect asynchronously connects to VPP using specified VPP adapter and returns the connection handle
// and ConnectionState channel. This call does not block until connection is established, it
// returns immediately. The caller is supposed to watch the returned ConnectionState channel for
// Connected/Disconnected events. In case of disconnect, the library will asynchronously try to reconnect.
func AsyncConnect(vppAdapter adapter.VppAdapter) (*Connection, chan ConnectionEvent, error) {
	// create new connection handle
	c, err := createConnection(vppAdapter)
	if err != nil {
		return nil, nil, err
	}

	// asynchronously attempt to connect to VPP
	connChan := make(chan ConnectionEvent, NotificationChanBufSize)
	go c.connectLoop(connChan)

	return c, connChan, nil
}

// Disconnect disconnects from VPP and releases all connection-related resources.
func (c *Connection) Disconnect() {
	if c == nil {
		return
	}

	connLock.Lock()
	defer connLock.Unlock()

	if c.vpp != nil {
		c.disconnectVPP()
	}
	conn = nil
}

// newConnection returns new connection handle.
func createConnection(vppAdapter adapter.VppAdapter) (*Connection, error) {
	connLock.Lock()
	defer connLock.Unlock()

	if conn != nil {
		return nil, errors.New("only one connection per process is supported")
	}

	conn = newConnection(vppAdapter)

	return conn, nil
}

// connectVPP performs blocking attempt to connect to VPP.
func (c *Connection) connectVPP() error {
	log.Debug("Connecting to VPP..")

	// blocking connect
	if err := c.vpp.Connect(); err != nil {
		return err
	}

	log.Debugf("Connected to VPP.")

	if err := c.retrieveMessageIDs(); err != nil {
		c.vpp.Disconnect()
		return fmt.Errorf("VPP is incompatible: %v", err)
	}

	// store connected state
	atomic.StoreUint32(&c.connected, 1)

	return nil
}

func (c *Connection) NewAPIChannel() (api.Channel, error) {
	return c.newAPIChannel(RequestChanBufSize, ReplyChanBufSize)
}

func (c *Connection) NewAPIChannelBuffered(reqChanBufSize, replyChanBufSize int) (api.Channel, error) {
	return c.newAPIChannel(reqChanBufSize, replyChanBufSize)
}

// NewAPIChannelBuffered returns a new API channel for communication with VPP via govpp core.
// It allows to specify custom buffer sizes for the request and reply Go channels.
func (c *Connection) newAPIChannel(reqChanBufSize, replyChanBufSize int) (*Channel, error) {
	if c == nil {
		return nil, errors.New("nil connection passed in")
	}

	// create new channel
	chID := uint16(atomic.AddUint32(&c.maxChannelID, 1) & 0x7fff)
	channel := newChannel(chID, c, c.codec, c, reqChanBufSize, replyChanBufSize)

	// store API channel within the client
	c.channelsLock.Lock()
	c.channels[chID] = channel
	c.channelsLock.Unlock()

	// start watching on the request channel
	go c.watchRequests(channel)

	return channel, nil
}

// releaseAPIChannel releases API channel that needs to be closed.
func (c *Connection) releaseAPIChannel(ch *Channel) {
	log.WithFields(logger.Fields{
		"channel": ch.id,
	}).Debug("API channel released")

	// delete the channel from channels map
	c.channelsLock.Lock()
	delete(c.channels, ch.id)
	c.channelsLock.Unlock()
}

// GetMessageID returns message identifier of given API message.
func (c *Connection) GetMessageID(msg api.Message) (uint16, error) {
	if c == nil {
		return 0, errors.New("nil connection passed in")
	}

	if msgID, ok := c.msgIDs[getMsgNameWithCrc(msg)]; ok {
		return msgID, nil
	}

	return 0, fmt.Errorf("unknown message: %s (%s)", msg.GetMessageName(), msg.GetCrcString())
}

// LookupByID looks up message name and crc by ID.
func (c *Connection) LookupByID(msgID uint16) (api.Message, error) {
	if c == nil {
		return nil, errors.New("nil connection passed in")
	}

	if msg, ok := c.msgMap[msgID]; ok {
		return msg, nil
	}

	return nil, fmt.Errorf("unknown message ID: %d", msgID)
}

// retrieveMessageIDs retrieves IDs for all registered messages and stores them in map
func (c *Connection) retrieveMessageIDs() (err error) {
	t := time.Now()

	var addMsg = func(msgID uint16, msg api.Message) {
		c.msgIDs[getMsgNameWithCrc(msg)] = msgID
		c.msgMap[msgID] = msg
	}

	msgs := api.GetAllMessages()

	for name, msg := range msgs {
		msgID, err := c.vpp.GetMsgID(msg.GetMessageName(), msg.GetCrcString())
		if err != nil {
			return err
		}

		addMsg(msgID, msg)

		if msg.GetMessageName() == msgControlPing.GetMessageName() {
			c.pingReqID = msgID
			msgControlPing = reflect.New(reflect.TypeOf(msg).Elem()).Interface().(api.Message)
		} else if msg.GetMessageName() == msgControlPingReply.GetMessageName() {
			c.pingReplyID = msgID
			msgControlPingReply = reflect.New(reflect.TypeOf(msg).Elem()).Interface().(api.Message)
		}

		if debugMsgIDs {
			log.Debugf("message %q (%s) has ID: %d", name, getMsgNameWithCrc(msg), msgID)
		}
	}

	log.Debugf("retrieving %d message IDs took %s", len(msgs), time.Since(t))

	// fallback for control ping when vpe package is not imported
	if c.pingReqID == 0 {
		c.pingReqID, err = c.vpp.GetMsgID(msgControlPing.GetMessageName(), msgControlPing.GetCrcString())
		if err != nil {
			return err
		}
		addMsg(c.pingReqID, msgControlPing)
	}
	if c.pingReplyID == 0 {
		c.pingReplyID, err = c.vpp.GetMsgID(msgControlPingReply.GetMessageName(), msgControlPingReply.GetCrcString())
		if err != nil {
			return err
		}
		addMsg(c.pingReplyID, msgControlPingReply)
	}

	return nil
}

// disconnectVPP disconnects from VPP in case it is connected.
func (c *Connection) disconnectVPP() {
	if atomic.CompareAndSwapUint32(&c.connected, 1, 0) {
		c.vpp.Disconnect()
	}
}

// connectLoop attempts to connect to VPP until it succeeds.
// Then it continues with healthCheckLoop.
func (c *Connection) connectLoop(connChan chan ConnectionEvent) {
	// loop until connected
	for {
		if err := c.vpp.WaitReady(); err != nil {
			log.Warnf("wait ready failed: %v", err)
		}
		if err := c.connectVPP(); err == nil {
			// signal connected event
			connChan <- ConnectionEvent{Timestamp: time.Now(), State: Connected}
			break
		} else {
			log.Errorf("connecting to VPP failed: %v", err)
			time.Sleep(time.Second)
		}
	}

	// we are now connected, continue with health check loop
	c.healthCheckLoop(connChan)
}

// healthCheckLoop checks whether connection to VPP is alive. In case of disconnect,
// it continues with connectLoop and tries to reconnect.
func (c *Connection) healthCheckLoop(connChan chan ConnectionEvent) {
	// create a separate API channel for health check probes
	ch, err := c.newAPIChannel(1, 1)
	if err != nil {
		log.Error("Failed to create health check API channel, health check will be disabled:", err)
		return
	}

	var (
		sinceLastReply time.Duration
		failedChecks   int
	)

	// send health check probes until an error or timeout occurs
	for {
		// sleep until next health check probe period
		time.Sleep(HealthCheckProbeInterval)

		if atomic.LoadUint32(&c.connected) == 0 {
			// Disconnect has been called in the meantime, return the healthcheck - reconnect loop
			log.Debug("Disconnected on request, exiting health check loop.")
			return
		}

		// try draining probe replies from previous request before sending next one
		select {
		case <-ch.replyChan:
			log.Debug("drained old probe reply from reply channel")
		default:
		}

		// send the control ping request
		ch.reqChan <- &vppRequest{msg: msgControlPing}

		for {
			// expect response within timeout period
			select {
			case vppReply := <-ch.replyChan:
				err = vppReply.err

			case <-time.After(HealthCheckReplyTimeout):
				err = ErrProbeTimeout

				// check if time since last reply from any other
				// channel is less than health check reply timeout
				c.lastReplyLock.Lock()
				sinceLastReply = time.Since(c.lastReply)
				c.lastReplyLock.Unlock()

				if sinceLastReply < HealthCheckReplyTimeout {
					log.Warnf("VPP health check probe timing out, but some request on other channel was received %v ago, continue waiting!", sinceLastReply)
					continue
				}
			}
			break
		}

		if err == ErrProbeTimeout {
			failedChecks++
			log.Warnf("VPP health check probe timed out after %v (%d. timeout)", HealthCheckReplyTimeout, failedChecks)
			if failedChecks > HealthCheckThreshold {
				// in case of exceeded failed check treshold, assume VPP disconnected
				log.Errorf("VPP health check exceeded treshold for timeouts (>%d), assuming disconnect", HealthCheckThreshold)
				connChan <- ConnectionEvent{Timestamp: time.Now(), State: Disconnected}
				break
			}
		} else if err != nil {
			// in case of error, assume VPP disconnected
			log.Errorf("VPP health check probe failed: %v", err)
			connChan <- ConnectionEvent{Timestamp: time.Now(), State: Disconnected, Error: err}
			break
		} else if failedChecks > 0 {
			// in case of success after failed checks, clear failed check counter
			failedChecks = 0
			log.Infof("VPP health check probe OK")
		}
	}

	// cleanup
	ch.Close()
	c.disconnectVPP()

	// we are now disconnected, start connect loop
	c.connectLoop(connChan)
}

func getMsgNameWithCrc(x api.Message) string {
	return x.GetMessageName() + "_" + x.GetCrcString()
}
