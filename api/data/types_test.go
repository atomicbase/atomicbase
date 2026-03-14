package data

import (
	"encoding/json"
	"testing"
)

func TestRowDataUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantFirst map[string]any
		wantErr   bool
	}{
		{
			name:    "single object",
			input:   `{"id":1,"name":"alice"}`,
			wantLen: 1,
			wantFirst: map[string]any{
				"id":   float64(1),
				"name": "alice",
			},
		},
		{
			name:    "array of objects",
			input:   `[{"id":1},{"id":2}]`,
			wantLen: 2,
			wantFirst: map[string]any{
				"id": float64(1),
			},
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantLen: 0,
		},
		{
			name:    "invalid scalar",
			input:   `42`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rows RowData
			err := json.Unmarshal([]byte(tt.input), &rows)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rows) != tt.wantLen {
				t.Fatalf("expected %d rows, got %d", tt.wantLen, len(rows))
			}
			if tt.wantFirst != nil {
				for key, want := range tt.wantFirst {
					if got := rows[0][key]; got != want {
						t.Fatalf("expected first row %s=%v, got %v", key, want, got)
					}
				}
			}
		})
	}
}
