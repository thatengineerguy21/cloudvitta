package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// RecoverMiddleware intercepts panics, logs stack traces, and returns RFC 7807 500 Internal Server Error.
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				slog.ErrorContext(r.Context(), "panic recovered in HTTP handler", "panic", rec, "stack", stack, "path", r.URL.Path)

				WriteJSONError(
					w, r,
					http.StatusInternalServerError,
					"https://cloudvitta.dev/errors/internal-server-error",
					"Internal Server Error",
					fmt.Sprintf("An unexpected internal error occurred: %v", rec),
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
