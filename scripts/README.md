# Development operations

The root Makefile provides the verified environment and database commands:

- `make up` applies pending migrations and starts the application and PostgreSQL.
- `make db-up` starts PostgreSQL.
- `make db-init` applies migrations and loads demonstration data.
- `make migrate-up`, `make migrate-down`, and `make migrate-version` manage schema versions.
- `make seed` reloads deterministic demonstration data.
- `make db-verify` verifies seeded relationships, constraints, alternatives, and indexes.
- `make run` applies pending migrations and runs the API locally.
- `make build`, `make test`, `make vet`, and `make check` verify the current Go code.
- `make test-integration` starts PostgreSQL and tests booking in temporary schemas, leaving demonstration data intact.
- `make demo` runs `scripts/demo.sh` against the running API, creating an appointment and retrieving its `Location`.
- `make down` stops containers while preserving PostgreSQL data.

Integration tests apply the schema migration and use their own fixtures and cleanup. The cURL demo accepts `START_TIME` and `BASE_URL`, uses seeded IDs, and defaults to one day ahead in UTC. Each successful run persists one appointment.
