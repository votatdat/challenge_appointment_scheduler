# Test plan

Status: business-rule coverage is complete. Unit and PostgreSQL/HTTP integration suites pass with race detection; build and static checks also pass. Additional concurrent allocation cases remain for Step 7.

## Run current tests

```bash
make test
make test-integration
```

The integration target starts PostgreSQL and uses `TEST_DATABASE_URL` or the local `.env` database URL. Tests require schema-creation permission; each creates a unique schema, applies the real migration, loads independent fixtures, and removes the schema on cleanup. They do not reset demonstration tables. Integration tests use the `integration` build tag and fail if their database URL is missing.

Verified coverage includes persisted associations, independent resource alternatives, overlaps and containment for both resources, adjacency in both directions, microsecond boundaries, equivalent timezone offsets, service duration, invalid references and inputs, qualification and dealership filtering, and bookings outside conventional opening hours. Failed commit rollback and cancellation while waiting for the lock are also covered. The contention test waits until six database sessions are blocked, then checks one committed booking and five capacity conflicts.

HTTP coverage verifies creation and identical retrieval, UTC fields, `Location`, malformed and oversized bodies, invalid IDs and media types, missing references, capacity conflicts, safe error envelopes, and request-log correlation. A real database commit failure returns `500` without a persisted row. Database error classification is unit-tested; a live PostgreSQL stop/restart rehearsal verified both endpoints return `503` and recover afterward.

Remaining concurrency review includes simultaneous successful independent pairs, a shared technician with multiple bays, a shared bay with multiple technicians, and cross-dealership requests. HTTP contention verification remains planned.

| Directory | Purpose |
| --- | --- |
| `../internal/**/*_test.go` | Input validation, HTTP contract behavior, and database error classification. |
| `integration/` | Persistence, atomicity, allocation, HTTP booking/retrieval, and concurrency against real PostgreSQL. |
| `e2e/` | Reserved for additional process-level workflows if needed; current HTTP tests use the integration setup. |
| `fixtures/` | Deterministic setup data shared where appropriate. |

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

Coordinate independent requests so they compete for the same interval. With exactly one eligible technician and one bay, one booking should succeed and the other attempts should receive the documented conflict result. Inspect persisted appointments as well as HTTP responses.

Also cover a shared technician with different bays, a shared bay with different technicians, and independent resource pairs that can both succeed. Repeat relevant cases across independent PostgreSQL connections; mocked repositories cannot validate the database's concurrency behavior.

Performance experiments are outside the current delivery scope. Current service-level test results are recorded above; remaining concurrency coverage will be added in Step 7.
