# Test coverage

Status: business-rule, concurrency, rollback, and operational coverage are complete. Unit and PostgreSQL/HTTP integration suites pass with race detection; build and static checks also pass. The new concurrency cases also pass five repeated runs with race detection.

## Run tests

```bash
make check
make test-integration
GOFLAGS=-race make test test-integration
go vet -tags=integration ./...
```

`make check` runs unit tests and static analysis; it excludes the integration build tag. Race detection requires CGO and a C compiler on a supported platform.

The integration target starts PostgreSQL and uses `TEST_DATABASE_URL` or the local `.env` database URL. Tests require schema-creation permission; each creates a unique schema, applies the real migration, loads independent fixtures, and removes the schema on cleanup. They do not reset demonstration tables. Integration tests use the `integration` build tag and fail if their database URL is missing.

Verified coverage includes persisted associations, independent resource alternatives, overlaps and containment for both resources, adjacency in both directions, microsecond boundaries, equivalent timezone offsets, service duration, invalid references and inputs, qualification and dealership filtering, and bookings outside conventional opening hours. Failed commit rollback and cancellation while waiting for the lock are also covered. HTTP contention tests wait until six database sessions are blocked, then verify outcomes and committed rows against the available capacity.

HTTP coverage verifies creation and identical retrieval, UTC fields, `Location`, malformed and oversized bodies, invalid IDs and media types, missing references, capacity conflicts, safe error envelopes, and request-log correlation. A real database commit failure returns `500` without a persisted row. Database error classification is unit-tested; a live PostgreSQL stop/restart rehearsal verified both endpoints return `503` and recover afterward.

Concurrent capacity checks, dealership independence, and queued recovery after a failed commit are covered below. The final clean-setup rehearsal is complete; video and submission remain in the [delivery plan](../docs/plan.md).

| Directory | Purpose |
| --- | --- |
| `../internal/**/*_test.go` | Input validation, HTTP contract behavior, and database error classification. |
| `integration/` | Persistence, atomicity, allocation, HTTP booking/retrieval, and concurrency against real PostgreSQL. |

## Business cases

The [business-rule integration tests](integration/business_rules_test.go) extend the [booking tests](integration/booking_test.go) and [HTTP tests](integration/http_test.go). Tests use future-relative appointments; exact current-time validation uses a fixed clock in [validation unit tests](../internal/appointments/booking_test.go).

| Rule | Verified evidence |
| --- | --- |
| Persist all booking associations | Successful booking is read back with the same customer, vehicle, dealership, service, technician, bay, status, and timestamps; a second set of catalog references is also exercised. |
| Reject overlap for either resource | `TestBookingIntervalMatrix` runs 13 cases separately for technicians and bays: identical intervals, partial overlaps, containment, shared endpoints, adjacency, gaps, and one-microsecond overlaps. Conflicts leave the original row unchanged. |
| Allow back-to-back bookings | Exact adjacency succeeds before and after an existing interval, reusing the same resources under `[start, end)`. |
| Normalize time and derive duration | Equivalent positive/negative offsets and UTC conflict consistently. Fractional input is truncated to PostgreSQL microseconds; service duration determines the end. Missing offsets, malformed timestamps, and non-future instants are rejected. |
| Require eligible resources | The technician must qualify for the selected service and belong to the dealership. A bay from another dealership cannot satisfy local capacity. |
| Use available alternatives | Tests occupy each first-choice resource separately and verify allocation of an alternative technician or bay; existing coverage also occupies both. |
| Reject invalid requests without writes | Missing references, customer/vehicle mismatches, and missing, null, zero, negative, fractional, string, or overflowing IDs leave no rows. Clients cannot supply resource assignments, end time, or status. |
| Honor continuous availability | A future Sunday at 02:00 UTC succeeds with a 90-minute service and can be retrieved with all associations intact. |

## HTTP contract cases

