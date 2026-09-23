# Unified Service Scheduler

A Go backend for booking vehicle service appointments at a dealership. A booking requires both a qualified technician and a service bay for the full service duration.

This project is the backend submission for Scenario A of the Keyloop Technical Assessment.

**Status:** Local application environment complete. The Go service connects to PostgreSQL, exposes `GET /healthz`, and shuts down gracefully. Appointment schema, booking endpoints, and business tests are planned next.

## Scope

Clients provide a customer, vehicle, dealership, service type, and desired start time. The server calculates the duration, selects eligible resources, and persists a confirmed appointment. Overlapping appointments must not share a technician or service bay.

The selected API consists of:

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/appointments` | Create a confirmed appointment; `201 Created` with `Location`. |
| GET | `/appointments/{id}` | Retrieve the persisted appointment; `200 OK`. |

The [API contract](api/README.md) is confirmed: positive integer IDs, RFC 3339 input timestamps, UTC responses, and consistent errors with request IDs. Endpoint implementation is pending.

Catalog data will be seeded. A cURL client will demonstrate the workflow. Authentication, catalog administration, cancellation, rescheduling, temporary holds, and a separate availability endpoint are outside scope.

Start times must be in the future, include a timezone offset, and are normalized to UTC. The service type determines duration, and `[start, end)` intervals allow back-to-back appointments. Resources are available unless booked, without working-hours or maintenance schedules. Booking validates the seeded customer/vehicle relationship but does not verify customer identity. See [Architecture assumptions](docs/architecture.md#assumptions).

## Technology choices

- Go 1.27.1 with `net/http` and `http.ServeMux`.
- PostgreSQL 18.6 on Alpine 3.23.
- `pgx/v5` 5.11.0 with `pgxpool`.
- `golang-migrate` 4.19.1 with SQL migration files, starting with the schema step.
- Docker Compose for the local application and database.
- `READ COMMITTED` transactions with a dealership row lock before checking availability.

See [Architecture](docs/architecture.md) for component responsibilities, data flow, and tradeoffs.

## Build, run, and test

Prerequisites are Go 1.27.1, Docker with Compose, GNU Make, and cURL.

Start the complete local environment:

```bash
cp .env.example .env
make up
curl --fail http://localhost:8080/healthz
```

A healthy service returns:

```json
{"database":"up","status":"ok"}
```

Stop the containers while preserving PostgreSQL data:

```bash
make down
```

For local Go development, start PostgreSQL and run the API outside its container:

```bash
make db-up
make run
```

Current verification commands are:

```bash
make build
make test
make vet
# Or run test and vet together:
make check
```

Use `make help` to list all available commands. Migration, seed, and booking demonstration commands will be added with the schema and API. The current commands have been verified against the containerized environment. Business-rule and concurrency tests remain planned; see the [test plan](tests/README.md).

## Planned project structure

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
|   `-- seeds/                # Demonstration data
|-- api/                      # API contract documentation
|-- tests/
|   |-- integration/          # PostgreSQL and concurrency tests
|   |-- e2e/                  # HTTP booking workflows
|   `-- fixtures/             # Deterministic test data
`-- scripts/                  # Setup and demonstration helpers
```

The tree shows the intended implementation layout. Source, migration, and test directories will be created when their first real files are added. Go unit tests will live beside the corresponding implementation as `*_test.go` files.

## AI Collaboration Narrative

I used AI to analyze requirements and compare designs, then selected a small Go/PostgreSQL backend focused on booking correctness. I reviewed the proposals against the brief and narrowed the scope to preserve time for tests, documentation, and the demonstration. Design review clarified customer associations, atomic resource allocation, and the conditions needed for meaningful concurrency tests.

I verified the current environment by compiling and vetting the Go service, starting both containers, checking database connectivity and the health response, retaining data across a database restart, and observing graceful shutdown. Booking quality will be refined through the planned business-rule and concurrent PostgreSQL tests.
