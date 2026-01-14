// Package database provides error definitions for the data access layer.
package database

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

// Error codes for SDK consumption.
// These codes are stable and can be used for programmatic error handling.
const (
	CodeTableNotFound       = "TABLE_NOT_FOUND"
	CodeColumnNotFound      = "COLUMN_NOT_FOUND"
	CodeDatabaseNotFound    = "DATABASE_NOT_FOUND"
	CodeTemplateNotFound    = "TEMPLATE_NOT_FOUND"
	CodeNoRelationship      = "NO_RELATIONSHIP"
	CodeInvalidOperator     = "INVALID_OPERATOR"
	CodeInvalidColumnType   = "INVALID_COLUMN_TYPE"
	CodeInvalidIdentifier   = "INVALID_IDENTIFIER"
	CodeMissingWhereClause  = "MISSING_WHERE_CLAUSE"
	CodeQueryTooDeep        = "QUERY_TOO_DEEP"
	CodeArrayTooLarge       = "ARRAY_TOO_LARGE"
	CodeReservedTable       = "RESERVED_TABLE"
	CodeNotDDLQuery         = "NOT_DDL_QUERY"
	CodeTemplateInUse       = "TEMPLATE_IN_USE"
	CodeUniqueViolation     = "UNIQUE_VIOLATION"
	CodeForeignKeyViolation = "FOREIGN_KEY_VIOLATION"
	CodeNotNullViolation    = "NOT_NULL_VIOLATION"
	CodeNoFTSIndex          = "NO_FTS_INDEX"
	CodeBatchTooLarge       = "BATCH_TOO_LARGE"
	CodeInternalError       = "INTERNAL_ERROR"

	// Turso-specific error codes
	CodeTursoConfigMissing  = "TURSO_CONFIG_MISSING"
	CodeTursoAuthFailed     = "TURSO_AUTH_FAILED"
	CodeTursoForbidden      = "TURSO_FORBIDDEN"
	CodeTursoNotFound       = "TURSO_NOT_FOUND"
	CodeTursoRateLimited    = "TURSO_RATE_LIMITED"
	CodeTursoConnection     = "TURSO_CONNECTION_ERROR"
	CodeTursoTokenExpired   = "TURSO_TOKEN_EXPIRED"
	CodeTursoServerError    = "TURSO_SERVER_ERROR"
)

// APIError represents a structured error response for the API.
// Code is a stable identifier for SDK/client error handling.
// Message describes what went wrong.
// Hint provides actionable guidance to resolve the issue.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

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
	ErrNoFTSIndex          = errors.New("no FTS index exists for table")
	ErrTemplateNotFound    = errors.New("template not found")
	ErrTemplateInUse   = errors.New("template is in use by one or more databases")
	ErrInArrayTooLarge = errors.New("IN array exceeds maximum size")
	ErrBatchTooLarge   = errors.New("batch exceeds maximum number of operations")
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
