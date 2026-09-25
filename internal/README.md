# Go application structure

The repository contains the environment and booking layers for a single Go backend using PostgreSQL. Appointment HTTP endpoints remain planned.

| Directory | Status and responsibility |
| --- | --- |
| `httpapi/` | Implements routing and the database-backed health response; appointment validation and response mapping are planned. |
| `appointments/` | Validates booking input and UTC timestamps; defines domain errors, appointment results, and the booking repository interface. |
| `catalog/` | Planned: customer, vehicle, dealership, service type, technician qualification, and bay models. |
| `postgres/` | Implements the connection pool and the complete dealership-locked booking transaction, including reference validation, duration, allocation, commit, and rollback. |
| `config/` | Implements validated environment configuration for HTTP and database settings. |

`cmd/api/main.go` configures dependencies, starts the HTTP server, and handles graceful shutdown. Booking rules are independent of HTTP details and use one transaction boundary for creating a confirmed appointment.

Booking unit tests live beside the service. PostgreSQL integration tests live in `tests/integration/` and use isolated schemas. HTTP booking workflows will live in `tests/e2e/`.

The module uses Go 1.27.1, `net/http` with `http.ServeMux`, and `pgx/v5` 5.11.0 with `pgxpool`. `golang-migrate` 4.19.1 applies versioned SQL migrations.
