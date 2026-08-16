package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// RateLimiter holds configuration for the Redis-backed per-IP rate limiter.
type RateLimiter struct {
	redisClient redis.Cmdable
	rateLimit   int64
	window      time.Duration
}

// NewRateLimiter constructs a new RateLimiter.
// Default limit is 60 requests per 1-minute window.
func NewRateLimiter(redisClient redis.Cmdable, limit int64) *RateLimiter {
	if limit <= 0 {
		limit = 60
	}
	return &RateLimiter{
		redisClient: redisClient,
		rateLimit:   limit,
		window:      time.Minute,
	}
}

// ExtractClientIP resolves the trusted client IP.
// Behind Cloud Run / Google Cloud Load Balancer, the UNFORGEABLE real IP is appended
// as the LAST element in X-Forwarded-For chain. Fallbacks to RemoteAddr.
func ExtractClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			lastIP := strings.TrimSpace(parts[len(parts)-1])
			if lastIP != "" {
				return lastIP
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	return r.RemoteAddr
}

// Handler returns HTTP middleware enforcing per-IP rate limiting using default limits.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return rl.WithProfile("default", rl.rateLimit)(next)
}

// WithProfile returns an HTTP middleware enforcing custom per-IP rate limits for a named profile (e.g., "login", limit: 10).
func (rl *RateLimiter) WithProfile(profile string, limit int64) func(http.Handler) http.Handler {
	if limit <= 0 {
		limit = rl.rateLimit
	}
	if profile == "" {
		profile = "default"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rl.redisClient == nil {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := ExtractClientIP(r)
			now := time.Now().UTC()
			minuteWindow := now.Unix() / 60
			key := fmt.Sprintf("ratelimit:ip:%s:%s:%d", profile, clientIP, minuteWindow)

			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()

			count, err := rl.redisClient.Incr(ctx, key).Result()
			if err != nil {
				slog.Warn("rate limiter redis error, failing open", "profile", profile, "ip", clientIP, "error", err)
				next.ServeHTTP(w, r)
				return
			}

			if count == 1 {
				_ = rl.redisClient.Expire(ctx, key, 70*time.Second).Err()
			}

			if count > limit {
				secondsRemaining := 60 - (now.Unix() % 60)
				if secondsRemaining <= 0 {
					secondsRemaining = 1
				}

				w.Header().Set("Retry-After", strconv.FormatInt(secondsRemaining, 10))
				middleware.WriteJSONError(
					w, r,
					http.StatusTooManyRequests,
					"https://cloudvitta.dev/errors/rate-limit-exceeded",
					"Rate limit exceeded",
					fmt.Sprintf("Rate limit of %d requests per minute exceeded. Please try again in %d seconds.", limit, secondsRemaining),
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
