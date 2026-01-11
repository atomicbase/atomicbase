package database

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/url"
	"os"
	"testing"
)

// checkFTS5Support checks if FTS5 is available in the current SQLite build
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

	// Use file-based database for FTS5 support
	client, err := sql.Open("libsql", "file:fts_test.db")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Check if FTS5 is available
	if !checkFTS5Support(client) {
		client.Close()
		os.Remove("fts_test.db")
		t.Skip("FTS5 not supported in this SQLite build - skipping FTS tests")
	}

	// Clean up any existing tables
	client.Exec("DROP TABLE IF EXISTS articles_fts")
	client.Exec("DROP TRIGGER IF EXISTS articles_fts_insert")
	client.Exec("DROP TRIGGER IF EXISTS articles_fts_delete")
	client.Exec("DROP TRIGGER IF EXISTS articles_fts_update")
	client.Exec("DROP TABLE IF EXISTS articles")
	client.Exec("DROP TABLE IF EXISTS test_table")
	client.Exec("DROP TABLE IF EXISTS numeric_table")

	// Create a test table with TEXT columns for FTS
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

	// Insert test data
	_, err = client.Exec(`
		INSERT INTO articles (title, content, author) VALUES
		('Introduction to SQLite', 'SQLite is a lightweight database engine', 'John Doe'),
		('Full-Text Search Guide', 'Learn how to implement FTS5 in your applications', 'Jane Smith'),
		('Database Performance', 'Tips for optimizing database queries', 'John Doe'),
		('Go Programming', 'Building web applications with Go and SQLite', 'Alice Brown')
	`)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	// Get schema
	cols, _ := schemaCols(client)
	fks, _ := schemaFks(client)
	ftsTables, _ := schemaFTS(client)

	db := &Database{Client: client, Schema: SchemaCache{cols, fks, ftsTables}, id: 0}

	return db, func() {
		client.Close()
		os.Remove("fts_test.db")
	}
}

