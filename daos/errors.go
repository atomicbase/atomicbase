// Package daos provides error definitions for the data access layer.
package daos

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Constants for identifier validation.
const (
	MaxIdentifierLength = 128
)

// Sentinel errors for common failure conditions.
var (
	ErrTableNotFound       = errors.New("table not found in schema")
	ErrColumnNotFound      = errors.New("column not found in table")
	ErrInvalidOperator     = errors.New("invalid filter operator")
	ErrInvalidColumnType   = errors.New("invalid column type")
	ErrReservedTable       = errors.New("cannot query reserved table")
	ErrMissingWhereClause  = errors.New("DELETE requires a WHERE clause")
	ErrDatabaseNotFound    = errors.New("database not found")
	ErrNoRelationship      = errors.New("no relationship exists between tables")
	ErrInvalidIdentifier   = errors.New("invalid identifier")
	ErrEmptyIdentifier     = errors.New("identifier cannot be empty")
	ErrIdentifierTooLong   = errors.New("identifier exceeds maximum length")
	ErrInvalidCharacter    = errors.New("identifier contains invalid characters")
	ErrNotDDLQuery         = errors.New("only DDL statements are allowed (CREATE, ALTER, DROP)")
	ErrQueryTooDeep        = errors.New("query nesting exceeds maximum depth")
)

// InvalidTypeErr returns an error indicating an invalid column type was specified.
func InvalidTypeErr(column, typeName string) error {
	return fmt.Errorf("%w: type %s for column %s", ErrInvalidColumnType, typeName, column)
}

// TableNotFoundErr returns an error indicating a table was not found.
func TableNotFoundErr(table string) error {
	return fmt.Errorf("%w: %s", ErrTableNotFound, table)
}

// ColumnNotFoundErr returns an error indicating a column was not found.
func ColumnNotFoundErr(table, column string) error {
	return fmt.Errorf("%w: %s in table %s", ErrColumnNotFound, column, table)
}

// NoRelationshipErr returns an error indicating no FK relationship exists.
func NoRelationshipErr(table1, table2 string) error {
	return fmt.Errorf("%w: %s and %s", ErrNoRelationship, table1, table2)
}

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
