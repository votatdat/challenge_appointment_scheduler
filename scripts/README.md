# Development operations

The root Makefile provides the verified environment and database commands:

- `make env` creates `.env` from `.env.example` if missing.
- `make up` builds the app, applies pending migrations, and starts the application and PostgreSQL; run `make db-init` first to seed the demo.
- `make db-up` starts PostgreSQL.
- `make db-init` applies migrations and loads demonstration data.
- `make migrate-up`, `make migrate-down`, and `make migrate-version` manage schema versions.
- `make seed` upserts fixed demonstration records.
- `make db-verify` verifies seeded relationships, constraints, alternatives, and indexes.
- `make run` applies pending migrations and runs the API locally.
- `make build` writes `bin/scheduler-api`; `make test` runs unit tests, `make vet` runs static checks, and `make check` runs both. These exclude integration tests.
- `make test-integration` starts PostgreSQL and tests booking in temporary schemas, leaving demonstration data intact.
- `make demo` runs `scripts/demo.sh` against the running API, creating an appointment and retrieving its `Location`.
- `make down` stops containers while preserving PostgreSQL data.

Integration tests apply the schema migration and use their own fixtures and cleanup. The cURL demo accepts `START_TIME` and `BASE_URL`, uses seeded IDs, and defaults to one day ahead in UTC. Each successful run persists one appointment.

For local Go execution, stop the container app first with `make down` to free its port, then run `make db-init` and `make run`. See the [README](../README.md#build-run-and-test) for prerequisites, configuration, and verification commands.
