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
	// ErrNoCategoriesRequested is returned when a composite calculate request specifies no categories.
	ErrNoCategoriesRequested = errors.New("no categories requested")
	// ErrUserAlreadyExists is returned when attempting to signup with an existing email.
	ErrUserAlreadyExists = errors.New("user with this email already exists")

	// Consolidated auth error sentinels
	ErrInvalidCredentials = auth.ErrInvalidCredentials
	ErrInvalidEmail       = auth.ErrInvalidEmail
	ErrPasswordTooShort   = auth.ErrPasswordTooShort
	ErrPasswordTooLong    = auth.ErrPasswordTooLong
)
