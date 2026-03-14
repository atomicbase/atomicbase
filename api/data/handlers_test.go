package data

import (
	"net/http/httptest"
	"testing"
)

func TestParsePreferHeaders(t *testing.T) {
	tests := []struct {
		name           string
		headers        []string
		wantOperation  string
		wantOnConflict string
		wantCountExact bool
	}{
		{
			name:          "operation only",
			headers:       []string{"operation=select"},
			wantOperation: "select",
		},
		{
			name:           "multiple prefer values",
			headers:        []string{"operation=insert", "on-conflict=replace", "count=exact"},
			wantOperation:  "insert",
			wantOnConflict: "replace",
			wantCountExact: true,
		},
		{
			name:           "comma separated with whitespace",
			headers:        []string{" operation = update , count = exact , on-conflict = ignore "},
			wantOperation:  "update",
			wantOnConflict: "ignore",
			wantCountExact: true,
		},
		{
			name:           "case insensitive",
			headers:        []string{"Operation=DELETE, COUNT=EXACT"},
			wantOperation:  "delete",
			wantCountExact: true,
		},
		{
			name:    "missing headers",
			headers: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/data/query/users", nil)
			for _, header := range tt.headers {
				req.Header.Add("Prefer", header)
			}

			operation, onConflict, countExact := parsePreferHeaders(req)
			if operation != tt.wantOperation {
				t.Fatalf("expected operation %q, got %q", tt.wantOperation, operation)
			}
			if onConflict != tt.wantOnConflict {
				t.Fatalf("expected onConflict %q, got %q", tt.wantOnConflict, onConflict)
			}
			if countExact != tt.wantCountExact {
				t.Fatalf("expected countExact %v, got %v", tt.wantCountExact, countExact)
			}
		})
	}
}
