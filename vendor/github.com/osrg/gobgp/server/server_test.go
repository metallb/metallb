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
	"net"
	"runtime"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
)

func TestModPolicyAssign(t *testing.T) {
	assert := assert.New(t)
	s := NewBgpServer()
	go s.Serve()
	err := s.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       1,
			RouterId: "1.1.1.1",
			Port:     -1,
		},
	})
	assert.Nil(err)
	defer s.Stop()

	err = s.AddPolicy(&table.Policy{Name: "p1"}, false)
	assert.Nil(err)

	err = s.AddPolicy(&table.Policy{Name: "p2"}, false)
	assert.Nil(err)

	err = s.AddPolicy(&table.Policy{Name: "p3"}, false)
	assert.Nil(err)

	err = s.AddPolicyAssignment("", table.POLICY_DIRECTION_IMPORT,
		[]*config.PolicyDefinition{&config.PolicyDefinition{Name: "p1"}, &config.PolicyDefinition{Name: "p2"}, &config.PolicyDefinition{Name: "p3"}}, table.ROUTE_TYPE_ACCEPT)
	assert.Nil(err)

	err = s.DeletePolicyAssignment("", table.POLICY_DIRECTION_IMPORT,
		[]*config.PolicyDefinition{&config.PolicyDefinition{Name: "p1"}}, false)
	assert.Nil(err)

	_, ps, _ := s.GetPolicyAssignment("", table.POLICY_DIRECTION_IMPORT)
	assert.Equal(len(ps), 2)
}

func TestMonitor(test *testing.T) {
	assert := assert.New(test)
	s := NewBgpServer()
	go s.Serve()
	err := s.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       1,
			RouterId: "1.1.1.1",
			Port:     10179,
		},
	})
	assert.Nil(err)
	defer s.Stop()

	n := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "127.0.0.1",
			PeerAs:          2,
		},
		Transport: config.Transport{
			Config: config.TransportConfig{
				PassiveMode: true,
			},
		},
	}
	err = s.AddNeighbor(n)
	assert.Nil(err)

	t := NewBgpServer()
	go t.Serve()
	err = t.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       2,
			RouterId: "2.2.2.2",
			Port:     -1,
		},
	})
	assert.Nil(err)
	defer t.Stop()

	m := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "127.0.0.1",
			PeerAs:          1,
		},
		Transport: config.Transport{
			Config: config.TransportConfig{
				RemotePort: 10179,
			},
		},
	}
	err = t.AddNeighbor(m)
	assert.Nil(err)

	for {
		time.Sleep(time.Second)
		if t.GetNeighbor("", false)[0].State.SessionState == config.SESSION_STATE_ESTABLISHED {
			break
		}
	}

	// Test WatchBestPath.
	w := s.Watch(WatchBestPath(false))

	// Advertises a route.
	attrs := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(0),
		bgp.NewPathAttributeNextHop("10.0.0.1"),
	}
	if _, err := t.AddPath("", []*table.Path{table.NewPath(nil, bgp.NewIPAddrPrefix(24, "10.0.0.0"), false, attrs, time.Now(), false)}); err != nil {
		log.Fatal(err)
	}
	ev := <-w.Event()
	b := ev.(*WatchEventBestPath)
	assert.Equal(1, len(b.PathList))
	assert.Equal("10.0.0.0/24", b.PathList[0].GetNlri().String())
	assert.False(b.PathList[0].IsWithdraw)

	// Withdraws the previous route.
	// NOTE: Withdow should not require any path attribute.
	if _, err := t.AddPath("", []*table.Path{table.NewPath(nil, bgp.NewIPAddrPrefix(24, "10.0.0.0"), true, nil, time.Now(), false)}); err != nil {
		log.Fatal(err)
	}
	ev = <-w.Event()
	b = ev.(*WatchEventBestPath)
	assert.Equal(1, len(b.PathList))
	assert.Equal("10.0.0.0/24", b.PathList[0].GetNlri().String())
	assert.True(b.PathList[0].IsWithdraw)

	// Stops the watcher still having an item.
	w.Stop()

	// Prepares an initial route to test WatchUpdate with "current" flag.
	if _, err := t.AddPath("", []*table.Path{table.NewPath(nil, bgp.NewIPAddrPrefix(24, "10.1.0.0"), false, attrs, time.Now(), false)}); err != nil {
		log.Fatal(err)
	}
	for {
		// Waits for the initial route will be advertised.
		rib, err := s.GetRib("", bgp.RF_IPv4_UC, nil)
		if err != nil {
			log.Fatal(err)
		}
		if len(rib.GetKnownPathList("")) > 0 {
			break
		}
		time.Sleep(1)
	}

	// Test WatchUpdate with "current" flag.
	w = s.Watch(WatchUpdate(true))

	// Test the initial route.
	ev = <-w.Event()
	u := ev.(*WatchEventUpdate)
	assert.Equal(1, len(u.PathList))
	assert.Equal("10.1.0.0/24", u.PathList[0].GetNlri().String())
	assert.False(u.PathList[0].IsWithdraw)
	ev = <-w.Event()
	u = ev.(*WatchEventUpdate)
	assert.Equal(len(u.PathList), 0) // End of RIB

	// Advertises an additional route.
	if _, err := t.AddPath("", []*table.Path{table.NewPath(nil, bgp.NewIPAddrPrefix(24, "10.2.0.0"), false, attrs, time.Now(), false)}); err != nil {
		log.Fatal(err)
	}
	ev = <-w.Event()
	u = ev.(*WatchEventUpdate)
	assert.Equal(1, len(u.PathList))
	assert.Equal("10.2.0.0/24", u.PathList[0].GetNlri().String())
	assert.False(u.PathList[0].IsWithdraw)

	// Withdraws the previous route.
	// NOTE: Withdow should not require any path attribute.
	if _, err := t.AddPath("", []*table.Path{table.NewPath(nil, bgp.NewIPAddrPrefix(24, "10.2.0.0"), true, nil, time.Now(), false)}); err != nil {
		log.Fatal(err)
	}
	ev = <-w.Event()
	u = ev.(*WatchEventUpdate)
	assert.Equal(1, len(u.PathList))
	assert.Equal("10.2.0.0/24", u.PathList[0].GetNlri().String())
	assert.True(u.PathList[0].IsWithdraw)

	// Stops the watcher still having an item.
	w.Stop()
}

