package platform

import (
	"testing"

	"github.com/atomicbase/atomicbase/tools"
)

func TestValidateResourceName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCode  string
		wantValid bool
	}{
		// Valid names
		{"lowercase", "my-database", "", true},
		{"with numbers", "tenant123", "", true},
		{"dashes", "a-b-c", "", true},
		{"single char", "a", "", true},
		{"64 chars", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", true},

		// Invalid: uppercase
		{"uppercase start", "My-Database", tools.CodeInvalidName, false},
		{"uppercase middle", "myTenant", tools.CodeInvalidName, false},
		{"all uppercase", "TENANT", tools.CodeInvalidName, false},

		// Invalid: special characters
		{"underscore", "tenant_name", tools.CodeInvalidName, false},
		{"dot", "database.name", tools.CodeInvalidName, false},
		{"space", "database name", tools.CodeInvalidName, false},
		{"at sign", "database@name", tools.CodeInvalidName, false},

		// Invalid: too long
		{"65 chars", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", tools.CodeInvalidName, false},
		{"100 chars", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", tools.CodeInvalidName, false},

		// Edge cases
		{"empty string", "", tools.CodeInvalidName, false},
		{"only dash", "-", "", true},
		{"starts with dash", "-database", "", true},
		{"ends with dash", "database-", "", true},
		{"only numbers", "12345", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, msg, hint := validateResourceName(tt.input)
			if tt.wantValid {
				if code != "" {
					t.Errorf("validateResourceName(%q) = (%q, %q, %q), want valid", tt.input, code, msg, hint)
				}
			} else {
				if code != tt.wantCode {
					t.Errorf("validateResourceName(%q) code = %q, want %q", tt.input, code, tt.wantCode)
				}
				if msg == "" {
					t.Errorf("validateResourceName(%q) message should not be empty for invalid input", tt.input)
				}
				if hint == "" {
					t.Errorf("validateResourceName(%q) hint should not be empty for invalid input", tt.input)
				}
			}
		})
	}
}
