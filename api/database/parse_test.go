package database

import (
	"testing"
)

func TestTokenKeyValList(t *testing.T) {
	str := "users.name\":\":eq.john\\.\\,doe,name,name:neq.776"

	keyVals := tokenKeyValList(str)

	if len(keyVals) != 3 {
		t.Error("expected 3 elements but got ", len(keyVals))
	}

	if keyVals[0][0][0] != "users" {
		t.Error(keyVals[0][0][0])
	}

	if keyVals[0][0][1] != "name:" {
		t.Error(keyVals[0][0][1])
	}

	if keyVals[0][1][0] != "eq" {
		t.Error(keyVals[0][1][0])
	}

	if keyVals[0][1][1] != "john.,doe" {
		t.Error(keyVals[0][1][1])
	}

	if keyVals[1][0][0] != "name" {
		t.Error(keyVals[1][0][0])
	}

	if keyVals[1][1] != nil {
		t.Error(keyVals[1][1][0])
	}

	if keyVals[2][0][0] != "name" {
		t.Error(keyVals[2][0][0])
	}

	if keyVals[2][1][0] != "neq" {
		t.Error(keyVals[2][1][0])
	}

	if keyVals[2][1][1] != "776" {
		t.Error(keyVals[2][1][1])
	}

}

func TestTokenKeyVal(t *testing.T) {

	str := "users.name\":\":eq.john\\.\\,doe"

	keyVal := tokenKeyVal(str)

	if len(keyVal[0]) != 2 {
		t.Error("expected 2 elements but got ", len(keyVal[0]))
	}

	if len(keyVal[1]) != 2 {
		t.Error("expected 2 elements but got ", len(keyVal[1]))
	}

	if keyVal[0][0] != "users" {
		t.Error(keyVal[0][0])
	}

	if keyVal[0][1] != "name:" {
		t.Error(keyVal[0][1])
	}

	if keyVal[1][0] != "eq" {
		t.Error(keyVal[1][0])
	}

	if keyVal[1][1] != "john.,doe" {
		t.Error(keyVal[1][1])
	}
}

func TestTokenList(t *testing.T) {
	str := "t\\\"est.name\".age\"\\.test,tbl.col\",\""

	list := tokenList(str)

	if len(list) != 2 {
		t.Error("expected 2 elements but got ", len(list))
	}

	if len(list[0]) != 2 {
		t.Error("expected 2 tokens but got ", len(list[0]))
	}

	if list[0][0] != "t\"est" {
		t.Error(list[0][0])
	}

	if list[0][1] != "name.age.test" {
		t.Error(list[0][1])
	}

	if list[1][0] != "tbl" {
		t.Error(list[1][0])
	}

	if list[1][1] != "col," {
		t.Error(list[1][1])
	}

}

func TestToken(t *testing.T) {
	str := "t\\\"est.name\".age\"\\.test"

	tokens := token(str)

	if len(tokens) != 2 {
		t.Error("expected 2 tokens but got ", len(tokens))
	}

	if tokens[0] != "t\"est" {
		t.Error(tokens[0])
	}

	if tokens[1] != "name.age.test" {
		t.Error(tokens[1])
	}

}

func TestParseAggregate(t *testing.T) {
	tests := []struct {
		input   string
		wantAgg string
		wantCol string
		wantIs  bool
	}{
		// Valid aggregates
		{"count(*)", "count", "*", true},
		{"sum(price)", "sum", "price", true},
		{"avg(rating)", "avg", "rating", true},
		{"min(age)", "min", "age", true},
		{"max(salary)", "max", "salary", true},
		{"COUNT(*)", "count", "*", true},
		{"SUM(price)", "sum", "price", true},

		// With spaces
		{"count( * )", "count", "*", true},
		{"sum( price )", "sum", "price", true},

		// Not aggregates - regular columns
		{"name", "", "name", false},
		{"price", "", "price", false},
		{"*", "", "*", false},

		// Not aggregates - function-like but not recognized
		{"foo(bar)", "", "foo(bar)", false},
		{"unknown(col)", "", "unknown(col)", false},

		// Edge cases
		{"", "", "", false},
		{"count", "", "count", false},   // No parentheses
		{"count()", "count", "", true},  // Empty parentheses - still valid aggregate
		{"sum(a,b)", "sum", "a,b", true}, // Multiple args - parsed as single column name
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			agg, col, isAgg := parseAggregate(tt.input)
			if agg != tt.wantAgg {
				t.Errorf("parseAggregate(%q) agg = %q, want %q", tt.input, agg, tt.wantAgg)
			}
			if col != tt.wantCol {
				t.Errorf("parseAggregate(%q) col = %q, want %q", tt.input, col, tt.wantCol)
			}
			if isAgg != tt.wantIs {
				t.Errorf("parseAggregate(%q) isAgg = %v, want %v", tt.input, isAgg, tt.wantIs)
			}
		})
	}
}

