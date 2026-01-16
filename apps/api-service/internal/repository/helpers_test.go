package repository

import (
	"database/sql"
	"testing"
	"time"
)

// TestNullString tests the NullString helper function
func TestNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected sql.NullString
	}{
		{
			name:  "NULL - empty string",
			input: "",
			expected: sql.NullString{
				String: "",
				Valid:  false,
			},
		},
		{
			name:  "valid - single character",
			input: "a",
			expected: sql.NullString{
				String: "a",
				Valid:  true,
			},
		},
		{
			name:  "valid - regular string",
			input: "hello",
			expected: sql.NullString{
				String: "hello",
				Valid:  true,
			},
		},
		{
			name:  "valid - string with spaces",
			input: "hello world",
			expected: sql.NullString{
				String: "hello world",
				Valid:  true,
			},
		},
		{
			name:  "valid - string with special characters",
			input: "test@example.com",
			expected: sql.NullString{
				String: "test@example.com",
				Valid:  true,
			},
		},
		{
			name:  "valid - long string",
			input: "This is a much longer string with multiple words and punctuation!",
			expected: sql.NullString{
				String: "This is a much longer string with multiple words and punctuation!",
				Valid:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullString(tt.input)
			if result.Valid != tt.expected.Valid {
				t.Errorf("NullString(%q).Valid = %v, want %v", tt.input, result.Valid, tt.expected.Valid)
			}
			if result.String != tt.expected.String {
				t.Errorf("NullString(%q).String = %q, want %q", tt.input, result.String, tt.expected.String)
			}
		})
	}
}

// TestNullInt64 tests the NullInt64 helper function
func TestNullInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    *int64
		expected sql.NullInt64
	}{
		{
			name:  "NULL - nil pointer",
			input: nil,
			expected: sql.NullInt64{
				Int64: 0,
				Valid: false,
			},
		},
		{
			name:  "valid - zero",
			input: ptrInt64(0),
			expected: sql.NullInt64{
				Int64: 0,
				Valid: true,
			},
		},
		{
			name:  "valid - positive number",
			input: ptrInt64(42),
			expected: sql.NullInt64{
				Int64: 42,
				Valid: true,
			},
		},
		{
			name:  "valid - negative number",
			input: ptrInt64(-100),
			expected: sql.NullInt64{
				Int64: -100,
				Valid: true,
			},
		},
		{
			name:  "valid - max int64",
			input: ptrInt64(9223372036854775807),
			expected: sql.NullInt64{
				Int64: 9223372036854775807,
				Valid: true,
			},
		},
		{
			name:  "valid - min int64",
			input: ptrInt64(-9223372036854775808),
			expected: sql.NullInt64{
				Int64: -9223372036854775808,
				Valid: true,
			},
		},
		{
			name:  "valid - large positive",
			input: ptrInt64(1234567890123456789),
			expected: sql.NullInt64{
				Int64: 1234567890123456789,
				Valid: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullInt64(tt.input)
			if result.Valid != tt.expected.Valid {
				t.Errorf("NullInt64(%v).Valid = %v, want %v", tt.input, result.Valid, tt.expected.Valid)
			}
			if result.Int64 != tt.expected.Int64 {
				t.Errorf("NullInt64(%v).Int64 = %d, want %d", tt.input, result.Int64, tt.expected.Int64)
			}
		})
	}
}

// TestNullInt32 tests the NullInt32 helper function
func TestNullInt32(t *testing.T) {
	tests := []struct {
		name     string
		input    *int
		expected sql.NullInt32
	}{
		{
			name:  "NULL - nil pointer",
			input: nil,
			expected: sql.NullInt32{
				Int32: 0,
				Valid: false,
			},
		},
		{
			name:  "valid - zero",
			input: ptrInt(0),
			expected: sql.NullInt32{
				Int32: 0,
				Valid: true,
			},
		},
		{
			name:  "valid - positive number",
			input: ptrInt(42),
			expected: sql.NullInt32{
				Int32: 42,
				Valid: true,
			},
		},
		{
			name:  "valid - negative number",
			input: ptrInt(-100),
			expected: sql.NullInt32{
				Int32: -100,
				Valid: true,
			},
		},
		{
			name:  "valid - max int32",
			input: ptrInt(2147483647),
			expected: sql.NullInt32{
				Int32: 2147483647,
				Valid: true,
			},
		},
		{
			name:  "valid - min int32",
			input: ptrInt(-2147483648),
			expected: sql.NullInt32{
				Int32: -2147483648,
				Valid: true,
			},
		},
		{
			name:  "valid - small positive",
			input: ptrInt(1),
			expected: sql.NullInt32{
				Int32: 1,
				Valid: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullInt32(tt.input)
			if result.Valid != tt.expected.Valid {
				t.Errorf("NullInt32(%v).Valid = %v, want %v", tt.input, result.Valid, tt.expected.Valid)
			}
			if result.Int32 != tt.expected.Int32 {
				t.Errorf("NullInt32(%v).Int32 = %d, want %d", tt.input, result.Int32, tt.expected.Int32)
			}
		})
	}
}

