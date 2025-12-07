package repository

import (
	"database/sql"
)

// Helper functions for null types

func NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func NullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

func NullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func NullInt32(i *int) sql.NullInt32 {
	if i == nil {
		return sql.NullInt32{Valid: false}
	}
	return sql.NullInt32{Int32: int32(*i), Valid: true}
}
