package database

import (
	"context"
	"database/sql"
	"testing"
)

// =============================================================================
// buildCreateTableQuery Tests - Edge cases for SQL generation
// =============================================================================

func TestBuildCreateTableQuery(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
		table     Table
		contains  []string // substrings that must be present
	}{
		{
			name:      "simple table with PK",
			tableName: "simple",
			table: Table{
				Pk:      "id",
				Columns: map[string]Col{"id": {Name: "id", Type: "INTEGER"}},
			},
			contains: []string{
				"CREATE TABLE [simple]",
				"[id] INTEGER PRIMARY KEY",
			},
		},
		{
			name:      "column with NOT NULL",
			tableName: "test",
			table: Table{
				Columns: map[string]Col{"email": {Name: "email", Type: "TEXT", NotNull: true}},
			},
			contains: []string{
				"[email] TEXT NOT NULL",
			},
		},
		{
			name:      "column with string default",
			tableName: "test",
			table: Table{
				Columns: map[string]Col{"status": {Name: "status", Type: "TEXT", Default: "active"}},
			},
			contains: []string{
				`DEFAULT "active"`,
			},
		},
		{
			name:      "column with integer default",
			tableName: "test",
			table: Table{
				Columns: map[string]Col{"count": {Name: "count", Type: "INTEGER", Default: 0}},
			},
			contains: []string{
				"DEFAULT 0",
			},
		},
		{
			name:      "column with float default",
			tableName: "test",
			table: Table{
				Columns: map[string]Col{"price": {Name: "price", Type: "REAL", Default: 9.99}},
			},
			contains: []string{
				"DEFAULT 9.99",
			},
		},
		{
			name:      "column with foreign key",
			tableName: "test",
			table: Table{
				Columns: map[string]Col{"user_id": {Name: "user_id", Type: "INTEGER", References: "users.id"}},
			},
			contains: []string{
				"FOREIGN KEY([user_id]) REFERENCES [users]([id])",
			},
		},
		{
			name:      "multiple columns",
			tableName: "posts",
			table: Table{
				Pk: "id",
				Columns: map[string]Col{
					"id":         {Name: "id", Type: "INTEGER"},
					"name":       {Name: "name", Type: "TEXT", NotNull: true},
					"created_at": {Name: "created_at", Type: "TEXT", Default: "CURRENT_TIMESTAMP"},
				},
			},
			contains: []string{
				"[id] INTEGER PRIMARY KEY",
				"[name] TEXT NOT NULL",
				"[created_at] TEXT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := buildCreateTableQuery(tt.tableName, tt.table)

			for _, substr := range tt.contains {
				if !containsString(query, substr) {
					t.Errorf("query missing %q\ngot: %s", substr, query)
				}
			}
		})
	}
}

// containsString checks if s contains substr (simple helper to avoid importing strings)
func containsString(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// syncSchemaToDatabase Tests - Complex context: multi-step schema sync
// =============================================================================

// setupSyncTestDB creates an in-memory database for sync testing.
func setupSyncTestDB(t *testing.T) *Database {
	t.Helper()
	client, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return &Database{Client: client, Schema: SchemaCache{}, id: 0}
}

// getTableNames returns all user table names in the database (excludes internal _ prefixed tables).
func getTableNames(t *testing.T, db *Database) []string {
	t.Helper()
	rows, err := db.Client.Query(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		// Skip internal tables (atomicbase_ prefix)
		if len(name) >= len(InternalTablePrefix) && name[:len(InternalTablePrefix)] == InternalTablePrefix {
			continue
		}
		names = append(names, name)
	}
	return names
}

// getColumnNames returns column names for a table.
func getColumnNames(t *testing.T, db *Database, tableName string) []string {
	t.Helper()
	rows, err := db.Client.Query("SELECT name FROM pragma_table_info(?)", tableName)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		names = append(names, name)
	}
	return names
}

