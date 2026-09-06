package rest

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	_ "github.com/thatengineerguy21/CloudVitta/internal/transport/rest/openapi"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/authmw"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/ratelimit"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// SwaggerHandler returns the OpenAPI Swagger documentation HTTP handler.
func SwaggerHandler() http.Handler {
	return httpSwagger.WrapHandler
}

// NewRouter constructs a net/http.ServeMux with all REST API routes, health probes, metrics, and middlewares wired.
func NewRouter(pricingSvc *service.PricingService, authSvc *service.AuthService, freshnessSvc *service.FreshnessService, catalogSvc *service.CatalogService, dbPool *pgxpool.Pool, redisClient redis.Cmdable, cfg *config.Config) http.Handler {
	mux := http.NewServeMux()

	// Probes & Metrics (unlimited)
	mux.HandleFunc("/healthz", HandleHealthz)
	mux.HandleFunc("/readyz", HandleReadyz(dbPool, redisClient))
	mux.Handle("/metrics", promhttp.Handler())

	// OpenAPI Documentation (unlimited)
	mux.Handle("/docs/", httpSwagger.WrapHandler)

	// Fallback for unmapped API routes within REST subsystem
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		middleware.WriteJSONError(w, r, http.StatusNotFound, "https://cloudvitta.dev/errors/not-found", "Not Found", "The requested API endpoint does not exist.")
	})

	var jwtSecret []byte
	var anonCookieSecret []byte
	var rateCfg ratelimit.Config
	var corsCfg config.CORSConfig
	loginLimit := int64(10)

	if cfg != nil {
		jwtSecret = []byte(cfg.Auth.JWTSecret)
		anonCookieSecret = []byte(cfg.Auth.AnonCookieSecret)
		rateCfg = ratelimit.Config{
			StandardTierRate: cfg.RateLimit.StandardTierRate,
			FreeTierRate:     cfg.RateLimit.FreeTierRate,
			IPCeilingRate:    cfg.RateLimit.IPCeilingRate,
			CookieSecret:     anonCookieSecret,
		}
		corsCfg = cfg.CORS
		if cfg.RateLimit.LoginRate > 0 {
			loginLimit = cfg.RateLimit.LoginRate
		}
	}

	limiter := ratelimit.NewRateLimiter(redisClient, rateCfg)
	authMw := authmw.NewAuthMiddleware(jwtSecret, time.Now)
	corsMw := middleware.NewCORSMiddleware(corsCfg)

	computeHandler := NewComputeHandler(pricingSvc)
	storageHandler := NewStorageHandler(pricingSvc)
	networkHandler := NewNetworkHandler(pricingSvc)
	databaseHandler := NewDatabaseHandler(pricingSvc)
	databaseNoSQLHandler := NewDatabaseNoSQLHandler(pricingSvc)
	kubernetesHandler := NewKubernetesHandler(pricingSvc)
	serverlessHandler := NewServerlessHandler(pricingSvc)
	calculateHandler := NewCalculateHandler(pricingSvc)
	statusHandler := NewStatusHandler(freshnessSvc)

	// Auth Handlers
	signupHandler := NewSignupHandler(authSvc)
	loginHandler := NewLoginHandler(authSvc)
	refreshHandler := NewRefreshHandler(authSvc)
	logoutHandler := NewLogoutHandler(authSvc)
	verifyEmailHandler := NewVerifyEmailHandler(authSvc)
	resendVerificationHandler := NewResendVerificationHandler(authSvc)

	mux.Handle("GET /api/v1/prices/compute", limiter.Handler(computeHandler))
	mux.Handle("GET /api/v1/prices/storage", limiter.Handler(storageHandler))
	mux.Handle("GET /api/v1/prices/network", limiter.Handler(networkHandler))
	mux.Handle("GET /api/v1/prices/database", limiter.Handler(databaseHandler))
	mux.Handle("GET /api/v1/prices/database-nosql", limiter.Handler(databaseNoSQLHandler))
	mux.Handle("GET /api/v1/prices/kubernetes", limiter.Handler(kubernetesHandler))
	mux.Handle("GET /api/v1/prices/serverless", limiter.Handler(serverlessHandler))
	mux.Handle("GET /api/v1/providers/{provider}/status", limiter.Handler(statusHandler))
	mux.Handle("POST /api/v1/calculate", limiter.Handler(calculateHandler))
	mux.Handle("POST /api/v1/auth/signup", limiter.Handler(signupHandler))
	// Rate limit: login gets a stricter profile as credential-guessing mitigation.
	mux.Handle("POST /api/v1/auth/login", limiter.WithProfile("login", loginLimit)(loginHandler))
	mux.Handle("POST /api/v1/auth/refresh", limiter.Handler(refreshHandler))
	mux.Handle("POST /api/v1/auth/logout", limiter.Handler(logoutHandler))
	mux.Handle("POST /api/v1/auth/verify-email", limiter.WithProfile("verification", 10)(verifyEmailHandler))
	mux.Handle("POST /api/v1/auth/resend-verification", limiter.WithProfile("verification", 5)(resendVerificationHandler))

	// Catalog Handlers
	if catalogSvc != nil {
		catalogSummaryHandler := NewCatalogSummaryHandler(catalogSvc)
		catalogInstancesHandler := NewCatalogInstancesHandler(catalogSvc)
		mux.Handle("GET /api/v1/catalog/compute/summary", limiter.WithProfile("catalog", 60)(catalogSummaryHandler))
		mux.Handle("GET /api/v1/catalog/compute/instances", limiter.WithProfile("catalog", 60)(catalogInstancesHandler))
	}

	// Global chain: CORS -> AuthContext -> Recovery -> ServeMux
	handler := authMw(mux)
	handler = corsMw.Handler(handler)
	handler = middleware.RecoverMiddleware(handler)

	return otelhttp.NewHandler(handler, "CloudVittaAPI")
}
