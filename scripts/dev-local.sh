#!/usr/bin/env bash
# Run the API against already-running local PostgreSQL and Redis (no Docker).
set -euo pipefail
: "${SARATHI_DATABASE_URL:?export SARATHI_DATABASE_URL first}"
export SARATHI_REDIS_ADDR="${SARATHI_REDIS_ADDR:-127.0.0.1:6379}"
cd "$(dirname "$0")/../backend"
go run ./cmd/api migrate
exec go run ./cmd/api
