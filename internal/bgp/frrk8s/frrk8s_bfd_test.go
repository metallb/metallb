// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"net"
	"testing"
	"time"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/pointer"
)

func TestBFDProfileNoSessions(t *testing.T) {
	pp := map[string]*config.BFDProfile{
		"foo": {
			Name:             "foo",
			ReceiveInterval:  pointer.Uint32Ptr(60),
			TransmitInterval: pointer.Uint32Ptr(70),
			DetectMultiplier: pointer.Uint32Ptr(5),
			EchoInterval:     pointer.Uint32Ptr(90),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       pointer.Uint32Ptr(60),
		},
		"bar": {
			Name:             "bar",
			ReceiveInterval:  pointer.Uint32Ptr(60),
			TransmitInterval: pointer.Uint32Ptr(70),
			DetectMultiplier: pointer.Uint32Ptr(5),
			EchoInterval:     pointer.Uint32Ptr(90),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       pointer.Uint32Ptr(60),
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
			ReceiveInterval:  pointer.Uint32Ptr(60),
			TransmitInterval: pointer.Uint32Ptr(70),
			DetectMultiplier: pointer.Uint32Ptr(5),
			EchoInterval:     pointer.Uint32Ptr(90),
			EchoMode:         true,
			PassiveMode:      true,
			MinimumTTL:       pointer.Uint32Ptr(60),
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
			ReceiveInterval:  pointer.Uint32Ptr(60),
			TransmitInterval: pointer.Uint32Ptr(70),
			DetectMultiplier: pointer.Uint32Ptr(5),
			EchoInterval:     pointer.Uint32Ptr(90),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       pointer.Uint32Ptr(60),
		},
		"bar": {
			Name:             "bar",
			ReceiveInterval:  pointer.Uint32Ptr(60),
			TransmitInterval: pointer.Uint32Ptr(70),
			DetectMultiplier: pointer.Uint32Ptr(5),
			EchoInterval:     pointer.Uint32Ptr(90),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       pointer.Uint32Ptr(60),
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
			HoldTime:      time.Second,
			KeepAliveTime: 2 * time.Second,
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
			ReceiveInterval:  pointer.Uint32Ptr(60),
			TransmitInterval: pointer.Uint32Ptr(70),
			DetectMultiplier: pointer.Uint32Ptr(5),
			EchoInterval:     pointer.Uint32Ptr(90),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       pointer.Uint32Ptr(60),
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
			ReceiveInterval:  pointer.Uint32Ptr(60),
			TransmitInterval: pointer.Uint32Ptr(70),
			DetectMultiplier: pointer.Uint32Ptr(5),
			EchoInterval:     pointer.Uint32Ptr(90),
			EchoMode:         false,
			PassiveMode:      false,
			MinimumTTL:       pointer.Uint32Ptr(60),
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
