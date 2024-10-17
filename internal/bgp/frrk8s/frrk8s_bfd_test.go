// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"net"
	"testing"
	"time"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"k8s.io/utils/ptr"
)

func TestBFDProfileNoSessions(t *testing.T) {
	pp := map[string]*config.BFDProfile{
		"foo": {
			Name:             "foo",
			ReceiveInterval:  ptr.To(uint32(60)),
			TransmitInterval: ptr.To(uint32(70)),
			DetectMultiplier: ptr.To(uint32(5)),
			EchoInterval:     ptr.To(uint32(90)),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       ptr.To(uint32(60)),
		},
		"bar": {
			Name:             "bar",
			ReceiveInterval:  ptr.To(uint32(60)),
			TransmitInterval: ptr.To(uint32(70)),
			DetectMultiplier: ptr.To(uint32(5)),
			EchoInterval:     ptr.To(uint32(90)),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       ptr.To(uint32(60)),
		},
	}
	sessionManager := newTestSessionManager(t)

	err := sessionManager.SyncBFDProfiles(pp)
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles: %s", err)
	}

	testCheckConfigFile(t)
	err = sessionManager.SyncBFDProfiles(map[string]*config.BFDProfile{})
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles: %s", err)
	}
}

func TestBFDProfileCornerCases(t *testing.T) {
	pp := map[string]*config.BFDProfile{
		"foo": {
			Name:             "foo",
			ReceiveInterval:  ptr.To(uint32(60)),
			TransmitInterval: ptr.To(uint32(70)),
			DetectMultiplier: ptr.To(uint32(5)),
			EchoInterval:     ptr.To(uint32(90)),
			EchoMode:         true,
			PassiveMode:      true,
			MinimumTTL:       ptr.To(uint32(60)),
		},
	}

	sessionManager := newTestSessionManager(t)

	err := sessionManager.SyncBFDProfiles(pp)
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles: %s", err)
	}

	testCheckConfigFile(t)
	err = sessionManager.SyncBFDProfiles(map[string]*config.BFDProfile{})
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles: %s", err)
	}
}

func TestBFDWithSession(t *testing.T) {
	pp := map[string]*config.BFDProfile{
		"foo": {
			Name:             "foo",
			ReceiveInterval:  ptr.To(uint32(60)),
			TransmitInterval: ptr.To(uint32(70)),
			DetectMultiplier: ptr.To(uint32(5)),
			EchoInterval:     ptr.To(uint32(90)),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       ptr.To(uint32(60)),
		},
		"bar": {
			Name:             "bar",
			ReceiveInterval:  ptr.To(uint32(60)),
			TransmitInterval: ptr.To(uint32(70)),
			DetectMultiplier: ptr.To(uint32(5)),
			EchoInterval:     ptr.To(uint32(90)),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       ptr.To(uint32(60)),
		},
	}

	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	err := sessionManager.SyncBFDProfiles(pp)
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles %s", err)
	}

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(2 * time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer",
			BFDProfile:    "foo"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
	err = sessionManager.SyncBFDProfiles(map[string]*config.BFDProfile{})
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles %s", err)
	}
}

func TestBFDProfileAllDefault(t *testing.T) {
	pp := map[string]*config.BFDProfile{
		"foo": {
			Name:             "foo",
			ReceiveInterval:  ptr.To(uint32(60)),
			TransmitInterval: ptr.To(uint32(70)),
			DetectMultiplier: ptr.To(uint32(5)),
			EchoInterval:     ptr.To(uint32(90)),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       ptr.To(uint32(60)),
		},
		"bar": {
			Name: "bar",
		},
	}

	sessionManager := newTestSessionManager(t)
	err := sessionManager.SyncBFDProfiles(pp)
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles %s", err)
	}

	testCheckConfigFile(t)
	err = sessionManager.SyncBFDProfiles(map[string]*config.BFDProfile{})
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles %s", err)
	}
}

func TestBFDProfileThenDelete(t *testing.T) {
	pp := map[string]*config.BFDProfile{
		"foo": {
			Name:             "foo",
			ReceiveInterval:  ptr.To(uint32(60)),
			TransmitInterval: ptr.To(uint32(70)),
			DetectMultiplier: ptr.To(uint32(5)),
			EchoInterval:     ptr.To(uint32(90)),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       ptr.To(uint32(60)),
		},
	}
	sessionManager := newTestSessionManager(t)

	err := sessionManager.SyncBFDProfiles(pp)
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles: %s", err)
	}

	err = sessionManager.SyncBFDProfiles(map[string]*config.BFDProfile{})
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles: %s", err)
	}
	testCheckConfigFile(t)
}
