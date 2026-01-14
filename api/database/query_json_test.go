package database

import (
	"errors"
	"testing"
)

// =============================================================================
// Test Table Definitions
// =============================================================================

// tableUsers: Basic users table with common column types
var tableUsers = Table{
	Name: "users",
	Pk:   "id",
	Columns: map[string]Col{
		"age":    {Name: "age", Type: ColTypeInteger, NotNull: false},
		"email":  {Name: "email", Type: ColTypeText, NotNull: false},
		"id":     {Name: "id", Type: ColTypeInteger, NotNull: true},
		"name":   {Name: "name", Type: ColTypeText, NotNull: true},
		"status": {Name: "status", Type: ColTypeText, NotNull: false},
	},
}

// tablePosts: Posts table for testing FTS
var tablePosts = Table{
	Name: "posts",
	Pk:   "id",
	Columns: map[string]Col{
		"body":  {Name: "body", Type: ColTypeText, NotNull: false},
		"id":    {Name: "id", Type: ColTypeInteger, NotNull: true},
		"title": {Name: "title", Type: ColTypeText, NotNull: true},
	},
}

// schemaWithFTS: Schema cache with FTS enabled on posts table
var schemaWithFTS = SchemaCache{
	Tables:    map[string]Table{"posts": tablePosts, "users": tableUsers},
	FTSTables: map[string]bool{"posts": true},
}

// schemaWithoutFTS: Schema cache without FTS
var schemaWithoutFTS = SchemaCache{
	Tables:    map[string]Table{"posts": tablePosts, "users": tableUsers},
	FTSTables: map[string]bool{},
}

// =============================================================================
// BuildWhereFromJSON Tests - Distinct code paths and edge cases
// =============================================================================

