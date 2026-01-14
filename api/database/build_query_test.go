package database

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/joe-ervin05/atomicbase/config"
)

// =============================================================================
// Test Schema Constants
// =============================================================================

// schemaPostsUsers: Two tables with FK relationship (posts -> users).
var schemaPostsUsers = SchemaCache{
	Tables: map[string]Table{
		"posts": {
			Name: "posts",
			Pk:   "id",
			Columns: map[string]Col{
				"author_id": {Name: "author_id", Type: ColTypeInteger},
				"body":      {Name: "body", Type: ColTypeText},
				"id":        {Name: "id", Type: ColTypeInteger},
				"title":     {Name: "title", Type: ColTypeText},
			},
		},
		"users": {
			Name: "users",
			Pk:   "id",
			Columns: map[string]Col{
				"email": {Name: "email", Type: ColTypeText},
				"id":    {Name: "id", Type: ColTypeInteger},
				"name":  {Name: "name", Type: ColTypeText},
			},
		},
	},
	Fks: map[string][]Fk{
		"posts": {{Table: "posts", References: "users", From: "author_id", To: "id"}},
	},
}

// schemaThreeTables: Three tables for testing nested joins (comments -> posts -> users).
var schemaThreeTables = SchemaCache{
	Tables: map[string]Table{
		"comments": {
			Name: "comments",
			Pk:   "id",
			Columns: map[string]Col{
				"body":    {Name: "body", Type: ColTypeText},
				"id":      {Name: "id", Type: ColTypeInteger},
				"post_id": {Name: "post_id", Type: ColTypeInteger},
			},
		},
		"posts": {
			Name: "posts",
			Pk:   "id",
			Columns: map[string]Col{
				"author_id": {Name: "author_id", Type: ColTypeInteger},
				"id":        {Name: "id", Type: ColTypeInteger},
				"title":     {Name: "title", Type: ColTypeText},
			},
		},
		"users": {
			Name: "users",
			Pk:   "id",
			Columns: map[string]Col{
				"id":   {Name: "id", Type: ColTypeInteger},
				"name": {Name: "name", Type: ColTypeText},
			},
		},
	},
	Fks: map[string][]Fk{
		"comments": {{Table: "comments", References: "posts", From: "post_id", To: "id"}},
		"posts":    {{Table: "posts", References: "users", From: "author_id", To: "id"}},
	},
}

// schemaNoFks: Two tables without foreign key relationships.
var schemaNoFks = SchemaCache{
	Tables: map[string]Table{
		"posts": {Name: "posts", Pk: "id", Columns: map[string]Col{"id": {Name: "id", Type: ColTypeInteger}}},
		"users": {Name: "users", Pk: "id", Columns: map[string]Col{"id": {Name: "id", Type: ColTypeInteger}}},
	},
	Fks: map[string][]Fk{},
}

// schemaWithBlob: Table containing BLOB column (should be excluded from JSON output).
var schemaWithBlob = SchemaCache{
	Tables: map[string]Table{
		"files": {
			Name: "files",
			Pk:   "id",
			Columns: map[string]Col{
				"data": {Name: "data", Type: ColTypeBlob},
				"id":   {Name: "id", Type: ColTypeInteger},
				"name": {Name: "name", Type: ColTypeText},
			},
		},
	},
}

// =============================================================================
// relationDepth Tests
// Edge cases: nil input, empty joins, single level, multi-level, sibling depth
// =============================================================================

func TestRelationDepth(t *testing.T) {
	tests := []struct {
		name string
		rel  *Relation
		want int
	}{
		{"nil relation", nil, 1},
		{"no joins", &Relation{name: "users"}, 1},
		{"one level", &Relation{name: "users", joins: []*Relation{{name: "posts"}}}, 2},
		{
			"two levels deep",
			&Relation{
				name: "users",
				joins: []*Relation{
					{name: "posts", joins: []*Relation{{name: "comments"}}},
				},
			},
			3,
		},
		{
			// Tests that max depth is taken among siblings, not sum
			"sibling depth takes max",
			&Relation{
				name: "users",
				joins: []*Relation{
					{name: "posts"}, // depth 1
					{name: "comments", joins: []*Relation{{name: "likes"}}}, // depth 2
				},
			},
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := relationDepth(tt.rel); got != tt.want {
				t.Errorf("relationDepth() = %d, want %d", got, tt.want)
			}
		})
	}
}

// =============================================================================
// parseSelect Tests
// Edge cases: aliases, nested relations, inner joins, quotes, escapes, multiple joins
// =============================================================================

