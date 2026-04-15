package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"
)

const jwksRefreshInterval = 1 * time.Hour

const jwksMaxRetries = 10

const jwksInitialBackoff = 1 * time.Second

const jwksMaxBackoff = 60 * time.Second

const jwksHTTPTimeout = 10 * time.Second

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	Alg string `json:"alg"`
}

const jwksMinRefreshInterval = 30 * time.Second

type JWKSFetcher struct {
	jwksURL         string
	mu              sync.RWMutex
	publicKey       *ecdsa.PublicKey
	lastRefresh     time.Time // for rate-limiting on-demand RefreshNow calls
	refreshInterval time.Duration
	client          *http.Client
	onRefresh func(key *ecdsa.PublicKey)
}

func NewJWKSFetcher(jwksURL string) *JWKSFetcher {
	return &JWKSFetcher{
		jwksURL:         jwksURL,
		refreshInterval: jwksRefreshInterval,
		client: &http.Client{
			Timeout: jwksHTTPTimeout,
		},
	}
}

func (f *JWKSFetcher) SetOnRefresh(fn func(key *ecdsa.PublicKey)) {
	f.onRefresh = fn
}

func (f *JWKSFetcher) FetchWithRetry(ctx context.Context) error {
	backoff := jwksInitialBackoff

	for attempt := 0; attempt < jwksMaxRetries; attempt++ {
		key, err := f.fetch()
		if err == nil {
			f.mu.Lock()
			f.publicKey = key
			f.mu.Unlock()
			return nil
		}

		if attempt == jwksMaxRetries-1 {
			return fmt.Errorf("JWKS fetch failed after %d attempts: %w", jwksMaxRetries, err)
		}

		log.Printf("JWKS fetch attempt %d/%d failed: %v (retrying in %s)", attempt+1, jwksMaxRetries, err, backoff)

		select {
		case <-ctx.Done():
			return fmt.Errorf("JWKS fetch cancelled: %w", ctx.Err())
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > jwksMaxBackoff {
			backoff = jwksMaxBackoff
		}
	}

	return fmt.Errorf("JWKS fetch failed after %d attempts", jwksMaxRetries)
}

func (f *JWKSFetcher) GetPublicKey() *ecdsa.PublicKey {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.publicKey
}

func (f *JWKSFetcher) StartRefreshLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(f.refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				key, err := f.fetch()
				if err != nil {
					log.Printf("WARN: JWKS background refresh failed (keeping existing key): %v", err)
					continue
				}

				f.mu.Lock()
				f.publicKey = key
				f.lastRefresh = time.Now()
				f.mu.Unlock()

				log.Printf("JWKS key refreshed successfully")

				if f.onRefresh != nil {
					f.onRefresh(key)
				}
			}
		}
	}()
}

func (f *JWKSFetcher) RefreshNow() error {
	f.mu.RLock()
	tooSoon := time.Since(f.lastRefresh) < jwksMinRefreshInterval
	f.mu.RUnlock()
	if tooSoon {
		return nil // rate-limited
	}

	key, err := f.fetch()
	if err != nil {
		return err
	}

	f.mu.Lock()
	f.publicKey = key
	f.lastRefresh = time.Now()
	f.mu.Unlock()

	log.Printf("JWKS key refreshed on-demand (validation retry)")

	if f.onRefresh != nil {
		f.onRefresh(key)
	}
	return nil
}

func (f *JWKSFetcher) fetch() (*ecdsa.PublicKey, error) {
	resp, err := f.client.Get(f.jwksURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", f.jwksURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read JWKS response body: %w", err)
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("parse JWKS JSON: %w", err)
	}

	for _, key := range jwks.Keys {
		if key.Kty != "EC" || key.Crv != "P-256" || key.Alg != "ES256" {
			continue
		}

		xBytes, err := base64.RawURLEncoding.DecodeString(key.X)
		if err != nil {
			return nil, fmt.Errorf("decode JWK x coordinate: %w", err)
		}

		yBytes, err := base64.RawURLEncoding.DecodeString(key.Y)
		if err != nil {
			return nil, fmt.Errorf("decode JWK y coordinate: %w", err)
		}

		x := new(big.Int).SetBytes(xBytes)
		y := new(big.Int).SetBytes(yBytes)

		curve := elliptic.P256()
		if !curve.IsOnCurve(x, y) {
			return nil, fmt.Errorf("JWK point (x, y) is not on the P-256 curve")
		}

		return &ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		}, nil
	}

	return nil, fmt.Errorf("no EC P-256 ES256 key found in JWKS response (found %d keys)", len(jwks.Keys))
}
