package auth_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
)

var testCookieSecret = []byte("super-secret-cookie-signing-key-32b!")

func TestMintAndVerifyAnonCookie_Success(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	cookie, err := auth.MintAnonCookie(testCookieSecret, now, true)
	if err != nil {
		t.Fatalf("unexpected mint error: %v", err)
	}

	if cookie.Name != auth.AnonCookieName {
		t.Errorf("expected cookie name %s, got %s", auth.AnonCookieName, cookie.Name)
	}
	if cookie.Path != "/" {
		t.Errorf("expected cookie path '/', got %s", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Errorf("expected HttpOnly true")
	}
	if !cookie.Secure {
		t.Errorf("expected Secure true")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite Lax")
	}
	if cookie.MaxAge <= 0 {
		t.Errorf("expected positive MaxAge, got %d", cookie.MaxAge)
	}

	// Verify the minted cookie
	anonID, err := auth.VerifyAnonCookie(cookie.Value, testCookieSecret, now)
	if err != nil {
		t.Fatalf("unexpected verify error: %v", err)
	}

	if _, err := uuid.Parse(anonID); err != nil {
		t.Errorf("expected valid UUID for anonID, got %s (err: %v)", anonID, err)
	}

	// Verify 10 minutes later within TTL
	laterTime := now.Add(10 * time.Minute)
	anonIDLater, err := auth.VerifyAnonCookie(cookie.Value, testCookieSecret, laterTime)
	if err != nil {
		t.Fatalf("unexpected verify error at later time: %v", err)
	}
	if anonIDLater != anonID {
		t.Errorf("expected same anonID %s, got %s", anonID, anonIDLater)
	}
}

func TestVerifyAnonCookie_TamperedSignature(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	cookie, err := auth.MintAnonCookie(testCookieSecret, now, false)
	if err != nil {
		t.Fatalf("unexpected mint error: %v", err)
	}

	// Tamper with value by modifying payload
	tamperedValue := "00000000-0000-0000-0000-000000000000" + cookie.Value[36:]
	_, err = auth.VerifyAnonCookie(tamperedValue, testCookieSecret, now)
	if err == nil {
		t.Fatal("expected error for tampered payload, got nil")
	}

	// Tamper with signature
	sigTampered := cookie.Value[:len(cookie.Value)-4] + "dead"
	_, err = auth.VerifyAnonCookie(sigTampered, testCookieSecret, now)
	if err == nil {
		t.Fatal("expected error for tampered signature, got nil")
	}

	// Verify with wrong secret
	wrongSecret := []byte("wrong-secret-key-32-bytes-length!!")
	_, err = auth.VerifyAnonCookie(cookie.Value, wrongSecret, now)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestVerifyAnonCookie_Expired(t *testing.T) {
	mintTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cookie, err := auth.MintAnonCookie(testCookieSecret, mintTime, false)
	if err != nil {
		t.Fatalf("unexpected mint error: %v", err)
	}

	// Verify 91 days later (> 90 days TTL)
	verifyTime := mintTime.Add(91 * 24 * time.Hour)
	_, err = auth.VerifyAnonCookie(cookie.Value, testCookieSecret, verifyTime)
	if err == nil {
		t.Fatal("expected error for expired cookie, got nil")
	}

	// Verify future timestamp (clock skew > 5 min)
	farPastVerifyTime := mintTime.Add(-10 * time.Minute)
	_, err = auth.VerifyAnonCookie(cookie.Value, testCookieSecret, farPastVerifyTime)
	if err == nil {
		t.Fatal("expected error for future cookie timestamp, got nil")
	}
}

func TestVerifyAnonCookie_InvalidFormat(t *testing.T) {
	now := time.Now().UTC()
	testCases := []struct {
		name  string
		value string
	}{
		{"empty string", ""},
		{"single part", "invalidcookievalue"},
		{"two parts", "id.timestamp"},
		{"four parts", "id.timestamp.sig.extra"},
		{"invalid uuid", "not-a-uuid.1755345600.abcdef"},
		{"invalid timestamp", fmt.Sprintf("%s.not-a-ts.abcdef", uuid.NewString())},
		{"invalid hex signature", fmt.Sprintf("%s.1755345600.zzzzzz", uuid.NewString())},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := auth.VerifyAnonCookie(tc.value, testCookieSecret, now)
			if err == nil {
				t.Errorf("expected error for test case %q, got nil", tc.name)
			}
		})
	}
}

func TestMintAnonCookie_SecretValidation(t *testing.T) {
	now := time.Now().UTC()
	_, err := auth.MintAnonCookie([]byte("short-key"), now, false)
	if err == nil {
		t.Fatal("expected error for short secret, got nil")
	}
}
