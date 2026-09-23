# Go application structure

This document describes the planned internal packages for a single Go backend using PostgreSQL. Packages will be created with their implementation; no application code exists yet.

| Directory | Responsibility |
| --- | --- |
| `httpapi/` | HTTP routing, request validation, and response/error mapping. |
| `appointments/` | Service duration, interval rules, resource selection, and booking workflow. |
| `catalog/` | Customer, vehicle, dealership, service type, technician qualification, and bay models. |
| `postgres/` | PostgreSQL queries, persistence, and transaction handling. |
| `config/` | Load and validate application configuration. |

The future `cmd/api/main.go` entry point will configure dependencies, start the HTTP server, and manage shutdown. Keep booking rules independent of HTTP details. Define one clear transaction boundary for creating a confirmed appointment.

Place Go unit tests beside the packages they verify as `*_test.go`. Reserve `tests/integration/` for PostgreSQL integration and concurrency suites and `tests/e2e/` for HTTP booking workflows.

Selected tools are `net/http` with `http.ServeMux`, `pgx/v5` with `pgxpool`, and `golang-migrate` with SQL files. The module path and exact versions remain to be pinned. See [architecture.md](../architecture.md) for the selected design.
