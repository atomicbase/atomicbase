package platform

import (
	"testing"

	"github.com/joe-ervin05/atomicbase/data"
)

// Test fixtures for schema diffing
var (
	// Simple table with one column
	tableUsers = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":   {Name: "id", Type: "INTEGER", NotNull: true},
			"name": {Name: "name", Type: "TEXT", NotNull: false},
		},
	}

	// Same table with added column
	tableUsersWithEmail = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":    {Name: "id", Type: "INTEGER", NotNull: true},
			"name":  {Name: "name", Type: "TEXT", NotNull: false},
			"email": {Name: "email", Type: "TEXT", NotNull: true},
		},
	}

	// Same table with modified column (type change)
	tableUsersModified = data.Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":   {Name: "id", Type: "INTEGER", NotNull: true},
			"name": {Name: "name", Type: "TEXT", NotNull: true}, // changed: NotNull false -> true
		},
	}

	// Table with foreign key reference
	tablePosts = data.Table{
		Name: "posts",
		Pk:   []string{"id"},
		Columns: map[string]data.Col{
			"id":      {Name: "id", Type: "INTEGER", NotNull: true},
			"user_id": {Name: "user_id", Type: "INTEGER", References: "users.id"},
			"title":   {Name: "title", Type: "TEXT", NotNull: true},
		},
	}
)

// TestDiffSchemas_NoChanges verifies that identical schemas produce no changes.
func TestDiffSchemas_NoChanges(t *testing.T) {
	old := []data.Table{tableUsers}
	new := []data.Table{tableUsers}

	changes := DiffSchemas(old, new)

	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d: %+v", len(changes), changes)
	}
}

// TestDiffSchemas_AddTable verifies detection of new tables.
func TestDiffSchemas_AddTable(t *testing.T) {
	old := []data.Table{tableUsers}
	new := []data.Table{tableUsers, tablePosts}

	changes := DiffSchemas(old, new)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Type != "add_table" {
		t.Errorf("expected add_table, got %s", changes[0].Type)
	}
	if changes[0].Table != "posts" {
		t.Errorf("expected table 'posts', got %s", changes[0].Table)
	}
	if changes[0].RequiresMig {
		t.Error("add_table should not require migration")
	}
}

// TestDiffSchemas_DropTable verifies detection of removed tables.
func TestDiffSchemas_DropTable(t *testing.T) {
	old := []data.Table{tableUsers, tablePosts}
	new := []data.Table{tableUsers}

	changes := DiffSchemas(old, new)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Type != "drop_table" {
		t.Errorf("expected drop_table, got %s", changes[0].Type)
	}
	if changes[0].Table != "posts" {
		t.Errorf("expected table 'posts', got %s", changes[0].Table)
	}
	if !changes[0].RequiresMig {
		t.Error("drop_table should require migration")
	}
}

// TestDiffSchemas_AddColumn verifies detection of new columns.
func TestDiffSchemas_AddColumn(t *testing.T) {
	old := []data.Table{tableUsers}
	new := []data.Table{tableUsersWithEmail}

	changes := DiffSchemas(old, new)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Type != "add_column" {
		t.Errorf("expected add_column, got %s", changes[0].Type)
	}
	if changes[0].Table != "users" {
		t.Errorf("expected table 'users', got %s", changes[0].Table)
	}
	if changes[0].Column != "email" {
		t.Errorf("expected column 'email', got %s", changes[0].Column)
	}
	if changes[0].RequiresMig {
		t.Error("add_column should not require migration")
	}
}

// TestDiffSchemas_DropColumn verifies detection of removed columns.
func TestDiffSchemas_DropColumn(t *testing.T) {
	old := []data.Table{tableUsersWithEmail}
	new := []data.Table{tableUsers}

	changes := DiffSchemas(old, new)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Type != "drop_column" {
		t.Errorf("expected drop_column, got %s", changes[0].Type)
	}
	if changes[0].Column != "email" {
		t.Errorf("expected column 'email', got %s", changes[0].Column)
	}
	if !changes[0].RequiresMig {
		t.Error("drop_column should require migration (SQLite limitation)")
	}
}

// TestDiffSchemas_ModifyColumn verifies detection of column changes.
func TestDiffSchemas_ModifyColumn(t *testing.T) {
	old := []data.Table{tableUsers}
	new := []data.Table{tableUsersModified}

	changes := DiffSchemas(old, new)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Type != "modify_column" {
		t.Errorf("expected modify_column, got %s", changes[0].Type)
	}
	if changes[0].Column != "name" {
		t.Errorf("expected column 'name', got %s", changes[0].Column)
	}
	if !changes[0].RequiresMig {
		t.Error("modify_column should require migration (SQLite limitation)")
	}
}

// TestDiffSchemas_MultipleChanges verifies handling of multiple simultaneous changes.
func TestDiffSchemas_MultipleChanges(t *testing.T) {
	old := []data.Table{tableUsers}
	new := []data.Table{tableUsersWithEmail, tablePosts} // add column + add table

	changes := DiffSchemas(old, new)

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %+v", len(changes), changes)
	}

	// Verify we have both an add_table and add_column
	hasAddTable := false
	hasAddColumn := false
	for _, c := range changes {
		if c.Type == "add_table" && c.Table == "posts" {
			hasAddTable = true
		}
		if c.Type == "add_column" && c.Table == "users" && c.Column == "email" {
			hasAddColumn = true
		}
	}
	if !hasAddTable {
		t.Error("missing add_table change for 'posts'")
	}
	if !hasAddColumn {
		t.Error("missing add_column change for 'users.email'")
	}
}

// TestDiffSchemas_EmptyToTables verifies diffing from empty schema.
func TestDiffSchemas_EmptyToTables(t *testing.T) {
	old := []data.Table{}
	new := []data.Table{tableUsers, tablePosts}

	changes := DiffSchemas(old, new)

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %+v", len(changes), changes)
	}

	for _, c := range changes {
		if c.Type != "add_table" {
			t.Errorf("expected add_table, got %s", c.Type)
		}
	}
}

// TestDiffSchemas_TablesToEmpty verifies diffing to empty schema.
func TestDiffSchemas_TablesToEmpty(t *testing.T) {
	old := []data.Table{tableUsers, tablePosts}
	new := []data.Table{}

	changes := DiffSchemas(old, new)

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %+v", len(changes), changes)
	}

	for _, c := range changes {
		if c.Type != "drop_table" {
			t.Errorf("expected drop_table, got %s", c.Type)
		}
	}
}