func TestParseSelect(t *testing.T) {
	t.Run("column with alias uses colon syntax", func(t *testing.T) {
		// alias:column - alias comes first
		rel := parseSelect("user_name:name", "users")
		if len(rel.columns) != 1 {
			t.Fatalf("expected 1 column, got %d", len(rel.columns))
		}
		if rel.columns[0].name != "name" || rel.columns[0].alias != "user_name" {
			t.Errorf("got alias=%q name=%q, want alias='user_name' name='name'",
				rel.columns[0].alias, rel.columns[0].name)
		}
	})

	t.Run("nested relation creates join", func(t *testing.T) {
		rel := parseSelect("id,posts(title,body)", "users")
		if len(rel.joins) != 1 || rel.joins[0].name != "posts" {
			t.Fatalf("expected join on 'posts'")
		}
		if len(rel.joins[0].columns) != 2 {
			t.Errorf("expected 2 columns in posts, got %d", len(rel.joins[0].columns))
		}
	})

	t.Run("! before paren marks inner join", func(t *testing.T) {
		// The ! must come before ( to set inner=true before Relation is created
		rel := parseSelect("posts!(title)", "users")
		if len(rel.joins) != 1 {
			t.Fatalf("expected 1 join")
		}
		if !rel.joins[0].inner {
			t.Error("expected inner join")
		}
	})

	t.Run("without ! defaults to left join", func(t *testing.T) {
		rel := parseSelect("posts(title)", "users")
		if rel.joins[0].inner {
			t.Error("expected left join (inner=false)")
		}
	})

	t.Run("deeply nested relations", func(t *testing.T) {
		// users -> posts -> comments -> likes
		rel := parseSelect("posts(comments(likes(user_id)))", "users")
		posts := rel.joins[0]
		comments := posts.joins[0]
		likes := comments.joins[0]
		if likes.name != "likes" || likes.columns[0].name != "user_id" {
			t.Error("expected likes with user_id column at depth 3")
		}
	})

	t.Run("quoted identifier preserves special chars", func(t *testing.T) {
		rel := parseSelect(`"column with spaces"`, "users")
		if rel.columns[0].name != "column with spaces" {
			t.Errorf("quoted name not preserved: %q", rel.columns[0].name)
		}
	})

	t.Run("backslash escapes delimiter", func(t *testing.T) {
		rel := parseSelect(`col\,name`, "users")
		if rel.columns[0].name != "col,name" {
			t.Errorf("escaped comma not preserved: %q", rel.columns[0].name)
		}
	})

	t.Run("multiple joins at same level", func(t *testing.T) {
		rel := parseSelect("posts(title),comments(body)", "users")
		if len(rel.joins) != 2 {
			t.Fatalf("expected 2 joins, got %d", len(rel.joins))
		}
		if rel.joins[0].name != "posts" || rel.joins[1].name != "comments" {
			t.Error("expected posts and comments joins")
		}
	})

	t.Run("relation with alias", func(t *testing.T) {
		rel := parseSelect("user_posts:posts(title)", "users")
		if rel.joins[0].alias != "user_posts" || rel.joins[0].name != "posts" {
			t.Errorf("got alias=%q name=%q", rel.joins[0].alias, rel.joins[0].name)
		}
	})
}

// =============================================================================
// findForeignKey Tests
// Binary search for FK relationships - criterion A (unlikely to change)
// =============================================================================

func TestFindForeignKey(t *testing.T) {
	schema := SchemaCache{
		Fks: map[string][]Fk{
			"comments": {{Table: "comments", References: "posts", From: "post_id", To: "id"}},
			"posts":    {{Table: "posts", References: "users", From: "author_id", To: "id"}},
		},
	}

	t.Run("finds existing FK", func(t *testing.T) {
		fk := schema.findForeignKey("posts", "users")
		if fk.From != "author_id" || fk.To != "id" {
			t.Errorf("expected posts.author_id -> users.id, got %+v", fk)
		}
	})

	t.Run("returns empty for nonexistent", func(t *testing.T) {
		fk := schema.findForeignKey("nonexistent", "users")
		if fk != (Fk{}) {
			t.Errorf("expected empty FK, got %+v", fk)
		}
	})

	t.Run("direction matters", func(t *testing.T) {
		// FK is posts->users, not users->posts
		fk := schema.findForeignKey("users", "posts")
		if fk != (Fk{}) {
			t.Errorf("reverse direction should return empty, got %+v", fk)
		}
	})
}

// =============================================================================
// buildSelect Tests
// Complex context: JOIN generation, GROUP BY, JSON aggregation
// =============================================================================