func TestNumGoroutineWithAddDeleteNeighbor(t *testing.T) {
	assert := assert.New(t)
	s := NewBgpServer()
	go s.Serve()
	err := s.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       1,
			RouterId: "1.1.1.1",
			Port:     -1,
		},
	})
	assert.Nil(err)
	defer s.Stop()

	// wait a few seconds to avoid taking effect from other test cases.
	time.Sleep(time.Second * 5)

	num := runtime.NumGoroutine()

	n := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "127.0.0.1",
			PeerAs:          2,
		},
		Transport: config.Transport{
			Config: config.TransportConfig{
				PassiveMode: true,
			},
		},
	}
	err = s.AddNeighbor(n)
	assert.Nil(err)

	err = s.DeleteNeighbor(n)
	assert.Nil(err)
	// wait goroutines to finish (e.g. internal goroutine for
	// InfiniteChannel)
	time.Sleep(time.Second * 5)
	assert.Equal(num, runtime.NumGoroutine())
}

func newPeerandInfo(myAs, as uint32, address string, rib *table.TableManager) (*Peer, *table.PeerInfo) {
	nConf := &config.Neighbor{Config: config.NeighborConfig{PeerAs: as, NeighborAddress: address}}
	gConf := &config.Global{Config: config.GlobalConfig{As: myAs}}
	config.SetDefaultNeighborConfigValues(nConf, nil, gConf)
	policy := table.NewRoutingPolicy()
	policy.Reset(&config.RoutingPolicy{}, nil)
	p := NewPeer(
		&config.Global{Config: config.GlobalConfig{As: myAs}},
		nConf,
		rib,
		policy)
	for _, f := range rib.GetRFlist() {
		p.fsm.rfMap[f] = bgp.BGP_ADD_PATH_NONE
	}
	return p, &table.PeerInfo{AS: as, Address: net.ParseIP(address)}
}

