package appointments

import (
	"errors"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	input := CreateInput{CustomerID: 1, VehicleID: 1, DealershipID: 1, ServiceTypeID: 1}
	for _, tc := range []struct {
		name, start string
		valid       bool
	}{
		{"offset", "2030-01-02T17:00:00+07:00", true},
		{"UTC", "2030-01-02T10:00:00Z", true},
		{"negative offset crosses date", "2029-12-31T23:30:00-01:00", true},
		{"one microsecond in future", "2030-01-01T00:00:00.000001Z", true},
		{"offset instant is now", "2030-01-01T07:00:00+07:00", false},
		{"UTC year overflow", "9999-12-31T23:59:59-01:00", false},
		{"fraction", "2030-01-02T10:00:00.123456789Z", true},
		{"no offset", "2030-01-02T10:00:00", false},
		{"empty", "", false},
		{"invalid offset hour", "2030-01-02T10:00:00+24:00", false},
		{"invalid offset minute", "2030-01-02T10:00:00+07:60", false},
		{"one-digit hour", "2030-01-02T1:00:00Z", false},
		{"comma fraction", "2030-01-02T10:00:00,123Z", false},
		{"invalid date", "2030-02-30T10:00:00Z", false},
		{"past", "2029-01-01T00:00:00Z", false},
		{"now", "2030-01-01T00:00:00Z", false},
		{"below database precision", "2030-01-01T00:00:00.000000001Z", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input
			in.StartTime = tc.start
			got, err := in.Validate(now)
			if !tc.valid {
				if !errors.Is(err, ErrInvalidRequest) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			expected, _ := time.Parse(time.RFC3339Nano, tc.start)
			if !got.StartTime.Equal(expected.Truncate(time.Microsecond)) || got.StartTime.Location() != time.UTC {
				t.Fatalf("unexpected normalized start: %v", got.StartTime)
			}
		})
	}
	for _, field := range []string{"customer", "vehicle", "dealership", "service"} {
		t.Run(field+" ID", func(t *testing.T) {
			for _, id := range []int64{0, -1} {
				in := input
				in.StartTime = "2030-01-02T10:00:00Z"
				switch field {
				case "customer":
					in.CustomerID = id
				case "vehicle":
					in.VehicleID = id
				case "dealership":
					in.DealershipID = id
				case "service":
					in.ServiceTypeID = id
				}
				if _, err := in.Validate(now); !errors.Is(err, ErrInvalidRequest) {
					t.Fatalf("got %v", err)
				}
			}
		})
	}
}
