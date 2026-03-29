package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// generateTestKeyPair generates an ECDSA P-256 key pair and returns the
// private key plus the PEM-encoded public key string.
func generateTestKeyPair(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})
	return priv, string(pubPEM)
}

// generateP384KeyPair generates an ECDSA P-384 key pair and returns the
// private key plus the PEM-encoded public key string (wrong curve for ES256).
func generateP384KeyPair(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-384 key: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal P-384 public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})
	return priv, string(pubPEM)
}

// generateP521KeyPair generates an ECDSA P-521 key pair and returns the
// private key plus the PEM-encoded public key string (wrong curve for ES256).
func generateP521KeyPair(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-521 key: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal P-521 public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})
	return priv, string(pubPEM)
}

// generateRSAKeyPair generates an RSA 2048 key pair and returns the
// PEM-encoded public key string.
func generateRSAKeyPair(t *testing.T) string {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal RSA public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})
	return string(pubPEM)
}

// createTestES256JWT creates an ES256-signed JWT with the given claims.
// The signature uses raw R||S concatenation (32 bytes each, left-padded with
// zeros), as required by the JWS spec for ES256.
func createTestES256JWT(t *testing.T, privKey *ecdsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()

	header := base64URLEncode([]byte(`{"alg":"ES256","typ":"JWT"}`))

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	payload := base64URLEncode(claimsJSON)

	signingInput := header + "." + payload

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		t.Fatalf("ECDSA sign: %v", err)
	}

	// Raw R||S format: each component is exactly 32 bytes, zero-padded on the left.
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	rawSig := make([]byte, 64)
	copy(rawSig[32-len(rBytes):32], rBytes)
	copy(rawSig[64-len(sBytes):64], sBytes)

	signature := base64URLEncode(rawSig)

	return signingInput + "." + signature
}

// base64URLEncode encodes data as base64url without padding.
func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// createJWTWithHeader constructs a JWT with an arbitrary header, payload, and
// raw signature bytes. Used for algorithm confusion and manipulation tests.
func createJWTWithHeader(t *testing.T, headerJSON []byte, claims map[string]interface{}, sig []byte) string {
	t.Helper()
	header := base64URLEncode(headerJSON)
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	payload := base64URLEncode(claimsJSON)
	signature := base64URLEncode(sig)
	return header + "." + payload + "." + signature
}

// validClaims returns a standard set of valid claims for testing.
func validClaims(machineID string) map[string]interface{} {
	return map[string]interface{}{
		"sub": "actor-456",
		"mid": machineID,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
}

// ---------------------------------------------------------------------------
// NewJWTValidator tests
// ---------------------------------------------------------------------------

func TestNewJWTValidator(t *testing.T) {
	t.Run("ValidP256PEM", func(t *testing.T) {
		_, pubPEM := generateTestKeyPair(t)
		v, err := NewJWTValidator(pubPEM, "mm-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v == nil {
			t.Fatal("expected non-nil validator")
		}
	})

	t.Run("EmptyPEM", func(t *testing.T) {
		_, err := NewJWTValidator("", "mm-123")
		if err == nil {
			t.Fatal("expected error for empty PEM")
		}
	})

	t.Run("EmptyMachineID", func(t *testing.T) {
		_, pubPEM := generateTestKeyPair(t)
		_, err := NewJWTValidator(pubPEM, "")
		if err == nil {
			t.Fatal("expected error for empty machine ID")
		}
	})

	t.Run("InvalidPEM", func(t *testing.T) {
		_, err := NewJWTValidator("not-valid-pem-data", "mm-123")
		if err == nil {
			t.Fatal("expected error for invalid PEM")
		}
	})

	t.Run("RSAKeyPEM", func(t *testing.T) {
		rsaPEM := generateRSAKeyPair(t)
		_, err := NewJWTValidator(rsaPEM, "mm-123")
		if err == nil {
			t.Fatal("expected error for RSA key PEM")
		}
	})

	t.Run("P384KeyPEM", func(t *testing.T) {
		_, p384PEM := generateP384KeyPair(t)
		_, err := NewJWTValidator(p384PEM, "mm-123")
		if err == nil {
			t.Fatal("expected error for P-384 key PEM")
		}
	})

	t.Run("P521KeyPEM", func(t *testing.T) {
		_, p521PEM := generateP521KeyPair(t)
		_, err := NewJWTValidator(p521PEM, "mm-123")
		if err == nil {
			t.Fatal("expected error for P-521 key PEM")
		}
	})

	t.Run("PrivateKeyPEM", func(t *testing.T) {
		priv, _ := generateTestKeyPair(t)
		privDER, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			t.Fatalf("marshal private key: %v", err)
		}
		privPEM := string(pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: privDER,
		}))
		_, err = NewJWTValidator(privPEM, "mm-123")
		if err == nil {
			t.Fatal("expected error for private key PEM")
		}
	})

	t.Run("CorruptedPEM", func(t *testing.T) {
		_, pubPEM := generateTestKeyPair(t)
		// Corrupt the base64 content within the PEM block
		corrupted := strings.Replace(pubPEM, "M", "!", 5)
		_, err := NewJWTValidator(corrupted, "mm-123")
		if err == nil {
			t.Fatal("expected error for corrupted PEM")
		}
	})

	t.Run("CertificatePEM", func(t *testing.T) {
		// Generate a self-signed certificate PEM
		priv, _ := generateTestKeyPair(t)
		template := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject:      pkix.Name{CommonName: "test"},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(1 * time.Hour),
		}
		certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
		if err != nil {
			t.Fatalf("create certificate: %v", err)
		}
		certPEM := string(pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certDER,
		}))
		_, err = NewJWTValidator(certPEM, "mm-123")
		if err == nil {
			t.Fatal("expected error for certificate PEM (not a raw public key)")
		}
	})
}

