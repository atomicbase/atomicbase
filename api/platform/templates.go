package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/atomicbase/atomicbase/tools"
)

// Re-export errors from tools for backward compatibility.
var (
	ErrTemplateNotFound = tools.ErrTemplateNotFound
	ErrTemplateInUse    = tools.ErrTemplateInUse
	ErrTemplateExists   = tools.ErrTemplateExists
	ErrNoChanges        = tools.ErrNoChanges
)

// ListTemplates returns all templates.
func ListTemplates(ctx context.Context) ([]Template, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, name, current_version, created_at, updated_at
		FROM %s
		ORDER BY name
	`, TableTemplates))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []Template
	for rows.Next() {
		var t Template
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.CurrentVersion, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		templates = append(templates, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if templates == nil {
		templates = []Template{}
	}

	return templates, nil
}

// GetTemplate returns a template with its current schema.
func GetTemplate(ctx context.Context, name string) (*TemplateWithSchema, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	row := conn.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT t.id, t.name, t.current_version, t.created_at, t.updated_at, h.schema
		FROM %s t
		JOIN %s h ON h.template_id = t.id AND h.version = t.current_version
		WHERE t.name = ?
	`, TableTemplates, TableTemplatesHistory), name)

	var t TemplateWithSchema
	var createdAt, updatedAt string
	var schemaJSON []byte

	if err := row.Scan(&t.ID, &t.Name, &t.CurrentVersion, &createdAt, &updatedAt, &schemaJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}

	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if err := tools.DecodeSchema(schemaJSON, &t.Schema); err != nil {
		return nil, err
	}

	return &t, nil
}

// CreateTemplate creates a new template with the given schema at version 1.
func CreateTemplate(ctx context.Context, name string, schema Schema) (*TemplateWithSchema, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	// Serialize schema to JSON
	schemaJSON, err := tools.EncodeSchema(schema)
	if err != nil {
		return nil, err
	}

	// Calculate checksum
	hash := sha256.Sum256(schemaJSON)
	checksum := hex.EncodeToString(hash[:])

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Insert template
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (name, current_version, created_at, updated_at)
		VALUES (?, 1, ?, ?)
	`, TableTemplates), name, now, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrTemplateExists
		}
		return nil, err
	}

	templateID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Insert first version into history
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (template_id, version, schema, checksum, created_at)
		VALUES (?, 1, ?, ?, ?)
	`, TableTemplatesHistory), templateID, schemaJSON, checksum, now)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	createdAt, _ := time.Parse(time.RFC3339, now)

	return &TemplateWithSchema{
		Template: Template{
			ID:             int32(templateID),
			Name:           name,
			CurrentVersion: 1,
			CreatedAt:      createdAt,
			UpdatedAt:      createdAt,
		},
		Schema: schema,
	}, nil
}

