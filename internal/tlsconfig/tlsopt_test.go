// SPDX-License-Identifier:Apache-2.0

package tlsconfig

import (
	"crypto/tls"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestParseCurvePreferences(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		want       []tls.CurveID
		wantErrMsg string
	}{
		{
			name:  "empty returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "single curve",
			input: "29",
			want:  []tls.CurveID{tls.X25519},
		},
		{
			name:  "multiple curves",
			input: "29,23,24",
			want:  []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384},
		},
		{
			name:  "all standard curves with PQC",
			input: "23,24,25,29,4588",
			want:  []tls.CurveID{tls.CurveP256, tls.CurveP384, tls.CurveP521, tls.X25519, tls.X25519MLKEM768},
		},
		{
			name:  "spaces are trimmed",
			input: "29, 23, 4588",
			want:  []tls.CurveID{tls.X25519, tls.CurveP256, tls.X25519MLKEM768},
		},
		{
			name:       "non-numeric input returns error",
			input:      "X25519",
			wantErrMsg: "invalid curve preference",
		},
		{
			name:       "out of range returns error",
			input:      "99999",
			wantErrMsg: "invalid curve preference",
		},
		{
			name:       "negative value returns error",
			input:      "-1",
			wantErrMsg: "invalid curve preference",
		},
		{
			name:       "zero returns error",
			input:      "0",
			wantErrMsg: "is out of range",
		},
		{
			name:       "duplicate returns error",
			input:      "29,29",
			wantErrMsg: "duplicate curve preference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCurvePreferences(tt.input)
			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("parseCurvePreferences(%q) expected error containing %q, got nil", tt.input, tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("parseCurvePreferences(%q) error %q does not contain %q", tt.input, err.Error(), tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCurvePreferences(%q) unexpected error: %v", tt.input, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("parseCurvePreferences(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantVer    uint16
		wantErrMsg string
	}{
		{
			name:    "empty defaults to TLS 1.3",
			input:   "",
			wantVer: tls.VersionTLS13,
		},
		{
			name:    "explicit TLS 1.3",
			input:   "VersionTLS13",
			wantVer: tls.VersionTLS13,
		},
		{
			name:    "explicit TLS 1.2",
			input:   "VersionTLS12",
			wantVer: tls.VersionTLS12,
		},
		{
			name:       "invalid version",
			input:      "VersionTLS99",
			wantErrMsg: "VersionTLS99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTLSVersion(tt.input)
			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("parseTLSVersion(%q) expected error containing %q, got nil", tt.input, tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("parseTLSVersion(%q) error %q does not contain %q", tt.input, err.Error(), tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTLSVersion(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.wantVer {
				t.Errorf("parseTLSVersion(%q) = %v, want %v", tt.input, got, tt.wantVer)
			}
		})
	}
}

func TestOptFor(t *testing.T) {
	tests := []struct {
		name                 string
		cipherSuites         string
		curvePreferences     string
		minVersion           string
		wantErrMsg           string
		wantCipherSuites     []uint16
		wantCurvePreferences []tls.CurveID
		wantMinVersion       uint16
	}{
		{
			name:                 "empty flags use defaults",
			wantCipherSuites:     nil,
			wantCurvePreferences: nil,
			wantMinVersion:       tls.VersionTLS13,
		},
		{
			name:                 "TLS 1.2 with cipher and curves",
			cipherSuites:         "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			curvePreferences:     fmt.Sprintf("%d,%d", tls.X25519, tls.X25519MLKEM768),
			minVersion:           "VersionTLS12",
			wantCipherSuites:     []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			wantCurvePreferences: []tls.CurveID{tls.X25519, tls.X25519MLKEM768},
			wantMinVersion:       tls.VersionTLS12,
		},
		{
			name:                 "TLS 1.3 without ciphers is fine",
			curvePreferences:     fmt.Sprintf("%d", tls.X25519),
			minVersion:           "VersionTLS13",
			wantCipherSuites:     nil,
			wantCurvePreferences: []tls.CurveID{tls.X25519},
			wantMinVersion:       tls.VersionTLS13,
		},
		{
			name:         "rejects cipher suites with TLS 1.3",
			cipherSuites: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			minVersion:   "VersionTLS13",
			wantErrMsg:   "cipher suites cannot be configured with TLS 1.3",
		},
		{
			name:         "invalid cipher returns error",
			cipherSuites: "FAKE_CIPHER",
			wantErrMsg:   "parsing tls-cipher-suites",
		},
		{
			name:             "non-numeric curve returns error",
			curvePreferences: "X25519",
			wantErrMsg:       "parsing tls-curve-preferences",
		},
		{
			name:       "invalid version returns error",
			minVersion: "VersionTLS99",
			wantErrMsg: "parsing tls-min-version",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opt, err := OptFor(tc.cipherSuites, tc.curvePreferences, tc.minVersion)
			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("OptFor() unexpected error: %v", err)
			}

			cfg := &tls.Config{}
			opt(cfg)

			if !slices.Equal(cfg.CipherSuites, tc.wantCipherSuites) {
				t.Errorf("CipherSuites = %v, want %v", cfg.CipherSuites, tc.wantCipherSuites)
			}
			if !slices.Equal(cfg.CurvePreferences, tc.wantCurvePreferences) {
				t.Errorf("CurvePreferences = %v, want %v", cfg.CurvePreferences, tc.wantCurvePreferences)
			}
			if cfg.MinVersion != tc.wantMinVersion {
				t.Errorf("MinVersion = %v, want %v", cfg.MinVersion, tc.wantMinVersion)
			}
		})
	}
}
