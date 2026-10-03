.DEFAULT_GOAL := help
SHELL := /bin/bash

TEST_DATABASE_URL ?= postgres://postgres:test@localhost:55432/rolodex_test?sslmode=disable
GO_DIRS := cmd internal migrations
COVERAGE_MIN ?= 80
K8S_NS := rolodex
# A dedicated minikube profile (and kube context) keeps this project isolated
# from any other local clusters.
MINIKUBE_PROFILE ?= rolodex
MINIKUBE := minikube -p $(MINIKUBE_PROFILE)
KUBECTL := kubectl --context $(MINIKUBE_PROFILE) -n $(K8S_NS)

.PHONY: help env dev run-mock seed-local run-api web up down logs \
	k8s-up k8s-images k8s-secret k8s-apply k8s-forward k8s-status k8s-logs k8s-down \
	test pg-up pg-down test-pg cover lint vuln fmt ci web-test

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*?## "} /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)} /^[a-zA-Z0-9_-]+:.*?## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ Setup

env: ## Create .env from .env.example if missing
	@test -f .env || (cp .env.example .env && echo "Created .env - edit the change-me values")

##@ Run locally without Docker (SQLite)

dev: env ## Run mock vendors, seed, API and web dev server together (Ctrl-C stops all)
	./scripts/dev.sh

run-mock: ## Run only the mock vendors (reads .env)
	set -a; source .env; set +a; MOCK_ABC_USERNAME=$$ABC_USERNAME MOCK_ABC_PASSWORD=$$ABC_PASSWORD \
	MOCK_XYC_USERNAME=$$XYC_USERNAME MOCK_XYC_PASSWORD=$$XYC_PASSWORD go run ./cmd/mockvendors

seed-local: ## Seed the local SQLite database
	set -a; source .env; set +a; DB_DRIVER=sqlite DATABASE_URL=rolodex.db go run ./cmd/seed

run-api: ## Run only the API against SQLite and local mock vendors
	set -a; source .env; set +a; DB_DRIVER=sqlite DATABASE_URL=rolodex.db \
	ABC_BASE_URL=http://localhost:9001 XYC_BASE_URL=http://localhost:9002 go run ./cmd/api

web: ## Run only the frontend dev server (http://localhost:5173)
	cd web && npm install && npm run dev

##@ Run with Docker Compose (PostgreSQL)

up: env ## Build and start the full stack
	docker compose up --build -d
	@echo "API: http://localhost:8080   Web: http://localhost:3000"

down: ## Stop the stack and remove volumes
	docker compose down -v

logs: ## Tail API logs
	docker compose logs -f api

##@ Run on minikube (Kubernetes)

k8s-up: env ## Start minikube if needed, build images, deploy everything and wait until ready
	@$(MINIKUBE) status >/dev/null 2>&1 || $(MINIKUBE) start --cpus=2 --memory=3072
	$(MAKE) k8s-images k8s-secret k8s-apply
	$(KUBECTL) rollout status statefulset/postgres --timeout=180s
	$(KUBECTL) wait --for=condition=complete job/seed --timeout=180s
	$(KUBECTL) rollout status deployment/mockvendors --timeout=120s
	$(KUBECTL) rollout status deployment/api --timeout=180s
	$(KUBECTL) rollout status deployment/web --timeout=120s
	@echo "Deployed. Run 'make k8s-forward', then open http://localhost:3000"

k8s-images: ## Build the API and web images inside minikube
	$(MINIKUBE) image build -t rolodex:local .
	$(MINIKUBE) image build -t rolodex-web:local --build-opt=build-arg=VITE_API_URL=http://localhost:8080 web

k8s-secret: ## Create/update the rolodex-env Secret from .env
	$(KUBECTL) apply -f deploy/k8s/namespace.yaml
	$(KUBECTL) create secret generic rolodex-env --from-env-file=.env --dry-run=client -o yaml | $(KUBECTL) apply -f -

k8s-apply: ## Apply the manifests (re-runs the idempotent seed Job)
	-$(KUBECTL) delete job seed --ignore-not-found
	$(KUBECTL) apply -k deploy/k8s
	$(KUBECTL) rollout restart deployment/api deployment/mockvendors deployment/web

k8s-forward: ## Port-forward API to :8080 and web to :3000 (Ctrl-C stops)
	@echo "API: http://localhost:8080   Web: http://localhost:3000"
	@trap 'kill 0' EXIT INT TERM; \
	$(KUBECTL) port-forward svc/api 8080:8080 & \
	$(KUBECTL) port-forward svc/web 3000:8080 & \
	wait

k8s-status: ## Show pods, services and jobs
	$(KUBECTL) get pods,svc,jobs,pvc

k8s-logs: ## Tail API logs from all replicas
	$(KUBECTL) logs -f -l app.kubernetes.io/name=api --prefix --max-log-requests 5

k8s-down: ## Delete the rolodex namespace (data included); `minikube -p rolodex stop` halts the VM
	kubectl --context $(MINIKUBE_PROFILE) delete namespace $(K8S_NS) --ignore-not-found

##@ Quality

test: ## Run unit tests (SQLite contract tests included)
	go test -race ./...

pg-up: ## Start the throwaway PostgreSQL test container (port 55432)
	@docker inspect rolodex-pgtest >/dev/null 2>&1 || docker run -d --rm --name rolodex-pgtest \
		-e POSTGRES_PASSWORD=test -e POSTGRES_DB=rolodex_test -p 55432:5432 postgres:16-alpine >/dev/null
	@until docker exec rolodex-pgtest pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done

pg-down: ## Stop the PostgreSQL test container
	-docker stop rolodex-pgtest >/dev/null

test-pg: pg-up ## Run all tests including PostgreSQL contract tests
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -count=1 ./...

cover: pg-up ## Run tests (SQLite + PostgreSQL) with coverage; fail below COVERAGE_MIN percent
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -count=1 -coverpkg=./internal/... -coverprofile=coverage.out ./...
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {sub("%","",$$3); print $$3}'); \
	echo "total coverage: $$total% (min $(COVERAGE_MIN)%)"; \
	awk -v t="$$total" -v m="$(COVERAGE_MIN)" 'BEGIN { exit (t+0 < m+0) }'

lint: ## Run gofmt, go vet and golangci-lint
	@test -z "$$(gofmt -l $(GO_DIRS))" || (gofmt -l $(GO_DIRS) && exit 1)
	go vet ./...
	golangci-lint run

vuln: ## Scan Go dependencies and stdlib for reachable vulnerabilities
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

fmt: ## Format Go code
	gofmt -w $(GO_DIRS)

web-test: ## Run frontend lint, typecheck and tests with coverage
	cd web && npm run lint && npm run typecheck && npm run test:coverage

ci: lint cover vuln web-test ## Run the CI checks locally
