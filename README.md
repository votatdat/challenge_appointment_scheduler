# Unified Service Scheduler

A Go and PostgreSQL backend for Scenario A of the Keyloop Technical Assessment. It confirms a vehicle service appointment only when a qualified technician and a service bay are available for the entire service duration.

**Status:** Implementation, business-rule tests, concurrency/rollback tests, and operational checks are complete. Final clean-setup rehearsal, video, and submission remain in the [delivery plan](docs/plan.md).

## API and scope

| Method | Path | Result |
| --- | --- | --- |
| POST | `/appointments` | Create a confirmed appointment; `201 Created` with `Location`. |
| GET | `/appointments/{id}` | Retrieve the persisted appointment; `200 OK`. |
| GET | `/healthz` | Database-backed health status; `200` or `503`. |

Clients supply customer, vehicle, dealership, service type, and start time. The server validates ownership, derives duration, allocates both resources, and commits the booking before returning success. See the [API contract and examples](api/README.md).

Starts must be in the future and include a timezone offset. Times normalize to UTC at microsecond precision; `[start, end)` intervals allow back-to-back bookings. Resources are continuously available unless booked. Customers and vehicles are seeded reference data. Authentication, catalog administration, working-hours schedules, cancellation, rescheduling, holds, and availability browsing are outside scope.

## Build, run, and test

Run commands from the repository root. Container setup requires Docker with Compose, GNU Make, cURL, and a POSIX shell with GNU coreutils, such as WSL. Local builds and tests additionally require Go 1.27.1. Race detection requires a supported Go platform with CGO enabled and a C compiler.

### Start with Docker

```bash
make env
make db-init
make up
curl --fail http://localhost:8080/healthz
make demo
```

`make env` creates `.env` from [.env.example](.env.example) only if it is missing. `make db-init` applies migrations and seeds the catalog. `make up` builds the application image, applies pending migrations, and starts the app and database; it does not seed data by itself. PostgreSQL data is stored in a named volume.

