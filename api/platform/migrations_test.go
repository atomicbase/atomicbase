package platform

import (
	"testing"

	"github.com/joe-ervin05/atomicbase/data"
)

// Test fixtures for migration testing
var (
	// Basic table
	migUsersTable = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":   {Name: "id", Type: "INTEGER", NotNull: true},
			"name": {Name: "name", Type: "TEXT", NotNull: false},
		},
	}

	// Table with added column
	migUsersWithEmail = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":    {Name: "id", Type: "INTEGER", NotNull: true},
			"name":  {Name: "name", Type: "TEXT", NotNull: false},
			"email": {Name: "email", Type: "TEXT", NotNull: false},
		},
	}

	// Table with NOT NULL column without default (requires mirror table)
	migUsersWithRequiredEmail = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":    {Name: "id", Type: "INTEGER", NotNull: true},
			"name":  {Name: "name", Type: "TEXT", NotNull: false},
			"email": {Name: "email", Type: "TEXT", NotNull: true}, // NOT NULL without default
		},
	}

	// Table with NOT NULL column with default (safe to add)
	migUsersWithDefaultEmail = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":    {Name: "id", Type: "INTEGER", NotNull: true},
			"name":  {Name: "name", Type: "TEXT", NotNull: false},
			"email": {Name: "email", Type: "TEXT", NotNull: true, Default: ""},
		},
	}

	// Table with type change (requires mirror table)
	migUsersTypeChanged = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":   {Name: "id", Type: "INTEGER", NotNull: true},
			"name": {Name: "name", Type: "INTEGER", NotNull: false}, // Changed from TEXT to INTEGER
		},
	}

	// Table with added NOT NULL constraint (requires mirror table)
	migUsersNotNullAdded = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":   {Name: "id", Type: "INTEGER", NotNull: true},
			"name": {Name: "name", Type: "TEXT", NotNull: true}, // Added NOT NULL
		},
	}

	// Table with foreign key
	migPostsTable = data.Table{
		Name: "posts",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":      {Name: "id", Type: "INTEGER", NotNull: true},
			"user_id": {Name: "user_id", Type: "INTEGER", References: "users.id"},
			"title":   {Name: "title", Type: "TEXT", NotNull: true},
		},
	}
)

// TestGenerateMigrationPlan_AddColumn verifies adding a nullable column generates correct SQL.
func TestGenerateMigrationPlan_AddColumn(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersWithEmail})

	if len(plan.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(plan.Changes), plan.Changes)
	}

	change := plan.Changes[0]
	if change.Type != "add_column" {
		t.Errorf("expected add_column, got %s", change.Type)
	}
	if change.Table != "users" {
		t.Errorf("expected table 'users', got %s", change.Table)
	}
	if change.Column != "email" {
		t.Errorf("expected column 'email', got %s", change.Column)
	}
	if change.RequiresMig {
		t.Error("add nullable column should not require migration")
	}

	if len(plan.MigrationSQL) != 1 {
		t.Fatalf("expected 1 SQL statement, got %d", len(plan.MigrationSQL))
	}
	if plan.MigrationSQL[0] != `ALTER TABLE [users] ADD COLUMN [email] TEXT` {
		t.Errorf("unexpected SQL: %s", plan.MigrationSQL[0])
	}
}

// TestGenerateMigrationPlan_AddColumnWithDefault verifies adding NOT NULL column with default.
func TestGenerateMigrationPlan_AddColumnWithDefault(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersWithDefaultEmail})

	if len(plan.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(plan.Changes))
	}

	if plan.Changes[0].RequiresMig {
		t.Error("add NOT NULL column WITH default should not require migration")
	}
	if plan.RequiresMigration {
		t.Error("plan should not require migration")
	}
}

// TestGenerateMigrationPlan_AddColumnNotNullNoDefault verifies NOT NULL without default requires migration.
func TestGenerateMigrationPlan_AddColumnNotNullNoDefault(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersWithRequiredEmail})

	if len(plan.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(plan.Changes))
	}

	if !plan.Changes[0].RequiresMig {
		t.Error("add NOT NULL column without default should require migration")
	}
	if !plan.RequiresMigration {
		t.Error("plan should require migration")
	}
}

