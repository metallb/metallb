// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"net"
	"testing"
	"time"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/logging"
)

func TestVRFSingleEBGPSessionMultiHop(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       100,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
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
			PeerAddress:   "10.4.4.255:179",
			SourceAddress: net.ParseIP("10.3.3.254"),
			MyASN:         300,
			RouterID:      net.ParseIP("10.3.3.254"),
			PeerASN:       400,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
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
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.3.3.254"),
			MyASN:         300,
			RouterID:      net.ParseIP("10.3.3.254"),
			PeerASN:       400,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			MyASN:         100,
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
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
			PeerAddress:   "10.2.2.254:179",
			MyASN:         100,
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []uint32{}
	community, _ := config.ParseCommunity("1111:2222")
	communities = append(communities, community)
	community, _ = config.ParseCommunity("3333:4444")
	communities = append(communities, community)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}

	adv := &bgp.Advertisement{
		Prefix: prefix,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	prefix = &net.IPNet{
		IP:   net.ParseIP("172.16.1.11"),
		Mask: classCMask,
	}

	adv = &bgp.Advertisement{
		Prefix: prefix,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestSingleAdvertisementWithPeerSelectorVRF(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []uint32{}
	community, _ := config.ParseCommunity("1111:2222")
	communities = append(communities, community)
	community, _ = config.ParseCommunity("3333:4444")
	communities = append(communities, community)
	adv := &bgp.Advertisement{
		Prefix:      prefix,
		Communities: communities,
		LocalPref:   300,
		Peers:       []string{"test-peer"},
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestTwoAdvertisementsVRF(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix1 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []uint32{}
	community, _ := config.ParseCommunity("1111:2222")
	communities = append(communities, community)
	adv1 := &bgp.Advertisement{
		Prefix:      prefix1,
		Communities: communities,
	}

	prefix2 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.11"),
		Mask: classCMask,
	}

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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
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
			PeerAddress:   "10.2.2.255:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer1",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()

	prefix1 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []uint32{}
	community, _ := config.ParseCommunity("1111:2222")
	communities = append(communities, community)
	adv1 := &bgp.Advertisement{
		Prefix:      prefix1,
		Communities: communities,
	}

	prefix2 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.11"),
		Mask: classCMask,
	}

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

func TestTwoAdvertisementsTwoSessionsOneWithPeerSelectorAndVRF(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.255:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer1",
		})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()

	prefix1 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []uint32{}
	community, _ := config.ParseCommunity("1111:2222")
	communities = append(communities, community)
	adv1 := &bgp.Advertisement{
		Prefix:      prefix1,
		Communities: communities,
		Peers:       []string{"test-peer"},
	}

	prefix2 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.11"),
		Mask: classCMask,
	}

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

func TestTwoAdvertisementsTwoSessionsVRFWithPeerSelector(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer",
			VRFName:       "red"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.255:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      time.Second,
			KeepAliveTime: time.Second,
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer1",
			VRFName:       "blue",
		})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()

	prefix1 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []uint32{}
	community, _ := config.ParseCommunity("1111:2222")
	communities = append(communities, community)
	adv1 := &bgp.Advertisement{
		Prefix:      prefix1,
		Communities: communities,
		Peers:       []string{"test-peer"},
	}

	prefix2 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.11"),
		Mask: classCMask,
	}

	adv2 := &bgp.Advertisement{
		Prefix:      prefix2,
		Communities: communities,
		LocalPref:   2,
		Peers:       []string{"test-peer1"},
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
