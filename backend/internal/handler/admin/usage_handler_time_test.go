package admin

import (
	"testing"
	"time"
)

func TestParseAdminUsageTime(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		wantDateOnly bool
		wantHour     int
	}{
		{name: "date", value: "2026-10-11", wantDateOnly: true, wantHour: 0},
		{name: "local datetime", value: "2026-10-11T12:34:56", wantDateOnly: false, wantHour: 12},
		{name: "rfc3339", value: "2026-10-11T12:34:56Z", wantDateOnly: false, wantHour: 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, dateOnly, err := parseAdminUsageTime(tt.value, "UTC")
			if err != nil {
				t.Fatalf("parseAdminUsageTime() error = %v", err)
			}
			if dateOnly != tt.wantDateOnly {
				t.Fatalf("dateOnly = %v, want %v", dateOnly, tt.wantDateOnly)
			}
			if parsed.Hour() != tt.wantHour {
				t.Fatalf("hour = %d, want %d", parsed.Hour(), tt.wantHour)
			}
		})
	}
}

func TestParseAdminUsageTimePreservesDateTimeEndBoundary(t *testing.T) {
	parsed, dateOnly, err := parseAdminUsageTime("2026-10-11T12:34:56", "UTC")
	if err != nil {
		t.Fatalf("parseAdminUsageTime() error = %v", err)
	}
	if dateOnly {
		t.Fatal("datetime input must not be treated as date-only")
	}
	end := parsed
	if dateOnly {
		end = end.AddDate(0, 0, 1)
	}
	want := time.Date(2026, time.October, 11, 12, 34, 56, 0, time.UTC)
	if !end.Equal(want) {
		t.Fatalf("end boundary = %s, want %s", end, want)
	}
}
