package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/votatdat/challenge_appointment_scheduler/internal/appointments"
)

type stubService struct {
	result appointments.Appointment
	err    error
	calls  int
}

func (s *stubService) Create(context.Context, appointments.CreateInput) (appointments.Appointment, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubService) Get(context.Context, int64) (appointments.Appointment, error) {
	s.calls++
	return s.result, s.err
}

type healthyDB struct{}

func (healthyDB) Ping(context.Context) error { return nil }

func handlerFor(service *stubService, logs io.Writer) http.Handler {
	return NewHandler(healthyDB{}, time.Second, service, slog.New(slog.NewJSONHandler(logs, nil)))
}

func request(handler http.Handler, method, path, body, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("X-Request-ID", "untrusted-client-value")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	// Use the public JSON field names, including the underscore.
	var wire struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	if rec.Code != status || wire.Error.Code != code || wire.Error.Message == "" ||
		wire.Error.RequestID == "" || wire.Error.RequestID != rec.Header().Get("X-Request-ID") ||
		wire.Error.RequestID == "untrusted-client-value" || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected response: status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("internal error leaked: %s", rec.Body.String())
	}
}

func TestErrorMappingAndRequestLogs(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{appointments.ErrInvalidRequest, 400, "INVALID_REQUEST"},
		{appointments.ErrCustomerVehicleMismatch, 400, "CUSTOMER_VEHICLE_MISMATCH"},
		{appointments.ErrCustomerNotFound, 404, "CUSTOMER_NOT_FOUND"},
		{appointments.ErrVehicleNotFound, 404, "VEHICLE_NOT_FOUND"},
		{appointments.ErrDealershipNotFound, 404, "DEALERSHIP_NOT_FOUND"},
		{appointments.ErrServiceTypeNotFound, 404, "SERVICE_TYPE_NOT_FOUND"},
		{appointments.ErrAppointmentNotFound, 404, "APPOINTMENT_NOT_FOUND"},
		{appointments.ErrNoAvailableResources, 409, "NO_AVAILABLE_RESOURCES"},
		{appointments.ErrUnavailable, 503, "SERVICE_UNAVAILABLE"},
		{errors.New("secret SQL and credentials"), 500, "INTERNAL_ERROR"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			for _, method := range []string{"POST", "GET"} {
				logs := &bytes.Buffer{}
				stub := &stubService{err: fmt.Errorf("secret details: %w", tc.err)}
				path := "/appointments"
				if method == "GET" {
					path += "/1"
				}
				rec := request(handlerFor(stub, logs), method, path, `{"customer_id":1,"vehicle_id":1,"dealership_id":1,"service_type_id":1,"start_time":"2099-01-01T00:00:00Z"}`, "application/json")
				assertError(t, rec, tc.status, tc.code)
				var log map[string]any
				if err := json.Unmarshal(logs.Bytes(), &log); err != nil {
					t.Fatal(err)
				}
				wantLevel := "INFO"
				if tc.status >= 500 {
					wantLevel = "ERROR"
				}
				if log["level"] != wantLevel || log["method"] != method || log["appointment_id"] != nil {
					t.Fatalf("wrong error diagnostics: %s", logs)
				}
				if duration, ok := log["duration_ms"].(float64); !ok || duration < 0 {
					t.Fatalf("missing request duration: %s", logs)
				}
				if log["request_id"] != rec.Header().Get("X-Request-ID") || log["code"] != tc.code ||
					log["status"] != float64(tc.status) || strings.Contains(logs.String(), "secret") {
					t.Fatalf("unexpected log: %s", logs)
				}
			}
		})
	}
}

func TestMalformedBodyDoesNotCallService(t *testing.T) {
	for _, tc := range []struct{ name, body, contentType string }{
		{"empty", "", "application/json"},
		{"syntax", "{", "application/json"},
		{"array", "[]", "application/json"},
		{"null", "null", "application/json"},
		{"wrong type", `{"customer_id":"1"}`, "application/json"},
		{"fractional ID", `{"customer_id":1.5}`, "application/json"},
		{"overflow", `{"customer_id":9223372036854775808}`, "application/json"},
		{"unknown field", `{"technician_id":1}`, "application/json"},
		{"extra value", "{} {}", "application/json"},
		{"extra null", "{} null", "application/json"},
		{"too large", `{"start_time":"` + strings.Repeat("x", 65536) + `"}`, "application/json"},
		{"missing type", "{}", ""},
		{"wrong media type", "{}", "text/plain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubService{}
			rec := request(handlerFor(stub, io.Discard), "POST", "/appointments", tc.body, tc.contentType)
			assertError(t, rec, 400, "INVALID_REQUEST")
			if stub.calls != 0 {
				t.Fatal("invalid JSON reached service")
			}
		})
	}
}

