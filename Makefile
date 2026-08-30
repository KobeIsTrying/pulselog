COMPOSE := docker compose -f infrastructure/docker-compose.yml
COMPOSE_PROD := docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml

.PHONY: up up-app up-obs up-prod down logs test run-ingest run-processor run-query run-dashboard fmt vet load-setup load-test load-query load-mixed images

up:
	$(COMPOSE) up -d

up-app:
	$(COMPOSE) --profile app up -d --build

up-obs:
	$(COMPOSE) --profile obs up -d

up-prod:
	$(COMPOSE_PROD) up -d --build

images:
	docker build -f services/ingestion-api/Dockerfile -t pulselog/ingestion-api:local .
	docker build -f services/log-processor/Dockerfile -t pulselog/log-processor:local .
	docker build -f services/query-api/Dockerfile -t pulselog/query-api:local .
	docker build -f cmd/migrate/Dockerfile -t pulselog/migrate:local .
	docker build -f apps/dashboard/Dockerfile -t pulselog/dashboard:local apps/dashboard

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=100

test:
	go test ./internal/... ./services/...

run-ingest:
	go run ./services/ingestion-api

run-processor:
	go run ./services/log-processor

run-query:
	go run ./services/query-api

run-dashboard:
	npm --prefix apps/dashboard run dev

fmt:
	gofmt -w ./internal ./services

vet:
	go vet ./...

load-setup:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/load/setup.ps1

load-test:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/load/ingest.ps1 -Rate 100 -Duration 20s

load-query:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/load/query.ps1

load-mixed:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/load/mixed.ps1
