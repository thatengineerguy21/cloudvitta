package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
)

var testJWTSecret = []byte("super-secret-jwt-key-with-at-least-32-bytes-length!")

func TestGenerateAndValidateAccessToken(t *testing.T) {
	userID := uuid.New()
	tier := "standard"
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	duration := 15 * time.Minute

	token, err := auth.GenerateAccessToken(userID, tier, testJWTSecret, now, duration)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	claims, err := auth.ValidateAccessToken(token, testJWTSecret, now)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if claims.Subject != userID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, userID.String())
	}
	if claims.Tier != tier {
		t.Errorf("Tier = %q, want %q", claims.Tier, tier)
	}
	if claims.IssuedAt != now.Unix() {
		t.Errorf("IssuedAt = %d, want %d", claims.IssuedAt, now.Unix())
	}
	if claims.ExpiresAt != now.Add(duration).Unix() {
		t.Errorf("ExpiresAt = %d, want %d", claims.ExpiresAt, now.Add(duration).Unix())
	}
}

func TestExpiredAccessToken(t *testing.T) {
	userID := uuid.New()
	tier := "standard"
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	duration := 15 * time.Minute

	token, err := auth.GenerateAccessToken(userID, tier, testJWTSecret, now, duration)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	// Validate 20 minutes later (past 15m expiration + 30s leeway)
	validateTime := now.Add(20 * time.Minute)
	_, err = auth.ValidateAccessToken(token, testJWTSecret, validateTime)
	if !errors.Is(err, auth.ErrExpiredToken) {
		t.Errorf("expected ErrExpiredToken, got: %v", err)
	}
}

func TestClockSkewTolerance(t *testing.T) {
	userID := uuid.New()
	tier := "standard"
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	duration := 15 * time.Minute

	token, err := auth.GenerateAccessToken(userID, tier, testJWTSecret, now, duration)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	// Validate 15 minutes and 15 seconds later (within 30s leeway)
	validateTime := now.Add(15*time.Minute + 15*time.Second)
	claims, err := auth.ValidateAccessToken(token, testJWTSecret, validateTime)
	if err != nil {
		t.Errorf("token within 30s leeway failed to validate: %v", err)
	}
	if claims.Subject != userID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, userID.String())
	}
}

func TestInvalidSignature(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	duration := 15 * time.Minute

	token, err := auth.GenerateAccessToken(userID, "standard", testJWTSecret, now, duration)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	wrongSecret := []byte("different-secret-key-that-is-at-least-32-bytes-long!")
	_, err = auth.ValidateAccessToken(token, wrongSecret, now)
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for invalid signature, got: %v", err)
	}
}

func TestShortJWTSecretRejected(t *testing.T) {
	userID := uuid.New()
	shortSecret := []byte("too-short")

	_, err := auth.GenerateAccessToken(userID, "standard", shortSecret, time.Now(), 15*time.Minute)
	if err == nil {
		t.Errorf("GenerateAccessToken expected error for short secret, got nil")
	}

	_, err = auth.ValidateAccessToken("dummy.token.string", shortSecret, time.Now())
	if err == nil {
		t.Errorf("ValidateAccessToken expected error for short secret, got nil")
	}
}
