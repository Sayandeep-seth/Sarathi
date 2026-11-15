# SARATHI developer commands. Docker is the only hard requirement for `up` and `smoke`;
# the test targets expect Go 1.27+, Node 22+ and Python 3.12 on PATH.
.PHONY: up down logs smoke test test-go test-web test-ai lint fmt migrate web-dev k8s-render k8s-local

up:
	@test -f .env || (echo "Create .env from .env.example first" && exit 1)
	docker compose up --build -d --wait
	@echo "Web: http://127.0.0.1:$${SARATHI_WEB_PORT:-3000}"

down:
	docker compose down

logs:
	docker compose logs -f --tail 100 api web

# End-to-end check against the running local stack (see scripts/smoke.mjs).
smoke:
	node scripts/smoke.mjs

test: test-go test-web test-ai

# Set SARATHI_TEST_DATABASE_URL to include the PostgreSQL-backed tests (skipped otherwise).
test-go:
	cd backend && go vet ./... && go test -race -count=1 ./...

test-web:
	cd web && npm ci && npm run build && npm test

test-ai:
	cd ai && ruff check . && ruff format --check . && python -m pytest -q

lint:
	cd backend && gofmt -l . | (! grep .) && go vet ./...
	cd ai && ruff check .

fmt:
	cd backend && gofmt -w .
	cd ai && ruff format .

migrate:
	cd backend && go run ./cmd/api migrate

# Vite dev server with hot reload; proxies /api to the API on SARATHI_API_URL.
web-dev:
	cd web && npm install && npm run dev

k8s-render:
	kubectl kustomize infra/k8s/overlays/local
	kubectl kustomize infra/k8s/overlays/production

k8s-local:
	docker compose build
	kubectl apply -k infra/k8s/overlays/local
