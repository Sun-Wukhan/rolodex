.DEFAULT_GOAL := help
SHELL := /bin/bash

TEST_DATABASE_URL ?= postgres://postgres:test@localhost:55432/rolodex_test?sslmode=disable

.PHONY: help env up down logs test test-pg cover lint fmt run-api run-mock seed-local web

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

env: ## Create .env from .env.example if missing
	@test -f .env || (cp .env.example .env && echo "Created .env - edit the change-me values")

up: env ## Build and start the full stack (Postgres, mock vendors, seed, API, web)
	docker compose up --build -d
	@echo "API: http://localhost:8080   Web: http://localhost:3000"

down: ## Stop the stack and remove volumes
	docker compose down -v

logs: ## Tail API logs
	docker compose logs -f api

test: ## Run unit tests (SQLite contract tests included)
	go test -race ./...

test-pg: ## Run all tests including PostgreSQL contract tests in a throwaway container
	docker run -d --rm --name rolodex-pgtest -e POSTGRES_PASSWORD=test -e POSTGRES_DB=rolodex_test -p 55432:5432 postgres:16-alpine >/dev/null
	@until docker exec rolodex-pgtest pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -count=1 ./... ; status=$$?; docker stop rolodex-pgtest >/dev/null; exit $$status

cover: ## Run tests with coverage report
	go test -coverpkg=./internal/... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint: ## Run gofmt, go vet and golangci-lint
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	go vet ./...
	golangci-lint run

fmt: ## Format Go code
	gofmt -w .

run-mock: ## Run mock vendors locally (reads .env)
	set -a; source .env; set +a; MOCK_ABC_USERNAME=$$ABC_USERNAME MOCK_ABC_PASSWORD=$$ABC_PASSWORD \
	MOCK_XYC_USERNAME=$$XYC_USERNAME MOCK_XYC_PASSWORD=$$XYC_PASSWORD go run ./cmd/mockvendors

seed-local: ## Seed the local SQLite database
	set -a; source .env; set +a; DB_DRIVER=sqlite DATABASE_URL=rolodex.db go run ./cmd/seed

run-api: ## Run the API locally against SQLite and local mock vendors (reads .env)
	set -a; source .env; set +a; DB_DRIVER=sqlite DATABASE_URL=rolodex.db \
	ABC_BASE_URL=http://localhost:9001 XYC_BASE_URL=http://localhost:9002 go run ./cmd/api

web: ## Run the frontend dev server
	cd web && npm install && npm run dev
