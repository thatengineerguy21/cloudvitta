package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// SignupRequest defines the JSON payload for user registration.
type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SignupResponse defines the JSON response returned upon successful user registration.
type SignupResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// SignupHandler handles POST /api/v1/auth/signup.
type SignupHandler struct {
	authSvc *service.AuthService
}

// NewSignupHandler constructs a new SignupHandler.
func NewSignupHandler(authSvc *service.AuthService) *SignupHandler {
	return &SignupHandler{
		authSvc: authSvc,
	}
}

// @Summary Register a new user
// @Description Creates a new user account with email and password
// @Tags Auth
// @Accept json
// @Produce json
// @Produce application/problem+json
// @Param request body SignupRequest true "Signup parameters"
// @Success 201 {object} SignupResponse "User registered successfully"
// @Failure 400 {object} middleware.RFC7807Error "Invalid email or password format"
// @Failure 409 {object} middleware.RFC7807Error "User with this email already exists"
// @Failure 405 {object} middleware.RFC7807Error "Method Not Allowed"
// @Failure 500 {object} middleware.RFC7807Error "Internal Server Error"
// @Router /api/v1/auth/signup [post]
func (h *SignupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var req SignupRequest
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

	user, err := h.authSvc.Signup(r.Context(), req.Email, req.Password)
	if err != nil {
		status, errType, title := middleware.MapServiceError(err)
		detail := err.Error()
		if status == http.StatusInternalServerError {
			detail = "An internal server error occurred while creating user"
		}
		middleware.WriteJSONError(w, r, status, errType, title, detail)
		return
	}

	resp := SignupResponse{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
