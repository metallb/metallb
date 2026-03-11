// SPDX-License-Identifier:Apache-2.0

package tlsconfig

import (
	"crypto/tls"
	"fmt"
	"math"
	"strconv"
	"strings"

	cliflag "k8s.io/component-base/cli/flag"
)

// OptFor parses the given cipher suites, curve preferences, and min version
// strings and returns a tls.Config modifier.
//
// Curve preferences are specified as comma-separated numeric CurveID values
// (see https://pkg.go.dev/crypto/tls#CurveID), matching the Kubernetes
// --tls-curve-preferences flag format (kubernetes/kubernetes#137115).
//
// Returns an error if cipher suites are specified with TLS 1.3 minimum, since
// Go's TLS 1.3 does not allow configuring cipher suites -- all TLS 1.3
// ciphers are always enabled. See: https://github.com/golang/go/issues/29349
func OptFor(cipherSuites, curvePreferences, minVersion string) (func(*tls.Config), error) {
	ciphers, err := parseCipherSuites(cipherSuites)
	if err != nil {
		return nil, fmt.Errorf("parsing tls-cipher-suites: %w", err)
	}
	curves, err := parseCurvePreferences(curvePreferences)
	if err != nil {
		return nil, fmt.Errorf("parsing tls-curve-preferences: %w", err)
	}
	minVer, err := parseTLSVersion(minVersion)
	if err != nil {
		return nil, fmt.Errorf("parsing tls-min-version: %w", err)
	}
	if ciphers != nil && minVer == tls.VersionTLS13 {
		return nil, fmt.Errorf("cipher suites cannot be configured with TLS 1.3")
	}
	return func(cfg *tls.Config) {
		if ciphers != nil {
			cfg.CipherSuites = ciphers
		}
		if curves != nil {
			cfg.CurvePreferences = curves
		}
		cfg.MinVersion = minVer
	}, nil
}

func parseCurvePreferences(curves string) ([]tls.CurveID, error) {
	if curves == "" {
		return nil, nil
	}
	parts := strings.Split(curves, ",")
	curveIDs := make([]int32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid curve preference %q: must be a numeric CurveID (0-%d)", p, ^uint16(0))
		}
		curveIDs = append(curveIDs, int32(n))
	}
	return tlsCurvePreferences(curveIDs)
}

// tlsCurvePreferences returns a list of Go's crypto/tls CurveID values from the ids passed.
// The supported values depend on the Go version used.
// See https://pkg.go.dev/crypto/tls#CurveID for values supported for each Go version.
// This mirrors cliflag.TLSCurvePreferences from component-base v0.36+
// (kubernetes/kubernetes#137115), inlined here until we bump the dependency.
func tlsCurvePreferences(curveIDs []int32) ([]tls.CurveID, error) {
	if len(curveIDs) == 0 {
		return nil, nil
	}
	seen := make(map[int32]bool, len(curveIDs))
	result := make([]tls.CurveID, 0, len(curveIDs))
	for _, id := range curveIDs {
		if id <= 0 || id > math.MaxUint16 {
			return nil, fmt.Errorf("curve preference %d is out of range (must be 1-%d)", id, math.MaxUint16)
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate curve preference %d", id)
		}
		seen[id] = true
		curve := tls.CurveID(id)
		if strings.HasPrefix(curve.String(), "CurveID(") {
			return nil, fmt.Errorf("curve preference %d is not supported by the current Go version", id)
		}
		result = append(result, curve)
	}
	return result, nil
}

func parseCipherSuites(ciphers string) ([]uint16, error) {
	if ciphers == "" {
		return nil, nil
	}
	names := strings.Split(ciphers, ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}
	return cliflag.TLSCipherSuites(names)
}

func parseTLSVersion(version string) (uint16, error) {
	if version == "" {
		return tls.VersionTLS13, nil
	}
	return cliflag.TLSVersion(version)
}
