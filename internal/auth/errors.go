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
	// ErrExpiredToken is returned when an access token or refresh token has expired.
	ErrExpiredToken = errors.New("token has expired")
	// ErrRevokedToken is returned when an already revoked refresh token is presented.
	ErrRevokedToken = errors.New("token has been revoked")
	// ErrTokenFamilyRevoked is returned when token theft is detected and the entire family is invalidated.
	ErrTokenFamilyRevoked = errors.New("token family revoked due to theft detection")
	// ErrMissingIdempotencyKey is returned when a refresh request is missing the required idempotency key.
	ErrMissingIdempotencyKey = errors.New("idempotency_key is required for refresh token rotation")
	// ErrInvalidCookie is returned when an anonymous tracking cookie is malformed or signature verification fails.
	ErrInvalidCookie = errors.New("invalid or tampered anonymous cookie")
	// ErrExpiredCookie is returned when an anonymous tracking cookie has expired.
	ErrExpiredCookie = errors.New("anonymous cookie has expired")
)
