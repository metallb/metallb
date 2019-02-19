// +build disabled

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
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	api "github.com/osrg/gobgp/api"
	gobgp "github.com/osrg/gobgp/pkg/server"
)

func ipnet(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func runGoBGP(ctx context.Context, password string, port int32) (chan *api.Path, error) {
	s := gobgp.NewBgpServer()
	go s.Serve()

	global := &api.StartBgpRequest{
		Global: &api.Global{
			As:         64543,
			RouterId:   "1.2.3.4",
			ListenPort: port,
		},
	}
	if err := s.StartBgp(context.Background(), global); err != nil {
		return nil, err
	}

	n := &api.AddPeerRequest{
		Peer: &api.Peer{
			Conf: &api.PeerConf{
				NeighborAddress: "127.0.0.1",
				PeerAs:          64543,
				AuthPassword:    password,
			},
			Transport: &api.Transport{
				PassiveMode: true,
			},
		},
	}
	if err := s.AddPeer(context.Background(), n); err != nil {
		return nil, err
	}

	ips := make(chan *api.Path, 1000)
	newPath := func(path *api.Path) {
		ips <- path
	}
	w := &api.MonitorTableRequest{
		Type:    api.Resource_GLOBAL,
		Name:    "",
		Current: true,
	}
	if err := s.MonitorTable(context.Background(), w, newPath); err != nil {
		return nil, err
	}
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

func checkPath(path *api.Path, adv *Advertisement) error {
	var nlri api.IPAddressPrefix
	if err := ptypes.UnmarshalAny(path.Nlri, &nlri); err != nil {
		return fmt.Errorf("error getting path info: %v", err)
	}
	pfx := fmt.Sprintf("%s/%d", nlri.Prefix, nlri.PrefixLen)
	if pfx != adv.Prefix.String() {
		return fmt.Errorf("wrong nlri, got %s, want %s", pfx, adv.Prefix.String())
	}

	var (
		nexthop     string
		localpref   uint32
		communities []uint32
	)
	for _, attr := range path.Pattrs {
		switch {
		case ptypes.Is(attr, &api.NextHopAttribute{}):
			var nh api.NextHopAttribute
			if err := ptypes.UnmarshalAny(attr, &nh); err != nil {
				return err
			}
			nexthop = nh.NextHop
		case ptypes.Is(attr, &api.LocalPrefAttribute{}):
			var lp api.LocalPrefAttribute
			if err := ptypes.UnmarshalAny(attr, &lp); err != nil {
				return err
			}
			localpref = lp.LocalPref
		case ptypes.Is(attr, &api.CommunitiesAttribute{}):
			var cm api.CommunitiesAttribute
			if err := ptypes.UnmarshalAny(attr, &cm); err != nil {
				return err
			}
			communities = cm.Communities
		}
	}

	if nexthop != adv.NextHop.String() {
		return fmt.Errorf("wrong nexthop, got %s, want %s", nexthop, adv.NextHop)
	}
	if localpref != adv.LocalPref {
		return fmt.Errorf("wrong localpref, got %d, want %d", localpref, adv.LocalPref)
	}
	sort.Slice(communities, func(i, j int) bool { return communities[i] < communities[j] })
	if diff := cmp.Diff(adv.Communities, communities); diff != "" {
		return fmt.Errorf("wrong communities (-want +got)\n%s", diff)
	}

	return nil
}
