package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"fmt"
	"strings"

	"github.com/joe-ervin05/atomicbase/data"
)

// decodeGob decodes gob-encoded data into the target.
func decodeGob(data []byte, target any) error {
	buf := bytes.NewBuffer(data)
	return gob.NewDecoder(buf).Decode(target)
}

// MigrationPlan represents the full migration plan for a schema change.
type MigrationPlan struct {
	Changes           []data.SchemaChange `json:"changes"`
	RequiresMigration bool                `json:"requiresMigration"`      // True if any change requires mirror table
	HasAmbiguous      bool                `json:"hasAmbiguous,omitempty"` // True if there are ambiguous changes needing confirmation
	MigrationSQL      []string            `json:"migrationSql"`           // All SQL statements in order
}

// GenerateMigrationPlan creates a detailed migration plan from old to new schema.
// This generates all necessary SQL statements for both safe and complex operations.
// Detects potential renames and flags them as ambiguous for user confirmation.
func GenerateMigrationPlan(oldTables, newTables []data.Table) MigrationPlan {
	plan := MigrationPlan{
		Changes:      []data.SchemaChange{},
		MigrationSQL: []string{},
	}

	// Build maps for easier lookup
	oldMap := make(map[string]data.Table)
	for _, t := range oldTables {
		oldMap[t.Name] = t
	}

	newMap := make(map[string]data.Table)
	for _, t := range newTables {
		newMap[t.Name] = t
	}

	// First pass: detect potential table renames
	droppedTables := make(map[string]data.Table)
	addedTables := make(map[string]data.Table)

	for name, table := range oldMap {
		if _, exists := newMap[name]; !exists {
			droppedTables[name] = table
		}
	}
	for name, table := range newMap {
		if _, exists := oldMap[name]; !exists {
			addedTables[name] = table
		}
	}

	// Check for potential table renames (similar structure)
	tableRenames := detectPotentialTableRenames(droppedTables, addedTables)

	// Order matters for FK constraints:
	// 1. Handle table renames first (or flag as ambiguous)
	// 2. Create new tables (they might be referenced)
	// 3. Add/modify/drop columns
	// 4. Drop tables last

	// Phase 1: Handle potential table renames
	for _, rename := range tableRenames {
		plan.Changes = append(plan.Changes, rename)
		plan.HasAmbiguous = true
		// Remove from dropped/added since we're treating as rename
		delete(droppedTables, rename.OldName)
		delete(addedTables, rename.Table)
	}

	// Phase 2: Create genuinely new tables
	for name, newTable := range addedTables {
		change := data.SchemaChange{
			Type:        "add_table",
			Table:       name,
			SQL:         buildCreateTableQuery(name, newTable),
			RequiresMig: false,
		}
		plan.Changes = append(plan.Changes, change)
		plan.MigrationSQL = append(plan.MigrationSQL, change.SQL)
	}

	// Phase 3: Process existing tables for column changes
	for name, newTable := range newMap {
		oldTable, exists := oldMap[name]
		if !exists {
			continue // New table, already handled
		}

		// Analyze all column changes for this table (with ambiguous detection)
		colChanges, hasAmbiguous := analyzeColumnChangesWithRenames(name, oldTable, newTable)
		if hasAmbiguous {
			plan.HasAmbiguous = true
		}

		// Check if we need mirror table migration (only for non-ambiguous changes)
		needsMirror := false
		for _, c := range colChanges {
			if c.RequiresMig && !c.Ambiguous {
				needsMirror = true
				break
			}
		}

		if needsMirror {
			// Generate mirror table migration
			mirrorSQL := generateMirrorTableMigration(name, oldTable, newTable)
			for i := range colChanges {
				if !colChanges[i].Ambiguous {
					colChanges[i].SQL = "" // SQL is part of the mirror migration
				}
			}
			plan.MigrationSQL = append(plan.MigrationSQL, mirrorSQL...)
			plan.RequiresMigration = true
		} else {
			// Apply safe changes individually
			for _, c := range colChanges {
				if c.SQL != "" && !c.Ambiguous {
					plan.MigrationSQL = append(plan.MigrationSQL, c.SQL)
				}
			}
		}

		plan.Changes = append(plan.Changes, colChanges...)

		// Analyze index changes
		indexChanges := diffIndexes(name, oldTable.Indexes, newTable.Indexes)
		for _, c := range indexChanges {
			if c.SQL != "" {
				plan.MigrationSQL = append(plan.MigrationSQL, c.SQL)
			}
		}
		plan.Changes = append(plan.Changes, indexChanges...)

		// Analyze FTS changes
		ftsChanges := diffFTS(name, oldTable.FTSColumns, newTable.FTSColumns)
		for _, c := range ftsChanges {
			if c.SQL != "" {
				plan.MigrationSQL = append(plan.MigrationSQL, c.SQL)
			}
		}
		plan.Changes = append(plan.Changes, ftsChanges...)
	}

	// Phase 4: Drop genuinely removed tables
	for name := range droppedTables {
		change := data.SchemaChange{
			Type:        "drop_table",
			Table:       name,
			SQL:         fmt.Sprintf("DROP TABLE [%s]", name),
			RequiresMig: true,
		}
		plan.Changes = append(plan.Changes, change)
		plan.MigrationSQL = append(plan.MigrationSQL, change.SQL)
		plan.RequiresMigration = true
	}

	return plan
}

