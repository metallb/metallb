// SPDX-License-Identifier:Apache-2.0

package speakerlist

import (
	"bytes"
	"crypto/sha256"
	"net"
	"strconv"
	"testing"

	"github.com/go-kit/log"

	"go.universe.tf/metallb/internal/k8s"
)

func TestSecretKeyFor(t *testing.T) {
	// The digest of the empty input is a public constant, so no part of it
	// may show up in a derived key.
	emptyDigest := sha256.Sum256(nil)

	secrets := []string{
		"s",
		"metallb",
		// 22 characters, the shape of the key CreateMlSecret generates.
		"TB6Su+iJ8AObGAX7x9NEnw",
		// Longer than the key, and sharing the first 32 bytes.
		"0123456789abcdef0123456789abcdefone",
		"0123456789abcdef0123456789abcdeftwo",
	}

	seen := map[string]string{}
	for _, secret := range secrets {
		key := secretKeyFor(secret)
		if len(key) != 32 {
			t.Fatalf("secret %q: got a %d byte key, want 32", secret, len(key))
		}

		inClear := secret
		if len(inClear) > len(key) {
			inClear = inClear[:len(key)]
		}
		if bytes.HasPrefix(key, []byte(inClear)) {
			t.Errorf("secret %q: key %x starts with the secret in the clear", secret, key)
		}
		if bytes.Contains(key, emptyDigest[:8]) {
			t.Errorf("secret %q: key %x is padded with the digest of the empty input", secret, key)
		}
		if other, ok := seen[string(key)]; ok {
			t.Errorf("secrets %q and %q both derive key %x", other, secret, key)
		}
		seen[string(key)] = secret
	}
}

// TestStartJoinsBeforeReturning checks that a speaker knows the running
// speakers as soon as Start returns. Start runs before the Kubernetes client,
// so a member list that holds only the local node at that point makes the
// first layer2 reconcile elect the local node for every service.
func TestStartJoinsBeforeReturning(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	if err := l.Close(); err != nil {
		t.Fatalf("closing the port probe: %v", err)
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	logger := log.NewNopLogger()

	// Two loopback addresses with the same port, as speakers on two nodes:
	// mlJoin passes bare IPs, which memberlist dials on its own bind port.
	peer, err := New(logger, "peer", "127.0.0.2", port, "secret", "metallb-system", "app=speaker", false, stopCh)
	if err != nil {
		t.Fatalf("creating the peer: %v", err)
	}
	defer func() { _ = peer.ml.Shutdown() }()

	self, err := New(logger, "self", "127.0.0.1", port, "secret", "metallb-system", "app=speaker", false, stopCh)
	if err != nil {
		t.Fatalf("creating the local speaker: %v", err)
	}
	defer func() { _ = self.ml.Shutdown() }()

	self.client = &k8s.Client{ForceSync: func() {}}
	self.start([]string{"127.0.0.1", "127.0.0.2"})

	nodes := self.UsableSpeakers().Nodes
	if !nodes["self"] || !nodes["peer"] {
		t.Fatalf("usable speakers right after start: %v, want both self and peer", nodes)
	}
}
