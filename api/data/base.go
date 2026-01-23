package data

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
	"strings"
	"sync"

	"github.com/joe-ervin05/atomicbase/config"
	"github.com/joe-ervin05/atomicbase/tools"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

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

	db, err := sql.Open("sqlite3", "file:"+config.Cfg.PrimaryDBPath)
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
		created_at TEXT DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_atomicbase_templates_name ON %s(name);
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
	CREATE UNIQUE INDEX IF NOT EXISTS idx_atomicbase_databases_name ON %s(name);
	INSERT INTO %s (id, schema) values(1, ?) ON CONFLICT (id) DO NOTHING;
	`, ReservedTableDatabases, ReservedTableTemplates, ReservedTableDatabases, ReservedTableDatabases), buf.Bytes())
	if err != nil {
		db.Close()
		return err
	}

	// Migration: Add template_id column if it doesn't exist (for existing databases)
	// SQLite doesn't have IF NOT EXISTS for ALTER TABLE, so we check for the expected error
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN template_id INTEGER REFERENCES %s(id)`,
		ReservedTableDatabases, ReservedTableTemplates))
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		db.Close()
		return fmt.Errorf("migration failed: %w", err)
	}

	// Create templates_history table for version tracking
	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s
	(
		id INTEGER PRIMARY KEY,
		template_id INTEGER NOT NULL REFERENCES %s(id),
		version INTEGER NOT NULL,
		schema BLOB NOT NULL,
		checksum TEXT NOT NULL,
		changes TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(template_id, version)
	);
	CREATE INDEX IF NOT EXISTS idx_templates_history_template ON %s(template_id);
	`, ReservedTableTemplatesHistory, ReservedTableTemplates, ReservedTableTemplatesHistory))
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to create templates_history table: %w", err)
	}

	// Migration: Add current_version column to templates table
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN current_version INTEGER DEFAULT 1`,
		ReservedTableTemplates))
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		db.Close()
		return fmt.Errorf("migration failed: %w", err)
	}

	// Migration: Add schema_version column to databases table
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN schema_version INTEGER DEFAULT 1`,
		ReservedTableDatabases))
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		db.Close()
		return fmt.Errorf("migration failed: %w", err)
	}

	// Migration: Rename tables column to schema in templates_history
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME COLUMN tables TO schema`,
		ReservedTableTemplatesHistory))
	if err != nil && !strings.Contains(err.Error(), "no such column") {
		db.Close()
		return fmt.Errorf("migration failed: %w", err)
	}

	primaryDB = db

	// Load schema for primary database
	dao := PrimaryDao{Database: Database{
		Client:        db,
		Schema:        SchemaCache{},
		ID:            1,
		TemplateID:    0, // Primary database doesn't use templates
		SchemaVersion: 0,
	}}
	if err := dao.Database.updateSchema(); err != nil {
		primaryDB = nil
		db.Close()
		return err
	}
	primarySchema = dao.Schema

	// Preload template schemas into cache
	if err := PreloadSchemaCache(db); err != nil {
		// Non-fatal - cache will be populated on demand
		log.Printf("Warning: failed to preload schema cache: %v", err)
	}

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
			Client:        primaryDB,
			Schema:        schema,
			ID:            1,
			TemplateID:    0,
			SchemaVersion: 0,
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
	org := config.Cfg.TursoOrganization

	if org == "" {
		return Database{}, errors.New("TURSO_ORGANIZATION environment variable is not set but is required to access external databases")
	}

	row := dao.Client.QueryRow(fmt.Sprintf(
		"SELECT id, token, COALESCE(template_id, 0), COALESCE(schema_version, 1) FROM %s WHERE name = ?",
		ReservedTableDatabases), dbName)

	var id sql.NullInt32
	var token sql.NullString
	var templateID int32
	var schemaVersion int

	err := row.Scan(&id, &token, &templateID, &schemaVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Database{}, tools.ErrDatabaseNotFound
		}
		return Database{}, err
	}

	if !token.Valid {
		return Database{}, errors.New("database token is missing")
	}

	// Get schema from cache (uses template_id and version)
	schema, err := GetCachedSchema(dao.Client, templateID, schemaVersion)
	if err != nil {
		return Database{}, fmt.Errorf("failed to load schema: %w", err)
	}

	client, err := sql.Open("libsql", fmt.Sprintf("libsql://%s-%s.turso.io?authToken=%s", dbName, org, token.String))
	if err != nil {
		return Database{}, err
	}

	err = client.Ping()
	if err != nil {
		client.Close()
		return Database{}, err
	}

	return Database{
		Client:        client,
		Schema:        schema,
		ID:            id.Int32,
		TemplateID:    templateID,
		SchemaVersion: schemaVersion,
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

// LoadSchema deserializes a SchemaCache from gob-encoded bytes.
func LoadSchema(data []byte) (SchemaCache, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	var schema SchemaCache

	err := dec.Decode(&schema)

	return schema, err
}
