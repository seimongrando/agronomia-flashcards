# ═══════════════════════════════════════════════════════════════════════════════
# Agronomia Flashcards — Makefile
# ═══════════════════════════════════════════════════════════════════════════════
# Run `make help` to see all available targets.
#
# Quick start:
#   cp .env.local.example .env.local   # fill in Google OAuth credentials
#   make dev                           # DB → wait → server (migrations run automatically)
#
# Migrations:
#   The Go server applies pending migrations automatically on startup via the
#   embedded runner (internal/migrate). You do NOT need to run `make migrate-up`
#   for normal development — just restart the server.
#
#   Use `make migrate-up` / `make migrate-down` only when you need to:
#     • Roll back a migration manually
#     • Inspect migration status (make migrate-status)
#     • Create a new migration file (make migrate-create name=add_foo)
# ═══════════════════════════════════════════════════════════════════════════════

.DEFAULT_GOAL := help

# Load .env.local when it exists (silent if missing so CI still works).
-include .env.local
export

# ── Config ────────────────────────────────────────────────────────────────────
COMPOSE_PROJECT   ?= flashcards
MIGRATIONS_DIR    := migrations
BIN               := bin/server

# Fallback Postgres creds (must match docker-compose.yml defaults).
POSTGRES_USER     ?= webapp
POSTGRES_PASSWORD ?= webapp_dev
POSTGRES_DB       ?= webapp

# ── Help ──────────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help message
	@echo ""
	@echo "  \033[1mAgronomia Flashcards — local dev commands\033[0m"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""

# ── Go tooling ────────────────────────────────────────────────────────────────
.PHONY: deps
deps: ## Download and tidy Go module dependencies
	go mod download && go mod tidy

.PHONY: fmt
fmt: ## Format all Go source files with gofmt
	gofmt -w -s ./...

.PHONY: vet
vet: ## Run go vet across all packages
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (falls back to go vet if not installed)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "  golangci-lint not found — running go vet instead."; \
		go vet ./...; \
	fi

.PHONY: test
test: ## Run all tests with race detector
	go test ./... -race -count=1

.PHONY: test-short
test-short: ## Run tests skipping slow/integration tests (-short flag)
	go test ./... -short -count=1

.PHONY: build
build: ## Build production binary to bin/server
	@mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN) ./cmd/server

# ── Docker ────────────────────────────────────────────────────────────────────
.PHONY: docker-up
docker-up: ## Start Postgres in the background (detached)
	docker compose -p $(COMPOSE_PROJECT) up -d postgres

.PHONY: docker-up-tools
docker-up-tools: ## Start Postgres + Adminer web UI (port 8081)
	docker compose -p $(COMPOSE_PROJECT) --profile tools up -d

.PHONY: docker-down
docker-down: ## Stop all compose services (keeps data volumes)
	docker compose -p $(COMPOSE_PROJECT) down

.PHONY: docker-logs
docker-logs: ## Stream Postgres container logs
	docker compose -p $(COMPOSE_PROJECT) logs -f postgres

.PHONY: docker-reset
docker-reset: ## ⚠ Delete all local DB data (asks for confirmation)
	@printf '\033[33m⚠  This will permanently DELETE all local Postgres data.\033[0m\n'
	@printf '   Press \033[1mEnter\033[0m to continue or \033[1mCtrl-C\033[0m to abort: '
	@read _
	docker compose -p $(COMPOSE_PROJECT) down -v
	rm -rf .data/postgres
	@echo "  Done. Run 'make dev' to start fresh."

.PHONY: docker-build
docker-build: ## Build the production Docker image (tag: flashcards:local)
	docker build -t flashcards:local .

.PHONY: docker-run
docker-run: ## Run the production image with .env.local
	docker run --rm -p 8080:8080 --env-file .env.local flashcards:local

# ── Database ──────────────────────────────────────────────────────────────────

# Internal: install golang-migrate CLI if not already on PATH.
# Strategy (in order):
#   1. brew (macOS)
#   2. go install (universal fallback — requires Go in PATH)
.PHONY: ensure-migrate
ensure-migrate:
	@if command -v migrate >/dev/null 2>&1; then \
		exit 0; \
	fi; \
	echo "  golang-migrate not found. Installing…"; \
	if [ "$$(uname -s)" = "Darwin" ] && command -v brew >/dev/null 2>&1; then \
		brew install golang-migrate; \
	else \
		go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
		echo "  Installed via 'go install'. Make sure $$(go env GOPATH)/bin is in your PATH."; \
	fi; \
	if ! command -v migrate >/dev/null 2>&1; then \
		echo ""; \
		echo "  ✗ 'migrate' still not found after installation."; \
		echo "    Add Go's bin directory to your PATH and retry:"; \
		echo "      export PATH=\$$PATH:\$$(go env GOPATH)/bin"; \
		echo ""; \
		exit 1; \
	fi; \
	echo "  ✓ golang-migrate is ready."

