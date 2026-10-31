# Security Policy

## Reporting a vulnerability
Email the maintainers privately (address to be set before first deployment: `security@<your-domain>`).
Do not open public issues for vulnerabilities. Expect acknowledgement within 3 business days.

## Current posture (Milestone 1)
Only unauthenticated health/version endpoints exist. Implemented: log redaction by omission, request-size and time limits, CORS allow-list (wildcard rejected in production), fail-closed service authentication for the AI service, no secrets in the repository (`.env` is git-ignored; `.env.example` holds placeholders).

## Not yet in place
Authentication, authorization, rate limiting, webhook verification, gateway device identity, TLS termination, dependency/container scanning, and a threat model document. These are required before any non-development deployment. See `docs/operations/known-limitations.md`.