func process(rib *table.TableManager, l []*table.Path) (*table.Path, *table.Path) {
	news, olds, _ := dstsToPaths(table.GLOBAL_RIB_NAME, rib.ProcessPaths(l), false)
	if len(news) != 1 {
		panic("can't handle multiple paths")
	}
	for idx, path := range news {
		var old *table.Path
		if olds != nil {
			old = olds[idx]
		}
		return path, old
	}
	return nil, nil
}

func TestFilterpathWitheBGP(t *testing.T) {
	as := uint32(65000)
	p1As := uint32(65001)
	p2As := uint32(65002)
	rib := table.NewTableManager([]bgp.RouteFamily{bgp.RF_IPv4_UC})
	p1, pi1 := newPeerandInfo(as, p1As, "192.168.0.1", rib)
	p2, pi2 := newPeerandInfo(as, p2As, "192.168.0.2", rib)

	nlri := bgp.NewIPAddrPrefix(24, "10.10.10.0")
	pa1 := []bgp.PathAttributeInterface{bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{p1As})}), bgp.NewPathAttributeLocalPref(200)}
	pa2 := []bgp.PathAttributeInterface{bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{p2As})})}

	path1 := table.NewPath(pi1, nlri, false, pa1, time.Now(), false)
	path2 := table.NewPath(pi2, nlri, false, pa2, time.Now(), false)

	new, old := process(rib, []*table.Path{path1, path2})
	assert.Equal(t, new, path1)
	filterpath(p1, new, old)
	filterpath(p2, new, old)

	new, old = process(rib, []*table.Path{path1.Clone(true)})
	assert.Equal(t, new, path2)
	// p1 and p2 advertized the same prefix and p1's was best. Then p1 withdraw it, so p2 must get withdawal.
	path := filterpath(p2, new, old)
	assert.NotNil(t, path)
	assert.True(t, path.IsWithdraw)

	// p1 should get the new best (from p2)
	assert.Equal(t, filterpath(p1, new, old), path2)

	new, old = process(rib, []*table.Path{path2.Clone(true)})
	assert.True(t, new.IsWithdraw)
	// p2 withdraw so p1 should get withdrawal.
	path = filterpath(p1, new, old)
	assert.True(t, path.IsWithdraw)

	// p2 withdraw so p2 should get nothing.
	path = filterpath(p2, new, old)
	assert.Nil(t, path)
}

func TestFilterpathWithiBGP(t *testing.T) {
	as := uint32(65000)

	rib := table.NewTableManager([]bgp.RouteFamily{bgp.RF_IPv4_UC})
	p1, pi1 := newPeerandInfo(as, as, "192.168.0.1", rib)
	//p2, pi2 := newPeerandInfo(as, as, "192.168.0.2", rib)
	p2, _ := newPeerandInfo(as, as, "192.168.0.2", rib)

	nlri := bgp.NewIPAddrPrefix(24, "10.10.10.0")
	pa1 := []bgp.PathAttributeInterface{bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{as})}), bgp.NewPathAttributeLocalPref(200)}
	//pa2 := []bgp.PathAttributeInterface{bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{as})})}

	path1 := table.NewPath(pi1, nlri, false, pa1, time.Now(), false)
	//path2 := table.NewPath(pi2, nlri, false, pa2, time.Now(), false)

	new, old := process(rib, []*table.Path{path1})
	assert.Equal(t, new, path1)
	path := filterpath(p1, new, old)
	assert.Nil(t, path)
	path = filterpath(p2, new, old)
	assert.Nil(t, path)

	new, old = process(rib, []*table.Path{path1.Clone(true)})
	path = filterpath(p1, new, old)
	assert.Nil(t, path)
	path = filterpath(p2, new, old)
	assert.Nil(t, path)

}

