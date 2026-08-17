package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "correctHorseBatteryStaple123"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if err := auth.CheckPassword(password, hash); err != nil {
		t.Errorf("CheckPassword with valid password failed: %v", err)
	}

	if err := auth.CheckPassword("wrongPassword", hash); err == nil {
		t.Errorf("CheckPassword with wrong password expected error, got nil")
	}
}

func TestDummyHash_MatchesCost(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(auth.DummyBcryptHashForTest))
	if err != nil {
		t.Fatalf("failed to extract cost from DummyBcryptHash: %v", err)
	}

	if cost != bcrypt.DefaultCost {
		t.Errorf("DummyBcryptHash cost = %d, want bcrypt.DefaultCost (%d)", cost, bcrypt.DefaultCost)
	}
}

func TestCheckPasswordTimingSafe(t *testing.T) {
	password := "securePassphrase456"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// User found, correct password
	if err := auth.CheckPasswordTimingSafe(true, hash, password); err != nil {
		t.Errorf("expected nil error for valid user and password, got: %v", err)
	}

	// User found, wrong password
	if err := auth.CheckPasswordTimingSafe(true, hash, "incorrectPass"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}

	// User not found (executes dummy hash check)
	if err := auth.CheckPasswordTimingSafe(false, "", password); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for nonexistent user, got: %v", err)
	}
}

func TestPasswordValidation(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"valid password", "password123", nil},
		{"minimum 8 characters", "12345678", nil},
		{"maximum 72 characters", strings.Repeat("a", 72), nil},
		{"too short (7 chars)", "1234567", auth.ErrPasswordTooShort},
		{"empty password", "", auth.ErrPasswordTooShort},
		{"too long (73 chars)", strings.Repeat("a", 73), auth.ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidatePassword(tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestEmailValidationAndNormalization(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		wantEmail string
		wantErr   error
	}{
		{"standard email", "user@example.com", "user@example.com", nil},
		{"uppercase email", "User@Example.COM", "user@example.com", nil},
		{"whitespace padded", "   user@example.com   ", "user@example.com", nil},
		{"mixed case with spaces", "   Alice.Smith@Domain.Co.UK  ", "alice.smith@domain.co.uk", nil},
		{"empty string", "", "", auth.ErrInvalidEmail},
		{"no at sign", "userexample.com", "", auth.ErrInvalidEmail},
		{"no domain dot", "user@localhost", "", auth.ErrInvalidEmail},
		{"no local part", "@example.com", "", auth.ErrInvalidEmail},
		{"no domain part", "user@", "", auth.ErrInvalidEmail},
		{"exceeds 255 chars", strings.Repeat("a", 250) + "@example.com", "", auth.ErrInvalidEmail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auth.NormalizeAndValidateEmail(tt.email)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NormalizeAndValidateEmail(%q) err = %v, want %v", tt.email, err, tt.wantErr)
			}
			if err == nil && got != tt.wantEmail {
				t.Errorf("NormalizeAndValidateEmail(%q) = %q, want %q", tt.email, got, tt.wantEmail)
			}
		})
	}
}
