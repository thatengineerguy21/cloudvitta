package email

import (
	"context"
	"fmt"
	"net/url"

	"github.com/resend/resend-go/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("internal/email")

// ResendOption defines functional configuration for ResendSender.
type ResendOption func(*ResendSender)

// WithBaseURL configures a custom base URL for the Resend API (useful for testing).
func WithBaseURL(rawURL string) ResendOption {
	return func(s *ResendSender) {
		if parsed, err := url.Parse(rawURL); err == nil {
			s.client.BaseURL = parsed
		}
	}
}

// ResendSender sends emails via the Resend REST API.
type ResendSender struct {
	client *resend.Client
	from   string // e.g. "CloudVitta <noreply@cloudvitta.dev>"
}

// NewResendSender creates a Resend-backed Sender.
// apiKey is the Resend API key. from is the verified sender address.
func NewResendSender(apiKey, from string, opts ...ResendOption) *ResendSender {
	s := &ResendSender{
		client: resend.NewClient(apiKey),
		from:   from,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Send dispatches a transactional email through Resend API.
func (s *ResendSender) Send(ctx context.Context, msg Message) error {
	ctx, span := tracer.Start(ctx, "email.ResendSender.Send",
		trace.WithAttributes(attribute.String("email.to", msg.To)))
	defer span.End()

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{msg.To},
		Subject: msg.Subject,
		Html:    msg.HTML,
		Text:    msg.Text,
	}

	_, err := s.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("resend send: %w", err)
	}
	return nil
}
