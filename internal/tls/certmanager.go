package tls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/caddyserver/certmagic"
)

// SetupCertMagic configures automatic TLS via ACME.
// Primary: Let's Encrypt. Fallback: self-signed if ACME fails.
// Returns TLS config, HTTP handler for ACME challenges, and error.
func SetupCertMagic(domain, email string) (*tls.Config, http.Handler, error) {
	// Skip certmagic in development (localhost domains can't get real certs)
	if domain == "" || strings.Contains(domain, "localhost") {
		return nil, nil, fmt.Errorf("certmagic skipped for localhost domain")
	}

	// Configure certmagic
	certmagic.DefaultACME.Email = email
	certmagic.DefaultACME.Agreed = true

	// Use Let's Encrypt production
	certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA

	// Storage: filesystem (default ~/.local/share/certmagic)
	dataDir := os.Getenv("IDAPT_CERT_DIR")
	if dataDir == "" {
		dataDir = "/var/lib/idapt/certs"
	}
	certmagic.Default.Storage = &certmagic.FileStorage{Path: dataDir}

	cfg := certmagic.NewDefault()

	// Get the TLS config and HTTP challenge handler
	tlsConfig := cfg.TLSConfig()
	tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)

	// Manage the domain (async — starts cert acquisition in background)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := cfg.ManageAsync(ctx, []string{domain}); err != nil {
		return nil, nil, fmt.Errorf("certmagic manage %s: %w", domain, err)
	}

	// HTTP challenge handler is on the ACME issuer, not the config
	acmeIssuer := certmagic.DefaultACME
	return tlsConfig, acmeIssuer.HTTPChallengeHandler(http.NewServeMux()), nil
}

// SelfSignedConfig generates a self-signed TLS certificate for the given domain.
// Used as a fallback when ACME is unavailable (e.g., no public DNS, development).
func SelfSignedConfig(domain string) (*tls.Config, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: domain},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		DNSNames:     []string{domain},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load keypair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