// ---------------------------------------------------------------------------
// Happy path tests
// ---------------------------------------------------------------------------

func TestValidate_HappyPath(t *testing.T) {
	priv, pubPEM := generateTestKeyPair(t)
	machineID := "mm-123"
	validator, err := NewJWTValidator(pubPEM, machineID)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	t.Run("ValidToken", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
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
		if claims.Mid != machineID {
			t.Errorf("Mid = %q, want %q", claims.Mid, machineID)
		}
	})

	t.Run("ValidToken_MinimalClaims", func(t *testing.T) {
		// Only required claims: sub, mid, exp
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-789",
			"mid": machineID,
			"exp": time.Now().Add(30 * time.Minute).Unix(),
		})

		claims, err := validator.Validate(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims.Sub != "actor-789" {
			t.Errorf("Sub = %q, want %q", claims.Sub, "actor-789")
		}
	})

	t.Run("ExtraClaimsAccepted", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub":     "actor-456",
			"mid":     machineID,
			"exp":     time.Now().Add(1 * time.Hour).Unix(),
			"iat":     time.Now().Unix(),
			"custom":  "value",
			"another": 42,
			"nested":  map[string]interface{}{"key": "val"},
		})

		claims, err := validator.Validate(token)
		if err != nil {
			t.Fatalf("extra claims should be accepted: %v", err)
		}
		if claims.Sub != "actor-456" {
			t.Error("known claims should still be parsed correctly")
		}
	})

	t.Run("ClockSkew_WithinTolerance", func(t *testing.T) {
		// Token expired 5 seconds ago — within 10s tolerance
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
			"exp": time.Now().Add(-5 * time.Second).Unix(),
			"iat": time.Now().Add(-1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err != nil {
			t.Fatalf("token within clock skew tolerance should be accepted: %v", err)
		}
	})

	t.Run("TokenIssuedNow", func(t *testing.T) {
		now := time.Now()
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
			"exp": now.Add(5 * time.Minute).Unix(),
			"iat": now.Unix(),
		})

		_, err := validator.Validate(token)
		if err != nil {
			t.Fatalf("token issued now should be accepted: %v", err)
		}
	})

	t.Run("TokenIssuedInPast", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
			"exp": time.Now().Add(30 * time.Minute).Unix(),
			"iat": time.Now().Add(-2 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err != nil {
			t.Fatalf("token issued in the past with valid exp should be accepted: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Expiration tests
// ---------------------------------------------------------------------------

func TestValidate_Expiration(t *testing.T) {
	priv, pubPEM := generateTestKeyPair(t)
	machineID := "mm-123"
	validator, err := NewJWTValidator(pubPEM, machineID)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	t.Run("Expired", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
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

	t.Run("ClockSkew_BeyondTolerance", func(t *testing.T) {
		// Expired 15 seconds ago — beyond 10s tolerance
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
			"exp": time.Now().Add(-15 * time.Second).Unix(),
			"iat": time.Now().Add(-1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for token beyond clock skew tolerance")
		}
		if !strings.Contains(err.Error(), "expired") {
			t.Errorf("error = %q, want to contain 'expired'", err.Error())
		}
	})

	t.Run("ExpiredByDays", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
			"exp": time.Now().Add(-72 * time.Hour).Unix(),
			"iat": time.Now().Add(-96 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for token expired by days")
		}
	})

	t.Run("ExpZero", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
			"exp": 0,
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for exp=0")
		}
	})

	t.Run("ExpNegative", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
			"exp": -1000,
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for negative exp")
		}
	})

	t.Run("FarFutureExp", func(t *testing.T) {
		// Token valid until year 2099 — should still be accepted (no max-lifetime check)
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": machineID,
			"exp": time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
			"iat": time.Now().Unix(),
		})

		_, err := validator.Validate(token)
		if err != nil {
			t.Fatalf("far-future exp should be accepted: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Machine ID tests
// ---------------------------------------------------------------------------

func TestValidate_MachineID(t *testing.T) {
	priv, pubPEM := generateTestKeyPair(t)
	machineID := "mm-123"
	validator, err := NewJWTValidator(pubPEM, machineID)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	t.Run("WrongMachineID", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
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

	t.Run("EmptyMachineID", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": "",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for empty machine ID in token")
		}
	})

	t.Run("MissingMidClaim", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for missing mid claim")
		}
	})

	t.Run("CaseSensitiveMachineID", func(t *testing.T) {
		// Machine ID comparison should be case-sensitive
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "actor-456",
			"mid": "MM-123", // uppercase
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for case-mismatched machine ID")
		}
	})
}

