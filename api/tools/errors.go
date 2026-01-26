// Package tools provides shared utilities for the API.
package tools

import (
	"errors"
	"fmt"
)

// Error codes for SDK consumption.
// These codes are stable and can be used for programmatic error handling.
const (
	CodeTableNotFound       = "TABLE_NOT_FOUND"
	CodeColumnNotFound      = "COLUMN_NOT_FOUND"
	CodeDatabaseNotFound    = "DATABASE_NOT_FOUND"
	CodeDatabaseOutOfSync   = "DATABASE_OUT_OF_SYNC"
	CodeTemplateNotFound    = "TEMPLATE_NOT_FOUND"
	CodeNoRelationship      = "NO_RELATIONSHIP"
	CodeInvalidOperator     = "INVALID_OPERATOR"
	CodeInvalidColumnType   = "INVALID_COLUMN_TYPE"
	CodeInvalidIdentifier   = "INVALID_IDENTIFIER"
	CodeMissingOperation    = "MISSING_OPERATION"
	CodeInvalidOnConflict   = "INVALID_ON_CONFLICT"
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
	CodeMissingTenant       = "MISSING_TENANT"
	CodeInternalError       = "INTERNAL_ERROR"

	// Turso-specific error codes
	CodeTursoConfigMissing = "TURSO_CONFIG_MISSING"
	CodeTursoAuthFailed    = "TURSO_AUTH_FAILED"
	CodeTursoForbidden     = "TURSO_FORBIDDEN"
	CodeTursoNotFound      = "TURSO_NOT_FOUND"
	CodeTursoRateLimited   = "TURSO_RATE_LIMITED"
	CodeTursoConnection    = "TURSO_CONNECTION_ERROR"
	CodeTursoTokenExpired  = "TURSO_TOKEN_EXPIRED"
	CodeTursoServerError   = "TURSO_SERVER_ERROR"
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
	ErrTableNotFound      = errors.New("table not found in schema")
	ErrColumnNotFound     = errors.New("column not found in table")
	ErrInvalidOperator    = errors.New("invalid filter operator")
	ErrInvalidColumnType  = errors.New("invalid column type")
	ErrReservedTable      = errors.New("cannot query reserved table")
	ErrMissingWhereClause = errors.New("DELETE requires a WHERE clause")
	ErrMissingOperation   = errors.New("No query operation specified")
	ErrInvalidOnConflict  = errors.New("Invalid on-conflict specified")
	ErrDatabaseNotFound   = errors.New("database not found")
	ErrDatabaseOutOfSync  = errors.New("database out of sync")
	ErrNoRelationship     = errors.New("no relationship exists between tables")
	ErrInvalidIdentifier  = errors.New("invalid identifier")
	ErrEmptyIdentifier    = errors.New("identifier cannot be empty")
	ErrIdentifierTooLong  = errors.New("identifier exceeds maximum length")
	ErrInvalidCharacter   = errors.New("identifier contains invalid characters")
	ErrNotDDLQuery        = errors.New("only DDL statements are allowed (CREATE, ALTER, DROP)")
	ErrQueryTooDeep       = errors.New("query nesting exceeds maximum depth")
	ErrNoFTSIndex         = errors.New("no FTS index exists for table")
	ErrTemplateNotFound   = errors.New("template not found")
	ErrTemplateInUse      = errors.New("template is in use by one or more databases")
	ErrInArrayTooLarge    = errors.New("IN array exceeds maximum size")
	ErrBatchTooLarge      = errors.New("batch exceeds maximum number of operations")
	ErrMissingTenant      = errors.New("Tenant header is required")
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
