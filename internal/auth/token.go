package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateRefreshToken generates a cryptographically secure 256-bit (32 bytes) random opaque token
// and returns both the plaintext token and its SHA-256 hash for database storage.
func GenerateRefreshToken() (rawToken string, tokenHash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random refresh token: %w", err)
	}

	rawToken = hex.EncodeToString(bytes)
	tokenHash = HashRefreshToken(rawToken)
	return rawToken, tokenHash, nil
}

// HashRefreshToken returns the hex-encoded SHA-256 hash of an opaque refresh token.
func HashRefreshToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