// ---------------------------------------------------------------------------
// Algorithm confusion tests (CRITICAL SECURITY)
// ---------------------------------------------------------------------------

func TestValidate_AlgorithmConfusion(t *testing.T) {
	priv, pubPEM := generateTestKeyPair(t)
	machineID := "mm-123"
	validator, err := NewJWTValidator(pubPEM, machineID)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	claims := validClaims(machineID)

	t.Run("AlgNone", func(t *testing.T) {
		headerJSON := []byte(`{"alg":"none","typ":"JWT"}`)
		token := createJWTWithHeader(t, headerJSON, claims, []byte{})
		// alg:none tokens may have empty or missing signature segment
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for alg:none")
		}
	})

	t.Run("AlgHS256", func(t *testing.T) {
		// Classic algorithm confusion attack: sign with HMAC-SHA256 using the
		// public key PEM bytes as the HMAC secret. If the validator naively
		// trusts the "alg" header, it would use the public key as an HMAC key
		// and verify successfully.
		headerJSON := []byte(`{"alg":"HS256","typ":"JWT"}`)
		header := base64URLEncode(headerJSON)
		claimsJSON, _ := json.Marshal(claims)
		payload := base64URLEncode(claimsJSON)
		signingInput := header + "." + payload

		mac := hmac.New(sha256.New, []byte(pubPEM))
		mac.Write([]byte(signingInput))
		sig := mac.Sum(nil)

		token := signingInput + "." + base64URLEncode(sig)

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for HS256 algorithm confusion attack")
		}
	})

	t.Run("AlgHS384", func(t *testing.T) {
		headerJSON := []byte(`{"alg":"HS384","typ":"JWT"}`)
		token := createJWTWithHeader(t, headerJSON, claims, []byte("fake-sig"))
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for HS384")
		}
	})

	t.Run("AlgHS512", func(t *testing.T) {
		headerJSON := []byte(`{"alg":"HS512","typ":"JWT"}`)
		token := createJWTWithHeader(t, headerJSON, claims, []byte("fake-sig"))
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for HS512")
		}
	})

	t.Run("AlgRS256", func(t *testing.T) {
		headerJSON := []byte(`{"alg":"RS256","typ":"JWT"}`)
		token := createJWTWithHeader(t, headerJSON, claims, []byte("fake-sig"))
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for RS256")
		}
		if !strings.Contains(err.Error(), "unsupported algorithm") && !strings.Contains(err.Error(), "algorithm") {
			t.Errorf("error = %q, want to mention algorithm", err.Error())
		}
	})

	t.Run("AlgPS256", func(t *testing.T) {
		headerJSON := []byte(`{"alg":"PS256","typ":"JWT"}`)
		token := createJWTWithHeader(t, headerJSON, claims, []byte("fake-sig"))
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for PS256")
		}
	})

	t.Run("AlgEdDSA", func(t *testing.T) {
		headerJSON := []byte(`{"alg":"EdDSA","typ":"JWT"}`)
		token := createJWTWithHeader(t, headerJSON, claims, []byte("fake-sig"))
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for EdDSA")
		}
	})

	t.Run("AlgEmpty", func(t *testing.T) {
		headerJSON := []byte(`{"alg":"","typ":"JWT"}`)
		token := createJWTWithHeader(t, headerJSON, claims, []byte("fake-sig"))
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for empty alg")
		}
	})

	t.Run("AlgMissing", func(t *testing.T) {
		headerJSON := []byte(`{"typ":"JWT"}`)
		token := createJWTWithHeader(t, headerJSON, claims, []byte("fake-sig"))
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for missing alg field")
		}
	})

	t.Run("AlgCaseSensitive", func(t *testing.T) {
		// "es256" lowercase — should be rejected (correct is "ES256")
		headerJSON := []byte(`{"alg":"es256","typ":"JWT"}`)
		// Sign with the real key to ensure the only issue is the alg field
		header := base64URLEncode(headerJSON)
		claimsJSON, _ := json.Marshal(claims)
		payload := base64URLEncode(claimsJSON)
		signingInput := header + "." + payload

		hash := sha256.Sum256([]byte(signingInput))
		r, s, sigErr := ecdsa.Sign(rand.Reader, priv, hash[:])
		if sigErr != nil {
			t.Fatalf("sign: %v", sigErr)
		}
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		rawSig := make([]byte, 64)
		copy(rawSig[32-len(rBytes):32], rBytes)
		copy(rawSig[64-len(sBytes):64], sBytes)

		token := signingInput + "." + base64URLEncode(rawSig)

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for lowercase 'es256' — algorithm check must be case-sensitive")
		}
	})
}