A healthy service returns `{"database":"up","status":"ok"}`. `make demo` creates an appointment one day ahead, retrieves its `Location`, and verifies identical JSON. Each successful run adds one appointment. Set `START_TIME` to another future RFC 3339 value if repeated runs fill capacity, or set `BASE_URL` to target another server. See [demo options](api/README.md#curl-demonstration).

Stop containers while retaining data:

```bash
make down
```

### Run Go locally

Stop the container environment first to free the application port, then start the database and local API:

```bash
make down
make db-init
make run
```

`make run` loads `.env` and runs `cmd/api`. Stop it with Ctrl+C; use `make down` afterward to stop PostgreSQL. To compile without starting the application:

```bash
make build
```

The binary is written to `bin/scheduler-api`. If Go is outside your shell's PATH, override the Make variable, for example `make GO=/usr/local/go/bin/go build`.

### Migrations and seed data

```bash
make migrate-up       # Apply pending migrations
make migrate-version  # Show the current schema version
make seed             # Upsert deterministic demonstration data
make db-verify        # Apply migrations/seed, then verify relationships and constraints
```

The seed includes two dealerships, qualified and unqualified technicians, alternative bays, customers, vehicles, services, and one occupied pair. [Database documentation](database/README.md) lists the IDs and relationships. Re-seeding restores the fixed demonstration records and preserves other appointment IDs. `make migrate-down` reverts the latest migration; with the initial migration, this deletes all application tables and their data.

### Verification

```bash
make check            # Unit tests and Go static analysis
make test-integration # Real PostgreSQL tests in temporary schemas
make db-verify        # Seed and schema checks
```

Run race detection and static analysis of integration tests with:

```bash
GOFLAGS=-race make test test-integration
go vet -tags=integration ./...
```

Integration tests start PostgreSQL and use `TEST_DATABASE_URL` when set, otherwise `DATABASE_URL` from `.env`. Each test applies the migration and loads its own fixtures in a unique schema, then removes that schema. The database role needs schema-creation permission; demonstration tables are preserved. `make check` does not run integration tests. See [test coverage](tests/README.md), including the distinction between automated tests and live operational rehearsals.

Use `make help` for all targets.

## Configuration and operations

Defaults in `.env.example` match the Compose setup:

| Setting | Default | Purpose |
| --- | --- | --- |
| `APP_PORT` | `8080` | Published container port and default demo port. |
| `APP_ADDR` | `:8080` | Listen address for local Go execution. |
| `POSTGRES_PORT` | `5432` | Published database port. |
| `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` | `scheduler` | Local PostgreSQL initialization and container connection settings. |
| `DATABASE_URL` | Local PostgreSQL URL in `.env.example` | Connection for local Go execution and integration tests. |
| `DB_MAX_CONNECTIONS` | `10` | Maximum application pool size. |

The app container connects to `db:5432`; local Go connects through the published port. If you change `POSTGRES_PORT`, also update `DATABASE_URL`. Adjust the cURL address when changing the app port; for local Go, keep `APP_ADDR` and the demo's `APP_PORT` or `BASE_URL` consistent. Connection and HTTP timeout settings are listed in [operational limits](docs/architecture.md#observability-and-failure-handling).

Each handled request receives an `X-Request-ID` correlated with a JSON outcome log. Successful creation and retrieval also log `appointment_id`; bodies, query values, and customer contact data are omitted. Dependency unavailability and database deadlines return safe `503` responses; unexpected failures return `500`. Inspect logs with `docker compose logs --no-color app`.

## Design and limits

The service uses Go `net/http` and `ServeMux`, `pgx/v5` 5.11.0 with `pgxpool`, PostgreSQL 18.6 on Alpine 3.23, and `golang-migrate` 4.19.1 SQL migrations.

- A `READ COMMITTED` transaction locks the dealership before checking availability and commits both resource assignments together.
- All booking writers must follow that protocol. Direct SQL writes do not receive overlap protection, and catalog data is assumed static.
- Bookings at the same dealership serialize, even when different resource pairs are available. Different dealerships have separate lock rows.
- A lost response or connection failure during commit can leave the client uncertain whether the booking exists. There is no idempotency key or automatic retry; another POST may create another booking if capacity remains.

[Architecture](docs/architecture.md) explains the components, data flow, relational constraints, assumptions, and tradeoffs. No throughput benchmark or production capacity claim is made.

## Project structure

```text
.
|-- README.md
|-- Makefile                  # Build, environment, database, and test commands
|-- compose.yaml              # Application, PostgreSQL, and migration containers
|-- Dockerfile                # Application image
|-- docs/
|   |-- architecture.md       # System design and tradeoffs
|   `-- plan.md               # Delivery progress and decisions
|-- cmd/api/                  # Startup and graceful shutdown
|-- internal/
|   |-- httpapi/              # Routes, JSON contracts, and request logs
|   |-- appointments/         # Input validation and repository contract
|   |-- postgres/             # Pool, transaction, allocation, and retrieval
|   `-- config/               # Environment configuration
|-- database/
|   |-- migrations/           # Versioned SQL schema
|   |-- seeds/                # Demonstration data
|   `-- verify.sql            # Schema and seed checks
|-- api/                      # API contract and examples
|-- tests/integration/        # PostgreSQL, HTTP, concurrency, and deadline tests
`-- scripts/demo.sh           # cURL creation/retrieval demonstration
```

Unit tests live beside their implementation. Catalog relationships are queried inside the PostgreSQL transaction; integration fixtures live with the tests.

## AI Collaboration Narrative

I guided AI through a small delivery plan, selected Go/PostgreSQL and dealership locking, and kept the scope focused on booking correctness. I reviewed its proposals against the brief and required evidence before marking steps complete.

Review refined timestamp parsing and cancellation cleanup and found that background connection attempts needed their own deadline. Verification combines unit tests, real PostgreSQL/HTTP tests, observed database contention, injected commit failures, race detection, and live slow-request/outage rehearsals. These checks support the delivered behavior; the final clean-setup rehearsal remains a separate delivery step.
