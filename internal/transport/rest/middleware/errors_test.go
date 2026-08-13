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
