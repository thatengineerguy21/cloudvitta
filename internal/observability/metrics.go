package observability

import (
	"context"
	"fmt"

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
	if c == nil || c.counter == nil {
		return
	}
	c.counter.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "hit")))
}

// RecordMiss increments the cache_requests_total counter with result="miss".
func (c *CacheMetrics) RecordMiss(ctx context.Context) {
	if c == nil || c.counter == nil {
		return
	}
	c.counter.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "miss")))
}
