package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
)

// =============================================================================
// SQL Schema Constants
// =============================================================================

// schemaUsersAndCars: FK relationships for testing joins
const schemaUsersAndCars = `
	DROP TABLE IF EXISTS cars;
	DROP TABLE IF EXISTS users;
	CREATE TABLE users (
		name TEXT,
		username TEXT UNIQUE
	);
	CREATE TABLE cars (
		id INTEGER PRIMARY KEY,
		make TEXT,
		model TEXT,
		user_id INTEGER,
		FOREIGN KEY(user_id) REFERENCES users(rowid)
	);
`

// schemaSimpleTable: Basic table for CRUD tests
const schemaSimpleTable = `
	DROP TABLE IF EXISTS items;
	CREATE TABLE items (
		id INTEGER PRIMARY KEY,
		name TEXT,
		value INTEGER
	);
`

// =============================================================================
// Helper Functions
// =============================================================================

func setupTestDB(t *testing.T) *Database {
	t.Helper()
	client, err := sql.Open("sqlite3", "file:test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return &Database{Client: client, Schema: SchemaCache{}, id: 0}
}

func setupSchema(t *testing.T, db *Database, schema string) {
	t.Helper()
	if _, err := db.Client.Exec(schema); err != nil {
		t.Fatal(err)
	}
	db.updateSchema()
}

func parseJSON(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var result []map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	return result
}

// =============================================================================
// CRUD Tests - Minimal coverage for error paths and edge cases
// =============================================================================

func TestSelectJSON(t *testing.T) {
	db := setupTestDB(t)
	setupSchema(t, db, schemaSimpleTable)
	ctx := context.Background()

	// Insert test data
	db.Client.Exec("INSERT INTO items (name, value) VALUES ('a', 1), ('b', 2), ('c', 3), ('d', 4), ('e', 5)")

	t.Run("basic select works", func(t *testing.T) {
		result, err := db.SelectJSON(ctx, "items", SelectQuery{}, false)
		if err != nil {
			t.Fatal(err)
		}
		rows := parseJSON(t, result.Data)
		if len(rows) != 5 {
			t.Errorf("expected 5 rows, got %d", len(rows))
		}
	})

	t.Run("non-existent table errors", func(t *testing.T) {
		_, err := db.SelectJSON(ctx, "nonexistent", SelectQuery{}, false)
		if !errors.Is(err, ErrTableNotFound) {
			t.Errorf("expected ErrTableNotFound, got %v", err)
		}
	})

	t.Run("invalid column errors", func(t *testing.T) {
		_, err := db.SelectJSON(ctx, "items", SelectQuery{
			Select: []any{"nonexistent"},
		}, false)
		if err == nil {
			t.Error("expected error for invalid column")
		}
	})

	// Pagination boundary conditions - negative values ignored
	t.Run("negative limit uses default", func(t *testing.T) {
		negLimit := -1
		result, err := db.SelectJSON(ctx, "items", SelectQuery{Limit: &negLimit}, false)
		if err != nil {
			t.Fatal(err)
		}
		rows := parseJSON(t, result.Data)
		// Negative limit ignored, should return all rows (or default limit)
		if len(rows) == 0 {
			t.Error("negative limit should be ignored, not return 0 rows")
		}
	})

	t.Run("negative offset uses zero", func(t *testing.T) {
		negOffset := -5
		limit := 2
		result, err := db.SelectJSON(ctx, "items", SelectQuery{Limit: &limit, Offset: &negOffset}, false)
		if err != nil {
			t.Fatal(err)
		}
		rows := parseJSON(t, result.Data)
		// Negative offset ignored, should start from beginning
		if len(rows) != 2 {
			t.Errorf("expected 2 rows with limit=2, got %d", len(rows))
		}
		if rows[0]["name"] != "a" {
			t.Errorf("expected first row 'a', got %v (offset not ignored)", rows[0]["name"])
		}
	})

	t.Run("limit and offset work together", func(t *testing.T) {
		limit := 2
		offset := 2
		result, err := db.SelectJSON(ctx, "items", SelectQuery{Limit: &limit, Offset: &offset}, false)
		if err != nil {
			t.Fatal(err)
		}
		rows := parseJSON(t, result.Data)
		if len(rows) != 2 {
			t.Errorf("expected 2 rows, got %d", len(rows))
		}
		// With 5 rows (a,b,c,d,e), offset 2 should skip a,b and return c,d
		if rows[0]["name"] != "c" {
			t.Errorf("expected row 'c' at offset 2, got %v", rows[0]["name"])
		}
	})
}

func TestInsertJSON(t *testing.T) {
	db := setupTestDB(t)
	setupSchema(t, db, schemaSimpleTable)
	ctx := context.Background()

	t.Run("basic insert works", func(t *testing.T) {
		resp, err := db.InsertJSON(ctx, "items", InsertRequest{
			Data: map[string]any{"name": "test", "value": 42},
		})
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		json.Unmarshal(resp, &result)
		if result["last_insert_id"] == nil {
			t.Error("expected last_insert_id")
		}
	})

	t.Run("invalid column errors", func(t *testing.T) {
		_, err := db.InsertJSON(ctx, "items", InsertRequest{
			Data: map[string]any{"invalid": "value"},
		})
		if err == nil {
			t.Error("expected error for invalid column")
		}
	})

	// RETURNING clause - distinct code path: QueryJSON instead of ExecContext
	t.Run("RETURNING * returns all columns", func(t *testing.T) {
		resp, err := db.InsertJSON(ctx, "items", InsertRequest{
			Data:      map[string]any{"name": "returning_test", "value": 100},
			Returning: []string{"*"},
		})
		if err != nil {
			t.Fatal(err)
		}
		rows := parseJSON(t, resp)
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		if rows[0]["name"] != "returning_test" || rows[0]["value"].(float64) != 100 {
			t.Errorf("unexpected row data: %v", rows[0])
		}
	})

	t.Run("RETURNING specific columns", func(t *testing.T) {
		resp, err := db.InsertJSON(ctx, "items", InsertRequest{
			Data:      map[string]any{"name": "specific_test", "value": 200},
			Returning: []string{"name"},
		})
		if err != nil {
			t.Fatal(err)
		}
		rows := parseJSON(t, resp)
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		// Should only have the requested column
		if rows[0]["name"] != "specific_test" {
			t.Errorf("expected name=specific_test, got %v", rows[0]["name"])
		}
	})

	t.Run("RETURNING invalid column errors", func(t *testing.T) {
		_, err := db.InsertJSON(ctx, "items", InsertRequest{
			Data:      map[string]any{"name": "error_test", "value": 1},
			Returning: []string{"nonexistent"},
		})
		if err == nil {
			t.Error("expected error for invalid RETURNING column")
		}
	})
}

func TestUpsertJSON(t *testing.T) {
	db := setupTestDB(t)
	setupSchema(t, db, schemaSimpleTable)
	ctx := context.Background()

	t.Run("upsert with PK conflict updates", func(t *testing.T) {
		// Insert
		db.UpsertJSON(ctx, "items", UpsertRequest{
			Data: []map[string]any{{"id": 1, "name": "original", "value": 10}},
		})

		// Upsert same PK
		_, err := db.UpsertJSON(ctx, "items", UpsertRequest{
			Data: []map[string]any{{"id": 1, "name": "updated", "value": 20}},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Verify update
		result, _ := db.SelectJSON(ctx, "items", SelectQuery{}, false)
		rows := parseJSON(t, result.Data)
		if len(rows) != 1 || rows[0]["name"] != "updated" {
			t.Errorf("upsert did not update: %v", rows)
		}
	})

	t.Run("empty data errors", func(t *testing.T) {
		_, err := db.UpsertJSON(ctx, "items", UpsertRequest{Data: []map[string]any{}})
		if err == nil {
			t.Error("expected error for empty data")
		}
	})

	// RETURNING with multi-row upsert - tests RETURNING in ON CONFLICT context
	t.Run("RETURNING with multi-row upsert", func(t *testing.T) {
		resp, err := db.UpsertJSON(ctx, "items", UpsertRequest{
			Data: []map[string]any{
				{"id": 100, "name": "batch1", "value": 1},
				{"id": 101, "name": "batch2", "value": 2},
			},
			Returning: []string{"id", "name"},
		})
		if err != nil {
			t.Fatal(err)
		}
		rows := parseJSON(t, resp)
		if len(rows) != 2 {
			t.Errorf("expected 2 rows returned, got %d", len(rows))
		}
	})
}

func TestUpdateJSON(t *testing.T) {
	db := setupTestDB(t)
	setupSchema(t, db, schemaSimpleTable)
	ctx := context.Background()

	db.Client.Exec("INSERT INTO items (name, value) VALUES ('test', 10)")

	t.Run("update with where works", func(t *testing.T) {
		_, err := db.UpdateJSON(ctx, "items", UpdateRequest{
			Data:  map[string]any{"value": 99},
			Where: []map[string]any{{"name": map[string]any{"eq": "test"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid column errors", func(t *testing.T) {
		_, err := db.UpdateJSON(ctx, "items", UpdateRequest{
			Data:  map[string]any{"invalid": "value"},
			Where: []map[string]any{{"name": map[string]any{"eq": "test"}}},
		})
		if err == nil {
			t.Error("expected error for invalid column")
		}
	})
}

func TestDeleteJSON(t *testing.T) {
	db := setupTestDB(t)
	setupSchema(t, db, schemaSimpleTable)
	ctx := context.Background()

	t.Run("delete without where errors", func(t *testing.T) {
		_, err := db.DeleteJSON(ctx, "items", DeleteRequest{})
		if !errors.Is(err, ErrMissingWhereClause) {
			t.Errorf("expected ErrMissingWhereClause, got %v", err)
		}
	})

	t.Run("delete with where works", func(t *testing.T) {
		db.Client.Exec("INSERT INTO items (name) VALUES ('delete_me')")
		resp, err := db.DeleteJSON(ctx, "items", DeleteRequest{
			Where: []map[string]any{{"name": map[string]any{"eq": "delete_me"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		json.Unmarshal(resp, &result)
		if result["rows_affected"].(float64) != 1 {
			t.Errorf("expected 1 row affected, got %v", result["rows_affected"])
		}
	})
}

// =============================================================================
// QueryMap Tests - NULL handling and type conversion edge cases
// =============================================================================

// schemaAllTypes: Table with all SQLite types for NULL testing
const schemaAllTypes = `
	DROP TABLE IF EXISTS all_types;
	CREATE TABLE all_types (
		id INTEGER PRIMARY KEY,
		text_col TEXT,
		int_col INTEGER,
		real_col REAL,
		blob_col BLOB
	);
`

func TestQueryMap(t *testing.T) {
	db := setupTestDB(t)
	setupSchema(t, db, schemaAllTypes)
	ctx := context.Background()

	// NULL handling - each type has distinct Valid/Invalid branch
	t.Run("NULL values return nil for each type", func(t *testing.T) {
		// Insert row with all NULL values (except PK)
		db.Client.Exec("INSERT INTO all_types (id) VALUES (1)")

		results, err := db.QueryMap(ctx, "SELECT text_col, int_col, real_col, blob_col FROM all_types WHERE id = 1")
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 row, got %d", len(results))
		}

		row := results[0].(map[string]interface{})

		// Each NULL should be nil (tests lines 287, 296, 305)
		if row["text_col"] != nil {
			t.Errorf("expected nil for NULL TEXT, got %v", row["text_col"])
		}
		if row["int_col"] != nil {
			t.Errorf("expected nil for NULL INTEGER, got %v", row["int_col"])
		}
		if row["real_col"] != nil {
			t.Errorf("expected nil for NULL REAL, got %v", row["real_col"])
		}
	})

	t.Run("non-NULL values return correct types", func(t *testing.T) {
		db.Client.Exec("INSERT INTO all_types (id, text_col, int_col, real_col, blob_col) VALUES (2, 'hello', 42, 3.14, X'DEADBEEF')")

		results, err := db.QueryMap(ctx, "SELECT text_col, int_col, real_col, blob_col FROM all_types WHERE id = 2")
		if err != nil {
			t.Fatal(err)
		}

		row := results[0].(map[string]interface{})

		// Verify correct type conversion (tests lines 285, 294, 303, 310)
		if str, ok := row["text_col"].(string); !ok || str != "hello" {
			t.Errorf("expected string 'hello', got %T: %v", row["text_col"], row["text_col"])
		}
		if i, ok := row["int_col"].(int64); !ok || i != 42 {
			t.Errorf("expected int64 42, got %T: %v", row["int_col"], row["int_col"])
		}
		if f, ok := row["real_col"].(float64); !ok || f != 3.14 {
			t.Errorf("expected float64 3.14, got %T: %v", row["real_col"], row["real_col"])
		}
		// BLOB returns *sql.RawBytes
		if row["blob_col"] == nil {
			t.Error("expected non-nil BLOB value")
		}
	})

	// Empty result set - edge case
	t.Run("empty result returns empty slice", func(t *testing.T) {
		results, err := db.QueryMap(ctx, "SELECT * FROM all_types WHERE id = 9999")
		if err != nil {
			t.Fatal(err)
		}
		if results == nil {
			t.Error("expected empty slice, got nil")
		}
		if len(results) != 0 {
			t.Errorf("expected 0 rows, got %d", len(results))
		}
	})
}

// =============================================================================
// Reserved Table Access - Security check
// =============================================================================

func TestReservedTableAccess(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// All operations on reserved tables should fail
	_, err := db.SelectJSON(ctx, ReservedTableDatabases, SelectQuery{}, false)
	if err == nil {
		t.Error("expected error for reserved table access")
	}
}
