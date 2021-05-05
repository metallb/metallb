// Copyright (C) 2015,2016 Nippon Telegraph and Telephone Corporation.
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
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/osrg/gobgp/internal/pkg/config"
	"github.com/osrg/gobgp/internal/pkg/table"
	"github.com/osrg/gobgp/pkg/packet/bgp"
	"github.com/osrg/gobgp/pkg/packet/rtr"

	"github.com/armon/go-radix"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	connectRetryInterval = 30
)

func before(a, b uint32) bool {
	return int32(a-b) < 0
}

type roaBucket struct {
	Prefix  *table.IPPrefix
	entries []*table.ROA
}

func (r *roaBucket) GetEntries() []*table.ROA {
	return r.entries
}

type roas []*table.ROA

func (r roas) Len() int {
	return len(r)
}

func (r roas) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r roas) Less(i, j int) bool {
	r1 := r[i]
	r2 := r[j]

	if r1.MaxLen < r2.MaxLen {
		return true
	} else if r1.MaxLen > r2.MaxLen {
		return false
	}

	if r1.AS < r2.AS {
		return true
	}
	return false
}

type roaEventType uint8

const (
	roaConnected roaEventType = iota
	roaDisconnected
	roaRTR
	roaLifetimeout
)

type roaEvent struct {
	EventType roaEventType
	Src       string
	Data      []byte
	conn      *net.TCPConn
}

type roaManager struct {
	AS        uint32
	Roas      map[bgp.RouteFamily]*radix.Tree
	eventCh   chan *roaEvent
	clientMap map[string]*roaClient
}

func newROAManager(as uint32) (*roaManager, error) {
	m := &roaManager{
		AS:   as,
		Roas: make(map[bgp.RouteFamily]*radix.Tree),
	}
	m.Roas[bgp.RF_IPv4_UC] = radix.New()
	m.Roas[bgp.RF_IPv6_UC] = radix.New()
	m.eventCh = make(chan *roaEvent)
	m.clientMap = make(map[string]*roaClient)
	return m, nil
}

func (m *roaManager) enabled() bool {
	return len(m.clientMap) != 0
}

func (m *roaManager) SetAS(as uint32) error {
	if m.AS != 0 {
		return fmt.Errorf("AS was already configured")
	}
	m.AS = as
	return nil
}

func (m *roaManager) AddServer(host string, lifetime int64) error {
	address, port, err := net.SplitHostPort(host)
	if err != nil {
		return err
	}
	if lifetime == 0 {
		lifetime = 3600
	}
	if _, ok := m.clientMap[host]; ok {
		return fmt.Errorf("ROA server exists %s", host)
	}
	m.clientMap[host] = newRoaClient(address, port, m.eventCh, lifetime)
	return nil
}

func (m *roaManager) DeleteServer(host string) error {
	client, ok := m.clientMap[host]
	if !ok {
		return fmt.Errorf("ROA server doesn't exists %s", host)
	}
	client.stop()
	m.deleteAllROA(host)
	delete(m.clientMap, host)
	return nil
}

func (m *roaManager) deleteAllROA(network string) {
	for _, tree := range m.Roas {
		deleteKeys := make([]string, 0, tree.Len())
		tree.Walk(func(s string, v interface{}) bool {
			b, _ := v.(*roaBucket)
			newEntries := make([]*table.ROA, 0, len(b.entries))
			for _, r := range b.entries {
				if r.Src != network {
					newEntries = append(newEntries, r)
				}
			}
			if len(newEntries) > 0 {
				b.entries = newEntries
			} else {
				deleteKeys = append(deleteKeys, s)
			}
			return false
		})
		for _, key := range deleteKeys {
			tree.Delete(key)
		}
	}
}

func (m *roaManager) Enable(address string) error {
	for network, client := range m.clientMap {
		add, _, _ := net.SplitHostPort(network)
		if add == address {
			client.enable(client.serialNumber)
			return nil
		}
	}
	return fmt.Errorf("ROA server not found %s", address)
}

func (m *roaManager) Disable(address string) error {
	for network, client := range m.clientMap {
		add, _, _ := net.SplitHostPort(network)
		if add == address {
			client.reset()
			m.deleteAllROA(add)
			return nil
		}
	}
	return fmt.Errorf("ROA server not found %s", address)
}

