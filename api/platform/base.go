package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"

	"github.com/atomicbase/atomicbase/config"
	_ "github.com/mattn/go-sqlite3"
)

// Table names for internal platform tables.
const (
	TableTemplates          = "atomicbase_schema_templates"
	TableTemplatesHistory   = "atomicbase_templates_history"
	TableDatabases          = "atomicbase_databases"
	TableMigrations         = "atomicbase_migrations"
	TableDatabaseMigrations = "atomicbase_database_migrations"
)

// Primary database connection for platform operations.
var (
	db   *sql.DB
	dbMu sync.RWMutex
)

// InitDB initializes the platform database connection.
// Must be called during server startup before using platform functions.
func InitDB() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	if db != nil {
		return nil // Already initialized
	}

	conn, err := sql.Open("sqlite3", "file:"+config.Cfg.PrimaryDBPath)
	if err != nil {
		conn, err = sql.Open("sqlite3", "file:atomicdata/databases.db")
		if err != nil {
			return err
		}
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return err
	}

	// Create migrations table if it doesn't exist
	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS ` + TableMigrations + ` (
			id INTEGER PRIMARY KEY,
			template_id INTEGER NOT NULL REFERENCES ` + TableTemplates + `(id),
			from_version INTEGER NOT NULL,
			to_version INTEGER NOT NULL,
			sql TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			state TEXT,
			total_dbs INTEGER NOT NULL DEFAULT 0,
			completed_dbs INTEGER NOT NULL DEFAULT 0,
			failed_dbs INTEGER NOT NULL DEFAULT 0,
			started_at TEXT,
			completed_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_migrations_template ON ` + TableMigrations + `(template_id);
		CREATE INDEX IF NOT EXISTS idx_migrations_status ON ` + TableMigrations + `(status);
	`)
	if err != nil {
		conn.Close()
		return err
	}

	// Create tenant_migrations table if it doesn't exist
	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS ` + TableDatabaseMigrations + ` (
			migration_id INTEGER NOT NULL REFERENCES ` + TableMigrations + `(id),
			database_id INTEGER NOT NULL REFERENCES ` + TableDatabases + `(id),
			status TEXT NOT NULL,
			error TEXT,
			attempts INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (migration_id, database_id)
		);
		CREATE INDEX IF NOT EXISTS idx_database_migrations_status ON ` + TableDatabaseMigrations + `(status);
	`)
	if err != nil {
		conn.Close()
		return err
	}

	db = conn
	return nil
}

// CloseDB closes the platform database connection.
func CloseDB() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	if db != nil {
		err := db.Close()
		db = nil
		return err
	}
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
