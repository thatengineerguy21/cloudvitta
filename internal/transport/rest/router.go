package rest

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	_ "github.com/thatengineerguy21/CloudVitta/internal/transport/rest/openapi"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/thatengineerguy21/CloudVitta/internal/middleware/ratelimit"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// NewRouter constructs a net/http.ServeMux with all API routes, health probes, metrics, and middlewares wired.
func NewRouter(pricingSvc *service.PricingService, authSvc *service.AuthService, dbPool *pgxpool.Pool, redisClient redis.Cmdable) http.Handler {
	mux := http.NewServeMux()

	// Probes & Metrics (unlimited)
	mux.HandleFunc("/healthz", HandleHealthz)
	mux.HandleFunc("/readyz", HandleReadyz(dbPool, redisClient))
	mux.Handle("/metrics", promhttp.Handler())

	// OpenAPI Documentation (unlimited)
	mux.Handle("/docs/", httpSwagger.WrapHandler)

	// Setup standard middlewares
	limiter := ratelimit.NewRateLimiter(redisClient, 60) // 60 req/min/IP
	computeHandler := NewComputeHandler(pricingSvc)
	storageHandler := NewStorageHandler(pricingSvc)
	networkHandler := NewNetworkHandler(pricingSvc)
	calculateHandler := NewCalculateHandler(pricingSvc)

	// Auth Handlers
	signupHandler := NewSignupHandler(authSvc)
	loginHandler := NewLoginHandler(authSvc)

	mux.Handle("GET /api/v1/prices/compute", limiter.Handler(computeHandler))
	mux.Handle("GET /api/v1/prices/storage", limiter.Handler(storageHandler))
	mux.Handle("GET /api/v1/prices/network", limiter.Handler(networkHandler))
	mux.Handle("POST /api/v1/calculate", limiter.Handler(calculateHandler))
	mux.Handle("POST /api/v1/auth/signup", limiter.Handler(signupHandler))
	mux.Handle("POST /api/v1/auth/login", limiter.WithProfile("login", 10)(loginHandler))

	// Wrap with recovery middleware and OpenTelemetry HTTP instrumentation
	recoveredHandler := middleware.RecoverMiddleware(mux)
	return otelhttp.NewHandler(recoveredHandler, "CloudVittaAPI")
}
