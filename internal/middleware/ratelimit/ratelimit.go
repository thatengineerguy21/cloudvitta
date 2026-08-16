package ratelimit

import (
	"context"
	"encoding/json"
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

type timeWindow struct {
	minuteWindow     int64
	secondsRemaining int64
	resetTimestamp   int64
}

func (rl *RateLimiter) calculateWindow() timeWindow {
	now := rl.clock().UTC()
	minWin := now.Unix() / 60
	secRem := 60 - (now.Unix() % 60)
	if secRem <= 0 {
		secRem = 1
	}
	return timeWindow{
		minuteWindow:     minWin,
		secondsRemaining: secRem,
		resetTimestamp:   (minWin + 1) * 60,
	}
}

// checkAndIncrement increments the sliding Redis counter for a given key and computes remaining allowance.
func (rl *RateLimiter) checkAndIncrement(ctx context.Context, key string, limit int64) (count int64, remaining int64, err error) {
	count, err = rl.redisClient.Incr(ctx, key).Result()
	if err != nil {
		return 0, 0, err
	}
	if count == 1 {
		_ = rl.redisClient.Expire(ctx, key, 70*time.Second).Err()
	}
	rem := limit - count
	if rem < 0 {
		rem = 0
	}
	return count, rem, nil
}

type rateLimitProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
}

func writeRateLimitRejection(w http.ResponseWriter, r *http.Request, limit int64, win timeWindow, detail string) {
	w.Header().Set("Retry-After", strconv.FormatInt(win.secondsRemaining, 10))
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(win.resetTimestamp, 10))
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusTooManyRequests)

	_ = json.NewEncoder(w).Encode(rateLimitProblemDetails{
		Type:     "https://cloudvitta.dev/errors/rate-limit-exceeded",
		Title:    "Rate limit exceeded",
		Status:   http.StatusTooManyRequests,
		Detail:   detail,
		Instance: r.URL.Path,
	})
}

// Handler returns the HTTP middleware executing the 4-step tiered rate limit resolution.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rl.redisClient == nil {
			next.ServeHTTP(w, r)
			return
		}

		win := rl.calculateWindow()
		clientIP := ExtractClientIP(r)

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		authCtx, isAuth := authmw.AuthFromContext(r.Context())
		if isAuth && authCtx.IsAuth && authCtx.UserID != "" {
			// Step 1: Standard Tier (Authenticated User JWT)
			// Authenticated users are keyed by user_id and receive their full 120 req/min quota
			// without being throttled by the anonymous blunt IP ceiling.
			tierKey := fmt.Sprintf("ratelimit:user:%s:%d", authCtx.UserID, win.minuteWindow)
			count, remaining, err := rl.checkAndIncrement(ctx, tierKey, rl.standardTierRate)
			if err != nil {
				slog.WarnContext(r.Context(), "rate limiter redis error on user tier, failing open", "userID", authCtx.UserID, "error", err)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(rl.standardTierRate, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(win.resetTimestamp, 10))

			if count > rl.standardTierRate {
				writeRateLimitRejection(
					w, r, rl.standardTierRate, win,
					fmt.Sprintf("Standard tier rate limit of %d requests per minute exceeded. Please try again in %d seconds.", rl.standardTierRate, win.secondsRemaining),
				)
				return
			}

			next.ServeHTTP(w, r)
			return
		}

		// Step 4: Outer Blunt IP Ceiling for Anonymous callers (Stops cookie churn & flood attacks)
		bluntKey := fmt.Sprintf("ratelimit:blunt_ip:%s:%d", clientIP, win.minuteWindow)
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
			writeRateLimitRejection(
				w, r, rl.ipCeilingRate, win,
				fmt.Sprintf("IP rate limit ceiling of %d requests per minute exceeded. Please try again in %d seconds.", rl.ipCeilingRate, win.secondsRemaining),
			)
			return
		}

		// Step 2 & 3: Anonymous Key Resolution & Cookie Staging
		var tierKey string
		var validAnonID string

		if cookie, cErr := r.Cookie(auth.AnonCookieName); cErr == nil && cookie != nil && cookie.Value != "" && len(rl.cookieSecret) >= 32 {
			if anonID, vErr := auth.VerifyAnonCookie(cookie.Value, rl.cookieSecret, rl.clock().UTC()); vErr == nil {
				validAnonID = anonID
			}
		}

		if validAnonID != "" {
			// Step 2: Free Tier (Anonymous Cookie)
			tierKey = fmt.Sprintf("ratelimit:anon:%s:%d", validAnonID, win.minuteWindow)
		} else {
			// Step 3: Free Tier (Client IP Fallback + Stage Fresh Cookie)
			tierKey = fmt.Sprintf("ratelimit:ip:%s:%d", clientIP, win.minuteWindow)

			if len(rl.cookieSecret) >= 32 {
				isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
				if freshCookie, mErr := auth.MintAnonCookie(rl.cookieSecret, rl.clock().UTC(), isSecure); mErr == nil && freshCookie != nil {
					http.SetCookie(w, freshCookie)
				}
			}
		}

		count, remaining, err := rl.checkAndIncrement(ctx, tierKey, rl.freeTierRate)
		if err != nil {
			slog.WarnContext(r.Context(), "rate limiter redis error on tier counter, failing open", "key", tierKey, "error", err)
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(rl.freeTierRate, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(win.resetTimestamp, 10))

		if count > rl.freeTierRate {
			writeRateLimitRejection(
				w, r, rl.freeTierRate, win,
				fmt.Sprintf("Rate limit of %d requests per minute exceeded. Please try again in %d seconds.", rl.freeTierRate, win.secondsRemaining),
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// WithProfile returns an HTTP middleware enforcing route-specific rate limits (e.g. login).
// Outer blunt IP ceiling is also enforced for credential protection.
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

			win := rl.calculateWindow()
			clientIP := ExtractClientIP(r)

			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()

			// Check Outer Blunt IP Ceiling
			bluntKey := fmt.Sprintf("ratelimit:blunt_ip:%s:%d", clientIP, win.minuteWindow)
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
				writeRateLimitRejection(
					w, r, rl.ipCeilingRate, win,
					fmt.Sprintf("IP rate limit ceiling of %d requests per minute exceeded. Please try again in %d seconds.", rl.ipCeilingRate, win.secondsRemaining),
				)
				return
			}

			// Profile-specific counter
			profileKey := fmt.Sprintf("ratelimit:profile:%s:ip:%s:%d", profile, clientIP, win.minuteWindow)
			count, remaining, err := rl.checkAndIncrement(ctx, profileKey, limit)
			if err != nil {
				slog.WarnContext(r.Context(), "rate limiter redis error on profile counter, failing open", "profile", profile, "key", profileKey, "error", err)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(win.resetTimestamp, 10))

			if count > limit {
				writeRateLimitRejection(
					w, r, limit, win,
					fmt.Sprintf("Rate limit of %d requests per minute exceeded for %s. Please try again in %d seconds.", limit, profile, win.secondsRemaining),
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