// TestNullFloat64 tests the NullFloat64 helper function
func TestNullFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    *float64
		expected sql.NullFloat64
	}{
		{
			name:  "NULL - nil pointer",
			input: nil,
			expected: sql.NullFloat64{
				Float64: 0,
				Valid:   false,
			},
		},
		{
			name:  "valid - zero",
			input: ptrFloat64(0.0),
			expected: sql.NullFloat64{
				Float64: 0.0,
				Valid:   true,
			},
		},
		{
			name:  "valid - positive number",
			input: ptrFloat64(3.14159),
			expected: sql.NullFloat64{
				Float64: 3.14159,
				Valid:   true,
			},
		},
		{
			name:  "valid - negative number",
			input: ptrFloat64(-42.5),
			expected: sql.NullFloat64{
				Float64: -42.5,
				Valid:   true,
			},
		},
		{
			name:  "valid - very small positive",
			input: ptrFloat64(0.00000001),
			expected: sql.NullFloat64{
				Float64: 0.00000001,
				Valid:   true,
			},
		},
		{
			name:  "valid - very large positive",
			input: ptrFloat64(1.23456789e10),
			expected: sql.NullFloat64{
				Float64: 1.23456789e10,
				Valid:   true,
			},
		},
		{
			name:  "valid - one",
			input: ptrFloat64(1.0),
			expected: sql.NullFloat64{
				Float64: 1.0,
				Valid:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullFloat64(tt.input)
			if result.Valid != tt.expected.Valid {
				t.Errorf("NullFloat64(%v).Valid = %v, want %v", tt.input, result.Valid, tt.expected.Valid)
			}
			if result.Float64 != tt.expected.Float64 {
				t.Errorf("NullFloat64(%v).Float64 = %f, want %f", tt.input, result.Float64, tt.expected.Float64)
			}
		})
	}
}

// TestNullTimeFromUnix tests the NullTimeFromUnix helper function
func TestNullTimeFromUnix(t *testing.T) {
	tests := []struct {
		name     string
		input    *int64
		expected sql.NullTime
	}{
		{
			name:  "NULL - nil pointer",
			input: nil,
			expected: sql.NullTime{
				Time:  time.Time{},
				Valid: false,
			},
		},
		{
			name:  "valid - Unix epoch (0)",
			input: ptrInt64(0),
			expected: sql.NullTime{
				Time:  time.Unix(0, 0).UTC(),
				Valid: true,
			},
		},
		{
			name:  "valid - January 1, 1970",
			input: ptrInt64(0),
			expected: sql.NullTime{
				Time:  time.Unix(0, 0).UTC(),
				Valid: true,
			},
		},
		{
			name:  "valid - positive timestamp (2000-01-01)",
			input: ptrInt64(946684800),
			expected: sql.NullTime{
				Time:  time.Unix(946684800, 0).UTC(),
				Valid: true,
			},
		},
		{
			name:  "valid - 2026-01-17",
			input: ptrInt64(1737072000), // Approximately 2026-01-17
			expected: sql.NullTime{
				Time:  time.Unix(1737072000, 0).UTC(),
				Valid: true,
			},
		},
		{
			name:  "valid - negative timestamp (1960-01-01 ish)",
			input: ptrInt64(-315619200),
			expected: sql.NullTime{
				Time:  time.Unix(-315619200, 0).UTC(),
				Valid: true,
			},
		},
		{
			name:  "valid - recent timestamp (2025-01-01)",
			input: ptrInt64(1735689600),
			expected: sql.NullTime{
				Time:  time.Unix(1735689600, 0).UTC(),
				Valid: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullTimeFromUnix(tt.input)
			if result.Valid != tt.expected.Valid {
				t.Errorf("NullTimeFromUnix(%v).Valid = %v, want %v", tt.input, result.Valid, tt.expected.Valid)
			}
			if result.Valid && result.Time != tt.expected.Time {
				t.Errorf("NullTimeFromUnix(%v).Time = %v, want %v", tt.input, result.Time, tt.expected.Time)
			}
		})
	}
}

// Helper functions for pointer creation
func ptrInt64(i int64) *int64 {
	return &i
}

func ptrInt(i int) *int {
	return &i
}

func ptrFloat64(f float64) *float64 {
	return &f
}
