package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/joe-ervin05/atomicbase/data"
	"github.com/joe-ervin05/atomicbase/tools"
)

// CreateTemplate creates a new schema template.
func CreateTemplate(ctx context.Context, dao data.PrimaryDao, name string, tables []data.Table) (data.SchemaTemplate, error) {
	// Serialize tables using gob
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tables); err != nil {
		return data.SchemaTemplate{}, fmt.Errorf("failed to encode tables: %w", err)
	}

	_, err := dao.Client.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (name, tables) VALUES (?, ?)
	`, data.ReservedTableTemplates), name, buf.Bytes())
	if err != nil {
		return data.SchemaTemplate{}, err
	}

	// Fetch the created template to get timestamps
	return GetTemplate(ctx, dao, name)
}

// GetTemplate retrieves a template by name.
func GetTemplate(ctx context.Context, dao data.PrimaryDao, name string) (data.SchemaTemplate, error) {
	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, name, tables, created_at, updated_at FROM %s WHERE name = ?
	`, data.ReservedTableTemplates), name)

	var template data.SchemaTemplate
	var tablesData []byte
	err := row.Scan(&template.ID, &template.Name, &tablesData, &template.CreatedAt, &template.UpdatedAt)
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
		SELECT id, name, tables, created_at, updated_at FROM %s ORDER BY name ASC
	`, data.ReservedTableTemplates))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []data.SchemaTemplate
	for rows.Next() {
		var template data.SchemaTemplate
		var tablesData []byte
		err := rows.Scan(&template.ID, &template.Name, &tablesData, &template.CreatedAt, &template.UpdatedAt)
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

	for _, col := range table.Columns {
		query += fmt.Sprintf("[%s] %s ", col.Name, col.Type)

		if col.Name == table.Pk {
			query += "PRIMARY KEY "
		}
		if col.NotNull {
			query += "NOT NULL "
		}
		if col.Default != nil {
			switch v := col.Default.(type) {
			case string:
				query += fmt.Sprintf(`DEFAULT "%s" `, v)
			case float64:
				query += fmt.Sprintf("DEFAULT %g ", v)
			case int:
				query += fmt.Sprintf("DEFAULT %d ", v)
			}
		}
		if col.References != "" {
			// Parse references (format: "table.column")
			parts := splitReference(col.References)
			if len(parts) == 2 {
				fks = append(fks, fmt.Sprintf("FOREIGN KEY([%s]) REFERENCES [%s]([%s])", col.Name, parts[0], parts[1]))
			}
		}

		query += ", "
	}

	for _, fk := range fks {
		query += fk + ", "
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

// splitReference splits a "table.column" reference string.
func splitReference(ref string) []string {
	for i := 0; i < len(ref); i++ {
		if ref[i] == '.' {
			return []string{ref[:i], ref[i+1:]}
		}
	}
	return nil
}
