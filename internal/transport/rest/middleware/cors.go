package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/config"
)

// DefaultPublicPrefixes defines paths that receive permissive CORS (*) without credentials.
var DefaultPublicPrefixes = []string{
	"/api/v1/prices/",
	"/healthz",
	"/readyz",
	"/metrics",
	"/docs/",
}

// CORSMiddleware manages Cross-Origin Resource Sharing policy for REST endpoints.
// It enforces narrowed CORS (explicit origins with credentials) for credentialed/cookie paths,
// while preserving permissive CORS (origin: *, no credentials) on public read-only paths.
type CORSMiddleware struct {
	allowedOrigins   []string
	allowCredentials bool
	allowAll         bool
	publicPrefixes   []string
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
		publicPrefixes:   DefaultPublicPrefixes,
	}
}

// Handler returns the HTTP middleware handler.
func (c *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		isPublic := c.isPublicPath(r.URL.Path)

		if isPublic {
			// Permissive CORS on public read-only paths (never combines * with credentials)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Vary", "Origin")
		} else if origin != "" {
			// Narrowed CORS on credentialed/calculation endpoints
			if c.isOriginAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if c.allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Vary", "Origin")
			}
		}

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			if isPublic || (origin != "" && c.isOriginAllowed(origin)) {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Requested-With")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *CORSMiddleware) isPublicPath(path string) bool {
	for _, prefix := range c.publicPrefixes {
		if strings.HasPrefix(path, prefix) || path == strings.TrimSuffix(prefix, "/") {
			return true
		}
	}
	return false
}

func (c *CORSMiddleware) isOriginAllowed(origin string) bool {
	if c.allowAll && !c.allowCredentials {
		return true
	}
	for _, pattern := range c.allowedOrigins {
		if matchAllowedOrigin(pattern, origin) {
			return true
		}
	}
	return false
}

func matchAllowedOrigin(pattern, origin string) bool {
	if pattern == origin {
		return true
	}
	if pattern == "*" {
		return false // wildcard '*' must never reflect credentials dynamically
	}

	// Support scheme-aware wildcard subdomains like https://*.cloudvitta.dev or http://*.localhost
	if strings.Contains(pattern, "://*.") {
		pURL, err := url.Parse(pattern)
		if err != nil {
			return false
		}
		oURL, err := url.Parse(origin)
		if err != nil {
			return false
		}
		if pURL.Scheme != oURL.Scheme {
			return false
		}
		// pURL.Host is e.g. "*.cloudvitta.dev"
		suffix := strings.TrimPrefix(pURL.Host, "*")
		base := strings.TrimPrefix(suffix, ".")
		return oURL.Hostname() == base || strings.HasSuffix(oURL.Hostname(), suffix)
	}

	return false
}
