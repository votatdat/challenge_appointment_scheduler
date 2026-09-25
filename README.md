# Unified Service Scheduler

A Go backend for booking vehicle service appointments at a dealership. A booking requires both a qualified technician and a service bay for the full service duration.

This project is the backend submission for Scenario A of the Keyloop Technical Assessment.

**Status:** Environment, schema, and booking transaction implemented. The booking service validates requests, allocates resources under a dealership lock, and persists confirmed appointments. Unit and PostgreSQL integration tests pass. The running HTTP server currently exposes only `GET /healthz`; appointment endpoints are next.

## Scope

Clients provide a customer, vehicle, dealership, service type, and desired start time. The server calculates the duration, selects eligible resources, and persists a confirmed appointment. Overlapping appointments must not share a technician or service bay.

The selected API consists of:

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/appointments` | Create a confirmed appointment; `201 Created` with `Location`. |
| GET | `/appointments/{id}` | Retrieve the persisted appointment; `200 OK`. |

The [API contract](api/README.md) is confirmed: positive integer IDs, RFC 3339 input timestamps, UTC responses, and consistent errors with request IDs. Endpoint implementation is pending.

Catalog data is seeded with deterministic IDs and resource combinations. A cURL client will demonstrate the workflow. Authentication, catalog administration, cancellation, rescheduling, temporary holds, and a separate availability endpoint are outside scope.

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

Prerequisites are Go 1.27.1, Docker with Compose, GNU Make, and cURL.

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
|   |-- httpapi/              # Health route; appointment routes planned
|   |-- appointments/         # Booking rules and allocation
|   |-- catalog/              # Catalog models and qualifications
|   |-- postgres/             # Connection pool and booking transaction
|   `-- config/               # Validated environment configuration
|-- database/
|   |-- migrations/           # Versioned SQL schema changes
|   |-- seeds/                # Deterministic demonstration data
|   `-- verify.sql            # Schema and seed verification
|-- api/                      # API contract documentation
|-- tests/
|   |-- integration/          # PostgreSQL and concurrency tests
|   |-- e2e/                  # HTTP booking workflows
|   `-- fixtures/             # Deterministic test data
`-- scripts/                  # Setup and demonstration helpers
```

The tree includes the implemented booking service, PostgreSQL transaction, and integration tests, plus planned catalog and HTTP test packages. Planned directories will be created with their first real files. Go unit tests will live beside the corresponding implementation as `*_test.go` files.

## AI Collaboration Narrative

I used AI to analyze requirements and compare designs, then selected a small Go/PostgreSQL backend focused on booking correctness. I reviewed the proposals against the brief and narrowed the scope to preserve time for tests, documentation, and the demonstration. Design review clarified customer associations, atomic resource allocation, and the conditions needed for meaningful concurrency tests.

Verification now includes unit tests and real PostgreSQL tests for persisted associations, overlapping intervals, alternative resources, rollback, and competing bookings. Review refined strict timestamp validation and cancellation cleanup. Six sessions competing for one resource pair produced one committed booking and five capacity conflicts; HTTP verification remains pending.
