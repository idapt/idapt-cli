package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
)

// ValidateMachineHMAC validates the HMAC-SHA256 signature on a management API request.
// Used by both firewall and proxy handlers for machine-level authentication.
// Message format: METHOD:PATH:TIMESTAMP
func ValidateMachineHMAC(r *http.Request, machineToken string) error {
	signature := r.Header.Get("X-Machine-Signature")
	if signature == "" {
		return fmt.Errorf("missing X-Machine-Signature header")
	}

	timestamp := r.Header.Get("X-Machine-Timestamp")
	if timestamp == "" {
		return fmt.Errorf("missing X-Machine-Timestamp header")
	}

	message := r.Method + ":" + r.URL.Path + ":" + timestamp
	// machineToken is hex-encoded, decode to binary for HMAC key
	keyBytes, decodeErr := hex.DecodeString(machineToken)
	if decodeErr != nil {
		keyBytes = []byte(machineToken) // fallback: raw bytes if not valid hex
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(message))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}
	expectedBytes, _ := hex.DecodeString(expectedSig)

	if !hmac.Equal(sigBytes, expectedBytes) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
