package mcp_test

import (
	"errors"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/mcp"
)

func TestMapServiceError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
		nilCheck bool
	}{
		{
			name:     "nil error",
			input:    nil,
			nilCheck: true,
		},
		{
			name:     "invalid parameters",
			input:    service.ErrInvalidParameters,
			expected: "invalid parameters: " + service.ErrInvalidParameters.Error(),
		},
		{
			name:     "no categories requested",
			input:    service.ErrNoCategoriesRequested,
			expected: "at least one category ('compute', 'storage', 'network', 'database_rdbms', 'database_nosql', 'kubernetes', or 'serverless') must be specified",
		},
		{
			name:     "provider unavailable",
			input:    service.ErrProviderUnavailable,
			expected: "all cloud providers failed to retrieve pricing data",
		},
		{
			name:     "provider not found",
			input:    service.ErrProviderNotFound,
			expected: "provider not found: " + service.ErrProviderNotFound.Error(),
		},
		{
			name:     "no match found",
			input:    service.ErrNoMatchFound,
			expected: "no matching SKU found within acceptable thresholds",
		},
		{
			name:     "category not supported",
			input:    service.ErrCategoryNotSupported,
			expected: "requested category is not supported by this provider",
		},
		{
			name:     "invalid credentials",
			input:    service.ErrInvalidCredentials,
			expected: "unauthorized: invalid or expired authentication token",
		},
		{
			name:     "invalid token",
			input:    service.ErrInvalidToken,
			expected: "unauthorized: invalid or expired authentication token",
		},
		{
			name:     "expired token",
			input:    service.ErrExpiredToken,
			expected: "unauthorized: invalid or expired authentication token",
		},
		{
			name:     "revoked token",
			input:    service.ErrRevokedToken,
			expected: "unauthorized: invalid or expired authentication token",
		},
		{
			name:     "token family revoked",
			input:    service.ErrTokenFamilyRevoked,
			expected: "unauthorized: invalid or expired authentication token",
		},
		{
			name:     "unknown internal error",
			input:    errors.New("something very secret in db"),
			expected: "an internal error occurred while processing the request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mcp.MapServiceError(tt.input)
			if tt.nilCheck {
				if mapped != nil {
					t.Fatalf("expected nil, got %v", mapped)
				}
				return
			}
			if mapped == nil {
				t.Fatalf("expected non-nil error, got nil")
			}
			if mapped.Error() != tt.expected {
				t.Errorf("expected error message %q, got %q", tt.expected, mapped.Error())
			}
		})
	}
}
