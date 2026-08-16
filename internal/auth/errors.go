package auth

import "errors"

var (
	// ErrInvalidCredentials is returned when email or password verification fails.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrInvalidEmail is returned when email format is invalid.
	ErrInvalidEmail = errors.New("invalid email format")
	// ErrPasswordTooShort is returned when password is shorter than 8 characters.
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	// ErrPasswordTooLong is returned when password exceeds 72 characters.
	ErrPasswordTooLong = errors.New("password exceeds maximum allowed length of 72 characters")
	// ErrInvalidToken is returned when a JWT or refresh token is malformed or invalid.
	ErrInvalidToken = errors.New("invalid or malformed token")
	// ErrExpiredToken is returned when an access token has expired.
	ErrExpiredToken = errors.New("token has expired")
)
