package tools

import (
	"encoding/json"
	"fmt"
)

// EncodeSchema encodes a schema ([]Table or similar) to JSON bytes for storage.
func EncodeSchema(schema any) ([]byte, error) {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to encode schema: %w", err)
	}
	return data, nil
}

// DecodeSchema decodes JSON bytes into the provided schema pointer.
// The target must be a pointer to the type that was encoded.
func DecodeSchema(data []byte, target any) error {
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to decode schema: %w", err)
	}
	return nil
}
