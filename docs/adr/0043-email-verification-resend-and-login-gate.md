# 43. Email Verification with Resend and Strict Login Gate

Date: 2026-09-06

## Status

Accepted

## Context

CloudVitta permits users to create accounts with an email address and a password. Without email verification, users can register with invalid or unowned email addresses. Malicious actors can also register fake accounts.

We need to verify user email ownership during account registration. We must also block unverified users from authenticating into the platform. At the same time, we must prevent email enumeration attacks and deny token flooding.

## Decision

We implement asynchronous email verification with Resend and a database-backed verification token lifecycle:

1. **Email Verification Status**:
   - Add column `email_verified BOOLEAN NOT NULL DEFAULT true` to table `users`.
   - Existing user accounts retain `email_verified = true` through the default value.
   - New sign-ups through `POST /api/v1/auth/signup` insert records with `email_verified = false`.

2. **Single-Use Verification Tokens**:
   - Create table `verification_tokens` storing SHA-256 token hashes, expiration timestamps (30 minutes), and consumption timestamps.
   - Raw tokens are 32 cryptographically secure random bytes formatted in hex.
   - Token consumption executes atomically: the system updates `consumed_at` and sets `users.email_verified = true`.

3. **Strict Login Gate**:
   - During `POST /api/v1/auth/login`, the system first validates the password using timing-safe comparison.
   - If the password is correct but `email_verified == false`, the service returns `ErrEmailNotVerified`.
   - The REST layer translates this error to HTTP 403 Forbidden with problem type `https://cloudvitta.dev/errors/email-not-verified`.

4. **Email Delivery and Anti-Enumeration Resend**:
   - In production, the system delivers transactional verification emails through Resend (`internal/email/resend.go`). In tests, it uses `NoopSender`.
   - The endpoint `POST /api/v1/auth/resend-verification` enforces a limit of 3 active tokens per user.
   - The endpoint always returns HTTP 202 Accepted, whether the account exists, is verified, or is unknown. This prevents account enumeration.

5. **Rate Limiting**:
   - `POST /api/v1/auth/verify-email`: 10 requests per minute per IP.
   - `POST /api/v1/auth/resend-verification`: 5 requests per minute per IP.

## Consequences

### Positive
- **Guaranteed Account Authenticity**: Only users who control the target email address can authenticate and create sessions.
- **Enumeration Resistance**: Uniform HTTP 202 responses prevent attackers from discovering registered email addresses.
- **Timing Safety**: Passwords check before email verification status checks, which prevents timing leaks.
- **Database Hygiene**: Single-use tokens expire in 30 minutes, and active tokens are limited to 3 per user.

### Negative
- Users must complete an extra email verification step before they can use the application.
- Operational dependency on an external transactional email provider (Resend).

## Alternatives Considered

### Alternative 1: Synchronous Automatic Login on Signup
Rejected. Permitting unverified users to start authenticated sessions allows unauthorized usage and leaves abandoned accounts without verified ownership.

### Alternative 2: Disclosing Account Nonexistence on Resend
Rejected. Returning HTTP 404 on `resend-verification` allows attackers to verify whether an email address exists in the system.
