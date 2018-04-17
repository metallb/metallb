// +build !race

package bgp

// This test is disabled when using the race detector, because of
// https://github.com/osrg/gobgp/issues/1530.

import (
	"context"
	"fmt"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/osrg/gobgp/config"
	gobgp "github.com/osrg/gobgp/server"
	"github.com/osrg/gobgp/table"
)

func ipnet(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func runGoBGP(ctx context.Context, password string, port int32) (chan *table.Path, error) {
	s := gobgp.NewBgpServer()
	go s.Serve()

	global := &config.Global{
		Config: config.GlobalConfig{
			As:       64543,
			RouterId: "1.2.3.4",
			Port:     port,
		},
	}
	if err := s.Start(global); err != nil {
		return nil, err
	}

	n := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "127.0.0.1",
			PeerAs:          64543,
			AuthPassword:    password,
		},
	}
	if err := s.AddNeighbor(n); err != nil {
		return nil, err
	}

	ips := make(chan *table.Path, 1000)
	w := s.Watch(gobgp.WatchBestPath(false))
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-w.Event():
				switch msg := ev.(type) {
				case *gobgp.WatchEventBestPath:
					for _, path := range msg.PathList {
						ips <- path
					}
				}
			}
		}
	}()
	return ips, nil
}

func TestInterop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	ips, err := runGoBGP(ctx, "", 4179)
	if err != nil {
		t.Fatalf("starting GoBGP: %s", err)
	}

	l := log.NewNopLogger()
	sess, err := New(l, "127.0.0.1:4179", 64543, net.ParseIP("2.3.4.5"), 64543, 10*time.Second, "")
	if err != nil {
		t.Fatalf("starting BGP session to GoBGP: %s", err)
	}
	defer sess.Close()

	adv := &Advertisement{
		Prefix:      ipnet("1.2.3.0/24"),
		NextHop:     net.ParseIP("10.20.30.40"),
		LocalPref:   42,
		Communities: []uint32{1234, 2345},
	}

	if err := sess.Set(adv); err != nil {
		t.Fatalf("setting advertisement: %s", err)
	}

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("test timed out waiting for route")
		case path := <-ips:
			if err := checkPath(path, adv); err != nil {
				t.Fatalf("path did not match expectations: %s", err)
			}
			return
		}
	}
}

func TestTCPMD5(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	ips, err := runGoBGP(ctx, "somepassword", 5179)
	if err != nil {
		t.Fatalf("starting GoBGP: %s", err)
	}

	l := log.NewNopLogger()
	sess, err := New(l, "127.0.0.1:5179", 64543, net.ParseIP("2.3.4.6"), 64543, 10*time.Second, "somepassword")
	if err != nil {
		t.Fatalf("starting BGP session to GoBGP: %s", err)
	}
	defer sess.Close()

	adv := &Advertisement{
		Prefix:      ipnet("1.2.3.0/24"),
		NextHop:     net.ParseIP("10.20.30.40"),
		LocalPref:   42,
		Communities: []uint32{1234, 2345},
	}

	if err := sess.Set(adv); err != nil {
		t.Fatalf("setting advertisement: %s", err)
	}

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("test timed out waiting for route")
		case path := <-ips:
			if err := checkPath(path, adv); err != nil {
				t.Fatalf("path did not match expectations: %s", err)
			}
			return
		}
	}
}

func checkPath(path *table.Path, adv *Advertisement) error {
	nlri := path.GetNlri()
	if nlri.String() != adv.Prefix.String() {
		return fmt.Errorf("wrong nlri, got %s, want %s", nlri.String(), adv.Prefix.String())
	}

	if !path.GetNexthop().Equal(adv.NextHop) {
		return fmt.Errorf("wrong nexthop, got %s, want %s", path.GetNexthop(), adv.NextHop)
	}

	lp, err := path.GetLocalPref()
	if err != nil {
		return fmt.Errorf("get localpref: %s", err)
	}
	if lp != adv.LocalPref {
		return fmt.Errorf("wrong localpref, got %d, want %d", lp, adv.LocalPref)
	}

	comms := path.GetCommunities()
	sort.Slice(comms, func(i, j int) bool { return comms[i] < comms[j] })
	if diff := cmp.Diff(adv.Communities, comms); diff != "" {
		return fmt.Errorf("wrong communities (-want +got)\n%s", diff)
	}

	return nil
}
