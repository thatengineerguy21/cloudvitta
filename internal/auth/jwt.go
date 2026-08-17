package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

type authCustomClaims struct {
	Tier string `json:"tier"`
	jwt.RegisteredClaims
}

// GenerateAccessToken mints an HS256-signed JWT access token.
func GenerateAccessToken(userID uuid.UUID, tier string, secret []byte, now time.Time, duration time.Duration) (string, error) {
	if len(secret) < 32 {
		return "", fmt.Errorf("jwt secret must be at least 32 bytes")
	}

	claims := authCustomClaims{
		Tier: tier,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now.UTC()),
			ExpiresAt: jwt.NewNumericDate(now.UTC().Add(duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ValidateAccessToken parses and validates an HS256 JWT access token, returning pure domain JWTClaims.
func ValidateAccessToken(tokenString string, secret []byte, now time.Time) (*domain.JWTClaims, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("jwt secret must be at least 32 bytes")
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithLeeway(30*time.Second),
		jwt.WithTimeFunc(func() time.Time {
			return now.UTC()
		}),
	)

	var claims authCustomClaims
	token, err := parser.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	var iat, exp int64
	if claims.IssuedAt != nil {
		iat = claims.IssuedAt.Unix()
	}
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Unix()
	}

	return &domain.JWTClaims{
		Subject:   claims.Subject,
		Tier:      claims.Tier,
		IssuedAt:  iat,
		ExpiresAt: exp,
	}, nil
}
