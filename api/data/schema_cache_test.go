package data

import (
	"testing"

	"github.com/joe-ervin05/atomicbase/tools"
)

// Test fixtures for schema cache
var (
	// Table with foreign key reference
	testTablePosts = Table{
		Name: "posts",
		Pk:   []string{"id"},
		Columns: map[string]Col{
			"id":      {Name: "id", Type: "INTEGER", NotNull: true},
			"user_id": {Name: "user_id", Type: "INTEGER", References: "users.id"},
			"title":   {Name: "title", Type: "TEXT", NotNull: true},
		},
	}

	// Table with multiple foreign keys
	testTableComments = Table{
		Name: "comments",
		Pk:   []string{"id"},
		Columns: map[string]Col{
			"id":      {Name: "id", Type: "INTEGER", NotNull: true},
			"post_id": {Name: "post_id", Type: "INTEGER", References: "posts.id"},
			"user_id": {Name: "user_id", Type: "INTEGER", References: "users.id"},
			"body":    {Name: "body", Type: "TEXT", NotNull: true},
		},
	}

	// Table with no foreign keys
	testTableUsers = Table{
		Name: "users",
		Pk:   []string{"id"},
		Columns: map[string]Col{
			"id":   {Name: "id", Type: "INTEGER", NotNull: true},
			"name": {Name: "name", Type: "TEXT", NotNull: false},
		},
	}
)

// TestTablesToSchemaCache_ExtractsForeignKeys verifies FK extraction from column references.
func TestTablesToSchemaCache_ExtractsForeignKeys(t *testing.T) {
	tables := []Table{testTableUsers, testTablePosts}

	cache := TablesToSchemaCache(tables)

	// Check that posts table has FK to users
	postsFks := cache.Fks["posts"]
	if len(postsFks) != 1 {
		t.Fatalf("expected 1 FK for posts, got %d", len(postsFks))
	}

	fk := postsFks[0]
	if fk.Table != "posts" {
		t.Errorf("expected FK table 'posts', got %s", fk.Table)
	}
	if fk.From != "user_id" {
		t.Errorf("expected FK from 'user_id', got %s", fk.From)
	}
	if fk.References != "users" {
		t.Errorf("expected FK references 'users', got %s", fk.References)
	}
	if fk.To != "id" {
		t.Errorf("expected FK to 'id', got %s", fk.To)
	}
}

// TestTablesToSchemaCache_MultipleForeignKeys verifies extraction of multiple FKs per table.
func TestTablesToSchemaCache_MultipleForeignKeys(t *testing.T) {
	tables := []Table{testTableUsers, testTablePosts, testTableComments}

	cache := TablesToSchemaCache(tables)

	// Comments table should have 2 FKs
	commentsFks := cache.Fks["comments"]
	if len(commentsFks) != 2 {
		t.Fatalf("expected 2 FKs for comments, got %d", len(commentsFks))
	}

	// Verify both FKs exist (order may vary)
	hasPostFK := false
	hasUserFK := false
	for _, fk := range commentsFks {
		if fk.From == "post_id" && fk.References == "posts" && fk.To == "id" {
			hasPostFK = true
		}
		if fk.From == "user_id" && fk.References == "users" && fk.To == "id" {
			hasUserFK = true
		}
	}
	if !hasPostFK {
		t.Error("missing FK for post_id -> posts.id")
	}
	if !hasUserFK {
		t.Error("missing FK for user_id -> users.id")
	}
}

// TestTablesToSchemaCache_NoForeignKeys verifies tables without FKs work correctly.
func TestTablesToSchemaCache_NoForeignKeys(t *testing.T) {
	tables := []Table{testTableUsers}

	cache := TablesToSchemaCache(tables)

	if len(cache.Fks["users"]) != 0 {
		t.Errorf("expected no FKs for users, got %d", len(cache.Fks["users"]))
	}
}

// TestTablesToSchemaCache_TablesMap verifies tables are correctly keyed by name.
func TestTablesToSchemaCache_TablesMap(t *testing.T) {
	tables := []Table{testTableUsers, testTablePosts}

	cache := TablesToSchemaCache(tables)

	if len(cache.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(cache.Tables))
	}
	if _, ok := cache.Tables["users"]; !ok {
		t.Error("missing 'users' table in cache")
	}
	if _, ok := cache.Tables["posts"]; !ok {
		t.Error("missing 'posts' table in cache")
	}
}

// TestSchemaCache_StoreAndRetrieve verifies the generic cache in tools package.
func TestSchemaCache_StoreAndRetrieve(t *testing.T) {
	// Store a schema
	schema := TablesToSchemaCache([]Table{testTableUsers})
	tools.SchemaCache(999, 1, schema)

	// Retrieve it
	cached, ok := tools.SchemaFromCache(999, 1)
	if !ok {
		t.Fatal("expected to find cached schema")
	}

	retrieved := cached.(SchemaCache)
	if len(retrieved.Tables) != 1 {
		t.Errorf("expected 1 table, got %d", len(retrieved.Tables))
	}
	if _, ok := retrieved.Tables["users"]; !ok {
		t.Error("missing 'users' table in retrieved cache")
	}

	// Clean up
	tools.InvalidateSchema(999, 1)
}

// TestSchemaCache_Miss verifies cache miss behavior.
func TestSchemaCache_Miss(t *testing.T) {
	_, ok := tools.SchemaFromCache(99999, 99999)
	if ok {
		t.Error("expected cache miss for non-existent key")
	}
}

// TestSchemaCache_Invalidate verifies cache invalidation.
func TestSchemaCache_Invalidate(t *testing.T) {
	schema := TablesToSchemaCache([]Table{testTableUsers})
	tools.SchemaCache(998, 1, schema)

	// Verify it's there
	_, ok := tools.SchemaFromCache(998, 1)
	if !ok {
		t.Fatal("expected to find cached schema before invalidation")
	}

	// Invalidate
	tools.InvalidateSchema(998, 1)

	// Verify it's gone
	_, ok = tools.SchemaFromCache(998, 1)
	if ok {
		t.Error("expected cache miss after invalidation")
	}
}

// TestSchemaCache_InvalidateAll verifies bulk invalidation for a template.
func TestSchemaCache_InvalidateAll(t *testing.T) {
	schema := TablesToSchemaCache([]Table{testTableUsers})

	// Store multiple versions
	tools.SchemaCache(997, 1, schema)
	tools.SchemaCache(997, 2, schema)
	tools.SchemaCache(997, 3, schema)

	// Invalidate all for template 997
	tools.InvalidateAllSchemas(997)

	// Verify all are gone
	for v := 1; v <= 3; v++ {
		if _, ok := tools.SchemaFromCache(997, v); ok {
			t.Errorf("expected cache miss for version %d after InvalidateAll", v)
		}
	}
}
