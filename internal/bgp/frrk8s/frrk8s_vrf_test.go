// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"net/netip"
	"testing"
	"time"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/bgp/community"
	"k8s.io/utils/ptr"
)

func TestVRFSingleEBGPSessionMultiHop(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.1.1.254"),
			MyASN:         100,
			RouterID:      netip.MustParseAddr("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleVRFIBGPSession(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.1.1.254"),
			MyASN:         100,
			RouterID:      netip.MustParseAddr("10.1.1.254"),
			PeerASN:       100,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestTwoSessionsOneVRF(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.1.1.254"),
			MyASN:         100,
			RouterID:      netip.MustParseAddr("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer1"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()
	session2, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.4.4.255",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.3.3.254"),
			MyASN:         300,
			RouterID:      netip.MustParseAddr("10.3.3.254"),
			PeerASN:       400,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer2",
			VRFName:       "red"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session2.Close()

	testCheckConfigFile(t)
}

func TestTwoSessionsSameIPVRF(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.1.1.254"),
			MyASN:         100,
			RouterID:      netip.MustParseAddr("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer1"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()
	session2, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.3.3.254"),
			MyASN:         300,
			RouterID:      netip.MustParseAddr("10.3.3.254"),
			PeerASN:       400,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer2",
			VRFName:       "red"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session2.Close()

	testCheckConfigFile(t)
}

func TestTwoSessionsSameIPRouterIDASNVRF(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			MyASN:         100,
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer1"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()
	session2, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			MyASN:         100,
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer2",
			VRFName:       "red"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session2.Close()

	testCheckConfigFile(t)
}

func TestSingleAdvertisementVRF(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.1.1.254"),
			MyASN:         100,
			RouterID:      netip.MustParseAddr("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := netip.MustParsePrefix("172.16.1.10/24")
	communities := []community.BGPCommunity{}
	community1, _ := community.New("1111:2222")
	communities = append(communities, community1)
	community2, _ := community.New("3333:4444")
	communities = append(communities, community2)
	adv := &bgp.Advertisement{
		Prefix:      prefix,
		Communities: communities,
		LocalPref:   300,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestSingleAdvertisementChangeVRF(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.1.1.254"),
			MyASN:         100,
			RouterID:      netip.MustParseAddr("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := netip.MustParsePrefix("172.16.1.10/24")

	adv := &bgp.Advertisement{
		Prefix: prefix,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	prefix = netip.MustParsePrefix("172.16.1.11/24")

	adv = &bgp.Advertisement{
		Prefix: prefix,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestTwoAdvertisementsVRF(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.1.1.254"),
			MyASN:         100,
			RouterID:      netip.MustParseAddr("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix1 := netip.MustParsePrefix("172.16.1.10/24")
	communities := []community.BGPCommunity{}
	community, _ := community.New("1111:2222")
	communities = append(communities, community)
	adv1 := &bgp.Advertisement{
		Prefix:      prefix1,
		Communities: communities,
	}

	prefix2 := netip.MustParsePrefix("172.16.1.11/24")

	adv2 := &bgp.Advertisement{
		Prefix: prefix2,
	}

	err = session.Set(adv1, adv2)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestTwoAdvertisementsTwoSessionsOneVRF(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.1.1.254"),
			MyASN:         100,
			RouterID:      netip.MustParseAddr("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.255",
			PeerPort:      179,
			SourceAddress: netip.MustParseAddr("10.1.1.254"),
			MyASN:         100,
			RouterID:      netip.MustParseAddr("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer1",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()

	prefix1 := netip.MustParsePrefix("172.16.1.10/24")
	communities := []community.BGPCommunity{}
	community, _ := community.New("1111:2222")
	communities = append(communities, community)
	adv1 := &bgp.Advertisement{
		Prefix:      prefix1,
		Communities: communities,
	}

	prefix2 := netip.MustParsePrefix("172.16.1.11/24")

	adv2 := &bgp.Advertisement{
		Prefix:      prefix2,
		Communities: communities,
		LocalPref:   2,
	}

	err = session.Set(adv1, adv2)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}
	err = session1.Set(adv1, adv2)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}
