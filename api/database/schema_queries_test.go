package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"testing"
)

// =============================================================================
// mapColType Tests - Simple switch, minimal coverage
// =============================================================================

func TestMapColType(t *testing.T) {
	// Valid type (case insensitive)
	if got := mapColType("text"); got != "TEXT" {
		t.Errorf("mapColType('text') = %q, want 'TEXT'", got)
	}
	// Invalid type returns empty
	if got := mapColType("VARCHAR"); got != "" {
		t.Errorf("mapColType('VARCHAR') = %q, want ''", got)
	}
}

// =============================================================================
// mapOnAction Tests - Simple switch, minimal coverage
// =============================================================================

func TestMapOnAction(t *testing.T) {
	// Valid action (case insensitive)
	if got := mapOnAction("cascade"); got != "CASCADE" {
		t.Errorf("mapOnAction('cascade') = %q, want 'CASCADE'", got)
	}
	// Invalid action returns empty
	if got := mapOnAction("DELETE"); got != "" {
		t.Errorf("mapOnAction('DELETE') = %q, want ''", got)
	}
}

// =============================================================================
// CreateTable Tests - Distinct code paths in SQL generation
// =============================================================================

func TestCreateTable(t *testing.T) {
	ctx := context.Background()

	t.Run("creates table with primary key", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)

		body := encodeBody(map[string]Column{
			"id":   {Type: "INTEGER", PrimaryKey: true},
			"name": {Type: "TEXT"},
		})
		result, err := db.CreateTable(ctx, "users", body)
		if err != nil {
			t.Fatal(err)
		}

		if !containsStr(string(result), "created") {
			t.Errorf("expected success message, got %s", result)
		}

		// Verify schema was updated
		tbl, err := db.Schema.SearchTbls("users")
		if err != nil {
			t.Fatal("table not in schema after creation")
		}
		if tbl.Pk != "id" {
			t.Errorf("expected pk 'id', got %q", tbl.Pk)
		}
	})

	// String vs numeric default - different switch cases in code
	t.Run("string default uses quoted format", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)

		body := encodeBody(map[string]Column{
			"id":     {Type: "INTEGER", PrimaryKey: true},
			"status": {Type: "TEXT", Default: "active"},
		})
		_, err := db.CreateTable(ctx, "items", body)
		if err != nil {
			t.Fatal(err)
		}

		db.Client.Exec("INSERT INTO items (id) VALUES (1)")
		var status string
		db.Client.QueryRow("SELECT status FROM items WHERE id = 1").Scan(&status)
		if status != "active" {
			t.Errorf("expected default 'active', got %q", status)
		}
	})

	t.Run("numeric default uses unquoted format", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)

		body := encodeBody(map[string]Column{
			"id":    {Type: "INTEGER", PrimaryKey: true},
			"count": {Type: "INTEGER", Default: float64(0)}, // JSON numbers are float64
		})
		_, err := db.CreateTable(ctx, "counters", body)
		if err != nil {
			t.Fatal(err)
		}

		db.Client.Exec("INSERT INTO counters (id) VALUES (1)")
		var count int
		db.Client.QueryRow("SELECT count FROM counters WHERE id = 1").Scan(&count)
		if count != 0 {
			t.Errorf("expected default 0, got %d", count)
		}
	})

	// FK has complex reference parsing logic
	t.Run("foreign key parses table.column reference", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)

		db.Client.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)")
		db.updateSchema()

		body := encodeBody(map[string]Column{
			"id":      {Type: "INTEGER", PrimaryKey: true},
			"user_id": {Type: "INTEGER", References: "users.id"},
		})
		_, err := db.CreateTable(ctx, "posts", body)
		if err != nil {
			t.Fatal(err)
		}

		fk := db.Schema.findForeignKey("posts", "users")
		if fk.From != "user_id" || fk.To != "id" {
			t.Errorf("FK not created correctly: %+v", fk)
		}
	})

	// Error cases
	t.Run("invalid column type errors", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)

		body := encodeBody(map[string]Column{
			"id": {Type: "VARCHAR"},
		})
		_, err := db.CreateTable(ctx, "test_invalid", body)
		if err == nil {
			t.Error("expected error for invalid column type")
		}
	})

	t.Run("invalid FK reference errors", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)

		body := encodeBody(map[string]Column{
			"id":      {Type: "INTEGER", PrimaryKey: true},
			"user_id": {Type: "INTEGER", References: "nonexistent.id"},
		})
		_, err := db.CreateTable(ctx, "bad_fk", body)
		if err == nil {
			t.Error("expected error for invalid FK reference")
		}
	})
}

