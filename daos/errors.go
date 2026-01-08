// Package daos provides error definitions for the data access layer.
package daos

import (
	"errors"
	"fmt"
)

// Sentinel errors for common failure conditions.
var (
	ErrTableNotFound      = errors.New("table not found in schema")
	ErrColumnNotFound     = errors.New("column not found in table")
	ErrInvalidOperator    = errors.New("invalid filter operator")
	ErrInvalidColumnType  = errors.New("invalid column type")
	ErrReservedTable      = errors.New("cannot query reserved table")
	ErrMissingWhereClause = errors.New("DELETE requires a WHERE clause")
	ErrDatabaseNotFound   = errors.New("database not found")
	ErrNoRelationship     = errors.New("no relationship exists between tables")
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
