# Unified Service Scheduler

A Go backend for booking vehicle service appointments at a dealership. A booking requires both a qualified technician and a service bay for the full service duration.

This project is the backend submission for Scenario A of the Keyloop Technical Assessment.

**Status:** Appointment creation and retrieval are implemented with PostgreSQL persistence, resource locking, JSON errors, and request IDs. Unit and HTTP/PostgreSQL integration tests pass. Business-rule, concurrency, and rollback coverage are complete; remaining delivery work includes operational review, documentation rehearsal, and video.

## Scope

Clients provide a customer, vehicle, dealership, service type, and desired start time. The server calculates the duration, selects eligible resources, and persists a confirmed appointment. Overlapping appointments must not share a technician or service bay.

The selected API consists of:

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/appointments` | Create a confirmed appointment; `201 Created` with `Location`. |
| GET | `/appointments/{id}` | Retrieve the persisted appointment; `200 OK`. |

The [API contract](api/README.md) is confirmed: positive integer IDs, RFC 3339 input timestamps, UTC responses, and consistent errors with request IDs. Both endpoints are implemented.

Catalog data is seeded with deterministic IDs and resource combinations. `make demo` demonstrates creation and retrieval using cURL. Authentication, catalog administration, cancellation, rescheduling, temporary holds, and a separate availability endpoint are outside scope.

Start times must be in the future, include a timezone offset, and are normalized to UTC. The service type determines duration, and `[start, end)` intervals allow back-to-back appointments. Resources are available unless booked, without working-hours or maintenance schedules. Booking validates the seeded customer/vehicle relationship but does not verify customer identity. See [Architecture assumptions](docs/architecture.md#assumptions).

## Technology choices

- Go 1.27.1 with `net/http` and `http.ServeMux`.
- PostgreSQL 18.6 on Alpine 3.23.
- `pgx/v5` 5.11.0 with `pgxpool`.
- `golang-migrate` 4.19.1 with SQL migration files.
- Docker Compose for the local application and database.
- `READ COMMITTED` transactions with a dealership row lock before checking availability.

See [Architecture](docs/architecture.md) for component responsibilities, data flow, and tradeoffs.

## Build, run, and test

Prerequisites are Go 1.27.1, Docker with Compose, GNU Make, cURL, and a POSIX shell with GNU coreutils (for example, WSL).

Initialize the database and start the complete local environment:

```bash
cp .env.example .env
make db-init
make up
curl --fail http://localhost:8080/healthz
```

A healthy service returns:

```json
{"database":"up","status":"ok"}
```

Create and retrieve an appointment with seeded IDs and a start time one day ahead:

```bash
make demo
```

The script expects `201`, follows the `Location` header, expects `200`, and checks that both JSON representations match. Each run creates a persisted appointment. Override `START_TIME` with a future RFC 3339 timestamp or `BASE_URL` for a different server. If a repeated run fills the available resources, choose a different start time. See [API usage](api/README.md#curl-demonstration).

Database operations are available separately:

```bash
make migrate-up       # Apply pending migrations
make migrate-version  # Show the current schema version
make seed             # Load deterministic demonstration data
make db-verify        # Verify seed relationships, constraints, and indexes
```

`make migrate-down` reverts the latest migration and deletes its schema data. Stop containers while preserving PostgreSQL data with `make down`.

For local Go development, initialize the database once and run the API outside its container:

```bash
make db-init
make run
```

Current verification commands are:

```bash
make build
make test
make vet
make db-verify
make test-integration  # Real PostgreSQL booking tests in temporary schemas
# Or run the current Go checks together:
make check
```

Use `make help` to list all available commands. `make test-integration` starts PostgreSQL and uses `TEST_DATABASE_URL` when set, otherwise `DATABASE_URL` from `.env`. Each test applies the migration in a unique schema and removes that schema afterward, preserving demonstration data. The database role must be allowed to create schemas. See [test coverage and remaining work](tests/README.md).

## Project structure

```text
.
|-- README.md                 # Project overview and usage
|-- docs/
|   |-- architecture.md       # System design and tradeoffs
|   `-- plan.md               # Delivery progress and decisions
|-- cmd/api/                  # Server startup and graceful shutdown
|-- internal/
|   |-- httpapi/              # Health and appointment HTTP handlers
|   |-- appointments/         # Booking rules and allocation
|   |-- catalog/              # Catalog models and qualifications
|   |-- postgres/             # Booking transaction and retrieval
|   `-- config/               # Validated environment configuration
|-- database/
|   |-- migrations/           # Versioned SQL schema changes
|   |-- seeds/                # Deterministic demonstration data
|   `-- verify.sql            # Schema and seed verification
|-- api/                      # API contract documentation
|-- tests/
|   |-- integration/          # PostgreSQL, HTTP, and concurrency tests
|   |-- e2e/                  # HTTP booking workflows
|   `-- fixtures/             # Deterministic test data
`-- scripts/                  # Setup and demonstration helpers
```

The tree includes the implemented booking service, PostgreSQL transaction, and integration tests, plus planned catalog and additional test packages. Planned directories will be created with their first real files. Go unit tests will live beside the corresponding implementation as `*_test.go` files.

## AI Collaboration Narrative

I used AI to analyze requirements and compare designs, then selected a small Go/PostgreSQL backend focused on booking correctness. I reviewed the proposals against the brief and narrowed the scope to preserve time for tests, documentation, and the demonstration. Design review clarified customer associations, atomic resource allocation, and the conditions needed for meaningful concurrency tests.

Verification now includes unit tests and real PostgreSQL tests for persisted associations, overlapping intervals, alternative resources, rollback, and competing bookings. Review refined strict timestamp validation and cancellation cleanup, then added independent technician/bay checks for containment, adjacency, and microsecond boundaries. Coordinated HTTP tests verify one or two successes according to available capacity, dealership independence, and recovery of a queued booking after a failed commit. HTTP tests and a cURL rehearsal verify creation/retrieval, safe errors, and recovery after database unavailability.
