package email

import (
	"context"
	"log/slog"
)

// NoopSender logs emails instead of sending them.
// Use in development and test environments.
type NoopSender struct{}

// Send logs the outbound message without dispatching to an external provider.
func (n *NoopSender) Send(ctx context.Context, msg Message) error {
	slog.InfoContext(ctx, "noop email sender: would send email",
		"to", msg.To,
		"subject", msg.Subject,
	)
	return nil
}
