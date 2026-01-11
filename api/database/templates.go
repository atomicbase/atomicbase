package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
)

// CreateTemplate creates a new schema template.
func (dao PrimaryDao) CreateTemplate(ctx context.Context, name string, tables []Table) (SchemaTemplate, error) {
	// Serialize tables using gob
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tables); err != nil {
		return SchemaTemplate{}, fmt.Errorf("failed to encode tables: %w", err)
	}

	_, err := dao.Client.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (name, tables) VALUES (?, ?)
	`, ReservedTableTemplates), name, buf.Bytes())
	if err != nil {
		return SchemaTemplate{}, err
	}

	// Fetch the created template to get timestamps
	return dao.GetTemplate(ctx, name)
}

// GetTemplate retrieves a template by name.
func (dao PrimaryDao) GetTemplate(ctx context.Context, name string) (SchemaTemplate, error) {
	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, name, tables, created_at, updated_at FROM %s WHERE name = ?
	`, ReservedTableTemplates), name)

	var template SchemaTemplate
	var tablesData []byte
	err := row.Scan(&template.ID, &template.Name, &tablesData, &template.CreatedAt, &template.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SchemaTemplate{}, ErrTemplateNotFound
		}
		return SchemaTemplate{}, err
	}

	// Deserialize tables
	buf := bytes.NewBuffer(tablesData)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&template.Tables); err != nil {
		return SchemaTemplate{}, fmt.Errorf("failed to decode tables: %w", err)
	}

	return template, nil
}

// ListTemplates returns all schema templates.
func (dao PrimaryDao) ListTemplates(ctx context.Context) ([]SchemaTemplate, error) {
	rows, err := dao.Client.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, name, tables, created_at, updated_at FROM %s ORDER BY name ASC
	`, ReservedTableTemplates))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []SchemaTemplate
	for rows.Next() {
		var template SchemaTemplate
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
func (dao PrimaryDao) ListTemplatesJSON(ctx context.Context) ([]byte, error) {
	templates, err := dao.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	if templates == nil {
		templates = []SchemaTemplate{}
	}
	return json.Marshal(templates)
}

// UpdateTemplate updates an existing template's tables.
func (dao PrimaryDao) UpdateTemplate(ctx context.Context, name string, tables []Table) (SchemaTemplate, error) {
	// Serialize tables
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tables); err != nil {
		return SchemaTemplate{}, fmt.Errorf("failed to encode tables: %w", err)
	}

	result, err := dao.Client.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET tables = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?
	`, ReservedTableTemplates), buf.Bytes(), name)
	if err != nil {
		return SchemaTemplate{}, err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return SchemaTemplate{}, ErrTemplateNotFound
	}

	return dao.GetTemplate(ctx, name)
}

// DeleteTemplate deletes a template by name.
// Returns error if template is still associated with databases.
func (dao PrimaryDao) DeleteTemplate(ctx context.Context, name string) error {
	// First check if any databases are using this template
	var templateID int32
	err := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE name = ?
	`, ReservedTableTemplates), name).Scan(&templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTemplateNotFound
		}
		return err
	}

	var count int
	err = dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE template_id = ?
	`, ReservedTableDatabases), templateID).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return ErrTemplateInUse
	}

	_, err = dao.Client.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE name = ?
	`, ReservedTableTemplates), name)
	return err
}

// AssociateTemplate associates a database with a template.
func (dao PrimaryDao) AssociateTemplate(ctx context.Context, dbName string, templateName string) error {
	// Get template ID
	var templateID int32
	err := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE name = ?
	`, ReservedTableTemplates), templateName).Scan(&templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTemplateNotFound
		}
		return err
	}

	result, err := dao.Client.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET template_id = ? WHERE name = ?
	`, ReservedTableDatabases), templateID, dbName)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrDatabaseNotFound
	}

	return nil
}

// DisassociateTemplate removes the template association from a database.
func (dao PrimaryDao) DisassociateTemplate(ctx context.Context, dbName string) error {
	result, err := dao.Client.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET template_id = NULL WHERE name = ?
	`, ReservedTableDatabases), dbName)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrDatabaseNotFound
	}

	return nil
}

