# Contributing

* Work in small milestones; keep existing tests green (`make test`).
* Never claim a feature works without a test or run that demonstrates it; record gaps in `docs/operations/known-limitations.md`.
* Quota is bytes (GiB, never GB). Money is integer minor units. Timestamps are UTC.
* Quota and payment changes must run in a database transaction and be idempotent.
* Do not commit secrets, certificates or `.env`.
* Format: `gofmt`, `ruff format`, `dart format`.
