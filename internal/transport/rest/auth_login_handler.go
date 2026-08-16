package rest

import (
	"encoding/json"
	"net/http"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// LoginRequest defines the JSON payload for user authentication.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse defines the JSON response returned upon successful authentication.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// LoginHandler handles POST /api/v1/auth/login.
type LoginHandler struct {
	authSvc *service.AuthService
}

// NewLoginHandler constructs a new LoginHandler.
func NewLoginHandler(authSvc *service.AuthService) *LoginHandler {
	return &LoginHandler{
		authSvc: authSvc,
	}
}

// @Summary Authenticate user and issue tokens
// @Description Validates credentials with timing-safe comparison and issues JWT access token and opaque refresh token pair
// @Tags Auth
// @Accept json
// @Produce json
// @Produce application/problem+json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse "Authentication successful"
// @Failure 400 {object} middleware.RFC7807Error "Malformed JSON request"
// @Failure 401 {object} middleware.RFC7807Error "Invalid email or password"
// @Failure 405 {object} middleware.RFC7807Error "Method Not Allowed"
// @Failure 429 {object} middleware.RFC7807Error "Rate limit exceeded"
// @Failure 500 {object} middleware.RFC7807Error "Internal Server Error"
// @Router /api/v1/auth/login [post]
func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var req LoginRequest
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

	tokens, err := h.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		status, errType, title := middleware.MapServiceError(err)
		detail := err.Error()
		if status == http.StatusInternalServerError {
			detail = "An internal server error occurred during authentication"
		}
		middleware.WriteJSONError(w, r, status, errType, title, detail)
		return
	}

	resp := LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
