package middleware_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

func TestMapServiceError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "Invalid Parameters",
			err:        service.ErrInvalidParameters,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Conflicting Fields",
			err:        service.ErrConflictingFields,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Provider Unavailable",
			err:        service.ErrProviderUnavailable,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "No Match Found",
			err:        service.ErrNoMatchFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Email Not Verified",
			err:        service.ErrEmailNotVerified,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "Verification Expired",
			err:        service.ErrVerificationExpired,
			wantStatus: http.StatusGone,
		},
		{
			name:       "Verification Consumed",
			err:        service.ErrVerificationConsumed,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "Verification Not Found",
			err:        service.ErrVerificationNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Too Many Verifications",
			err:        service.ErrTooManyVerifications,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "Unknown Error",
			err:        errors.New("some unknown DB error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, _, _ := middleware.MapServiceError(tc.err)
			if status != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, status)
			}
		})
	}
}
