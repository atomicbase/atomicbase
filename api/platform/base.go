package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
)

// Table names for internal platform tables.
const (
	TableTemplates         = "atomicbase_schema_templates"
	TableTemplatesHistory  = "atomicbase_templates_history"
	TableDatabases         = "atomicbase_databases"
	TableMigrations        = "atomicbase_migrations"
	TableMigrationFailures = "atomicbase_migration_failures"
)

// Primary database connection for platform operations.
var (
	db   *sql.DB
	dbMu sync.RWMutex
)

// InitDB initializes platform access to the shared primary database connection.
func InitDB(conn *sql.DB) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	if conn == nil {
		return errors.New("nil primary database connection")
	}

	db = conn
	return nil
}

// CloseDB detaches platform from the shared database connection.
// The caller that created the shared connection is responsible for closing it.
func CloseDB() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	db = nil
	return nil
}

// getDB returns the database connection.
func getDB() (*sql.DB, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	if db == nil {
		return nil, errors.New("platform database not initialized")
	}
	return db, nil
}

// queryJSON executes a query and returns results as JSON bytes.
func queryJSON(ctx context.Context, query string, args ...any) ([]byte, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]any)
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return json.Marshal(results)
}
