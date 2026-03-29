package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDeriveSigningKey(t *testing.T) {
	t.Run("same inputs produce same key", func(t *testing.T) {
		key1, err := DeriveSigningKey("test-secret", "managed-machine")
		if err != nil {
			t.Fatal(err)
		}
		key2, err := DeriveSigningKey("test-secret", "managed-machine")
		if err != nil {
			t.Fatal(err)
		}
		if !hmac.Equal(key1, key2) {
			t.Error("same inputs should produce identical keys")
		}
	})

	t.Run("different secrets produce different keys", func(t *testing.T) {
		key1, _ := DeriveSigningKey("secret-a", "managed-machine")
		key2, _ := DeriveSigningKey("secret-b", "managed-machine")
		if hmac.Equal(key1, key2) {
			t.Error("different secrets should produce different keys")
		}
	})

	t.Run("different purposes produce different keys", func(t *testing.T) {
		key1, _ := DeriveSigningKey("test-secret", "managed-machine")
		key2, _ := DeriveSigningKey("test-secret", "verification")
		if hmac.Equal(key1, key2) {
			t.Error("different purposes should produce different keys")
		}
	})

	t.Run("empty secret returns error", func(t *testing.T) {
		_, err := DeriveSigningKey("", "managed-machine")
		if err == nil {
			t.Error("expected error for empty secret")
		}
	})

	t.Run("key is 32 bytes", func(t *testing.T) {
		key, _ := DeriveSigningKey("test-secret", "managed-machine")
		if len(key) != 32 {
			t.Errorf("key length = %d, want 32", len(key))
		}
	})
}