// TestGenerateMigrationPlan_TypeChange verifies type changes require mirror table.
func TestGenerateMigrationPlan_TypeChange(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersTypeChanged})

	if len(plan.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(plan.Changes))
	}

	if plan.Changes[0].Type != "modify_column" {
		t.Errorf("expected modify_column, got %s", plan.Changes[0].Type)
	}
	if !plan.Changes[0].RequiresMig {
		t.Error("type change should require migration")
	}
	if !plan.RequiresMigration {
		t.Error("plan should require migration for type change")
	}

	// Should have mirror table SQL: CREATE, INSERT, DROP, RENAME
	if len(plan.MigrationSQL) != 4 {
		t.Errorf("expected 4 SQL statements for mirror migration, got %d: %v", len(plan.MigrationSQL), plan.MigrationSQL)
	}
}

// TestGenerateMigrationPlan_AddNotNull verifies adding NOT NULL constraint requires migration.
func TestGenerateMigrationPlan_AddNotNull(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersNotNullAdded})

	if len(plan.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(plan.Changes))
	}

	if !plan.Changes[0].RequiresMig {
		t.Error("adding NOT NULL to existing column should require migration")
	}
}

// TestGenerateMigrationPlan_DropColumn verifies column drop is detected.
func TestGenerateMigrationPlan_DropColumn(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersWithEmail}, []data.Table{migUsersTable})

	if len(plan.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(plan.Changes))
	}

	if plan.Changes[0].Type != "drop_column" {
		t.Errorf("expected drop_column, got %s", plan.Changes[0].Type)
	}
	if plan.Changes[0].Column != "email" {
		t.Errorf("expected column 'email', got %s", plan.Changes[0].Column)
	}
}

// TestGenerateMigrationPlan_AddTable verifies adding a table.
func TestGenerateMigrationPlan_AddTable(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersTable, migPostsTable})

	// Should have 1 change: add_table for posts
	foundAddTable := false
	for _, change := range plan.Changes {
		if change.Type == "add_table" && change.Table == "posts" {
			foundAddTable = true
			if change.RequiresMig {
				t.Error("add_table should not require migration")
			}
		}
	}
	if !foundAddTable {
		t.Error("expected add_table change for 'posts'")
	}
}

// TestGenerateMigrationPlan_DropTable verifies dropping a table.
func TestGenerateMigrationPlan_DropTable(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable, migPostsTable}, []data.Table{migUsersTable})

	foundDropTable := false
	for _, change := range plan.Changes {
		if change.Type == "drop_table" && change.Table == "posts" {
			foundDropTable = true
			if !change.RequiresMig {
				t.Error("drop_table should require migration (destructive)")
			}
		}
	}
	if !foundDropTable {
		t.Error("expected drop_table change for 'posts'")
	}
}

// TestGenerateMigrationPlan_NoChanges verifies empty plan for identical schemas.
func TestGenerateMigrationPlan_NoChanges(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersTable})

	if len(plan.Changes) != 0 {
		t.Errorf("expected no changes, got %d", len(plan.Changes))
	}
	if len(plan.MigrationSQL) != 0 {
		t.Errorf("expected no SQL, got %d statements", len(plan.MigrationSQL))
	}
}

// TestDetectColumnModifications_TypeChange verifies type change detection.
func TestDetectColumnModifications_TypeChange(t *testing.T) {
	old := data.Col{Name: "age", Type: "TEXT"}
	new := data.Col{Name: "age", Type: "INTEGER"}

	mods := detectColumnModifications(old, new)

	if len(mods) != 1 {
		t.Fatalf("expected 1 modification, got %d", len(mods))
	}
	if mods[0] != ModTypeChange {
		t.Errorf("expected ModTypeChange, got %v", mods[0])
	}
}

// TestDetectColumnModifications_NotNullAdded verifies NOT NULL detection.
func TestDetectColumnModifications_NotNullAdded(t *testing.T) {
	old := data.Col{Name: "name", Type: "TEXT", NotNull: false}
	new := data.Col{Name: "name", Type: "TEXT", NotNull: true}

	mods := detectColumnModifications(old, new)

	if len(mods) != 1 {
		t.Fatalf("expected 1 modification, got %d", len(mods))
	}
	if mods[0] != ModNotNullAdded {
		t.Errorf("expected ModNotNullAdded, got %v", mods[0])
	}
}

// TestDetectColumnModifications_NotNullRemoved verifies NOT NULL removal detection.
func TestDetectColumnModifications_NotNullRemoved(t *testing.T) {
	old := data.Col{Name: "name", Type: "TEXT", NotNull: true}
	new := data.Col{Name: "name", Type: "TEXT", NotNull: false}

	mods := detectColumnModifications(old, new)

	if len(mods) != 1 {
		t.Fatalf("expected 1 modification, got %d", len(mods))
	}
	if mods[0] != ModNotNullRemoved {
		t.Errorf("expected ModNotNullRemoved, got %v", mods[0])
	}
}

