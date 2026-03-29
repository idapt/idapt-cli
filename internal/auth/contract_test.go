package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"os"
	"testing"
)

// CryptoVectors holds cross-language test vectors for Go/TypeScript compatibility.
// ES256 vectors contain pre-signed JWTs + public keys for validation.
// HMAC vectors remain for heartbeat auth (future).
type CryptoVectors struct {
	ES256JWTVectors []ES256JWTVector `json:"es256_jwt_vectors"`
	HMACVectors     []HMACVector     `json:"hmac_vectors"`
}

// ES256JWTVector is a pre-signed ES256 JWT with the corresponding public key PEM.
type ES256JWTVector struct {
	Description    string `json:"description"`
	PublicKeyPEM   string `json:"public_key_pem"`
	Token          string `json:"token"`
	MachineID      string `json:"machine_id"`
	ShouldValidate bool   `json:"should_validate"`
	ExpectedError  string `json:"expected_error"`
}

type HMACVector struct {
	Secret         string `json:"secret"`
	Message        string `json:"message"`
	ExpectedLength int    `json:"expected_length"`
}

func loadVectors(t *testing.T) CryptoVectors {
	t.Helper()
	data, err := os.ReadFile("../../testdata/crypto-vectors.json")
	if err != nil {
		t.Skipf("crypto vectors file not found (run scripts/generate-jwt-vectors.ts first): %v", err)
	}
	var vectors CryptoVectors
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("failed to parse crypto vectors: %v", err)
	}
	return vectors
}

func TestContract_ES256JWTValidation(t *testing.T) {
	vectors := loadVectors(t)

	if len(vectors.ES256JWTVectors) == 0 {
		t.Skip("no ES256 JWT vectors (run scripts/generate-jwt-vectors.ts first)")
	}

	for _, v := range vectors.ES256JWTVectors {
		t.Run(v.Description, func(t *testing.T) {
			// Create validator from the vector's public key PEM
			validator, err := NewJWTValidator(v.PublicKeyPEM, v.MachineID)
			if err != nil {
				if !v.ShouldValidate {
					// Expected to fail at validator creation
					return
				}
				t.Fatalf("NewJWTValidator failed: %v", err)
			}

			// Validate the pre-signed token
			_, validateErr := validator.Validate(v.Token)

			if v.ShouldValidate {
				if validateErr != nil {
					t.Errorf("expected valid, got error: %v", validateErr)
				}
			} else {
				if validateErr == nil {
					t.Error("expected error, got nil")
				}
			}
		})
	}
}

func TestContract_HMAC(t *testing.T) {
	vectors := loadVectors(t)

	if len(vectors.HMACVectors) == 0 {
		t.Skip("no HMAC vectors")
	}

	for _, v := range vectors.HMACVectors {
		t.Run(v.Message, func(t *testing.T) {
			mac := hmac.New(sha256.New, []byte(v.Secret))
			mac.Write([]byte(v.Message))
			sig := mac.Sum(nil)

			if len(sig) != v.ExpectedLength {
				t.Errorf("HMAC length = %d, want %d", len(sig), v.ExpectedLength)
			}

			// Deterministic
			mac2 := hmac.New(sha256.New, []byte(v.Secret))
			mac2.Write([]byte(v.Message))
			sig2 := mac2.Sum(nil)

			if !hmac.Equal(sig, sig2) {
				t.Error("HMAC must be deterministic")
			}
		})
	}
}
