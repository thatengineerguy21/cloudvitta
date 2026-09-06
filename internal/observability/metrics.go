package observability

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// CacheMetrics records cache hits and misses.
type CacheMetrics struct {
	counter metric.Int64Counter
}

// NewCacheMetrics initializes the cache_requests_total counter metric.
func NewCacheMetrics(meter metric.Meter) (*CacheMetrics, error) {
	if meter == nil {
		return &CacheMetrics{}, nil
	}

	counter, err := meter.Int64Counter(
		"cache_requests_total",
		metric.WithDescription("Total number of cache requests partitioned by result (hit or miss)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: create cache_requests_total counter: %w", err)
	}

	return &CacheMetrics{counter: counter}, nil
}

// RecordHit increments the cache_requests_total counter with result="hit".
func (c *CacheMetrics) RecordHit(ctx context.Context) {
	c.record(ctx, "hit")
}

// RecordMiss increments the cache_requests_total counter with result="miss".
func (c *CacheMetrics) RecordMiss(ctx context.Context) {
	c.record(ctx, "miss")
}

func (c *CacheMetrics) record(ctx context.Context, result string) {
	if c == nil || c.counter == nil {
		return
	}
	c.counter.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

// PoolStatSource provides database connection pool statistics.
type PoolStatSource interface {
	Stat() *pgxpool.Stat
}

// RegisterPoolStatsCollector registers asynchronous Prometheus gauge metrics
// that observe pgxpool connection pool statistics.
func RegisterPoolStatsCollector(pool *pgxpool.Pool, meter metric.Meter) (metric.Registration, error) {
	if pool == nil {
		return nil, nil
	}
	return RegisterPoolStatsCollectorSource(pool, meter)
}

// RegisterPoolStatsCollectorSource registers asynchronous Prometheus gauge metrics
// from any PoolStatSource.
func RegisterPoolStatsCollectorSource(source PoolStatSource, meter metric.Meter) (metric.Registration, error) {
	if source == nil || meter == nil {
		return nil, nil
	}

	activeGauge, err := meter.Int64ObservableGauge(
		"db.pool.active_connections",
		metric.WithDescription("Number of active (acquired) connections in the database pool"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: create db.pool.active_connections gauge: %w", err)
	}

	idleGauge, err := meter.Int64ObservableGauge(
		"db.pool.idle_connections",
		metric.WithDescription("Number of idle connections in the database pool"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: create db.pool.idle_connections gauge: %w", err)
	}

	totalGauge, err := meter.Int64ObservableGauge(
		"db.pool.total_connections",
		metric.WithDescription("Total number of connections in the database pool"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: create db.pool.total_connections gauge: %w", err)
	}

	maxGauge, err := meter.Int64ObservableGauge(
		"db.pool.max_connections",
		metric.WithDescription("Maximum configured connections in the database pool"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: create db.pool.max_connections gauge: %w", err)
	}

	reg, err := meter.RegisterCallback(func(_ context.Context, obs metric.Observer) error {
		stat := source.Stat()
		if stat != nil {
			obs.ObserveInt64(activeGauge, int64(stat.AcquiredConns()))
			obs.ObserveInt64(idleGauge, int64(stat.IdleConns()))
			obs.ObserveInt64(totalGauge, int64(stat.TotalConns()))
			obs.ObserveInt64(maxGauge, int64(stat.MaxConns()))
		}
		return nil
	}, activeGauge, idleGauge, totalGauge, maxGauge)
	if err != nil {
		return nil, fmt.Errorf("observability: register pool stats callback: %w", err)
	}

	return reg, nil
}
