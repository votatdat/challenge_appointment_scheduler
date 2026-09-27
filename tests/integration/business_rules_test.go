//go:build integration

package integration

import (
	"errors"
	"testing"
	"time"

	"github.com/votatdat/challenge_appointment_scheduler/internal/appointments"
)

func TestBookingIntervalMatrix(t *testing.T) {
	for _, resource := range []string{"technician", "bay"} {
		for _, tc := range []struct {
			name      string
			offset    time.Duration
			serviceID int64
			duration  time.Duration
			conflict  bool
		}{
			{"identical", 0, 1, time.Hour, true},
			{"partial before", -30 * time.Minute, 1, time.Hour, true},
			{"partial after", 30 * time.Minute, 1, time.Hour, true},
			{"contains", -15 * time.Minute, 2, 90 * time.Minute, true},
			{"contained", 15 * time.Minute, 3, 30 * time.Minute, true},
			{"same start shorter", 0, 3, 30 * time.Minute, true},
			{"same end shorter", 30 * time.Minute, 3, 30 * time.Minute, true},
			{"adjacent before", -time.Hour, 1, time.Hour, false},
			{"adjacent after", time.Hour, 1, time.Hour, false},
			{"gap before", -61 * time.Minute, 1, time.Hour, false},
			{"gap after", 61 * time.Minute, 1, time.Hour, false},
			{"microsecond overlap before", -time.Hour + time.Microsecond, 1, time.Hour, true},
			{"microsecond overlap after", time.Hour - time.Microsecond, 1, time.Hour, true},
		} {
			t.Run(resource+"/"+tc.name, func(t *testing.T) {
				pool, service, _ := setup(t)
				// Both technicians qualify for every test duration, so a bay
				// failure cannot be masked by a qualification failure.
				execSQL(t, pool, `
                    INSERT INTO service_types(id,name,duration_minutes) VALUES (3,'Short service',30);
                    INSERT INTO technician_qualifications VALUES (1,3),(2,2),(2,3);
                `)
				if resource == "technician" {
					execSQL(t, pool, "DELETE FROM technician_qualifications WHERE technician_id=2")
				} else {
					execSQL(t, pool, "DELETE FROM service_bays WHERE id=2")
				}
				in := input()
				start, err := time.Parse(time.RFC3339, in.StartTime)
				if err != nil {
					t.Fatal(err)
				}
				in.StartTime = start.Add(123456 * time.Microsecond).Format(time.RFC3339Nano)
				first := mustBook(t, service, in)
				in.ServiceTypeID = tc.serviceID
				wantStart := first.StartTime.Add(tc.offset)
				in.StartTime = wantStart.Format(time.RFC3339Nano)
				got, err := service.Create(t.Context(), in)
				wantCount := 2
				if tc.conflict {
					wantCount = 1
					if !errors.Is(err, appointments.ErrNoAvailableResources) || got != (appointments.Appointment{}) {
						t.Fatalf("expected conflict with no result, got %+v %v", got, err)
					}
				} else if err != nil || got.TechnicianID != 1 || got.ServiceBayID != 1 ||
					got.ServiceTypeID != tc.serviceID || !got.StartTime.Equal(wantStart) ||
					!got.EndTime.Equal(wantStart.Add(tc.duration)) {
					t.Fatalf("expected resource reuse and service-derived interval, got %+v %v", got, err)
				}
				if n := count(t, pool); n != wantCount {
					t.Fatalf("persisted %d rows, want %d", n, wantCount)
				}
				persisted, err := service.Get(t.Context(), first.ID)
				if err != nil || persisted != first {
					t.Fatalf("existing booking changed: %+v %v", persisted, err)
				}
			})
		}
	}
}

func TestBookingIndependentResourceAlternatives(t *testing.T) {
	for _, tc := range []struct {
		name, fixture       string
		technicianID, bayID int64
	}{
		// Move the first appointment to leave only one first-choice resource
		// occupied. Updates configure test fixtures in an isolated schema.
		{"technician occupied", "UPDATE appointments SET service_bay_id=2 WHERE id=$1", 2, 1},
		{"bay occupied", "UPDATE appointments SET technician_id=2 WHERE id=$1", 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, service, _ := setup(t)
			in := input()
			first := mustBook(t, service, in)
			execSQL(t, pool, tc.fixture, first.ID)
			got := mustBook(t, service, in)
			if got.TechnicianID != tc.technicianID || got.ServiceBayID != tc.bayID || count(t, pool) != 2 {
				t.Fatalf("wrong alternative allocation: %+v", got)
			}
		})
	}
}