func (m *roaManager) Reset(address string) error {
	return m.Disable(address)
}

func (m *roaManager) SoftReset(address string) error {
	for network, client := range m.clientMap {
		add, _, _ := net.SplitHostPort(network)
		if add == address {
			client.softReset()
			m.deleteAllROA(network)
			return nil
		}
	}
	return fmt.Errorf("ROA server not found %s", address)
}

func (m *roaManager) ReceiveROA() chan *roaEvent {
	return m.eventCh
}

func (c *roaClient) lifetimeout() {
	c.eventCh <- &roaEvent{
		EventType: roaLifetimeout,
		Src:       c.host,
	}
}

func (m *roaManager) HandleROAEvent(ev *roaEvent) {
	client, y := m.clientMap[ev.Src]
	if !y {
		if ev.EventType == roaConnected {
			ev.conn.Close()
		}
		log.WithFields(log.Fields{"Topic": "rpki"}).Errorf("Can't find %s ROA server configuration", ev.Src)
		return
	}
	switch ev.EventType {
	case roaDisconnected:
		log.WithFields(log.Fields{"Topic": "rpki"}).Infof("ROA server %s is disconnected", ev.Src)
		client.state.Downtime = time.Now().Unix()
		// clear state
		client.endOfData = false
		client.pendingROAs = make([]*table.ROA, 0)
		client.state.RpkiMessages = config.RpkiMessages{}
		client.conn = nil
		go client.tryConnect()
		client.timer = time.AfterFunc(time.Duration(client.lifetime)*time.Second, client.lifetimeout)
		client.oldSessionID = client.sessionID
	case roaConnected:
		log.WithFields(log.Fields{"Topic": "rpki"}).Infof("ROA server %s is connected", ev.Src)
		client.conn = ev.conn
		client.state.Uptime = time.Now().Unix()
		go client.established()
	case roaRTR:
		m.handleRTRMsg(client, &client.state, ev.Data)
	case roaLifetimeout:
		// a) already reconnected but hasn't received
		// EndOfData -> needs to delete stale ROAs
		// b) not reconnected -> needs to delete stale ROAs
		//
		// c) already reconnected and received EndOfData so
		// all stale ROAs were deleted -> timer was cancelled
		// so should not be here.
		if client.oldSessionID != client.sessionID {
			log.WithFields(log.Fields{"Topic": "rpki"}).Infof("Reconnected to %s. Ignore timeout", client.host)
		} else {
			log.WithFields(log.Fields{"Topic": "rpki"}).Infof("Deleting all ROAs due to timeout with:%s", client.host)
			m.deleteAllROA(client.host)
		}
	}
}

func (m *roaManager) roa2tree(roa *table.ROA) (*radix.Tree, string) {
	tree := m.Roas[bgp.RF_IPv4_UC]
	if roa.Family == bgp.AFI_IP6 {
		tree = m.Roas[bgp.RF_IPv6_UC]
	}
	return tree, table.IpToRadixkey(roa.Prefix.Prefix, roa.Prefix.Length)
}

func (m *roaManager) deleteROA(roa *table.ROA) {
	tree, key := m.roa2tree(roa)
	b, _ := tree.Get(key)
	if b != nil {
		bucket := b.(*roaBucket)
		newEntries := make([]*table.ROA, 0, len(bucket.entries))
		for _, r := range bucket.entries {
			if !r.Equal(roa) {
				newEntries = append(newEntries, r)
			}
		}
		if len(newEntries) != len(bucket.entries) {
			bucket.entries = newEntries
			if len(newEntries) == 0 {
				tree.Delete(key)
			}
			return
		}
	}
	log.WithFields(log.Fields{
		"Topic":         "rpki",
		"Prefix":        roa.Prefix.Prefix.String(),
		"Prefix Length": roa.Prefix.Length,
		"AS":            roa.AS,
		"Max Length":    roa.MaxLen,
	}).Info("Can't withdraw a ROA")
}

func (m *roaManager) DeleteROA(roa *table.ROA) {
	m.deleteROA(roa)
}

