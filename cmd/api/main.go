package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/observability"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	serviceName := "cloudvitta-api"

	// --- Observability & OpenTelemetry Setup ---
	otelProviders, err := observability.InitOTel(ctx, observability.Config{
		ServiceName: serviceName,
		Endpoint:    cfg.Observability.OTLPEndpoint,
		Headers:     cfg.Observability.OTLPHeaders,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "OpenTelemetry initialization error: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelProviders.Shutdown(shutdownCtx)
	}()

	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Primary.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	logger := observability.SetupLogger(serviceName, level, otelProviders.LoggerProvider, os.Stdout)
	slog.SetDefault(logger)

	slog.InfoContext(ctx, "starting CloudVitta API server", "port", cfg.Server.Port, "environment", cfg.Primary.Environment)

	// --- Cache Metrics ---
	cacheMetrics, err := observability.NewCacheMetrics(otelProviders.Meter)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create cache metrics", "error", err)
	}

	// --- Database ---
	dbPool, err := store.NewPool(ctx, cfg.Database)
	if err != nil {
		slog.ErrorContext(ctx, "database connection failed", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// --- Redis Cache ---
	var redisClient redis.Cmdable
	if cfg.Redis.URL != "" {
		rc, err := cache.NewClient(cfg.Redis.URL)
		if err != nil {
			safeURL := "<invalid-url>"
			if parsed, pErr := url.Parse(cfg.Redis.URL); pErr == nil {
				parsed.User = nil
				safeURL = parsed.String()
			}
			slog.WarnContext(ctx, "failed to connect to redis, proceeding with database-only read path", "url", safeURL, "error", err)
		} else {
			defer func() { _ = rc.Close() }()
			redisClient = rc
		}
	}

	// --- Services & Router ---
	queries := store.New(dbPool)
	transactor := store.NewTransactor(dbPool)
	pricingSvc := service.NewPricingService(
		queries,
		redisClient,
		service.WithTracer(otelProviders.Tracer),
		service.WithCacheMetrics(cacheMetrics),
	)
	authSvc := service.NewAuthService(
		queries,
		[]byte(cfg.Auth.JWTSecret),
		service.WithTransactor(transactor),
		service.WithAuthTracer(otelProviders.Tracer),
	)
	router := rest.NewRouter(pricingSvc, authSvc, dbPool, redisClient, cfg)

	// --- HTTP Server ---
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-shutdown
	slog.InfoContext(ctx, "shutting down CloudVitta API server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, "server forced to shutdown", "error", err)
	}

	slog.InfoContext(ctx, "server exited cleanly")
}
