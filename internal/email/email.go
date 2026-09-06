package email

import "context"

// Message holds the data required to send a single transactional email.
type Message struct {
	To      string // recipient email address
	Subject string
	HTML    string // rendered HTML body
	Text    string // plaintext fallback
}

// Sender sends transactional emails. Implementations must be safe for
// concurrent use and must propagate context for tracing/cancellation.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
