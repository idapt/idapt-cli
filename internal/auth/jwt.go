package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"
	"io"
)

// ClockSkewTolerance is the maximum clock skew allowed for JWT validation.
const ClockSkewTolerance = 10 * time.Second

// MaxTokenSize is the maximum JWT string length accepted before parsing.
const MaxTokenSize = 8192

// Claims represents the JWT payload for machine auth tokens.
type Claims struct {
	Sub string `json:"sub"` // Actor ID
	Mid string `json:"mid"` // Machine ID
	Exp int64  `json:"exp"` // Expiration (unix timestamp)
	Iat int64  `json:"iat"` // Issued at (unix timestamp)
}

// JWTValidator validates machine auth JWTs using HKDF-derived keys.
type JWTValidator struct {
	signingKey []byte
	machineID  string
}

// NewJWTValidator creates a validator that derives the signing key from the
// base secret using HKDF-SHA256 with purpose "managed-machine".
// This matches the TypeScript implementation in lib/auth/key-derivation.ts.
func NewJWTValidator(baseSecret string, machineID string) (*JWTValidator, error) {
	if baseSecret == "" {
		return nil, errors.New("base secret is required")
	}
	if machineID == "" {
		return nil, errors.New("machine ID is required")
	}

	key, err := DeriveSigningKey(baseSecret, "managed-machine")
	if err != nil {
		return nil, fmt.Errorf("derive signing key: %w", err)
	}

	return &JWTValidator{
		signingKey: key,
		machineID:  machineID,
	}, nil
}

// DeriveSigningKey derives a purpose-specific signing key from the base secret
// using HKDF-SHA256. This must produce identical output to the TypeScript
// implementation in lib/auth/key-derivation.ts.
func DeriveSigningKey(baseSecret string, purpose string) ([]byte, error) {
	if baseSecret == "" {
		return nil, errors.New("empty secret")
	}

	// HKDF-SHA256 with:
	// - IKM: base secret as UTF-8 bytes
	// - Salt: nil (uses zero-length salt per RFC 5869)
	// - Info: purpose string as UTF-8 bytes
	// - Output: 32 bytes
	hkdfReader := hkdf.New(sha256.New, []byte(baseSecret), nil, []byte(purpose))
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("HKDF expand: %w", err)
	}

	return key, nil
}

// Validate parses and validates a JWT string, returning the claims if valid.
func (v *JWTValidator) Validate(tokenString string) (*Claims, error) {
	if len(tokenString) > MaxTokenSize {
		return nil, errors.New("token too large")
	}
	if tokenString == "" {
		return nil, errors.New("empty token")
	}

	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token: expected 3 parts")
	}

	// Decode header
	headerJSON, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	// Only accept HS256
	if header.Alg != "HS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s (only HS256 accepted)", header.Alg)
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	expectedSig := computeHMACSHA256(v.signingKey, []byte(signingInput))
	actualSig, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, errors.New("invalid signature")
	}

	// Decode payload
	payloadJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	// Validate claims
	now := time.Now().Unix()

	if claims.Exp == 0 {
		return nil, errors.New("missing exp claim")
	}
	if now > claims.Exp+int64(ClockSkewTolerance.Seconds()) {
		return nil, errors.New("token expired")
	}

	if claims.Sub == "" {
		return nil, errors.New("missing sub claim")
	}
	if claims.Mid == "" {
		return nil, errors.New("missing mid claim")
	}

	// Verify machine ID matches this agent's machine
	if claims.Mid != v.machineID {
		return nil, fmt.Errorf("machine ID mismatch: token for %s, agent is %s", claims.Mid, v.machineID)
	}

	return &claims, nil
}

func computeHMACSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func base64URLDecode(s string) ([]byte, error) {
	// Add padding if missing
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}
