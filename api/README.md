# API contract

**Status:** Contract confirmed. The booking service and transaction are implemented; appointment HTTP endpoints, error mapping, and executable client examples are pending.

Request bodies and responses use JSON (`Content-Type: application/json`). Timestamps use RFC 3339 with a timezone offset, including `Z` for UTC. All returned appointment timestamps are normalized to UTC.

Catalog IDs are positive integers referencing seeded records. Appointment IDs are positive integers assigned by the server when appointments are created.

## POST /appointments

Create and immediately confirm an appointment. The server validates references and the customer/vehicle association, derives the service duration, allocates a qualified technician and a bay, and persists the appointment atomically.

### Request

```json
{
  "customer_id": 1,
  "vehicle_id": 1,
  "dealership_id": 1,
  "service_type_id": 1,
  "start_time": "2026-09-25T10:00:00+07:00"
}
```

| Field | Type | Rules |
| --- | --- | --- |
| `customer_id` | integer | Required, positive, and must exist. |
| `vehicle_id` | integer | Required, positive, must exist, and must belong to the supplied customer. |
| `dealership_id` | integer | Required, positive, and must exist. |
| `service_type_id` | integer | Required, positive, and must exist. |
| `start_time` | string | Required RFC 3339 timestamp with a timezone offset; must be in the future. |

Technician, bay, end time, duration, and status are server-controlled and are not request fields. Examples use illustrative dates and IDs; runnable demonstrations must use seeded IDs and a future start time.

### Success

Return `201 Created` with `Location: /appointments/{id}` and the appointment representation below.

```json
{
  "id": 123,
  "customer_id": 1,
  "vehicle_id": 1,
  "dealership_id": 1,
  "service_type_id": 1,
  "technician_id": 3,
  "service_bay_id": 2,
  "start_time": "2026-09-25T03:00:00Z",
  "end_time": "2026-09-25T04:00:00Z",
  "status": "CONFIRMED",
  "created_at": "2026-09-23T02:30:00Z"
}
```

This example assumes a 60-minute service. All fields are required: IDs are positive integers, timestamps are UTC RFC 3339 strings, and `status` is `CONFIRMED`. Success is returned only after the transaction commits.

## GET /appointments/{id}

Return `200 OK` with the same appointment representation used by POST. No request body is required.

The path ID must be a positive integer. An invalid ID returns `400 INVALID_REQUEST`; a valid ID with no matching appointment returns `404 APPOINTMENT_NOT_FOUND`.

## Error format

All API errors use this envelope:

```json
{
  "error": {
    "code": "NO_AVAILABLE_RESOURCES",
    "message": "No qualified technician and service bay are available for the requested time.",
    "request_id": "example-request-id"
  }
}
```

`code`, `message`, and `request_id` are required strings. Codes identify the error category; messages explain it to the reader. The same request ID appears in structured server logs. Internal SQL, database, stack-trace, and infrastructure details are never returned to clients.

## Error behavior

| HTTP status | Error code | Condition |
| --- | --- | --- |
| 400 | `INVALID_REQUEST` | Malformed JSON, missing or invalid fields, wrong field types, non-positive IDs, invalid timestamps, missing timezone offsets, or a start time that is not in the future. |
| 400 | `CUSTOMER_VEHICLE_MISMATCH` | The existing vehicle does not belong to the supplied customer. |
| 404 | `CUSTOMER_NOT_FOUND` | Referenced customer does not exist. |
| 404 | `VEHICLE_NOT_FOUND` | Referenced vehicle does not exist. |
| 404 | `DEALERSHIP_NOT_FOUND` | Referenced dealership does not exist. |
| 404 | `SERVICE_TYPE_NOT_FOUND` | Referenced service type does not exist. |
| 404 | `APPOINTMENT_NOT_FOUND` | Requested appointment does not exist. |
| 409 | `NO_AVAILABLE_RESOURCES` | No qualified technician and bay can be allocated for the full interval, including capacity lost to a competing booking. |
| 503 | `SERVICE_UNAVAILABLE` | A required dependency, such as PostgreSQL, is known to be unavailable. |
| 500 | `INTERNAL_ERROR` | Unexpected server failure not mapped to a known business or dependency error. |

The 409 response does not expose which resource prevented allocation. Waiting for a lock is not itself a capacity conflict; the booking operation checks capacity after acquiring the lock. Database unavailability and unexpected failures must not be reported as booking conflicts.

## Domain rules and scope

Intervals use `[start, end)`, allowing back-to-back bookings. Resource availability accounts for existing appointments, not shifts, opening hours, breaks, holidays, or maintenance schedules. Customer/vehicle association checks do not authenticate the caller or verify identity. See [Architecture assumptions](../docs/architecture.md#assumptions).

The API excludes availability browsing, catalog administration, cancellation, rescheduling, holds, authentication, and idempotency guarantees. Runnable cURL examples will be added with implementation.
