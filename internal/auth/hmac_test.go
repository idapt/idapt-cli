package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateMachineHMAC_HexEncodedToken(t *testing.T) {
	// machineToken is stored as hex in config.json — must be decoded to binary for HMAC.
	// This test verifies the fix for the heartbeat 401 / proxy 403 bug.
	rawKey := []byte("32-byte-key-for-hmac-testing-ok!")
	hexToken := hex.EncodeToString(rawKey)

	timestamp := "1700000000"
	message := "POST:/api/proxy:" + timestamp

	// Compute signature using the raw binary key (what TypeScript server does)
	mac := hmac.New(sha256.New, rawKey)
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/api/proxy", nil)
	req.Header.Set("X-Machine-Signature", signature)
	req.Header.Set("X-Machine-Timestamp", timestamp)

	err := ValidateMachineHMAC(req, hexToken)
	if err != nil {
		t.Errorf("ValidateMachineHMAC should accept hex-decoded token signature: %v", err)
	}
}

func TestValidateMachineHMAC_PlainStringToken(t *testing.T) {
	// Backward compatibility: plain string tokens (not valid hex) should still work
	token := "plain-secret-not-hex"
	timestamp := "1700000000"
	message := "GET:/api/firewall:" + timestamp

	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("GET", "/api/firewall", nil)
	req.Header.Set("X-Machine-Signature", signature)
	req.Header.Set("X-Machine-Timestamp", timestamp)

	err := ValidateMachineHMAC(req, token)
	if err != nil {
		t.Errorf("ValidateMachineHMAC should accept plain string token signature: %v", err)
	}
}

func TestValidateMachineHMAC_RejectsOldBuggySignature(t *testing.T) {
	// Old bug: daemon used []byte(hexString) instead of hex.DecodeString(hexString).
	// The server now correctly decodes, so old-format signatures must be rejected.
	rawKey := []byte("32-byte-key-for-hmac-testing-ok!")
	hexToken := hex.EncodeToString(rawKey)

	timestamp := "1700000000"
	message := "POST:/api/proxy:" + timestamp

	// Compute signature using the WRONG approach (hex string as raw bytes)
	mac := hmac.New(sha256.New, []byte(hexToken)) // old buggy way
	mac.Write([]byte(message))
	wrongSignature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/api/proxy", nil)
	req.Header.Set("X-Machine-Signature", wrongSignature)
	req.Header.Set("X-Machine-Timestamp", timestamp)

	err := ValidateMachineHMAC(req, hexToken)
	if err == nil {
		t.Error("ValidateMachineHMAC should reject signature computed with hex string as raw bytes")
	}
}

func TestValidateMachineHMAC_MissingHeaders(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		timestamp string
	}{
		{"missing signature", "", "1700000000"},
		{"missing timestamp", "abc123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/proxy", nil)
			if tt.signature != "" {
				req.Header.Set("X-Machine-Signature", tt.signature)
			}
			if tt.timestamp != "" {
				req.Header.Set("X-Machine-Timestamp", tt.timestamp)
			}

			err := ValidateMachineHMAC(req, "some-token")
			if err == nil {
				t.Error("expected error for missing headers")
			}
		})
	}
}

func TestValidateMachineHMAC_InvalidSignatureEncoding(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/proxy", nil)
	req.Header.Set("X-Machine-Signature", "not-valid-hex!!!")
	req.Header.Set("X-Machine-Timestamp", "1700000000")

	err := ValidateMachineHMAC(req, "token")
	if err == nil {
		t.Error("expected error for invalid signature encoding")
	}
}

func TestValidateMachineHMAC_MethodAndPath(t *testing.T) {
	// Verify that different methods/paths produce different signatures
	token := "test-token"
	timestamp := "1700000000"

	signWith := func(method, path string) string {
		message := method + ":" + path + ":" + timestamp
		mac := hmac.New(sha256.New, []byte(token))
		mac.Write([]byte(message))
		return hex.EncodeToString(mac.Sum(nil))
	}

	// Sign for GET /api/proxy
	getSig := signWith("GET", "/api/proxy")

	// But send as POST — should fail
	req := httptest.NewRequest(http.MethodPost, "/api/proxy", nil)
	req.Header.Set("X-Machine-Signature", getSig)
	req.Header.Set("X-Machine-Timestamp", timestamp)

	err := ValidateMachineHMAC(req, token)
	if err == nil {
		t.Error("expected error when method doesn't match signature")
	}
}
