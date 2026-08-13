package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HandleHealthz returns 200 OK for liveness checks.
func HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","service":"cloudvitta-api"}`))
}

// HandleReadyz creates a readiness check handler verifying Postgres and Redis reachability.
func HandleReadyz(dbPool *pgxpool.Pool, redisClient redis.Cmdable) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		dbStatus := "ok"
		dbOK := true
		if dbPool == nil {
			dbStatus = "nil database pool"
			dbOK = false
		} else if err := dbPool.Ping(ctx); err != nil {
			dbStatus = err.Error()
			dbOK = false
		}

		redisStatus := "ok"
		redisOK := true
		if redisClient != nil {
			if err := redisClient.Ping(ctx).Err(); err != nil {
				redisStatus = err.Error()
				redisOK = false
			}
		}

		isReady := dbOK && redisOK
		status := http.StatusOK
		statusText := "ready"
		if !isReady {
			status = http.StatusServiceUnavailable
			statusText = "unavailable"
		}

		resp := map[string]interface{}{
			"status":   statusText,
			"database": dbStatus,
			"redis":    redisStatus,
		}

		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
