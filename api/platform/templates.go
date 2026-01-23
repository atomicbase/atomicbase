package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/joe-ervin05/atomicbase/data"
	"github.com/joe-ervin05/atomicbase/tools"
)

// CreateTemplate creates a new schema template with initial version.
func CreateTemplate(ctx context.Context, dao data.PrimaryDao, name string, tables []data.Table) (data.SchemaTemplate, error) {
	// Serialize tables using gob
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tables); err != nil {
		return data.SchemaTemplate{}, fmt.Errorf("failed to encode tables: %w", err)
	}

	checksum := computeChecksum(tables)

	// Begin transaction
	tx, err := dao.Client.BeginTx(ctx, nil)
	if err != nil {
		return data.SchemaTemplate{}, err
	}
	defer tx.Rollback()

	// Insert template with version 1
	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (name, current_version) VALUES (?, 1)
	`, data.ReservedTableTemplates), name)
	if err != nil {
		return data.SchemaTemplate{}, err
	}

	templateID, err := result.LastInsertId()
	if err != nil {
		return data.SchemaTemplate{}, err
	}

	// Create initial version in history
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (template_id, version, schema, checksum) VALUES (?, 1, ?, ?)
	`, data.ReservedTableTemplatesHistory), templateID, buf.Bytes(), checksum)
	if err != nil {
		return data.SchemaTemplate{}, err
	}

	if err := tx.Commit(); err != nil {
		return data.SchemaTemplate{}, err
	}

	// Fetch the created template to get timestamps
	return GetTemplate(ctx, dao, name)
}

// GetTemplate retrieves a template by name.
func GetTemplate(ctx context.Context, dao data.PrimaryDao, name string) (data.SchemaTemplate, error) {
	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT t.id, t.name, h.schema, COALESCE(t.current_version, 1), t.created_at, t.updated_at
		FROM %s t
		JOIN %s h ON h.template_id = t.id AND h.version = COALESCE(t.current_version, 1)
		WHERE t.name = ?
	`, data.ReservedTableTemplates, data.ReservedTableTemplatesHistory), name)

	var template data.SchemaTemplate
	var tablesData []byte
	err := row.Scan(&template.ID, &template.Name, &tablesData, &template.CurrentVersion, &template.CreatedAt, &template.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return data.SchemaTemplate{}, tools.ErrTemplateNotFound
		}
		return data.SchemaTemplate{}, err
	}

	// Deserialize tables
	buf := bytes.NewBuffer(tablesData)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&template.Tables); err != nil {
		return data.SchemaTemplate{}, fmt.Errorf("failed to decode tables: %w", err)
	}

	return template, nil
}

// ListTemplates returns all schema templates.
func ListTemplates(ctx context.Context, dao data.PrimaryDao) ([]data.SchemaTemplate, error) {
	rows, err := dao.Client.QueryContext(ctx, fmt.Sprintf(`
		SELECT t.id, t.name, h.schema, COALESCE(t.current_version, 1), t.created_at, t.updated_at
		FROM %s t
		JOIN %s h ON h.template_id = t.id AND h.version = COALESCE(t.current_version, 1)
		ORDER BY t.name ASC
	`, data.ReservedTableTemplates, data.ReservedTableTemplatesHistory))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []data.SchemaTemplate
	for rows.Next() {
		var template data.SchemaTemplate
		var tablesData []byte
		err := rows.Scan(&template.ID, &template.Name, &tablesData, &template.CurrentVersion, &template.CreatedAt, &template.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Deserialize tables
		buf := bytes.NewBuffer(tablesData)
		dec := gob.NewDecoder(buf)
		if err := dec.Decode(&template.Tables); err != nil {
			return nil, fmt.Errorf("failed to decode tables for template %s: %w", template.Name, err)
		}

		templates = append(templates, template)
	}

	return templates, rows.Err()
}

// ListTemplatesJSON returns all templates as JSON bytes.
func ListTemplatesJSON(ctx context.Context, dao data.PrimaryDao) ([]byte, error) {
	templates, err := ListTemplates(ctx, dao)
	if err != nil {
		return nil, err
	}
	if templates == nil {
		templates = []data.SchemaTemplate{}
	}
	return json.Marshal(templates)
}

// DeleteTemplate deletes a template by name.
// Returns error if template is still associated with databases.
func DeleteTemplate(ctx context.Context, dao data.PrimaryDao, name string) error {
	// First check if any databases are using this template
	var templateID int32
	err := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE name = ?
	`, data.ReservedTableTemplates), name).Scan(&templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tools.ErrTemplateNotFound
		}
		return err
	}

	var count int
	err = dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE template_id = ?
	`, data.ReservedTableDatabases), templateID).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return tools.ErrTemplateInUse
	}

	_, err = dao.Client.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE name = ?
	`, data.ReservedTableTemplates), name)
	return err
}

