package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

// APIKeyPrefix is the required prefix for machine API keys.
const APIKeyPrefix = "mk_"

// APIKeyValidator validates machine API keys against stored hashes.
type APIKeyValidator struct {
	// knownKeys maps SHA-256 hex hashes to key metadata.
	// In production, this would query the app's API; for now it's in-memory
	// populated by the firewall config pushes.
	knownKeys map[string]bool
}

// NewAPIKeyValidator creates a new API key validator.
func NewAPIKeyValidator() *APIKeyValidator {
	return &APIKeyValidator{
		knownKeys: make(map[string]bool),
	}
}

// AddKeyHash registers a known key hash for validation.
func (v *APIKeyValidator) AddKeyHash(hashHex string) {
	v.knownKeys[hashHex] = true
}

// Validate checks if the given API key is valid.
// Returns nil if valid, error if invalid.
func (v *APIKeyValidator) Validate(rawKey string) error {
	if rawKey == "" {
		return errors.New("empty key")
	}

	if !strings.HasPrefix(rawKey, APIKeyPrefix) {
		return errors.New("invalid key prefix: expected mk_")
	}

	// Compute SHA-256 hash of the raw key
	hash := sha256.Sum256([]byte(rawKey))

	// Timing-safe check against all known hashes
	found := false
	for knownHash := range v.knownKeys {
		knownBytes, err := hex.DecodeString(knownHash)
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare(hash[:], knownBytes) == 1 {
			found = true
			// Don't break — constant-time requires checking all keys
		}
	}

	if !found {
		return errors.New("invalid API key")
	}

	return nil
}