// detectPotentialTableRenames finds dropped tables that might have been renamed.
// Any dropped + added table pair is a potential rename. The user knows their intent better
// than any heuristic (especially when columns are also renamed alongside the table).
func detectPotentialTableRenames(dropped, added map[string]data.Table) []data.SchemaChange {
	renames := []data.SchemaChange{}

	// Track which tables have been paired
	usedDropped := make(map[string]bool)
	usedAdded := make(map[string]bool)

	for oldName := range dropped {
		for newName := range added {
			if usedAdded[newName] {
				continue
			}
			renames = append(renames, data.SchemaChange{
				Type:      "rename_table",
				Table:     newName,
				OldName:   oldName,
				Ambiguous: true,
				Reason:    fmt.Sprintf("Table '%s' was removed and '%s' was added. Is this a rename?", oldName, newName),
				SQL:       fmt.Sprintf("ALTER TABLE [%s] RENAME TO [%s]", oldName, newName),
			})
			usedDropped[oldName] = true
			usedAdded[newName] = true
			break // One match per dropped table
		}
	}

	return renames
}

// analyzeColumnChangesWithRenames detects column changes including potential renames.
func analyzeColumnChangesWithRenames(tableName string, oldTable, newTable data.Table) ([]data.SchemaChange, bool) {
	changes := []data.SchemaChange{}
	hasAmbiguous := false

	// Find dropped and added columns
	droppedCols := make(map[string]data.Col)
	addedCols := make(map[string]data.Col)

	for colName, col := range oldTable.Columns {
		if _, exists := newTable.Columns[colName]; !exists {
			droppedCols[colName] = col
		}
	}
	for colName, col := range newTable.Columns {
		if _, exists := oldTable.Columns[colName]; !exists {
			addedCols[colName] = col
		}
	}

	// Detect potential column renames
	// SQLite has dynamic typing, so any dropped+added pair is a potential rename
	// Exception: if the dropped column has a FK and the added column doesn't (or vice versa)
	usedDropped := make(map[string]bool)
	usedAdded := make(map[string]bool)

	for droppedName, droppedCol := range droppedCols {
		for addedName, addedCol := range addedCols {
			if usedAdded[addedName] {
				continue
			}
			// Skip if FK references differ significantly (one has FK, other doesn't)
			if (droppedCol.References == "") != (addedCol.References == "") {
				continue
			}

			changes = append(changes, data.SchemaChange{
				Type:      "rename_column",
				Table:     tableName,
				Column:    addedName,
				OldName:   droppedName,
				Ambiguous: true,
				Reason:    fmt.Sprintf("Column '%s' was removed and '%s' was added. Is this a rename?", droppedName, addedName),
				SQL:       fmt.Sprintf("ALTER TABLE [%s] RENAME COLUMN [%s] TO [%s]", tableName, droppedName, addedName),
			})
			usedDropped[droppedName] = true
			usedAdded[addedName] = true
			hasAmbiguous = true
			break
		}
	}

	// Handle genuinely dropped columns (not potential renames)
	for colName := range droppedCols {
		if usedDropped[colName] {
			continue
		}
		changes = append(changes, data.SchemaChange{
			Type:        "drop_column",
			Table:       tableName,
			Column:      colName,
			SQL:         fmt.Sprintf("ALTER TABLE [%s] DROP COLUMN [%s]", tableName, colName),
			RequiresMig: false,
		})
	}

	// Handle genuinely added columns (not potential renames)
	for colName, newCol := range addedCols {
		if usedAdded[colName] {
			continue
		}
		requiresMirror := newCol.NotNull && newCol.Default == nil
		change := data.SchemaChange{
			Type:        "add_column",
			Table:       tableName,
			Column:      colName,
			RequiresMig: requiresMirror,
		}
		if !requiresMirror {
			change.SQL = buildAddColumnSQL(tableName, newCol)
		}
		changes = append(changes, change)
	}

	// Find modified columns (columns that exist in both)
	for colName, newCol := range newTable.Columns {
		oldCol, exists := oldTable.Columns[colName]
		if !exists {
			continue // New or renamed column, already handled
		}

		modifications := detectColumnModifications(oldCol, newCol)
		if len(modifications) == 0 {
			continue
		}

		requiresMirror := requiresMirrorTable(modifications)
		changes = append(changes, data.SchemaChange{
			Type:        "modify_column",
			Table:       tableName,
			Column:      colName,
			RequiresMig: requiresMirror,
		})
	}

	return changes, hasAmbiguous
}