func TestFilterpathWithRejectPolicy(t *testing.T) {
	rib1 := table.NewTableManager([]bgp.RouteFamily{bgp.RF_IPv4_UC})
	_, pi1 := newPeerandInfo(1, 2, "192.168.0.1", rib1)
	rib2 := table.NewTableManager([]bgp.RouteFamily{bgp.RF_IPv4_UC})
	p2, _ := newPeerandInfo(1, 3, "192.168.0.2", rib2)

	comSet1 := config.CommunitySet{
		CommunitySetName: "comset1",
		CommunityList:    []string{"100:100"},
	}
	s, _ := table.NewCommunitySet(comSet1)
	p2.policy.AddDefinedSet(s)

	statement := config.Statement{
		Name: "stmt1",
		Conditions: config.Conditions{
			BgpConditions: config.BgpConditions{
				MatchCommunitySet: config.MatchCommunitySet{
					CommunitySet: "comset1",
				},
			},
		},
		Actions: config.Actions{
			RouteDisposition: config.ROUTE_DISPOSITION_REJECT_ROUTE,
		},
	}
	policy := config.PolicyDefinition{
		Name:       "policy1",
		Statements: []config.Statement{statement},
	}
	p, _ := table.NewPolicy(policy)
	p2.policy.AddPolicy(p, false)
	policies := []*config.PolicyDefinition{
		&config.PolicyDefinition{
			Name: "policy1",
		},
	}
	p2.policy.AddPolicyAssignment(p2.TableID(), table.POLICY_DIRECTION_EXPORT, policies, table.ROUTE_TYPE_ACCEPT)

	for _, addCommunity := range []bool{false, true, false, true} {
		nlri := bgp.NewIPAddrPrefix(24, "10.10.10.0")
		pa1 := []bgp.PathAttributeInterface{bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{1})}), bgp.NewPathAttributeLocalPref(200)}
		if addCommunity {
			pa1 = append(pa1, bgp.NewPathAttributeCommunities([]uint32{100<<16 | 100}))
		}
		path1 := table.NewPath(pi1, nlri, false, pa1, time.Now(), false)
		new, old := process(rib2, []*table.Path{path1})
		assert.Equal(t, new, path1)
		path2 := p2.filterpath(new, old)
		if addCommunity {
			assert.True(t, path2.IsWithdraw)
		} else {
			assert.False(t, path2.IsWithdraw)
		}
	}

}

func TestPeerGroup(test *testing.T) {
	assert := assert.New(test)
	log.SetLevel(log.DebugLevel)
	s := NewBgpServer()
	go s.Serve()
	err := s.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       1,
			RouterId: "1.1.1.1",
			Port:     10179,
		},
	})
	assert.Nil(err)
	defer s.Stop()

	g := &config.PeerGroup{
		Config: config.PeerGroupConfig{
			PeerAs:        2,
			PeerGroupName: "g",
		},
	}
	err = s.AddPeerGroup(g)
	assert.Nil(err)

	n := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "127.0.0.1",
			PeerGroup:       "g",
		},
		Transport: config.Transport{
			Config: config.TransportConfig{
				PassiveMode: true,
			},
		},
	}
	configured := map[string]interface{}{
		"config": map[string]interface{}{
			"neigbor-address": "127.0.0.1",
			"peer-group":      "g",
		},
		"transport": map[string]interface{}{
			"config": map[string]interface{}{
				"passive-mode": true,
			},
		},
	}
	config.RegisterConfiguredFields("127.0.0.1", configured)
	err = s.AddNeighbor(n)
	assert.Nil(err)

	t := NewBgpServer()
	go t.Serve()
	err = t.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       2,
			RouterId: "2.2.2.2",
			Port:     -1,
		},
	})
	assert.Nil(err)
	defer t.Stop()

	m := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "127.0.0.1",
			PeerAs:          1,
		},
		Transport: config.Transport{
			Config: config.TransportConfig{
				RemotePort: 10179,
			},
		},
	}
	err = t.AddNeighbor(m)
	assert.Nil(err)

	for {
		time.Sleep(time.Second)
		if t.GetNeighbor("", false)[0].State.SessionState == config.SESSION_STATE_ESTABLISHED {
			break
		}
	}
}

