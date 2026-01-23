package data

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"log"

	"github.com/joe-ervin05/atomicbase/tools"
)

// GetCachedSchema retrieves a schema from cache, loading it from DB if not present.
// For tenant databases, this uses template_id and schema_version.
// For the primary database (templateID=0), it returns the primary schema.
func GetCachedSchema(db *sql.DB, templateID int32, version int) (SchemaCache, error) {
	// Primary database has its own schema management
	if templateID == 0 {
		schemaMu.RLock()
		schema := primarySchema
		schemaMu.RUnlock()
		return schema, nil
	}

	// Check cache first
	if cached, ok := tools.SchemaFromCache(templateID, version); ok {
		return cached.(SchemaCache), nil
	}

	// Load from templates_history table
	schema, err := loadSchemaFromHistory(db, templateID, version)
	if err != nil {
		return SchemaCache{}, err
	}

	// Store in cache
	tools.SchemaCache(templateID, version, schema)

	return schema, nil
}

// loadSchemaFromHistory loads a schema from the templates_history table.
func loadSchemaFromHistory(db *sql.DB, templateID int32, version int) (SchemaCache, error) {
	if db == nil {
		return SchemaCache{}, errors.New("Can't load schema from nil database")
	}

	row := db.QueryRow(fmt.Sprintf(`
		SELECT schema FROM %s WHERE template_id = ? AND version = ?
	`, ReservedTableTemplatesHistory), templateID, version)

	var tablesData []byte
	if err := row.Scan(&tablesData); err != nil {
		if err == sql.ErrNoRows {
			return SchemaCache{}, fmt.Errorf("schema version %d not found for template %d", version, templateID)
		}
		return SchemaCache{}, err
	}

	// Deserialize tables
	buf := bytes.NewBuffer(tablesData)
	dec := gob.NewDecoder(buf)
	var tables []Table
	if err := dec.Decode(&tables); err != nil {
		return SchemaCache{}, fmt.Errorf("failed to decode schema tables: %w", err)
	}

	// Convert []Table to SchemaCache format
	return TablesToSchemaCache(tables), nil
}

// TablesToSchemaCache converts a slice of Table definitions to a SchemaCache.
func TablesToSchemaCache(tables []Table) SchemaCache {
	cache := SchemaCache{
		Tables:    make(map[string]Table),
		Fks:       make(map[string][]Fk),
		FTSTables: make(map[string]bool),
	}

	for _, t := range tables {
		cache.Tables[t.Name] = t

		// Extract foreign keys from column references
		for _, col := range t.Columns {
			if col.References != "" {
				// Parse "table.column" format
				for i := 0; i < len(col.References); i++ {
					if col.References[i] == '.' {
						refTable := col.References[:i]
						refCol := col.References[i+1:]
						fk := Fk{
							Table:      t.Name,
							References: refTable,
							From:       col.Name,
							To:         refCol,
						}
						cache.Fks[t.Name] = append(cache.Fks[t.Name], fk)
						break
					}
				}
			}
		}
	}

	return cache
}

// PreloadSchemaCache loads current schema versions into cache.
// Only loads the current version for each template to minimize memory usage.
func PreloadSchemaCache(db *sql.DB) error {
	rows, err := db.Query(fmt.Sprintf(`
		SELECT h.template_id, h.version, h.schema
		FROM %s h
		JOIN %s t ON h.template_id = t.id AND h.version = t.current_version
	`, ReservedTableTemplatesHistory, ReservedTableTemplates))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var templateID int32
		var version int
		var tablesData []byte

		if err := rows.Scan(&templateID, &version, &tablesData); err != nil {
			return err
		}

		buf := bytes.NewBuffer(tablesData)
		dec := gob.NewDecoder(buf)
		var tables []Table
		if err := dec.Decode(&tables); err != nil {
			log.Printf("Warning: failed to decode schema cache for template %d version %d: %v", templateID, version, err)
			continue
		}

		tools.SchemaCache(templateID, version, TablesToSchemaCache(tables))
	}

	return rows.Err()
}