func (m *roaManager) addROA(roa *table.ROA) {
	tree, key := m.roa2tree(roa)
	b, _ := tree.Get(key)
	var bucket *roaBucket
	if b == nil {
		bucket = &roaBucket{
			Prefix:  roa.Prefix,
			entries: make([]*table.ROA, 0),
		}
		tree.Insert(key, bucket)
	} else {
		bucket = b.(*roaBucket)
		for _, r := range bucket.entries {
			if r.Equal(roa) {
				// we already have the same one
				return
			}
		}
	}
	bucket.entries = append(bucket.entries, roa)
}

func (m *roaManager) AddROA(roa *table.ROA) {
	m.addROA(roa)
}

func (m *roaManager) handleRTRMsg(client *roaClient, state *config.RpkiServerState, buf []byte) {
	received := &state.RpkiMessages.RpkiReceived

	m1, err := rtr.ParseRTR(buf)
	if err == nil {
		switch msg := m1.(type) {
		case *rtr.RTRSerialNotify:
			if before(client.serialNumber, msg.RTRCommon.SerialNumber) {
				client.enable(client.serialNumber)
			} else if client.serialNumber == msg.RTRCommon.SerialNumber {
				// nothing
			} else {
				// should not happen. try to get the whole ROAs.
				client.softReset()
			}
			received.SerialNotify++
		case *rtr.RTRSerialQuery:
		case *rtr.RTRResetQuery:
		case *rtr.RTRCacheResponse:
			received.CacheResponse++
			client.endOfData = false
		case *rtr.RTRIPPrefix:
			family := bgp.AFI_IP
			if msg.Type == rtr.RTR_IPV4_PREFIX {
				received.Ipv4Prefix++
			} else {
				family = bgp.AFI_IP6
				received.Ipv6Prefix++
			}
			roa := table.NewROA(family, msg.Prefix, msg.PrefixLen, msg.MaxLen, msg.AS, client.host)
			if (msg.Flags & 1) == 1 {
				if client.endOfData {
					m.addROA(roa)
				} else {
					client.pendingROAs = append(client.pendingROAs, roa)
				}
			} else {
				m.deleteROA(roa)
			}
		case *rtr.RTREndOfData:
			received.EndOfData++
			if client.sessionID != msg.RTRCommon.SessionID {
				// remove all ROAs related with the
				// previous session
				m.deleteAllROA(client.host)
			}
			client.sessionID = msg.RTRCommon.SessionID
			client.serialNumber = msg.RTRCommon.SerialNumber
			client.endOfData = true
			if client.timer != nil {
				client.timer.Stop()
				client.timer = nil
			}
			for _, roa := range client.pendingROAs {
				m.addROA(roa)
			}
			client.pendingROAs = make([]*table.ROA, 0)
		case *rtr.RTRCacheReset:
			client.softReset()
			received.CacheReset++
		case *rtr.RTRErrorReport:
			received.Error++
		}
	} else {
		log.WithFields(log.Fields{
			"Topic": "rpki",
			"Host":  client.host,
			"Error": err,
		}).Info("Failed to parse an RTR message")
	}
}

func (m *roaManager) GetServers() []*config.RpkiServer {
	f := func(tree *radix.Tree) (map[string]uint32, map[string]uint32) {
		records := make(map[string]uint32)
		prefixes := make(map[string]uint32)

		tree.Walk(func(s string, v interface{}) bool {
			b, _ := v.(*roaBucket)
			tmpRecords := make(map[string]uint32)
			for _, roa := range b.entries {
				tmpRecords[roa.Src]++
			}

			for src, r := range tmpRecords {
				if r > 0 {
					records[src] += r
					prefixes[src]++
				}
			}
			return false
		})
		return records, prefixes
	}

	recordsV4, prefixesV4 := f(m.Roas[bgp.RF_IPv4_UC])
	recordsV6, prefixesV6 := f(m.Roas[bgp.RF_IPv6_UC])

	l := make([]*config.RpkiServer, 0, len(m.clientMap))
	for _, client := range m.clientMap {
		state := &client.state

		if client.conn == nil {
			state.Up = false
		} else {
			state.Up = true
		}
		f := func(m map[string]uint32, key string) uint32 {
			if r, ok := m[key]; ok {
				return r
			}
			return 0
		}
		state.RecordsV4 = f(recordsV4, client.host)
		state.RecordsV6 = f(recordsV6, client.host)
		state.PrefixesV4 = f(prefixesV4, client.host)
		state.PrefixesV6 = f(prefixesV6, client.host)
		state.SerialNumber = client.serialNumber

		addr, port, _ := net.SplitHostPort(client.host)
		l = append(l, &config.RpkiServer{
			Config: config.RpkiServerConfig{
				Address: addr,
				// Note: RpkiServerConfig.Port is uint32 type, but the TCP/UDP
				// port is 16-bit length.
				Port: func() uint32 { p, _ := strconv.ParseUint(port, 10, 16); return uint32(p) }(),
			},
			State: client.state,
		})
	}
	return l
}

