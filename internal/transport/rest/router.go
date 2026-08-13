package rest

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	_ "github.com/thatengineerguy21/CloudVitta/internal/transport/rest/openapi"

	"github.com/thatengineerguy21/CloudVitta/internal/middleware/ratelimit"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

// NewRouter constructs a net/http.ServeMux with all API routes, health probes, and middlewares wired.
func NewRouter(pricingSvc *service.PricingService, dbPool *pgxpool.Pool, redisClient redis.Cmdable) http.Handler {
	mux := http.NewServeMux()

	// Probes (unlimited)
	mux.HandleFunc("GET /healthz", HandleHealthz)
	mux.HandleFunc("GET /readyz", HandleReadyz(dbPool, redisClient))

	// OpenAPI Documentation (unlimited)
	mux.Handle("GET /docs/", httpSwagger.WrapHandler)

	// Setup standard middlewares
	limiter := ratelimit.NewRateLimiter(redisClient, 60) // 60 req/min/IP
	computeHandler := NewComputeHandler(pricingSvc)

	mux.Handle("GET /api/v1/prices/compute", limiter.Handler(computeHandler))

	return mux
}
