package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"os"
	"testing"
)

type CryptoVectors struct {
	HKDFVectors []HKDFVector `json:"hkdf_vectors"`
	JWTVectors  []JWTVector  `json:"jwt_vectors"`
	HMACVectors []HMACVector `json:"hmac_vectors"`
}

type HKDFVector struct {
	Secret            string `json:"secret"`
	Purpose           string `json:"purpose"`
	ExpectedKeyLength int    `json:"expected_key_length"`
}

type JWTVector struct {
	Description    string                 `json:"description"`
	Secret         string                 `json:"secret"`
	Purpose        string                 `json:"purpose"`
	MachineID      string                 `json:"machine_id"`
	Claims         map[string]interface{} `json:"claims"`
	ShouldValidate bool                   `json:"should_validate"`
	ExpectedError  string                 `json:"expected_error"`
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
		t.Fatalf("failed to load crypto vectors: %v", err)
	}
	var vectors CryptoVectors
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("failed to parse crypto vectors: %v", err)
	}
	return vectors
}

func TestContract_HKDFKeyDerivation(t *testing.T) {
	vectors := loadVectors(t)

	for _, v := range vectors.HKDFVectors {
		t.Run(v.Purpose, func(t *testing.T) {
			key, err := DeriveSigningKey(v.Secret, v.Purpose)
			if err != nil {
				t.Fatalf("DeriveSigningKey failed: %v", err)
			}
			if len(key) != v.ExpectedKeyLength {
				t.Errorf("key length = %d, want %d", len(key), v.ExpectedKeyLength)
			}
		})
	}

	// Same secret + purpose must produce same key deterministically
	t.Run("deterministic", func(t *testing.T) {
		key1, _ := DeriveSigningKey("test-secret-for-vectors", "managed-machine")
		key2, _ := DeriveSigningKey("test-secret-for-vectors", "managed-machine")
		if !hmac.Equal(key1, key2) {
			t.Error("same inputs must produce identical keys")
		}
	})

	// Different purposes must produce different keys
	t.Run("purpose-isolation", func(t *testing.T) {
		key1, _ := DeriveSigningKey("test-secret-for-vectors", "managed-machine")
		key2, _ := DeriveSigningKey("test-secret-for-vectors", "verification")
		if hmac.Equal(key1, key2) {
			t.Error("different purposes must produce different keys")
		}
	})
}

func TestContract_JWTValidation(t *testing.T) {
	vectors := loadVectors(t)

	for _, v := range vectors.JWTVectors {
		t.Run(v.Description, func(t *testing.T) {
			// Derive key
			key, err := DeriveSigningKey(v.Secret, v.Purpose)
			if err != nil {
				t.Fatalf("DeriveSigningKey failed: %v", err)
			}

			// Create JWT from claims
			token := createTestJWT(t, key, v.Claims)

			// Create validator for the machine ID in the vector
			validator, err := NewJWTValidator(v.Secret, v.MachineID)
			if err != nil {
				t.Fatalf("NewJWTValidator failed: %v", err)
			}

			// Validate
			_, validateErr := validator.Validate(token)

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
