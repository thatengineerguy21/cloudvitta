package middleware

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

// RFC7807Error represents an RFC 7807 Problem Details error response.
type RFC7807Error struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

// WriteJSONError renders an RFC 7807 problem details JSON payload to the ResponseWriter.
func WriteJSONError(w http.ResponseWriter, r *http.Request, status int, errorType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	instance := r.URL.RequestURI()
	if instance == "" {
		instance = r.URL.Path
	}

	resp := RFC7807Error{
		Type:     errorType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// MapServiceError maps a service-layer error to its corresponding HTTP status, RFC 7807 type URI, and title.
func MapServiceError(err error) (status int, errorType string, title string) {
	switch {
	case errors.Is(err, service.ErrInvalidParameters),
		errors.Is(err, service.ErrInvalidEmail),
		errors.Is(err, service.ErrPasswordTooShort),
		errors.Is(err, service.ErrPasswordTooLong),
		errors.Is(err, service.ErrNoCategoriesRequested),
		errors.Is(err, service.ErrMissingIdempotencyKey):
		return http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters"
	case errors.Is(err, service.ErrInvalidCredentials),
		errors.Is(err, service.ErrInvalidToken),
		errors.Is(err, service.ErrExpiredToken),
		errors.Is(err, service.ErrRevokedToken),
		errors.Is(err, service.ErrTokenFamilyRevoked):
		return http.StatusUnauthorized, "https://cloudvitta.dev/errors/unauthorized", "Unauthorized"
	case errors.Is(err, service.ErrUserAlreadyExists):
		return http.StatusConflict, "https://cloudvitta.dev/errors/conflict", "Conflict"
	case errors.Is(err, service.ErrProviderUnavailable):
		return http.StatusBadGateway, "https://cloudvitta.dev/errors/provider-unavailable", "Provider Unavailable"
	case errors.Is(err, service.ErrNoMatchFound):
		return http.StatusNotFound, "https://cloudvitta.dev/errors/not-found", "Not Found"
	default:
		return http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-server-error", "Internal Server Error"
	}
}
