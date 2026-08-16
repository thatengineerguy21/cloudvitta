package service

import "errors"

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
	// ErrInvalidCredentials is returned when email/password verification fails.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrInvalidEmail is returned when email format is invalid.
	ErrInvalidEmail = errors.New("invalid email format")
	// ErrPasswordTooShort is returned when password is shorter than 8 characters.
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	// ErrPasswordTooLong is returned when password exceeds 72 characters.
	ErrPasswordTooLong = errors.New("password exceeds maximum allowed length of 72 characters")
)
