package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestAPIKeyValidator_Validate(t *testing.T) {
	v := NewAPIKeyValidator()

	// Register a known key hash
	testKey := "mk_test-api-key-12345"
	hash := sha256.Sum256([]byte(testKey))
	hashHex := hex.EncodeToString(hash[:])
	v.AddKeyHash(hashHex)

	t.Run("valid key accepted", func(t *testing.T) {
		err := v.Validate(testKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("wrong key rejected", func(t *testing.T) {
		err := v.Validate("mk_wrong-key-99999")
		if err == nil {
			t.Fatal("expected error for wrong key")
		}
	})

	t.Run("empty key rejected", func(t *testing.T) {
		err := v.Validate("")
		if err == nil {
			t.Fatal("expected error for empty key")
		}
	})

	t.Run("key without mk_ prefix rejected", func(t *testing.T) {
		err := v.Validate("test-api-key-12345")
		if err == nil {
			t.Fatal("expected error for key without mk_ prefix")
		}
	})

	t.Run("mk_ prefix only rejected", func(t *testing.T) {
		err := v.Validate("mk_")
		if err == nil {
			t.Fatal("expected error for mk_ prefix only")
		}
	})

	t.Run("no known keys rejects everything", func(t *testing.T) {
		emptyValidator := NewAPIKeyValidator()
		err := emptyValidator.Validate("mk_any-key")
		if err == nil {
			t.Fatal("expected error when no keys registered")
		}
	})
}
