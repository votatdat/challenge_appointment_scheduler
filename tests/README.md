# Test plan

Status: planned tests only. Test directories will be created with their first executable tests or fixtures.

| Directory | Purpose |
| --- | --- |
| `../internal/**/*_test.go` | Go unit tests alongside duration, interval, and qualification logic. |
| `integration/` | Persistence, atomicity, resource allocation, and concurrency against real PostgreSQL. |
| `e2e/` | HTTP requests, response contracts, and complete booking workflows. |
| `fixtures/` | Deterministic setup data shared where appropriate. |

## Business cases

- A qualified technician and bay are available for the full duration: booking succeeds.
- The technician or bay has an overlapping booking: the conflicting resource cannot be allocated.
- An appointment starts exactly when another ends: allowed under the confirmed `[start, end)` interval rule.
- Start timestamps without a timezone offset or with invalid values are rejected without creating an appointment.
- Different offset representations of the same instant normalize to the same UTC start and receive consistent overlap handling.
- The selected service type determines duration, and the server calculates the appointment end time.
- A start time at or before the validation clock's current instant is rejected. Use a controlled clock or stable future-relative fixtures to avoid date-dependent failures.
- A vehicle associated with the supplied customer is accepted; a customer/vehicle mismatch is rejected without creating an appointment.
- A future interval is not rejected solely for falling outside conventional opening hours when qualified resources are available; shift, holiday, break, and maintenance rules are not modeled.
- The technician is unqualified or belongs to another dealership: cannot be allocated.
- One candidate is occupied but an alternate pair is available: booking succeeds with eligible resources.
- A required record is missing or the input is invalid: reject without creating an appointment.
- A database write fails: no partial appointment remains.
- A booking succeeds: its customer, vehicle, service, dealership, technician, bay, and interval can be retrieved.

## HTTP contract cases

- Creation returns `201` with `Location`; retrieval returns `200` with the same appointment fields, positive integer IDs, and UTC RFC 3339 timestamps.
- Malformed JSON, missing or invalid fields, invalid IDs, invalid timestamps, and non-future starts return `400 INVALID_REQUEST`. Customer/vehicle mismatches return `400 CUSTOMER_VEHICLE_MISMATCH`.
- Missing catalog records or appointments return `404` with the documented entity-specific code.
- Insufficient capacity, including capacity consumed by a competing booking, returns `409 NO_AVAILABLE_RESOURCES`.
- Known PostgreSQL unavailability returns `503 SERVICE_UNAVAILABLE`; unexpected failures return `500 INTERNAL_ERROR`.
- Each error contains `code`, `message`, and `request_id` inside `error`, matches the request ID in logs, and exposes no internal database details.

The [API contract](../api/README.md) defines the full response shape and status mapping. These checks remain unimplemented.

## Concurrency cases

Coordinate independent requests so they compete for the same interval. With exactly one eligible technician and one bay, one booking should succeed and the other attempts should receive the documented conflict result. Inspect persisted appointments as well as HTTP responses.

Also cover a shared technician with different bays, a shared bay with different technicians, and independent resource pairs that can both succeed. Repeat relevant cases across independent PostgreSQL connections; mocked repositories cannot validate the database's concurrency behavior.

Performance experiments are outside the current delivery scope. Record actual test outcomes as implementation progresses; no result is claimed by this plan.
