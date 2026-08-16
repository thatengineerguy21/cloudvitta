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
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/authmw"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// Config holds settings for constructing the tiered RateLimiter.
type Config struct {
	StandardTierRate int64
	FreeTierRate     int64
	IPCeilingRate    int64
	CookieSecret     []byte
	Clock            func() time.Time
}

// RateLimiter implements the centralized 4-step tiered rate limiting engine.
type RateLimiter struct {
	redisClient      redis.Cmdable
	cookieSecret     []byte
	standardTierRate int64
	freeTierRate     int64
	ipCeilingRate    int64
	clock            func() time.Time
}

// NewRateLimiter constructs a new RateLimiter instance.
func NewRateLimiter(redisClient redis.Cmdable, cfg Config) *RateLimiter {
	if cfg.StandardTierRate <= 0 {
		cfg.StandardTierRate = 120
	}
	if cfg.FreeTierRate <= 0 {
		cfg.FreeTierRate = 20
	}
	if cfg.IPCeilingRate <= 0 {
		cfg.IPCeilingRate = 60
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}

	return &RateLimiter{
		redisClient:      redisClient,
		cookieSecret:     cfg.CookieSecret,
		standardTierRate: cfg.StandardTierRate,
		freeTierRate:     cfg.FreeTierRate,
		ipCeilingRate:    cfg.IPCeilingRate,
		clock:            cfg.Clock,
	}
}

// ExtractClientIP resolves the trusted client IP.
// Behind Cloud Run / Google Cloud Load Balancer, the unforgeable client IP is the
// last element in the X-Forwarded-For chain. Fallbacks to RemoteAddr.
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

