package tools

import (
	"fmt"
	"strings"
	"unicode"
)

// Constants for identifier validation.
const (
	MaxIdentifierLength = 128
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
