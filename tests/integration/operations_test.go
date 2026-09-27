//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/votatdat/challenge_appointment_scheduler/internal/httpapi"
)

// Log writes happen on the server goroutine, potentially after the client has
// read the response. A channel synchronizes inspection without a buffer race.
type logRecords chan []byte

func (records logRecords) Write(data []byte) (int, error) {
	records <- bytes.Clone(data)
	return len(data), nil
}

func TestHTTPOperationDeadlinesAndRecovery(t *testing.T) {
	for _, tc := range []struct{ name, method, lock string }{
		{"booking lock", "POST", "SELECT id FROM dealerships WHERE id=1 FOR UPDATE"},
		{"retrieval query", "GET", "LOCK TABLE appointments IN ACCESS EXCLUSIVE MODE"},
		{"pool acquisition", "GET", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, service, schema := setup(t)
			existing := mustBook(t, service, input())
			records := make(logRecords, 4)
			server := httptest.NewServer(httpapi.NewHandler(pool, time.Second, service, slog.New(slog.NewJSONHandler(records, nil))))
			t.Cleanup(server.Close)
			var release func()
			if tc.lock != "" {
				lock := holdTestLock(t, pool, tc.lock)
				release = func() {
					if err := lock.Rollback(t.Context()); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				var held []*pgxpool.Conn
				release = func() {
					for _, conn := range held {
						conn.Release()
					}
				}
				t.Cleanup(release)
				for range pool.Config().MaxConns {
					conn, err := pool.Acquire(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					held = append(held, conn)
				}
			}
			path := "/appointments/" + strconv.FormatInt(existing.ID, 10)
			var body any
			if tc.method == "POST" {
				path, body = "/appointments", httpInput()
			}
			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			var worker sync.WaitGroup
			defer func() { cancel(); worker.Wait() }()
			results := make(chan concurrentHTTPResult, 1)
			started := time.Now()
			worker.Add(1)
			go func() {
				defer worker.Done()
				result, err := requestHTTP(ctx, server.Client(), tc.method, server.URL+path, body)
				results <- concurrentHTTPResult{result, err}
			}()
			if tc.lock != "" {
				waitForLockWaiters(t, pool, schema, 1)
			}
			response := receiveHTTP(t, ctx, results)
			elapsed := time.Since(started)
			checkHTTPError(t, response, 503, "SERVICE_UNAVAILABLE")
			if elapsed < 4*time.Second || elapsed > 8*time.Second || response.header.Get("Location") != "" {
				t.Fatalf("unexpected deadline behavior: elapsed=%v headers=%v", elapsed, response.header)
			}
			select {
			case data := <-records:
				var record map[string]any
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				if record["request_id"] != response.header.Get("X-Request-ID") || record["status"] != float64(503) ||
					record["code"] != "SERVICE_UNAVAILABLE" || record["level"] != "ERROR" || record["appointment_id"] != nil {
					t.Fatalf("missing deadline diagnostics: %s", data)
				}
				if duration, ok := record["duration_ms"].(float64); !ok || duration < 4000 {
					t.Fatalf("wrong duration: %s", data)
				}
			case <-ctx.Done():
				t.Fatal("deadline response was not logged")
			}
			release()
			if count(t, pool) != 1 {
				t.Fatal("timed-out operation changed persisted data")
			}
			recovered := callHTTP(t, server, tc.method, path, body)
			if tc.method == "POST" {
				assertCreatedRetrievable(t, server, recovered)
				if count(t, pool) != 2 {
					t.Fatal("booking did not recover after timeout")
				}
			} else if recovered.status != 200 {
				t.Fatalf("retrieval did not recover: %d %s", recovered.status, recovered.body)
			}
		})
	}
}
