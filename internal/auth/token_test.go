package auth_test

import (
	"encoding/hex"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/auth"
)

func TestGenerateRefreshToken(t *testing.T) {
	rawToken, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	// 256 bits = 32 bytes = 64 hex characters
	if len(rawToken) != 64 {
		t.Errorf("rawToken length = %d, want 64", len(rawToken))
	}
	if len(tokenHash) != 64 {
		t.Errorf("tokenHash length = %d, want 64", len(tokenHash))
	}

	// Must be valid hex
	if _, err := hex.DecodeString(rawToken); err != nil {
		t.Errorf("rawToken is not valid hex: %v", err)
	}
	if _, err := hex.DecodeString(tokenHash); err != nil {
		t.Errorf("tokenHash is not valid hex: %v", err)
	}

	// Token hash must match deterministic HashRefreshToken calculation
	expectedHash := auth.HashRefreshToken(rawToken)
	if tokenHash != expectedHash {
		t.Errorf("tokenHash = %q, want %q", tokenHash, expectedHash)
	}

	// Raw token and token hash must be distinct
	if rawToken == tokenHash {
		t.Errorf("rawToken and tokenHash should not be equal")
	}
}
