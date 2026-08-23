package mcp

import (
	"errors"
	"fmt"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

// MapServiceError maps a domain or service error to an informative error for MCP tools.
func MapServiceError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, service.ErrInvalidParameters):
		return fmt.Errorf("invalid parameters: %w", err)
	case errors.Is(err, service.ErrNoCategoriesRequested):
		return errors.New("at least one category ('compute', 'storage', 'network', 'database_rdbms', 'database_nosql', 'kubernetes', or 'serverless') must be specified")
	case errors.Is(err, service.ErrProviderUnavailable):
		return errors.New("all cloud providers failed to retrieve pricing data")
	case errors.Is(err, service.ErrProviderNotFound):
		return fmt.Errorf("provider not found: %w", err)
	case errors.Is(err, service.ErrNoMatchFound):
		return errors.New("no matching SKU found within acceptable thresholds")
	case errors.Is(err, service.ErrCategoryNotSupported):
		return errors.New("requested category is not supported by this provider")
	case errors.Is(err, service.ErrInvalidCredentials),
		errors.Is(err, service.ErrInvalidToken),
		errors.Is(err, service.ErrExpiredToken),
		errors.Is(err, service.ErrRevokedToken),
		errors.Is(err, service.ErrTokenFamilyRevoked):
		return errors.New("unauthorized: invalid or expired authentication token")
	default:
		return errors.New("an internal error occurred while processing the request")
	}
}
