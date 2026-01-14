package database

import (
	"context"
	"errors"
	"testing"
)

// =============================================================================
// Batch Tests - Atomic transaction behavior and edge cases
// =============================================================================

func TestBatch(t *testing.T) {
	db := setupTestDB(t)
	setupSchema(t, db, schemaSimpleTable)
	ctx := context.Background()

	t.Run("empty batch returns empty results", func(t *testing.T) {
		result, err := db.Batch(ctx, BatchRequest{Operations: []BatchOperation{}})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Results) != 0 {
			t.Errorf("expected 0 results, got %d", len(result.Results))
		}
	})

	t.Run("multiple inserts succeed atomically", func(t *testing.T) {
		result, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "insert", Table: "items", Body: map[string]any{"data": map[string]any{"name": "batch1", "value": 1}}},
				{Operation: "insert", Table: "items", Body: map[string]any{"data": map[string]any{"name": "batch2", "value": 2}}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Results) != 2 {
			t.Errorf("expected 2 results, got %d", len(result.Results))
		}

		// Verify both inserts succeeded
		selectResult, _ := db.SelectJSON(ctx, "items", SelectQuery{
			Where: []map[string]any{{"name": map[string]any{"like": "batch%"}}},
		}, false)
		rows := parseJSON(t, selectResult.Data)
		if len(rows) != 2 {
			t.Errorf("expected 2 rows inserted, got %d", len(rows))
		}
	})

	t.Run("batch rollback on error", func(t *testing.T) {
		// Insert a row first
		db.Client.Exec("INSERT INTO items (id, name, value) VALUES (999, 'rollback_test', 0)")

		// Batch with second operation failing (invalid column)
		_, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "update", Table: "items", Body: map[string]any{
					"data":  map[string]any{"value": 100},
					"where": []any{map[string]any{"id": map[string]any{"eq": 999}}},
				}},
				{Operation: "insert", Table: "items", Body: map[string]any{
					"data": map[string]any{"nonexistent_column": "fail"},
				}},
			},
		})
		if err == nil {
			t.Fatal("expected error for invalid column")
		}

		// Verify first operation was rolled back
		selectResult, _ := db.SelectJSON(ctx, "items", SelectQuery{
			Where: []map[string]any{{"id": map[string]any{"eq": 999}}},
		}, false)
		rows := parseJSON(t, selectResult.Data)
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		if rows[0]["value"].(float64) != 0 {
			t.Errorf("expected value 0 (rolled back), got %v", rows[0]["value"])
		}
	})

	t.Run("batch with select operation", func(t *testing.T) {
		db.Client.Exec("INSERT INTO items (name, value) VALUES ('select_test', 42)")

		result, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "select", Table: "items", Body: map[string]any{
					"select": []any{"name", "value"},
					"where":  []any{map[string]any{"name": map[string]any{"eq": "select_test"}}},
				}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Select returns array of rows
		rows, ok := result.Results[0].([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result.Results[0])
		}
		if len(rows) != 1 {
			t.Errorf("expected 1 row, got %d", len(rows))
		}
	})

	t.Run("batch with update operation", func(t *testing.T) {
		db.Client.Exec("INSERT INTO items (id, name, value) VALUES (888, 'update_test', 10)")

		result, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "update", Table: "items", Body: map[string]any{
					"data":  map[string]any{"value": 20},
					"where": []any{map[string]any{"id": map[string]any{"eq": 888}}},
				}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		r := result.Results[0].(map[string]any)
		if r["rows_affected"].(float64) != 1 {
			t.Errorf("expected 1 row affected, got %v", r["rows_affected"])
		}
	})

	t.Run("batch with delete operation", func(t *testing.T) {
		db.Client.Exec("INSERT INTO items (id, name, value) VALUES (777, 'delete_test', 10)")

		result, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "delete", Table: "items", Body: map[string]any{
					"where": []any{map[string]any{"id": map[string]any{"eq": 777}}},
				}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		r := result.Results[0].(map[string]any)
		if r["rows_affected"].(float64) != 1 {
			t.Errorf("expected 1 row affected, got %v", r["rows_affected"])
		}
	})

	t.Run("batch with upsert operation", func(t *testing.T) {
		result, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "upsert", Table: "items", Body: map[string]any{
					"data": []any{
						map[string]any{"id": 666, "name": "upsert1", "value": 1},
						map[string]any{"id": 667, "name": "upsert2", "value": 2},
					},
				}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		r := result.Results[0].(map[string]any)
		if r["rows_affected"].(float64) != 2 {
			t.Errorf("expected 2 rows affected, got %v", r["rows_affected"])
		}
	})

	t.Run("unknown operation errors", func(t *testing.T) {
		_, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "invalid_op", Table: "items", Body: map[string]any{}},
			},
		})
		if err == nil {
			t.Error("expected error for unknown operation")
		}
	})

	t.Run("non-existent table errors", func(t *testing.T) {
		_, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "select", Table: "nonexistent", Body: map[string]any{}},
			},
		})
		if !errors.Is(err, ErrTableNotFound) {
			t.Errorf("expected ErrTableNotFound, got %v", err)
		}
	})
}

// =============================================================================
// Batch Limit Tests
// =============================================================================

func TestBatchLimit(t *testing.T) {
	db := setupTestDB(t)
	setupSchema(t, db, schemaSimpleTable)
	ctx := context.Background()

	t.Run("batch at limit succeeds", func(t *testing.T) {
		ops := make([]BatchOperation, MaxBatchOperations)
		for i := range ops {
			ops[i] = BatchOperation{
				Operation: "insert",
				Table:     "items",
				Body:      map[string]any{"data": map[string]any{"name": "limit_test", "value": i}},
			}
		}

		result, err := db.Batch(ctx, BatchRequest{Operations: ops})
		if err != nil {
			t.Fatalf("batch at limit should succeed: %v", err)
		}
		if len(result.Results) != MaxBatchOperations {
			t.Errorf("expected %d results, got %d", MaxBatchOperations, len(result.Results))
		}
	})

	t.Run("batch exceeding limit errors", func(t *testing.T) {
		ops := make([]BatchOperation, MaxBatchOperations+1)
		for i := range ops {
			ops[i] = BatchOperation{
				Operation: "insert",
				Table:     "items",
				Body:      map[string]any{"data": map[string]any{"name": "over_limit", "value": i}},
			}
		}

		_, err := db.Batch(ctx, BatchRequest{Operations: ops})
		if !errors.Is(err, ErrBatchTooLarge) {
			t.Errorf("expected ErrBatchTooLarge, got %v", err)
		}
	})
}

// =============================================================================
// Batch Error Path Tests - Update/Delete require WHERE
// =============================================================================

func TestBatchRequiresWhere(t *testing.T) {
	db := setupTestDB(t)
	setupSchema(t, db, schemaSimpleTable)
	ctx := context.Background()

	t.Run("update without where errors", func(t *testing.T) {
		_, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "update", Table: "items", Body: map[string]any{
					"data": map[string]any{"value": 999},
				}},
			},
		})
		if !errors.Is(err, ErrMissingWhereClause) {
			t.Errorf("expected ErrMissingWhereClause, got %v", err)
		}
	})

	t.Run("delete without where errors", func(t *testing.T) {
		_, err := db.Batch(ctx, BatchRequest{
			Operations: []BatchOperation{
				{Operation: "delete", Table: "items", Body: map[string]any{}},
			},
		})
		if !errors.Is(err, ErrMissingWhereClause) {
			t.Errorf("expected ErrMissingWhereClause, got %v", err)
		}
	})
}
