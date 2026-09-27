# PostgreSQL persistence

The database uses PostgreSQL 18.6, `golang-migrate` 4.19.1, and versioned SQL files. The initial migration and deterministic demonstration data are implemented.

## Data model

| Entity | Stored relationship |
| --- | --- |
| Customer | Owns one or more seeded vehicles. |
| Vehicle | Belongs to exactly one customer. |
| Dealership | Owns technicians and service bays and forms the booking lock boundary. |
| ServiceType | Defines a positive duration in minutes. |
| Technician | Belongs to one dealership. |
| TechnicianQualification | Links a technician to a service type they can perform. |
| ServiceBay | Belongs to one dealership. |
| Appointment | Links the customer, vehicle, dealership, service, qualified technician, bay, and positive time interval. |

All IDs are positive `bigint` identity values. Appointment instants use `timestamptz`, and only `CONFIRMED` appointments are stored. Composite foreign keys prevent customer/vehicle mismatches, cross-dealership technician or bay assignments, and unqualified technician assignments.

Indexes support lookups by customer, dealership, qualification, and appointment intervals by technician or bay. Overlap checks run inside the implemented dealership-locked booking transaction described in [architecture.md](../docs/architecture.md).

## Commands

```bash
make db-init          # Apply migrations and load demonstration data
make migrate-up       # Apply pending migrations
make migrate-version  # Show the current migration version
make seed             # Upsert deterministic demonstration data
make db-verify        # Verify relationships, constraints, and indexes
make migrate-down     # Revert the latest migration and its schema data
```

`make db-init`, `make seed`, and `make db-verify` all apply migrations and upsert fixed demonstration records. Repeating them restores those records, including appointment 1, while preserving other appointment IDs. Use them to initialize demonstration data before booking. Migration execution reports `no change` when the schema is current.

`make up` applies migrations but does not seed; run `make db-init` first for the cURL demo. `make migrate-down` is destructive: reverting the initial migration drops all application tables. `make down` only stops containers and retains the named database volume.

## Demonstration data

| IDs | Data |
| --- | --- |
| Dealerships 1-2 | Central Service Centre and Riverside Service Centre. |
| Customers 1-2 | Alice Nguyen and Ben Carter. |
| Vehicles 1-3 | Vehicles 1 and 3 belong to customer 1; vehicle 2 belongs to customer 2. |
| Services 1-3 | Oil Change (60 minutes), Brake Inspection (90), and Wheel Alignment (45). |
| Technicians 1-5 | Dealership-specific qualifications; technician 3 is intentionally unqualified. |
| Bays 1-4 | Two bays at each dealership. |
| Appointment 1 | Technician 1 and bay 1 occupied from 10:00 to 11:00 UTC on January 15, 2099. |

At dealership 1, technicians 1 and 2 can both perform service 1, and bays 1 and 2 provide an alternative resource pair. This supports deterministic success, occupied-resource, unqualified-resource, wrong-dealership, and customer/vehicle mismatch scenarios.
