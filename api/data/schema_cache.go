package data

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/atomicbase/atomicbase/tools"
)

// GetCachedTemplate retrieves the current schema and version for a template.
// For tenant databases, this uses template_id to get the current version.
// For the primary database (templateID=0), it returns the primary schema with version 0.
func GetCachedTemplate(db *sql.DB, templateID int32) (SchemaCache, int, error) {
	// Primary database has its own schema management
	if templateID == 0 {
		schemaMu.RLock()
		schema := primarySchema
		schemaMu.RUnlock()
		return schema, 0, nil
	}

	// Check cache first
	if cached, ok := tools.GetTemplate(templateID); ok {
		return cached.Schema.(SchemaCache), cached.Version, nil
	}

	// Load from database and cache
	schema, version, err := loadCurrentSchemaFromDB(db, templateID)
	if err != nil {
		return SchemaCache{}, 0, err
	}

	tools.SetTemplate(templateID, version, schema)
	return schema, version, nil
}

// loadCurrentSchemaFromDB loads the current schema version for a template.
func loadCurrentSchemaFromDB(db *sql.DB, templateID int32) (SchemaCache, int, error) {
	if db == nil {
		return SchemaCache{}, 0, fmt.Errorf("cannot load schema from nil database")
	}

	row := db.QueryRow(fmt.Sprintf(`
		SELECT h.version, h.schema
		FROM %s h
		JOIN %s t ON h.template_id = t.id AND h.version = t.current_version
		WHERE h.template_id = ?
	`, ReservedTableTemplatesHistory, ReservedTableTemplates), templateID)

	var version int
	var tablesData []byte
	if err := row.Scan(&version, &tablesData); err != nil {
		if err == sql.ErrNoRows {
			return SchemaCache{}, 0, fmt.Errorf("schema not found for template %d", templateID)
		}
		return SchemaCache{}, 0, err
	}

	// Deserialize schema from JSON (format: {"tables": [...]})
	var schema struct {
		Tables []Table `json:"tables"`
	}
	if err := tools.DecodeSchema(tablesData, &schema); err != nil {
		return SchemaCache{}, 0, err
	}

	return TablesToSchemaCache(schema.Tables), version, nil
}

// TablesToSchemaCache converts a slice of Table definitions to a SchemaCache.
func TablesToSchemaCache(tables []Table) SchemaCache {
	cache := SchemaCache{
		Tables:    make(map[string]CacheTable),
		Fks:       make(map[string][]CacheFk),
		FTSTables: make(map[string]bool),
	}

	for _, t := range tables {
		tbl := CacheTable{
			Name:    t.Name,
			Pk:      t.Pk,
			Columns: make(map[string]string),
		}
		// Extract foreign keys from column references
		for _, col := range t.Columns {

			tbl.Columns[col.Name] = col.Type

			if col.References != "" {
				// Parse "table.column" format
				for i := 0; i < len(col.References); i++ {
					if col.References[i] == '.' {
						refTable := col.References[:i]
						refCol := col.References[i+1:]
						fk := CacheFk{
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
		cache.Tables[t.Name] = tbl
	}

	return cache
}

// PreloadSchemaCache loads current schema versions into cache.
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

		var schema struct {
			Tables []Table `json:"tables"`
		}
		if err := tools.DecodeSchema(tablesData, &schema); err != nil {
			log.Printf("Warning: failed to decode schema cache for template %d version %d: %v", templateID, version, err)
			continue
		}

		tools.SetTemplate(templateID, version, TablesToSchemaCache(schema.Tables))
	}

	return rows.Err()
}
