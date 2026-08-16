package middleware

import (
	"net/http"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/config"
)

// CORSMiddleware manages Cross-Origin Resource Sharing policy for REST endpoints.
type CORSMiddleware struct {
	allowedOrigins   []string
	allowCredentials bool
	allowAll         bool
}

// NewCORSMiddleware constructs a CORSMiddleware instance.
func NewCORSMiddleware(cfg config.CORSConfig) *CORSMiddleware {
	allowAll := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allowAll = true
			break
		}
	}
	return &CORSMiddleware{
		allowedOrigins:   cfg.AllowedOrigins,
		allowCredentials: cfg.AllowCredentials,
		allowAll:         allowAll,
	}
}

// Handler returns the HTTP middleware handler.
func (c *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" {
			isAllowed := c.isOriginAllowed(origin)

			if isAllowed {
				if c.allowCredentials {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				} else if c.allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				w.Header().Set("Vary", "Origin")
			}
		} else if c.allowAll && !c.allowCredentials {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Requested-With")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *CORSMiddleware) isOriginAllowed(origin string) bool {
	if c.allowAll {
		return true
	}
	for _, pattern := range c.allowedOrigins {
		if pattern == origin || matchWildcardOrigin(pattern, origin) {
			return true
		}
	}
	return false
}

func matchWildcardOrigin(pattern, origin string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*.") {
		parts := strings.Split(pattern, "*.")
		if len(parts) == 2 {
			schemePrefix := parts[0]
			baseDomain := parts[1]

			if strings.HasPrefix(origin, schemePrefix) {
				originWithoutScheme := strings.TrimPrefix(origin, schemePrefix)
				if strings.HasSuffix(originWithoutScheme, "."+baseDomain) || originWithoutScheme == baseDomain {
					return true
				}
			}
		}
	}
	return false
}
