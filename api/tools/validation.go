package tools

import (
	"fmt"
	"strings"
	"unicode"
)

// Constants for identifier validation.
const (
	MaxIdentifierLength = 128
	MaxResourceNameLen  = 64
)

// ValidateIdentifier validates a table or column name.
// Returns nil if valid, or an error describing the problem.
func ValidateIdentifier(name string) error {
	if name == "" {
		return ErrEmptyIdentifier
	}
	if len(name) > MaxIdentifierLength {
		return fmt.Errorf("%w: %d characters (max %d)", ErrIdentifierTooLong, len(name), MaxIdentifierLength)
	}
	for i, r := range name {
		if i == 0 {
			// First character must be letter or underscore
			if !unicode.IsLetter(r) && r != '_' {
				return fmt.Errorf("%w: identifier must start with letter or underscore", ErrInvalidCharacter)
			}
		} else {
			// Subsequent characters can be letter, digit, or underscore
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return fmt.Errorf("%w: '%c' at position %d", ErrInvalidCharacter, r, i)
			}
		}
	}
	return nil
}

// ValidateTableName validates a table name.
func ValidateTableName(name string) error {
	if err := ValidateIdentifier(name); err != nil {
		return fmt.Errorf("invalid table name %q: %w", name, err)
	}
	return nil
}

// ValidateColumnName validates a column name.
func ValidateColumnName(name string) error {
	if err := ValidateIdentifier(name); err != nil {
		return fmt.Errorf("invalid column name %q: %w", name, err)
	}
	return nil
}

// ValidateResourceName validates platform resource names such as template and database names.
// Valid names are 1-64 chars and may contain only lowercase letters, numbers, and dashes.
func ValidateResourceName(name string) (code, message, hint string) {
	const baseHint = "Names must be 1-64 characters, containing only lowercase letters, numbers, and dashes."

	if len(name) == 0 {
		return CodeInvalidName, "name cannot be empty", baseHint
	}
	if len(name) > MaxResourceNameLen {
		return CodeInvalidName, "name exceeds maximum length of 64 characters", baseHint
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return CodeInvalidName, "name contains invalid characters",
				"Names must contain only lowercase letters, numbers, and dashes."
		}
	}
	return "", "", ""
}

// ValidateDDLQuery validates that a SQL query is a DDL statement.
// Only CREATE, ALTER, and DROP statements are allowed.
// This prevents arbitrary SQL execution through the schema editing endpoint.
func ValidateDDLQuery(query string) error {
	// Trim whitespace and get the first word
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ErrNotDDLQuery
	}

	// Get the first keyword (case-insensitive)
	firstWord := strings.ToUpper(strings.Fields(trimmed)[0])

	// Only allow DDL statements
	switch firstWord {
	case "CREATE", "ALTER", "DROP":
		return nil
	default:
		return fmt.Errorf("%w: got %s", ErrNotDDLQuery, firstWord)
	}
}
