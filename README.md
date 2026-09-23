# Unified Service Scheduler

A Go backend for booking vehicle service appointments at a dealership. A booking requires both a qualified technician and a service bay for the full service duration.

This project is the backend submission for Scenario A of the Keyloop Technical Assessment.

**Status:** Design and documentation. The technology choices are recorded, but the application, database migrations, and automated tests are not implemented yet.

## Scope

Clients provide a customer, vehicle, dealership, service type, and desired start time. The server calculates the duration, selects eligible resources, and persists a confirmed appointment. Overlapping appointments must not share a technician or service bay.

The selected API consists of:

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/appointments` | Create a confirmed appointment; `201 Created` with `Location`. |
| GET | `/appointments/{id}` | Retrieve the persisted appointment; `200 OK`. |

The [API contract](api/README.md) is confirmed: positive integer IDs, RFC 3339 input timestamps, UTC responses, and consistent errors with request IDs. Endpoint implementation is pending.

Catalog data will be seeded. A cURL client will demonstrate the workflow. Authentication, catalog administration, cancellation, rescheduling, temporary holds, and a separate availability endpoint are outside scope.

Start times must be in the future, include a timezone offset, and are normalized to UTC. The service type determines duration, and `[start, end)` intervals allow back-to-back appointments. Resources are available unless booked, without working-hours or maintenance schedules. Booking validates the seeded customer/vehicle relationship but does not verify customer identity. See [Architecture assumptions](architecture.md#assumptions).

## Technology choices

- Go with `net/http` and `http.ServeMux`.
- PostgreSQL with `pgx/v5` and `pgxpool`.
- `golang-migrate` with SQL migration files.
- `READ COMMITTED` transactions with a dealership row lock before checking availability.

Exact runtime and dependency versions will be pinned during setup. See [Architecture](architecture.md) for component responsibilities, data flow, and tradeoffs.

## Build, run, and test

The repository does not yet contain a Go module or runnable service. There are currently no executable build, migration, startup, or test commands.

The implementation will require Go, PostgreSQL, the migration CLI, and cURL. Verified setup instructions will be published here with the first runnable version, including configuration, migrations, sample data, build/run commands, and test commands.

See the [delivery plan](plan.md) for current progress and the [test plan](tests/README.md) for intended coverage. No automated application tests have run yet.

## Planned project structure

```text
.
|-- README.md                 # Project overview and usage
|-- plan.md                   # Delivery progress and decisions
|-- architecture.md           # System design and tradeoffs
|-- cmd/api/                  # Server entry point
|-- internal/
|   |-- httpapi/              # Routes, validation, and responses
|   |-- appointments/         # Booking rules and allocation
|   |-- catalog/              # Catalog models and qualifications
|   |-- postgres/             # SQL queries and transactions
|   `-- config/               # Application configuration
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

Implementation verification is still pending. Final quality will be assessed through business-rule tests, competing requests against PostgreSQL, and a clean-setup rehearsal. This section will report the actual refinements and results as implementation progresses.
