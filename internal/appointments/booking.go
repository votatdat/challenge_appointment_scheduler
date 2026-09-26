// Package appointments defines booking inputs, results, and business errors.
package appointments

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var (
	ErrAppointmentNotFound     = errors.New("appointment not found")
	ErrUnavailable             = errors.New("required dependency unavailable")
	ErrInvalidRequest          = errors.New("invalid booking request")
	ErrCustomerNotFound        = errors.New("customer not found")
	ErrVehicleNotFound         = errors.New("vehicle not found")
	ErrDealershipNotFound      = errors.New("dealership not found")
	ErrServiceTypeNotFound     = errors.New("service type not found")
	ErrCustomerVehicleMismatch = errors.New("vehicle does not belong to customer")
	ErrNoAvailableResources    = errors.New("no qualified technician and service bay available")
)

// CreateInput keeps the original timestamp so a missing offset can be rejected.
type CreateInput struct {
	CustomerID    int64
	VehicleID     int64
	DealershipID  int64
	ServiceTypeID int64
	StartTime     string
}

// BookingRequest contains a validated UTC instant at PostgreSQL microsecond precision.
type BookingRequest struct {
	CustomerID    int64
	VehicleID     int64
	DealershipID  int64
	ServiceTypeID int64
	StartTime     time.Time
}

type Appointment struct {
	ID            int64
	CustomerID    int64
	VehicleID     int64
	DealershipID  int64
	ServiceTypeID int64
	TechnicianID  int64
	ServiceBayID  int64
	StartTime     time.Time
	EndTime       time.Time
	Status        string
	CreatedAt     time.Time
}

type Repository interface {
	Get(context.Context, int64) (Appointment, error)
	Create(context.Context, BookingRequest) (Appointment, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Appointment, error) {
	request, err := input.Validate(time.Now())
	if err != nil {
		return Appointment{}, err
	}
	return s.repository.Create(ctx, request)
}

// time.Parse accepts a few non-RFC forms (including comma fractions and
// out-of-range offsets), so first check the wire format explicitly.
var timestampPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

func (input CreateInput) Validate(now time.Time) (BookingRequest, error) {
	if input.CustomerID <= 0 || input.VehicleID <= 0 || input.DealershipID <= 0 || input.ServiceTypeID <= 0 {
		return BookingRequest{}, fmt.Errorf("%w: IDs must be positive", ErrInvalidRequest)
	}
	if !timestampPattern.MatchString(input.StartTime) {
		return BookingRequest{}, fmt.Errorf("%w: start_time must be RFC 3339 with a timezone offset", ErrInvalidRequest)
	}
	start, err := time.Parse(time.RFC3339Nano, input.StartTime)
	if err != nil {
		return BookingRequest{}, fmt.Errorf("%w: start_time must be RFC 3339 with a timezone offset", ErrInvalidRequest)
	}
	start = start.UTC().Truncate(time.Microsecond)
	if !start.After(now) || start.Year() > 9999 {
		return BookingRequest{}, fmt.Errorf("%w: start_time must be a future RFC 3339 instant", ErrInvalidRequest)
	}
	return BookingRequest{
		CustomerID: input.CustomerID, VehicleID: input.VehicleID,
		DealershipID: input.DealershipID, ServiceTypeID: input.ServiceTypeID,
		StartTime: start,
	}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (Appointment, error) {
	if id <= 0 {
		return Appointment{}, ErrInvalidRequest
	}
	return s.repository.Get(ctx, id)
}