// ListDatabasesByTemplate returns all databases associated with a template.
func ListDatabasesByTemplate(ctx context.Context, dao data.PrimaryDao, templateName string) ([]string, error) {
	// Get template ID
	var templateID int32
	err := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE name = ?
	`, data.ReservedTableTemplates), templateName).Scan(&templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, tools.ErrTemplateNotFound
		}
		return nil, err
	}

	rows, err := dao.Client.QueryContext(ctx, fmt.Sprintf(`
		SELECT name FROM %s WHERE template_id = ? AND name IS NOT NULL ORDER BY name ASC
	`, data.ReservedTableDatabases), templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		databases = append(databases, name)
	}

	return databases, rows.Err()
}

// syncSchemaToDatabase applies template tables to a database.
func syncSchemaToDatabase(ctx context.Context, db *data.Database, templateTables []data.Table, dropExtra bool) ([]string, error) {
	var changes []string

	// Build map of template tables for quick lookup
	templateMap := make(map[string]data.Table)
	for _, t := range templateTables {
		templateMap[t.Name] = t
	}

	// Get current tables from database (refresh schema)
	currentTables, err := data.SchemaCols(db.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to get current schema: %w", err)
	}

	// Filter out internal tables
	currentMap := make(map[string]data.Table)
	for name, t := range currentTables {
		// Skip internal tables
		if len(name) >= len(data.InternalTablePrefix) && name[:len(data.InternalTablePrefix)] == data.InternalTablePrefix {
			continue
		}
		currentMap[name] = t
	}

	// Drop extra tables if requested
	if dropExtra {
		for tableName := range currentMap {
			if _, exists := templateMap[tableName]; !exists {
				_, err := db.Client.ExecContext(ctx, fmt.Sprintf("DROP TABLE [%s]", tableName))
				if err != nil {
					return changes, fmt.Errorf("failed to drop table %s: %w", tableName, err)
				}
				changes = append(changes, fmt.Sprintf("dropped table: %s", tableName))
			}
		}
	}

	// Create missing tables and sync columns
	for tableName, templateTable := range templateMap {
		currentTable, exists := currentMap[tableName]
		if !exists {
			// Create the table
			query := buildCreateTableQuery(tableName, templateTable)
			_, err := db.Client.ExecContext(ctx, query)
			if err != nil {
				return changes, fmt.Errorf("failed to create table %s: %w", tableName, err)
			}
			changes = append(changes, fmt.Sprintf("created table: %s", tableName))

			// Create indexes for the new table
			for _, idx := range templateTable.Indexes {
				idxSQL := buildCreateIndexSQL(tableName, idx)
				_, err := db.Client.ExecContext(ctx, idxSQL)
				if err != nil {
					return changes, fmt.Errorf("failed to create index %s: %w", idx.Name, err)
				}
				changes = append(changes, fmt.Sprintf("created index: %s", idx.Name))
			}

			// Create FTS5 virtual table if ftsColumns is specified
			if len(templateTable.FTSColumns) > 0 {
				ftsSQL := buildCreateFTSSQL(tableName, templateTable.FTSColumns)
				_, err := db.Client.ExecContext(ctx, ftsSQL)
				if err != nil {
					return changes, fmt.Errorf("failed to create FTS table for %s: %w", tableName, err)
				}
				changes = append(changes, fmt.Sprintf("created FTS: %s_fts", tableName))
			}
		} else {
			// Table exists, sync columns
			colChanges, err := syncTableColumns(ctx, db, tableName, currentTable, templateTable)
			if err != nil {
				return changes, err
			}
			changes = append(changes, colChanges...)
		}
	}

	// Invalidate schema cache after changes
	if len(changes) > 0 {
		if err := db.InvalidateSchema(ctx); err != nil {
			return changes, fmt.Errorf("failed to update schema cache: %w", err)
		}
	}

	return changes, nil
}

// buildCreateTableQuery generates a CREATE TABLE statement from a Table definition.
func buildCreateTableQuery(name string, table data.Table) string {
	query := fmt.Sprintf("CREATE TABLE [%s] (", name)

	var fks []string
	var checks []string

	// Check if we have a composite primary key
	isCompositePk := len(table.Pk) > 1

	for _, col := range table.Columns {
		// Generated columns have special syntax
		if col.Generated != nil {
			storage := "VIRTUAL"
			if col.Generated.Stored {
				storage = "STORED"
			}
			query += fmt.Sprintf("[%s] %s GENERATED ALWAYS AS (%s) %s ", col.Name, col.Type, col.Generated.Expr, storage)
		} else {
			query += fmt.Sprintf("[%s] %s ", col.Name, col.Type)
		}

		// Only add inline PRIMARY KEY for single-column primary keys
		if !isCompositePk && len(table.Pk) == 1 && col.Name == table.Pk[0] {
			query += "PRIMARY KEY "
		}
		if col.NotNull {
			query += "NOT NULL "
		}
		if col.Unique {
			query += "UNIQUE "
		}
		if col.Collate != "" {
			query += "COLLATE " + col.Collate + " "
		}
		if col.Default != nil && col.Generated == nil {
			switch v := col.Default.(type) {
			case string:
				query += fmt.Sprintf(`DEFAULT "%s" `, v)
			case float64:
				query += fmt.Sprintf("DEFAULT %g ", v)
			case int:
				query += fmt.Sprintf("DEFAULT %d ", v)
			}
		}
		if col.Check != "" {
			// Column-level CHECK with column name prefix for uniqueness
			checks = append(checks, fmt.Sprintf("CHECK(%s)", col.Check))
		}
		if col.References != "" {
			// Parse references (format: "table.column")
			parts := splitReference(col.References)
			if len(parts) == 2 {
				fk := fmt.Sprintf("FOREIGN KEY([%s]) REFERENCES [%s]([%s])", col.Name, parts[0], parts[1])
				if col.OnDelete != "" {
					fk += " ON DELETE " + col.OnDelete
				}
				if col.OnUpdate != "" {
					fk += " ON UPDATE " + col.OnUpdate
				}
				fks = append(fks, fk)
			}
		}

		query += ", "
	}

	// Add composite primary key constraint if needed
	if isCompositePk {
		pkCols := make([]string, len(table.Pk))
		for i, col := range table.Pk {
			pkCols[i] = fmt.Sprintf("[%s]", col)
		}
		query += fmt.Sprintf("PRIMARY KEY (%s), ", strings.Join(pkCols, ", "))
	}

	for _, fk := range fks {
		query += fk + ", "
	}

	for _, chk := range checks {
		query += chk + ", "
	}

	// Remove trailing comma and space, close parenthesis
	query = query[:len(query)-2] + ")"
	return query
}

// syncTableColumns adds missing columns to an existing table.
func syncTableColumns(ctx context.Context, db *data.Database, tableName string, current data.Table, template data.Table) ([]string, error) {
	var changes []string

	// Add missing columns (current.Columns and template.Columns are already maps)
	for colName, templateCol := range template.Columns {
		if _, exists := current.Columns[colName]; !exists {
			query := fmt.Sprintf("ALTER TABLE [%s] ADD COLUMN [%s] %s ", tableName, templateCol.Name, templateCol.Type)

			if templateCol.NotNull {
				query += "NOT NULL "
			}
			if templateCol.Default != nil {
				switch v := templateCol.Default.(type) {
				case string:
					query += fmt.Sprintf(`DEFAULT "%s" `, v)
				case float64:
					query += fmt.Sprintf("DEFAULT %g ", v)
				case int:
					query += fmt.Sprintf("DEFAULT %d ", v)
				}
			}
			if templateCol.References != "" {
				parts := splitReference(templateCol.References)
				if len(parts) == 2 {
					query += fmt.Sprintf("REFERENCES [%s]([%s]) ", parts[0], parts[1])
					if templateCol.OnDelete != "" {
						query += "ON DELETE " + templateCol.OnDelete + " "
					}
					if templateCol.OnUpdate != "" {
						query += "ON UPDATE " + templateCol.OnUpdate + " "
					}
				}
			}

			_, err := db.Client.ExecContext(ctx, query)
			if err != nil {
				return changes, fmt.Errorf("failed to add column %s.%s: %w", tableName, templateCol.Name, err)
			}
			changes = append(changes, fmt.Sprintf("added column: %s.%s", tableName, templateCol.Name))
		}
	}

	return changes, nil
}

// buildCreateFTSSQL generates a CREATE VIRTUAL TABLE statement for FTS5.
func buildCreateFTSSQL(tableName string, ftsColumns []string) string {
	cols := make([]string, len(ftsColumns))
	for i, c := range ftsColumns {
		cols[i] = fmt.Sprintf("[%s]", c)
	}
	return fmt.Sprintf("CREATE VIRTUAL TABLE [%s_fts] USING fts5(%s, content=[%s], content_rowid=rowid)",
		tableName, strings.Join(cols, ", "), tableName)
}

// buildCreateIndexSQL generates a CREATE INDEX statement.
func buildCreateIndexSQL(tableName string, idx data.Index) string {
	unique := ""
	if idx.Unique {
		unique = "UNIQUE "
	}
	cols := make([]string, len(idx.Columns))
	for i, c := range idx.Columns {
		cols[i] = fmt.Sprintf("[%s]", c)
	}
	return fmt.Sprintf("CREATE %sINDEX [%s] ON [%s] (%s)", unique, idx.Name, tableName, strings.Join(cols, ", "))
}

// splitReference splits a "table.column" reference string.
func splitReference(ref string) []string {
	for i := 0; i < len(ref); i++ {
		if ref[i] == '.' {
			return []string{ref[:i], ref[i+1:]}
		}
	}
	return nil
}

// DiffSchemas compares two sets of tables and returns the changes needed.
func DiffSchemas(oldTables, newTables []data.Table) []data.SchemaChange {
	var changes []data.SchemaChange

	// Build maps for easier lookup
	oldMap := make(map[string]data.Table)
	for _, t := range oldTables {
		oldMap[t.Name] = t
	}

	newMap := make(map[string]data.Table)
	for _, t := range newTables {
		newMap[t.Name] = t
	}

	// Find dropped tables
	for name := range oldMap {
		if _, exists := newMap[name]; !exists {
			changes = append(changes, data.SchemaChange{
				Type:        "drop_table",
				Table:       name,
				SQL:         fmt.Sprintf("DROP TABLE [%s]", name),
				RequiresMig: true,
			})
		}
	}

	// Find added tables and column changes
	for name, newTable := range newMap {
		oldTable, exists := oldMap[name]
		if !exists {
			// New table
			changes = append(changes, data.SchemaChange{
				Type:        "add_table",
				Table:       name,
				SQL:         buildCreateTableQuery(name, newTable),
				RequiresMig: false,
			})
		} else {
			// Table exists, compare columns
			colChanges := diffTableColumns(name, oldTable, newTable)
			changes = append(changes, colChanges...)
		}
	}

	return changes
}

// diffTableColumns compares columns between two table versions.
func diffTableColumns(tableName string, oldTable, newTable data.Table) []data.SchemaChange {
	var changes []data.SchemaChange

	// Find dropped columns
	for colName := range oldTable.Columns {
		if _, exists := newTable.Columns[colName]; !exists {
			changes = append(changes, data.SchemaChange{
				Type:        "drop_column",
				Table:       tableName,
				Column:      colName,
				RequiresMig: true, // SQLite doesn't support DROP COLUMN easily
			})
		}
	}

	// Find added columns
	for colName, newCol := range newTable.Columns {
		oldCol, exists := oldTable.Columns[colName]
		if !exists {
			changes = append(changes, data.SchemaChange{
				Type:        "add_column",
				Table:       tableName,
				Column:      colName,
				SQL:         buildAddColumnSQL(tableName, newCol),
				RequiresMig: false,
			})
		} else {
			// Column exists, check for modifications
			if columnsDiffer(oldCol, newCol) {
				changes = append(changes, data.SchemaChange{
					Type:        "modify_column",
					Table:       tableName,
					Column:      colName,
					RequiresMig: true, // SQLite requires table rebuild for column modifications
				})
			}
		}
	}

	// Compare indexes
	indexChanges := diffIndexes(tableName, oldTable.Indexes, newTable.Indexes)
	changes = append(changes, indexChanges...)

	// Compare FTS columns
	ftsChanges := diffFTS(tableName, oldTable.FTSColumns, newTable.FTSColumns)
	changes = append(changes, ftsChanges...)

	return changes
}

// diffIndexes compares indexes between two table versions.
func diffIndexes(tableName string, oldIndexes, newIndexes []data.Index) []data.SchemaChange {
	var changes []data.SchemaChange

	// Build maps for lookup
	oldMap := make(map[string]data.Index)
	for _, idx := range oldIndexes {
		oldMap[idx.Name] = idx
	}

	newMap := make(map[string]data.Index)
	for _, idx := range newIndexes {
		newMap[idx.Name] = idx
	}

	// Find dropped indexes
	for name := range oldMap {
		if _, exists := newMap[name]; !exists {
			changes = append(changes, data.SchemaChange{
				Type:        "drop_index",
				Table:       tableName,
				Column:      name, // Using Column field for index name
				SQL:         fmt.Sprintf("DROP INDEX [%s]", name),
				RequiresMig: false,
			})
		}
	}

	// Find added or modified indexes
	for name, newIdx := range newMap {
		oldIdx, exists := oldMap[name]
		if !exists {
			changes = append(changes, data.SchemaChange{
				Type:        "add_index",
				Table:       tableName,
				Column:      name,
				SQL:         buildCreateIndexSQL(tableName, newIdx),
				RequiresMig: false,
			})
		} else if indexesDiffer(oldIdx, newIdx) {
			// Index modified - drop and recreate
			changes = append(changes, data.SchemaChange{
				Type:        "drop_index",
				Table:       tableName,
				Column:      name,
				SQL:         fmt.Sprintf("DROP INDEX [%s]", name),
				RequiresMig: false,
			})
			changes = append(changes, data.SchemaChange{
				Type:        "add_index",
				Table:       tableName,
				Column:      name,
				SQL:         buildCreateIndexSQL(tableName, newIdx),
				RequiresMig: false,
			})
		}
	}

	return changes
}

// indexesDiffer checks if two index definitions are different.
func indexesDiffer(old, new data.Index) bool {
	if old.Unique != new.Unique {
		return true
	}
	if len(old.Columns) != len(new.Columns) {
		return true
	}
	for i, col := range old.Columns {
		if col != new.Columns[i] {
			return true
		}
	}
	return false
}

// diffFTS compares FTS columns between two table versions.
func diffFTS(tableName string, oldFTS, newFTS []string) []data.SchemaChange {
	var changes []data.SchemaChange

	// Check if FTS configuration changed
	if !stringSlicesEqual(oldFTS, newFTS) {
		if len(oldFTS) > 0 {
			// Drop old FTS table
			changes = append(changes, data.SchemaChange{
				Type:        "drop_fts",
				Table:       tableName,
				SQL:         fmt.Sprintf("DROP TABLE IF EXISTS [%s_fts]", tableName),
				RequiresMig: false,
			})
		}
		if len(newFTS) > 0 {
			// Create new FTS table
			changes = append(changes, data.SchemaChange{
				Type:        "add_fts",
				Table:       tableName,
				SQL:         buildCreateFTSSQL(tableName, newFTS),
				RequiresMig: false,
			})
		}
	}

	return changes
}

// stringSlicesEqual checks if two string slices are equal.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// columnsDiffer checks if two column definitions are different.
func columnsDiffer(old, new data.Col) bool {
	if old.Type != new.Type {
		return true
	}
	if old.NotNull != new.NotNull {
		return true
	}
	if old.Unique != new.Unique {
		return true
	}
	if old.Collate != new.Collate {
		return true
	}
	if old.Check != new.Check {
		return true
	}
	if old.References != new.References {
		return true
	}
	if old.OnDelete != new.OnDelete {
		return true
	}
	if old.OnUpdate != new.OnUpdate {
		return true
	}
	// Compare defaults (simplistic)
	if fmt.Sprintf("%v", old.Default) != fmt.Sprintf("%v", new.Default) {
		return true
	}
	// Compare generated columns
	if (old.Generated == nil) != (new.Generated == nil) {
		return true
	}
	if old.Generated != nil && new.Generated != nil {
		if old.Generated.Expr != new.Generated.Expr || old.Generated.Stored != new.Generated.Stored {
			return true
		}
	}
	return false
}

// buildAddColumnSQL generates an ALTER TABLE ADD COLUMN statement.
func buildAddColumnSQL(tableName string, col data.Col) string {
	query := fmt.Sprintf("ALTER TABLE [%s] ADD COLUMN [%s] %s", tableName, col.Name, col.Type)
	if col.NotNull {
		query += " NOT NULL"
	}
	if col.Default != nil {
		switch v := col.Default.(type) {
		case string:
			query += fmt.Sprintf(` DEFAULT "%s"`, v)
		case float64:
			query += fmt.Sprintf(" DEFAULT %g", v)
		case int:
			query += fmt.Sprintf(" DEFAULT %d", v)
		}
	}
	if col.References != "" {
		parts := splitReference(col.References)
		if len(parts) == 2 {
			query += fmt.Sprintf(" REFERENCES [%s]([%s])", parts[0], parts[1])
			if col.OnDelete != "" {
				query += " ON DELETE " + col.OnDelete
			}
			if col.OnUpdate != "" {
				query += " ON UPDATE " + col.OnUpdate
			}
		}
	}
	return query
}

// UpdateTemplate updates an existing template and creates a new version.
func UpdateTemplate(ctx context.Context, dao data.PrimaryDao, name string, tables []data.Table) (data.SchemaTemplate, []data.SchemaChange, error) {
	// Get current template
	current, err := GetTemplate(ctx, dao, name)
	if err != nil {
		return data.SchemaTemplate{}, nil, err
	}

	// Compute diff
	changes := DiffSchemas(current.Tables, tables)

	// If no changes, return current
	if len(changes) == 0 {
		return current, changes, nil
	}

	// Serialize new tables
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tables); err != nil {
		return data.SchemaTemplate{}, nil, fmt.Errorf("failed to encode tables: %w", err)
	}

	// Serialize changes for history
	changesJSON, err := json.Marshal(changes)
	if err != nil {
		return data.SchemaTemplate{}, nil, fmt.Errorf("failed to encode changes: %w", err)
	}

	// Compute checksum
	checksum := computeChecksum(tables)

	newVersion := current.CurrentVersion + 1

	// Begin transaction
	tx, err := dao.Client.BeginTx(ctx, nil)
	if err != nil {
		return data.SchemaTemplate{}, nil, err
	}
	defer tx.Rollback()

	// Update template
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET current_version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, data.ReservedTableTemplates), newVersion, current.ID)
	if err != nil {
		return data.SchemaTemplate{}, nil, err
	}

	// Insert version history
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (template_id, version, schema, checksum, changes) VALUES (?, ?, ?, ?, ?)
	`, data.ReservedTableTemplatesHistory), current.ID, newVersion, buf.Bytes(), checksum, string(changesJSON))
	if err != nil {
		return data.SchemaTemplate{}, nil, err
	}

	if err := tx.Commit(); err != nil {
		return data.SchemaTemplate{}, nil, err
	}

	// Warm the cache with the new version
	tools.SchemaCache(current.ID, newVersion, data.TablesToSchemaCache(tables))

	// Fetch updated template
	updated, err := GetTemplate(ctx, dao, name)
	return updated, changes, err
}