func TestDynamicNeighbor(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	s1 := NewBgpServer()
	go s1.Serve()
	err := s1.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       1,
			RouterId: "1.1.1.1",
			Port:     10179,
		},
	})
	assert.Nil(err)
	defer s1.Stop()

	g := &config.PeerGroup{
		Config: config.PeerGroupConfig{
			PeerAs:        2,
			PeerGroupName: "g",
		},
	}
	err = s1.AddPeerGroup(g)
	assert.Nil(err)

	d := &config.DynamicNeighbor{
		Config: config.DynamicNeighborConfig{
			Prefix:    "127.0.0.0/24",
			PeerGroup: "g",
		},
	}
	err = s1.AddDynamicNeighbor(d)
	assert.Nil(err)

	s2 := NewBgpServer()
	go s2.Serve()
	err = s2.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       2,
			RouterId: "2.2.2.2",
			Port:     -1,
		},
	})
	assert.Nil(err)
	defer s2.Stop()

	m := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "127.0.0.1",
			PeerAs:          1,
		},
		Transport: config.Transport{
			Config: config.TransportConfig{
				RemotePort: 10179,
			},
		},
	}
	err = s2.AddNeighbor(m)

	assert.Nil(err)

	for {
		time.Sleep(time.Second)
		if s2.GetNeighbor("", false)[0].State.SessionState == config.SESSION_STATE_ESTABLISHED {
			break
		}
	}
}

func TestGracefulRestartTimerExpired(t *testing.T) {
	assert := assert.New(t)
	s1 := NewBgpServer()
	go s1.Serve()
	err := s1.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       1,
			RouterId: "1.1.1.1",
			Port:     10179,
		},
	})
	assert.Nil(err)
	defer s1.Stop()

	n := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "127.0.0.1",
			PeerAs:          2,
		},
		Transport: config.Transport{
			Config: config.TransportConfig{
				PassiveMode: true,
			},
		},
		GracefulRestart: config.GracefulRestart{
			Config: config.GracefulRestartConfig{
				Enabled:     true,
				RestartTime: 10,
			},
		},
	}
	err = s1.AddNeighbor(n)
	assert.Nil(err)

	s2 := NewBgpServer()
	go s2.Serve()
	err = s2.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       2,
			RouterId: "2.2.2.2",
			Port:     -1,
		},
	})
	assert.Nil(err)
	defer s2.Stop()

	m := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "127.0.0.1",
			PeerAs:          1,
		},
		Transport: config.Transport{
			Config: config.TransportConfig{
				RemotePort: 10179,
			},
		},
		GracefulRestart: config.GracefulRestart{
			Config: config.GracefulRestartConfig{
				Enabled:     true,
				RestartTime: 10,
			},
		},
	}
	err = s2.AddNeighbor(m)
	assert.Nil(err)

	// Waiting for BGP session established.
	for {
		time.Sleep(time.Second)
		if s2.GetNeighbor("", false)[0].State.SessionState == config.SESSION_STATE_ESTABLISHED {
			break
		}
	}

	// Force TCP session disconnected in order to cause Graceful Restart at s1
	// side.
	for _, n := range s2.neighborMap {
		n.fsm.conn.Close()
	}
	s2.Stop()

	time.Sleep(5 * time.Second)

	// Create dummy session which does NOT send BGP OPEN message in order to
	// cause Graceful Restart timer expired.
	var conn net.Conn
	for {
		time.Sleep(time.Second)
		var err error
		conn, err = net.Dial("tcp", "127.0.0.1:10179")
		if err != nil {
			log.Warn("net.Dial:", err)
		}
		break
	}
	defer conn.Close()

	// Waiting for Graceful Restart timer expired and moving on to IDLE state.
	for {
		time.Sleep(time.Second)
		if s1.GetNeighbor("", false)[0].State.SessionState == config.SESSION_STATE_IDLE {
			break
		}
	}
}