// DeleteTemplate deletes a template if no databases are using it.
func DeleteTemplate(ctx context.Context, name string) error {
	conn, err := getDB()
	if err != nil {
		return err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get template ID
	var templateID int32
	err = tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE name = ?
	`, TableTemplates), name).Scan(&templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTemplateNotFound
		}
		return err
	}

	// Check if any databases are using this template
	var tenantCount int
	err = tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE template_id = ?
	`, TableDatabases), templateID).Scan(&tenantCount)
	if err != nil {
		return err
	}

	if tenantCount > 0 {
		return ErrTemplateInUse
	}

	// Delete history entries first (FK constraint)
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE template_id = ?
	`, TableTemplatesHistory), templateID)
	if err != nil {
		return err
	}

	// Delete migrations for this template
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE template_id = ?
	`, TableMigrations), templateID)
	if err != nil {
		return err
	}

	// Delete the template
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE id = ?
	`, TableTemplates), templateID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Remove from schema cache
	tools.InvalidateTemplate(templateID)
	return nil
}

// DiffTemplate compares a new schema against the current template version.
// Returns raw changes without ambiguity detection (CLI handles that).
func DiffTemplate(ctx context.Context, name string, newSchema Schema) (*DiffResult, error) {
	// Get current template schema
	template, err := GetTemplate(ctx, name)
	if err != nil {
		return nil, err
	}

	changes := diffSchemas(template.Schema, newSchema)
	if len(changes) == 0 {
		return nil, ErrNoChanges
	}

	return &DiffResult{Changes: changes}, nil
}

// GetTemplateHistory returns the version history for a template.
func GetTemplateHistory(ctx context.Context, name string) ([]TemplateVersion, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	// First get template ID
	var templateID int32
	err = conn.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE name = ?
	`, TableTemplates), name).Scan(&templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, template_id, version, schema, checksum, created_at
		FROM %s
		WHERE template_id = ?
		ORDER BY version DESC
	`, TableTemplatesHistory), templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []TemplateVersion
	for rows.Next() {
		var v TemplateVersion
		var createdAt string
		var schemaJSON []byte

		if err := rows.Scan(&v.ID, &v.TemplateID, &v.Version, &schemaJSON, &v.Checksum, &createdAt); err != nil {
			return nil, err
		}

		v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if err := tools.DecodeSchema(schemaJSON, &v.Schema); err != nil {
			return nil, fmt.Errorf("failed to decode schema for version %d: %w", v.Version, err)
		}

		versions = append(versions, v)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if versions == nil {
		versions = []TemplateVersion{}
	}

	return versions, nil
}

// GetTemplateVersion returns a specific version of a template's schema.
func GetTemplateVersion(ctx context.Context, templateID int32, version int) (*TemplateVersion, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	row := conn.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, template_id, version, schema, checksum, created_at
		FROM %s
		WHERE template_id = ? AND version = ?
	`, TableTemplatesHistory), templateID, version)

	var v TemplateVersion
	var createdAt string
	var schemaJSON []byte

	if err := row.Scan(&v.ID, &v.TemplateID, &v.Version, &schemaJSON, &v.Checksum, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("version %d not found for template", version)
		}
		return nil, err
	}

	v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if err := tools.DecodeSchema(schemaJSON, &v.Schema); err != nil {
		return nil, err
	}

	return &v, nil
}

// diffSchemas compares two schemas and returns the changes.
func diffSchemas(old, new Schema) []SchemaDiff {
	var changes []SchemaDiff

	oldTables := make(map[string]Table)
	for _, t := range old.Tables {
		oldTables[t.Name] = t
	}

	newTables := make(map[string]Table)
	for _, t := range new.Tables {
		newTables[t.Name] = t
	}

	// Check for dropped tables
	for name := range oldTables {
		if _, exists := newTables[name]; !exists {
			changes = append(changes, SchemaDiff{
				Type:  "drop_table",
				Table: name,
			})
		}
	}

	// Check for added tables and modified tables
	for name, newTable := range newTables {
		oldTable, exists := oldTables[name]
		if !exists {
			changes = append(changes, SchemaDiff{
				Type:  "add_table",
				Table: name,
			})
			continue
		}

		// Compare columns
		changes = append(changes, diffColumns(name, oldTable, newTable)...)

		// Compare indexes
		changes = append(changes, diffIndexes(name, oldTable, newTable)...)

		// Compare FTS columns
		changes = append(changes, diffFTS(name, oldTable, newTable)...)

		// Check for PK type change
		if pkTypeChanged(oldTable, newTable) {
			changes = append(changes, SchemaDiff{
				Type:  "change_pk_type",
				Table: name,
			})
		}
	}

	return changes
}

// diffColumns compares columns between two tables.
func diffColumns(tableName string, old, new Table) []SchemaDiff {
	var changes []SchemaDiff

	// Check for dropped columns
	for colName := range old.Columns {
		if _, exists := new.Columns[colName]; !exists {
			changes = append(changes, SchemaDiff{
				Type:   "drop_column",
				Table:  tableName,
				Column: colName,
			})
		}
	}

	// Check for added and modified columns
	for colName, newCol := range new.Columns {
		oldCol, exists := old.Columns[colName]
		if !exists {
			changes = append(changes, SchemaDiff{
				Type:   "add_column",
				Table:  tableName,
				Column: colName,
			})
			continue
		}

		// Check if column was modified
		if columnModified(oldCol, newCol) {
			changes = append(changes, SchemaDiff{
				Type:   "modify_column",
				Table:  tableName,
				Column: colName,
			})
		}
	}

	return changes
}