func TestBuildWhereFromJSON(t *testing.T) {
	tests := []struct {
		name        string
		table       Table
		where       []map[string]any
		schema      SchemaCache
		wantQuery   string
		wantArgs    int
		wantErr     bool
		errContains string
	}{
		// Empty case
		{
			name:      "empty where",
			table:     tableUsers,
			where:     []map[string]any{},
			schema:    schemaWithoutFTS,
			wantQuery: "",
			wantArgs:  0,
		},

		// eq - representative of all comparison operators (neq/gt/gte/lt/lte use same code path)
		{
			name:      "eq operator",
			table:     tableUsers,
			where:     []map[string]any{{"id": map[string]any{"eq": 5}}},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[id] = ? ",
			wantArgs:  1,
		},

		// like - representative of pattern matching (glob uses same code path)
		{
			name:      "like operator",
			table:     tableUsers,
			where:     []map[string]any{{"name": map[string]any{"like": "%john%"}}},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[name] LIKE ? ",
			wantArgs:  1,
		},

		// IS operator - two branches: nil vs non-nil
		{
			name:      "is null - nil branch",
			table:     tableUsers,
			where:     []map[string]any{{"email": map[string]any{"is": nil}}},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[email] IS NULL ",
			wantArgs:  0,
		},
		{
			name:      "is true - non-nil branch, literal in query",
			table:     tableUsers,
			where:     []map[string]any{{"status": map[string]any{"is": true}}},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[status] IS true ",
			wantArgs:  0,
		},

		// Array operators - distinct handling with placeholder generation
		{
			name:      "in operator - dynamic placeholder count",
			table:     tableUsers,
			where:     []map[string]any{{"id": map[string]any{"in": []any{1, 2, 3}}}},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[id] IN (?, ?, ?) ",
			wantArgs:  3,
		},
		{
			name:      "between operator - exactly 2 args validation",
			table:     tableUsers,
			where:     []map[string]any{{"age": map[string]any{"between": []any{18, 65}}}},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[age] BETWEEN ? AND ? ",
			wantArgs:  2,
		},

		// NOT wrapper - distinct behaviors for different operators
		{
			name:      "not in - generates NOT IN",
			table:     tableUsers,
			where:     []map[string]any{{"id": map[string]any{"not": map[string]any{"in": []any{1, 2}}}}},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[id] NOT IN (?, ?) ",
			wantArgs:  2,
		},
		{
			name:      "not is null - generates IS NOT NULL",
			table:     tableUsers,
			where:     []map[string]any{{"email": map[string]any{"not": map[string]any{"is": nil}}}},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[email] IS NOT NULL ",
			wantArgs:  0,
		},
		{
			name:      "not like - generates NOT LIKE",
			table:     tableUsers,
			where:     []map[string]any{{"name": map[string]any{"not": map[string]any{"like": "%test%"}}}},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[name] NOT LIKE ? ",
			wantArgs:  1,
		},

		// Logical operators
		{
			name:  "or conditions - builds OR clause",
			table: tableUsers,
			where: []map[string]any{
				{"or": []any{
					map[string]any{"status": map[string]any{"eq": "active"}},
					map[string]any{"status": map[string]any{"eq": "pending"}},
				}},
			},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE ([users].[status] = ? OR [users].[status] = ?) ",
			wantArgs:  2,
		},
		{
			name:  "multiple conditions - joins with AND",
			table: tableUsers,
			where: []map[string]any{
				{"age": map[string]any{"gte": 18}},
				{"status": map[string]any{"eq": "active"}},
			},
			schema:    schemaWithoutFTS,
			wantQuery: "WHERE [users].[age] >= ? AND [users].[status] = ? ",
			wantArgs:  2,
		},

		// FTS - requires schema lookup + generates subquery
		{
			name:      "fts with index - generates MATCH subquery",
			table:     tablePosts,
			where:     []map[string]any{{"title": map[string]any{"fts": "search term"}}},
			schema:    schemaWithFTS,
			wantQuery: "WHERE rowid IN (SELECT rowid FROM [posts_fts] WHERE [posts_fts] MATCH ?) ",
			wantArgs:  1,
		},
		{
			name:        "fts without index errors",
			table:       tableUsers,
			where:       []map[string]any{{"name": map[string]any{"fts": "search"}}},
			schema:      schemaWithoutFTS,
			wantErr:     true,
			errContains: "no FTS index",
		},

		// Error cases - validation paths
		{
			name:        "invalid column",
			table:       tableUsers,
			where:       []map[string]any{{"nonexistent": map[string]any{"eq": "value"}}},
			schema:      schemaWithoutFTS,
			wantErr:     true,
			errContains: "column not found",
		},
		{
			name:        "invalid operator",
			table:       tableUsers,
			where:       []map[string]any{{"name": map[string]any{"invalid": "value"}}},
			schema:      schemaWithoutFTS,
			wantErr:     true,
			errContains: "invalid filter operator",
		},
		{
			name:        "invalid not operator - not.gt unsupported",
			table:       tableUsers,
			where:       []map[string]any{{"age": map[string]any{"not": map[string]any{"gt": 10}}}},
			schema:      schemaWithoutFTS,
			wantErr:     true,
			errContains: "invalid filter operator",
		},
		{
			name:        "between wrong length",
			table:       tableUsers,
			where:       []map[string]any{{"age": map[string]any{"between": []any{18}}}},
			schema:      schemaWithoutFTS,
			wantErr:     true,
			errContains: "exactly 2 elements",
		},
		{
			name:        "in requires array",
			table:       tableUsers,
			where:       []map[string]any{{"id": map[string]any{"in": "not-an-array"}}},
			schema:      schemaWithoutFTS,
			wantErr:     true,
			errContains: "must be an array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args, err := tt.table.BuildWhereFromJSON(tt.where, tt.schema)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if query != tt.wantQuery {
				t.Errorf("query mismatch:\n  got:  %q\n  want: %q", query, tt.wantQuery)
			}
			if len(args) != tt.wantArgs {
				t.Errorf("args count: got %d, want %d", len(args), tt.wantArgs)
			}
		})
	}
}

// =============================================================================
// BuildOrderFromJSON Tests - ORDER BY clause generation
// =============================================================================

func TestBuildOrderFromJSON(t *testing.T) {
	tests := []struct {
		name      string
		order     map[string]string
		wantQuery string
		wantErr   bool
	}{
		{
			name:      "empty order",
			order:     map[string]string{},
			wantQuery: "",
		},
		{
			name:      "single column asc",
			order:     map[string]string{"name": "asc"},
			wantQuery: "ORDER BY [users].[name] ASC ",
		},
		{
			name:      "single column desc",
			order:     map[string]string{"age": "desc"},
			wantQuery: "ORDER BY [users].[age] DESC ",
		},
		{
			name:      "case insensitive direction",
			order:     map[string]string{"name": "DESC"},
			wantQuery: "ORDER BY [users].[name] DESC ",
		},
		{
			name:    "invalid column errors",
			order:   map[string]string{"nonexistent": "asc"},
			wantErr: true,
		},
		{
			name:    "invalid direction errors",
			order:   map[string]string{"name": "sideways"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := tableUsers.BuildOrderFromJSON(tt.order)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if query != tt.wantQuery {
				t.Errorf("query mismatch:\n  got:  %q\n  want: %q", query, tt.wantQuery)
			}
		})
	}
}

