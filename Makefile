# SARATHI developer commands. Go is expected on PATH (go 1.27+).
.PHONY: test test-go test-ai lint fmt up down docker-build migrate \
	redis-up redis-down redis-logs redis-cli redis-ping redis-info

test: test-go test-ai

# Set SARATHI_TEST_DATABASE_URL to include the PostgreSQL migration tests.
test-go:
	cd backend && go vet ./... && go test -count=1 ./...

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

up:
	@test -f .env || (echo "Create .env from .env.example first" && exit 1)
	docker compose up --build -d

down:
	docker compose down

docker-build:
	docker compose build


redis-up:
	docker compose up -d redis

redis-down:
	docker compose stop redis

redis-logs:
	docker logs -f sarathi-redis

redis-cli:
	docker exec -it sarathi-redis redis-cli -a $(REDIS_PASSWORD)

redis-ping:
	docker exec sarathi-redis redis-cli -a $(REDIS_PASSWORD) ping

redis-info:
	docker exec sarathi-redis redis-cli -a $(REDIS_PASSWORD) INFO
