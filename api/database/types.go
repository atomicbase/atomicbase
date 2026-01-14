package database

import (
	"context"
	"database/sql"
)

// Executor is an interface that both *sql.DB and *sql.Tx implement.
// This allows query methods to work with either a direct connection or a transaction.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// SelectQuery represents a JSON SELECT query request body.
// Used with POST /query/{table} and Prefer: operation=select header.
type SelectQuery struct {
	Select []any            `json:"select,omitempty"` // Columns: ["id", "name", {"posts": ["title"]}]
	Where  []map[string]any `json:"where,omitempty"`  // Filters: [{"id": {"eq": 5}}, {"or": [...]}]
	Order  map[string]string `json:"order,omitempty"` // Ordering: {"created_at": "desc"}
	Limit  *int             `json:"limit,omitempty"`
	Offset *int             `json:"offset,omitempty"`
}

// InsertRequest represents a JSON INSERT request body.
// Used with POST /query/{table} (no Prefer header or on-conflict header).
type InsertRequest struct {
	Data      map[string]any `json:"data"`                // Row data to insert
	Returning []string       `json:"returning,omitempty"` // Columns to return after insert
}

// UpsertRequest represents a JSON UPSERT request body.
// Used with POST /query/{table} and Prefer: on-conflict=replace header.
type UpsertRequest struct {
	Data      []map[string]any `json:"data"`                // Array of rows to upsert
	Returning []string         `json:"returning,omitempty"` // Columns to return after upsert
}

// UpdateRequest represents a JSON UPDATE request body.
// Used with PATCH /query/{table}.
type UpdateRequest struct {
	Data  map[string]any   `json:"data"`  // Column values to update
	Where []map[string]any `json:"where"` // Required: filter conditions
}

// DeleteRequest represents a JSON DELETE request body.
// Used with DELETE /query/{table}.
type DeleteRequest struct {
	Where []map[string]any `json:"where"` // Required: filter conditions
}

// Filter represents a single filter condition on a column.
// Only one field should be set per filter.
type Filter struct {
	Eq      any     `json:"eq,omitempty"`
	Neq     any     `json:"neq,omitempty"`
	Gt      any     `json:"gt,omitempty"`
	Gte     any     `json:"gte,omitempty"`
	Lt      any     `json:"lt,omitempty"`
	Lte     any     `json:"lte,omitempty"`
	Like    string  `json:"like,omitempty"`
	Glob    string  `json:"glob,omitempty"`
	In      []any   `json:"in,omitempty"`
	Between []any   `json:"between,omitempty"` // Exactly 2 elements
	Is      any     `json:"is,omitempty"`      // null, true, false
	Fts     string  `json:"fts,omitempty"`     // Full-text search query
	Not     *Filter `json:"not,omitempty"`     // Negation wrapper
}

// BatchRequest represents a JSON batch request body.
// Used with POST /batch.
type BatchRequest struct {
	Operations []BatchOperation `json:"operations"`
}

// BatchOperation represents a single operation within a batch.
type BatchOperation struct {
	Operation string         `json:"operation"` // select, insert, upsert, update, delete
	Table     string         `json:"table"`
	Body      map[string]any `json:"body"` // Operation-specific body
}

// BatchResponse represents the response from a batch request.
type BatchResponse struct {
	Results []any `json:"results"`
}

// Prefer header values
const (
	PreferOperationSelect  = "operation=select"
	PreferOnConflictReplace = "on-conflict=replace"
	PreferOnConflictIgnore  = "on-conflict=ignore"
	PreferCountExact        = "count=exact"
)