// Helper to create a test JWT
func createTestJWT(t *testing.T, key []byte, claims map[string]interface{}) string {
	t.Helper()

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	payload := base64URLEncode(claimsJSON)

	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signingInput))
	signature := base64URLEncode(mac.Sum(nil))

	return signingInput + "." + signature
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func TestJWTValidator_Validate(t *testing.T) {
	key, _ := DeriveSigningKey("test-secret", "managed-machine")
	validator, err := NewJWTValidator("test-secret", "mm-123")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid token accepted", func(t *testing.T) {
		token := createTestJWT(t, key, map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
			"iat": time.Now().Unix(),
		})

		claims, err := validator.Validate(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims.Sub != "actor-456" {
			t.Errorf("Sub = %q, want %q", claims.Sub, "actor-456")
		}
		if claims.Mid != "mm-123" {
			t.Errorf("Mid = %q, want %q", claims.Mid, "mm-123")
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		token := createTestJWT(t, key, map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(-1 * time.Minute).Unix(),
			"iat": time.Now().Add(-2 * time.Minute).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for expired token")
		}
		if !strings.Contains(err.Error(), "expired") {
			t.Errorf("error = %q, want to contain 'expired'", err.Error())
		}
	})

	t.Run("wrong machine ID rejected", func(t *testing.T) {
		token := createTestJWT(t, key, map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-wrong",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for wrong machine ID")
		}
		if !strings.Contains(err.Error(), "machine ID mismatch") {
			t.Errorf("error = %q, want to contain 'machine ID mismatch'", err.Error())
		}
	})

	t.Run("tampered signature rejected", func(t *testing.T) {
		token := createTestJWT(t, key, map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		// Flip a bit in the signature
		parts := strings.Split(token, ".")
		sig := []byte(parts[2])
		if len(sig) > 0 {
			sig[0] ^= 0xFF
		}
		parts[2] = string(sig)
		tampered := strings.Join(parts, ".")

		_, err := validator.Validate(tampered)
		if err == nil {
			t.Fatal("expected error for tampered signature")
		}
	})

	t.Run("wrong key rejected", func(t *testing.T) {
		wrongKey, _ := DeriveSigningKey("wrong-secret", "managed-machine")
		token := createTestJWT(t, wrongKey, map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for wrong signing key")
		}
	})

	t.Run("raw secret without HKDF rejected", func(t *testing.T) {
		rawKey := []byte("test-secret") // Not HKDF-derived
		token := createTestJWT(t, rawKey, map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for token signed with raw secret (not HKDF)")
		}
	})

	t.Run("wrong HKDF purpose rejected", func(t *testing.T) {
		wrongPurposeKey, _ := DeriveSigningKey("test-secret", "verification")
		token := createTestJWT(t, wrongPurposeKey, map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for wrong HKDF purpose")
		}
	})

	t.Run("clock skew within tolerance accepted", func(t *testing.T) {
		token := createTestJWT(t, key, map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(-5 * time.Second).Unix(), // 5s past expiry
		})

		_, err := validator.Validate(token)
		if err != nil {
			t.Fatalf("expected token within clock skew to be accepted: %v", err)
		}
	})

	t.Run("clock skew beyond tolerance rejected", func(t *testing.T) {
		token := createTestJWT(t, key, map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(-15 * time.Second).Unix(), // 15s past expiry
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for token beyond clock skew tolerance")
		}
	})

	t.Run("missing sub claim rejected", func(t *testing.T) {
		token := createTestJWT(t, key, map[string]interface{}{
			"mid": "mm-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for missing sub claim")
		}
	})

	t.Run("missing mid claim rejected", func(t *testing.T) {
		token := createTestJWT(t, key, map[string]interface{}{
			"sub": "actor-456",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for missing mid claim")
		}
	})

	t.Run("empty string rejected", func(t *testing.T) {
		_, err := validator.Validate("")
		if err == nil {
			t.Fatal("expected error for empty string")
		}
	})

	t.Run("malformed token rejected", func(t *testing.T) {
		_, err := validator.Validate("not-a-jwt")
		if err == nil {
			t.Fatal("expected error for malformed token")
		}
	})

	t.Run("alg:none rejected", func(t *testing.T) {
		header := base64URLEncode([]byte(`{"alg":"none","typ":"JWT"}`))
		claimsJSON, _ := json.Marshal(map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		payload := base64URLEncode(claimsJSON)
		token := header + "." + payload + "."

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for alg:none")
		}
	})

	t.Run("alg:RS256 rejected", func(t *testing.T) {
		header := base64URLEncode([]byte(`{"alg":"RS256","typ":"JWT"}`))
		claimsJSON, _ := json.Marshal(map[string]interface{}{
			"sub": "actor-456",
			"mid": "mm-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		payload := base64URLEncode(claimsJSON)
		token := header + "." + payload + ".fake-signature"

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for alg:RS256")
		}
		if !strings.Contains(err.Error(), "unsupported algorithm") {
			t.Errorf("error = %q, want to contain 'unsupported algorithm'", err.Error())
		}
	})

	t.Run("oversized token rejected", func(t *testing.T) {
		huge := strings.Repeat("a", MaxTokenSize+1)
		_, err := validator.Validate(huge)
		if err == nil {
			t.Fatal("expected error for oversized token")
		}
		if !strings.Contains(err.Error(), "too large") {
			t.Errorf("error = %q, want to contain 'too large'", err.Error())
		}
	})

	t.Run("extra claims accepted (forward compatible)", func(t *testing.T) {
		token := createTestJWT(t, key, map[string]interface{}{
			"sub":     "actor-456",
			"mid":     "mm-123",
			"exp":     time.Now().Add(1 * time.Hour).Unix(),
			"custom":  "value",
			"another": 42,
		})

		claims, err := validator.Validate(token)
		if err != nil {
			t.Fatalf("extra claims should be accepted: %v", err)
		}
		if claims.Sub != "actor-456" {
			t.Error("known claims should still be parsed")
		}
	})
}

func TestNewJWTValidator_EmptySecret(t *testing.T) {
	_, err := NewJWTValidator("", "mm-123")
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestNewJWTValidator_EmptyMachineID(t *testing.T) {
	_, err := NewJWTValidator("test-secret", "")
	if err == nil {
		t.Fatal("expected error for empty machine ID")
	}
}

// Benchmark JWT validation
func BenchmarkValidate(b *testing.B) {
	key, _ := DeriveSigningKey("bench-secret", "managed-machine")
	validator, _ := NewJWTValidator("bench-secret", "mm-bench")

	claims := map[string]interface{}{
		"sub": "actor-bench",
		"mid": "mm-bench",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claimsJSON, _ := json.Marshal(claims)
	payload := base64URLEncode(claimsJSON)
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signingInput))
	sig := base64URLEncode(mac.Sum(nil))
	token := fmt.Sprintf("%s.%s", signingInput, sig)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.Validate(token)
	}
}