// =============================================================================
// AlterTable Tests - Distinct branches: rename/drop/add columns, rename table
// =============================================================================

func TestAlterTable(t *testing.T) {
	ctx := context.Background()

	// Each test covers a distinct branch in AlterTable
	t.Run("newColumns branch - adds column", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)
		db.Client.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)")
		db.updateSchema()

		body := encodeBody(map[string]any{
			"newColumns": map[string]any{
				"email": map[string]any{"type": "TEXT"},
			},
		})
		_, err := db.AlterTable(ctx, "users", body)
		if err != nil {
			t.Fatal(err)
		}

		tbl, _ := db.Schema.SearchTbls("users")
		if _, err = tbl.SearchCols("email"); err != nil {
			t.Error("column not added")
		}
	})

	t.Run("renameColumns branch", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)
		db.Client.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
		db.updateSchema()

		body := encodeBody(map[string]any{
			"renameColumns": map[string]string{"name": "full_name"},
		})
		_, err := db.AlterTable(ctx, "users", body)
		if err != nil {
			t.Fatal(err)
		}

		tbl, _ := db.Schema.SearchTbls("users")
		if _, err := tbl.SearchCols("name"); err == nil {
			t.Error("old name should not exist")
		}
		if _, err := tbl.SearchCols("full_name"); err != nil {
			t.Error("new name should exist")
		}
	})

	t.Run("dropColumns branch", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)
		db.Client.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, temp TEXT)")
		db.updateSchema()

		body := encodeBody(map[string]any{
			"dropColumns": []string{"temp"},
		})
		_, err := db.AlterTable(ctx, "users", body)
		if err != nil {
			t.Fatal(err)
		}

		tbl, _ := db.Schema.SearchTbls("users")
		if _, err := tbl.SearchCols("temp"); err == nil {
			t.Error("dropped column should not exist")
		}
	})

	t.Run("newName branch - renames table", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)
		db.Client.Exec("CREATE TABLE old_name (id INTEGER PRIMARY KEY)")
		db.updateSchema()

		body := encodeBody(map[string]any{
			"newName": "new_name",
		})
		_, err := db.AlterTable(ctx, "old_name", body)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := db.Schema.SearchTbls("old_name"); err == nil {
			t.Error("old table should not exist")
		}
		if _, err := db.Schema.SearchTbls("new_name"); err != nil {
			t.Error("new table should exist")
		}
	})

	// Error cases
	t.Run("non-existent table errors", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)

		body := encodeBody(map[string]any{
			"newColumns": map[string]any{"x": map[string]any{"type": "TEXT"}},
		})
		_, err := db.AlterTable(ctx, "nonexistent", body)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("invalid column type errors", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)
		db.Client.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY)")
		db.updateSchema()

		body := encodeBody(map[string]any{
			"newColumns": map[string]any{"x": map[string]any{"type": "VARCHAR"}},
		})
		_, err := db.AlterTable(ctx, "t", body)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// =============================================================================
// DropTable Tests
// =============================================================================

func TestDropTable(t *testing.T) {
	ctx := context.Background()

	t.Run("drops existing table", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)
		db.Client.Exec("CREATE TABLE to_drop (id INTEGER PRIMARY KEY)")
		db.updateSchema()

		// Verify table exists
		_, err := db.Schema.SearchTbls("to_drop")
		if err != nil {
			t.Fatal("table should exist before drop")
		}

		result, err := db.DropTable(ctx, "to_drop")
		if err != nil {
			t.Fatal(err)
		}

		if !containsStr(string(result), "dropped") {
			t.Errorf("expected success message, got %s", result)
		}

		// Verify table gone from schema
		_, err = db.Schema.SearchTbls("to_drop")
		if err == nil {
			t.Error("table should not exist after drop")
		}
	})

	t.Run("non-existent table errors", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)

		_, err := db.DropTable(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent table")
		}
	})

	t.Run("reserved table errors", func(t *testing.T) {
		db := setupSchemaQueryTestDB(t)

		_, err := db.DropTable(ctx, ReservedTableDatabases)
		if err == nil {
			t.Error("expected error for reserved table")
		}
	})
}

// =============================================================================
// Helpers
// =============================================================================

func setupSchemaQueryTestDB(t *testing.T) *Database {
	t.Helper()
	// Use in-memory database for isolation between tests
	client, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return &Database{Client: client, Schema: SchemaCache{}, id: 0}
}

func encodeBody(v any) io.ReadCloser {
	data, _ := json.Marshal(v)
	return io.NopCloser(bytes.NewReader(data))
}

func containsStr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
