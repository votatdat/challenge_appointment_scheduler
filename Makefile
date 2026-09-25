SHELL := /bin/sh

GO ?= go
DOCKER_COMPOSE ?= docker compose
APP_BINARY := bin/scheduler-api
MIGRATE := $(DOCKER_COMPOSE) run --rm migrate

.DEFAULT_GOAL := help

.PHONY: help env db-up migrate-up migrate-down migrate-version seed db-init db-verify up down logs run build test test-integration fmt vet check clean

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

env: ## Create .env from .env.example when it is missing
	@test -f .env || cp .env.example .env

db-up: env ## Start PostgreSQL and wait until it is healthy
	$(DOCKER_COMPOSE) up -d --wait db

migrate-up: db-up ## Apply all pending database migrations
	$(MIGRATE) up

migrate-down: db-up ## Revert the latest database migration
	$(MIGRATE) down 1

migrate-version: db-up ## Show the current database migration version
	$(MIGRATE) version

seed: migrate-up ## Load deterministic demonstration data
	$(DOCKER_COMPOSE) exec -T db sh -c 'psql --set ON_ERROR_STOP=1 --username "$$POSTGRES_USER" --dbname "$$POSTGRES_DB"' < database/seeds/demo.sql

db-init: seed ## Apply migrations and load demonstration data

db-verify: db-init ## Verify seeded data and database constraints
	$(DOCKER_COMPOSE) exec -T db sh -c 'psql --set ON_ERROR_STOP=1 --username "$$POSTGRES_USER" --dbname "$$POSTGRES_DB"' < database/verify.sql

up: env migrate-up ## Build and start the app and PostgreSQL
	$(DOCKER_COMPOSE) up -d --build --wait

down: ## Stop containers and preserve PostgreSQL data
	$(DOCKER_COMPOSE) down

logs: ## Follow app and PostgreSQL logs
	$(DOCKER_COMPOSE) logs -f app db

run: env migrate-up ## Run the API locally with Go
	set -a; . ./.env; set +a; $(GO) run ./cmd/api

build: ## Build the API binary
	mkdir -p bin
	$(GO) build -trimpath -o $(APP_BINARY) ./cmd/api

test: ## Run all Go tests
	$(GO) test ./...

test-integration: db-up ## Run booking tests in isolated PostgreSQL schemas
	set -a; . ./.env; set +a; TEST_DATABASE_URL="$${TEST_DATABASE_URL:-$$DATABASE_URL}" $(GO) test -tags=integration -count=1 -timeout=90s ./tests/integration/...

fmt: ## Format Go source files
	$(GO) fmt ./...

vet: ## Run Go static analysis
	$(GO) vet ./...

check: test vet ## Run the current verification checks

clean: ## Remove local build output
	rm -rf bin
