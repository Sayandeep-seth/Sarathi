# Security Policy

## Reporting a vulnerability
Email the maintainers privately (address to be set before first deployment: `security@<your-domain>`).
Do not open public issues for vulnerabilities. Expect acknowledgement within 3 business days.

## Controls in place
* **Authentication**: phone OTP (HMAC stored in Redis, 5-minute TTL, 5 attempts, single-use) → 15-minute HS256 access token + rotating 30-day refresh token. Only the SHA-256 of a refresh token is stored; reuse of a rotated token revokes all of that user's sessions. Browsers hold the refresh token in an HttpOnly, SameSite=Strict, Secure cookie scoped to `/api/v1/auth`; the access token lives in memory only.
* **Authorization**: role checks (passenger / driver / admin) on every route; trips are readable only by their passenger, their driver and admins (others get 404).
* **Rate limiting** (Redis): per IP on everything; per phone on OTP; per user on booking, purchases and location; per trip on boarding-code attempts. Sensitive limits fail closed.
* **Input handling**: 1 MiB body limit, single JSON object, unknown fields rejected, server-side pricing, parameterised SQL throughout.
* **Integrity**: append-only quota ledger (database trigger), idempotent usage reports and bookings, row locks and partial unique indexes against double booking and overdraw.
* **Transport and browser**: strict CSP, `frame-ancestors 'none'`, nosniff, exact-origin CORS (wildcard rejected in production), WebSocket origin check and single-use tickets instead of tokens in URLs.
* **Runtime**: non-root, read-only root filesystems, all capabilities dropped, distroless API image, restricted Pod Security, default-deny NetworkPolicies, no service-account tokens mounted.
* **Configuration**: production refuses to start with the log OTP sender, the mock payment provider, short secrets or wildcard CORS. Secrets are never in the repository (`.env` is git-ignored).
* **Logging**: no headers, query strings or bodies; phone numbers masked.

## Not yet in place
Per-device gateway identity (one shared token today), payment webhook verification (no provider yet), secret rotation with overlap, container image scanning and signing, audit log for admin actions, account deletion, WAF/bot protection, and a written threat model. See `docs/operations/known-limitations.md`.