func (m *roaManager) GetRoa(family bgp.RouteFamily) ([]*table.ROA, error) {
	if len(m.clientMap) == 0 {
		return []*table.ROA{}, fmt.Errorf("RPKI server isn't configured.")
	}
	var rfList []bgp.RouteFamily
	switch family {
	case bgp.RF_IPv4_UC:
		rfList = []bgp.RouteFamily{bgp.RF_IPv4_UC}
	case bgp.RF_IPv6_UC:
		rfList = []bgp.RouteFamily{bgp.RF_IPv6_UC}
	default:
		rfList = []bgp.RouteFamily{bgp.RF_IPv4_UC, bgp.RF_IPv6_UC}
	}
	l := make([]*table.ROA, 0)
	for _, rf := range rfList {
		if tree, ok := m.Roas[rf]; ok {
			tree.Walk(func(s string, v interface{}) bool {
				b, _ := v.(*roaBucket)
				var roaList roas
				for _, r := range b.entries {
					roaList = append(roaList, r)
				}
				sort.Sort(roaList)
				for _, roa := range roaList {
					l = append(l, roa)
				}
				return false
			})
		}
	}
	return l, nil
}

func validatePath(ownAs uint32, tree *radix.Tree, cidr string, asPath *bgp.PathAttributeAsPath) *table.Validation {
	var as uint32

	validation := &table.Validation{
		Status:          config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND,
		Reason:          table.RPKI_VALIDATION_REASON_TYPE_NONE,
		Matched:         make([]*table.ROA, 0),
		UnmatchedLength: make([]*table.ROA, 0),
		UnmatchedAs:     make([]*table.ROA, 0),
	}

	if asPath == nil || len(asPath.Value) == 0 {
		as = ownAs
	} else {
		param := asPath.Value[len(asPath.Value)-1]
		switch param.GetType() {
		case bgp.BGP_ASPATH_ATTR_TYPE_SEQ:
			asList := param.GetAS()
			if len(asList) == 0 {
				as = ownAs
			} else {
				as = asList[len(asList)-1]
			}
		case bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SET, bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SEQ:
			as = ownAs
		default:
			return validation
		}
	}
	_, n, _ := net.ParseCIDR(cidr)
	ones, _ := n.Mask.Size()
	prefixLen := uint8(ones)
	key := table.IpToRadixkey(n.IP, prefixLen)
	_, b, _ := tree.LongestPrefix(key)
	if b == nil {
		return validation
	}

	var bucket *roaBucket
	fn := radix.WalkFn(func(k string, v interface{}) bool {
		bucket, _ = v.(*roaBucket)
		for _, r := range bucket.entries {
			if prefixLen <= r.MaxLen {
				if r.AS != 0 && r.AS == as {
					validation.Matched = append(validation.Matched, r)
				} else {
					validation.UnmatchedAs = append(validation.UnmatchedAs, r)
				}
			} else {
				validation.UnmatchedLength = append(validation.UnmatchedLength, r)
			}
		}
		return false
	})
	tree.WalkPath(key, fn)

	if len(validation.Matched) != 0 {
		validation.Status = config.RPKI_VALIDATION_RESULT_TYPE_VALID
		validation.Reason = table.RPKI_VALIDATION_REASON_TYPE_NONE
	} else if len(validation.UnmatchedAs) != 0 {
		validation.Status = config.RPKI_VALIDATION_RESULT_TYPE_INVALID
		validation.Reason = table.RPKI_VALIDATION_REASON_TYPE_AS
	} else if len(validation.UnmatchedLength) != 0 {
		validation.Status = config.RPKI_VALIDATION_RESULT_TYPE_INVALID
		validation.Reason = table.RPKI_VALIDATION_REASON_TYPE_LENGTH
	} else {
		validation.Status = config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND
		validation.Reason = table.RPKI_VALIDATION_REASON_TYPE_NONE
	}

	return validation
}

