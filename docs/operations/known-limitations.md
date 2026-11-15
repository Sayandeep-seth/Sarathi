# Known limitations

This is the honest gap list between the repository and a public launch. Nothing here
is hidden behind a flag: if it is listed, it does not exist yet.

## Blocking a real launch

1. **No payment provider.** `SARATHI_PAYMENT_PROVIDER` accepts only `mock` (rejected in
   production) and `none`. In production Wi-Fi plans cannot be bought and trip fares are
   recorded but never collected. Integrating a provider means: order creation, signed
   webhook verification, idempotent settlement into `payments`, refunds and reconciliation.
2. **No SMS provider.** Outside local mode OTPs are POSTed to `SARATHI_OTP_WEBHOOK_URL`
   as `{"to","message"}` with a bearer token. You must run or adapt an endpoint for your
   provider (and complete DLT template registration for India).
3. **The map is not real.** `backend/internal/geo` is an 11×11 grid with stylised place
   positions. Distances, ETAs and fares are consistent with that grid, not with real
   roads. A real launch needs a routing engine and real geocoding.
4. **No driver app and no vehicle gateway.** The driver endpoints (availability,
   location, start with boarding code, complete) and the gateway usage endpoint are
   implemented and tested, but nothing except the simulator and the smoke test calls
   them. Wi-Fi is not actually enforced on any network.
5. **Gateway authentication is one shared token** for all gateways. Per-vehicle
   credentials (mTLS or signed reports) are required before trusting real meters.
6. **No legal or privacy review** (DPDP Act, telecom and payment obligations), no terms
   or privacy policy, no data-retention or deletion workflow beyond MongoDB TTLs.

## Verified, and how

| Area | Evidence |
|---|---|
| Go API | `go vet`, `go test -race` (unit + PostgreSQL-backed store and migration tests), `govulncheck`: 0 reachable vulnerabilities |
| Whole stack | `docker compose up --wait`, then `scripts/smoke.mjs`: 50 checks covering OTP, token rotation and reuse detection, authorization, booking race, idempotency, WebSocket, a simulated trip to completion, ledger arithmetic and MongoDB timelines |
| Web app | Type-check, build, unit tests, and a scripted headless-browser run (sign in, plan, book, ride) with no console errors beyond the expected first-visit 401 |
| Kubernetes manifests | Both overlays render with `kubectl kustomize` and pass `kubeconform -strict` for Kubernetes 1.31 |

## Not verified

* **Kubernetes manifests have never been applied to a cluster.** Expect to adjust for
  your ingress controller, storage class and CNI (NetworkPolicy enforcement needs one
  that supports it). The `preStop.sleep` hook needs Kubernetes 1.30+.
* **Multiple API replicas** have not been exercised together. Cross-replica WebSocket
  fan-out (Redis pub/sub) and the simulator leader lease are implemented but untested
  beyond a single replica.
* **GitHub Actions** workflow has not run on a runner.
* **No load or soak testing.** Limits such as 20 000 WebSocket connections per replica
  and the rate-limit thresholds are reasoned defaults, not measured.
* **Browsers**: only Chromium (Edge) was exercised. Safari and Firefox are untested.
* The Flutter prototype and the AI service were not changed or re-run in this round.

## Design trade-offs to know about

* **WebSocket events are not replayed.** Clients resync over HTTP after every
  reconnect; a client that never reconnects could show stale state.
* **Role changes take up to 15 minutes** (the access-token lifetime) to take effect,
  because roles are read from the token.
* **Rate limits are fixed windows** in Redis and fail open for general traffic when
  Redis is unreachable (they fail closed for OTP, purchases and boarding codes).
* **One secret, three uses**: `SARATHI_JWT_SECRET` signs access tokens and keys the OTP
  and boarding-code HMACs. Rotating it signs everyone out and changes pending boarding
  codes. There is no key-rotation overlap window.
* **Telemetry is lossy by design**: MongoDB writes are buffered (4096 records) and
  dropped under back-pressure rather than slowing trips.
* **Live cab positions are visible to every signed-in user**, including cabs carrying
  other passengers. Acceptable for a demo fleet; revisit for real riders.
* **Compose and the in-cluster datastores are single-node with no backups.** Use
  managed PostgreSQL/Redis/MongoDB with backups and TLS for anything that matters.
* **Migrations** are forward-only and checksum verification of applied files is not
  enforced. Never edit an applied migration.
* **No tracing or alerting.** Prometheus metrics (`/metrics`) and structured JSON logs
  exist; dashboards, alerts and runbooks do not.
* **No admin UI.** Admin and driver functions are API-only.
