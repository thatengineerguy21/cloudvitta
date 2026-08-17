# 21. Refresh Token Families for Theft Containment

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta handles authentication using short-lived stateless JWTs and stateful refresh tokens. Refresh tokens are rotated periodically. To protect against token theft, if a previously used (revoked) refresh token is presented again without a matching idempotency key (ADR-007), the system must immediately revoke the entire chain of tokens to lock out the attacker.

We needed to decide how to group these tokens in the database to enable fast revocation without walking a linked list of `replaced_by` rows, and how to define the blast radius of a theft event (e.g., whether a stolen phone token should log the user out of their laptop as well).

## Decision

We will map refresh token chains using a **`family_id`** UUID generated uniquely per physical login event.

1. **Unique per Session**: Each time a user explicitly logs in, a brand new `family_id` is generated. Separate devices (phone vs. laptop) receive different family IDs.
2. **Targeted Revocation**: Upon detecting theft on a specific device, the system executes `UPDATE refresh_tokens SET revoked_at = now() WHERE family_id = $1`. This contains the blast radius strictly to the compromised device.
3. **Global Revocation**: To support a "Revoke All Sessions" feature, the system executes `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1`. This leverages the same mechanism with a wider scope, requiring only an index on `user_id`.

## Consequences

### Positive
- **O(1) Revocation**: Entire token chains can be revoked with a single bulk update query, avoiding recursive or multi-step database lookups.
- **Proper Containment**: A compromised mobile connection won't needlessly terminate a secure desktop session.
- **Simple Extensibility**: The exact same table structure natively supports both targeted device revocation and global account resets.

### Negative
- Requires maintaining two indexes on the `refresh_tokens` table (one for `family_id` and one for `user_id`) to ensure fast updates.

## Alternatives Considered

### Alternative 1: Single Reusable Refresh Token Without Rotation
Rejected. Reusable refresh tokens allow an attacker who obtains a token to maintain indefinite unauthorized session access without detection.

### Alternative 2: Linked-List Token Chains Without Family Identifier
Rejected. Tracking token chains solely through `replaced_by` foreign keys requires recursive CTE queries or iterative lookups to revoke token trees, slowing down theft mitigation and increasing database lock contention.