func TestIsAggregateFunc(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"count", true},
		{"sum", true},
		{"avg", true},
		{"min", true},
		{"max", true},
		{"COUNT", true},
		{"SUM", true},
		{"AVG", true},
		{"MIN", true},
		{"MAX", true},
		{"foo", false},
		{"total", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAggregateFunc(tt.name)
			if got != tt.want {
				t.Errorf("isAggregateFunc(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestAutoAggAlias(t *testing.T) {
	tests := []struct {
		agg  string
		col  string
		want string
	}{
		{"count", "*", "count"},
		{"sum", "price", "sum_price"},
		{"avg", "rating", "avg_rating"},
		{"min", "age", "min_age"},
		{"max", "salary", "max_salary"},
	}

	for _, tt := range tests {
		t.Run(tt.agg+"_"+tt.col, func(t *testing.T) {
			got := autoAggAlias(tt.agg, tt.col)
			if got != tt.want {
				t.Errorf("autoAggAlias(%q, %q) = %q, want %q", tt.agg, tt.col, got, tt.want)
			}
		})
	}
}

func TestBuildFilter(t *testing.T) {
	tests := []struct {
		name      string
		table     string
		column    string
		where     []string
		wantQuery string
		wantArgs  int
	}{
		{
			name:      "simple eq",
			table:     "users",
			column:    "name",
			where:     []string{"eq", "john"},
			wantQuery: "[users].[name] = ? ",
			wantArgs:  1,
		},
		{
			name:      "not eq",
			table:     "users",
			column:    "name",
			where:     []string{"not", "eq", "john"},
			wantQuery: "[users].[name] NOT = ? ",
			wantArgs:  1,
		},
		{
			name:      "not like",
			table:     "users",
			column:    "name",
			where:     []string{"not", "like", "%test%"},
			wantQuery: "[users].[name] NOT LIKE ? ",
			wantArgs:  1,
		},
		{
			name:      "not is null",
			table:     "users",
			column:    "email",
			where:     []string{"not", "is", "null"},
			wantQuery: "[users].[email] IS NOT NULL ",
			wantArgs:  0,
		},
		{
			name:      "not is true",
			table:     "users",
			column:    "active",
			where:     []string{"not", "is", "true"},
			wantQuery: "[users].[active] IS NOT TRUE ",
			wantArgs:  0,
		},
		{
			name:      "not is false",
			table:     "users",
			column:    "active",
			where:     []string{"not", "is", "false"},
			wantQuery: "[users].[active] IS NOT FALSE ",
			wantArgs:  0,
		},
		{
			name:      "not in",
			table:     "users",
			column:    "status",
			where:     []string{"not", "in", "(a,b,c)"},
			wantQuery: "[users].[status] NOT IN (a,b,c) ",
			wantArgs:  0,
		},
		{
			name:      "is null",
			table:     "users",
			column:    "email",
			where:     []string{"is", "null"},
			wantQuery: "[users].[email] IS ? ",
			wantArgs:  1,
		},
		{
			name:      "gt",
			table:     "users",
			column:    "age",
			where:     []string{"gt", "18"},
			wantQuery: "[users].[age] > ? ",
			wantArgs:  1,
		},
		{
			name:      "not gt",
			table:     "users",
			column:    "age",
			where:     []string{"not", "gt", "18"},
			wantQuery: "[users].[age] NOT > ? ",
			wantArgs:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := buildFilter(tt.table, tt.column, tt.where)
			if query != tt.wantQuery {
				t.Errorf("buildFilter() query = %q, want %q", query, tt.wantQuery)
			}
			if len(args) != tt.wantArgs {
				t.Errorf("buildFilter() args count = %d, want %d", len(args), tt.wantArgs)
			}
		})
	}
}

func TestParseSelectWithAggregates(t *testing.T) {
	tests := []struct {
		name      string
		param     string
		table     string
		wantCols  int
		checkFunc func(*testing.T, Relation)
	}{
		{
			name:     "simple count",
			param:    "count(*)",
			table:    "users",
			wantCols: 1,
			checkFunc: func(t *testing.T, rel Relation) {
				if rel.columns[0].aggregate != "count" {
					t.Errorf("Expected aggregate 'count', got %q", rel.columns[0].aggregate)
				}
				if rel.columns[0].name != "*" {
					t.Errorf("Expected column '*', got %q", rel.columns[0].name)
				}
			},
		},
		{
			name:     "sum with column",
			param:    "sum(price)",
			table:    "orders",
			wantCols: 1,
			checkFunc: func(t *testing.T, rel Relation) {
				if rel.columns[0].aggregate != "sum" {
					t.Errorf("Expected aggregate 'sum', got %q", rel.columns[0].aggregate)
				}
				if rel.columns[0].name != "price" {
					t.Errorf("Expected column 'price', got %q", rel.columns[0].name)
				}
			},
		},
		{
			name:     "multiple aggregates",
			param:    "count(*),sum(price),avg(rating)",
			table:    "products",
			wantCols: 3,
			checkFunc: func(t *testing.T, rel Relation) {
				if rel.columns[0].aggregate != "count" {
					t.Errorf("Expected first aggregate 'count', got %q", rel.columns[0].aggregate)
				}
				if rel.columns[1].aggregate != "sum" {
					t.Errorf("Expected second aggregate 'sum', got %q", rel.columns[1].aggregate)
				}
				if rel.columns[2].aggregate != "avg" {
					t.Errorf("Expected third aggregate 'avg', got %q", rel.columns[2].aggregate)
				}
			},
		},
		{
			name:     "mixed columns and aggregates",
			param:    "category,count(*),sum(price)",
			table:    "products",
			wantCols: 3,
			checkFunc: func(t *testing.T, rel Relation) {
				// First column is regular
				if rel.columns[0].aggregate != "" {
					t.Errorf("Expected first column to have no aggregate, got %q", rel.columns[0].aggregate)
				}
				if rel.columns[0].name != "category" {
					t.Errorf("Expected first column 'category', got %q", rel.columns[0].name)
				}
				// Second is aggregate
				if rel.columns[1].aggregate != "count" {
					t.Errorf("Expected second aggregate 'count', got %q", rel.columns[1].aggregate)
				}
			},
		},
		{
			name:     "aggregate with alias",
			param:    "total:sum(price)",
			table:    "orders",
			wantCols: 1,
			checkFunc: func(t *testing.T, rel Relation) {
				if rel.columns[0].aggregate != "sum" {
					t.Errorf("Expected aggregate 'sum', got %q", rel.columns[0].aggregate)
				}
				if rel.columns[0].alias != "total" {
					t.Errorf("Expected alias 'total', got %q", rel.columns[0].alias)
				}
			},
		},
		{
			name:     "nested relation not confused with aggregate",
			param:    "name,posts(title,body)",
			table:    "users",
			wantCols: 1,
			checkFunc: func(t *testing.T, rel Relation) {
				// Should have 1 column and 1 join
				if rel.columns[0].name != "name" {
					t.Errorf("Expected column 'name', got %q", rel.columns[0].name)
				}
				if len(rel.joins) != 1 {
					t.Errorf("Expected 1 join, got %d", len(rel.joins))
				}
				if rel.joins[0].name != "posts" {
					t.Errorf("Expected join 'posts', got %q", rel.joins[0].name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel := parseSelect(tt.param, tt.table)
			if len(rel.columns) != tt.wantCols {
				t.Errorf("parseSelect(%q, %q) got %d columns, want %d", tt.param, tt.table, len(rel.columns), tt.wantCols)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, rel)
			}
		})
	}
}
