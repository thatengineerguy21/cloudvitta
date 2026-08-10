# 20. Prevention of Timing Attacks via Dummy Hashes

Date: 2026-08-06

## Status

Accepted

## Context

To protect user accounts from enumeration, it is standard practice to return an identical response ("Invalid credentials") whether an email address exists or the password was simply wrong. However, if the server only runs the computationally expensive `bcrypt` hash when the email *exists*, an attacker can measure the response time to accurately determine if an email is registered (a timing attack).

To prevent this, the architecture mandates running a dummy `bcrypt` hash even when the user is not found. However, this introduces a theoretical Denial of Service (DoS) vulnerability: an attacker could flood the endpoint with fake emails, forcing the server to max out its CPU running dummy hashes.

## Decision

We will strictly enforce the **Dummy Hash Mitigation**, explicitly accepting and separately mitigating the DoS risk.

1. **Dummy Hash**: The login endpoint must run a `bcrypt` verify against a fixed dummy hash if the email is not found.
2. **Identical Cost Factor**: The dummy hash must be configured with the exact same work factor as real password hashes to ensure the timing signal remains completely indistinguishable.
3. **Layered Defense**: The DoS threat is mitigated by placing the API's strict Tiered Rate Limiter (ADR-009) *in front* of the bcrypt call, specifically tightening the ceiling for the `/login` route.

## Consequences

### Positive
- Completely eliminates account enumeration via timing attacks.
- The DoS risk is bounded by the rate limiter. Once an IP or anonymous session hits the login-attempt ceiling, requests are rejected with HTTP 429 before the handler ever reaches the CPU-intensive bcrypt logic.

### Negative
- Removing the dummy hash would fix a bounded problem (DoS, capped by rate limits) by reopening an unmitigated problem (account enumeration). Layering the two controls ensures each solves its respective threat without trading one for the other.
