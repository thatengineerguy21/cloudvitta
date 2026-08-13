package observability

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
)

// MultiHandler fans out slog records to multiple slog Handlers (e.g. stdout JSON handler + OTel log handler).
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler creates a new MultiHandler combining the given slog handlers.
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, record slog.Record) error {
	// Inject trace_id and span_id if an active span exists in context
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}

	for _, h := range m.handlers {
		if h.Enabled(ctx, record.Level) {
			_ = h.Handle(ctx, record.Clone())
		}
	}
	return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: newHandlers}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: newHandlers}
}

// SetupLogger builds a multi-handler slog.Logger that outputs JSON to stdout and sends logs to OTel.
func SetupLogger(serviceName string, level slog.Level, lp *sdklog.LoggerProvider, out io.Writer) *slog.Logger {
	if serviceName == "" {
		serviceName = "cloudvitta"
	}
	if out == nil {
		out = os.Stdout
	}

	stdoutHandler := slog.NewJSONHandler(out, &slog.HandlerOptions{Level: level})
	otelHandler := otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp))

	multi := NewMultiHandler(stdoutHandler, otelHandler)
	return slog.New(multi)
}
