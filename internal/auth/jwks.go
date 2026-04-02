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

// jwksRefreshInterval is how often the background goroutine refreshes the JWKS key.
const jwksRefreshInterval = 1 * time.Hour

// jwksMaxRetries is the maximum number of retry attempts for initial fetch.
const jwksMaxRetries = 10

// jwksInitialBackoff is the starting backoff duration for retries.
const jwksInitialBackoff = 1 * time.Second

// jwksMaxBackoff is the maximum backoff duration between retries.
const jwksMaxBackoff = 60 * time.Second

// jwksHTTPTimeout is the HTTP client timeout for JWKS fetches.
const jwksHTTPTimeout = 10 * time.Second

// jwksResponse represents the JSON structure of a JWKS endpoint response.
type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

// jwkKey represents a single JSON Web Key in the JWKS response.
type jwkKey struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	Alg string `json:"alg"`
}

// jwksMinRefreshInterval is the minimum time between on-demand RefreshNow calls.
// Prevents excessive JWKS fetches when multiple requests fail simultaneously.
const jwksMinRefreshInterval = 30 * time.Second

// JWKSFetcher fetches and caches an ECDSA P-256 public key from a JWKS endpoint.
// It supports startup fetch with retry, background refresh, on-demand refresh
// (for key rotation handling), and thread-safe access.
type JWKSFetcher struct {
	jwksURL         string
	mu              sync.RWMutex
	publicKey       *ecdsa.PublicKey
	lastRefresh     time.Time // for rate-limiting on-demand RefreshNow calls
	refreshInterval time.Duration
	client          *http.Client
	// onRefresh is called after a successful background refresh with the new key.
	// Used by serve.go to update the JWT validator's key.
	onRefresh func(key *ecdsa.PublicKey)
}

// NewJWKSFetcher creates a new JWKS fetcher for the given URL.
func NewJWKSFetcher(jwksURL string) *JWKSFetcher {
	return &JWKSFetcher{
		jwksURL:         jwksURL,
		refreshInterval: jwksRefreshInterval,
		client: &http.Client{
			Timeout: jwksHTTPTimeout,
		},
	}
}

// SetOnRefresh sets a callback that is invoked after each successful background
// key refresh. The callback receives the newly fetched public key.
func (f *JWKSFetcher) SetOnRefresh(fn func(key *ecdsa.PublicKey)) {
	f.onRefresh = fn
}

// FetchWithRetry fetches the JWKS key with exponential backoff retry.
// It retries up to jwksMaxRetries times, starting at jwksInitialBackoff and
// doubling up to jwksMaxBackoff. Returns an error if all attempts fail or
// the context is cancelled.
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

	// Unreachable, but satisfies the compiler.
	return fmt.Errorf("JWKS fetch failed after %d attempts", jwksMaxRetries)
}

// GetPublicKey returns the currently cached public key. Thread-safe.
// Returns nil if no key has been fetched yet.
func (f *JWKSFetcher) GetPublicKey() *ecdsa.PublicKey {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.publicKey
}

// StartRefreshLoop starts a background goroutine that periodically re-fetches
// the JWKS key. On failure, the existing key is kept and a warning is logged.
// The loop exits when the context is cancelled.
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

// RefreshNow triggers an immediate JWKS key refresh, rate-limited to once per
// jwksMinRefreshInterval. Used by the auth callback to handle key rotation:
// when a JWT validation fails, the middleware calls RefreshNow and retries once.
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

// fetch performs a single HTTP GET to the JWKS URL and parses the first
// EC P-256 ES256 key from the response.
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

	// Find the first EC P-256 ES256 key.
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