// diffIndexes compares indexes between two tables.
func diffIndexes(tableName string, old, new Table) []SchemaDiff {
	var changes []SchemaDiff

	oldIndexes := make(map[string]Index)
	for _, idx := range old.Indexes {
		oldIndexes[idx.Name] = idx
	}

	newIndexes := make(map[string]Index)
	for _, idx := range new.Indexes {
		newIndexes[idx.Name] = idx
	}

	// Check for dropped indexes
	for name := range oldIndexes {
		if _, exists := newIndexes[name]; !exists {
			changes = append(changes, SchemaDiff{
				Type:   "drop_index",
				Table:  tableName,
				Column: name, // Using Column field for index name
			})
		}
	}

	// Check for added indexes
	for name := range newIndexes {
		if _, exists := oldIndexes[name]; !exists {
			changes = append(changes, SchemaDiff{
				Type:   "add_index",
				Table:  tableName,
				Column: name, // Using Column field for index name
			})
		}
	}

	return changes
}

// diffFTS compares FTS columns between two tables.
func diffFTS(tableName string, old, new Table) []SchemaDiff {
	var changes []SchemaDiff

	oldFTS := make(map[string]bool)
	for _, col := range old.FTSColumns {
		oldFTS[col] = true
	}

	newFTS := make(map[string]bool)
	for _, col := range new.FTSColumns {
		newFTS[col] = true
	}

	// Check if FTS configuration changed
	if len(oldFTS) == 0 && len(newFTS) > 0 {
		changes = append(changes, SchemaDiff{
			Type:  "add_fts",
			Table: tableName,
		})
	} else if len(oldFTS) > 0 && len(newFTS) == 0 {
		changes = append(changes, SchemaDiff{
			Type:  "drop_fts",
			Table: tableName,
		})
	} else if !equalStringMaps(oldFTS, newFTS) {
		// FTS columns changed - need to recreate
		changes = append(changes, SchemaDiff{
			Type:  "drop_fts",
			Table: tableName,
		})
		changes = append(changes, SchemaDiff{
			Type:  "add_fts",
			Table: tableName,
		})
	}

	return changes
}

// pkTypeChanged checks if the primary key type changed.
func pkTypeChanged(old, new Table) bool {
	// Compare PK columns
	if len(old.Pk) != len(new.Pk) {
		return true
	}

	for i, pk := range old.Pk {
		if new.Pk[i] != pk {
			return true
		}

		// Check if PK column type changed
		oldCol, oldExists := old.Columns[pk]
		newCol, newExists := new.Columns[pk]
		if oldExists && newExists && oldCol.Type != newCol.Type {
			return true
		}
	}

	return false
}

// columnModified checks if a column definition changed.
func columnModified(old, new Col) bool {
	// Note: Type changes for non-PK columns are metadata-only in SQLite
	// but we still report them for tracking
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

	// Compare defaults (handling nil)
	if !equalDefaults(old.Default, new.Default) {
		return true
	}

	// Compare generated columns
	if !equalGenerated(old.Generated, new.Generated) {
		return true
	}

	return false
}

// equalDefaults compares two default values.
func equalDefaults(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Compare as JSON for type-agnostic comparison
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

// equalGenerated compares two Generated structs.
func equalGenerated(a, b *Generated) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Expr == b.Expr && a.Stored == b.Stored
}

// equalStringMaps compares two string sets.
func equalStringMaps(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// isUniqueConstraintError checks if an error is a unique constraint violation.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "UNIQUE constraint failed") || strings.Contains(errStr, "unique constraint")
}