func TestPathAndMethodValidation(t *testing.T) {
	for _, id := range []string{"0", "-1", "+1", "abc", "1.5", "9223372036854775808"} {
		t.Run(id, func(t *testing.T) {
			stub := &stubService{}
			rec := request(handlerFor(stub, io.Discard), "GET", "/appointments/"+id, "", "")
			assertError(t, rec, 400, "INVALID_REQUEST")
			if stub.calls != 0 {
				t.Fatal("invalid ID reached service")
			}
		})
	}
	stub := &stubService{}
	handler := handlerFor(stub, io.Discard)
	rec := request(handler, "DELETE", "/appointments/1", "", "")
	assertError(t, rec, 405, "METHOD_NOT_ALLOWED")
	if rec.Header().Get("Allow") != "GET" {
		t.Fatal("missing Allow")
	}
	assertError(t, request(handler, "GET", "/missing", "", ""), 404, "ROUTE_NOT_FOUND")
}

func TestCreateAndGetHaveSameRepresentation(t *testing.T) {
	start := time.Date(2099, 1, 1, 10, 0, 0, 123456000, time.FixedZone("offset", 7*3600))
	stub := &stubService{result: appointments.Appointment{
		ID: 42, CustomerID: 1, VehicleID: 1, DealershipID: 1, ServiceTypeID: 1,
		TechnicianID: 2, ServiceBayID: 2, StartTime: start, EndTime: start.Add(time.Hour),
		Status: "CONFIRMED", CreatedAt: start.Add(-time.Hour),
	}}
	h := handlerFor(stub, io.Discard)
	post := request(h, "POST", "/appointments", "{}", "application/json; charset=utf-8")
	get := request(h, "GET", "/appointments/42", "", "")
	if post.Code != 201 || post.Header().Get("Location") != "/appointments/42" || get.Code != 200 ||
		!bytes.Equal(post.Body.Bytes(), get.Body.Bytes()) {
		t.Fatalf("POST %s GET %s", post.Body, get.Body)
	}
	var result map[string]any
	if err := json.Unmarshal(post.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 11 {
		t.Fatalf("unexpected fields: %v", result)
	}
	for _, key := range []string{"start_time", "end_time", "created_at"} {
		if !strings.HasSuffix(result[key].(string), "Z") {
			t.Fatalf("%s is not UTC", key)
		}
	}
}

func TestSuccessfulRequestLogs(t *testing.T) {
	for _, method := range []string{"POST", "GET"} {
		t.Run(method, func(t *testing.T) {
			logs := &bytes.Buffer{}
			service := &stubService{result: appointments.Appointment{ID: 42}}
			path, route, status := "/appointments", "/appointments", 201
			if method == "GET" {
				path, route, status = "/appointments/42", "/appointments/{id}", 200
			}
			rec := request(handlerFor(service, logs), method, path+"?email=private@example.test", `{"start_time":"private-body-value"}`, "application/json")
			var record map[string]any
			if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if rec.Code != status || record["status"] != float64(status) || record["appointment_id"] != float64(42) ||
				record["request_id"] != rec.Header().Get("X-Request-ID") || record["method"] != method ||
				record["route"] != route || record["level"] != "INFO" || record["code"] != "" {
				t.Fatalf("wrong success diagnostics: %s", logs)
			}
			if duration, ok := record["duration_ms"].(float64); !ok || duration < 0 {
				t.Fatalf("missing duration: %s", logs)
			}
			for _, forbidden := range []string{"private", "untrusted-client-value", "/appointments/42", "customer_id", "vehicle_id"} {
				if strings.Contains(logs.String(), forbidden) {
					t.Fatalf("request data leaked: %s", logs)
				}
			}
		})
	}
}

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestHealthDeadlineAndSafeLog(t *testing.T) {
	logs := &bytes.Buffer{}
	var pingErr error
	database := pingFunc(func(ctx context.Context) error {
		<-ctx.Done()
		pingErr = ctx.Err()
		return errors.New("secret database connection details")
	})
	handler := NewHandler(database, 20*time.Millisecond, &stubService{}, slog.New(slog.NewJSONHandler(logs, nil)))
	start := time.Now()
	rec := request(handler, "GET", "/healthz", "", "")
	if !errors.Is(pingErr, context.DeadlineExceeded) || time.Since(start) > time.Second || rec.Code != 503 {
		t.Fatalf("health deadline failed: %v %s", pingErr, rec.Body)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["database"] != "down" || body["status"] != "unavailable" {
		t.Fatalf("health response: %s", rec.Body)
	}
	var record map[string]any
	if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["code"] != "SERVICE_UNAVAILABLE" || record["level"] != "ERROR" || record["status"] != float64(503) ||
		record["request_id"] != rec.Header().Get("X-Request-ID") || strings.Contains(logs.String()+rec.Body.String(), "secret") {
		t.Fatalf("unsafe or incomplete health diagnostics: %s %s", logs, rec.Body)
	}
}
