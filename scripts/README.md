# Development operations

The root Makefile provides the verified environment commands:

- `make up` builds and starts the application and PostgreSQL.
- `make db-up` starts PostgreSQL for local Go development.
- `make run` runs the API locally.
- `make build`, `make test`, `make vet`, and `make check` verify the current Go code.
- `make down` stops containers while preserving PostgreSQL data.

Migration, seed, isolated test reset, and cURL demonstration commands will be added with their implementations.
