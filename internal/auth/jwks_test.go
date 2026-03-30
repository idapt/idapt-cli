package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// generateTestEC256Key generates a fresh ECDSA P-256 key pair for testing.
func generateTestEC256Key(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	return priv
}

// buildJWKSResponse builds a JWKS JSON response containing the given EC public key.
func buildJWKSResponse(t *testing.T, pub *ecdsa.PublicKey) []byte {
	t.Helper()
	x := base64.RawURLEncoding.EncodeToString(pub.X.Bytes())
	y := base64.RawURLEncoding.EncodeToString(pub.Y.Bytes())

	resp := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "EC",
				"crv": "P-256",
				"alg": "ES256",
				"x":   x,
				"y":   y,
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal JWKS response: %v", err)
	}
	return data
}

// buildJWKSResponseWithAlg builds a JWKS JSON response with a custom algorithm string.
func buildJWKSResponseWithAlg(t *testing.T, pub *ecdsa.PublicKey, alg string) []byte {
	t.Helper()
	x := base64.RawURLEncoding.EncodeToString(pub.X.Bytes())
	y := base64.RawURLEncoding.EncodeToString(pub.Y.Bytes())

	resp := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "EC",
				"crv": "P-256",
				"alg": alg,
				"x":   x,
				"y":   y,
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal JWKS response: %v", err)
	}
	return data
}

// signES256JWT creates a minimal ES256-signed JWT for testing.
func signES256JWT(t *testing.T, privKey *ecdsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()

	headerB64 := base64URLEncode([]byte(`{"alg":"ES256","typ":"JWT"}`))

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	payloadB64 := base64URLEncode(claimsJSON)

	signingInput := headerB64 + "." + payloadB64
	hash := sha256.Sum256([]byte(signingInput))

	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		t.Fatalf("ECDSA sign: %v", err)
	}

	// Raw R||S: 32 bytes each, zero-padded left.
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	rawSig := make([]byte, 64)
	copy(rawSig[32-len(rBytes):32], rBytes)
	copy(rawSig[64-len(sBytes):64], sBytes)

	sigB64 := base64URLEncode(rawSig)
	return signingInput + "." + sigB64
}

// ---------------------------------------------------------------------------
// TestFetch_ValidJWKS
// ---------------------------------------------------------------------------

func TestFetch_ValidJWKS(t *testing.T) {
	priv := generateTestEC256Key(t)
	jwksBody := buildJWKSResponse(t, &priv.PublicKey)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksBody)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	key, err := fetcher.fetch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
	if key.Curve != elliptic.P256() {
		t.Fatalf("expected P-256 curve, got %s", key.Curve.Params().Name)
	}
	if key.X.Cmp(priv.PublicKey.X) != 0 || key.Y.Cmp(priv.PublicKey.Y) != 0 {
		t.Fatal("fetched key does not match original public key")
	}
}

// ---------------------------------------------------------------------------
// TestFetch_EmptyKeysArray
// ---------------------------------------------------------------------------

func TestFetch_EmptyKeysArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	_, err := fetcher.fetch()
	if err == nil {
		t.Fatal("expected error for empty keys array")
	}
	if !strings.Contains(err.Error(), "no EC P-256 ES256 key found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestFetch_InvalidJSON
// ---------------------------------------------------------------------------

func TestFetch_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	_, err := fetcher.fetch()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse JWKS JSON") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestFetch_WrongAlgorithm
// ---------------------------------------------------------------------------

func TestFetch_WrongAlgorithm(t *testing.T) {
	priv := generateTestEC256Key(t)
	// Return a key with RS256 algorithm instead of ES256.
	jwksBody := buildJWKSResponseWithAlg(t, &priv.PublicKey, "RS256")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksBody)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	_, err := fetcher.fetch()
	if err == nil {
		t.Fatal("expected error when no ES256 key found")
	}
	if !strings.Contains(err.Error(), "no EC P-256 ES256 key found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestFetch_NetworkError
// ---------------------------------------------------------------------------

func TestFetch_NetworkError(t *testing.T) {
	// Use a URL that will fail to connect (closed server).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close() // Close immediately to force network error.

	fetcher := NewJWKSFetcher(serverURL)
	_, err := fetcher.fetch()
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

// ---------------------------------------------------------------------------
// TestFetch_HTTPErrorStatus
// ---------------------------------------------------------------------------

func TestFetch_HTTPErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`Internal Server Error`))
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	_, err := fetcher.fetch()
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestFetch_InvalidCoordinates
// ---------------------------------------------------------------------------

func TestFetch_InvalidCoordinates(t *testing.T) {
	// Construct a JWKS response where x,y are not on the P-256 curve.
	resp := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "EC",
				"crv": "P-256",
				"alg": "ES256",
				"x":   base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x02, 0x03}),
				"y":   base64.RawURLEncoding.EncodeToString([]byte{0x04, 0x05, 0x06}),
			},
		},
	}
	jwksBody, _ := json.Marshal(resp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksBody)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	_, err := fetcher.fetch()
	if err == nil {
		t.Fatal("expected error for point not on curve")
	}
	if !strings.Contains(err.Error(), "not on the P-256 curve") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestFetchWithRetry_SuccessOnSecondAttempt