// =============================================================================
// ParseSelectFromJSON Tests - Complex context (nested relations)
// =============================================================================

func TestParseSelectFromJSON(t *testing.T) {
	tests := []struct {
		name      string
		sel       []any
		tableName string
		wantCols  int
		wantJoins int
		checkFunc func(*testing.T, Relation)
		wantErr   bool
	}{
		{
			name:      "empty defaults to star",
			sel:       []any{},
			tableName: "users",
			wantCols:  1,
			checkFunc: func(t *testing.T, rel Relation) {
				if rel.columns[0].name != "*" {
					t.Errorf("expected '*', got %q", rel.columns[0].name)
				}
			},
		},
		{
			name:      "nested relation",
			sel:       []any{"id", map[string]any{"posts": []any{"title"}}},
			tableName: "users",
			wantCols:  1,
			wantJoins: 1,
			checkFunc: func(t *testing.T, rel Relation) {
				if rel.joins[0].name != "posts" {
					t.Errorf("expected join 'posts', got %q", rel.joins[0].name)
				}
			},
		},
		{
			name:      "deeply nested",
			sel:       []any{"id", map[string]any{"posts": []any{"title", map[string]any{"comments": []any{"body"}}}}},
			tableName: "users",
			wantCols:  1,
			wantJoins: 1,
			checkFunc: func(t *testing.T, rel Relation) {
				if len(rel.joins[0].joins) != 1 {
					t.Errorf("expected nested join, got %d", len(rel.joins[0].joins))
				}
			},
		},
		{
			name:      "aliased column",
			sel:       []any{map[string]any{"fullName": "name"}},
			tableName: "users",
			wantCols:  1,
			checkFunc: func(t *testing.T, rel Relation) {
				if rel.columns[0].alias != "fullName" {
					t.Errorf("expected alias 'fullName', got %q", rel.columns[0].alias)
				}
			},
		},
		{
			name:      "invalid type errors",
			sel:       []any{123},
			tableName: "users",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel, err := ParseSelectFromJSON(tt.sel, tt.tableName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(rel.columns) != tt.wantCols {
				t.Errorf("columns: got %d, want %d", len(rel.columns), tt.wantCols)
			}
			if len(rel.joins) != tt.wantJoins {
				t.Errorf("joins: got %d, want %d", len(rel.joins), tt.wantJoins)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, rel)
			}
		})
	}
}

// =============================================================================
// Error Sentinel Tests
// =============================================================================

func TestQueryJSONErrorTypes(t *testing.T) {
	t.Run("ErrInvalidOperator", func(t *testing.T) {
		where := []map[string]any{{"name": map[string]any{"unknown": "value"}}}
		_, _, err := tableUsers.BuildWhereFromJSON(where, schemaWithoutFTS)
		if !errors.Is(err, ErrInvalidOperator) {
			t.Errorf("expected ErrInvalidOperator, got %v", err)
		}
	})

	t.Run("ErrColumnNotFound", func(t *testing.T) {
		where := []map[string]any{{"missing": map[string]any{"eq": "value"}}}
		_, _, err := tableUsers.BuildWhereFromJSON(where, schemaWithoutFTS)
		if !errors.Is(err, ErrColumnNotFound) {
			t.Errorf("expected ErrColumnNotFound, got %v", err)
		}
	})

	t.Run("ErrNoFTSIndex", func(t *testing.T) {
		where := []map[string]any{{"name": map[string]any{"fts": "search"}}}
		_, _, err := tableUsers.BuildWhereFromJSON(where, schemaWithoutFTS)
		if !errors.Is(err, ErrNoFTSIndex) {
			t.Errorf("expected ErrNoFTSIndex, got %v", err)
		}
	})
}