// Handler returns the HTTP middleware executing the 4-step tiered rate limit resolution.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rl.redisClient == nil {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := ExtractClientIP(r)
		now := rl.clock().UTC()
		minuteWindow := now.Unix() / 60
		secondsRemaining := 60 - (now.Unix() % 60)
		if secondsRemaining <= 0 {
			secondsRemaining = 1
		}
		resetTimestamp := (minuteWindow + 1) * 60

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		// Step 4: Outer Blunt IP Ceiling (Enforced in ALL Cases)
		bluntKey := fmt.Sprintf("ratelimit:blunt_ip:%s:%d", clientIP, minuteWindow)
		bluntCount, err := rl.redisClient.Incr(ctx, bluntKey).Result()
		if err != nil {
			slog.WarnContext(r.Context(), "rate limiter redis error on blunt ceiling, failing open", "ip", clientIP, "error", err)
			next.ServeHTTP(w, r)
			return
		}
		if bluntCount == 1 {
			_ = rl.redisClient.Expire(ctx, bluntKey, 70*time.Second).Err()
		}

		if bluntCount > rl.ipCeilingRate {
			w.Header().Set("Retry-After", strconv.FormatInt(secondsRemaining, 10))
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(rl.ipCeilingRate, 10))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTimestamp, 10))
			middleware.WriteJSONError(
				w, r,
				http.StatusTooManyRequests,
				"https://cloudvitta.dev/errors/rate-limit-exceeded",
				"Rate limit exceeded",
				fmt.Sprintf("IP rate limit ceiling of %d requests per minute exceeded. Please try again in %d seconds.", rl.ipCeilingRate, secondsRemaining),
			)
			return
		}

		// Step 1 to 3: Tier Key Resolution & Cookie Staging
		var tierKey string
		var tierLimit int64

		authCtx, isAuth := authmw.AuthFromContext(r.Context())
		if isAuth && authCtx.IsAuth && authCtx.UserID != "" {
			// Step 1: Standard Tier (Authenticated User JWT)
			tierKey = fmt.Sprintf("ratelimit:user:%s:%d", authCtx.UserID, minuteWindow)
			tierLimit = rl.standardTierRate
		} else {
			// Check for valid cv_anon_id cookie
			var validAnonID string
			if cookie, cErr := r.Cookie(auth.AnonCookieName); cErr == nil && cookie != nil && cookie.Value != "" && len(rl.cookieSecret) >= 32 {
				if anonID, vErr := auth.VerifyAnonCookie(cookie.Value, rl.cookieSecret, now); vErr == nil {
					validAnonID = anonID
				}
			}

			if validAnonID != "" {
				// Step 2: Free Tier (Anonymous Cookie)
				tierKey = fmt.Sprintf("ratelimit:anon:%s:%d", validAnonID, minuteWindow)
				tierLimit = rl.freeTierRate
			} else {
				// Step 3: Free Tier (Client IP Fallback + Stage Fresh Cookie)
				tierKey = fmt.Sprintf("ratelimit:ip:%s:%d", clientIP, minuteWindow)
				tierLimit = rl.freeTierRate

				if len(rl.cookieSecret) >= 32 {
					isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
					if freshCookie, mErr := auth.MintAnonCookie(rl.cookieSecret, now, isSecure); mErr == nil && freshCookie != nil {
						http.SetCookie(w, freshCookie)
					}
				}
			}
		}

		// Increment resolved Tier counter
		tierCount, err := rl.redisClient.Incr(ctx, tierKey).Result()
		if err != nil {
			slog.WarnContext(r.Context(), "rate limiter redis error on tier counter, failing open", "key", tierKey, "error", err)
			next.ServeHTTP(w, r)
			return
		}
		if tierCount == 1 {
			_ = rl.redisClient.Expire(ctx, tierKey, 70*time.Second).Err()
		}

		remaining := tierLimit - tierCount
		if remaining < 0 {
			remaining = 0
		}

		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(tierLimit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTimestamp, 10))

		if tierCount > tierLimit {
			w.Header().Set("Retry-After", strconv.FormatInt(secondsRemaining, 10))
			middleware.WriteJSONError(
				w, r,
				http.StatusTooManyRequests,
				"https://cloudvitta.dev/errors/rate-limit-exceeded",
				"Rate limit exceeded",
				fmt.Sprintf("Rate limit of %d requests per minute exceeded. Please try again in %d seconds.", tierLimit, secondsRemaining),
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// WithProfile returns an HTTP middleware enforcing route-specific rate limits (e.g. login).
// Outer blunt IP ceiling is still enforced.
func (rl *RateLimiter) WithProfile(profile string, limit int64) func(http.Handler) http.Handler {
	if profile == "" || profile == "default" {
		return rl.Handler
	}
	if limit <= 0 {
		limit = rl.freeTierRate
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rl.redisClient == nil {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := ExtractClientIP(r)
			now := rl.clock().UTC()
			minuteWindow := now.Unix() / 60
			secondsRemaining := 60 - (now.Unix() % 60)
			if secondsRemaining <= 0 {
				secondsRemaining = 1
			}
			resetTimestamp := (minuteWindow + 1) * 60

			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()

			// Check Outer Blunt IP Ceiling
			bluntKey := fmt.Sprintf("ratelimit:blunt_ip:%s:%d", clientIP, minuteWindow)
			bluntCount, err := rl.redisClient.Incr(ctx, bluntKey).Result()
			if err != nil {
				slog.WarnContext(r.Context(), "rate limiter redis error on blunt ceiling, failing open", "profile", profile, "ip", clientIP, "error", err)
				next.ServeHTTP(w, r)
				return
			}
			if bluntCount == 1 {
				_ = rl.redisClient.Expire(ctx, bluntKey, 70*time.Second).Err()
			}

			if bluntCount > rl.ipCeilingRate {
				w.Header().Set("Retry-After", strconv.FormatInt(secondsRemaining, 10))
				w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(rl.ipCeilingRate, 10))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTimestamp, 10))
				middleware.WriteJSONError(
					w, r,
					http.StatusTooManyRequests,
					"https://cloudvitta.dev/errors/rate-limit-exceeded",
					"Rate limit exceeded",
					fmt.Sprintf("IP rate limit ceiling of %d requests per minute exceeded. Please try again in %d seconds.", rl.ipCeilingRate, secondsRemaining),
				)
				return
			}

			// Profile-specific counter
			profileKey := fmt.Sprintf("ratelimit:profile:%s:ip:%s:%d", profile, clientIP, minuteWindow)
			count, err := rl.redisClient.Incr(ctx, profileKey).Result()
			if err != nil {
				slog.WarnContext(r.Context(), "rate limiter redis error on profile counter, failing open", "profile", profile, "key", profileKey, "error", err)
				next.ServeHTTP(w, r)
				return
			}
			if count == 1 {
				_ = rl.redisClient.Expire(ctx, profileKey, 70*time.Second).Err()
			}

			remaining := limit - count
			if remaining < 0 {
				remaining = 0
			}

			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTimestamp, 10))

			if count > limit {
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
