package mcp

import (
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Option allows configuring optional observability dependencies for the MCP server.
type Option func(*serverConfig)

type serverConfig struct {
	tracer trace.Tracer
	meter  metric.Meter
}

// WithTracer attaches an OpenTelemetry Tracer to the MCP server.
func WithTracer(tracer trace.Tracer) Option {
	return func(cfg *serverConfig) {
		cfg.tracer = tracer
	}
}

// WithMeter attaches an OpenTelemetry Meter to the MCP server.
func WithMeter(meter metric.Meter) Option {
	return func(cfg *serverConfig) {
		cfg.meter = meter
	}
}

// NewServer initializes the MCP server and registers all 5 CloudVitta comparison and calculation tools.
func NewServer(pricingSvc *service.PricingService, freshnessSvc *service.FreshnessService, opts ...Option) *sdk.Server {
	impl := &sdk.Implementation{
		Name:        "cloudvitta",
		Version:     "1.0.0",
		Title:       "CloudVitta Cloud Pricing Engine",
		Description: "Multi-cloud normalized pricing comparison and workload cost calculation engine.",
	}
	server := sdk.NewServer(impl, nil)

	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	RegisterTools(server, pricingSvc, freshnessSvc, cfg)

	return server
}
