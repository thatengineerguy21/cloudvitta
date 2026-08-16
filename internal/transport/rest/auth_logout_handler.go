package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// LogoutRequest defines the JSON payload for terminating an active session.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutResponse defines the standard success message returned upon session revocation.
type LogoutResponse struct {
	Message string `json:"message"`
}

// LogoutHandler handles POST /api/v1/auth/logout.
type LogoutHandler struct {
	authSvc *service.AuthService
}

// NewLogoutHandler constructs a new LogoutHandler.
func NewLogoutHandler(authSvc *service.AuthService) *LogoutHandler {
	return &LogoutHandler{
		authSvc: authSvc,
	}
}

// @Summary Revoke session / Logout
// @Description Revokes the specified refresh token and purges it from the rotation cache
// @Tags Auth
// @Accept json
// @Produce json
// @Produce application/problem+json
// @Param request body LogoutRequest true "Logout parameters"
// @Success 200 {object} LogoutResponse "Session revoked successfully"
// @Failure 400 {object} middleware.RFC7807Error "Invalid or missing refresh_token"
// @Failure 405 {object} middleware.RFC7807Error "Method Not Allowed"
// @Failure 500 {object} middleware.RFC7807Error "Internal Server Error"
// @Router /api/v1/auth/logout [post]
func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB maximum payload

	var req LogoutRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
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

	err := h.authSvc.Logout(r.Context(), req.RefreshToken)
	if err != nil {
		status, errType, title := middleware.MapServiceError(err)
		detail := err.Error()
		if status == http.StatusInternalServerError {
			detail = "An internal server error occurred while logging out"
		}
		middleware.WriteJSONError(w, r, status, errType, title, detail)
		return
	}

	resp := LogoutResponse{
		Message: "logged out successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
