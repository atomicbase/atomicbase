package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/joe-ervin05/atomicbase/config"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

// PrimaryDao wraps Database for operations on the primary/local database.
type PrimaryDao struct {
	Database
}

// Database represents a connection to a SQLite/Turso database with cached schema.
type Database struct {
	Client *sql.DB     // SQL database connection
	Schema SchemaCache // Cached schema for validation
	id     int32       // Internal database ID (1 = primary)
}

// SchemaCache holds cached table and foreign key information for query validation.
type SchemaCache struct {
	Tables    []Table  // Sorted by table name
	Fks       []Fk     // Sorted by table, then references
	FTSTables []string // Sorted list of tables that have FTS5 indexes (table names without _fts suffix)
}

// Fk represents a foreign key relationship between tables.
type Fk struct {
	Table      string // Table containing the FK column
	References string // Referenced table
	From       string // FK column name
	To         string // Referenced column name
}

// Table represents a database table's schema.
type Table struct {
	Name    string `json:"name"`    // Table name
	Pk      string `json:"pk"`      // Primary key column name (empty if rowid)
	Columns []Col  `json:"columns"` // Sorted by column name
}

// Col represents a column definition.
type Col struct {
	Name       string `json:"name"`                 // Column name
	Type       string `json:"type"`                 // SQLite type (TEXT, INTEGER, REAL, BLOB)
	NotNull    bool   `json:"notNull,omitempty"`    // NOT NULL constraint
	Default    any    `json:"default,omitempty"`    // Default value (nil if none)
	References string `json:"references,omitempty"` // Foreign key reference (format: "table.column")
}

// SchemaTemplate represents a reusable schema template for multi-tenant databases.
// Daughter databases associated with a template will inherit its schema.
type SchemaTemplate struct {
	ID        int32   `json:"id"`
	Name      string  `json:"name"`
	Tables    []Table `json:"tables"`    // Table definitions for this template
	CreatedAt string  `json:"createdAt"` // ISO timestamp
	UpdatedAt string  `json:"updatedAt"` // ISO timestamp
}

// Primary database connection (reused across requests)
var (
	primaryDB     *sql.DB
	primarySchema SchemaCache
	schemaMu      sync.RWMutex
)

func init() {
	if err := initPrimaryDB(); err != nil {
		log.Fatal(err)
	}
}

