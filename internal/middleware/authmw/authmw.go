package authmw

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/auth"
)

type contextKey string

const (
	// UserContextKey is the context key for authenticated user identity and claims.
	UserContextKey contextKey = "cloudvitta.auth.user"
)

// AuthContext holds validated user identity and subscription tier.
type AuthContext struct {
	UserID string
	Tier   string
	IsAuth bool
}

// ContextWithAuth injects AuthContext into the context.
func ContextWithAuth(ctx context.Context, authCtx AuthContext) context.Context {
	return context.WithValue(ctx, UserContextKey, authCtx)
}

// AuthFromContext extracts AuthContext from the context.
func AuthFromContext(ctx context.Context) (AuthContext, bool) {
	val, ok := ctx.Value(UserContextKey).(AuthContext)
	return val, ok
}

// NewAuthMiddleware returns transport-agnostic HTTP middleware that parses Authorization Bearer tokens.
// If valid, it injects the user identity and tier into the request context.
// Unauthenticated, expired, or malformed requests proceed to downstream handlers without failure.
func NewAuthMiddleware(jwtSecret []byte, clock func() time.Time) func(http.Handler) http.Handler {
	if clock == nil {
		clock = time.Now
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
				if tokenStr != "" {
					claims, err := auth.ValidateAccessToken(tokenStr, jwtSecret, clock())
					if err == nil && claims != nil {
						ctx := ContextWithAuth(r.Context(), AuthContext{
							UserID: claims.Subject,
							Tier:   claims.Tier,
							IsAuth: true,
						})
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					slog.DebugContext(r.Context(), "invalid or expired bearer token", "error", err)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
