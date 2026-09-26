package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/votatdat/challenge_appointment_scheduler/internal/appointments"
)

func TestDatabaseErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cause       error
		unavailable bool
	}{
		{"timeout", context.DeadlineExceeded, true},
		{"cancelled", context.Canceled, true},
		{"connection refused", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, true},
		{"EOF", io.EOF, true},
		{"shutdown", &pgconn.PgError{Code: "57P01"}, true},
		{"connection SQLSTATE", &pgconn.PgError{Code: "08006"}, true},
		{"too many connections", &pgconn.PgError{Code: "53300"}, true},
		{"constraint", &pgconn.PgError{Code: "23514"}, false},
		{"SQL bug", &pgconn.PgError{Code: "42P01"}, false},
		{"not found", appointments.ErrAppointmentNotFound, false},
		{"capacity", appointments.ErrNoAvailableResources, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := databaseError(fmt.Errorf("operation: %w", tc.cause))
			if errors.Is(err, appointments.ErrUnavailable) != tc.unavailable || !errors.Is(err, tc.cause) {
				t.Fatalf("wrong classification or lost cause: %v", err)
			}
		})
	}
	if databaseError(nil) != nil {
		t.Fatal("nil became error")
	}
}
