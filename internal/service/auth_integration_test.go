package service_test

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

func setupIntegrationAuthService(t *testing.T, clock func() time.Time) (*service.AuthService, *store.Queries, func()) {
	t.Helper()
	dbURL := testDatabaseURL(t)
	ctx := context.Background()

	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatalf("parse db url: %v", err)
	}
	pwd, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 5432
	}

	cfg := config.DatabaseConfig{
		Host:            u.Hostname(),
		Port:            port,
		User:            u.User.Username(),
		Password:        pwd,
		Name:            strings.TrimPrefix(u.Path, "/"),
		SSLMode:         u.Query().Get("sslmode"),
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 60,
		ConnMaxIdleTime: 30,
	}

	pool, err := store.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}

	queries := store.New(pool)
	transactor := store.NewTransactor(pool)

	opts := []service.AuthOption{
		service.WithTransactor(transactor),
	}
	if clock != nil {
		opts = append(opts, service.WithClock(clock))
	}

	authSvc := service.NewAuthService(queries, testJWTSecret, opts...)

	cleanup := func() {
		pool.Close()
	}

	return authSvc, queries, cleanup
}

func TestAuthIntegration_NormalRotation(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	authSvc, queries, cleanup := setupIntegrationAuthService(t, func() time.Time { return now })
	defer cleanup()

	testEmail := "rotate-test-" + uuid.New().String() + "@example.com"
	_, err := authSvc.Signup(ctx, testEmail, "securePassword123")
	if err != nil {
		t.Fatalf("Signup failed: %v", err)
	}

	loginPair, err := authSvc.Login(ctx, testEmail, "securePassword123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	idempKey := "rotation-key-1"
	newPair, err := authSvc.Refresh(ctx, loginPair.RefreshToken, idempKey)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if newPair.AccessToken == "" || newPair.RefreshToken == "" {
		t.Fatalf("expected non-empty rotated tokens")
	}

	// Verify in database that old token is marked revoked with replacement
	oldHash := auth.HashRefreshToken(loginPair.RefreshToken)
	oldRow, err := queries.GetRefreshTokenByHashForUpdate(ctx, oldHash)
	if err != nil {
		t.Fatalf("failed to query old token row: %v", err)
	}
	if !oldRow.RevokedAt.Valid {
		t.Errorf("expected old token revoked_at to be valid")
	}
	if !oldRow.ReplacedBy.Valid {
		t.Errorf("expected old token replaced_by to be set")
	}
}

func TestAuthIntegration_BenignReplayAndTheft(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	authSvc, _, cleanup := setupIntegrationAuthService(t, func() time.Time { return now })
	defer cleanup()

	testEmail := "theft-test-" + uuid.New().String() + "@example.com"
	_, err := authSvc.Signup(ctx, testEmail, "securePassword123")
	if err != nil {
		t.Fatalf("Signup failed: %v", err)
	}

	loginPair, err := authSvc.Login(ctx, testEmail, "securePassword123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	idempKey := "client-key-abc"
	rotatedPair, err := authSvc.Refresh(ctx, loginPair.RefreshToken, idempKey)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// 1. Benign replay with identical key -> returns exact same tokens
	replayPair, err := authSvc.Refresh(ctx, loginPair.RefreshToken, idempKey)
	if err != nil {
		t.Fatalf("expected benign replay to succeed, got: %v", err)
	}
	if replayPair.AccessToken != rotatedPair.AccessToken {
		t.Errorf("replay accessToken = %q, want %q", replayPair.AccessToken, rotatedPair.AccessToken)
	}

	// 2. Theft replay with mismatched key -> revokes whole family
	_, err = authSvc.Refresh(ctx, loginPair.RefreshToken, "attacker-mismatched-key")
	if !errors.Is(err, service.ErrTokenFamilyRevoked) {
		t.Fatalf("expected ErrTokenFamilyRevoked on mismatched key, got: %v", err)
	}

	// 3. Subsequent refresh with newly rotated token should also be rejected
	_, err = authSvc.Refresh(ctx, rotatedPair.RefreshToken, "any-key")
	if !errors.Is(err, service.ErrTokenFamilyRevoked) && !errors.Is(err, service.ErrRevokedToken) {
		t.Errorf("expected token rejected after family revocation, got: %v", err)
	}
}

func TestAuthIntegration_ConcurrentRefreshRace(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	authSvc, _, cleanup := setupIntegrationAuthService(t, func() time.Time { return now })
	defer cleanup()

	testEmail := "race-test-" + uuid.New().String() + "@example.com"
	_, err := authSvc.Signup(ctx, testEmail, "securePassword123")
	if err != nil {
		t.Fatalf("Signup failed: %v", err)
	}

	loginPair, err := authSvc.Login(ctx, testEmail, "securePassword123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Concurrent benign replays with identical idempotency key
	const concurrency = 8
	var wg sync.WaitGroup
	results := make([]*domain.TokenPair, concurrency)
	errorsList := make([]error, concurrency)
	sameKey := "concurrent-same-idemp-key"

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		idx := i
		go func() {
			defer wg.Done()
			results[idx], errorsList[idx] = authSvc.Refresh(ctx, loginPair.RefreshToken, sameKey)
		}()
	}
	wg.Wait()

	// All concurrent requests must succeed and return identical access tokens
	firstToken := ""
	for i := 0; i < concurrency; i++ {
		if errorsList[i] != nil {
			t.Errorf("goroutine %d failed: %v", i, errorsList[i])
			continue
		}
		if firstToken == "" {
			firstToken = results[i].AccessToken
		} else if results[i].AccessToken != firstToken {
			t.Errorf("goroutine %d received divergent access token %q, want %q", i, results[i].AccessToken, firstToken)
		}
	}
}
