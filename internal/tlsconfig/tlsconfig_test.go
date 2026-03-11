// SPDX-License-Identifier:Apache-2.0

package tlsconfig

import (
	"crypto/tls"
	"testing"
)

func TestTLSOptFor_Defaults(t *testing.T) {
	opt, err := TLSOptFor("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := &tls.Config{}
	opt(cfg)
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("expected default min version TLS 1.3, got %d", cfg.MinVersion)
	}
	if len(cfg.CipherSuites) != 0 {
		t.Errorf("expected no cipher suites, got %v", cfg.CipherSuites)
	}
	if len(cfg.CurvePreferences) != 0 {
		t.Errorf("expected no curve preferences, got %v", cfg.CurvePreferences)
	}
}

func TestTLSOptFor_CipherSuitesRejectedWithTLS13(t *testing.T) {
	_, err := TLSOptFor("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "", "")
	if err == nil {
		t.Fatal("expected error when specifying cipher suites with TLS 1.3 default")
	}
	_, err = TLSOptFor("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "", "VersionTLS13")
	if err == nil {
		t.Fatal("expected error when specifying cipher suites with explicit TLS 1.3")
	}
}

func TestTLSOptFor_CipherSuitesAllowedWithTLS12(t *testing.T) {
	opt, err := TLSOptFor("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "", "VersionTLS12")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := &tls.Config{}
	opt(cfg)
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("expected min version TLS 1.2, got %d", cfg.MinVersion)
	}
	if len(cfg.CipherSuites) != 1 {
		t.Errorf("expected 1 cipher suite, got %d", len(cfg.CipherSuites))
	}
}

func TestTLSOptFor_CurvePreferencesIANA(t *testing.T) {
	opt, err := TLSOptFor("", "x25519,secp256r1,secp384r1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := &tls.Config{}
	opt(cfg)
	expected := []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}
	if len(cfg.CurvePreferences) != len(expected) {
		t.Fatalf("expected %d curves, got %d", len(expected), len(cfg.CurvePreferences))
	}
	for i, c := range cfg.CurvePreferences {
		if c != expected[i] {
			t.Errorf("curve[%d]: expected %v, got %v", i, expected[i], c)
		}
	}
}

func TestTLSOptFor_CurvePreferencesGoNames(t *testing.T) {
	opt, err := TLSOptFor("", "CurveP256,CurveP384,CurveP521", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := &tls.Config{}
	opt(cfg)
	expected := []tls.CurveID{tls.CurveP256, tls.CurveP384, tls.CurveP521}
	if len(cfg.CurvePreferences) != len(expected) {
		t.Fatalf("expected %d curves, got %d", len(expected), len(cfg.CurvePreferences))
	}
	for i, c := range cfg.CurvePreferences {
		if c != expected[i] {
			t.Errorf("curve[%d]: expected %v, got %v", i, expected[i], c)
		}
	}
}

func TestTLSOptFor_X25519MLKEM768(t *testing.T) {
	opt, err := TLSOptFor("", "X25519MLKEM768,x25519", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := &tls.Config{}
	opt(cfg)
	if len(cfg.CurvePreferences) != 2 {
		t.Fatalf("expected 2 curves, got %d", len(cfg.CurvePreferences))
	}
	if cfg.CurvePreferences[0] != tls.X25519MLKEM768 {
		t.Errorf("expected X25519MLKEM768, got %v", cfg.CurvePreferences[0])
	}
	if cfg.CurvePreferences[1] != tls.X25519 {
		t.Errorf("expected X25519, got %v", cfg.CurvePreferences[1])
	}
}

func TestTLSOptFor_UnknownCurve(t *testing.T) {
	_, err := TLSOptFor("", "unknown_curve", "")
	if err == nil {
		t.Fatal("expected error for unknown curve")
	}
}

func TestTLSOptFor_InvalidMinVersion(t *testing.T) {
	_, err := TLSOptFor("", "", "VersionSSL30")
	if err == nil {
		t.Fatal("expected error for invalid min version")
	}
}

func TestSelfSignedCert(t *testing.T) {
	cert, err := SelfSignedCert()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Error("expected at least one certificate")
	}
	if cert.PrivateKey == nil {
		t.Error("expected non-nil private key")
	}
}