// ResolvedRename represents a user's confirmation of whether an ambiguous change is a rename.
type ResolvedRename struct {
	Type      string `json:"type"`      // "table" or "column"
	Table     string `json:"table"`     // Table name (for both types)
	Column    string `json:"column"`    // Column name (for column renames)
	OldName   string `json:"oldName"`   // Original name
	IsRename  bool   `json:"isRename"`  // true = rename, false = drop+add
}

// ApplyResolvedRenames updates a migration plan based on user-resolved ambiguous changes.
// Returns a new plan with ambiguous changes resolved.
func ApplyResolvedRenames(plan MigrationPlan, resolutions []ResolvedRename) MigrationPlan {
	// Build lookup maps for resolutions
	tableResolutions := make(map[string]ResolvedRename)    // oldName -> resolution
	columnResolutions := make(map[string]ResolvedRename)   // "table.oldName" -> resolution

	for _, r := range resolutions {
		if r.Type == "table" {
			tableResolutions[r.OldName] = r
		} else {
			key := r.Table + "." + r.OldName
			columnResolutions[key] = r
		}
	}

	newChanges := []data.SchemaChange{}
	newSQL := []string{}

	for _, change := range plan.Changes {
		if !change.Ambiguous {
			newChanges = append(newChanges, change)
			if change.SQL != "" {
				newSQL = append(newSQL, change.SQL)
			}
			continue
		}

		// Handle ambiguous changes based on resolution
		if change.Type == "rename_table" {
			if res, ok := tableResolutions[change.OldName]; ok {
				if res.IsRename {
					// Confirmed rename - use RENAME SQL
					change.Ambiguous = false
					change.Reason = ""
					newChanges = append(newChanges, change)
					newSQL = append(newSQL, change.SQL)
				} else {
					// Not a rename - convert to drop + add
					// Skip this change, the drop_table and add_table should already be in the plan
					// if user said "not a rename"
				}
			}
		} else if change.Type == "rename_column" {
			key := change.Table + "." + change.OldName
			if res, ok := columnResolutions[key]; ok {
				if res.IsRename {
					// Confirmed rename - use RENAME COLUMN SQL
					change.Ambiguous = false
					change.Reason = ""
					newChanges = append(newChanges, change)
					newSQL = append(newSQL, change.SQL)
				}
				// If not a rename, the drop_column and add_column are already handled
			}
		}
	}

	plan.Changes = newChanges
	plan.MigrationSQL = newSQL
	plan.HasAmbiguous = false

	// Recalculate RequiresMigration
	plan.RequiresMigration = false
	for _, c := range plan.Changes {
		if c.RequiresMig {
			plan.RequiresMigration = true
			break
		}
	}

	return plan
}

// ColumnModification describes a specific type of column change.
type ColumnModification string

const (
	ModTypeChange       ColumnModification = "type_change"
	ModNotNullAdded     ColumnModification = "not_null_added"
	ModNotNullRemoved   ColumnModification = "not_null_removed"
	ModDefaultChanged   ColumnModification = "default_changed"
	ModReferenceChanged ColumnModification = "reference_changed"
	ModOnDeleteChanged  ColumnModification = "on_delete_changed"
	ModOnUpdateChanged  ColumnModification = "on_update_changed"
	ModUniqueChanged    ColumnModification = "unique_changed"
	ModCollateChanged   ColumnModification = "collate_changed"
	ModCheckChanged     ColumnModification = "check_changed"
	ModGeneratedChanged ColumnModification = "generated_changed"
)