// GetDatabaseTemplate returns the template associated with a database, if any.
func (dao PrimaryDao) GetDatabaseTemplate(ctx context.Context, dbName string) (*SchemaTemplate, error) {
	var templateID sql.NullInt32
	err := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT template_id FROM %s WHERE name = ?
	`, ReservedTableDatabases), dbName).Scan(&templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDatabaseNotFound
		}
		return nil, err
	}

	if !templateID.Valid {
		return nil, nil // No template associated
	}

	// Get template by ID
	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, name, tables, created_at, updated_at FROM %s WHERE id = ?
	`, ReservedTableTemplates), templateID.Int32)

	var template SchemaTemplate
	var tablesData []byte
	err = row.Scan(&template.ID, &template.Name, &tablesData, &template.CreatedAt, &template.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// Deserialize tables
	buf := bytes.NewBuffer(tablesData)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&template.Tables); err != nil {
		return nil, fmt.Errorf("failed to decode tables: %w", err)
	}

	return &template, nil
}

// ListDatabasesByTemplate returns all databases associated with a template.
func (dao PrimaryDao) ListDatabasesByTemplate(ctx context.Context, templateName string) ([]string, error) {
	// Get template ID
	var templateID int32
	err := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE name = ?
	`, ReservedTableTemplates), templateName).Scan(&templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}

	rows, err := dao.Client.QueryContext(ctx, fmt.Sprintf(`
		SELECT name FROM %s WHERE template_id = ? AND name IS NOT NULL ORDER BY name ASC
	`, ReservedTableDatabases), templateID)
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

// SyncResult contains the result of syncing a template to databases.
type SyncResult struct {
	Database string   `json:"database"`
	Success  bool     `json:"success"`
	Error    string   `json:"error,omitempty"`
	Changes  []string `json:"changes,omitempty"`
}

// SyncTemplate syncs a template's schema to all associated databases.
// It creates missing tables, drops extra tables (if dropExtra is true),
// and adds missing columns to existing tables.
func (dao PrimaryDao) SyncTemplate(ctx context.Context, templateName string, dropExtra bool) ([]SyncResult, error) {
	template, err := dao.GetTemplate(ctx, templateName)
	if err != nil {
		return nil, err
	}

	databases, err := dao.ListDatabasesByTemplate(ctx, templateName)
	if err != nil {
		return nil, err
	}

	var results []SyncResult

	for _, dbName := range databases {
		result := SyncResult{Database: dbName, Success: true}

		// Connect to the database
		targetDB, err := dao.ConnTurso(dbName)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to connect: %v", err)
			results = append(results, result)
			continue
		}

		changes, err := syncSchemaToDatabase(ctx, &targetDB, template.Tables, dropExtra)
		targetDB.Client.Close()

		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Changes = changes
		}

		results = append(results, result)
	}

	return results, nil
}

// SyncDatabaseToTemplate syncs a single database to its associated template.
func (dao PrimaryDao) SyncDatabaseToTemplate(ctx context.Context, dbName string, dropExtra bool) ([]string, error) {
	template, err := dao.GetDatabaseTemplate(ctx, dbName)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errors.New("database has no associated template")
	}

	// Connect to the database
	targetDB, err := dao.ConnTurso(dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer targetDB.Client.Close()

	return syncSchemaToDatabase(ctx, &targetDB, template.Tables, dropExtra)
}

// syncSchemaToDatabase applies template tables to a database.
func syncSchemaToDatabase(ctx context.Context, db *Database, templateTables []Table, dropExtra bool) ([]string, error) {
	var changes []string

	// Build map of template tables for quick lookup
	templateMap := make(map[string]Table)
	for _, t := range templateTables {
		templateMap[t.Name] = t
	}

	// Get current tables from database (refresh schema)
	currentTables, err := schemaCols(db.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to get current schema: %w", err)
	}

	currentMap := make(map[string]Table)
	for _, t := range currentTables {
		// Skip internal tables
		if len(t.Name) > 0 && t.Name[0] == '_' {
			continue
		}
		currentMap[t.Name] = t
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
func buildCreateTableQuery(name string, table Table) string {
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
func syncTableColumns(ctx context.Context, db *Database, tableName string, current Table, template Table) ([]string, error) {
	var changes []string

	// Build map of current columns
	currentCols := make(map[string]Col)
	for _, c := range current.Columns {
		currentCols[c.Name] = c
	}

	// Add missing columns
	for _, templateCol := range template.Columns {
		if _, exists := currentCols[templateCol.Name]; !exists {
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