// ---------------------------------------------------------------------------

func TestFetchWithRetry_SuccessOnSecondAttempt(t *testing.T) {
	priv := generateTestEC256Key(t)
	jwksBody := buildJWKSResponse(t, &priv.PublicKey)

	var mu sync.Mutex
	attempt := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		current := attempt
		attempt++
		mu.Unlock()

		if current == 0 {
			// First attempt: fail.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Second attempt: succeed.
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksBody)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	// Override refresh interval is not needed here — FetchWithRetry uses backoff.
	// Use a short backoff by setting the internal constants... but they're package-level.
	// Instead, just run with the default (1s initial backoff is acceptable for tests).

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := fetcher.FetchWithRetry(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key := fetcher.GetPublicKey()
	if key == nil {
		t.Fatal("expected non-nil key after retry")
	}
	if key.X.Cmp(priv.PublicKey.X) != 0 || key.Y.Cmp(priv.PublicKey.Y) != 0 {
		t.Fatal("fetched key does not match original public key")
	}
}

// ---------------------------------------------------------------------------
// TestFetchWithRetry_AllFail
// ---------------------------------------------------------------------------

func TestFetchWithRetry_AllFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)

	// Use a context with a short timeout to avoid waiting for all 10 retries
	// with exponential backoff.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := fetcher.FetchWithRetry(ctx)
	if err == nil {
		t.Fatal("expected error when all retries fail")
	}
}

// ---------------------------------------------------------------------------
// TestFetchWithRetry_ContextCancelled
// ---------------------------------------------------------------------------

func TestFetchWithRetry_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately after first failure attempt.
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	err := fetcher.FetchWithRetry(ctx)
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestGetPublicKey_ThreadSafe
// ---------------------------------------------------------------------------

func TestGetPublicKey_ThreadSafe(t *testing.T) {
	priv := generateTestEC256Key(t)
	jwksBody := buildJWKSResponse(t, &priv.PublicKey)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksBody)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)

	ctx := context.Background()
	if err := fetcher.FetchWithRetry(ctx); err != nil {
		t.Fatalf("initial fetch failed: %v", err)
	}

	// Launch many concurrent reads — should not panic or race.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fetcher.GetPublicKey()
			if key == nil {
				t.Error("expected non-nil key")
			}
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// TestRefresh_UpdatesKey
// ---------------------------------------------------------------------------

func TestRefresh_UpdatesKey(t *testing.T) {
	key1 := generateTestEC256Key(t)
	key2 := generateTestEC256Key(t)

	var mu sync.Mutex
	currentKey := key1

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		k := currentKey
		mu.Unlock()

		body := buildJWKSResponse(t, &k.PublicKey)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)

	// Initial fetch — returns key1.
	ctx := context.Background()
	if err := fetcher.FetchWithRetry(ctx); err != nil {
		t.Fatalf("initial fetch failed: %v", err)
	}

	got := fetcher.GetPublicKey()
	if got.X.Cmp(key1.PublicKey.X) != 0 || got.Y.Cmp(key1.PublicKey.Y) != 0 {
		t.Fatal("expected key1 after initial fetch")
	}

	// Switch server to return key2.
	mu.Lock()
	currentKey = key2
	mu.Unlock()

	// Manually trigger a fetch to simulate refresh.
	newKey, err := fetcher.fetch()
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}

	// Update the fetcher's cached key (simulating what StartRefreshLoop does).
	fetcher.mu.Lock()
	fetcher.publicKey = newKey
	fetcher.mu.Unlock()

	got = fetcher.GetPublicKey()
	if got.X.Cmp(key2.PublicKey.X) != 0 || got.Y.Cmp(key2.PublicKey.Y) != 0 {
		t.Fatal("expected key2 after refresh")
	}
}

// ---------------------------------------------------------------------------
// TestStartRefreshLoop_CallsOnRefresh
// ---------------------------------------------------------------------------

