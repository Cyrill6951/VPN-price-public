.PHONY: help up down logs build run tidy lint test migrate-up migrate-down

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

up: ## Start full stack via docker compose
	docker compose -f deploy/docker-compose.yml up -d --build

down: ## Stop stack
	docker compose -f deploy/docker-compose.yml down

logs: ## Tail app logs
	docker compose -f deploy/docker-compose.yml logs -f api

build: ## Build api binary in Docker
	docker compose -f deploy/docker-compose.yml build api

tidy: ## go mod tidy (requires local Go)
	go mod tidy

lint: ## Run golangci-lint
	golangci-lint run ./...

test: ## Run tests
	go test ./... -race -count=1

seed: ## Seed dev catalogue (countries, plans, dev server)
	docker run --rm --network vpn-platform_default -v "$(CURDIR)":/src -w /src \
		-e DATABASE_URL=postgres://vpn:vpn@postgres:5432/vpn?sslmode=disable \
		golang:1.23-alpine go run ./cmd/seed

build-agent: ## Build the node agent binary (linux/amd64) into bin/agent
	docker run --rm -v "$(CURDIR)":/src -w /src -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 \
		golang:1.23-alpine go build -o bin/agent ./cmd/agent

migrate-up: ## Apply DB migrations (goose in container)
	docker compose -f deploy/docker-compose.yml run --rm migrate up

migrate-down: ## Rollback last migration
	docker compose -f deploy/docker-compose.yml run --rm migrate down
