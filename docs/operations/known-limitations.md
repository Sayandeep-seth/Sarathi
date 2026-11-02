# Known limitations and unverified items (Milestone 1)

## Verified locally
1. **Flutter passenger prototype** (`apps/passenger_app`): `flutter analyze` and widget tests cover demo phone sign-in, route planning, bookings, commute, shopping, Wi-Fi, and narrow layouts. No Android/iOS device build has been verified; platform folders are not generated. Run `flutter create` as shown in the README.

## Unverified — written but never run
1. **Docker** (`infra/docker/*`, `compose.yaml`) and **GitHub Actions** (`.github/workflows/ci.yml`): never executed. Expect to fix small issues on first run.

## Environment-driven deviations
3. **`go.mod` replace directives**: the sandbox could not reach `proxy.golang.org`, `golang.org` or `gopkg.in`, so indirect modules are resolved from GitHub mirrors. `go.sum` reflects that. On a normal network, delete the `replace` block and run `go mod tidy`.
4. **Race detector not run**: the Go toolchain used had no cgo, so `go test -race` could not execute locally. CI runs it; that result is unknown.
5. Go was installed from conda-forge (1.27.3) rather than go.dev.

## Not implemented yet (by design, scheduled)
Production authentication/OTP, users, cabs, dispatch, trip persistence, fare calculation, payment processing, public-transit validation, Wi-Fi sessions, quota ledger, gateway protocol and agent, admin dashboard, driver app, AI analytics logic, monitoring stack, backup/restore scripts, runbooks, OpenAPI coverage beyond Milestone 1 endpoints, load tests.

The passenger app's phone sign-in is a local-only UI demo. It accepts a public fixed test code (`123456`); no SMS is sent, no phone is verified, no account is created, and no credentials or session are persisted or sent to the API. Do not use it to authenticate real users. Production launch requires a backend identity service, an SMS/OTP provider, secure session storage and expiry/refresh, abuse protections, and published legal/privacy documents.

The passenger prototype displays sample routes, estimated vehicle dimensions, station visits/ratings, fares, tour and event inventory, ride rewards, and Wi-Fi usage. These are not live routing, GPS observations, verified vehicle fit advice, partner inventory, or real prices. Its QR is a demo payload and is not a ticket or valid boarding credential. No actual payment, trip, data top-up, or seat reservation is created. Production use requires authoritative data sources, safety validation, operator/fleet agreements, privacy review, and backend integrations.

## Hardening items already known
* Migration checksum verification of applied files is not enforced.
* Redis probe is a minimal RESP client for health checks only.
* No rate limiting, authentication or TLS termination yet (no sensitive endpoints exist yet).
* No legal/privacy review has been performed (DPDP Act, payment-provider obligations). Nothing here claims compliance.
