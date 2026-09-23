# Persistence plan

Selected tools: **PostgreSQL**, `pgx/v5` with `pgxpool`, and `golang-migrate` with SQL files. Status: proposed schema; migrations are not implemented. Exact versions remain to be pinned.

| Entity | Purpose |
| --- | --- |
| Customer | Customer associated with the appointment. |
| Vehicle | Seeded vehicle receiving service, associated with a customer. |
| Dealership | Location providing resources and service. |
| ServiceType | Service definition, duration, and qualification requirements. |
| Technician | Dealership technician eligible for allocation. |
| TechnicianQualification | Qualifications used to match a technician to a service. |
| ServiceBay | Dealership bay eligible for allocation. |
| Appointment | Confirmed booking linking customer, vehicle, service, dealership, technician, bay, and start/end times. |

## Schema work

- Define keys, relationships, validation constraints, and required indexes.
- Use positive integer IDs for catalog records and server-assigned positive integer appointment IDs, as defined by the [API contract](../api/README.md).
- Implement the selected dealership row lock before availability checks in a READ COMMITTED transaction. See [architecture.md](../architecture.md) for the protocol and limitations.
- Ensure both resource assignments and the appointment are persisted atomically.
- Preserve each seeded vehicle's customer association and validate it during booking.
- Store appointment instants consistently with application UTC normalization. Derive the end from the service duration and use `[start, end)` overlap checks.
- Follow the confirmed availability assumption: bookings occupy resources; shifts, opening hours, breaks, holidays, and bay maintenance are not modeled.
- Add versioned schema changes to `migrations/` and deterministic demo data to `seeds/`.

Sample data should include qualified and unqualified technicians, more than one dealership, occupied resources, and an alternate available resource pair. Include at least two customer/vehicle associations to test mismatches. Test data must remain isolated from demonstration data. See [Architecture assumptions](../architecture.md#assumptions).
