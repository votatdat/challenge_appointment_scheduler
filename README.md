# Unified Service Scheduler

A Go backend for booking vehicle service appointments at a dealership. A booking requires both a qualified technician and a service bay for the full service duration.

This project is the backend submission for Scenario A of the Keyloop Technical Assessment.

**Status:** Environment, schema, and demonstration data complete. The Go service connects to PostgreSQL, exposes `GET /healthz`, and shuts down gracefully. Booking endpoints and business tests are planned next.

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
# Or run the current Go checks together:
make check
```

Use `make help` to list all available commands. These commands have been verified against a fresh containerized database. Business-rule and concurrency tests remain planned; see the [test plan](tests/README.md).

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
|   |-- postgres/             # Connection pool; booking queries planned
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

The tree combines implemented environment and database files with planned booking and test packages. Planned directories will be created with their first real files. Go unit tests will live beside the corresponding implementation as `*_test.go` files.

## AI Collaboration Narrative

I used AI to analyze requirements and compare designs, then selected a small Go/PostgreSQL backend focused on booking correctness. I reviewed the proposals against the brief and narrowed the scope to preserve time for tests, documentation, and the demonstration. Design review clarified customer associations, atomic resource allocation, and the conditions needed for meaningful concurrency tests.

I verified the environment by compiling and vetting the service, checking health and graceful shutdown, and rebuilding the database from an empty volume. I refined the schema with composite foreign keys and executable checks for invalid relationships, intervals, qualifications, and indexes. Booking quality will be assessed through the planned business-rule and concurrent PostgreSQL tests.