// ---------------------------------------------------------------------------
// Signature manipulation tests
// ---------------------------------------------------------------------------

func TestValidate_SignatureManipulation(t *testing.T) {
	priv, pubPEM := generateTestKeyPair(t)
	machineID := "mm-123"
	validator, err := NewJWTValidator(pubPEM, machineID)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	t.Run("TamperedPayload", func(t *testing.T) {
		token := createTestES256JWT(t, priv, validClaims(machineID))

		// Change a byte in the payload segment
		parts := strings.Split(token, ".")
		payloadBytes := []byte(parts[1])
		if len(payloadBytes) > 5 {
			payloadBytes[5] ^= 0xFF
		}
		parts[1] = string(payloadBytes)
		tampered := strings.Join(parts, ".")

		_, err := validator.Validate(tampered)
		if err == nil {
			t.Fatal("expected error for tampered payload")
		}
	})

	t.Run("TamperedSignature", func(t *testing.T) {
		token := createTestES256JWT(t, priv, validClaims(machineID))

		parts := strings.Split(token, ".")
		sigBytes := []byte(parts[2])
		if len(sigBytes) > 0 {
			sigBytes[0] ^= 0xFF
		}
		parts[2] = string(sigBytes)
		tampered := strings.Join(parts, ".")

		_, err := validator.Validate(tampered)
		if err == nil {
			t.Fatal("expected error for tampered signature")
		}
	})

	t.Run("EmptySignature", func(t *testing.T) {
		token := createTestES256JWT(t, priv, validClaims(machineID))
		parts := strings.Split(token, ".")
		parts[2] = ""
		noSig := strings.Join(parts, ".")

		_, err := validator.Validate(noSig)
		if err == nil {
			t.Fatal("expected error for empty signature")
		}
	})

	t.Run("TruncatedSignature", func(t *testing.T) {
		token := createTestES256JWT(t, priv, validClaims(machineID))
		parts := strings.Split(token, ".")
		// Keep only first half of signature
		if len(parts[2]) > 4 {
			parts[2] = parts[2][:len(parts[2])/2]
		}
		truncated := strings.Join(parts, ".")

		_, err := validator.Validate(truncated)
		if err == nil {
			t.Fatal("expected error for truncated signature")
		}
	})

	t.Run("OversizedSignature", func(t *testing.T) {
		token := createTestES256JWT(t, priv, validClaims(machineID))
		parts := strings.Split(token, ".")
		// Append extra bytes to the signature
		oversized := make([]byte, 128) // ES256 sig should be exactly 64 bytes
		for i := range oversized {
			oversized[i] = 0xAB
		}
		parts[2] = base64URLEncode(oversized)
		bad := strings.Join(parts, ".")

		_, err := validator.Validate(bad)
		if err == nil {
			t.Fatal("expected error for oversized signature")
		}
	})

	t.Run("DEREncodedSignature", func(t *testing.T) {
		// ES256 JWTs use raw R||S, NOT DER encoding. If someone provides a
		// DER-encoded ECDSA signature instead of raw, it must be rejected.
		claims := validClaims(machineID)
		headerJSON := []byte(`{"alg":"ES256","typ":"JWT"}`)
		header := base64URLEncode(headerJSON)
		claimsJSON, _ := json.Marshal(claims)
		payload := base64URLEncode(claimsJSON)
		signingInput := header + "." + payload

		hash := sha256.Sum256([]byte(signingInput))
		r, s, sigErr := ecdsa.Sign(rand.Reader, priv, hash[:])
		if sigErr != nil {
			t.Fatalf("sign: %v", sigErr)
		}

		// Encode as ASN.1/DER instead of raw R||S
		derSig, sigErr := encodeECDSASignatureDER(r, s)
		if sigErr != nil {
			t.Fatalf("DER encode: %v", sigErr)
		}

		token := signingInput + "." + base64URLEncode(derSig)

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for DER-encoded signature (should require raw R||S)")
		}
	})

	t.Run("AllZerosSignature", func(t *testing.T) {
		token := createTestES256JWT(t, priv, validClaims(machineID))
		parts := strings.Split(token, ".")
		zeros := make([]byte, 64)
		parts[2] = base64URLEncode(zeros)
		bad := strings.Join(parts, ".")

		_, err := validator.Validate(bad)
		if err == nil {
			t.Fatal("expected error for all-zeros signature")
		}
	})

	t.Run("WrongKeyPair", func(t *testing.T) {
		// Sign with a different key pair than the validator was created with
		otherPriv, _ := generateTestKeyPair(t)
		token := createTestES256JWT(t, otherPriv, validClaims(machineID))

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for token signed with wrong key pair")
		}
	})
}