// computeChecksum generates a checksum for a set of tables.
func computeChecksum(tables []data.Table) string {
	// Sort tables by name for deterministic output
	sorted := make([]data.Table, len(tables))
	copy(sorted, tables)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Name > sorted[j].Name {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(sorted)

	// Simple hash
	h := 0
	for _, b := range buf.Bytes() {
		h = h*31 + int(b)
	}
	return fmt.Sprintf("%x", uint32(h))
}

// GetTemplateHistory retrieves version history for a template.
func GetTemplateHistory(ctx context.Context, dao data.PrimaryDao, name string) ([]data.TemplateVersion, error) {
	// Get template ID
	template, err := GetTemplate(ctx, dao, name)
	if err != nil {
		return nil, err
	}

	rows, err := dao.Client.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, template_id, version, schema, checksum, changes, created_at
		FROM %s WHERE template_id = ? ORDER BY version DESC
	`, data.ReservedTableTemplatesHistory), template.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []data.TemplateVersion
	for rows.Next() {
		var v data.TemplateVersion
		var tablesData []byte
		var changes sql.NullString
		err := rows.Scan(&v.ID, &v.TemplateID, &v.Version, &tablesData, &v.Checksum, &changes, &v.CreatedAt)
		if err != nil {
			return nil, err
		}

		// Deserialize tables
		buf := bytes.NewBuffer(tablesData)
		dec := gob.NewDecoder(buf)
		if err := dec.Decode(&v.Tables); err != nil {
			return nil, fmt.Errorf("failed to decode tables for version %d: %w", v.Version, err)
		}

		if changes.Valid {
			v.Changes = changes.String
		}

		versions = append(versions, v)
	}

	return versions, rows.Err()
}

// RollbackTemplate reverts a template to a specific version.
func RollbackTemplate(ctx context.Context, dao data.PrimaryDao, name string, version int) (data.SchemaTemplate, error) {
	// Get template
	template, err := GetTemplate(ctx, dao, name)
	if err != nil {
		return data.SchemaTemplate{}, err
	}

	// Get the target version
	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT schema FROM %s WHERE template_id = ? AND version = ?
	`, data.ReservedTableTemplatesHistory), template.ID, version)

	var tablesData []byte
	if err := row.Scan(&tablesData); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return data.SchemaTemplate{}, fmt.Errorf("version %d not found", version)
		}
		return data.SchemaTemplate{}, err
	}

	// Deserialize tables
	buf := bytes.NewBuffer(tablesData)
	dec := gob.NewDecoder(buf)
	var tables []data.Table
	if err := dec.Decode(&tables); err != nil {
		return data.SchemaTemplate{}, fmt.Errorf("failed to decode tables: %w", err)
	}

	// Use UpdateTemplate to create a new version with the old tables
	updated, _, err := UpdateTemplate(ctx, dao, name, tables)
	return updated, err
}