.PHONY: db-wait
db-wait: ## Block until Postgres is ready to accept connections (max 60 s)
	@echo "  Waiting for Postgres…"
	@for i in $$(seq 1 60); do \
		docker compose -p $(COMPOSE_PROJECT) exec -T postgres \
			pg_isready -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" \
			>/dev/null 2>&1 \
		&& echo "  ✓ Postgres is ready." && exit 0; \
		printf "  … attempt %d/60\r" $$i; \
		sleep 1; \
	done; \
	echo ""; \
	echo "  ✗ Postgres did not become ready within 60 s."; \
	exit 1

.PHONY: migrate-up
migrate-up: ensure-migrate ## Apply all pending migrations (installs golang-migrate if needed)
	migrate -path $(MIGRATIONS_DIR) \
	        -database "$(DATABASE_URL)" \
	        up

.PHONY: migrate-down
migrate-down: ensure-migrate ## Roll back the last migration
	migrate -path $(MIGRATIONS_DIR) \
	        -database "$(DATABASE_URL)" \
	        down 1

.PHONY: migrate-down-all
migrate-down-all: ensure-migrate ## Roll back ALL migrations (drops all app tables)
	@printf '\033[33m⚠  This will drop all app tables.\033[0m Press Enter to continue: '
	@read _
	migrate -path $(MIGRATIONS_DIR) \
	        -database "$(DATABASE_URL)" \
	        down

.PHONY: migrate-create
migrate-create: ensure-migrate ## Create a new migration pair  (usage: make migrate-create name=add_foo)
	@test -n "$(name)" || (echo "Error: name is required. Usage: make migrate-create name=<slug>" && exit 1)
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

.PHONY: migrate-status
migrate-status: ensure-migrate ## Print current migration version
	migrate -path $(MIGRATIONS_DIR) \
	        -database "$(DATABASE_URL)" \
	        version

# ── Running ───────────────────────────────────────────────────────────────────
.PHONY: run
run: ## Start the Go server — migrations run automatically on startup
	go run ./cmd/server

.PHONY: dev
dev: docker-up db-wait run ## 🚀 Full local bootstrap: DB → wait → server (auto-migrates)

# ── PWA / Push helpers ────────────────────────────────────────────────────────
.PHONY: vapid-keys
vapid-keys: ## Generate VAPID key pair for PWA push notifications
	go run ./cmd/genvapid

# ── Admin helpers ─────────────────────────────────────────────────────────────
.PHONY: seed-admin
seed-admin: ## Show instructions for enabling admin access via ADMIN_EMAILS
	@echo ""
	@echo "  \033[1mAdmin bootstrap — how to get admin access locally\033[0m"
	@echo "  ─────────────────────────────────────────────────────────"
	@echo ""
	@echo "  1. Edit \033[36m.env.local\033[0m and set your Google account email:"
	@echo "       ADMIN_EMAILS=seuemail@gmail.com"
	@echo ""
	@echo "  2. Restart the server: \033[36mmake run\033[0m (or \033[36mmake dev\033[0m)."
	@echo ""
	@echo "  3. Open \033[36mhttp://localhost:8080\033[0m → 'Entrar com Google'."
	@echo "     On first login your account is automatically promoted to admin."
	@echo ""
	@echo "  4. Admin-only pages:"
	@echo "       http://localhost:8080/teach.html       (professor / admin)"
	@echo "       http://localhost:8080/admin_users.html (admin only)"
	@echo ""
	@echo "  Tip: to grant a role manually via psql:"
	@echo "       \033[36mmake psql\033[0m"
	@echo "       INSERT INTO user_roles (user_id, role)"
	@echo "         SELECT id, 'admin' FROM users WHERE email = 'seuemail@gmail.com';"
	@echo "  ─────────────────────────────────────────────────────────"
	@echo ""

.PHONY: psql
psql: ## Open a psql shell inside the Postgres container
	docker compose -p $(COMPOSE_PROJECT) exec postgres \
		psql -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)"