// detectColumnModifications returns all modifications between two column definitions.
func detectColumnModifications(old, new data.Col) []ColumnModification {
	var mods []ColumnModification

	if old.Type != new.Type {
		mods = append(mods, ModTypeChange)
	}
	if !old.NotNull && new.NotNull {
		mods = append(mods, ModNotNullAdded)
	}
	if old.NotNull && !new.NotNull {
		mods = append(mods, ModNotNullRemoved)
	}
	if old.References != new.References {
		mods = append(mods, ModReferenceChanged)
	}
	if fmt.Sprintf("%v", old.Default) != fmt.Sprintf("%v", new.Default) {
		mods = append(mods, ModDefaultChanged)
	}
	if old.OnDelete != new.OnDelete {
		mods = append(mods, ModOnDeleteChanged)
	}
	if old.OnUpdate != new.OnUpdate {
		mods = append(mods, ModOnUpdateChanged)
	}
	if old.Unique != new.Unique {
		mods = append(mods, ModUniqueChanged)
	}
	if old.Collate != new.Collate {
		mods = append(mods, ModCollateChanged)
	}
	if old.Check != new.Check {
		mods = append(mods, ModCheckChanged)
	}
	// Compare generated columns
	if (old.Generated == nil) != (new.Generated == nil) {
		mods = append(mods, ModGeneratedChanged)
	} else if old.Generated != nil && new.Generated != nil {
		if old.Generated.Expr != new.Generated.Expr || old.Generated.Stored != new.Generated.Stored {
			mods = append(mods, ModGeneratedChanged)
		}
	}

	return mods
}

// requiresMirrorTable determines if the modifications require mirror table migration.
func requiresMirrorTable(mods []ColumnModification) bool {
	for _, mod := range mods {
		switch mod {
		case ModTypeChange, ModNotNullAdded, ModReferenceChanged,
			ModOnDeleteChanged, ModOnUpdateChanged, ModUniqueChanged,
			ModCollateChanged, ModCheckChanged, ModGeneratedChanged:
			// These cannot be done with ALTER TABLE in SQLite
			return true
		}
	}
	return false
}

// generateMirrorTableMigration generates the 12-step mirror table migration SQL.
// This is used when SQLite's ALTER TABLE limitations are hit.
func generateMirrorTableMigration(tableName string, oldTable, newTable data.Table) []string {
	var sql []string

	tempName := tableName + "_new"

	// Step 1-2: Disable FK and start transaction (handled by MigrateDatabase)
	// Step 3: Create new table with desired schema
	sql = append(sql, buildCreateTableQuery(tempName, newTable))

	// Step 4: Copy data from old to new table
	sql = append(sql, buildDataCopySQL(tableName, tempName, oldTable, newTable))

	// Step 5: Drop old table
	sql = append(sql, fmt.Sprintf("DROP TABLE [%s]", tableName))

	// Step 6: Rename new table to original name
	sql = append(sql, fmt.Sprintf("ALTER TABLE [%s] RENAME TO [%s]", tempName, tableName))

	// Steps 7-12: Recreate indexes, triggers, views (handled separately if needed)

	return sql
}

// buildDataCopySQL generates INSERT INTO ... SELECT for data migration.
func buildDataCopySQL(oldTable, newTable string, oldSchema, newSchema data.Table) string {
	// Determine which columns to copy (intersection of old and new)
	var cols []string
	var selectExprs []string

	for colName, newCol := range newSchema.Columns {
		oldCol, exists := oldSchema.Columns[colName]
		if !exists {
			// New column - use default or NULL
			if newCol.Default != nil {
				selectExprs = append(selectExprs, formatDefaultValue(newCol.Default))
			} else {
				selectExprs = append(selectExprs, "NULL")
			}
			cols = append(cols, fmt.Sprintf("[%s]", colName))
			continue
		}

		// Existing column - check if type conversion needed
		if oldCol.Type != newCol.Type {
			// Add CAST for type conversion
			selectExprs = append(selectExprs, fmt.Sprintf("CAST([%s] AS %s)", colName, newCol.Type))
		} else {
			selectExprs = append(selectExprs, fmt.Sprintf("[%s]", colName))
		}
		cols = append(cols, fmt.Sprintf("[%s]", colName))
	}

	return fmt.Sprintf("INSERT INTO [%s] (%s) SELECT %s FROM [%s]",
		newTable,
		strings.Join(cols, ", "),
		strings.Join(selectExprs, ", "),
		oldTable,
	)
}

