// Copyright (C) 2016 Nippon Telegraph and Telephone Corporation.
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
	"github.com/influxdata/influxdb/client/v2"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
	log "github.com/sirupsen/logrus"
	"time"
)

type Collector struct {
	s        *BgpServer
	url      string
	dbName   string
	interval uint64
	client   client.Client
}

const (
	MEATUREMENT_UPDATE = "update"
	MEATUREMENT_PEER   = "peer"
	MEATUREMENT_TABLE  = "table"
)

func (c *Collector) writePoints(points []*client.Point) error {
	bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  c.dbName,
		Precision: "ms",
	})
	bp.AddPoints(points)
	return c.client.Write(bp)
}

func (c *Collector) writePeer(msg *WatchEventPeerState) error {
	var state string
	switch msg.State {
	case bgp.BGP_FSM_ESTABLISHED:
		state = "Established"
	case bgp.BGP_FSM_IDLE:
		state = "Idle"
	default:
		return fmt.Errorf("unexpected fsm state %v", msg.State)
	}

	tags := map[string]string{
		"PeerAddress": msg.PeerAddress.String(),
		"PeerAS":      fmt.Sprintf("%v", msg.PeerAS),
		"State":       state,
	}

	fields := map[string]interface{}{
		"PeerID": msg.PeerID.String(),
	}

	pt, err := client.NewPoint(MEATUREMENT_PEER, tags, fields, msg.Timestamp)
	if err != nil {
		return err
	}
	return c.writePoints([]*client.Point{pt})
}

func path2data(path *table.Path) (map[string]interface{}, map[string]string) {
	fields := map[string]interface{}{
		"RouterID": path.GetSource().ID,
	}
	if asPath := path.GetAsPath(); asPath != nil {
		fields["ASPath"] = asPath.String()
	}
	if origin, err := path.GetOrigin(); err == nil {
		typ := "-"
		switch origin {
		case bgp.BGP_ORIGIN_ATTR_TYPE_IGP:
			typ = "i"
		case bgp.BGP_ORIGIN_ATTR_TYPE_EGP:
			typ = "e"
		case bgp.BGP_ORIGIN_ATTR_TYPE_INCOMPLETE:
			typ = "?"
		}
		fields["Origin"] = typ
	}
	if med, err := path.GetMed(); err == nil {
		fields["Med"] = med
	}

	tags := map[string]string{
		"PeerAddress": path.GetSource().Address.String(),
		"PeerAS":      fmt.Sprintf("%v", path.GetSource().AS),
		"Timestamp":   path.GetTimestamp().String(),
	}
	if nexthop := path.GetNexthop(); len(nexthop) > 0 {
		fields["NextHop"] = nexthop.String()
	}
	if originAS := path.GetSourceAs(); originAS != 0 {
		fields["OriginAS"] = fmt.Sprintf("%v", originAS)
	}

	if err := bgp.FlatUpdate(tags, path.GetNlri().Flat()); err != nil {
		log.WithFields(log.Fields{"Type": "collector", "Error": err}).Error("NLRI FlatUpdate failed")
	}
	for _, p := range path.GetPathAttrs() {
		if err := bgp.FlatUpdate(tags, p.Flat()); err != nil {
			log.WithFields(log.Fields{"Type": "collector", "Error": err}).Error("PathAttr FlatUpdate failed")
		}
	}
	return fields, tags
}

func (c *Collector) writeUpdate(msg *WatchEventUpdate) error {
	if len(msg.PathList) == 0 {
		// EOR
		return nil
	}
	now := time.Now()
	points := make([]*client.Point, 0, len(msg.PathList))
	for _, path := range msg.PathList {
		fields, tags := path2data(path)
		tags["Withdraw"] = fmt.Sprintf("%v", path.IsWithdraw)
		pt, err := client.NewPoint(MEATUREMENT_UPDATE, tags, fields, now)
		if err != nil {
			return fmt.Errorf("failed to write update, %v", err)
		}
		points = append(points, pt)
	}
	return c.writePoints(points)
}

func (c *Collector) writeTable(msg *WatchEventAdjIn) error {
	now := time.Now()
	points := make([]*client.Point, 0, len(msg.PathList))
	for _, path := range msg.PathList {
		fields, tags := path2data(path)
		pt, err := client.NewPoint(MEATUREMENT_TABLE, tags, fields, now)
		if err != nil {
			return fmt.Errorf("failed to write table, %v", err)
		}
		points = append(points, pt)
	}
	return c.writePoints(points)
}

func (c *Collector) loop() {
	w := c.s.Watch(WatchPeerState(true), WatchUpdate(false))
	defer w.Stop()

	ticker := func() *time.Ticker {
		if c.interval == 0 {
			return &time.Ticker{}
		}
		return time.NewTicker(time.Second * time.Duration(c.interval))
	}()

	for {
		select {
		case <-ticker.C:
			w.Generate(WATCH_EVENT_TYPE_PRE_UPDATE)
		case ev := <-w.Event():
			switch msg := ev.(type) {
			case *WatchEventUpdate:
				if err := c.writeUpdate(msg); err != nil {
					log.WithFields(log.Fields{"Type": "collector", "Error": err}).Error("Failed to write update event message")
				}
			case *WatchEventPeerState:
				if err := c.writePeer(msg); err != nil {
					log.WithFields(log.Fields{"Type": "collector", "Error": err}).Error("Failed to write state changed event message")
				}
			case *WatchEventAdjIn:
				if err := c.writeTable(msg); err != nil {
					log.WithFields(log.Fields{"Type": "collector", "Error": err}).Error("Failed to write Adj-In event message")
				}
			}
		}
	}
}

func NewCollector(s *BgpServer, url, dbName string, interval uint64) (*Collector, error) {
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr: url,
	})
	if err != nil {
		return nil, err
	}

	_, _, err = c.Ping(0)
	if err != nil {
		log.Error("can not connect to InfluxDB")
		log.WithFields(log.Fields{"Type": "collector", "Error": err}).Error("Failed to connect to InfluxDB")
		return nil, err
	}

	q := client.NewQuery("CREATE DATABASE "+dbName, "", "")
	if response, err := c.Query(q); err != nil || response.Error() != nil {
		log.WithFields(log.Fields{"Type": "collector", "Error": err}).Errorf("Failed to create database:%s", dbName)
		return nil, err
	}

	collector := &Collector{
		s:        s,
		url:      url,
		dbName:   dbName,
		interval: interval,
		client:   c,
	}
	go collector.loop()
	return collector, nil
}
