package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSelfSignedConfig_ValidCert(t *testing.T) {
	domain := "test.example.com"
	tlsConfig, err := SelfSignedConfig(domain)
	if err != nil {
		t.Fatalf("SelfSignedConfig(%q) error: %v", domain, err)
	}
	if tlsConfig == nil {
		t.Fatal("SelfSignedConfig returned nil config")
	}
	if len(tlsConfig.Certificates) == 0 {
		t.Fatal("TLS config has no certificates")
	}

	// Parse the leaf certificate and verify it's valid
	cert := tlsConfig.Certificates[0]
	if len(cert.Certificate) == 0 {
		t.Fatal("certificate chain is empty")
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf certificate: %v", err)
	}

	if leaf.Subject.CommonName != domain {
		t.Errorf("CommonName = %q, want %q", leaf.Subject.CommonName, domain)
	}
}

func TestSelfSignedConfig_DomainInSAN(t *testing.T) {
	domain := "myhost.idapt.app"
	tlsConfig, err := SelfSignedConfig(domain)
	if err != nil {
		t.Fatalf("SelfSignedConfig(%q) error: %v", domain, err)
	}

	leaf, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	found := false
	for _, san := range leaf.DNSNames {
		if san == domain {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("domain %q not found in SANs: %v", domain, leaf.DNSNames)
	}
}

func TestSelfSignedConfig_NotExpired(t *testing.T) {
	domain := "fresh.example.com"
	tlsConfig, err := SelfSignedConfig(domain)
	if err != nil {
		t.Fatalf("SelfSignedConfig(%q) error: %v", domain, err)
	}

	leaf, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	now := time.Now()
	if now.Before(leaf.NotBefore) {
		t.Errorf("certificate NotBefore (%v) is in the future", leaf.NotBefore)
	}
	if now.After(leaf.NotAfter) {
		t.Errorf("certificate NotAfter (%v) is in the past", leaf.NotAfter)
	}

	// Verify cert is valid for a reasonable duration (at least 30 days)
	remaining := leaf.NotAfter.Sub(now)
	if remaining < 30*24*time.Hour {
		t.Errorf("certificate expires in %v, expected at least 30 days", remaining)
	}
}

func TestSetupCertMagic_LocalhostSkips(t *testing.T) {
	// When domain contains "localhost", SetupCertMagic should NOT attempt
	// ACME (which would fail). The current implementation returns an error
	// for localhost domains. The desired behavior after the fix is to fall
	// back to a self-signed config instead of erroring.
	//
	// This is a TDD test: it will FAIL until the production code is updated
	// to return a self-signed config for localhost domains.
	domain := "localhost"
	email := "test@example.com"

	tlsConfig, _, err := SetupCertMagic(domain, email)
	if err != nil {
		t.Fatalf("SetupCertMagic(%q, %q) returned error: %v — expected self-signed fallback for localhost", domain, email, err)
	}
	if tlsConfig == nil {
		t.Fatal("SetupCertMagic returned nil TLS config for localhost — expected self-signed fallback")
	}

	// The returned config should have at least one certificate or a
	// GetCertificate function that works.
	if len(tlsConfig.Certificates) == 0 && tlsConfig.GetCertificate == nil {
		t.Fatal("TLS config has no certificates and no GetCertificate — cannot serve TLS")
	}

	// If it has static certificates, verify the leaf is parseable
	if len(tlsConfig.Certificates) > 0 {
		leaf, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
		if err != nil {
			t.Fatalf("parse self-signed certificate: %v", err)
		}
		// The CN or SAN should reference localhost
		hasLocalhost := leaf.Subject.CommonName == domain
		for _, san := range leaf.DNSNames {
			if san == domain {
				hasLocalhost = true
			}
		}
		if !hasLocalhost {
			t.Errorf("self-signed cert does not reference %q (CN=%q, SANs=%v)", domain, leaf.Subject.CommonName, leaf.DNSNames)
		}
	}

	// If it has GetCertificate, verify it can produce a cert for localhost
	if tlsConfig.GetCertificate != nil {
		cert, err := tlsConfig.GetCertificate(&tls.ClientHelloInfo{
			ServerName: domain,
		})
		if err != nil {
			t.Fatalf("GetCertificate(%q) error: %v", domain, err)
		}
		if cert == nil {
			t.Fatalf("GetCertificate(%q) returned nil", domain)
		}
	}
}

func TestSelfSignedConfig_P256Key(t *testing.T) {
	domain := "p256.example.com"
	tlsConfig, err := SelfSignedConfig(domain)
	if err != nil {
		t.Fatalf("SelfSignedConfig(%q) error: %v", domain, err)
	}

	leaf, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	pubKey, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("public key is %T, want *ecdsa.PublicKey", leaf.PublicKey)
	}

	if pubKey.Curve != elliptic.P256() {
		t.Errorf("curve = %v, want P-256", pubKey.Curve.Params().Name)
	}
}