// TestDetectColumnModifications_ReferenceChanged verifies FK reference detection.
func TestDetectColumnModifications_ReferenceChanged(t *testing.T) {
	old := data.Col{Name: "user_id", Type: "INTEGER", References: "users.id"}
	new := data.Col{Name: "user_id", Type: "INTEGER", References: "accounts.id"}

	mods := detectColumnModifications(old, new)

	foundRefChange := false
	for _, mod := range mods {
		if mod == ModReferenceChanged {
			foundRefChange = true
		}
	}
	if !foundRefChange {
		t.Error("expected ModReferenceChanged in modifications")
	}
}

// TestDetectColumnModifications_DefaultChanged verifies default value detection.
func TestDetectColumnModifications_DefaultChanged(t *testing.T) {
	old := data.Col{Name: "status", Type: "TEXT", Default: "pending"}
	new := data.Col{Name: "status", Type: "TEXT", Default: "active"}

	mods := detectColumnModifications(old, new)

	foundDefaultChange := false
	for _, mod := range mods {
		if mod == ModDefaultChanged {
			foundDefaultChange = true
		}
	}
	if !foundDefaultChange {
		t.Error("expected ModDefaultChanged in modifications")
	}
}

// TestDetectColumnModifications_NoChanges verifies no false positives.
func TestDetectColumnModifications_NoChanges(t *testing.T) {
	col := data.Col{Name: "id", Type: "INTEGER", NotNull: true}

	mods := detectColumnModifications(col, col)

	if len(mods) != 0 {
		t.Errorf("expected no modifications, got %d: %v", len(mods), mods)
	}
}

// TestRequiresMirrorTable_TypeChange verifies type change requires mirror.
func TestRequiresMirrorTable_TypeChange(t *testing.T) {
	if !requiresMirrorTable([]ColumnModification{ModTypeChange}) {
		t.Error("type change should require mirror table")
	}
}

// TestRequiresMirrorTable_NotNullAdded verifies NOT NULL requires mirror.
func TestRequiresMirrorTable_NotNullAdded(t *testing.T) {
	if !requiresMirrorTable([]ColumnModification{ModNotNullAdded}) {
		t.Error("adding NOT NULL should require mirror table")
	}
}

// TestRequiresMirrorTable_ReferenceChanged verifies FK change requires mirror.
func TestRequiresMirrorTable_ReferenceChanged(t *testing.T) {
	if !requiresMirrorTable([]ColumnModification{ModReferenceChanged}) {
		t.Error("FK reference change should require mirror table")
	}
}

// TestRequiresMirrorTable_SafeChanges verifies safe changes don't require mirror.
func TestRequiresMirrorTable_SafeChanges(t *testing.T) {
	safeMods := []ColumnModification{ModNotNullRemoved, ModDefaultChanged}
	if requiresMirrorTable(safeMods) {
		t.Error("removing NOT NULL and changing default should not require mirror table")
	}
}

