SHELL := /bin/sh

GO ?= go
DOCKER_COMPOSE ?= docker compose
APP_BINARY := bin/scheduler-api

.DEFAULT_GOAL := help

.PHONY: help env db-up up down logs run build test fmt vet check clean

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

env: ## Create .env from .env.example when it is missing
	@test -f .env || cp .env.example .env

db-up: env ## Start PostgreSQL and wait until it is healthy
	$(DOCKER_COMPOSE) up -d --wait db

up: env ## Build and start the app and PostgreSQL
	$(DOCKER_COMPOSE) up -d --build --wait

down: ## Stop containers and preserve PostgreSQL data
	$(DOCKER_COMPOSE) down

logs: ## Follow app and PostgreSQL logs
	$(DOCKER_COMPOSE) logs -f app db

run: env db-up ## Run the API locally with Go
	set -a; . ./.env; set +a; $(GO) run ./cmd/api

build: ## Build the API binary
	mkdir -p bin
	$(GO) build -trimpath -o $(APP_BINARY) ./cmd/api

test: ## Run all Go tests
	$(GO) test ./...

fmt: ## Format Go source files
	$(GO) fmt ./...

vet: ## Run Go static analysis
	$(GO) vet ./...

check: test vet ## Run the current verification checks

clean: ## Remove local build output
	rm -rf bin