// encodeECDSASignatureDER encodes r, s as an ASN.1 DER ECDSA-Sig-Value.
func encodeECDSASignatureDER(r, s *big.Int) ([]byte, error) {
	// SEQUENCE { INTEGER r, INTEGER s }
	rBytes := asn1IntegerBytes(r)
	sBytes := asn1IntegerBytes(s)

	rField := append([]byte{0x02, byte(len(rBytes))}, rBytes...)
	sField := append([]byte{0x02, byte(len(sBytes))}, sBytes...)

	inner := append(rField, sField...)
	return append([]byte{0x30, byte(len(inner))}, inner...), nil
}

// asn1IntegerBytes returns the minimal big-endian encoding of an integer for
// ASN.1, prepending a 0x00 byte if the high bit is set (to avoid sign confusion).
func asn1IntegerBytes(n *big.Int) []byte {
	b := n.Bytes()
	if len(b) == 0 {
		return []byte{0}
	}
	if b[0]&0x80 != 0 {
		b = append([]byte{0x00}, b...)
	}
	return b
}

// ---------------------------------------------------------------------------
// Input validation tests
// ---------------------------------------------------------------------------

func TestValidate_InputValidation(t *testing.T) {
	_, pubPEM := generateTestKeyPair(t)
	machineID := "mm-123"
	validator, err := NewJWTValidator(pubPEM, machineID)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	t.Run("EmptyString", func(t *testing.T) {
		_, err := validator.Validate("")
		if err == nil {
			t.Fatal("expected error for empty string")
		}
	})

	t.Run("Whitespace", func(t *testing.T) {
		_, err := validator.Validate("   \t\n  ")
		if err == nil {
			t.Fatal("expected error for whitespace-only input")
		}
	})

	t.Run("OnePart", func(t *testing.T) {
		_, err := validator.Validate("singlepart")
		if err == nil {
			t.Fatal("expected error for single-part token")
		}
	})

	t.Run("TwoParts", func(t *testing.T) {
		_, err := validator.Validate("part1.part2")
		if err == nil {
			t.Fatal("expected error for two-part token")
		}
	})

	t.Run("FourParts", func(t *testing.T) {
		_, err := validator.Validate("part1.part2.part3.part4")
		if err == nil {
			t.Fatal("expected error for four-part token")
		}
	})

	t.Run("OversizedToken", func(t *testing.T) {
		huge := strings.Repeat("a", MaxTokenSize+1)
		_, err := validator.Validate(huge)
		if err == nil {
			t.Fatal("expected error for oversized token")
		}
		if !strings.Contains(err.Error(), "too large") {
			t.Errorf("error = %q, want to contain 'too large'", err.Error())
		}
	})

	t.Run("InvalidBase64Header", func(t *testing.T) {
		token := "!!!invalid-base64!!!.payload.signature"
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for invalid base64 header")
		}
	})

	t.Run("InvalidBase64Payload", func(t *testing.T) {
		header := base64URLEncode([]byte(`{"alg":"ES256","typ":"JWT"}`))
		token := header + ".!!!invalid!!!" + ".sig"
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for invalid base64 payload")
		}
	})

	t.Run("InvalidJSONHeader", func(t *testing.T) {
		header := base64URLEncode([]byte(`not json at all`))
		payload := base64URLEncode([]byte(`{"sub":"a","mid":"mm-123","exp":9999999999}`))
		token := header + "." + payload + ".sig"
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for invalid JSON header")
		}
	})

	t.Run("InvalidJSONPayload", func(t *testing.T) {
		header := base64URLEncode([]byte(`{"alg":"ES256","typ":"JWT"}`))
		payload := base64URLEncode([]byte(`{broken json`))
		token := header + "." + payload + ".sig"
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for invalid JSON payload")
		}
	})

	t.Run("NullBytesInToken", func(t *testing.T) {
		token := "eyJ\x00hbGciOiJFUzI1NiJ9.eyJ\x00zdWIiOiJ0ZXN0In0.sig"
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for null bytes in token")
		}
	})

	t.Run("UnicodeInToken", func(t *testing.T) {
		token := "eyJhbGci\u00e9OiJFUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.sig"
		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for unicode in token")
		}
	})
}

