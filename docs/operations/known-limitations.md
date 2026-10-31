# Known limitations and unverified items (Milestone 1)

## Unverified — written but never run
1. **Flutter app** (`apps/passenger_app`): the authoring environment could not install Flutter (SDK hosts are unreachable). The code has not been compiled, analyzed or tested. Dependency versions were chosen from memory and **no `pubspec.lock` exists**. Platform folders (android/ios) are not generated; run `flutter create` as shown in the README.
2. **Docker** (`infra/docker/*`, `compose.yaml`) and **GitHub Actions** (`.github/workflows/ci.yml`): never executed. Expect to fix small issues on first run.

## Environment-driven deviations
3. **`go.mod` replace directives**: the sandbox could not reach `proxy.golang.org`, `golang.org` or `gopkg.in`, so indirect modules are resolved from GitHub mirrors. `go.sum` reflects that. On a normal network, delete the `replace` block and run `go mod tidy`.
4. **Race detector not run**: the Go toolchain used had no cgo, so `go test -race` could not execute locally. CI runs it; that result is unknown.
5. Go was installed from conda-forge (1.27.3) rather than go.dev.

## Not implemented yet (by design, scheduled)
Authentication/OTP, users, cabs, trips, QR, Wi-Fi sessions, quota ledger, payments, gateway protocol and agent, admin dashboard, driver app, AI analytics logic, monitoring stack, backup/restore scripts, runbooks, OpenAPI coverage beyond Milestone 1 endpoints, load tests.

## Hardening items already known
* Migration checksum verification of applied files is not enforced.
* Redis probe is a minimal RESP client for health checks only.
* No rate limiting, authentication or TLS termination yet (no sensitive endpoints exist yet).
* No legal/privacy review has been performed (DPDP Act, payment-provider obligations). Nothing here claims compliance.
