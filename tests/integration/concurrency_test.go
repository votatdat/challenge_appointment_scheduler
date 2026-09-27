//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/votatdat/challenge_appointment_scheduler/internal/appointments"
	"github.com/votatdat/challenge_appointment_scheduler/internal/httpapi"
)

func holdTestLock(t *testing.T, pool *pgxpool.Pool, query string, args ...any) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("release test lock: %v", err)
		}
	})
	if _, err := tx.Exec(t.Context(), query, args...); err != nil {
		t.Fatal(err)
	}
	return tx
}

// A goroutine start barrier alone does not prove database contention. Every
// waiter here is a distinct PostgreSQL backend in this test's isolated schema.
func waitForLockWaiters(t *testing.T, pool *pgxpool.Pool, schema string, want int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		err := pool.QueryRow(ctx, `
            SELECT count(*) FROM pg_stat_activity
            WHERE application_name=$1 AND state='active' AND wait_event_type='Lock'`, schema).Scan(&waiting)
		if err != nil {
			t.Fatalf("observe lock waiters: %v", err)
		}
		if waiting == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("only %d database sessions waiting; want %d", waiting, want)
		case <-ticker.C:
		}
	}
}

func assertNoResourceOverlaps(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var overlaps int
	err := pool.QueryRow(t.Context(), `
        SELECT count(*) FROM appointments a JOIN appointments b
          ON a.id < b.id AND a.start_time < b.end_time AND b.start_time < a.end_time
         AND (a.technician_id=b.technician_id OR a.service_bay_id=b.service_bay_id)
    `).Scan(&overlaps)
	if err != nil {
		t.Fatal(err)
	}
	if overlaps != 0 {
		t.Fatalf("committed %d overlapping resource allocations", overlaps)
	}
}

type concurrentHTTPResult struct {
	response httpResult
	err      error
}

func receiveHTTP(t *testing.T, ctx context.Context, results <-chan concurrentHTTPResult) httpResult {
	t.Helper()
	select {
	case result := <-results:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result.response
	case <-ctx.Done():
		t.Fatalf("waiting for HTTP response: %v", ctx.Err())
		return httpResult{}
	}
}

func assertCreatedRetrievable(t *testing.T, server *httptest.Server, result httpResult) {
	t.Helper()
	if result.status != http.StatusCreated || result.header.Get("Location") == "" {
		t.Fatalf("create: status=%d body=%s", result.status, result.body)
	}
	retrieved := callHTTP(t, server, "GET", result.header.Get("Location"), nil)
	if retrieved.status != http.StatusOK || !bytes.Equal(result.body, retrieved.body) {
		t.Fatalf("committed response differs from retrieval: %s", retrieved.body)
	}
}

func TestHTTPConcurrentResourceCapacity(t *testing.T) {
	for _, tc := range []struct {
		name, fixture string
		capacity      int
	}{
		{"one pair", `DELETE FROM technician_qualifications WHERE technician_id=2;
                      DELETE FROM service_bays WHERE id=2;`, 1},
		{"shared technician", "DELETE FROM technician_qualifications WHERE technician_id=2", 1},
		{"shared bay", "DELETE FROM service_bays WHERE id=2", 1},
		{"two independent pairs", "", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, service, schema := setup(t)
			if tc.fixture != "" {
				execSQL(t, pool, tc.fixture)
			}
			server := httptest.NewServer(httpapi.NewHandler(pool, time.Second, service, slog.New(slog.NewJSONHandler(io.Discard, nil))))
			t.Cleanup(server.Close)
			client := server.Client()
			client.Timeout = 10 * time.Second
			lock := holdTestLock(t, pool, "SELECT id FROM dealerships WHERE id=1 FOR UPDATE")
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			var workers sync.WaitGroup
			defer func() { cancel(); workers.Wait() }()
			const requests = 6
			results := make(chan concurrentHTTPResult, requests)
			body := httpInput()
			for range requests {
				workers.Add(1)
				go func() {
					defer workers.Done()
					result, err := requestHTTP(ctx, client, "POST", server.URL+"/appointments", body)
					results <- concurrentHTTPResult{result, err}
				}()
			}
			waitForLockWaiters(t, pool, schema, requests)
			if err := lock.Commit(t.Context()); err != nil {
				t.Fatal(err)
			}

			successes, conflicts := 0, 0
			requestIDs := make(map[string]bool)
			locations := make(map[string]bool)
			for range requests {
				result := receiveHTTP(t, ctx, results)
				requestID := result.header.Get("X-Request-ID")
				if requestID == "" || requestIDs[requestID] {
					t.Fatal("missing or reused request ID")
				}
				requestIDs[requestID] = true
				switch result.status {
				case http.StatusCreated:
					successes++
					assertCreatedRetrievable(t, server, result)
					location := result.header.Get("Location")
					if locations[location] {
						t.Fatal("two requests returned the same appointment")
					}
					locations[location] = true
				case http.StatusConflict:
					conflicts++
					checkHTTPError(t, result, http.StatusConflict, "NO_AVAILABLE_RESOURCES")
					if result.header.Get("Location") != "" {
						t.Fatal("conflict returned a Location")
					}
				default:
					t.Fatalf("unexpected response: %d %s", result.status, result.body)
				}
			}
			if successes != tc.capacity || conflicts != requests-tc.capacity || count(t, pool) != tc.capacity {
				t.Fatalf("success=%d conflicts=%d committed=%d; capacity=%d", successes, conflicts, count(t, pool), tc.capacity)
			}
			assertNoResourceOverlaps(t, pool)
		})
	}
}

