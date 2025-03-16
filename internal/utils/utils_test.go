package utils_test

import (
	"testing"
	"time"

	"github.com/marianozunino/drop/internal/utils"
)

func TestParseExpirationTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, got time.Time)
	}{
		{
			name:    "hours format",
			input:   "24",
			wantErr: false,
			check: func(t *testing.T, got time.Time) {
				// Should be approximately 24 hours from now
				expectedApprox := time.Now().Add(24 * time.Hour)
				diff := expectedApprox.Sub(got)
				if diff < -5*time.Second || diff > 5*time.Second {
					t.Errorf("Expected time close to %v, got %v, diff: %v", expectedApprox, got, diff)
				}
			},
		},
		{
			name:    "milliseconds timestamp format - large value",
			input:   "1742164682000",
			wantErr: false,
			check: func(t *testing.T, got time.Time) {
				// Run a test to get the exact expected time:
				// time.UnixMilli(1742164682000).UTC()
				expected := time.Date(2025, 3, 16, 22, 38, 2, 0, time.UTC)
				gotUTC := got.UTC()
				if !gotUTC.Equal(expected) {
					t.Errorf("Expected %v, got %v (UTC: %v)", expected, got, gotUTC)
				}
			},
		},
		{
			name:    "milliseconds timestamp format - small value",
			input:   "1000000000000", // Smaller timestamp but still clearly a timestamp
			wantErr: false,
			check: func(t *testing.T, got time.Time) {
				// September 9, 2001
				expected := time.Date(2001, 9, 9, 1, 46, 40, 0, time.UTC)
				gotUTC := got.UTC()
				if !gotUTC.Equal(expected) {
					t.Errorf("Expected %v, got %v (UTC: %v)", expected, got, gotUTC)
				}
			},
		},
		{
			name:    "RFC3339 format",
			input:   "2023-06-15T14:30:45Z",
			wantErr: false,
			check: func(t *testing.T, got time.Time) {
				expected := time.Date(2023, 6, 15, 14, 30, 45, 0, time.UTC)
				if !got.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, got)
				}
			},
		},
		{
			name:    "ISO date format",
			input:   "2023-06-15",
			wantErr: false,
			check: func(t *testing.T, got time.Time) {
				expected := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)
				if !got.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, got)
				}
			},
		},
		{
			name:    "ISO datetime format without timezone",
			input:   "2023-06-15T14:30:45",
			wantErr: false,
			check: func(t *testing.T, got time.Time) {
				expected := time.Date(2023, 6, 15, 14, 30, 45, 0, time.UTC)
				if !got.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, got)
				}
			},
		},
		{
			name:    "SQL datetime format",
			input:   "2023-06-15 14:30:45",
			wantErr: false,
			check: func(t *testing.T, got time.Time) {
				expected := time.Date(2023, 6, 15, 14, 30, 45, 0, time.UTC)
				if !got.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, got)
				}
			},
		},
		{
			name:    "invalid format",
			input:   "not-a-valid-time",
			wantErr: true,
			check:   func(t *testing.T, got time.Time) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.ParseExpirationTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExpirationTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				tt.check(t, got)
			}
		})
	}
}
