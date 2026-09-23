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
- `make down` stops containers while preserving PostgreSQL data.

Isolated integration-test reset and cURL demonstration commands will be added with their implementations.
