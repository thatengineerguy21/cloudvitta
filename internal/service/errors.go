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
)
