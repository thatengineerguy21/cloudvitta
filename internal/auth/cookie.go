package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// AnonCookieName is the standard cookie name for stateless anonymous client tracking.
	AnonCookieName = "cv_anon_id"
	// AnonCookieTTL defines the lifetime of the anonymous cookie (~90 days).
	AnonCookieTTL = 90 * 24 * time.Hour
)

// MintAnonCookie generates a stateless HMAC-signed anonymous tracking cookie.
func MintAnonCookie(secret []byte, now time.Time, isSecure bool) (*http.Cookie, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("cookie secret must be at least 32 bytes")
	}

	anonID := uuid.NewString()
	ts := now.UTC().Unix()
	payload := fmt.Sprintf("%s.%d", anonID, ts)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sigHex := hex.EncodeToString(mac.Sum(nil))

	cookieVal := fmt.Sprintf("%s.%s", payload, sigHex)

	return &http.Cookie{
		Name:     AnonCookieName,
		Value:    cookieVal,
		Path:     "/",
		MaxAge:   int(AnonCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
	}, nil
}

// VerifyAnonCookie validates the stateless HMAC signature, format, and expiry of a raw cv_anon_id cookie value.
func VerifyAnonCookie(rawCookie string, secret []byte, now time.Time) (string, error) {
	if len(secret) < 32 {
		return "", fmt.Errorf("cookie secret must be at least 32 bytes")
	}

	parts := strings.Split(rawCookie, ".")
	if len(parts) != 3 {
		return "", ErrInvalidCookie
	}

	anonID := parts[0]
	tsStr := parts[1]
	sigHex := parts[2]

	if _, err := uuid.Parse(anonID); err != nil {
		return "", ErrInvalidCookie
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return "", ErrInvalidCookie
	}

	cookieTime := time.Unix(ts, 0).UTC()
	nowUTC := now.UTC()
	if nowUTC.Sub(cookieTime) > AnonCookieTTL || nowUTC.Unix() < ts-300 {
		return "", ErrExpiredCookie
	}

	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return "", ErrInvalidCookie
	}

	payload := fmt.Sprintf("%s.%s", anonID, tsStr)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(expectedSig, sigBytes) {
		return "", ErrInvalidCookie
	}

	return anonID, nil
}