// ---------------------------------------------------------------------------
// Claims edge case tests
// ---------------------------------------------------------------------------

func TestValidate_ClaimsEdgeCases(t *testing.T) {
	priv, pubPEM := generateTestKeyPair(t)
	machineID := "mm-123"
	validator, err := NewJWTValidator(pubPEM, machineID)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	t.Run("SubEmpty", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"sub": "",
			"mid": machineID,
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for empty sub claim")
		}
	})

	t.Run("SubMissing", func(t *testing.T) {
		token := createTestES256JWT(t, priv, map[string]interface{}{
			"mid": machineID,
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})

		_, err := validator.Validate(token)
		if err == nil {
			t.Fatal("expected error for missing sub claim")
		}
	})
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkValidate_ES256(b *testing.B) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	pubDER, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}))

	validator, err := NewJWTValidator(pubPEM, "mm-bench")
	if err != nil {
		b.Fatal(err)
	}

	// Pre-generate the token outside the benchmark loop
	claims := map[string]interface{}{
		"sub": "actor-bench",
		"mid": "mm-bench",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	header := base64URLEncode([]byte(`{"alg":"ES256","typ":"JWT"}`))
	claimsJSON, _ := json.Marshal(claims)
	payload := base64URLEncode(claimsJSON)
	signingInput := header + "." + payload

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		b.Fatal(err)
	}
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	rawSig := make([]byte, 64)
	copy(rawSig[32-len(rBytes):32], rBytes)
	copy(rawSig[64-len(sBytes):64], sBytes)

	token := fmt.Sprintf("%s.%s", signingInput, base64URLEncode(rawSig))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validator.Validate(token)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewValidator(b *testing.B) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	pubDER, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewJWTValidator(pubPEM, "mm-bench")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Ensure the crypto import is used (the _ assignment satisfies the compiler
// if no other reference exists, but ecdsa.Sign uses it implicitly).
var _ = crypto.SHA256
