package service

import (
	"errors"

	"github.com/thatengineerguy21/CloudVitta/internal/auth"
)

var (
	// ErrInvalidParameters is returned when input parameters are invalid.
	ErrInvalidParameters = errors.New("invalid parameters")
	// ErrProviderUnavailable is returned when a requested provider is not available or down.
	ErrProviderUnavailable = errors.New("provider unavailable")
	// ErrNoMatchFound is returned when no matches can be found.
	ErrNoMatchFound = errors.New("no match found")
	// ErrCategoryNotSupported is returned when a provider does not support the requested category.
	ErrCategoryNotSupported = errors.New("category not supported by provider")
	// ErrEngineMismatch is returned when database candidates were excluded due to engine mismatch.
	ErrEngineMismatch = errors.New("database candidate excluded due to engine mismatch")
	// ErrNoCategoriesRequested is returned when a composite calculate request specifies no categories.
	ErrNoCategoriesRequested = errors.New("no categories requested")
	// ErrUserAlreadyExists is returned when attempting to signup with an existing email.
	ErrUserAlreadyExists = errors.New("user with this email already exists")
	// ErrProviderNotFound is returned when a requested cloud provider is unrecognized or invalid.
	ErrProviderNotFound = errors.New("provider not found")

	// Consolidated auth error sentinels
	ErrInvalidCredentials    = auth.ErrInvalidCredentials
	ErrInvalidEmail          = auth.ErrInvalidEmail
	ErrPasswordTooShort      = auth.ErrPasswordTooShort
	ErrPasswordTooLong       = auth.ErrPasswordTooLong
	ErrInvalidToken          = auth.ErrInvalidToken
	ErrExpiredToken          = auth.ErrExpiredToken
	ErrRevokedToken          = auth.ErrRevokedToken
	ErrTokenFamilyRevoked    = auth.ErrTokenFamilyRevoked
	ErrMissingIdempotencyKey = auth.ErrMissingIdempotencyKey
)
