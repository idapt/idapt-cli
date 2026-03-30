package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

// ClockSkewTolerance is the maximum clock skew allowed for JWT validation.
const ClockSkewTolerance = 10 * time.Second

// MaxTokenSize is the maximum JWT string length accepted before parsing.
const MaxTokenSize = 8192

// es256SignatureLen is the expected raw R||S signature length for P-256 (32+32).
const es256SignatureLen = 64

// Claims represents the JWT payload for machine auth tokens.
type Claims struct {
	Sub string `json:"sub"` // Actor ID
	Mid string `json:"mid"` // Machine ID
	Exp int64  `json:"exp"` // Expiration (unix timestamp)
	Iat int64  `json:"iat"` // Issued at (unix timestamp)
}

// JWTValidator validates machine auth JWTs using an ES256 (ECDSA P-256) public key.
// The public key is the only material needed — zero secrets on the machine.
// Thread-safe: the public key can be updated at runtime via SetPublicKey (e.g.,
// after a JWKS refresh) while Validate calls are in flight.
type JWTValidator struct {
	mu        sync.RWMutex
	publicKey *ecdsa.PublicKey
	machineID string
}

// NewJWTValidator creates a validator from a PEM-encoded EC P-256 public key.
// Only accepts ECDSA public keys on the P-256 curve.
func NewJWTValidator(publicKeyPEM string, machineID string) (*JWTValidator, error) {
	if publicKeyPEM == "" {
		return nil, errors.New("public key PEM is required")
	}
	if machineID == "" {
		return nil, errors.New("machine ID is required")
	}

	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an ECDSA public key")
	}

	if ecPub.Curve != elliptic.P256() {
		return nil, fmt.Errorf("key is not on P-256 curve (got %s)", ecPub.Curve.Params().Name)
	}

	return &JWTValidator{publicKey: ecPub, machineID: machineID}, nil
}

// NewJWTValidatorFromKey creates a validator from an already-parsed ECDSA P-256
// public key. Used when the key is fetched via JWKS instead of PEM config.
func NewJWTValidatorFromKey(publicKey *ecdsa.PublicKey, machineID string) (*JWTValidator, error) {
	if publicKey == nil {
		return nil, errors.New("public key is required")
	}
	if machineID == "" {
		return nil, errors.New("machine ID is required")
	}
	if publicKey.Curve != elliptic.P256() {
		return nil, fmt.Errorf("key is not on P-256 curve (got %s)", publicKey.Curve.Params().Name)
	}
	return &JWTValidator{publicKey: publicKey, machineID: machineID}, nil
}

// SetPublicKey replaces the validator's public key at runtime. Thread-safe.
// Used by the JWKS refresh loop to hot-swap keys without restarting.
func (v *JWTValidator) SetPublicKey(key *ecdsa.PublicKey) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.publicKey = key
}

// Validate parses and validates an ES256-signed JWT string, returning the claims if valid.
// Only accepts alg:"ES256" — rejects HS256, RS256, none, and all other algorithms
// to prevent algorithm confusion attacks.
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

	// Decode and validate header
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

	// CRITICAL SECURITY: Only accept ES256. Reject HS256, RS256, none, and all others.
	// This prevents algorithm confusion attacks where an attacker crafts an HS256 token
	// using the public key as the HMAC secret.
	if header.Alg != "ES256" {
		return nil, fmt.Errorf("unsupported algorithm: %q (only ES256 accepted)", header.Alg)
	}

	// Decode signature (raw R||S format, 32 bytes each for P-256)
	sigBytes, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	if len(sigBytes) != es256SignatureLen {
		return nil, fmt.Errorf("invalid ES256 signature length: %d (expected %d)", len(sigBytes), es256SignatureLen)
	}

	// Split into R and S components (each 32 bytes for P-256)
	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])

	// Verify ECDSA signature over the signing input (header.payload)
	signingInput := []byte(parts[0] + "." + parts[1])
	hash := sha256.Sum256(signingInput)

	// Read lock: the public key may be swapped by JWKS refresh concurrently.
	v.mu.RLock()
	pubKey := v.publicKey
	v.mu.RUnlock()

	if !ecdsa.Verify(pubKey, hash[:], r, s) {
		return nil, errors.New("invalid signature")
	}

	// Decode and validate payload
	payloadJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	// Validate expiration
	now := time.Now().Unix()
	if claims.Exp == 0 {
		return nil, errors.New("missing exp claim")
	}
	if now > claims.Exp+int64(ClockSkewTolerance.Seconds()) {
		return nil, errors.New("token expired")
	}

	// Validate required claims
	if claims.Sub == "" {
		return nil, errors.New("missing sub claim")
	}
	if claims.Mid == "" {
		return nil, errors.New("missing mid claim")
	}

	// Verify machine ID matches this daemon's machine
	if claims.Mid != v.machineID {
		return nil, fmt.Errorf("machine ID mismatch: token for %s, agent is %s", claims.Mid, v.machineID)
	}

	return &claims, nil
}

func base64URLDecode(s string) ([]byte, error) {
	// Add padding if missing (JWT uses base64url without padding)
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}
