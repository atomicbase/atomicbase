package data

import (
	"strings"
	"testing"
)

// =============================================================================
// Test Fixtures
// =============================================================================

// Test table with various column types for query tests
var usersTable = Table{
	Name: "users",
	Pk:   []string{"id"},
	Columns: map[string]Col{
		"id":         {Name: "id", Type: "INTEGER", NotNull: true},
		"name":       {Name: "name", Type: "TEXT", NotNull: false},
		"email":      {Name: "email", Type: "TEXT", NotNull: true},
		"age":        {Name: "age", Type: "INTEGER", NotNull: false},
		"score":      {Name: "score", Type: "REAL", NotNull: false},
		"created_at": {Name: "created_at", Type: "TEXT", NotNull: false},
		"is_active":  {Name: "is_active", Type: "INTEGER", NotNull: false},
	},
}

// Test schema with tables and FKs
var testSchema = SchemaCache{
	Tables: map[string]Table{
		"users": usersTable,
		"posts": {
			Name: "posts",
			Pk:   []string{"id"},
			Columns: map[string]Col{
				"id":      {Name: "id", Type: "INTEGER", NotNull: true},
				"user_id": {Name: "user_id", Type: "INTEGER", References: "users.id"},
				"title":   {Name: "title", Type: "TEXT", NotNull: true},
			},
		},
	},
	Fks: map[string][]Fk{
		"posts": {{Table: "posts", References: "users", From: "user_id", To: "id"}},
	},
	FTSTables: map[string]bool{
		"users": true,
	},
}

// =============================================================================
// BuildWhereFromJSON Tests - Many edge cases (criteria B)
// =============================================================================

func TestBuildWhereFromJSON_Empty(t *testing.T) {
	query, args, err := usersTable.BuildWhereFromJSON(nil, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if query != "" {
		t.Errorf("expected empty query, got %q", query)
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %v", args)
	}
}

func TestBuildWhereFromJSON_Eq(t *testing.T) {
	where := []map[string]any{
		{"id": map[string]any{"eq": 5}},
	}
	query, args, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "[users].[id] = ?") {
		t.Errorf("expected eq clause, got %q", query)
	}
	if len(args) != 1 || args[0] != 5 {
		t.Errorf("expected args [5], got %v", args)
	}
}

func TestBuildWhereFromJSON_Neq(t *testing.T) {
	where := []map[string]any{
		{"name": map[string]any{"neq": "admin"}},
	}
	query, args, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "[users].[name] != ?") {
		t.Errorf("expected neq clause, got %q", query)
	}
	if len(args) != 1 || args[0] != "admin" {
		t.Errorf("expected args [admin], got %v", args)
	}
}

func TestBuildWhereFromJSON_GtGteLtLte(t *testing.T) {
	tests := []struct {
		op       string
		expected string
	}{
		{"gt", ">"},
		{"gte", ">="},
		{"lt", "<"},
		{"lte", "<="},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			where := []map[string]any{
				{"age": map[string]any{tt.op: 18}},
			}
			query, _, err := usersTable.BuildWhereFromJSON(where, testSchema)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(query, tt.expected) {
				t.Errorf("expected %s operator, got %q", tt.expected, query)
			}
		})
	}
}

func TestBuildWhereFromJSON_Like(t *testing.T) {
	where := []map[string]any{
		{"name": map[string]any{"like": "%john%"}},
	}
	query, args, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "LIKE ?") {
		t.Errorf("expected LIKE clause, got %q", query)
	}
	if len(args) != 1 || args[0] != "%john%" {
		t.Errorf("expected args [%%john%%], got %v", args)
	}
}

func TestBuildWhereFromJSON_Glob(t *testing.T) {
	where := []map[string]any{
		{"name": map[string]any{"glob": "*john*"}},
	}
	query, _, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "GLOB ?") {
		t.Errorf("expected GLOB clause, got %q", query)
	}
}

