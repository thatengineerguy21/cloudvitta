package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// RefreshRequest defines the JSON payload for token rotation.
// Both refresh_token and idempotency_key are required fields.
type RefreshRequest struct {
	RefreshToken   string `json:"refresh_token"`
	IdempotencyKey string `json:"idempotency_key"`
}

// RefreshResponse defines the JSON response returned upon successful token rotation.
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// RefreshHandler handles POST /api/v1/auth/refresh.
type RefreshHandler struct {
	authSvc *service.AuthService
}

// NewRefreshHandler constructs a new RefreshHandler.
func NewRefreshHandler(authSvc *service.AuthService) *RefreshHandler {
	return &RefreshHandler{
		authSvc: authSvc,
	}
}

// @Summary Rotate refresh token
// @Description Rotates a valid refresh token, minting a new access and refresh token pair with idempotency-key gated replay and whole-family theft containment
// @Tags Auth
// @Accept json
// @Produce json
// @Produce application/problem+json
// @Param request body RefreshRequest true "Refresh token parameters"
// @Success 200 {object} RefreshResponse "Tokens rotated successfully"
// @Failure 400 {object} middleware.RFC7807Error "Invalid or missing refresh_token or idempotency_key"
// @Failure 401 {object} middleware.RFC7807Error "Invalid, expired, revoked, or compromised token family"
// @Failure 405 {object} middleware.RFC7807Error "Method Not Allowed"
// @Failure 500 {object} middleware.RFC7807Error "Internal Server Error"
// @Router /api/v1/auth/refresh [post]
func (h *RefreshHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		middleware.WriteJSONError(
			w, r,
			http.StatusMethodNotAllowed,
			"https://cloudvitta.dev/errors/method-not-allowed",
			"Method Not Allowed",
			"Only POST requests are supported on this endpoint",
		)
		return
	}

	var req RefreshRequest
	if err := DecodeJSONBody(w, r, &req); err != nil {
		middleware.WriteJSONError(
			w, r,
			http.StatusBadRequest,
			"https://cloudvitta.dev/errors/invalid-parameter",
			"Invalid Parameters",
			"Malformed JSON request body",
		)
		return
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		middleware.WriteJSONError(
			w, r,
			http.StatusBadRequest,
			"https://cloudvitta.dev/errors/invalid-parameter",
			"Invalid Parameters",
			"refresh_token is required",
		)
		return
	}

	if strings.TrimSpace(req.IdempotencyKey) == "" {
		middleware.WriteJSONError(
			w, r,
			http.StatusBadRequest,
			"https://cloudvitta.dev/errors/invalid-parameter",
			"Invalid Parameters",
			"idempotency_key is required for refresh token rotation",
		)
		return
	}

	tokens, err := h.authSvc.Refresh(r.Context(), req.RefreshToken, req.IdempotencyKey)
	if err != nil {
		status, errType, title := middleware.MapServiceError(err)
		detail := err.Error()
		if status == http.StatusInternalServerError {
			detail = "An internal server error occurred while refreshing token"
		}
		middleware.WriteJSONError(w, r, status, errType, title, detail)
		return
	}

	resp := RefreshResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
