// SPDX-License-Identifier:Apache-2.0

package speakerlist

import (
	"bytes"
	"crypto/sha256"
	"testing"
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
