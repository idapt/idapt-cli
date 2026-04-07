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

func SetupCertMagic(domain, email string) (*tls.Config, http.Handler, error) {
	if domain == "" || strings.Contains(domain, "localhost") {
		selfSigned, err := SelfSignedConfig(domain)
		if err != nil {
			return nil, nil, fmt.Errorf("self-signed for localhost: %w", err)
		}
		return selfSigned, nil, nil
	}

	certmagic.DefaultACME.Email = email
	certmagic.DefaultACME.Agreed = true

	certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA

	dataDir := os.Getenv("IDAPT_CERT_DIR")
	if dataDir == "" {
		dataDir = "/var/lib/idapt/certs"
	}
	certmagic.Default.Storage = &certmagic.FileStorage{Path: dataDir}

	cfg := certmagic.NewDefault()

	tlsConfig := cfg.TLSConfig()
	tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)

	if err := cfg.ManageAsync(context.Background(), []string{domain}); err != nil {
		return nil, nil, fmt.Errorf("certmagic manage %s: %w", domain, err)
	}

	acmeIssuer := certmagic.DefaultACME
	return tlsConfig, acmeIssuer.HTTPChallengeHandler(http.NewServeMux()), nil
}

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