func TestBookingEquivalentOffsetsAndPrecision(t *testing.T) {
	pool, service, _ := setup(t)
	execSQL(t, pool, "DELETE FROM technician_qualifications WHERE technician_id=2")
	in := input()
	start, err := time.Parse(time.RFC3339, in.StartTime)
	if err != nil {
		t.Fatal(err)
	}
	instant := start.Add(123456789 * time.Nanosecond)
	in.StartTime = instant.In(time.FixedZone("east", 7*3600)).Format(time.RFC3339Nano)
	first := mustBook(t, service, in)
	wantStart := start.Add(123456 * time.Microsecond)
	if first.StartTime != wantStart || first.EndTime != wantStart.Add(time.Hour) {
		t.Fatalf("wrong UTC microsecond interval: %+v", first)
	}
	for _, offset := range []int{0, -5 * 3600} {
		in.StartTime = instant.In(time.FixedZone("offset", offset)).Format(time.RFC3339Nano)
		got, err := service.Create(t.Context(), in)
		if !errors.Is(err, appointments.ErrNoAvailableResources) || got.ID != 0 || count(t, pool) != 1 {
			t.Fatalf("same instant must conflict: %+v %v", got, err)
		}
	}
	// The fraction is truncated before comparison and storage, so this
	// normalizes to exactly the first booking's end and must be accepted.
	in.StartTime = first.EndTime.Add(999 * time.Nanosecond).Format(time.RFC3339Nano)
	adjacent := mustBook(t, service, in)
	if adjacent.StartTime != first.EndTime || adjacent.TechnicianID != first.TechnicianID || count(t, pool) != 2 {
		t.Fatalf("wrong normalized boundary: %+v", adjacent)
	}
	persisted, err := service.Get(t.Context(), first.ID)
	if err != nil || persisted != first {
		t.Fatalf("precision changed on retrieval: %+v %v", persisted, err)
	}
}

func TestBookingRequiresQualificationForSelectedService(t *testing.T) {
	pool, service, _ := setup(t)
	execSQL(t, pool, "INSERT INTO technician_qualifications VALUES (4,2)")
	in := input()
	in.ServiceTypeID = 2
	mustBook(t, service, in)
	// Technician 2 is free but qualified only for service 1. Technician 4
	// qualifies for service 2 but belongs to another dealership.
	got, err := service.Create(t.Context(), in)
	if !errors.Is(err, appointments.ErrNoAvailableResources) || got.ID != 0 || count(t, pool) != 1 {
		t.Fatalf("ineligible technician allocated: %+v %v", got, err)
	}
}

func TestBookingExcludesOtherDealershipBays(t *testing.T) {
	pool, service, _ := setup(t)
	execSQL(t, pool, "DELETE FROM service_bays WHERE dealership_id=1")
	in := input()
	got, err := service.Create(t.Context(), in)
	if !errors.Is(err, appointments.ErrNoAvailableResources) || got.ID != 0 || count(t, pool) != 0 {
		t.Fatalf("remote bay allocated: %+v %v", got, err)
	}
	in.DealershipID = 2
	got = mustBook(t, service, in)
	if got.ServiceBayID != 3 || got.TechnicianID != 4 {
		t.Fatalf("wrong dealership resources: %+v", got)
	}
}

func TestBookingPersistsOtherReferencesOutsideConventionalHours(t *testing.T) {
	pool, service, _ := setup(t)
	execSQL(t, pool, "INSERT INTO technician_qualifications VALUES (4,2)")
	in := input()
	in.CustomerID, in.VehicleID, in.DealershipID, in.ServiceTypeID = 2, 2, 2, 2
	future := time.Now().UTC().AddDate(1, 0, 0)
	sunday := time.Date(future.Year(), future.Month(), future.Day(), 2, 0, 0, 0, time.UTC).
		AddDate(0, 0, (7-int(future.Weekday()))%7)
	in.StartTime = sunday.Format(time.RFC3339)
	got := mustBook(t, service, in)
	if got.CustomerID != 2 || got.VehicleID != 2 || got.DealershipID != 2 || got.ServiceTypeID != 2 ||
		got.TechnicianID != 4 || got.ServiceBayID != 3 || got.StartTime != sunday ||
		got.EndTime != sunday.Add(90*time.Minute) || got.Status != "CONFIRMED" ||
		got.CreatedAt.IsZero() || count(t, pool) != 1 {
		t.Fatalf("wrong references or interval: %+v", got)
	}
	persisted, err := service.Get(t.Context(), got.ID)
	if err != nil || persisted != got {
		t.Fatalf("retrieval mismatch: %+v %v", persisted, err)
	}
}