func TestBuildWhereFromJSON_In(t *testing.T) {
	where := []map[string]any{
		{"id": map[string]any{"in": []any{1, 2, 3}}},
	}
	query, args, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "IN (?, ?, ?)") {
		t.Errorf("expected IN clause with 3 placeholders, got %q", query)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestBuildWhereFromJSON_InEmpty(t *testing.T) {
	where := []map[string]any{
		{"id": map[string]any{"in": []any{}}},
	}
	_, _, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err == nil {
		t.Error("expected error for empty IN array")
	}
}

func TestBuildWhereFromJSON_Between(t *testing.T) {
	where := []map[string]any{
		{"age": map[string]any{"between": []any{18, 65}}},
	}
	query, args, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "BETWEEN ? AND ?") {
		t.Errorf("expected BETWEEN clause, got %q", query)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildWhereFromJSON_BetweenInvalid(t *testing.T) {
	where := []map[string]any{
		{"age": map[string]any{"between": []any{18}}}, // Only 1 element
	}
	_, _, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err == nil {
		t.Error("expected error for invalid BETWEEN array")
	}
}

func TestBuildWhereFromJSON_IsNull(t *testing.T) {
	where := []map[string]any{
		{"name": map[string]any{"is": nil}},
	}
	query, args, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "IS NULL") {
		t.Errorf("expected IS NULL clause, got %q", query)
	}
	if len(args) != 0 {
		t.Errorf("expected no args for IS NULL, got %v", args)
	}
}

func TestBuildWhereFromJSON_NotEq(t *testing.T) {
	where := []map[string]any{
		{"name": map[string]any{"not": map[string]any{"eq": "admin"}}},
	}
	query, args, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "!= ?") {
		t.Errorf("expected NOT eq clause, got %q", query)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestBuildWhereFromJSON_NotIn(t *testing.T) {
	where := []map[string]any{
		{"id": map[string]any{"not": map[string]any{"in": []any{1, 2}}}},
	}
	query, _, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "NOT IN") {
		t.Errorf("expected NOT IN clause, got %q", query)
	}
}

func TestBuildWhereFromJSON_NotIsNull(t *testing.T) {
	where := []map[string]any{
		{"name": map[string]any{"not": map[string]any{"is": nil}}},
	}
	query, _, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "IS NOT NULL") {
		t.Errorf("expected IS NOT NULL clause, got %q", query)
	}
}

