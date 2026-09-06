package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// VerifyEmailRequest defines the JSON payload for verifying an email address.
type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// VerifyEmailResponse defines the JSON response returned upon successful email verification.
type VerifyEmailResponse struct {
	Message string `json:"message"`
}

// VerifyEmailHandler handles POST /api/v1/auth/verify-email.
type VerifyEmailHandler struct {
	authSvc *service.AuthService
}

// NewVerifyEmailHandler constructs a new VerifyEmailHandler.
func NewVerifyEmailHandler(authSvc *service.AuthService) *VerifyEmailHandler {
	return &VerifyEmailHandler{
		authSvc: authSvc,
	}
}

// @Summary Verify email address
// @Description Consumes a single-use verification token to activate a user account
// @Tags Auth
// @Accept json
// @Produce json
// @Produce application/problem+json
// @Param request body VerifyEmailRequest true "Verification token"
// @Success 200 {object} VerifyEmailResponse "Email verified successfully"
// @Failure 400 {object} middleware.RFC7807Error "Malformed request or missing token"
// @Failure 404 {object} middleware.RFC7807Error "Verification token not found"
// @Failure 409 {object} middleware.RFC7807Error "Verification token already consumed"
// @Failure 410 {object} middleware.RFC7807Error "Verification token expired"
// @Failure 405 {object} middleware.RFC7807Error "Method Not Allowed"
// @Failure 429 {object} middleware.RFC7807Error "Rate limit exceeded"
// @Failure 500 {object} middleware.RFC7807Error "Internal Server Error"
// @Router /api/v1/auth/verify-email [post]
func (h *VerifyEmailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var req VerifyEmailRequest
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

	if strings.TrimSpace(req.Token) == "" {
		middleware.WriteJSONError(
			w, r,
			http.StatusBadRequest,
			"https://cloudvitta.dev/errors/invalid-parameter",
			"Invalid Parameters",
			"Verification token is required",
		)
		return
	}

	if err := h.authSvc.VerifyEmail(r.Context(), req.Token); err != nil {
		status, errType, title := middleware.MapServiceError(err)
		detail := err.Error()
		if status == http.StatusInternalServerError {
			detail = "An internal server error occurred while verifying email"
		}
		middleware.WriteJSONError(w, r, status, errType, title, detail)
		return
	}

	resp := VerifyEmailResponse{
		Message: "Email verified successfully.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// ResendVerificationRequest defines the JSON payload for resending a verification email.
type ResendVerificationRequest struct {
	Email string `json:"email"`
}

// ResendVerificationResponse defines the JSON response returned for a resend verification request.
type ResendVerificationResponse struct {
	Message string `json:"message"`
}

// ResendVerificationHandler handles POST /api/v1/auth/resend-verification.
type ResendVerificationHandler struct {
	authSvc *service.AuthService
}

// NewResendVerificationHandler constructs a new ResendVerificationHandler.
func NewResendVerificationHandler(authSvc *service.AuthService) *ResendVerificationHandler {
	return &ResendVerificationHandler{
		authSvc: authSvc,
	}
}

// @Summary Resend verification email
// @Description Requests a fresh email verification link without revealing account existence
// @Tags Auth
// @Accept json
// @Produce json
// @Produce application/problem+json
// @Param request body ResendVerificationRequest true "Recipient email"
// @Success 202 {object} ResendVerificationResponse "Verification email request accepted"
// @Failure 400 {object} middleware.RFC7807Error "Malformed request or missing email"
// @Failure 405 {object} middleware.RFC7807Error "Method Not Allowed"
// @Failure 429 {object} middleware.RFC7807Error "Too many verification requests"
// @Failure 500 {object} middleware.RFC7807Error "Internal Server Error"
// @Router /api/v1/auth/resend-verification [post]
func (h *ResendVerificationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var req ResendVerificationRequest
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

	if strings.TrimSpace(req.Email) == "" {
		middleware.WriteJSONError(
			w, r,
			http.StatusBadRequest,
			"https://cloudvitta.dev/errors/invalid-parameter",
			"Invalid Parameters",
			"Email address is required",
		)
		return
	}

	if err := h.authSvc.ResendVerification(r.Context(), req.Email); err != nil {
		status, errType, title := middleware.MapServiceError(err)
		detail := err.Error()
		if status == http.StatusInternalServerError {
			detail = "An internal server error occurred while processing verification request"
		}
		middleware.WriteJSONError(w, r, status, errType, title, detail)
		return
	}

	resp := ResendVerificationResponse{
		Message: "If that email is registered and unverified, a new verification email has been sent.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(resp)
}
