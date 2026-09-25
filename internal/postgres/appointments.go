package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/votatdat/challenge_appointment_scheduler/internal/appointments"
)

const bookingTimeout = 5 * time.Second

type AppointmentStore struct {
	pool *pgxpool.Pool
}

var _ appointments.Creator = (*AppointmentStore)(nil)

func NewAppointmentStore(pool *pgxpool.Pool) *AppointmentStore {
	return &AppointmentStore{pool: pool}
}

// Create allocates both resources under one dealership lock and returns only
// after commit. All booking writers must use this protocol.
func (s *AppointmentStore) Create(ctx context.Context, request appointments.BookingRequest) (appointments.Appointment, error) {
	ctx, cancel := context.WithTimeout(ctx, bookingTimeout)
	defer cancel()

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return appointments.Appointment{}, fmt.Errorf("begin booking: %w", err)
	}
	// Cancellation does not automatically roll back a pgx transaction. Cleanup
	// gets its own bounded context so a cancelled request cannot retain the lock.
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cleanupCancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	var dealershipID int64
	err = tx.QueryRow(ctx, "SELECT id FROM dealerships WHERE id = $1 FOR UPDATE", request.DealershipID).Scan(&dealershipID)
	if err != nil {
		return appointments.Appointment{}, referenceError(err, appointments.ErrDealershipNotFound, "lock dealership")
	}

	// A request can become stale while waiting for another booking to commit.
	if !request.StartTime.After(time.Now()) {
		return appointments.Appointment{}, fmt.Errorf("%w: start_time must still be in the future", appointments.ErrInvalidRequest)
	}

	var customerID int64
	err = tx.QueryRow(ctx, "SELECT id FROM customers WHERE id = $1", request.CustomerID).Scan(&customerID)
	if err != nil {
		return appointments.Appointment{}, referenceError(err, appointments.ErrCustomerNotFound, "find customer")
	}
	var vehicleCustomerID int64
	err = tx.QueryRow(ctx, "SELECT customer_id FROM vehicles WHERE id = $1", request.VehicleID).Scan(&vehicleCustomerID)
	if err != nil {
		return appointments.Appointment{}, referenceError(err, appointments.ErrVehicleNotFound, "find vehicle")
	}
	if vehicleCustomerID != request.CustomerID {
		return appointments.Appointment{}, appointments.ErrCustomerVehicleMismatch
	}

	// Compute in SQL to support the integer duration column without overflowing
	// Go's nanosecond-based time.Duration for large catalog values.
	var end time.Time
	err = tx.QueryRow(ctx, `
		SELECT $2::timestamptz + duration_minutes * interval '1 minute'
		FROM service_types WHERE id = $1`, request.ServiceTypeID, request.StartTime).Scan(&end)
	if err != nil {
		return appointments.Appointment{}, referenceError(err, appointments.ErrServiceTypeNotFound, "find service duration")
	}
	if end.Year() > 9999 {
		return appointments.Appointment{}, fmt.Errorf("%w: end_time is outside RFC 3339 range", appointments.ErrInvalidRequest)
	}

	// These queries must run AFTER the lock statement: at READ COMMITTED they
	// then see appointments committed by the previous lock holder.
	var technicianID int64
	err = tx.QueryRow(ctx, `
		SELECT t.id
		FROM technicians t
		JOIN technician_qualifications q ON q.technician_id = t.id
		WHERE t.dealership_id = $1 AND q.service_type_id = $2
		  AND NOT EXISTS (
			SELECT 1 FROM appointments a
			WHERE a.dealership_id = t.dealership_id AND a.technician_id = t.id
			  AND a.start_time < $4 AND a.end_time > $3
		  )
		ORDER BY t.id LIMIT 1`,
		request.DealershipID, request.ServiceTypeID, request.StartTime, end).Scan(&technicianID)
	if err != nil {
		return appointments.Appointment{}, referenceError(err, appointments.ErrNoAvailableResources, "select technician")
	}
	var bayID int64
	err = tx.QueryRow(ctx, `
		SELECT b.id FROM service_bays b
		WHERE b.dealership_id = $1
		  AND NOT EXISTS (
			SELECT 1 FROM appointments a
			WHERE a.dealership_id = b.dealership_id AND a.service_bay_id = b.id
			  AND a.start_time < $3 AND a.end_time > $2
		  )
		ORDER BY b.id LIMIT 1`,
		request.DealershipID, request.StartTime, end).Scan(&bayID)
	if err != nil {
		return appointments.Appointment{}, referenceError(err, appointments.ErrNoAvailableResources, "select bay")
	}

	var result appointments.Appointment
	err = tx.QueryRow(ctx, `
		INSERT INTO appointments (
			customer_id, vehicle_id, dealership_id, service_type_id,
			technician_id, service_bay_id, start_time, end_time
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, customer_id, vehicle_id, dealership_id, service_type_id,
		          technician_id, service_bay_id, start_time, end_time, status, created_at`,
		request.CustomerID, request.VehicleID, request.DealershipID, request.ServiceTypeID,
		technicianID, bayID, request.StartTime, end).Scan(
		&result.ID, &result.CustomerID, &result.VehicleID, &result.DealershipID, &result.ServiceTypeID,
		&result.TechnicianID, &result.ServiceBayID, &result.StartTime, &result.EndTime, &result.Status, &result.CreatedAt)
	if err != nil {
		return appointments.Appointment{}, fmt.Errorf("insert appointment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return appointments.Appointment{}, fmt.Errorf("commit appointment: %w", err)
	}
	result.StartTime = result.StartTime.UTC()
	result.EndTime = result.EndTime.UTC()
	result.CreatedAt = result.CreatedAt.UTC()
	return result, nil
}

func referenceError(err, missing error, operation string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return missing
	}
	return fmt.Errorf("%s: %w", operation, err)
}