func TestBuildWhereFromJSON_Or(t *testing.T) {
	where := []map[string]any{
		{"or": []any{
			map[string]any{"name": map[string]any{"eq": "john"}},
			map[string]any{"name": map[string]any{"eq": "jane"}},
		}},
	}
	query, args, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, " OR ") {
		t.Errorf("expected OR clause, got %q", query)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildWhereFromJSON_MultipleConditions(t *testing.T) {
	where := []map[string]any{
		{"age": map[string]any{"gte": 18}},
		{"is_active": map[string]any{"eq": 1}},
	}
	query, args, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "AND") {
		t.Errorf("expected AND between conditions, got %q", query)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildWhereFromJSON_InvalidColumn(t *testing.T) {
	where := []map[string]any{
		{"nonexistent": map[string]any{"eq": 5}},
	}
	_, _, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err == nil {
		t.Error("expected error for invalid column")
	}
}

func TestBuildWhereFromJSON_InvalidOperator(t *testing.T) {
	where := []map[string]any{
		{"id": map[string]any{"invalid_op": 5}},
	}
	_, _, err := usersTable.BuildWhereFromJSON(where, testSchema)
	if err == nil {
		t.Error("expected error for invalid operator")
	}
}

// =============================================================================
// BuildOrderFromJSON Tests
// =============================================================================

func TestBuildOrderFromJSON_Empty(t *testing.T) {
	query, err := usersTable.BuildOrderFromJSON(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if query != "" {
		t.Errorf("expected empty query, got %q", query)
	}
}

func TestBuildOrderFromJSON_Asc(t *testing.T) {
	order := map[string]string{"name": "asc"}
	query, err := usersTable.BuildOrderFromJSON(order)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "ASC") {
		t.Errorf("expected ASC, got %q", query)
	}
}

func TestBuildOrderFromJSON_Desc(t *testing.T) {
	order := map[string]string{"created_at": "desc"}
	query, err := usersTable.BuildOrderFromJSON(order)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "DESC") {
		t.Errorf("expected DESC, got %q", query)
	}
}

func TestBuildOrderFromJSON_InvalidColumn(t *testing.T) {
	order := map[string]string{"nonexistent": "asc"}
	_, err := usersTable.BuildOrderFromJSON(order)
	if err == nil {
		t.Error("expected error for invalid column")
	}
}

func TestBuildOrderFromJSON_InvalidDirection(t *testing.T) {
	order := map[string]string{"name": "invalid"}
	_, err := usersTable.BuildOrderFromJSON(order)
	if err == nil {
		t.Error("expected error for invalid direction")
	}
}

// =============================================================================
// ParseSelectFromJSON Tests
// =============================================================================

func TestParseSelectFromJSON_Empty(t *testing.T) {
	rel, err := ParseSelectFromJSON(nil, "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rel.columns) != 1 || rel.columns[0].name != "*" {
		t.Errorf("expected default *, got %v", rel.columns)
	}
}

func TestParseSelectFromJSON_SimpleColumns(t *testing.T) {
	sel := []any{"id", "name", "email"}
	rel, err := ParseSelectFromJSON(sel, "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rel.columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(rel.columns))
	}
}

func TestParseSelectFromJSON_Star(t *testing.T) {
	sel := []any{"*"}
	rel, err := ParseSelectFromJSON(sel, "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rel.columns) != 1 || rel.columns[0].name != "*" {
		t.Errorf("expected *, got %v", rel.columns)
	}
}

func TestParseSelectFromJSON_AliasedColumn(t *testing.T) {
	sel := []any{map[string]any{"user_name": "name"}}
	rel, err := ParseSelectFromJSON(sel, "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rel.columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(rel.columns))
	}
	if rel.columns[0].name != "name" || rel.columns[0].alias != "user_name" {
		t.Errorf("expected aliased column, got %+v", rel.columns[0])
	}
}

func TestParseSelectFromJSON_NestedRelation(t *testing.T) {
	sel := []any{"id", map[string]any{"posts": []any{"title"}}}
	rel, err := ParseSelectFromJSON(sel, "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rel.columns) != 1 {
		t.Errorf("expected 1 direct column, got %d", len(rel.columns))
	}
	if len(rel.joins) != 1 {
		t.Fatalf("expected 1 join, got %d", len(rel.joins))
	}
	if rel.joins[0].name != "posts" {
		t.Errorf("expected posts join, got %s", rel.joins[0].name)
	}
}

// =============================================================================
// Schema Lookup Tests - Stable functions (criteria A)
// =============================================================================

func TestSearchTbls_Found(t *testing.T) {
	tbl, err := testSchema.SearchTbls("users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tbl.Name != "users" {
		t.Errorf("expected users, got %s", tbl.Name)
	}
}

func TestSearchTbls_NotFound(t *testing.T) {
	_, err := testSchema.SearchTbls("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent table")
	}
}

func TestSearchCols_Found(t *testing.T) {
	col, err := usersTable.SearchCols("name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if col.Name != "name" {
		t.Errorf("expected name, got %s", col.Name)
	}
}

func TestSearchCols_NotFound(t *testing.T) {
	_, err := usersTable.SearchCols("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent column")
	}
}

func TestSearchFks_Found(t *testing.T) {
	fk, found := testSchema.SearchFks("posts", "users")
	if !found {
		t.Fatal("expected to find FK")
	}
	if fk.From != "user_id" || fk.To != "id" {
		t.Errorf("unexpected FK: %+v", fk)
	}
}

func TestSearchFks_NotFound(t *testing.T) {
	_, found := testSchema.SearchFks("users", "posts")
	if found {
		t.Error("expected not to find FK in wrong direction")
	}
}