- Creation returns `201` with `Location`; retrieval returns `200` with the same appointment fields, positive integer IDs, and UTC RFC 3339 timestamps.
- Malformed JSON, missing or invalid fields, invalid IDs, invalid timestamps, and non-future starts return `400 INVALID_REQUEST`. Customer/vehicle mismatches return `400 CUSTOMER_VEHICLE_MISMATCH`.
- Missing catalog records or appointments return `404` with the documented entity-specific code.
- Insufficient capacity, including capacity consumed by a competing booking, returns `409 NO_AVAILABLE_RESOURCES`.
- Known PostgreSQL unavailability returns `503 SERVICE_UNAVAILABLE`; unexpected failures return `500 INTERNAL_ERROR`.
- Each error contains `code`, `message`, and `request_id` inside `error`, matches the request ID in logs, and exposes no internal database details.

The [API contract](../api/README.md) defines the full response shape and status mapping. These contracts are covered by handler unit tests and HTTP/PostgreSQL integration tests; live outage recovery was also rehearsed.

## Concurrency cases

[Concurrency integration tests](integration/concurrency_test.go) use independent PostgreSQL connections. The HTTP capacity matrix holds a dealership row lock until all six requests appear as lock waiters in `pg_stat_activity`, then releases them. This proves database contention before checking HTTP results, identical retrieval, row counts, and absence of overlapping resource allocations.

| Resource configuration | Expected and verified outcome for six requests |
| --- | --- |
| One qualified technician and one bay | One `201`, five `409`, one committed row. |
| One qualified technician and two bays | One `201`, five `409`, one committed row. |
| Two qualified technicians and one bay | One `201`, five `409`, one committed row. |
| Two qualified technicians and two bays | Two `201`, four `409`, two committed rows using distinct resources. |

`TestBookingOtherDealershipProceedsWhileLocked` confirms that another dealership can commit while the first dealership still has a waiting request. Releasing the first lock allows its request to commit as well.

`TestHTTPCommitFailureReleasesWaitingBooking` pauses the first request at commit using a test-only deferred trigger and advisory lock. A second request waits on the dealership row. Releasing the test lock fails the first commit: its HTTP response is `500`, its inserted row is rolled back, and the waiting request returns `201` using the same resource pair. Only the successful request remains in the database. These advisory locks exist only in test fixtures; application booking uses the dealership row lock.

Existing booking tests also verify cancellation while waiting for a lock and successful booking after cancellation. Tests cancel and join concurrent workers before cleanup, and temporary schemas are removed after each test.

Performance experiments are outside the delivery scope. These tests verify the supported booking protocol under coordinated contention; they are not a throughput benchmark.

## Operational cases

| Check | Verification |
| --- | --- |
| Safe request logs | HTTP unit tests verify request/response correlation, success appointment IDs, error codes, duration, severity, and exclusion of request bodies, query values, and raw error details. |
| Bounded health checks | A stalled pinger reaches its configured deadline and produces a safe health response and failure log. |
| Bounded connection establishment | A TCP peer accepts but never answers the PostgreSQL handshake. The pool returns within the test's deadline, including cleanup of its background connection attempt. |
| Database operation deadlines | [Operational integration tests](integration/operations_test.go) hold a dealership lock, block appointment reads, or exhaust the connection pool. Each request reaches its five-second deadline, returns a correlated `503`, leaves data unchanged, and succeeds after the obstruction is released. |
| Slow requests | A live container rehearsal confirmed incomplete headers terminate at five seconds and an incomplete JSON body returns `400` at ten seconds. |
| Database outage/recovery | A live PostgreSQL stop/restart rehearsal verified safe `503` responses on both appointment routes and health, correlated error logs, successful recovery, and unchanged demonstration appointments. |

The first four checks run in the automated suites. Slow-request and database stop/restart checks were performed against the running application; ordinary integration tests do not stop the shared database.

## Clean-setup rehearsal

A copy of the tracked source files was tested with a newly generated `.env` and an isolated Compose project and PostgreSQL volume. Separate ports protected the existing development environment. The README workflows passed without implementation fixes:

- First migration and seed, repeat initialization, migration version, and schema verification.
- Docker startup, health, and two cURL bookings using distinct resource pairs; a third request returned `409` without another row.
- Container removal and recreation while retaining the volume; both created appointments were retrieved unchanged.
- Local Go startup against the same database, retrieval, a new cURL booking, and graceful shutdown.
- Local/container builds, unit and full integration suites with race detection, static analysis including integration tests, and formatting.

Integration tests left no temporary schemas. Rehearsal containers, volume, and image were removed afterward. The existing development environment file and database volume were preserved.
