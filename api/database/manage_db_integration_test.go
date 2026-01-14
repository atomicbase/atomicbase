//go:build integration

package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"
)

// =============================================================================
// Integration Tests - Require real Turso credentials
//
// Run with: go test -tags integration ./database/... -v
//
// Required environment variables:
//   - TURSO_ORGANIZATION
//   - TURSO_API_KEY
// =============================================================================

func skipIfNoCredentials(t *testing.T) {
	t.Helper()
	if os.Getenv("TURSO_ORGANIZATION") == "" {
		t.Skip("TURSO_ORGANIZATION not set")
	}
	if os.Getenv("TURSO_API_KEY") == "" {
		t.Skip("TURSO_API_KEY not set")
	}
}

func setupIntegrationDB(t *testing.T) (PrimaryDao, func()) {
	t.Helper()

	client, err := sql.Open("sqlite3", "file:integration_test.db")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create required tables
	_, err = client.Exec(`
		CREATE TABLE IF NOT EXISTS atomicbase_schema_templates (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			tables BLOB,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS atomicbase_databases (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE,
			token TEXT,
			schema BLOB,
			template_id INTEGER REFERENCES atomicbase_schema_templates(id)
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	dao := PrimaryDao{Database{Client: client, Schema: SchemaCache{}, id: 1}}

	return dao, func() {
		client.Close()
		os.Remove("integration_test.db")
	}
}

func TestIntegration_CreateAndDeleteDB(t *testing.T) {
	skipIfNoCredentials(t)
	dao, cleanup := setupIntegrationDB(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use timestamp to create unique DB name
	dbName := "integration_test_" + time.Now().Format("20060102150405")

	t.Run("create database", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{"name":"` + dbName + `"}`))
		result, err := dao.CreateDB(ctx, body)
		if err != nil {
			t.Fatalf("failed to create database: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		t.Logf("Create result: %s", result)
	})

	// Wait for Turso to propagate
	time.Sleep(2 * time.Second)

	t.Run("list databases includes new db", func(t *testing.T) {
		result, err := dao.ListDBs(ctx)
		if err != nil {
			t.Fatalf("failed to list databases: %v", err)
		}

		var dbs []map[string]any
		if err := json.Unmarshal(result, &dbs); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}

		found := false
		for _, db := range dbs {
			if db["name"] == dbName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find %s in list, got %s", dbName, result)
		}
	})

	t.Run("delete database", func(t *testing.T) {
		result, err := dao.DeleteDB(ctx, dbName)
		if err != nil {
			t.Fatalf("failed to delete database: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		t.Logf("Delete result: %s", result)
	})
}

func TestIntegration_RegisterDB(t *testing.T) {
	skipIfNoCredentials(t)
	dao, cleanup := setupIntegrationDB(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a test database first
	dbName := "integration_register_" + time.Now().Format("20060102150405")

	createBody := io.NopCloser(bytes.NewBufferString(`{"name":"` + dbName + `"}`))
	_, err := dao.CreateDB(ctx, createBody)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Clean up: delete from local registry so we can re-register
	dao.Client.Exec("DELETE FROM atomicbase_databases WHERE name = ?", dbName)

	time.Sleep(2 * time.Second)

	t.Run("register existing database", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{"name":"` + dbName + `"}`))
		result, err := dao.RegisterDB(ctx, body, "")
		if err != nil {
			t.Fatalf("failed to register database: %v", err)
		}
		t.Logf("Register result: %s", result)
	})

	// Cleanup
	t.Cleanup(func() {
		dao.DeleteDB(ctx, dbName)
	})
}

func TestIntegration_ListDBs(t *testing.T) {
	skipIfNoCredentials(t)
	dao, cleanup := setupIntegrationDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("list returns valid JSON", func(t *testing.T) {
		result, err := dao.ListDBs(ctx)
		if err != nil {
			t.Fatalf("failed to list databases: %v", err)
		}

		var dbs []map[string]any
		if err := json.Unmarshal(result, &dbs); err != nil {
			t.Fatalf("expected valid JSON array, got: %s", result)
		}
		t.Logf("Found %d databases", len(dbs))
	})
}