func TestHasFTSIndex_True(t *testing.T) {
	if !testSchema.HasFTSIndex("users") {
		t.Error("expected users to have FTS index")
	}
}

func TestHasFTSIndex_False(t *testing.T) {
	if testSchema.HasFTSIndex("posts") {
		t.Error("expected posts to not have FTS index")
	}
}

// =============================================================================
// parseDefaultValue Tests - Edge cases (criteria B)
// =============================================================================

func TestParseDefaultValue_QuotedString(t *testing.T) {
	result := parseDefaultValue("'hello'")
	if result != "hello" {
		t.Errorf("expected hello, got %v", result)
	}
}

func TestParseDefaultValue_DoubleQuotedString(t *testing.T) {
	result := parseDefaultValue(`"hello"`)
	if result != "hello" {
		t.Errorf("expected hello, got %v", result)
	}
}

func TestParseDefaultValue_Null(t *testing.T) {
	result := parseDefaultValue("NULL")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseDefaultValue_NullLowercase(t *testing.T) {
	result := parseDefaultValue("null")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseDefaultValue_Expression(t *testing.T) {
	result := parseDefaultValue("CURRENT_TIMESTAMP")
	if result != "CURRENT_TIMESTAMP" {
		t.Errorf("expected CURRENT_TIMESTAMP, got %v", result)
	}
}

func TestParseDefaultValue_Number(t *testing.T) {
	result := parseDefaultValue("42")
	if result != "42" {
		t.Errorf("expected 42, got %v", result)
	}
}

// =============================================================================
// Utility Function Tests
// =============================================================================

func TestSplitTableColumn_WithDot(t *testing.T) {
	parts := splitTableColumn("users.id")
	if len(parts) != 2 || parts[0] != "users" || parts[1] != "id" {
		t.Errorf("expected [users, id], got %v", parts)
	}
}

func TestSplitTableColumn_WithoutDot(t *testing.T) {
	parts := splitTableColumn("name")
	if len(parts) != 1 || parts[0] != "name" {
		t.Errorf("expected [name], got %v", parts)
	}
}

func TestOpToSQL(t *testing.T) {
	tests := []struct {
		op       string
		expected string
	}{
		{OpEq, "="},
		{OpNeq, "!="},
		{OpGt, ">"},
		{OpGte, ">="},
		{OpLt, "<"},
		{OpLte, "<="},
		{"unknown", "="}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			result := opToSQL(tt.op)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestBuildJSONAggregation_Empty(t *testing.T) {
	result := buildJSONAggregation(nil)
	if result != "json_object()" {
		t.Errorf("expected json_object(), got %s", result)
	}
}

func TestBuildJSONAggregation_Simple(t *testing.T) {
	pairs := []string{"'id', [id]", "'name', [name]"}
	result := buildJSONAggregation(pairs)
	if !strings.Contains(result, "json_object(") {
		t.Errorf("expected json_object, got %s", result)
	}
	if !strings.Contains(result, "'id', [id]") {
		t.Errorf("expected id pair, got %s", result)
	}
}

func TestRelationDepth_Flat(t *testing.T) {
	rel := &Relation{name: "users"}
	if d := relationDepth(rel); d != 1 {
		t.Errorf("expected depth 1, got %d", d)
	}
}

func TestRelationDepth_Nested(t *testing.T) {
	child := &Relation{name: "posts"}
	rel := &Relation{name: "users", joins: []*Relation{child}}
	if d := relationDepth(rel); d != 2 {
		t.Errorf("expected depth 2, got %d", d)
	}
}

func TestRelationDepth_DeepNested(t *testing.T) {
	grandchild := &Relation{name: "comments"}
	child := &Relation{name: "posts", joins: []*Relation{grandchild}}
	rel := &Relation{name: "users", joins: []*Relation{child}}
	if d := relationDepth(rel); d != 3 {
		t.Errorf("expected depth 3, got %d", d)
	}
}
