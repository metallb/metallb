package main

import (
	"strings"
	"testing"
)

// This file tracks the subset of test cases that are known to be
// broken with some combinations of network addons or other k8s
// features. Each "feature tag" has a list of glob patterns for the
// tests that cannot pass that feature check.

var broken = map[string][]string{
	"client-ip-correct": {
		// Client source IP assignment is broken in kube-proxy, in
		// that the masquerade IP is not stable. This is reproducible
		// with multiple network addons. Therefore we don't verify it
		// here.
		"*/*/cluster",
		"*/*/shared",
		// Weave masquerades traffic even with externalTrafficPolicy=Local.
		"*/weave/local",
	},
}

func testBroken(t *testing.T, featureName string, f func()) {
	testName := t.Name()
patterns:
	for _, pattern := range broken[featureName] {
		fields := strings.Split(testName, "/")
		patternFields := strings.Split(pattern, "/")
		if len(fields) != len(patternFields) {
			t.Fatalf("Pattern has wrong length for feature test, %q vs. %q", testName, pattern)
		}
		for i := range fields {
			if patternFields[i] != "*" && patternFields[i] != fields[i] {
				continue patterns
			}
		}

		// Pattern matched, don't run the test. Instead run a stub to
		// mark it skipped.
		t.Run(featureName, func(t *testing.T) {
			t.Skip("known broken")
		})
		return
	}

	// No patterns matched, test needs to be run.
	f()
}