// TestFormatDefaultValue verifies default value formatting.
func TestFormatDefaultValue(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{"hello", "'hello'"},
		{"it's", "'it''s'"}, // SQL escaping
		{"CURRENT_TIMESTAMP", "CURRENT_TIMESTAMP"},
		{42, "42"},
		{3.14, "3.14"},
		{true, "1"},
		{false, "0"},
		{nil, "NULL"},
	}

	for _, tt := range tests {
		result := formatDefaultValue(tt.input)
		if result != tt.expected {
			t.Errorf("formatDefaultValue(%v) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

// TestBuildDataCopySQL verifies data copy SQL generation.
func TestBuildDataCopySQL(t *testing.T) {
	old := data.Table{
		Name: "users",
		Columns: map[string]data.Col{
			"id":   {Name: "id", Type: "INTEGER"},
			"name": {Name: "name", Type: "TEXT"},
		},
	}
	new := data.Table{
		Name: "users",
		Columns: map[string]data.Col{
			"id":   {Name: "id", Type: "INTEGER"},
			"name": {Name: "name", Type: "INTEGER"}, // Type change
		},
	}

	sql := buildDataCopySQL("users", "users_new", old, new)

	// Should include CAST for type conversion
	if sql == "" {
		t.Error("expected non-empty SQL")
	}
	// The SQL should contain CAST for the type conversion
	if !containsAll(sql, "CAST", "[name]", "INTEGER") {
		t.Errorf("expected SQL with CAST for type conversion: %s", sql)
	}
}

// TestGenerateMirrorTableMigration verifies mirror table SQL generation.
func TestGenerateMirrorTableMigration(t *testing.T) {
	sql := generateMirrorTableMigration("users", migUsersTable, migUsersTypeChanged)

	if len(sql) != 4 {
		t.Fatalf("expected 4 SQL statements, got %d: %v", len(sql), sql)
	}

	// Verify order: CREATE, INSERT, DROP, RENAME
	if !containsAll(sql[0], "CREATE TABLE", "users_new") {
		t.Errorf("step 1 should be CREATE TABLE users_new: %s", sql[0])
	}
	if !containsAll(sql[1], "INSERT INTO", "users_new", "SELECT", "users") {
		t.Errorf("step 2 should be INSERT INTO users_new SELECT: %s", sql[1])
	}
	if !containsAll(sql[2], "DROP TABLE", "users") {
		t.Errorf("step 3 should be DROP TABLE users: %s", sql[2])
	}
	if !containsAll(sql[3], "ALTER TABLE", "users_new", "RENAME TO", "users") {
		t.Errorf("step 4 should be ALTER TABLE users_new RENAME TO users: %s", sql[3])
	}
}

// containsAll checks if the string contains all substrings.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// === Ambiguous Change Detection Tests ===

// Test fixtures for ambiguous changes
var (
	// Table with renamed column (same type, different name)
	migUsersRenamed = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":        {Name: "id", Type: "INTEGER", NotNull: true},
			"full_name": {Name: "full_name", Type: "TEXT", NotNull: false}, // was "name"
		},
	}

	// Table that looks like a rename of migUsersTable (for table rename testing)
	migPeopleTable = data.Table{
		Name: "people",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":   {Name: "id", Type: "INTEGER", NotNull: true},
			"name": {Name: "name", Type: "TEXT", NotNull: false},
		},
	}
)

// TestGenerateMigrationPlan_PotentialColumnRename verifies detection of potential column renames.
func TestGenerateMigrationPlan_PotentialColumnRename(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersRenamed})

	if !plan.HasAmbiguous {
		t.Error("expected HasAmbiguous to be true for potential rename")
	}

	// Should have a rename_column change that's ambiguous
	foundRename := false
	for _, change := range plan.Changes {
		if change.Type == "rename_column" && change.Ambiguous {
			foundRename = true
			if change.OldName != "name" {
				t.Errorf("expected OldName 'name', got %s", change.OldName)
			}
			if change.Column != "full_name" {
				t.Errorf("expected Column 'full_name', got %s", change.Column)
			}
			if change.Reason == "" {
				t.Error("expected Reason to be set for ambiguous change")
			}
		}
	}
	if !foundRename {
		t.Error("expected to find ambiguous rename_column change")
	}
}

// TestGenerateMigrationPlan_PotentialTableRename verifies detection of potential table renames.
func TestGenerateMigrationPlan_PotentialTableRename(t *testing.T) {
	// Remove "users" table, add "people" table with same structure
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migPeopleTable})

	if !plan.HasAmbiguous {
		t.Error("expected HasAmbiguous to be true for potential table rename")
	}

	// Should have a rename_table change that's ambiguous
	foundRename := false
	for _, change := range plan.Changes {
		if change.Type == "rename_table" && change.Ambiguous {
			foundRename = true
			if change.OldName != "users" {
				t.Errorf("expected OldName 'users', got %s", change.OldName)
			}
			if change.Table != "people" {
				t.Errorf("expected Table 'people', got %s", change.Table)
			}
		}
	}
	if !foundRename {
		t.Error("expected to find ambiguous rename_table change")
	}
}

// TestApplyResolvedRenames_ConfirmColumnRename verifies resolving column rename as actual rename.
func TestApplyResolvedRenames_ConfirmColumnRename(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersRenamed})

	resolutions := []ResolvedRename{{
		Type:     "column",
		Table:    "users",
		Column:   "full_name",
		OldName:  "name",
		IsRename: true,
	}}

	resolved := ApplyResolvedRenames(plan, resolutions)

	if resolved.HasAmbiguous {
		t.Error("expected HasAmbiguous to be false after resolution")
	}

	// Should have the rename with SQL
	foundRename := false
	for _, change := range resolved.Changes {
		if change.Type == "rename_column" && !change.Ambiguous {
			foundRename = true
			if change.SQL == "" {
				t.Error("expected SQL to be set for confirmed rename")
			}
		}
	}
	if !foundRename {
		t.Error("expected to find confirmed rename_column change")
	}
}

