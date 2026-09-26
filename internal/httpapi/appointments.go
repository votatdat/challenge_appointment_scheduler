package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/votatdat/challenge_appointment_scheduler/internal/appointments"
)

type appointmentService interface {
	Create(context.Context, appointments.CreateInput) (appointments.Appointment, error)
	Get(context.Context, int64) (appointments.Appointment, error)
}

type appointmentHandler struct{ service appointmentService }

type createRequest struct {
	CustomerID    int64  `json:"customer_id"`
	VehicleID     int64  `json:"vehicle_id"`
	DealershipID  int64  `json:"dealership_id"`
	ServiceTypeID int64  `json:"service_type_id"`
	StartTime     string `json:"start_time"`
}

type appointmentResponse struct {
	ID            int64     `json:"id"`
	CustomerID    int64     `json:"customer_id"`
	VehicleID     int64     `json:"vehicle_id"`
	DealershipID  int64     `json:"dealership_id"`
	ServiceTypeID int64     `json:"service_type_id"`
	TechnicianID  int64     `json:"technician_id"`
	ServiceBayID  int64     `json:"service_bay_id"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func representation(a appointments.Appointment) appointmentResponse {
	return appointmentResponse{
		ID: a.ID, CustomerID: a.CustomerID, VehicleID: a.VehicleID,
		DealershipID: a.DealershipID, ServiceTypeID: a.ServiceTypeID,
		TechnicianID: a.TechnicianID, ServiceBayID: a.ServiceBayID,
		StartTime: a.StartTime.UTC(), EndTime: a.EndTime.UTC(),
		Status: a.Status, CreatedAt: a.CreatedAt.UTC(),
	}
}

func (h appointmentHandler) create(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Content-Type must be application/json.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body *createRequest
	if err := decoder.Decode(&body); err != nil || body == nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Body must be a JSON object with only the documented fields (maximum 64 KiB).")
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Body must contain exactly one JSON object.")
		return
	}
	result, err := h.service.Create(r.Context(), appointments.CreateInput{
		CustomerID: body.CustomerID, VehicleID: body.VehicleID,
		DealershipID: body.DealershipID, ServiceTypeID: body.ServiceTypeID, StartTime: body.StartTime,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Location", "/appointments/"+strconv.FormatInt(result.ID, 10))
	writeJSON(w, http.StatusCreated, representation(result))
}

func (h appointmentHandler) get(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 || raw[0] < '0' || raw[0] > '9' {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Appointment ID must be a positive integer.")
		return
	}
	result, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, representation(result))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred."
	switch {
	case errors.Is(err, appointments.ErrInvalidRequest):
		status, code, message = 400, "INVALID_REQUEST", "Provide positive integer IDs and a future RFC 3339 start time with a timezone offset."
	case errors.Is(err, appointments.ErrCustomerVehicleMismatch):
		status, code, message = 400, "CUSTOMER_VEHICLE_MISMATCH", "The vehicle does not belong to the supplied customer."
	case errors.Is(err, appointments.ErrCustomerNotFound):
		status, code, message = 404, "CUSTOMER_NOT_FOUND", "Customer not found."
	case errors.Is(err, appointments.ErrVehicleNotFound):
		status, code, message = 404, "VEHICLE_NOT_FOUND", "Vehicle not found."
	case errors.Is(err, appointments.ErrDealershipNotFound):
		status, code, message = 404, "DEALERSHIP_NOT_FOUND", "Dealership not found."
	case errors.Is(err, appointments.ErrServiceTypeNotFound):
		status, code, message = 404, "SERVICE_TYPE_NOT_FOUND", "Service type not found."
	case errors.Is(err, appointments.ErrAppointmentNotFound):
		status, code, message = 404, "APPOINTMENT_NOT_FOUND", "Appointment not found."
	case errors.Is(err, appointments.ErrNoAvailableResources):
		status, code, message = 409, "NO_AVAILABLE_RESOURCES", "No qualified technician and service bay are available for the requested time."
	case errors.Is(err, appointments.ErrUnavailable):
		status, code, message = 503, "SERVICE_UNAVAILABLE", "The service is temporarily unavailable."
	}
	writeError(w, r, status, code, message)
}
