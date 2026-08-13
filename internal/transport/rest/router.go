package rest

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// NewRouter constructs a net/http.ServeMux with all API routes, health probes, and middlewares wired.
func NewRouter(pricingSvc *service.PricingService, dbPool *pgxpool.Pool, redisClient redis.Cmdable) http.Handler {
	mux := http.NewServeMux()

	// Probes (unlimited)
	mux.HandleFunc("GET /healthz", HandleHealthz)
	mux.HandleFunc("GET /readyz", HandleReadyz(dbPool, redisClient))

	// OpenAPI Documentation (unlimited)
	mux.Handle("GET /docs/", httpSwagger.WrapHandler)

	// API Endpoints (rate limited)
	rateLimiter := middleware.NewRateLimiter(redisClient, 60)
	computeHandler := NewComputeHandler(pricingSvc)

	mux.Handle("GET /api/v1/prices/compute", rateLimiter.Handler(computeHandler))

	return mux
}
