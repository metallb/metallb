// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"net"
	"testing"
	"time"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/logging"
	"go.universe.tf/metallb/internal/pointer"
)

func TestBFDProfileNoSessions(t *testing.T) {
	testSetup(t)

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
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

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
	testSetup(t)

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

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

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
	testSetup(t)

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
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

	err := sessionManager.SyncBFDProfiles(pp)
	if err != nil {
		t.Fatalf("Failed to sync bfd profiles %s", err)
	}

	session, err := sessionManager.NewSession(l, "10.2.2.254:179", net.ParseIP("10.1.1.254"), 100, net.ParseIP("10.1.1.254"), 200, time.Second, 2*time.Second, "password", "hostname", "foo", true, "test-peer")
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
	testSetup(t)

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

	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

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
