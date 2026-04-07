package listener

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
	"log"
	"math/big"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

type ListenerManager struct {
	mu        sync.Mutex
	listeners map[int]*http.Server // port → running TLS server
	handler   http.Handler         // shared mux (auth + proxy)
	tlsConfig *tls.Config          // CertMagic TLS config (ACME cert)
	ipCert    *tls.Certificate     // self-signed cert with IP SAN
	domain    string
	publicIP  string          // bind dynamic listeners to this IP (avoids conflict with 127.0.0.1 services)
	errCh     chan<- error // propagate fatal listener errors
}

func New(handler http.Handler, tlsConfig *tls.Config, domain string, errCh chan<- error) *ListenerManager {
	return &ListenerManager{
		listeners: make(map[int]*http.Server),
		handler:   handler,
		tlsConfig: tlsConfig,
		domain:    domain,
		errCh:     errCh,
	}
}

func (lm *ListenerManager) SetPublicIP(ip string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.publicIP = ip
}

func (lm *ListenerManager) bindAddr(port int) string {
	if lm.publicIP != "" {
		return fmt.Sprintf("%s:%d", lm.publicIP, port)
	}
	return fmt.Sprintf(":%d", port)
}

func (lm *ListenerManager) Reconcile(tcpPorts []int) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	desired := make(map[int]bool, len(tcpPorts))
	for _, p := range tcpPorts {
		if p == 22 || p == 80 || p == 443 {
			continue
		}
		desired[p] = true
	}

	for port, srv := range lm.listeners {
		if !desired[port] {
			log.Printf("Stopping TLS listener on :%d (rule removed)", port)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			srv.Shutdown(ctx)
			cancel()
			delete(lm.listeners, port)
		}
	}

	for port := range desired {
		if _, exists := lm.listeners[port]; exists {
			continue // already running
		}
		lm.startListener(port)
	}
}

func (lm *ListenerManager) startListener(port int) {
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithListenerPort(r.Context(), port)
		lm.handler.ServeHTTP(w, r.WithContext(ctx))
	})

	srv := &http.Server{
		Addr:      lm.bindAddr(port),
		Handler:   wrappedHandler,
		TLSConfig: lm.buildTLSConfig(),
	}

	lm.listeners[port] = srv

	go func() {
		log.Printf("Starting TLS listener on :%d", port)
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("TLS listener :%d failed: %v (port may be in use — service should bind to 127.0.0.1)", port, err)
			lm.mu.Lock()
			delete(lm.listeners, port)
			lm.mu.Unlock()
		}
	}()
}

func (lm *ListenerManager) buildTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if lm.tlsConfig != nil && lm.tlsConfig.GetCertificate != nil {
				cert, err := lm.tlsConfig.GetCertificate(hello)
				if err == nil && cert != nil {
					return cert, nil
				}
			}

			if lm.ipCert != nil {
				return lm.ipCert, nil
			}

			return nil, fmt.Errorf("no certificate available for %s", hello.ServerName)
		},
	}
}

func (lm *ListenerManager) SetIPCert(ip string, domain string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if ip == "" {
		lm.ipCert = nil
		return nil
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	dnsNames := []string{}
	if domain != "" {
		dnsNames = append(dnsNames, domain)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: domain},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		DNSNames:     dnsNames,
		IPAddresses:  []net.IP{parsedIP},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("load keypair: %w", err)
	}

	lm.ipCert = &cert
	log.Printf("Generated self-signed cert for IP %s (domain: %s)", ip, domain)
	return nil
}

func (lm *ListenerManager) ActivePorts() []int {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	ports := make([]int, 0, len(lm.listeners))
	for p := range lm.listeners {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

func (lm *ListenerManager) Shutdown(ctx context.Context) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	for port, srv := range lm.listeners {
		log.Printf("Shutting down TLS listener on :%d", port)
		srv.Shutdown(ctx)
	}
	lm.listeners = make(map[int]*http.Server)
}
