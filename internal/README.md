# Go application structure

The repository contains the environment and booking layers for a single Go backend using PostgreSQL. Appointment creation and retrieval are wired through the HTTP handlers, service, and PostgreSQL repository.

| Directory | Status and responsibility |
| --- | --- |
| `httpapi/` | Implements health, creation, and retrieval routes; JSON validation, response mapping, request IDs, and request logging. |
| `appointments/` | Validates booking input and UTC timestamps; defines domain errors, appointment results, and the booking repository interface. |
| `postgres/` | Implements the connection pool and the complete dealership-locked booking transaction, including reference validation, duration, allocation, commit, rollback, and persisted retrieval. |
| `config/` | Implements validated environment configuration for HTTP and database settings. |

`cmd/api/main.go` configures dependencies, starts the HTTP server, and handles graceful shutdown. The service owns input validation and the repository contract; the PostgreSQL repository performs catalog checks and allocation within one transaction. The service has no HTTP dependency.

Unit tests live beside input validation, HTTP handling, and PostgreSQL connection/error code. [Integration tests](../tests/README.md) use isolated schemas for business rules, HTTP workflows, concurrency, rollback, and operation deadlines.

The module uses Go 1.27.1, `net/http` with `http.ServeMux`, and `pgx/v5` 5.11.0 with `pgxpool`. `golang-migrate` 4.19.1 applies versioned SQL migrations.