func (m *roaManager) validate(path *table.Path) *table.Validation {
	if len(m.clientMap) == 0 || path.IsWithdraw || path.IsEOR() {
		// RPKI isn't enabled or invalid path
		return nil
	}
	if tree, ok := m.Roas[path.GetRouteFamily()]; ok {
		return validatePath(m.AS, tree, path.GetNlri().String(), path.GetAsPath())
	}
	return nil
}

type roaClient struct {
	host         string
	conn         *net.TCPConn
	state        config.RpkiServerState
	eventCh      chan *roaEvent
	sessionID    uint16
	oldSessionID uint16
	serialNumber uint32
	timer        *time.Timer
	lifetime     int64
	endOfData    bool
	pendingROAs  []*table.ROA
	cancelfnc    context.CancelFunc
	ctx          context.Context
}

func newRoaClient(address, port string, ch chan *roaEvent, lifetime int64) *roaClient {
	ctx, cancel := context.WithCancel(context.Background())
	c := &roaClient{
		host:        net.JoinHostPort(address, port),
		eventCh:     ch,
		lifetime:    lifetime,
		pendingROAs: make([]*table.ROA, 0),
		ctx:         ctx,
		cancelfnc:   cancel,
	}
	go c.tryConnect()
	return c
}

func (c *roaClient) enable(serial uint32) error {
	if c.conn != nil {
		r := rtr.NewRTRSerialQuery(c.sessionID, serial)
		data, _ := r.Serialize()
		_, err := c.conn.Write(data)
		if err != nil {
			return err
		}
		c.state.RpkiMessages.RpkiSent.SerialQuery++
	}
	return nil
}

func (c *roaClient) softReset() error {
	if c.conn != nil {
		r := rtr.NewRTRResetQuery()
		data, _ := r.Serialize()
		_, err := c.conn.Write(data)
		if err != nil {
			return err
		}
		c.state.RpkiMessages.RpkiSent.ResetQuery++
		c.endOfData = false
		c.pendingROAs = make([]*table.ROA, 0)
	}
	return nil
}

func (c *roaClient) reset() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *roaClient) stop() {
	c.cancelfnc()
	c.reset()
}

func (c *roaClient) tryConnect() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		if conn, err := net.Dial("tcp", c.host); err != nil {
			// better to use context with timeout
			time.Sleep(connectRetryInterval * time.Second)
		} else {
			c.eventCh <- &roaEvent{
				EventType: roaConnected,
				Src:       c.host,
				conn:      conn.(*net.TCPConn),
			}
			return
		}
	}
}

func (c *roaClient) established() (err error) {
	defer func() {
		c.conn.Close()
		c.eventCh <- &roaEvent{
			EventType: roaDisconnected,
			Src:       c.host,
		}
	}()

	if err := c.softReset(); err != nil {
		return err
	}

	for {
		header := make([]byte, rtr.RTR_MIN_LEN)
		if _, err = io.ReadFull(c.conn, header); err != nil {
			return err
		}
		totalLen := binary.BigEndian.Uint32(header[4:8])
		if totalLen < rtr.RTR_MIN_LEN {
			return fmt.Errorf("too short header length %v", totalLen)
		}

		body := make([]byte, totalLen-rtr.RTR_MIN_LEN)
		if _, err = io.ReadFull(c.conn, body); err != nil {
			return
		}

		c.eventCh <- &roaEvent{
			EventType: roaRTR,
			Src:       c.host,
			Data:      append(header, body...),
		}
	}
}