// TestApplyResolvedRenames_RejectColumnRename verifies resolving column rename as drop+add.
func TestApplyResolvedRenames_RejectColumnRename(t *testing.T) {
	plan := GenerateMigrationPlan([]data.Table{migUsersTable}, []data.Table{migUsersRenamed})

	resolutions := []ResolvedRename{{
		Type:     "column",
		Table:    "users",
		Column:   "full_name",
		OldName:  "name",
		IsRename: false, // Not a rename
	}}

	resolved := ApplyResolvedRenames(plan, resolutions)

	// Should NOT have the rename change (since user said it's not a rename)
	for _, change := range resolved.Changes {
		if change.Type == "rename_column" {
			t.Error("should not have rename_column when user rejected rename")
		}
	}
}

// TestGenerateMigrationPlan_MultipleDroppedAddedTables verifies all pairs are flagged.
func TestGenerateMigrationPlan_MultipleDroppedAddedTables(t *testing.T) {
	// Drop 2 tables, add 2 tables - should pair them up
	oldTable1 := data.Table{Name: "old1", Columns: map[string]data.Col{"id": {Name: "id", Type: "INTEGER"}}}
	oldTable2 := data.Table{Name: "old2", Columns: map[string]data.Col{"id": {Name: "id", Type: "INTEGER"}}}
	newTable1 := data.Table{Name: "new1", Columns: map[string]data.Col{"id": {Name: "id", Type: "INTEGER"}}}
	newTable2 := data.Table{Name: "new2", Columns: map[string]data.Col{"id": {Name: "id", Type: "INTEGER"}}}

	plan := GenerateMigrationPlan(
		[]data.Table{oldTable1, oldTable2},
		[]data.Table{newTable1, newTable2},
	)

	// Should have 2 potential renames
	renameCount := 0
	for _, change := range plan.Changes {
		if change.Type == "rename_table" && change.Ambiguous {
			renameCount++
		}
	}
	if renameCount != 2 {
		t.Errorf("expected 2 potential table renames, got %d", renameCount)
	}
}

// TestGenerateMigrationPlan_NoAmbiguityFKMismatch verifies no rename when FK status differs.
func TestGenerateMigrationPlan_NoAmbiguityFKMismatch(t *testing.T) {
	// Drop column without FK, add column with FK - not a rename
	old := data.Table{
		Name: "test",
		Columns: map[string]data.Col{
			"id":      {Name: "id", Type: "INTEGER"},
			"old_col": {Name: "old_col", Type: "INTEGER"}, // No FK
		},
	}
	new := data.Table{
		Name: "test",
		Columns: map[string]data.Col{
			"id":      {Name: "id", Type: "INTEGER"},
			"new_col": {Name: "new_col", Type: "INTEGER", References: "users.id"}, // Has FK
		},
	}

	plan := GenerateMigrationPlan([]data.Table{old}, []data.Table{new})

	// Should NOT be ambiguous since FK status differs
	for _, change := range plan.Changes {
		if change.Type == "rename_column" {
			t.Error("should not detect rename when FK status differs")
		}
	}
}

// TestGenerateMigrationPlan_AmbiguousDifferentTypes verifies rename detected even with different types.
func TestGenerateMigrationPlan_AmbiguousDifferentTypes(t *testing.T) {
	// SQLite has dynamic typing - different declared types can still be a rename
	old := data.Table{
		Name: "test",
		Columns: map[string]data.Col{
			"id":   {Name: "id", Type: "INTEGER"},
			"name": {Name: "name", Type: "TEXT"},
		},
	}
	new := data.Table{
		Name: "test",
		Columns: map[string]data.Col{
			"id":        {Name: "id", Type: "INTEGER"},
			"full_name": {Name: "full_name", Type: "VARCHAR"}, // Different type but could be rename
		},
	}

	plan := GenerateMigrationPlan([]data.Table{old}, []data.Table{new})

	// SHOULD be ambiguous - SQLite doesn't care about type differences
	if !plan.HasAmbiguous {
		t.Error("should detect potential rename regardless of type difference")
	}
}