func TestStartRefreshLoop_CallsOnRefresh(t *testing.T) {
	priv := generateTestEC256Key(t)
	jwksBody := buildJWKSResponse(t, &priv.PublicKey)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksBody)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	// Use a very short refresh interval for testing.
	fetcher.refreshInterval = 50 * time.Millisecond

	ctx := context.Background()
	if err := fetcher.FetchWithRetry(ctx); err != nil {
		t.Fatalf("initial fetch failed: %v", err)
	}

	refreshCh := make(chan *ecdsa.PublicKey, 5)
	fetcher.SetOnRefresh(func(key *ecdsa.PublicKey) {
		refreshCh <- key
	})

	refreshCtx, cancel := context.WithCancel(context.Background())
	fetcher.StartRefreshLoop(refreshCtx)

	// Wait for at least one refresh callback.
	select {
	case key := <-refreshCh:
		if key == nil {
			t.Fatal("expected non-nil key in refresh callback")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for refresh callback")
	}

	cancel()
}

// ---------------------------------------------------------------------------
// TestValidateWithJWKSKey — full integration flow
// ---------------------------------------------------------------------------

func TestValidateWithJWKSKey(t *testing.T) {
	priv := generateTestEC256Key(t)
	jwksBody := buildJWKSResponse(t, &priv.PublicKey)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksBody)
	}))
	defer server.Close()

	// Step 1: Fetch JWKS.
	fetcher := NewJWKSFetcher(server.URL)
	ctx := context.Background()
	if err := fetcher.FetchWithRetry(ctx); err != nil {
		t.Fatalf("JWKS fetch failed: %v", err)
	}

	// Step 2: Create validator from fetched key.
	machineID := "mm-test-456"
	validator, err := NewJWTValidatorFromKey(fetcher.GetPublicKey(), machineID)
	if err != nil {
		t.Fatalf("NewJWTValidatorFromKey failed: %v", err)
	}

	// Step 3: Sign a JWT with the private key.
	claims := map[string]interface{}{
		"sub": "actor-789",
		"mid": machineID,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := signES256JWT(t, priv, claims)

	// Step 4: Validate the JWT.
	parsed, err := validator.Validate(token)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if parsed.Sub != "actor-789" {
		t.Fatalf("expected sub=actor-789, got %s", parsed.Sub)
	}
	if parsed.Mid != machineID {
		t.Fatalf("expected mid=%s, got %s", machineID, parsed.Mid)
	}
}

// ---------------------------------------------------------------------------
// TestValidateWithJWKSKey_WrongKey — JWT signed with different key fails
// ---------------------------------------------------------------------------

