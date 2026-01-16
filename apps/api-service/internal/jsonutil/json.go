package jsonutil

import (
	"encoding/json"
	"fmt"
)

// Marshal wraps json.Marshal with consistent error handling.
// Returns the JSON encoding of v with context-aware error wrapping.
func Marshal(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return data, nil
}

// Unmarshal wraps json.Unmarshal with consistent error handling.
// Parses the JSON-encoded data and stores the result in the value pointed to by v.
func Unmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return nil
}
