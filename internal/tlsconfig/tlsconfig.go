// SPDX-License-Identifier:Apache-2.0

package tlsconfig

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	certutil "k8s.io/client-go/util/cert"
	cliflag "k8s.io/component-base/cli/flag"
)

// TLSOptFor parses the given cipher suites, curve preferences, and min version
// strings and returns a tls.Config modifier.
//
// Returns an error if cipher suites are specified with TLS 1.3 minimum, since
// Go's TLS 1.3 does not allow configuring cipher suites -- all TLS 1.3
// ciphers are always enabled. See: https://github.com/golang/go/issues/29349
func TLSOptFor(cipherSuites, curvePreferences, minVersion string) (func(*tls.Config), error) {
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
		return nil, fmt.Errorf("cipher suites cannot be configured with TLS 1.3 (Go ignores them, see https://github.com/golang/go/issues/29349)")
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

// SelfSignedCert generates a self-signed TLS certificate in memory using
// the hostname as the CN. Copied from kube-rbac-proxy self-signed cert logic:
// https://github.com/brancz/kube-rbac-proxy/blob/v0.21.0/cmd/kube-rbac-proxy/app/kube-rbac-proxy.go#L327-L340
func SelfSignedCert() (tls.Certificate, error) {
	host, err := os.Hostname()
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("getting hostname: %w", err)
	}
	certBytes, keyBytes, err := certutil.GenerateSelfSignedCertKey(host, nil, nil)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generating self-signed cert: %w", err)
	}
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("loading self-signed cert: %w", err)
	}
	return cert, nil
}

// curveNameToID maps curve name strings to tls.CurveID values.
// Supports IANA names (https://www.iana.org/assignments/tls-parameters/tls-parameters.xhtml#tls-parameters-8),
// Go constant names (crypto/tls), and OpenShift API TLSCurve enum names
// (https://github.com/openshift/api/pull/2583).
//
// Neither Go nor Kubernetes currently provide a string-to-CurveID parser.
// This map may become obsolete if one is added to crypto/tls (golang/go#77712)
// or component-base (kubernetes/kubernetes#137115).
var curveNameToID = map[string]tls.CurveID{
	"x25519":         tls.X25519,
	"X25519":         tls.X25519,
	"secp256r1":      tls.CurveP256,
	"secp384r1":      tls.CurveP384,
	"secp521r1":      tls.CurveP521,
	"X25519MLKEM768": tls.X25519MLKEM768,
	"CurveP256":      tls.CurveP256,
	"CurveP384":      tls.CurveP384,
	"CurveP521":      tls.CurveP521,
}

func curveNamesToIDs(names []string) ([]tls.CurveID, error) {
	if len(names) == 0 {
		return nil, nil
	}
	ids := make([]tls.CurveID, 0, len(names))
	for _, n := range names {
		id, ok := curveNameToID[n]
		if !ok {
			return nil, fmt.Errorf("unknown curve %q", n)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseCurvePreferences(curves string) ([]tls.CurveID, error) {
	if curves == "" {
		return nil, nil
	}
	names := strings.Split(curves, ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}
	return curveNamesToIDs(names)
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