func TestBuildSelect(t *testing.T) {
	t.Run("join generates LEFT JOIN and GROUP BY", func(t *testing.T) {
		rel := Relation{
			name:  "users",
			joins: []*Relation{{name: "posts"}},
		}
		rel.joins[0].parent = &rel

		query, agg, err := schemaPostsUsers.buildSelect(rel)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(query, "LEFT JOIN") {
			t.Error("expected LEFT JOIN in query")
		}
		if !strings.Contains(query, "GROUP BY") {
			t.Error("expected GROUP BY when joining")
		}
		if !strings.Contains(agg, "'posts'") {
			t.Error("expected 'posts' in aggregation")
		}
	})

	t.Run("inner join uses INNER JOIN", func(t *testing.T) {
		rel := Relation{
			name:  "users",
			joins: []*Relation{{name: "posts", inner: true}},
		}
		rel.joins[0].parent = &rel

		query, _, err := schemaPostsUsers.buildSelect(rel)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(query, "INNER JOIN") {
			t.Error("expected INNER JOIN")
		}
	})

	t.Run("column alias appears in aggregation", func(t *testing.T) {
		rel := Relation{
			name:    "users",
			columns: []column{{name: "name", alias: "user_name"}},
		}
		_, agg, err := schemaPostsUsers.buildSelect(rel)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(agg, "'user_name'") {
			t.Errorf("alias not in aggregation: %s", agg)
		}
	})

	t.Run("blob columns excluded", func(t *testing.T) {
		rel := Relation{name: "files"}
		_, agg, err := schemaWithBlob.buildSelect(rel)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(agg, "'data'") {
			t.Error("BLOB column should be excluded from aggregation")
		}
	})

	t.Run("missing FK relationship errors", func(t *testing.T) {
		rel := Relation{
			name:  "users",
			joins: []*Relation{{name: "posts"}},
		}
		rel.joins[0].parent = &rel

		_, _, err := schemaNoFks.buildSelect(rel)
		if !errors.Is(err, ErrNoRelationship) {
			t.Errorf("expected ErrNoRelationship, got %v", err)
		}
	})

	t.Run("invalid table errors", func(t *testing.T) {
		rel := Relation{name: "nonexistent"}
		_, _, err := schemaPostsUsers.buildSelect(rel)
		if !errors.Is(err, ErrTableNotFound) {
			t.Errorf("expected ErrTableNotFound, got %v", err)
		}
	})

	t.Run("invalid column errors", func(t *testing.T) {
		rel := Relation{
			name:    "users",
			columns: []column{{name: "nonexistent"}},
		}
		_, _, err := schemaPostsUsers.buildSelect(rel)
		if !errors.Is(err, ErrColumnNotFound) {
			t.Errorf("expected ErrColumnNotFound, got %v", err)
		}
	})

	t.Run("depth limit enforced", func(t *testing.T) {
		origDepth := config.Cfg.MaxQueryDepth
		config.Cfg.MaxQueryDepth = 2
		defer func() { config.Cfg.MaxQueryDepth = origDepth }()

		// Depth of 3 exceeds limit of 2
		rel := Relation{
			name: "users",
			joins: []*Relation{
				{name: "posts", joins: []*Relation{{name: "posts"}}},
			},
		}
		rel.joins[0].parent = &rel
		rel.joins[0].joins[0].parent = rel.joins[0]

		_, _, err := schemaPostsUsers.buildSelect(rel)
		if !errors.Is(err, ErrQueryTooDeep) {
			t.Errorf("expected ErrQueryTooDeep, got %v", err)
		}
	})

	t.Run("many columns uses json_patch", func(t *testing.T) {
		// Create a table with more columns than MaxSelectColumns
		cols := make(map[string]Col)
		for i := 0; i < MaxSelectColumns+10; i++ {
			name := fmt.Sprintf("col%d", i)
			cols[name] = Col{Name: name, Type: ColTypeText}
		}
		cols["id"] = Col{Name: "id", Type: ColTypeInteger}

		schemaMany := SchemaCache{
			Tables: map[string]Table{
				"wide": {Name: "wide", Pk: "id", Columns: cols},
			},
		}

		// SELECT * should expand to all columns and use json_patch
		rel := Relation{name: "wide", columns: []column{{"*", ""}}}
		_, agg, err := schemaMany.buildSelect(rel)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should use json_patch to combine multiple json_object chunks
		if !strings.Contains(agg, "json_patch") {
			t.Errorf("expected json_patch for many columns, got: %s", agg)
		}
	})
}

// =============================================================================
// buildSelCurr Tests
// Complex context: Nested query generation with FK column inclusion
// =============================================================================

func TestBuildSelCurr(t *testing.T) {
	t.Run("includes FK column for parent join", func(t *testing.T) {
		rel := Relation{name: "posts"}
		query, _, err := schemaThreeTables.buildSelCurr(rel, "users")
		if err != nil {
			t.Fatal(err)
		}
		// Should include author_id (FK to users) even if not explicitly selected
		if !strings.Contains(query, "[posts].[author_id]") {
			t.Errorf("FK column not included: %s", query)
		}
	})

	t.Run("nested join within nested query", func(t *testing.T) {
		rel := Relation{
			name:  "posts",
			joins: []*Relation{{name: "comments"}},
		}
		rel.joins[0].parent = &rel

		query, agg, err := schemaThreeTables.buildSelCurr(rel, "users")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(query, "LEFT JOIN") {
			t.Error("expected LEFT JOIN for nested relation")
		}
		if !strings.Contains(agg, "'comments'") {
			t.Error("expected 'comments' in aggregation")
		}
	})

	t.Run("column alias in nested query", func(t *testing.T) {
		rel := Relation{
			name:    "posts",
			columns: []column{{name: "title", alias: "post_title"}},
		}
		_, agg, err := schemaThreeTables.buildSelCurr(rel, "users")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(agg, "'post_title'") {
			t.Errorf("alias not in aggregation: %s", agg)
		}
	})
}
