# Go application structure

The repository contains the environment layer for a single Go backend using PostgreSQL. Booking packages will be created with their first implementation files.

| Directory | Status and responsibility |
| --- | --- |
| `httpapi/` | Implements routing and the database-backed health response; appointment validation and response mapping are planned. |
| `appointments/` | Planned: service duration, interval rules, resource selection, and booking workflow. |
| `catalog/` | Planned: customer, vehicle, dealership, service type, technician qualification, and bay models. |
| `postgres/` | Implements the PostgreSQL connection pool and startup check; booking queries and transactions are planned. |
| `config/` | Implements validated environment configuration for HTTP and database settings. |

`cmd/api/main.go` configures dependencies, starts the HTTP server, and handles graceful shutdown. Booking rules will remain independent of HTTP details and use one clear transaction boundary for creating a confirmed appointment.

Go unit tests will live beside the packages they verify as `*_test.go`. PostgreSQL integration and concurrency suites will live in `tests/integration/`, and HTTP booking workflows will live in `tests/e2e/`.

The module uses Go 1.27.1, `net/http` with `http.ServeMux`, and `pgx/v5` 5.11.0 with `pgxpool`. `golang-migrate` 4.19.1 with SQL files will be introduced with the schema.
