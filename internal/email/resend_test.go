package email_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/email"
)

func TestResendSender_Send_Success(t *testing.T) {
	t.Parallel()

	var capturedPath string
	var capturedAuth string
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		_ = json.Unmarshal(bodyBytes, &capturedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "msg_test_123"}`))
	}))
	defer server.Close()

	sender := email.NewResendSender("re_test_key_123", "CloudVitta <noreply@cloudvitta.dev>",
		email.WithBaseURL(server.URL),
	)

	msg := email.Message{
		To:      "user@example.com",
		Subject: "Verify your email",
		HTML:    "<p>Verify link</p>",
		Text:    "Verify link",
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if capturedPath != "/emails" {
		t.Errorf("expected path /emails, got %s", capturedPath)
	}
	if capturedAuth != "Bearer re_test_key_123" {
		t.Errorf("expected auth Bearer re_test_key_123, got %s", capturedAuth)
	}
	if capturedBody["from"] != "CloudVitta <noreply@cloudvitta.dev>" {
		t.Errorf("expected from CloudVitta <noreply@cloudvitta.dev>, got %v", capturedBody["from"])
	}
	if capturedBody["subject"] != "Verify your email" {
		t.Errorf("expected subject 'Verify your email', got %v", capturedBody["subject"])
	}
	if capturedBody["html"] != "<p>Verify link</p>" {
		t.Errorf("expected html '<p>Verify link</p>', got %v", capturedBody["html"])
	}
	if capturedBody["text"] != "Verify link" {
		t.Errorf("expected text 'Verify link', got %v", capturedBody["text"])
	}
}

func TestResendSender_Send_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"statusCode": 401, "message": "API key is invalid", "name": "validation_error"}`))
	}))
	defer server.Close()

	sender := email.NewResendSender("invalid_key", "noreply@cloudvitta.dev",
		email.WithBaseURL(server.URL),
	)

	msg := email.Message{
		To:      "user@example.com",
		Subject: "Test",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error on 401 response, got nil")
	}
}

func TestNoopSender_Send(t *testing.T) {
	t.Parallel()

	var sender email.Sender = &email.NoopSender{}
	err := sender.Send(context.Background(), email.Message{
		To:      "noop@example.com",
		Subject: "Noop subject",
		Text:    "Noop body",
	})
	if err != nil {
		t.Fatalf("expected nil error from NoopSender, got: %v", err)
	}
}