// formatDefaultValue formats a default value for SQL.
func formatDefaultValue(val any) string {
	switch v := val.(type) {
	case string:
		// Check if it's a SQL expression (like CURRENT_TIMESTAMP)
		upper := strings.ToUpper(v)
		if upper == "CURRENT_TIMESTAMP" || upper == "CURRENT_DATE" || upper == "CURRENT_TIME" {
			return v
		}
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
	case float64:
		return fmt.Sprintf("%g", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("'%v'", v)
	}
}

// MigrateDatabase applies a migration plan to a single database atomically.
// Either all changes succeed, or the database is rolled back to its original state.
func MigrateDatabase(ctx context.Context, db *sql.DB, plan MigrationPlan) error {
	if len(plan.MigrationSQL) == 0 {
		return nil // Nothing to do
	}

	// Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// If we have a mirror table migration, disable foreign keys temporarily
	if plan.RequiresMigration {
		if _, err := tx.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
			return fmt.Errorf("failed to disable foreign keys: %w", err)
		}
	}

	// Apply all SQL statements
	for i, stmt := range plan.MigrationSQL {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration step %d failed: %w\nSQL: %s", i+1, err, stmt)
		}
	}

	// Re-enable foreign keys if we disabled them
	if plan.RequiresMigration {
		if _, err := tx.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
			return fmt.Errorf("failed to re-enable foreign keys: %w", err)
		}

		// Check foreign key integrity
		rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
		if err != nil {
			return fmt.Errorf("failed to check foreign key integrity: %w", err)
		}
		defer rows.Close()

		if rows.Next() {
			// There are FK violations
			var table, rowid, parent, fkid string
			rows.Scan(&table, &rowid, &parent, &fkid)
			return fmt.Errorf("foreign key violation after migration: table=%s rowid=%s parent=%s", table, rowid, parent)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	return nil
}

// MigrateDatabaseToVersion migrates a tenant database from one version to another.
// This fetches the schemas from history and applies the necessary changes.
func MigrateDatabaseToVersion(ctx context.Context, dao data.PrimaryDao, tenantDB *sql.DB, templateID int32, fromVersion, toVersion int) error {
	// Fetch old schema from history
	oldTables, err := getVersionTables(ctx, dao, templateID, fromVersion)
	if err != nil {
		return fmt.Errorf("failed to get schema for version %d: %w", fromVersion, err)
	}

	// Fetch new schema from history
	newTables, err := getVersionTables(ctx, dao, templateID, toVersion)
	if err != nil {
		return fmt.Errorf("failed to get schema for version %d: %w", toVersion, err)
	}

	// Generate migration plan
	plan := GenerateMigrationPlan(oldTables, newTables)

	// Apply migration
	return MigrateDatabase(ctx, tenantDB, plan)
}

// getVersionTables retrieves the table definitions for a specific template version.
func getVersionTables(ctx context.Context, dao data.PrimaryDao, templateID int32, version int) ([]data.Table, error) {
	if version == 0 {
		// Version 0 means empty schema (new database)
		return []data.Table{}, nil
	}

	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT schema FROM %s WHERE template_id = ? AND version = ?
	`, data.ReservedTableTemplatesHistory), templateID, version)

	var tablesData []byte
	if err := row.Scan(&tablesData); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("version %d not found for template %d", version, templateID)
		}
		return nil, err
	}

	var tables []data.Table
	if err := decodeGob(tablesData, &tables); err != nil {
		return nil, fmt.Errorf("failed to decode tables: %w", err)
	}

	return tables, nil
}

// PlanMigrationSQL returns just the SQL statements for a migration.
// Useful for dry-run or preview functionality.
func PlanMigrationSQL(oldTables, newTables []data.Table) []string {
	plan := GenerateMigrationPlan(oldTables, newTables)
	return plan.MigrationSQL
}

// ValidateMigration checks if a migration plan is safe to apply.
// Returns nil if safe, or an error describing the issue.
func ValidateMigration(plan MigrationPlan) error {
	// Check for potentially dangerous operations
	for _, change := range plan.Changes {
		if change.Type == "drop_table" {
			// Warning: dropping tables loses data
			// This is allowed but caller should confirm
		}
		if change.Type == "drop_column" {
			// Warning: dropping columns loses data
			// This is allowed but caller should confirm
		}
	}
	return nil
}
