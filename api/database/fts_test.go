package database

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"os"
	"testing"
)

// =============================================================================
// Helper
// =============================================================================

func checkFTS5Support(client *sql.DB) bool {
	_, err := client.Exec(`CREATE VIRTUAL TABLE fts5_test USING fts5(content)`)
	if err != nil {
		return false
	}
	client.Exec(`DROP TABLE fts5_test`)
	return true
}

func setupFTSTestDB(t *testing.T) (*Database, func()) {
	t.Helper()

	// Use sqlite3 driver (mattn/go-sqlite3) which has FTS5 support
	client, err := sql.Open("sqlite3", "file:fts_test.db")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if !checkFTS5Support(client) {
		client.Close()
		os.Remove("fts_test.db")
		t.Skip("FTS5 not supported in this SQLite build")
	}

	// Clean slate
	client.Exec("DROP TABLE IF EXISTS articles_fts")
	client.Exec("DROP TRIGGER IF EXISTS articles_fts_insert")
	client.Exec("DROP TRIGGER IF EXISTS articles_fts_delete")
	client.Exec("DROP TRIGGER IF EXISTS articles_fts_update")
	client.Exec("DROP TABLE IF EXISTS articles")
	client.Exec("DROP TABLE IF EXISTS numeric_table")

	_, err = client.Exec(`
		CREATE TABLE articles (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			author TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = client.Exec(`
		INSERT INTO articles (title, content, author) VALUES
		('Introduction to SQLite', 'SQLite is a lightweight database engine', 'John Doe'),
		('Full-Text Search Guide', 'Learn how to implement FTS5 in your applications', 'Jane Smith'),
		('Database Performance', 'Tips for optimizing database queries', 'John Doe')
	`)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	cols, _ := schemaCols(client)
	fks, _ := schemaFks(client)
	ftsTables, _ := schemaFTS(client)

	db := &Database{Client: client, Schema: SchemaCache{cols, fks, ftsTables}, id: 0}

	return db, func() {
		client.Close()
		os.Remove("fts_test.db")
	}
}

// =============================================================================
// CreateFTSIndex Tests - Error cases
// =============================================================================

func TestCreateFTSIndex(t *testing.T) {
	db, cleanup := setupFTSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("error on invalid table", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{"columns": ["title"]}`))
		_, err := db.CreateFTSIndex(ctx, "nonexistent", body)
		if err == nil {
			t.Error("expected error for invalid table")
		}
	})

	t.Run("error on non-TEXT column", func(t *testing.T) {
		_, err := db.Client.Exec(`CREATE TABLE numeric_table (id INTEGER PRIMARY KEY, value INTEGER)`)
		if err != nil {
			t.Fatalf("failed to create numeric table: %v", err)
		}
		db.InvalidateSchema(ctx)

		body := io.NopCloser(bytes.NewBufferString(`{"columns": ["value"]}`))
		_, err = db.CreateFTSIndex(ctx, "numeric_table", body)
		if err == nil {
			t.Error("expected error for non-TEXT column")
		}
	})

	t.Run("error on duplicate FTS index", func(t *testing.T) {
		// Create first index
		body := io.NopCloser(bytes.NewBufferString(`{"columns": ["title"]}`))
		_, err := db.CreateFTSIndex(ctx, "articles", body)
		if err != nil {
			t.Fatalf("failed to create first FTS index: %v", err)
		}

		// Try duplicate
		body = io.NopCloser(bytes.NewBufferString(`{"columns": ["title"]}`))
		_, err = db.CreateFTSIndex(ctx, "articles", body)
		if err == nil {
			t.Error("expected error for duplicate FTS index")
		}
	})
}

// =============================================================================
// DropFTSIndex Tests - Error case
// =============================================================================

func TestDropFTSIndex(t *testing.T) {
	db, cleanup := setupFTSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("error on nonexistent FTS index", func(t *testing.T) {
		_, err := db.DropFTSIndex(ctx, "articles")
		if err == nil {
			t.Error("expected error for dropping nonexistent FTS index")
		}
	})
}

// =============================================================================
// FTS Triggers Tests - Complex context (INSERT/UPDATE/DELETE sync)
// =============================================================================

func TestFTSTriggers(t *testing.T) {
	db, cleanup := setupFTSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create FTS index
	body := io.NopCloser(bytes.NewBufferString(`{"columns": ["title", "content"]}`))
	_, err := db.CreateFTSIndex(ctx, "articles", body)
	if err != nil {
		t.Fatalf("failed to create FTS index: %v", err)
	}

	t.Run("insert triggers FTS update", func(t *testing.T) {
		_, err := db.Client.Exec(`INSERT INTO articles (title, content, author) VALUES ('Rust Programming', 'Building fast applications with Rust', 'Bob Wilson')`)
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}

		result, err := db.SelectJSON(ctx, "articles", SelectQuery{
			Where: []map[string]any{{"title": map[string]any{"fts": "Rust"}}},
		}, false)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if !bytes.Contains(result.Data, []byte("Rust")) {
			t.Errorf("expected result to contain 'Rust', got %s", result.Data)
		}
	})

	t.Run("update triggers FTS update", func(t *testing.T) {
		_, err := db.Client.Exec(`UPDATE articles SET title = 'Python Guide' WHERE title = 'Rust Programming'`)
		if err != nil {
			t.Fatalf("failed to update: %v", err)
		}

		// New title should match
		result, err := db.SelectJSON(ctx, "articles", SelectQuery{
			Where: []map[string]any{{"title": map[string]any{"fts": "Python"}}},
		}, false)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if !bytes.Contains(result.Data, []byte("Python")) {
			t.Errorf("expected result to contain 'Python', got %s", result.Data)
		}

		// Old title should not match
		result, err = db.SelectJSON(ctx, "articles", SelectQuery{
			Where: []map[string]any{{"title": map[string]any{"fts": "Rust"}}},
		}, false)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if bytes.Contains(result.Data, []byte("Rust Programming")) {
			t.Error("expected old title not to match")
		}
	})

	t.Run("delete triggers FTS update", func(t *testing.T) {
		_, err := db.Client.Exec(`DELETE FROM articles WHERE title = 'Python Guide'`)
		if err != nil {
			t.Fatalf("failed to delete: %v", err)
		}

		result, err := db.SelectJSON(ctx, "articles", SelectQuery{
			Where: []map[string]any{{"title": map[string]any{"fts": "Python"}}},
		}, false)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if bytes.Contains(result.Data, []byte("Python")) {
			t.Error("expected deleted article not to appear in search results")
		}
	})
}

// =============================================================================
// FTS Search Error - Requires index
// =============================================================================

func TestFTSSearchRequiresIndex(t *testing.T) {
	db, cleanup := setupFTSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// No FTS index created
	_, err := db.SelectJSON(ctx, "articles", SelectQuery{
		Where: []map[string]any{{"title": map[string]any{"fts": "SQLite"}}},
	}, false)
	if err == nil {
		t.Error("expected error when searching without FTS index")
	}
}
