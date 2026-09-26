package postgres

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/votatdat/challenge_appointment_scheduler/internal/appointments"
)

// Keep the cause for diagnostics while exposing a transport-independent error.
// Constraint and SQL programming errors must remain unexpected failures.
func databaseError(err error) error {
	if err == nil {
		return nil
	}
	var connect *pgconn.ConnectError
	var network net.Error
	var pgErr *pgconn.PgError
	unavailable := errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.As(err, &connect) || errors.As(err, &network)
	if errors.As(err, &pgErr) {
		unavailable = unavailable || strings.HasPrefix(pgErr.Code, "08") ||
			pgErr.Code == "53300" || pgErr.Code == "57P01" || pgErr.Code == "57P02" || pgErr.Code == "57P03"
	}
	if unavailable {
		return errors.Join(appointments.ErrUnavailable, err)
	}
	return err
}