func TestBookingOtherDealershipProceedsWhileLocked(t *testing.T) {
	pool, service, schema := setup(t)
	lock := holdTestLock(t, pool, "SELECT id FROM dealerships WHERE id=1 FOR UPDATE")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	var worker sync.WaitGroup
	defer func() { cancel(); worker.Wait() }()
	type bookingResult struct {
		appointment appointments.Appointment
		err         error
	}
	results := make(chan bookingResult, 1)
	in := input()
	worker.Add(1)
	go func() {
		defer worker.Done()
		got, err := service.Create(ctx, in)
		results <- bookingResult{got, err}
	}()
	waitForLockWaiters(t, pool, schema, 1)
	other := in
	other.DealershipID = 2
	got := mustBook(t, service, other)
	if got.DealershipID != 2 || got.TechnicianID != 4 || got.ServiceBayID != 3 || count(t, pool) != 1 {
		t.Fatalf("wrong independent booking: %+v", got)
	}
	// The other dealership has committed while the first is still waiting.
	waitForLockWaiters(t, pool, schema, 1)
	select {
	case result := <-results:
		t.Fatalf("locked dealership returned early: %+v", result)
	default:
	}
	if err := lock.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-results:
		if result.err != nil || result.appointment.DealershipID != 1 || result.appointment.TechnicianID != 1 || result.appointment.ServiceBayID != 1 {
			t.Fatalf("blocked booking failed after release: %+v", result)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if count(t, pool) != 2 {
		t.Fatal("both dealerships must have a committed appointment")
	}
	assertNoResourceOverlaps(t, pool)
}

func TestHTTPCommitFailureReleasesWaitingBooking(t *testing.T) {
	pool, service, schema := setup(t)
	execSQL(t, pool, `
        DELETE FROM technician_qualifications WHERE technician_id=2;
        DELETE FROM service_bays WHERE id=2;
        CREATE FUNCTION fail_first_commit() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN
            IF NEW.customer_id=1 THEN
                PERFORM pg_advisory_xact_lock(hashtextextended(TG_TABLE_SCHEMA,0));
                RAISE EXCEPTION 'injected commit failure' USING ERRCODE='23514';
            END IF;
            RETURN NEW;
        END;
        $$;
        CREATE CONSTRAINT TRIGGER fail_first_commit AFTER INSERT ON appointments
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_first_commit();
    `)
	server := httptest.NewServer(httpapi.NewHandler(pool, time.Second, service, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 10 * time.Second
	// Test-only advisory lock pauses the first request during COMMIT, after
	// INSERT and while it owns the dealership lock. The schema gives it a
	// distinct key so separate test runs do not share the gate.
	gate := holdTestLock(t, pool, "SELECT pg_advisory_xact_lock(hashtextextended(current_schema(),0))")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	failed := make(chan concurrentHTTPResult, 1)
	succeeded := make(chan concurrentHTTPResult, 1)
	first := httpInput()
	workers.Add(1)
	go func() {
		defer workers.Done()
		result, err := requestHTTP(ctx, client, "POST", server.URL+"/appointments", first)
		failed <- concurrentHTTPResult{result, err}
	}()
	waitForLockWaiters(t, pool, schema, 1)
	if count(t, pool) != 0 {
		t.Fatal("uncommitted appointment became visible")
	}
	second := httpInput()
	second["start_time"] = first["start_time"]
	second["customer_id"], second["vehicle_id"] = 2, 2
	workers.Add(1)
	go func() {
		defer workers.Done()
		result, err := requestHTTP(ctx, client, "POST", server.URL+"/appointments", second)
		succeeded <- concurrentHTTPResult{result, err}
	}()
	// One backend waits at COMMIT; another waits for the dealership row.
	waitForLockWaiters(t, pool, schema, 2)
	if err := gate.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	failure := receiveHTTP(t, ctx, failed)
	checkHTTPError(t, failure, http.StatusInternalServerError, "INTERNAL_ERROR")
	if failure.header.Get("Location") != "" || bytes.Contains(failure.body, []byte("injected")) {
		t.Fatalf("failed commit exposed a result or database details: %s", failure.body)
	}
	success := receiveHTTP(t, ctx, succeeded)
	assertCreatedRetrievable(t, server, success)
	var customerID, vehicleID, technicianID, bayID int64
	err := pool.QueryRow(t.Context(), "SELECT customer_id,vehicle_id,technician_id,service_bay_id FROM appointments").
		Scan(&customerID, &vehicleID, &technicianID, &bayID)
	if err != nil {
		t.Fatal(err)
	}
	if count(t, pool) != 1 || customerID != 2 || vehicleID != 2 || technicianID != 1 || bayID != 1 {
		t.Fatalf("waiting request did not reuse the released pair: customer=%d vehicle=%d technician=%d bay=%d", customerID, vehicleID, technicianID, bayID)
	}
	assertNoResourceOverlaps(t, pool)
}
