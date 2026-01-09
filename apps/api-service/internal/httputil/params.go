package httputil

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Default parameter values for API endpoints.
const (
	DefaultLimit    = 50
	DefaultMaxLimit = 100
	DefaultPeriod   = "30d"
)

// ParseInt64 parses an int64 from a query parameter.
// Returns 0 and error if parsing fails.
func ParseInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
}

// ParseIntWithDefault parses an int query parameter with bounds checking.
// Returns defaultVal if empty, too small, or unparseable.
// Caps at maxVal if the parsed value exceeds it.
func ParseIntWithDefault(r *http.Request, name string, defaultVal, minVal, maxVal int) int {
	str := r.URL.Query().Get(name)
	if str == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(str)
	if err != nil || val < minVal {
		return defaultVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

// ParseBool parses a boolean query parameter.
// Accepts "true", "false", "1", "0" (case insensitive).
// Returns defaultVal if empty or unparseable.
func ParseBool(r *http.Request, name string, defaultVal bool) bool {
	str := r.URL.Query().Get(name)
	if str == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(str)
	if err != nil {
		return defaultVal
	}
	return val
}

// ParseTimezone parses the timezone from "tz" query parameter.
// Returns UTC if not provided or invalid.
func ParseTimezone(r *http.Request) *time.Location {
	tzStr := r.URL.Query().Get("tz")
	if tzStr == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		slog.Warn("invalid timezone, using UTC", "timezone", tzStr, "error", err)
		return time.UTC
	}
	return loc
}

// ParseDateParam parses a date parameter in YYYY-MM-DD format.
// Returns nil if the parameter is empty.
func ParseDateParam(r *http.Request, name string) (*time.Time, error) {
	str := r.URL.Query().Get(name)
	if str == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", str)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ParsePeriod returns the period parameter or a default value.
func ParsePeriod(r *http.Request, defaultPeriod string) string {
	period := r.URL.Query().Get("period")
	if period == "" {
		return defaultPeriod
	}
	return period
}
