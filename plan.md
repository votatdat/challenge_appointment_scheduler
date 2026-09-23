# Delivery plan and progress

## Objective

Deliver the Unified Service Scheduler for Scenario A: a Go and PostgreSQL backend that confirms appointments only when a qualified technician and service bay are available for the entire service duration.

**Budget:** One week, approximately 15 hours of planned work and up to 5 hours of buffer.

## Current progress

| Area | Status | Evidence or remaining work |
| --- | --- | --- |
| Project structure | Planned | The README describes the layout; implementation directories will be created with their first files. |
| Technology and concurrency choices | Selected | Recorded in [My decision](#my-decision). |
| System design | Design recorded | [Architecture](architecture.md) covers confirmed decisions; versions and physical schema details will be finalized during implementation. |
| Domain assumptions | Confirmed | Time, working hours, and customer/vehicle validation are documented in [Architecture](architecture.md#assumptions). |
| Scope freeze | Complete | Technology choices, domain assumptions, and the [API contract](api/README.md) are confirmed. |
| Application and database | Not implemented | Go module, server, migrations, and seed data remain to be created. |
| Verification | Not run | [Test plan](tests/README.md) defines the intended coverage. |
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

The domain assumptions and [API contract](api/README.md) are confirmed, completing Step 1. Exact versions will be pinned during setup. These choices describe the intended implementation, not completed application behavior.

## Scope boundaries

The booking operation checks both resource availability and technician qualification, then persists customer, vehicle, dealership, service, technician, bay, and appointment times together.

Catalog administration, authentication, cancellation, rescheduling, temporary holds, idempotency, a separate availability endpoint, large benchmarks, caching, event processing, cloud deployment, and monitoring platforms are excluded. Client examples and local checks cover the demonstration and verification needs without adding OpenAPI or CI work.

## Delivery steps

### 1. Freeze scope - 30 minutes

- [x] Select the HTTP approach, database driver, migration approach, endpoints, and concurrency strategy.
- [x] Confirm time, working-hours, and customer/vehicle validation assumptions in the architecture document.
- [x] Finalize request/response schemas and error behavior in the API document.

**Completion evidence:** [Architecture](architecture.md) records assumptions, the transaction boundary, and limitations. The [API contract](api/README.md) defines fields, success responses, and error behavior. Step 1 is complete; implementation starts with Step 2.

### 2. Start the environment - 90 minutes

- [ ] Set up Go and PostgreSQL; initialize the Go module and pin versions.
- [ ] Add a minimal server, configuration, and database connection.
- [ ] Configure persistent database storage and document startup.
- [ ] Verify connectivity and clean shutdown.

**Completion evidence:** Startup succeeds using the documented instructions.

### 3. Create schema and seed data - 90 minutes

- [ ] Add an initial SQL migration for entities and associations.
- [ ] Add foreign keys, positive duration/interval checks, and relevant lookup indexes.
- [ ] Seed two dealerships, qualified/unqualified technicians, bays, customers, vehicles, and services.
- [ ] Verify initialization on a fresh database.

**Completion evidence:** Repeatable data setup supports successful booking, invalid resource selection, and alternative available resources.

### 4. Implement safe booking - 2 hours

- [ ] Validate references and the customer/vehicle association; require future start times, normalize them to UTC, and derive the end from the selected service duration.
- [ ] Acquire the dealership lock before resource availability queries.
- [ ] Select eligible resources for the entire interval, considering available alternatives.
- [ ] Persist all associations atomically; handle no-capacity results and rollback.

**Completion evidence:** The booking operation creates valid appointments and rejects conflicting assignments.

### 5. Expose two HTTP endpoints - 1 hour

- [ ] Implement appointment creation and retrieval.
- [ ] Return `201` with `Location` for creation and `200` for retrieval; map `400/404/409/503/500` outcomes to the documented error envelope.
- [ ] Add cURL examples with seeded IDs and a configurable future start time.

**Completion evidence:** The client can create and retrieve the same complete appointment.

### 6. Test business rules - 90 minutes

- [ ] Verify persisted associations on successful booking.
- [ ] Reject technician and bay overlaps, including partial overlap and containment.
- [ ] Allow adjacent intervals under the confirmed `[start, end)` rule.
- [ ] Verify UTC normalization, required timezone offsets, server-derived end times, and rejection of non-future start times.
- [ ] Exclude unqualified and wrong-dealership resources.
- [ ] Select available alternatives when a candidate is occupied.
- [ ] Reject invalid references, customer/vehicle mismatches, and invalid inputs without persisting appointments.

**Completion evidence:** Focused unit and PostgreSQL integration tests pass for the documented rules.

### 7. Test concurrency and rollback - 90 minutes

- [ ] Coordinate 5-10 requests with one eligible technician and bay for the interval.
- [ ] Verify one success, remaining capacity conflicts, and one committed appointment.
- [ ] Verify independent resource pairs can both be booked without false conflicts.
- [ ] Verify rollback leaves no committed appointment and releases the lock.

**Completion evidence:** Tests use independent PostgreSQL connections and inspect committed data. A one-connection pool is insufficient to verify the locking protocol.

### 8. Add basic operational behavior - 30 minutes

- [ ] Add request IDs and structured outcome/duration logs without customer contact data.
- [ ] Bound request and database waits with timeouts.
- [ ] Handle database unavailability without leaking raw errors.

**Completion evidence:** Success, conflict, and failure are distinguishable in logs; failure paths terminate predictably.

### 9. Complete documentation - 90 minutes

- [ ] Align architecture, data flow, technology rationale, and observability with implementation.
- [ ] Explain guarantees, contention, assumptions, and limitations.
- [ ] Publish exact build, run, migration, seed, and test instructions in the README.
- [ ] Update both AI narratives with actual design and implementation evidence.

**Completion evidence:** Reader-facing documents describe the delivered system and distinguish any remaining limitations.

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

Core estimates total 14.5 hours, plus 0.5 hour for transitions and review. Actual effort will be recorded as work progresses. The schedule uses relative days within the submission deadline.

Feature scope freezes after day 4. Remaining time goes to correctness issues, reproducibility, documentation, and submission. Tests and the required video remain part of delivery if implementation takes longer than expected.

## Final readiness

- [ ] All three Scenario A acceptance criteria demonstrated.
- [ ] Concurrent requests cannot double-book through the supported API.
- [ ] REST API, PostgreSQL persistence, and business-rule tests work.
- [ ] Client examples are reproducible.
- [ ] Architecture covers components, data flow, technologies, observability, and GenAI use.
- [ ] README includes build/run/test instructions and a concise AI Collaboration Narrative.
- [ ] Video meets the 5-10 minute requirement, including 1-2 minutes on AI collaboration.
- [ ] Repository and video links are accessible and submitted on time.
