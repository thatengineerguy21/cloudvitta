package auth

import (
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// DummyBcryptHash is a fixed bcrypt hash computed at bcrypt.DefaultCost
// to provide constant execution time when verifying nonexistent users (ADR 0020).
var DummyBcryptHash = "$2a$10$e8wNu2eK.fPjY2sN7t1qfeO4r0L4VqX6yJ4FzT3L8W7z5yB.z1L0W"

func init() {
	if cost, err := bcrypt.Cost([]byte(DummyBcryptHash)); err != nil || cost != bcrypt.DefaultCost {
		h, err := bcrypt.GenerateFromPassword([]byte("cloudvitta-dummy-password"), bcrypt.DefaultCost)
		if err == nil {
			DummyBcryptHash = string(h)
		}
	}
}

// NormalizeAndValidateEmail trims whitespace, converts to lowercase, and validates basic email structure.
func NormalizeAndValidateEmail(email string) (string, error) {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" || len(trimmed) > 255 {
		return "", ErrInvalidEmail
	}

	normalized := strings.ToLower(trimmed)
	addr, err := mail.ParseAddress(normalized)
	if err != nil || addr.Address != normalized {
		return "", ErrInvalidEmail
	}

	parts := strings.Split(normalized, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || !strings.Contains(parts[1], ".") {
		return "", ErrInvalidEmail
	}

	return normalized, nil
}

// ValidatePassword validates password length boundaries (8 <= len <= 72).
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if len(password) > 72 {
		return ErrPasswordTooLong
	}
	return nil
}

// HashPassword hashes a plaintext password using bcrypt with DefaultCost.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// CheckPasswordTimingSafe executes bcrypt verification for both valid and nonexistent users,
// guaranteeing indistinguishable execution time to prevent email enumeration timing attacks (ADR 0020).
func CheckPasswordTimingSafe(userFound bool, realHash, password string) error {
	if userFound {
		if err := CheckPassword(password, realHash); err != nil {
			return ErrInvalidCredentials
		}
		return nil
	}

	_ = CheckPassword(password, DummyBcryptHash)
	return ErrInvalidCredentials
}