func TestValidateWithJWKSKey_WrongKey(t *testing.T) {
	fetchedKey := generateTestEC256Key(t)
	signingKey := generateTestEC256Key(t)

	jwksBody := buildJWKSResponse(t, &fetchedKey.PublicKey)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksBody)
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	ctx := context.Background()
	if err := fetcher.FetchWithRetry(ctx); err != nil {
		t.Fatalf("JWKS fetch failed: %v", err)
	}

	machineID := "mm-test-456"
	validator, err := NewJWTValidatorFromKey(fetcher.GetPublicKey(), machineID)
	if err != nil {
		t.Fatalf("NewJWTValidatorFromKey failed: %v", err)
	}

	// Sign with a DIFFERENT key.
	claims := map[string]interface{}{
		"sub": "actor-789",
		"mid": machineID,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := signES256JWT(t, signingKey, claims)

	_, err = validator.Validate(token)
	if err == nil {
		t.Fatal("expected error validating JWT signed with wrong key")
	}
	if !strings.Contains(err.Error(), "invalid signature") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestNewJWTValidatorFromKey — constructor edge cases
// ---------------------------------------------------------------------------

func TestNewJWTValidatorFromKey(t *testing.T) {
	t.Run("NilKey", func(t *testing.T) {
		_, err := NewJWTValidatorFromKey(nil, "mm-123")
		if err == nil {
			t.Fatal("expected error for nil key")
		}
	})

	t.Run("EmptyMachineID", func(t *testing.T) {
		priv := generateTestEC256Key(t)
		_, err := NewJWTValidatorFromKey(&priv.PublicKey, "")
		if err == nil {
			t.Fatal("expected error for empty machine ID")
		}
	})

	t.Run("WrongCurve", func(t *testing.T) {
		p384Key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		if err != nil {
			t.Fatalf("generate P-384 key: %v", err)
		}
		_, err = NewJWTValidatorFromKey(&p384Key.PublicKey, "mm-123")
		if err == nil {
			t.Fatal("expected error for P-384 key")
		}
		if !strings.Contains(err.Error(), "not on P-256 curve") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ValidKey", func(t *testing.T) {
		priv := generateTestEC256Key(t)
		v, err := NewJWTValidatorFromKey(&priv.PublicKey, "mm-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v == nil {
			t.Fatal("expected non-nil validator")
		}
	})
}

// ---------------------------------------------------------------------------
// TestSetPublicKey_UpdatesValidation
// ---------------------------------------------------------------------------

func TestSetPublicKey_UpdatesValidation(t *testing.T) {
	key1 := generateTestEC256Key(t)
	key2 := generateTestEC256Key(t)

	machineID := "mm-rotate-test"

	// Create validator with key1.
	validator, err := NewJWTValidatorFromKey(&key1.PublicKey, machineID)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	claims := map[string]interface{}{
		"sub": "actor-abc",
		"mid": machineID,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	// Token signed with key1 should validate.
	token1 := signES256JWT(t, key1, claims)
	if _, err := validator.Validate(token1); err != nil {
		t.Fatalf("expected key1 token to validate: %v", err)
	}

	// Token signed with key2 should NOT validate yet.
	token2 := signES256JWT(t, key2, claims)
	if _, err := validator.Validate(token2); err == nil {
		t.Fatal("expected key2 token to fail before key rotation")
	}

	// Rotate to key2.
	validator.SetPublicKey(&key2.PublicKey)

	// Now token2 should validate.
	if _, err := validator.Validate(token2); err != nil {
		t.Fatalf("expected key2 token to validate after rotation: %v", err)
	}

	// And token1 should fail.
	if _, err := validator.Validate(token1); err == nil {
		t.Fatal("expected key1 token to fail after rotation")
	}
}

// ---------------------------------------------------------------------------
// TestFetch_MultipleKeys_PicksES256
// ---------------------------------------------------------------------------

func TestFetch_MultipleKeys_PicksES256(t *testing.T) {
	priv := generateTestEC256Key(t)
	x := base64.RawURLEncoding.EncodeToString(priv.PublicKey.X.Bytes())
	y := base64.RawURLEncoding.EncodeToString(priv.PublicKey.Y.Bytes())

	// JWKS with multiple keys: first is RS256 (should be skipped), second is ES256.
	resp := fmt.Sprintf(`{
		"keys": [
			{"kty": "RSA", "alg": "RS256", "n": "abc", "e": "AQAB"},
			{"kty": "EC", "crv": "P-256", "alg": "ES256", "x": %q, "y": %q}
		]
	}`, x, y)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	key, err := fetcher.fetch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.X.Cmp(priv.PublicKey.X) != 0 || key.Y.Cmp(priv.PublicKey.Y) != 0 {
		t.Fatal("fetched key does not match expected ES256 key")
	}
}

// ---------------------------------------------------------------------------
// TestGetPublicKey_NilBeforeFetch
// ---------------------------------------------------------------------------

func TestGetPublicKey_NilBeforeFetch(t *testing.T) {
	fetcher := NewJWKSFetcher("http://localhost:0/jwks")
	key := fetcher.GetPublicKey()
	if key != nil {
		t.Fatal("expected nil key before any fetch")
	}
}

// ---------------------------------------------------------------------------
// TestFetch_InvalidBase64X
// ---------------------------------------------------------------------------

func TestFetch_InvalidBase64X(t *testing.T) {
	resp := `{"keys":[{"kty":"EC","crv":"P-256","alg":"ES256","x":"!!!invalid!!!","y":"AAAA"}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	_, err := fetcher.fetch()
	if err == nil {
		t.Fatal("expected error for invalid base64 x coordinate")
	}
	if !strings.Contains(err.Error(), "decode JWK x coordinate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestFetch_InvalidBase64Y
// ---------------------------------------------------------------------------

func TestFetch_InvalidBase64Y(t *testing.T) {
	priv := generateTestEC256Key(t)
	validX := base64.RawURLEncoding.EncodeToString(priv.PublicKey.X.Bytes())

	resp := fmt.Sprintf(`{"keys":[{"kty":"EC","crv":"P-256","alg":"ES256","x":%q,"y":"!!!invalid!!!"}]}`, validX)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer server.Close()

	fetcher := NewJWKSFetcher(server.URL)
	_, err := fetcher.fetch()
	if err == nil {
		t.Fatal("expected error for invalid base64 y coordinate")
	}
	if !strings.Contains(err.Error(), "decode JWK y coordinate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers (re-used from jwt_test.go — package-level, no conflict since
// both files are in the same test package).
// ---------------------------------------------------------------------------

// padCoord left-pads a big.Int to exactly byteLen bytes (used for JWK coordinates).
func padCoord(b *big.Int, byteLen int) []byte {
	raw := b.Bytes()
	if len(raw) >= byteLen {
		return raw[len(raw)-byteLen:]
	}
	padded := make([]byte, byteLen)
	copy(padded[byteLen-len(raw):], raw)
	return padded
}
