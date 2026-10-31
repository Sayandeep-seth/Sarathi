# SARATHI — Smart Mobility Connectivity Platform

Cab connectivity and passenger Wi-Fi management: QR-verified trips, a byte-exact quota ledger,
gateway-enforced access, and (later) payments, admin tooling and advisory analytics.

> **Status: Milestone 1 (Foundation) of 8.** Most product features do **not** exist yet.
> See [What works today](#what-works-today) and [docs/operations/known-limitations.md](docs/operations/known-limitations.md).

## What works today

| Component | State | Verified how |
|---|---|---|
| Go API: config, structured logging, request IDs, body/time limits, CORS allow-list, `/healthz`, `/readyz`, `/api/v1/version`, graceful shutdown | Implemented | `go vet`, 17 unit tests, live smoke test (binary run against real PostgreSQL + Redis) |
| SQL migration runner (advisory-locked, transactional, idempotent) + baseline migration | Implemented | 3 tests against a real PostgreSQL 16 |
| Python AI service: `/health`, fail-closed `/internal/v1/models/status` | Implemented (detectors are *planned*, not built) | `ruff`, 7 pytest tests, live smoke test |
| Flutter passenger app: onboarding + service-reachability home screen | Source written | **Not compiled or tested** (no Flutter SDK in authoring environment) |
| Docker images, `compose.yaml`, GitHub Actions CI | Source written | **Never executed** (no Docker / runner in authoring environment) |
| Auth, cabs, trips, QR, Wi-Fi sessions, quota, payments, gateway, admin dashboard, driver app, AI analytics | **Not started** | — scheduled in Milestones 2–8 |

Nothing in this repository enforces quota, verifies payments or authenticates users yet.

## Layout

```
backend/    Go modular monolith (cmd/api, internal/*, migrations/)
ai/         FastAPI advisory analytics service (never authoritative)
apps/       Flutter apps (passenger_app now; driver_app in a later milestone)
docs/       Architecture, security, operations
infra/      Dockerfiles
scripts/    Developer scripts
```
Directories for gateway/, admin/web and driver_app are intentionally absent until they contain real code.

## Quick start (Docker)

```
cp .env.example .env        # edit the placeholder secrets
make up                     # API on 127.0.0.1:8080, AI on 127.0.0.1:8090
curl localhost:8080/readyz
```
PostgreSQL and Redis publish no host ports. *(Compose file is unverified — see limitations.)*

## Quick start (no Docker)

Requires Go 1.27+, PostgreSQL 16, Redis 7, Python 3.12. Create a database and role, then:

```
export SARATHI_DATABASE_URL='postgres://USER:PASS@127.0.0.1:5432/DB?sslmode=disable'
export SARATHI_REDIS_ADDR=127.0.0.1:6379
make migrate
cd backend && go run ./cmd/api
```

## Tests

```
make test                                  # Go + Python
export SARATHI_TEST_DATABASE_URL=...       # a throwaway DB; enables PostgreSQL migration tests
cd apps/passenger_app && flutter create --org in.sarathi --project-name sarathi_passenger . \
  && flutter pub get && flutter test       # generates platform folders; first run only
```
Without `SARATHI_TEST_DATABASE_URL` the database tests are **skipped**, not passed.

## Product terms used consistently

* Quota is stored in **bytes**. **GiB** = 2³⁰ bytes; GB (10⁹) is never used for quota.
* Money is stored as integer minor units (paise).
* The mobile app is never a trusted source of usage or payment state.

## More
[Architecture](docs/architecture/overview.md) · [Known limitations](docs/operations/known-limitations.md) · [Security policy](SECURITY.md) · [Contributing](CONTRIBUTING.md)
