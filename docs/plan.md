# Delivery plan and progress

## Objective

Deliver the Unified Service Scheduler for Scenario A: a Go and PostgreSQL backend that confirms appointments only when a qualified technician and service bay are available for the entire service duration.

**Budget:** One week, approximately 15 hours of planned work and up to 5 hours of buffer.

## Current progress

| Area | Status | Evidence or remaining work |
| --- | --- | --- |
| Project structure | Complete | Documentation lists the delivered packages, SQL files, demo script, and integration tests. |
| Technology and concurrency choices | Selected | Recorded in [My decision](#my-decision). |
| System design | Documented | [Architecture](architecture.md) matches implemented components, transaction behavior, constraints, operational limits, and pinned versions. |
| Domain assumptions | Confirmed | Time, working hours, and customer/vehicle validation are documented in [Architecture](architecture.md#assumptions). |
| Scope freeze | Complete | Technology choices, domain assumptions, and the [API contract](../api/README.md) are confirmed. |
| Application environment | Complete | The Go service, PostgreSQL connection pool, health route, Compose services, persistent volume, and Make targets run successfully. |
| Schema and seed data | Complete | Versioned migrations, relational constraints, lookup indexes, deterministic catalog data, and schema verification are implemented. |
| Booking transaction | Complete | Validation, dealership locking, allocation, atomic commit, and rollback pass service-level tests. |
| Appointment HTTP API | Complete | Creation returns `201` and `Location`; retrieval returns the same persisted representation; errors carry request IDs. |
| Business-rule tests | Complete | Independent technician/bay interval checks, time precision, eligibility, alternatives, persisted associations, and input rejection pass. |
| Concurrency and rollback tests | Complete | Coordinated HTTP capacity checks, dealership independence, and recovery of a queued booking after commit failure pass. |
| Operational behavior | Complete | Correlated logs, appointment IDs, request/database deadlines, safe dependency errors, and recovery are verified. |
| Verification | Business-rule, HTTP, concurrency, and operational tests pass | Unit tests, PostgreSQL/HTTP integration tests, race detection, build, vet, cURL demonstration, and database outage/recovery checks pass. Final clean-setup rehearsal remains. |
| Documentation | Complete | README setup/verification commands, API examples, architecture, package notes, and both AI narratives match the delivered implementation. |
| Video and submission | Not completed | Recording and final repository checks follow implementation. |

Completed checkboxes record finished work. Unchecked items remain planned and do not imply implementation or successful verification.

## My decision

### Step 1: Scope and technology choices

I selected a single Go backend with PostgreSQL and a cURL client. The implementation focuses on resource allocation and persistent, concurrency-safe booking within the assignment time budget.

| Area | Decision | Reason |
| --- | --- | --- |
| HTTP | `net/http` and `http.ServeMux` | Keep the two-endpoint API within the Go standard library. |
| Database | PostgreSQL | Persist the relational booking model and coordinate transactions. |
| Driver | `pgx/v5` with `pgxpool` | Use explicit PostgreSQL queries, transactions, and pooled connections. |
| Migrations | `golang-migrate` with SQL files | Keep schema changes versioned and reviewable. |
| API | `POST /appointments` and `GET /appointments/{id}` | Demonstrate creation and retrieval of a persisted booking. |
| API contract | Positive integer IDs, RFC 3339 input, UTC responses, and one appointment representation | Keep requests and persisted results consistent. |
| HTTP outcomes | `201` with `Location` for creation; `200` for retrieval; `400/404/409/503/500` errors with request IDs | Separate invalid input, missing records, capacity conflicts, dependency failures, and unexpected errors. |
| Concurrency | `READ COMMITTED` plus a dealership row lock before availability checks | Serialize allocation within a dealership using one transaction boundary. |
| Client | No frontend; cURL test harness | Demonstrate the backend directly. |
| Domain | Seeded static catalog, confirmed-only appointments, server-side allocation | Focus implementation on the required booking flow. |
| Time | Future start times with a timezone offset, normalized to UTC; server-derived end time; `[start, end)` intervals | Define consistent time handling and allow back-to-back bookings. |
| Working hours | Resources are continuously available unless booked | Keep shifts, opening hours, breaks, holidays, and bay maintenance outside scope. |
| Customer and vehicle | Validate the seeded vehicle's association with the supplied customer | Enforce the reference-data relationship without authentication or identity verification. |

The concurrency tradeoff is reduced throughput within a dealership. All booking writers must follow the same locking protocol. The full data flow and limitations are documented in [architecture.md](architecture.md).

The domain assumptions and [API contract](../api/README.md) are confirmed. Runtime and dependency versions are pinned in `go.mod`, `Dockerfile`, and `compose.yaml`.

### Step 3: Schema boundaries

The schema uses composite foreign keys to enforce customer/vehicle ownership, dealership membership for technicians and bays, and technician/service qualification. Positive duration and appointment interval checks protect stored values. Resource overlap is a booking-transaction responsibility, enforced through the selected dealership lock before availability checks.

## Scope boundaries

The booking operation checks both resource availability and technician qualification, then persists customer, vehicle, dealership, service, technician, bay, and appointment times together.

Catalog administration, authentication, cancellation, rescheduling, temporary holds, idempotency, a separate availability endpoint, large benchmarks, caching, event processing, cloud deployment, and monitoring platforms are excluded. Client examples and local checks cover the demonstration and verification needs without adding OpenAPI or CI work.

## Delivery steps

### 1. Freeze scope - 30 minutes

- [x] Select the HTTP approach, database driver, migration approach, endpoints, and concurrency strategy.
- [x] Confirm time, working-hours, and customer/vehicle validation assumptions in the architecture document.
- [x] Finalize request/response schemas and error behavior in the API document.

**Completion evidence:** [Architecture](architecture.md) records assumptions, the transaction boundary, and limitations. The [API contract](../api/README.md) defines fields, success responses, and error behavior. Implementation and verification evidence are recorded in the following steps.

### 2. Start the environment - 90 minutes

- [x] Initialize the Go 1.27.1 module with `pgx/v5` 5.11.0 and select PostgreSQL 18.6 on Alpine 3.23.
- [x] Add the HTTP server, validated environment configuration, PostgreSQL connection pool, and database-backed health route.
- [x] Add application and database containers, a persistent PostgreSQL volume, example environment values, and Make targets.
- [x] Verify database connectivity, health reporting, local and container builds, static checks, data retention, and graceful shutdown.

**Completion evidence:** `make up` starts a healthy application and database; `GET /healthz` reports database connectivity. Local build, test command, vet, persistent-volume restart, and graceful shutdown checks pass.

### 3. Create schema and seed data - 90 minutes

- [x] Add an initial reversible SQL migration for entities and associations.
- [x] Add foreign keys, positive duration and interval checks, and resource lookup indexes.
- [x] Seed two dealerships, qualified and unqualified technicians, alternative bays, customers, vehicles, services, and one occupied resource pair.
- [x] Verify initialization on a fresh database, repeat initialization, and migration rollback and reapplication.

**Completion evidence:** `make db-verify` applies the migration, loads deterministic data, and verifies relationships, alternatives, constraints, and indexes. The same command passes on a fresh database and when repeated.

### 4. Implement safe booking - 2 hours

- [x] Validate references and the customer/vehicle association; require future start times, normalize them to UTC, and derive the end from the selected service duration.
- [x] Acquire the dealership lock before resource availability queries.
- [x] Select eligible resources for the entire interval, considering available alternatives.
- [x] Persist all associations atomically; handle no-capacity results and rollback.

**Completion evidence:** Unit and PostgreSQL integration tests verify persisted associations, UTC and duration handling, resource conflicts and alternatives, failed-commit rollback, and cancellation. Six blocked database sessions produce one success, five capacity conflicts, and one committed row. The integration suite also passes with the race detector.

### 5. Expose two HTTP endpoints - 1 hour

- [x] Implement appointment creation and retrieval.
- [x] Return `201` with `Location` for creation and `200` for retrieval; map `400/404/409/503/500` outcomes to the documented error envelope.
- [x] Add cURL examples with seeded IDs and a configurable future start time.

**Completion evidence:** `make demo` creates and retrieves identical appointment JSON using the returned `Location`. HTTP tests cover invalid requests, missing records, conflicts, and safe internal errors. Live database stop/restart checks verify `503` on both endpoints and successful retrieval after recovery.

### 6. Test business rules - 90 minutes

Business-rule coverage is complete. The interval matrix tests technicians and bays independently so one resource conflict cannot hide a missing check for the other.

- [x] Verify persisted associations on successful booking.
- [x] Reject technician and bay overlaps, including partial overlap and containment.
- [x] Allow adjacent intervals under the confirmed `[start, end)` rule.
- [x] Verify UTC normalization, required timezone offsets, server-derived end times, and rejection of non-future start times.
- [x] Exclude unqualified and wrong-dealership resources.
- [x] Select available alternatives when a candidate is occupied.
- [x] Reject invalid references, customer/vehicle mismatches, and invalid inputs without persisting appointments.

**Completion evidence:** Unit and PostgreSQL/HTTP integration tests pass with race detection. The matrix covers 13 interval cases per resource, including containment, adjacency in both directions, and microsecond overlaps. Additional checks cover equivalent timezone offsets, service-specific qualifications, independent resource alternatives, invalid IDs without writes, and persisted associations outside conventional opening hours. Build and static checks pass. See [test coverage](../tests/README.md#business-cases).

### 7. Test concurrency and rollback - 90 minutes

Concurrency and rollback coverage is complete. HTTP tests coordinate six independent database sessions for each resource combination and inspect committed data as well as responses.

- [x] Coordinate 5-10 requests with one eligible technician and bay for the interval.
- [x] Verify one success, remaining capacity conflicts, and one committed appointment.
- [x] Verify independent resource pairs can both be booked without false conflicts.
- [x] Verify rollback leaves no committed appointment and releases the lock.

**Completion evidence:** Six concurrent HTTP requests yield one `201` and five `409` responses with one pair, a shared technician, or a shared bay; two available pairs yield two `201` and four `409` responses. Stored rows contain no resource overlaps, and successful responses match retrieval. A different dealership commits while the first remains locked. A failed commit returns `500`, leaves no failed row, and releases its pair for a queued request. The full integration suite and five repeated runs of the new concurrency cases pass with race detection; build and static checks pass. See [concurrency coverage](../tests/README.md#concurrency-cases).

### 8. Add basic operational behavior - 30 minutes

Operational review is complete. Successful appointment logs include the persisted appointment ID, health failures include a diagnostic code, and PostgreSQL background connection attempts use the configured connection deadline.

- [x] Add request IDs and structured outcome/duration logs without customer contact data.
- [x] Bound request and database waits with timeouts.
- [x] Handle database unavailability without leaking raw errors.

**Completion evidence:** Unit tests verify correlated success/conflict/failure logs, omitted request data, bounded health checks, and stalled connection handshakes. PostgreSQL tests verify five-second booking-lock, retrieval-query, and pool-acquisition deadlines, safe `503` responses, no extra rows, and recovery. Live checks verify five-second header and ten-second body read limits, database outage/recovery, and correlated logs. The full suite passes with race detection; local/container builds and static checks pass.

### 9. Complete documentation - 90 minutes

- [x] Align architecture, data flow, technology rationale, and observability with implementation.
- [x] Explain guarantees, contention, assumptions, and limitations.
- [x] Publish exact build, run, migration, seed, and test instructions in the README.
- [x] Update both AI narratives with actual design and implementation evidence.

**Completion evidence:** README commands match Make targets, configuration, and prior runtime verification. Architecture records implemented components, the locking guarantee, contention, commit uncertainty, and operational limits. API examples match seeded relationships; package/test documents describe existing files. Both AI narratives are concise and evidence-based. Public Markdown passes ASCII, local-link, and anchor checks. Fresh-database rehearsal remains Step 10.

### 10. Rehearse from clean setup - 1 hour

- [ ] Follow the README against a fresh database and verify persistence across restart.
- [ ] Run the build, tests, and appropriate Go static checks.
- [ ] Rehearse successful booking, retrieval, conflict, and concurrency verification.
- [ ] Resolve blockers and stale documentation.

**Completion evidence:** The demonstration is reproducible without unwritten setup steps.

### 11. Record the video - 90 minutes

- [ ] Record a 5-10 minute walkthrough covering introduction, design, implementation, demonstration, challenges, and lessons.
- [ ] Include 1-2 minutes on AI guidance, review, and verification.
- [ ] Review the recording for readable text and audible narration.

**Completion evidence:** An accessible recording demonstrates the implemented behavior and explains its decisions.

### 12. Prepare and submit - 30 minutes

- [ ] Push final code, documentation, examples, and tests.
- [ ] Verify reviewer access to the repository and video.
- [ ] Submit both links before the deadline.

**Completion evidence:** All required artifacts are accessible and submitted.

## Schedule and contingency

| Day | Focus | Planned hours |
| --- | --- | --- |
| 1 | Steps 1-2: scope and environment | 2 |
| 2 | Steps 3-4: schema and safe booking | 3.5 |
| 3 | Steps 5-6: API and business tests | 2.5 |
| 4 | Steps 7-8: concurrency and operational behavior | 2 |
| 5 | Steps 9-10: documentation and rehearsal | 2.5 |
| 6 | Steps 11-12: video and submission | 2 |
| 7 | Buffer for defects, setup, or access issues | Up to 5 |

Core estimates total 14.5 hours, plus 0.5 hour for transitions and review. The schedule uses relative days within the submission deadline.

Feature scope freezes after day 4. Remaining time goes to correctness issues, reproducibility, documentation, and submission. Tests and the required video remain part of delivery if implementation takes longer than expected.

## Final readiness

- [ ] All three Scenario A acceptance criteria demonstrated.
- [ ] Concurrent requests cannot double-book through the supported API.
- [ ] REST API, PostgreSQL persistence, and business-rule tests work.
- [ ] Client examples are reproducible.
- [x] Architecture covers components, data flow, technologies, observability, and GenAI use.
- [x] README includes build/run/test instructions and a concise AI Collaboration Narrative.
- [ ] Video meets the 5-10 minute requirement, including 1-2 minutes on AI collaboration.
- [ ] Repository and video links are accessible and submitted on time.