func TestSyncSchemaToDatabase(t *testing.T) {
	ctx := context.Background()

	t.Run("creates missing table", func(t *testing.T) {
		db := setupSyncTestDB(t)

		template := []Table{
			{
				Name: "users",
				Pk:   "id",
				Columns: map[string]Col{
					"id":   {Name: "id", Type: ColTypeInteger},
					"name": {Name: "name", Type: ColTypeText},
				},
			},
		}

		changes, err := syncSchemaToDatabase(ctx, db, template, false)
		if err != nil {
			t.Fatal(err)
		}

		// Verify table was created
		tables := getTableNames(t, db)
		if len(tables) != 1 || tables[0] != "users" {
			t.Errorf("expected [users], got %v", tables)
		}

		// Verify change was reported
		if len(changes) != 1 || !containsString(changes[0], "created table: users") {
			t.Errorf("expected create change, got %v", changes)
		}
	})

	t.Run("drops extra table when dropExtra=true", func(t *testing.T) {
		db := setupSyncTestDB(t)

		// Create an existing table that's not in template
		db.Client.Exec("CREATE TABLE extra (id INTEGER)")

		template := []Table{} // empty template

		changes, err := syncSchemaToDatabase(ctx, db, template, true)
		if err != nil {
			t.Fatal(err)
		}

		// Verify table was dropped
		tables := getTableNames(t, db)
		if len(tables) != 0 {
			t.Errorf("expected no tables, got %v", tables)
		}

		if len(changes) != 1 || !containsString(changes[0], "dropped table: extra") {
			t.Errorf("expected drop change, got %v", changes)
		}
	})

	t.Run("keeps extra table when dropExtra=false", func(t *testing.T) {
		db := setupSyncTestDB(t)

		// Create an existing table that's not in template
		db.Client.Exec("CREATE TABLE extra (id INTEGER)")

		template := []Table{} // empty template

		changes, err := syncSchemaToDatabase(ctx, db, template, false)
		if err != nil {
			t.Fatal(err)
		}

		// Verify table was NOT dropped
		tables := getTableNames(t, db)
		if len(tables) != 1 || tables[0] != "extra" {
			t.Errorf("expected [extra] to remain, got %v", tables)
		}

		if len(changes) != 0 {
			t.Errorf("expected no changes, got %v", changes)
		}
	})

	t.Run("adds missing column to existing table", func(t *testing.T) {
		db := setupSyncTestDB(t)

		// Create table with only id column
		db.Client.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)")

		// Template has additional column
		template := []Table{
			{
				Name: "users",
				Pk:   "id",
				Columns: map[string]Col{
					"id":    {Name: "id", Type: ColTypeInteger},
					"email": {Name: "email", Type: ColTypeText}, // new column
				},
			},
		}

		changes, err := syncSchemaToDatabase(ctx, db, template, false)
		if err != nil {
			t.Fatal(err)
		}

		// Verify column was added
		cols := getColumnNames(t, db, "users")
		hasEmail := false
		for _, c := range cols {
			if c == "email" {
				hasEmail = true
			}
		}
		if !hasEmail {
			t.Errorf("expected email column, got %v", cols)
		}

		if len(changes) != 1 || !containsString(changes[0], "added column: users.email") {
			t.Errorf("expected add column change, got %v", changes)
		}
	})

	t.Run("no changes when schema matches", func(t *testing.T) {
		db := setupSyncTestDB(t)

		// Create table matching template
		db.Client.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")

		template := []Table{
			{
				Name: "users",
				Pk:   "id",
				Columns: map[string]Col{
					"id":   {Name: "id", Type: ColTypeInteger},
					"name": {Name: "name", Type: ColTypeText},
				},
			},
		}

		changes, err := syncSchemaToDatabase(ctx, db, template, false)
		if err != nil {
			t.Fatal(err)
		}

		if len(changes) != 0 {
			t.Errorf("expected no changes, got %v", changes)
		}
	})

	t.Run("skips internal tables with atomicbase_ prefix", func(t *testing.T) {
		db := setupSyncTestDB(t)

		// Create internal table (should be ignored)
		db.Client.Exec("CREATE TABLE atomicbase_internal (id INTEGER)")

		template := []Table{} // empty template

		changes, err := syncSchemaToDatabase(ctx, db, template, true)
		if err != nil {
			t.Fatal(err)
		}

		// Internal table should NOT be dropped even with dropExtra=true
		var count int
		db.Client.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name = 'atomicbase_internal'").Scan(&count)
		if count != 1 {
			t.Error("internal table should not be dropped")
		}

		if len(changes) != 0 {
			t.Errorf("expected no changes for internal tables, got %v", changes)
		}
	})

	t.Run("creates table with foreign key", func(t *testing.T) {
		db := setupSyncTestDB(t)

		template := []Table{
			{
				Name: "users",
				Pk:   "id",
				Columns: map[string]Col{
					"id": {Name: "id", Type: ColTypeInteger},
				},
			},
			{
				Name: "posts",
				Pk:   "id",
				Columns: map[string]Col{
					"id":      {Name: "id", Type: ColTypeInteger},
					"user_id": {Name: "user_id", Type: ColTypeInteger, References: "users.id"},
				},
			},
		}

		changes, err := syncSchemaToDatabase(ctx, db, template, false)
		if err != nil {
			t.Fatal(err)
		}

		// Verify both tables created
		tables := getTableNames(t, db)
		if len(tables) != 2 {
			t.Errorf("expected 2 tables, got %v", tables)
		}

		if len(changes) != 2 {
			t.Errorf("expected 2 create changes, got %v", changes)
		}
	})

	t.Run("multiple operations in single sync", func(t *testing.T) {
		db := setupSyncTestDB(t)

		// Setup: one table to keep, one to drop, one missing
		db.Client.Exec("CREATE TABLE keep_me (id INTEGER)")
		db.Client.Exec("CREATE TABLE drop_me (id INTEGER)")

		template := []Table{
			{
				Name: "keep_me",
				Columns: map[string]Col{
					"id":      {Name: "id", Type: ColTypeInteger},
					"new_col": {Name: "new_col", Type: ColTypeText},
				},
			},
			{
				Name: "create_me",
				Columns: map[string]Col{
					"id": {Name: "id", Type: ColTypeInteger},
				},
			},
		}

		changes, err := syncSchemaToDatabase(ctx, db, template, true)
		if err != nil {
			t.Fatal(err)
		}

		// Should have: drop drop_me, create create_me, add column to keep_me
		if len(changes) != 3 {
			t.Errorf("expected 3 changes, got %d: %v", len(changes), changes)
		}

		tables := getTableNames(t, db)
		if len(tables) != 2 {
			t.Errorf("expected 2 tables, got %v", tables)
		}
	})
}