func initPrimaryDB() error {
	if err := os.MkdirAll(config.Cfg.DataDir, os.ModePerm); err != nil {
		return err
	}

	db, err := sql.Open("libsql", "file:"+config.Cfg.PrimaryDBPath)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return err
	}

	var buf bytes.Buffer
	gob.NewEncoder(&buf).Encode(SchemaCache{})

	// Create templates table first (referenced by databases)
	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s
	(
		id INTEGER PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		tables BLOB NOT NULL,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_ab_templates_name ON %s(name);
	`, ReservedTableTemplates, ReservedTableTemplates))
	if err != nil {
		db.Close()
		return err
	}

	// Create databases table with template_id reference
	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s
	(
		id INTEGER PRIMARY KEY,
		name TEXT UNIQUE,
		token TEXT,
		schema BLOB,
		template_id INTEGER REFERENCES %s(id)
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_ab_databases_name ON %s(name);
	INSERT INTO %s (id, schema) values(1, ?) ON CONFLICT (id) DO NOTHING;
	`, ReservedTableDatabases, ReservedTableTemplates, ReservedTableDatabases, ReservedTableDatabases), buf.Bytes())
	if err != nil {
		db.Close()
		return err
	}

	// Migration: Add template_id column if it doesn't exist (for existing databases)
	// SQLite doesn't have IF NOT EXISTS for ALTER TABLE, so we just try and ignore the error
	_, _ = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN template_id INTEGER REFERENCES %s(id)`,
		ReservedTableDatabases, ReservedTableTemplates))

	primaryDB = db

	// Load schema
	dao := PrimaryDao{Database: Database{db, SchemaCache{}, 1}}
	if err := dao.updateSchema(); err != nil {
		db.Close()
		return err
	}
	primarySchema = dao.Schema

	return nil
}

// ClosePrimaryDB closes the primary database connection. Call on shutdown.
func ClosePrimaryDB() error {
	if primaryDB != nil {
		return primaryDB.Close()
	}
	return nil
}

// ConnPrimary returns a reference to the primary database.
// Do NOT close the returned connection - it's shared.
func ConnPrimary() (PrimaryDao, error) {
	if primaryDB == nil {
		return PrimaryDao{}, errors.New("primary database not initialized")
	}

	schemaMu.RLock()
	schema := primarySchema
	schemaMu.RUnlock()

	return PrimaryDao{
		Database: Database{
			Client: primaryDB,
			Schema: schema,
			id:     1,
		},
	}, nil
}

// updatePrimarySchema updates the cached schema for the primary database.
func updatePrimarySchema(schema SchemaCache) {
	schemaMu.Lock()
	primarySchema = schema
	schemaMu.Unlock()
}

// ConnTurso opens a connection to an external Turso database by name.
func (dao PrimaryDao) ConnTurso(dbName string) (Database, error) {
	org := os.Getenv("TURSO_ORGANIZATION")

	if org == "" {
		return Database{}, errors.New("TURSO_ORGANIZATION environment variable is not set but is required to access external databases")
	}

	row := dao.Client.QueryRow(fmt.Sprintf("SELECT id, token, schema from %s WHERE name = ?", ReservedTableDatabases), dbName)

	var id sql.NullInt32
	var token sql.NullString
	var sData []byte

	err := row.Scan(&id, &token, &sData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Database{}, ErrDatabaseNotFound
		}
		return Database{}, err
	}

	schema, err := loadSchema(sData)

	if err != nil {
		return Database{}, err
	}

	client, err := sql.Open("libsql", fmt.Sprintf("libsql://%s-%s.turso.io?authToken=%s", dbName, org, token.String))
	if err != nil {
		return Database{}, err
	}

	err = client.Ping()

	if err != nil {
		return Database{}, err
	}

	return Database{
		client, schema, id.Int32,
	}, nil
}

// QueryMap executes a query and returns results as a slice of maps.
func (dao *Database) QueryMap(ctx context.Context, query string, args ...any) ([]interface{}, error) {
	rows, err := dao.Client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()

	if err != nil {
		return nil, err
	}

	count := len(columnTypes)
	finalRows := []interface{}{}

	for rows.Next() {

		scanArgs := make([]interface{}, count)

		for i, v := range columnTypes {

			// doesnt use scanType to support more sqlite drivers
			switch v.DatabaseTypeName() {
			case "TEXT":
				scanArgs[i] = new(sql.NullString)
			case "INTEGER":
				scanArgs[i] = new(sql.NullInt64)
			case "REAL":
				scanArgs[i] = new(sql.NullFloat64)
			case "BLOB":
				scanArgs[i] = new(sql.RawBytes)
			default:
				scanArgs[i] = new(sql.NullString)
			}
		}

		err := rows.Scan(scanArgs...)

		if err != nil {
			return nil, err
		}

		masterData := map[string]interface{}{}

		for i, v := range columnTypes {
			if z, ok := (scanArgs[i]).(*sql.NullString); ok {
				if z.Valid {
					masterData[v.Name()] = z.String
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt64); ok {
				if z.Valid {
					masterData[v.Name()] = z.Int64
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullFloat64); ok {
				if z.Valid {
					masterData[v.Name()] = z.Float64
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			masterData[v.Name()] = scanArgs[i]
		}

		finalRows = append(finalRows, masterData)
	}

	return finalRows, nil
}

// QueryJSON executes a query and returns results as JSON bytes.
func (dao *Database) QueryJSON(ctx context.Context, query string, args ...any) ([]byte, error) {
	m, err := dao.QueryMap(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return json.Marshal(&m)
}
