//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/votatdat/challenge_appointment_scheduler/internal/httpapi"
)

type httpResult struct {
	status int
	header http.Header
	body   []byte
}

func callHTTP(t *testing.T, server *httptest.Server, method, path string, body any) httpResult {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := server.Client()
	client.Timeout = 10 * time.Second
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return httpResult{status: response.StatusCode, header: response.Header, body: content}
}

func httpInput() map[string]any {
	in := input()
	return map[string]any{
		"customer_id": in.CustomerID, "vehicle_id": in.VehicleID, "dealership_id": in.DealershipID,
		"service_type_id": in.ServiceTypeID, "start_time": in.StartTime,
	}
}

func checkHTTPError(t *testing.T, result httpResult, status int, code string) {
	t.Helper()
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(result.body, &body); err != nil {
		t.Fatal(err)
	}
	if result.status != status || body.Error.Code != code || body.Error.Message == "" ||
		body.Error.RequestID == "" || body.Error.RequestID != result.header.Get("X-Request-ID") ||
		result.header.Get("Content-Type") != "application/json" {
		t.Fatalf("status=%d headers=%v body=%s", result.status, result.header, result.body)
	}
}

func TestHTTPCreateRetrieveAndConflict(t *testing.T) {
	pool, service, _ := setup(t)
	server := httptest.NewServer(httpapi.NewHandler(pool, time.Second, service, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	t.Cleanup(server.Close)
	body := httpInput()
	start, _ := time.Parse(time.RFC3339, body["start_time"].(string))
	body["start_time"] = start.In(time.FixedZone("UTC+7", 7*3600)).Format(time.RFC3339)
	created := callHTTP(t, server, "POST", "/appointments", body)
	if created.status != 201 || created.header.Get("Location") == "" ||
		created.header.Get("Content-Type") != "application/json" {
		t.Fatalf("create: %d %s", created.status, created.body)
	}
	var appointment map[string]any
	if err := json.Unmarshal(created.body, &appointment); err != nil {
		t.Fatal(err)
	}
	if len(appointment) != 11 || appointment["id"].(float64) <= 0 ||
		appointment["customer_id"] != float64(1) || appointment["vehicle_id"] != float64(1) ||
		appointment["dealership_id"] != float64(1) || appointment["service_type_id"] != float64(1) ||
		appointment["technician_id"] != float64(1) || appointment["service_bay_id"] != float64(1) ||
		appointment["status"] != "CONFIRMED" || appointment["start_time"] != start.Format(time.RFC3339) ||
		appointment["end_time"] != start.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("wrong appointment: %s", created.body)
	}
	if _, err := time.Parse(time.RFC3339Nano, appointment["created_at"].(string)); err != nil {
		t.Fatal(err)
	}
	// Verify a database row exists before the response is used for retrieval.
	if count(t, pool) != 1 {
		t.Fatal("create response arrived without committed data")
	}
	retrieved := callHTTP(t, server, "GET", created.header.Get("Location"), nil)
	var same map[string]any
	if err := json.Unmarshal(retrieved.body, &same); err != nil {
		t.Fatal(err)
	}
	if retrieved.status != 200 || !reflect.DeepEqual(appointment, same) {
		t.Fatalf("retrieval mismatch: %s", retrieved.body)
	}
	if retrieved.header.Get("X-Request-ID") == created.header.Get("X-Request-ID") {
		t.Fatal("request ID reused")
	}
	alternative := callHTTP(t, server, "POST", "/appointments", body)
	if alternative.status != 201 {
		t.Fatalf("alternative: %s", alternative.body)
	}
	checkHTTPError(t, callHTTP(t, server, "POST", "/appointments", body), 409, "NO_AVAILABLE_RESOURCES")
	if count(t, pool) != 2 {
		t.Fatal("conflict changed persisted appointments")
	}
	checkHTTPError(t, callHTTP(t, server, "GET", "/appointments/999999", nil), 404, "APPOINTMENT_NOT_FOUND")
	checkHTTPError(t, callHTTP(t, server, "GET", "/appointments/no-id", nil), 400, "INVALID_REQUEST")
}

func TestHTTPValidationAndMissingReferences(t *testing.T) {
	pool, service, _ := setup(t)
	server := httptest.NewServer(httpapi.NewHandler(pool, time.Second, service, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	t.Cleanup(server.Close)
	for _, tc := range []struct {
		name, field string
		value       any
		status      int
		code        string
	}{
		{"missing customer", "customer_id", 999, 404, "CUSTOMER_NOT_FOUND"},
		{"missing vehicle", "vehicle_id", 999, 404, "VEHICLE_NOT_FOUND"},
		{"missing dealership", "dealership_id", 999, 404, "DEALERSHIP_NOT_FOUND"},
		{"missing service", "service_type_id", 999, 404, "SERVICE_TYPE_NOT_FOUND"},
		{"mismatched vehicle", "vehicle_id", 2, 400, "CUSTOMER_VEHICLE_MISMATCH"},
		{"missing field", "start_time", nil, 400, "INVALID_REQUEST"},
		{"null start", "start_time", nil, 400, "INVALID_REQUEST"},
		{"invalid date", "start_time", "2099-02-30T10:00:00Z", 400, "INVALID_REQUEST"},
		{"no offset", "start_time", "2099-01-01T10:00:00", 400, "INVALID_REQUEST"},
		{"past time", "start_time", "2000-01-01T10:00:00Z", 400, "INVALID_REQUEST"},
		{"client-controlled technician", "technician_id", 1, 400, "INVALID_REQUEST"},
		{"client-controlled bay", "service_bay_id", 1, 400, "INVALID_REQUEST"},
		{"client-controlled end", "end_time", "2099-01-01T11:00:00Z", 400, "INVALID_REQUEST"},
		{"client-controlled status", "status", "CONFIRMED", 400, "INVALID_REQUEST"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := httpInput()
			body[tc.field] = tc.value
			if tc.name == "missing field" {
				delete(body, tc.field)
			}
			checkHTTPError(t, callHTTP(t, server, "POST", "/appointments", body), tc.status, tc.code)
			if count(t, pool) != 0 {
				t.Fatal("invalid request persisted data")
			}
		})
	}
}

func TestHTTPFailedCommitReturns500WithoutData(t *testing.T) {
	pool, service, _ := setup(t)
	execSQL(t, pool, `
		CREATE FUNCTION reject_http_booking() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'secret injected database error' USING ERRCODE='23514'; END;
		$$;
		CREATE CONSTRAINT TRIGGER reject_http_booking AFTER INSERT ON appointments
		DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_http_booking();
	`)
	server := httptest.NewServer(httpapi.NewHandler(pool, time.Second, service, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	t.Cleanup(server.Close)
	result := callHTTP(t, server, "POST", "/appointments", httpInput())
	checkHTTPError(t, result, 500, "INTERNAL_ERROR")
	if bytes.Contains(result.body, []byte("secret")) {
		t.Fatalf("database error leaked: %s", result.body)
	}
	if result.header.Get("Location") != "" || count(t, pool) != 0 {
		t.Fatal("failed commit appeared successful")
	}
}

func TestHTTPRejectsInvalidIDsWithoutPersistence(t *testing.T) {
	pool, service, _ := setup(t)
	server := httptest.NewServer(httpapi.NewHandler(pool, time.Second, service, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	t.Cleanup(server.Close)
	for _, field := range []string{"customer_id", "vehicle_id", "dealership_id", "service_type_id"} {
		for _, tc := range []struct {
			name  string
			value any
		}{
			{"missing", nil}, {"null", nil}, {"zero", 0}, {"negative", -1},
			{"fractional", 1.5}, {"string", "1"}, {"overflow", uint64(1) << 63},
		} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				body := httpInput()
				body[field] = tc.value
				if tc.name == "missing" {
					delete(body, field)
				}
				result := callHTTP(t, server, "POST", "/appointments", body)
				checkHTTPError(t, result, 400, "INVALID_REQUEST")
				if result.header.Get("Location") != "" || count(t, pool) != 0 {
					t.Fatal("invalid ID produced a persisted appointment or Location")
				}
			})
		}
	}
	// Rejected inputs must not prevent a subsequent valid booking.
	result := callHTTP(t, server, "POST", "/appointments", httpInput())
	if result.status != 201 || count(t, pool) != 1 {
		t.Fatalf("valid booking after invalid input failed: %d %s", result.status, result.body)
	}
}
