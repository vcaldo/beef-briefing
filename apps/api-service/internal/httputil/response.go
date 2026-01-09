package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// RespondJSON writes a JSON response with the given status code.
// The error from json encoding is logged but not returned since
// response headers have already been written at that point.
func RespondJSON(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// RespondError writes a JSON error response.
// Error messages should be lowercase for consistency.
func RespondError(w http.ResponseWriter, message string, status int) {
	RespondJSON(w, map[string]string{"error": message}, status)
}

// RespondOK writes a JSON success response with {"status": "ok"}.
func RespondOK(w http.ResponseWriter) {
	RespondJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}