func TestCreateFTSIndex(t *testing.T) {
	db, cleanup := setupFTSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("create FTS index on single column", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{"columns": ["title"]}`))
		result, err := db.CreateFTSIndex(ctx, "articles", body)
		if err != nil {
			t.Fatalf("failed to create FTS index: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}

		// Verify FTS table exists
		if !db.Schema.HasFTSIndex("articles") {
			t.Error("expected articles to have FTS index")
		}
	})

	t.Run("error on duplicate FTS index", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{"columns": ["title"]}`))
		_, err := db.CreateFTSIndex(ctx, "articles", body)
		if err == nil {
			t.Error("expected error for duplicate FTS index")
		}
	})

	t.Run("error on invalid table", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{"columns": ["title"]}`))
		_, err := db.CreateFTSIndex(ctx, "nonexistent", body)
		if err == nil {
			t.Error("expected error for invalid table")
		}
	})

	t.Run("error on invalid column", func(t *testing.T) {
		// Create a new table without FTS for this test
		_, err := db.Client.Exec(`CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)`)
		if err != nil {
			t.Fatalf("failed to create test table: %v", err)
		}
		db.InvalidateSchema(ctx)

		body := io.NopCloser(bytes.NewBufferString(`{"columns": ["nonexistent"]}`))
		_, err = db.CreateFTSIndex(ctx, "test_table", body)
		if err == nil {
			t.Error("expected error for invalid column")
		}
	})

	t.Run("error on non-TEXT column", func(t *testing.T) {
		// Create a new table with non-TEXT column
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
}

func TestDropFTSIndex(t *testing.T) {
	db, cleanup := setupFTSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// First create an FTS index
	body := io.NopCloser(bytes.NewBufferString(`{"columns": ["title", "content"]}`))
	_, err := db.CreateFTSIndex(ctx, "articles", body)
	if err != nil {
		t.Fatalf("failed to create FTS index: %v", err)
	}

	t.Run("drop FTS index", func(t *testing.T) {
		result, err := db.DropFTSIndex(ctx, "articles")
		if err != nil {
			t.Fatalf("failed to drop FTS index: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}

		// Verify FTS table is removed
		if db.Schema.HasFTSIndex("articles") {
			t.Error("expected articles to not have FTS index after drop")
		}
	})

	t.Run("error on nonexistent FTS index", func(t *testing.T) {
		_, err := db.DropFTSIndex(ctx, "articles")
		if err == nil {
			t.Error("expected error for dropping nonexistent FTS index")
		}
	})
}

func TestListFTSIndexes(t *testing.T) {
	db, cleanup := setupFTSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("empty list when no FTS indexes", func(t *testing.T) {
		result, err := db.ListFTSIndexes()
		if err != nil {
			t.Fatalf("failed to list FTS indexes: %v", err)
		}
		if string(result) != "null" && string(result) != "[]" {
			t.Errorf("expected empty list, got %s", result)
		}
	})

	// Create FTS index
	body := io.NopCloser(bytes.NewBufferString(`{"columns": ["title", "content"]}`))
	_, err := db.CreateFTSIndex(ctx, "articles", body)
	if err != nil {
		t.Fatalf("failed to create FTS index: %v", err)
	}

	t.Run("list with FTS index", func(t *testing.T) {
		result, err := db.ListFTSIndexes()
		if err != nil {
			t.Fatalf("failed to list FTS indexes: %v", err)
		}
		if !bytes.Contains(result, []byte("articles")) {
			t.Errorf("expected result to contain 'articles', got %s", result)
		}
	})
}

func TestFTSSearch(t *testing.T) {
	db, cleanup := setupFTSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create FTS index on title and content
	body := io.NopCloser(bytes.NewBufferString(`{"columns": ["title", "content"]}`))
	_, err := db.CreateFTSIndex(ctx, "articles", body)
	if err != nil {
		t.Fatalf("failed to create FTS index: %v", err)
	}

	t.Run("search single term", func(t *testing.T) {
		params := url.Values{}
		params.Set("title", "fts.SQLite")

		result, err := db.Select(ctx, "articles", params)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if !bytes.Contains(result, []byte("SQLite")) {
			t.Errorf("expected result to contain 'SQLite', got %s", result)
		}
	})

	t.Run("search multiple terms", func(t *testing.T) {
		params := url.Values{}
		params.Set("content", "fts.database queries")

		result, err := db.Select(ctx, "articles", params)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if !bytes.Contains(result, []byte("Performance")) {
			t.Errorf("expected result to contain 'Performance', got %s", result)
		}
	})

	t.Run("error when no FTS index", func(t *testing.T) {
		// Drop the FTS index
		_, err := db.DropFTSIndex(ctx, "articles")
		if err != nil {
			t.Fatalf("failed to drop FTS index: %v", err)
		}

		params := url.Values{}
		params.Set("title", "fts.SQLite")

		_, err = db.Select(ctx, "articles", params)
		if err == nil {
			t.Error("expected error when searching without FTS index")
		}
	})
}

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
		// Insert a new article
		_, err := db.Client.Exec(`INSERT INTO articles (title, content, author) VALUES ('Rust Programming', 'Building fast applications with Rust', 'Bob Wilson')`)
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}

		// Search for the new article
		params := url.Values{}
		params.Set("title", "fts.Rust")

		result, err := db.Select(ctx, "articles", params)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if !bytes.Contains(result, []byte("Rust")) {
			t.Errorf("expected result to contain 'Rust', got %s", result)
		}
	})

	t.Run("update triggers FTS update", func(t *testing.T) {
		// Update an article
		_, err := db.Client.Exec(`UPDATE articles SET title = 'Python Programming' WHERE title = 'Go Programming'`)
		if err != nil {
			t.Fatalf("failed to update: %v", err)
		}

		// Search for the updated title
		params := url.Values{}
		params.Set("title", "fts.Python")

		result, err := db.Select(ctx, "articles", params)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if !bytes.Contains(result, []byte("Python")) {
			t.Errorf("expected result to contain 'Python', got %s", result)
		}

		// Old title should not match
		params.Set("title", "fts.Go")
		result, err = db.Select(ctx, "articles", params)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if bytes.Contains(result, []byte("Go Programming")) {
			t.Errorf("expected old title not to match")
		}
	})

	t.Run("delete triggers FTS update", func(t *testing.T) {
		// Delete an article
		_, err := db.Client.Exec(`DELETE FROM articles WHERE title = 'Python Programming'`)
		if err != nil {
			t.Fatalf("failed to delete: %v", err)
		}

		// Search should not find deleted article
		params := url.Values{}
		params.Set("title", "fts.Python")

		result, err := db.Select(ctx, "articles", params)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if bytes.Contains(result, []byte("Python")) {
			t.Errorf("expected deleted article not to appear in search results")
		}
	})
}

func TestHasFTSIndex(t *testing.T) {
	schema := SchemaCache{
		FTSTables: []string{"articles", "posts", "users"},
	}

	tests := []struct {
		table    string
		expected bool
	}{
		{"articles", true},
		{"posts", true},
		{"users", true},
		{"comments", false},
		{"", false},
		{"article", false}, // partial match should fail
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			result := schema.HasFTSIndex(tt.table)
			if result != tt.expected {
				t.Errorf("HasFTSIndex(%s) = %v, want %v", tt.table, result, tt.expected)
			}
		})
	}
}
