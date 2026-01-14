package database

import (
	"database/sql"
	"os"
	"testing"
)

// =============================================================================
// Test Schema Constants - Each tests a specific edge case
// =============================================================================

// schemaComprehensive: Realistic schema with FKs, constraints, defaults, composite PK
const schemaComprehensive = `
CREATE TABLE users (
	id INTEGER PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	name TEXT DEFAULT 'Anonymous'
);
CREATE TABLE posts (
	id INTEGER PRIMARY KEY,
	title TEXT NOT NULL,
	user_id INTEGER NOT NULL,
	FOREIGN KEY(user_id) REFERENCES users(id)
);
CREATE TABLE comments (
	id INTEGER PRIMARY KEY,
	post_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	FOREIGN KEY(post_id) REFERENCES posts(id),
	FOREIGN KEY(user_id) REFERENCES users(id)
);
CREATE TABLE post_tags (
	post_id INTEGER NOT NULL,
	tag_id INTEGER NOT NULL,
	PRIMARY KEY(post_id, tag_id)
);
`

// schemaWithView: Tests that views are discovered alongside tables
const schemaWithView = `
CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, price REAL);
CREATE VIEW expensive AS SELECT * FROM products WHERE price > 100;
`

// =============================================================================
// Helper
// =============================================================================

func setupSchemaTestDB(t *testing.T, name string) (*sql.DB, func()) {
	t.Helper()
	dbPath := name + ".db"
	client, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	return client, func() {
		client.Close()
		os.Remove(dbPath)
	}
}

// =============================================================================
// Schema Discovery Tests
// =============================================================================

func TestSchemaCols(t *testing.T) {
	client, cleanup := setupSchemaTestDB(t, "testSchema")
	defer cleanup()

	_, err := client.Exec(schemaComprehensive)
	if err != nil {
		t.Fatal(err)
	}

	tbls, err := schemaCols(client)
	if err != nil {
		t.Fatal(err)
	}

	// Verify table count
	if len(tbls) != 4 {
		t.Errorf("expected 4 tables, got %d", len(tbls))
	}

	// Verify FK references are embedded in columns
	posts, exists := tbls["posts"]
	if !exists {
		t.Fatal("posts table not found")
	}

	col, exists := posts.Columns["user_id"]
	if !exists {
		t.Fatal("user_id column not found")
	}
	if col.References != "users.id" {
		t.Errorf("expected user_id.References='users.id', got %q", col.References)
	}
}

func TestSchemaFks(t *testing.T) {
	client, cleanup := setupSchemaTestDB(t, "testFks")
	defer cleanup()

	_, err := client.Exec(schemaComprehensive)
	if err != nil {
		t.Fatal(err)
	}

	fks, err := schemaFks(client)
	if err != nil {
		t.Fatal(err)
	}

	// posts->users, comments->posts, comments->users = 3 FKs
	// Count total FKs across all tables
	totalFKs := 0
	for _, tableFks := range fks {
		totalFKs += len(tableFks)
	}
	if totalFKs != 3 {
		t.Errorf("expected 3 FKs, got %d", totalFKs)
	}
}

func TestSchemaWithViews(t *testing.T) {
	client, cleanup := setupSchemaTestDB(t, "testViews")
	defer cleanup()

	_, err := client.Exec(schemaWithView)
	if err != nil {
		t.Fatal(err)
	}

	tbls, err := schemaCols(client)
	if err != nil {
		t.Fatal(err)
	}

	foundTable := false
	foundView := false
	for _, tbl := range tbls {
		if tbl.Name == "products" {
			foundTable = true
		}
		if tbl.Name == "expensive" {
			foundView = true
		}
	}

	if !foundTable || !foundView {
		t.Errorf("table=%v view=%v, expected both true", foundTable, foundView)
	}
}

// =============================================================================
// Map Lookup Tests - Critical for correctness
// =============================================================================

func TestSearchTbls(t *testing.T) {
	schema := SchemaCache{
		Tables: map[string]Table{
			"a": {Name: "a"},
			"b": {Name: "b"},
			"c": {Name: "c"},
		},
	}

	// Found
	if tbl, err := schema.SearchTbls("b"); err != nil || tbl.Name != "b" {
		t.Errorf("SearchTbls('b') failed")
	}

	// Not found
	if _, err := schema.SearchTbls("z"); err == nil {
		t.Error("expected error for missing table")
	}

	// Empty
	empty := SchemaCache{}
	if _, err := empty.SearchTbls("x"); err == nil {
		t.Error("expected error for empty schema")
	}
}

func TestSearchCols(t *testing.T) {
	tbl := Table{
		Columns: map[string]Col{
			"a": {Name: "a"},
			"b": {Name: "b"},
			"c": {Name: "c"},
		},
	}

	// Found
	if col, err := tbl.SearchCols("b"); err != nil || col.Name != "b" {
		t.Errorf("SearchCols('b') failed")
	}

	// Not found
	if _, err := tbl.SearchCols("z"); err == nil {
		t.Error("expected error for missing column")
	}
}

func TestSearchFks(t *testing.T) {
	schema := SchemaCache{
		Fks: map[string][]Fk{
			"a": {{Table: "a", References: "x"}},
			"b": {{Table: "b", References: "y"}},
		},
	}

	// Found
	if _, found := schema.SearchFks("b", "y"); !found {
		t.Error("expected to find FK")
	}

	// Not found
	if _, found := schema.SearchFks("z", "z"); found {
		t.Error("expected not to find missing FK")
	}
}

// =============================================================================
// parseDefaultValue - Edge cases for SQLite default value parsing
// =============================================================================

func TestParseDefaultValue(t *testing.T) {
	tests := []struct {
		input string
		want  any
	}{
		{"'hello'", "hello"}, // quoted string
		{`"world"`, "world"}, // double quoted
		{"NULL", nil},        // null
		{"42", "42"},         // number as string
		{"CURRENT_TIMESTAMP", "CURRENT_TIMESTAMP"}, // expression
	}

	for _, tt := range tests {
		if got := parseDefaultValue(tt.input); got != tt.want {
			t.Errorf("parseDefaultValue(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
