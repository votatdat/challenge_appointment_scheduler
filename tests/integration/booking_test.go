//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/votatdat/challenge_appointment_scheduler/internal/appointments"
	"github.com/votatdat/challenge_appointment_scheduler/internal/postgres"
)

// Every test uses the real migration in its own schema. No public tables or
// demonstration data are modified, and failures still clean up the schema.
func setup(t *testing.T) (*pgxpool.Pool, *appointments.Service, string) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL is required for integration tests")
	}
	ctx := t.Context()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("booking_test_%x", randomBytes(t))
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		defer admin.Close()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+quotedSchema+" CASCADE"); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 10
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	cfg.ConnConfig.RuntimeParams["application_name"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	migration, err := os.ReadFile(filepath.Join("..", "..", "database", "migrations", "000001_initial_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	execSQL(t, pool, string(migration))
	execSQL(t, pool, `
		INSERT INTO dealerships(id, name) VALUES (1,'Central'),(2,'Riverside');
		INSERT INTO customers(id, full_name) VALUES (1,'Customer One'),(2,'Customer Two');
		INSERT INTO vehicles(id, customer_id, registration_number, make, model)
			VALUES (1,1,'TEST-1','Toyota','Corolla'),(2,2,'TEST-2','Ford','Focus');
		INSERT INTO service_types(id,name,duration_minutes) VALUES (1,'Oil Change',60),(2,'Inspection',90);
		INSERT INTO technicians(id,dealership_id,full_name)
			VALUES (1,1,'First'),(2,1,'Alternative'),(3,1,'Unqualified'),(4,2,'Other dealership');
		INSERT INTO technician_qualifications VALUES (1,1),(1,2),(2,1),(4,1);
		INSERT INTO service_bays(id,dealership_id,name) VALUES (1,1,'Bay One'),(2,1,'Bay Two'),(3,2,'Bay Three');
	`)
	return pool, appointments.NewService(postgres.NewAppointmentStore(pool)), schema
}

func randomBytes(t *testing.T) []byte {
	t.Helper()
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return value
}

func execSQL(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func input() appointments.CreateInput {
	return appointments.CreateInput{
		CustomerID: 1, VehicleID: 1, DealershipID: 1, ServiceTypeID: 1,
		StartTime: time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Hour).Format(time.RFC3339),
	}
}

func count(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM appointments").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func mustBook(t *testing.T, service *appointments.Service, in appointments.CreateInput) appointments.Appointment {
	t.Helper()
	got, err := service.Create(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestBookingPersistsAndSelectsAlternatives(t *testing.T) {
	pool, service, _ := setup(t)
	in := input()
	start, _ := time.Parse(time.RFC3339, in.StartTime)
	in.StartTime = start.In(time.FixedZone("UTC+7", 7*60*60)).Format(time.RFC3339)
	first := mustBook(t, service, in)
	if first.ID <= 0 || first.CustomerID != 1 || first.VehicleID != 1 || first.DealershipID != 1 ||
		first.ServiceTypeID != 1 || first.TechnicianID != 1 || first.ServiceBayID != 1 ||
		first.Status != "CONFIRMED" || first.CreatedAt.IsZero() ||
		!first.StartTime.Equal(start) || !first.EndTime.Equal(start.Add(time.Hour)) ||
		first.StartTime.Location() != time.UTC || first.EndTime.Location() != time.UTC || first.CreatedAt.Location() != time.UTC {
		t.Fatalf("unexpected appointment: %+v", first)
	}
	// Read the committed row on an independent connection, not the booking transaction.
	var persisted appointments.Appointment
	err := pool.QueryRow(t.Context(), `
		SELECT id,customer_id,vehicle_id,dealership_id,service_type_id,technician_id,
		       service_bay_id,start_time,end_time,status,created_at
		FROM appointments WHERE id=$1`, first.ID).Scan(
		&persisted.ID, &persisted.CustomerID, &persisted.VehicleID, &persisted.DealershipID,
		&persisted.ServiceTypeID, &persisted.TechnicianID, &persisted.ServiceBayID,
		&persisted.StartTime, &persisted.EndTime, &persisted.Status, &persisted.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ID != first.ID || persisted.CustomerID != first.CustomerID || persisted.VehicleID != first.VehicleID ||
		persisted.DealershipID != first.DealershipID || persisted.ServiceTypeID != first.ServiceTypeID ||
		persisted.TechnicianID != first.TechnicianID || persisted.ServiceBayID != first.ServiceBayID ||
		!persisted.StartTime.Equal(first.StartTime) || !persisted.EndTime.Equal(first.EndTime) ||
		persisted.Status != first.Status || !persisted.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("persisted result differs: %+v", persisted)
	}
	second := mustBook(t, service, in)
	if second.TechnicianID != 2 || second.ServiceBayID != 2 {
		t.Fatalf("no alternative pair: %+v", second)
	}
	if got, err := service.Create(t.Context(), in); !errors.Is(err, appointments.ErrNoAvailableResources) || got.ID != 0 {
		t.Fatalf("expected capacity error, got %+v %v", got, err)
	}
	if n := count(t, pool); n != 2 {
		t.Fatalf("committed %d rows", n)
	}
	in.StartTime = first.EndTime.Format(time.RFC3339)
	adjacent := mustBook(t, service, in)
	if adjacent.TechnicianID != 1 || adjacent.ServiceBayID != 1 {
		t.Fatalf("adjacent resources not reusable: %+v", adjacent)
	}
}

func TestBookingRejectsInvalidReferences(t *testing.T) {
	pool, service, _ := setup(t)
	for _, tc := range []struct {
		name   string
		change func(*appointments.CreateInput)
		want   error
	}{
		{"customer", func(in *appointments.CreateInput) { in.CustomerID = 999 }, appointments.ErrCustomerNotFound},
		{"vehicle", func(in *appointments.CreateInput) { in.VehicleID = 999 }, appointments.ErrVehicleNotFound},
		{"dealership", func(in *appointments.CreateInput) { in.DealershipID = 999 }, appointments.ErrDealershipNotFound},
		{"service", func(in *appointments.CreateInput) { in.ServiceTypeID = 999 }, appointments.ErrServiceTypeNotFound},
		{"mismatch", func(in *appointments.CreateInput) { in.VehicleID = 2 }, appointments.ErrCustomerVehicleMismatch},
		{"past", func(in *appointments.CreateInput) { in.StartTime = time.Now().Add(-time.Hour).Format(time.RFC3339) }, appointments.ErrInvalidRequest},
		{"missing offset", func(in *appointments.CreateInput) { in.StartTime = "2099-01-01T10:00:00" }, appointments.ErrInvalidRequest},
		{"negative ID", func(in *appointments.CreateInput) { in.CustomerID = -1 }, appointments.ErrInvalidRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input()
			tc.change(&in)
			got, err := service.Create(t.Context(), in)
			if !errors.Is(err, tc.want) || got.ID != 0 {
				t.Fatalf("got %+v %v; want %v", got, err, tc.want)
			}
			if n := count(t, pool); n != 0 {
				t.Fatalf("invalid request persisted %d rows", n)
			}
		})
	}
	mustBook(t, service, input()) // Failed transactions must release the dealership lock.
}

func TestBookingExcludesUnqualifiedAndOtherDealershipResources(t *testing.T) {
	pool, service, _ := setup(t)
	execSQL(t, pool, "DELETE FROM technician_qualifications WHERE technician_id IN (1,2)")
	if _, err := service.Create(t.Context(), input()); !errors.Is(err, appointments.ErrNoAvailableResources) {
		t.Fatalf("got %v", err)
	}
	if count(t, pool) != 0 {
		t.Fatal("ineligible resources produced a booking")
	}
	in := input()
	in.DealershipID = 2
	got := mustBook(t, service, in)
	if got.TechnicianID != 4 || got.ServiceBayID != 3 {
		t.Fatalf("wrong dealership allocation: %+v", got)
	}
}

func TestBookingCommitFailureRollsBack(t *testing.T) {
	pool, service, _ := setup(t)
	execSQL(t, pool, `
		CREATE FUNCTION reject_booking() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected commit failure' USING ERRCODE='23514'; END;
		$$;
		CREATE CONSTRAINT TRIGGER reject_booking AFTER INSERT ON appointments
		DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_booking();
	`)
	got, err := service.Create(t.Context(), input())
	var pgErr *pgconn.PgError
	if got.ID != 0 || !errors.As(err, &pgErr) || pgErr.Code != "23514" || errors.Is(err, appointments.ErrNoAvailableResources) {
		t.Fatalf("commit failure misreported: %+v %v", got, err)
	}
	if n := count(t, pool); n != 0 {
		t.Fatalf("failed commit left %d rows", n)
	}
	execSQL(t, pool, "DROP TRIGGER reject_booking ON appointments")
	mustBook(t, service, input())
}

func TestBookingCancellationReleasesConnection(t *testing.T) {
	pool, service, _ := setup(t)
	lock, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(context.Background())
	if _, err := lock.Exec(t.Context(), "SELECT id FROM dealerships WHERE id=1 FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	got, err := service.Create(ctx, input())
	if got.ID != 0 || !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, appointments.ErrNoAvailableResources) {
		t.Fatalf("lock cancellation misreported: %+v %v", got, err)
	}
	if err := lock.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := count(t, pool); n != 0 {
		t.Fatalf("cancelled booking persisted %d rows", n)
	}
	mustBook(t, service, input())
}
