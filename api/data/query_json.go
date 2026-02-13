package data

import (
	"fmt"
	"strings"

	"github.com/atomicbase/atomicbase/tools"
)

// BuildWhereFromJSON builds a WHERE clause from JSON filter array.
// Each element in the array is ANDed together.
// Example input: [{"id": {"eq": 5}}, {"or": [{"status": {"eq": "active"}}, {"role": {"eq": "admin"}}]}]
func (table CacheTable) BuildWhereFromJSON(where []map[string]any, schema SchemaCache) (string, []any, error) {
	if len(where) == 0 {
		return "", nil, nil
	}

	query := "WHERE "
	var args []any
	first := true

	for _, condition := range where {
		for key, value := range condition {
			// Handle OR conditions
			if key == "or" {
				orConditions, ok := value.([]any)
				if !ok {
					return "", nil, fmt.Errorf("or value must be an array")
				}

				orClause, orArgs, err := table.buildOrClause(orConditions, schema)
				if err != nil {
					return "", nil, err
				}

				if !first {
					query += "AND "
				}
				first = false
				query += "(" + orClause + ") "
				args = append(args, orArgs...)
				continue
			}

			// Handle table-wide FTS search: {"__fts": {"fts": "query"}}
			if key == "__fts" {
				filterMap, ok := value.(map[string]any)
				if !ok {
					return "", nil, fmt.Errorf("__fts value must be an object")
				}
				ftsQuery, ok := filterMap["fts"]
				if !ok {
					return "", nil, fmt.Errorf("__fts requires fts operator")
				}
				if !schema.HasFTSIndex(table.Name) {
					return "", nil, fmt.Errorf("%w: %s", tools.ErrNoFTSIndex, table.Name)
				}
				ftsTable := table.Name + FTSSuffix
				clause := fmt.Sprintf("rowid IN (SELECT rowid FROM [%s] WHERE [%s] MATCH ?) ", ftsTable, ftsTable)

				if !first {
					query += "AND "
				}
				first = false
				query += clause
				args = append(args, ftsQuery)
				continue
			}

			// Regular column filter
			filterMap, ok := value.(map[string]any)
			if !ok {
				return "", nil, fmt.Errorf("filter for column %s must be an object", key)
			}

			clause, clauseArgs, err := table.buildFilterClause(key, filterMap, schema)
			if err != nil {
				return "", nil, err
			}

			if !first {
				query += "AND "
			}
			first = false
			query += clause
			args = append(args, clauseArgs...)
		}
	}

	return query, args, nil
}

// buildOrClause builds an OR clause from an array of conditions.
func (table CacheTable) buildOrClause(conditions []any, schema SchemaCache) (string, []any, error) {
	var parts []string
	var args []any

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]any)
		if !ok {
			return "", nil, fmt.Errorf("or condition must be an object")
		}

		for col, filter := range condMap {
			filterMap, ok := filter.(map[string]any)
			if !ok {
				return "", nil, fmt.Errorf("filter must be an object")
			}

			clause, clauseArgs, err := table.buildFilterClause(col, filterMap, schema)
			if err != nil {
				return "", nil, err
			}

			parts = append(parts, strings.TrimSuffix(clause, " "))
			args = append(args, clauseArgs...)
		}
	}

	return strings.Join(parts, " OR "), args, nil
}

// isColumnRef checks if a value is a column reference marker {"__col": "column_name"}
// and returns the column name if so.
func isColumnRef(val any) (string, bool) {
	m, ok := val.(map[string]any)
	if !ok {
		return "", false
	}
	if colName, ok := m["__col"].(string); ok {
		return colName, true
	}
	return "", false
}

// buildFilterClause builds a single filter clause for a column.
// Supports table.column syntax for join queries.
func (table CacheTable) buildFilterClause(column string, filter map[string]any, schema SchemaCache) (string, []any, error) {
	// Parse table.column format if present
	tableName := table.Name
	colName := column
	if idx := strings.Index(column, "."); idx != -1 {
		tableName = column[:idx]
		colName = column[idx+1:]
		// Validate the table exists in schema
		tbl, err := schema.SearchTbls(tableName)
		if err != nil {
			return "", nil, err
		}
		// Validate column exists in that table
		if _, err := tbl.SearchCols(colName); err != nil {
			return "", nil, err
		}
	} else {
		// Validate column exists in base table
		_, err := table.SearchCols(column)
		if err != nil {
			return "", nil, err
		}
	}

	var args []any

	// Check for NOT wrapper
	if notFilter, ok := filter["not"]; ok {
		notMap, ok := notFilter.(map[string]any)
		if !ok {
			return "", nil, fmt.Errorf("not value must be an object")
		}
		return table.buildNotFilterClauseWithTable(tableName, colName, notMap, schema)
	}

	// Handle each operator
	for op, val := range filter {
		// Check if value is a column reference
		if colRef, isCol := isColumnRef(val); isCol {
			// Validate referenced column exists
			if _, err := table.SearchCols(colRef); err != nil {
				return "", nil, err
			}
			sqlOp := opToSQL(op)
			return fmt.Sprintf("[%s].[%s] %s [%s].[%s] ", tableName, colName, sqlOp, table.Name, colRef), nil, nil
		}

		switch op {
		case OpEq:
			return fmt.Sprintf("[%s].[%s] = ? ", tableName, colName), []any{val}, nil
		case OpNeq:
			return fmt.Sprintf("[%s].[%s] != ? ", tableName, colName), []any{val}, nil
		case OpGt:
			return fmt.Sprintf("[%s].[%s] > ? ", tableName, colName), []any{val}, nil
		case OpGte:
			return fmt.Sprintf("[%s].[%s] >= ? ", tableName, colName), []any{val}, nil
		case OpLt:
			return fmt.Sprintf("[%s].[%s] < ? ", tableName, colName), []any{val}, nil
		case OpLte:
			return fmt.Sprintf("[%s].[%s] <= ? ", tableName, colName), []any{val}, nil
		case OpLike:
			return fmt.Sprintf("[%s].[%s] LIKE ? ", tableName, colName), []any{val}, nil
		case OpGlob:
			return fmt.Sprintf("[%s].[%s] GLOB ? ", tableName, colName), []any{val}, nil
		case OpIs:
			// IS NULL, IS TRUE, IS FALSE
			if val == nil {
				return fmt.Sprintf("[%s].[%s] IS NULL ", tableName, colName), nil, nil
			}
			return fmt.Sprintf("[%s].[%s] IS %v ", tableName, colName, val), nil, nil
		case OpIn:
			arr, ok := val.([]any)
			if !ok {
				return "", nil, fmt.Errorf("in value must be an array")
			}
			if len(arr) == 0 {
				return "", nil, fmt.Errorf("in array cannot be empty")
			}
			if len(arr) > MaxInArraySize {
				return "", nil, fmt.Errorf("%w: %d elements (max %d)", tools.ErrInArrayTooLarge, len(arr), MaxInArraySize)
			}
			placeholders := make([]string, len(arr))
			for i := range arr {
				placeholders[i] = "?"
			}
			return fmt.Sprintf("[%s].[%s] IN (%s) ", tableName, colName, strings.Join(placeholders, ", ")), arr, nil
		case OpBetween:
			arr, ok := val.([]any)
			if !ok || len(arr) != 2 {
				return "", nil, fmt.Errorf("between value must be an array of exactly 2 elements")
			}
			return fmt.Sprintf("[%s].[%s] BETWEEN ? AND ? ", tableName, colName), arr, nil
		case OpFts:
			// Full-text search on specific column (only supported on base table)
			if !schema.HasFTSIndex(tableName) {
				return "", nil, fmt.Errorf("%w: %s", tools.ErrNoFTSIndex, tableName)
			}
			ftsTable := tableName + FTSSuffix
			// Search specific column within the FTS index
			query := fmt.Sprintf("rowid IN (SELECT rowid FROM [%s] WHERE [%s] MATCH ?) ", ftsTable, colName)
			return query, []any{val}, nil
		default:
			return "", nil, fmt.Errorf("%w: %s", tools.ErrInvalidOperator, op)
		}
	}

	return "", args, nil
}

// buildNotFilterClauseWithTable builds a NOT filter clause with explicit table name.
func (table CacheTable) buildNotFilterClauseWithTable(tableName, colName string, filter map[string]any, schema SchemaCache) (string, []any, error) {
	for op, val := range filter {
		switch op {
		case OpEq:
			return fmt.Sprintf("[%s].[%s] != ? ", tableName, colName), []any{val}, nil
		case OpIn:
			arr, ok := val.([]any)
			if !ok {
				return "", nil, fmt.Errorf("in value must be an array")
			}
			if len(arr) == 0 {
				return "", nil, fmt.Errorf("in array cannot be empty")
			}
			if len(arr) > MaxInArraySize {
				return "", nil, fmt.Errorf("%w: %d elements (max %d)", tools.ErrInArrayTooLarge, len(arr), MaxInArraySize)
			}
			placeholders := make([]string, len(arr))
			for i := range arr {
				placeholders[i] = "?"
			}
			return fmt.Sprintf("[%s].[%s] NOT IN (%s) ", tableName, colName, strings.Join(placeholders, ", ")), arr, nil
		case OpIs:
			if val == nil {
				return fmt.Sprintf("[%s].[%s] IS NOT NULL ", tableName, colName), nil, nil
			}
			return fmt.Sprintf("[%s].[%s] IS NOT %v ", tableName, colName, val), nil, nil
		case OpLike:
			return fmt.Sprintf("[%s].[%s] NOT LIKE ? ", tableName, colName), []any{val}, nil
		case OpGlob:
			return fmt.Sprintf("[%s].[%s] NOT GLOB ? ", tableName, colName), []any{val}, nil
		default:
			return "", nil, fmt.Errorf("%w: not.%s", tools.ErrInvalidOperator, op)
		}
	}
	return "", nil, nil
}

// BuildOrderFromJSON builds an ORDER BY clause from JSON order map.
// Example input: {"created_at": "desc", "name": "asc"}
func (table CacheTable) BuildOrderFromJSON(order map[string]string) (string, error) {
	if len(order) == 0 {
		return "", nil
	}

	query := "ORDER BY "
	first := true

	for col, dir := range order {
		// Validate column exists
		_, err := table.SearchCols(col)
		if err != nil {
			return "", err
		}

		if !first {
			query += ", "
		}
		first = false

		query += fmt.Sprintf("[%s].[%s] ", table.Name, col)

		switch strings.ToLower(dir) {
		case OrderAsc:
			query += "ASC"
		case OrderDesc:
			query += "DESC"
		default:
			return "", fmt.Errorf("invalid order direction: %s", dir)
		}
	}

	return query + " ", nil
}

// ParseSelectFromJSON parses JSON select array into a Relation tree.
// Example input: ["id", "name", {"posts": ["title", {"comments": ["body"]}]}]
func ParseSelectFromJSON(sel []any, tableName string) (Relation, error) {
	rel := Relation{name: tableName, columns: nil, joins: nil, parent: nil}

	if len(sel) == 0 {
		// Default to all columns
		rel.columns = []column{{"*", ""}}
		return rel, nil
	}

	for _, item := range sel {
		switch v := item.(type) {
		case string:
			// Simple column name or "*"
			rel.columns = append(rel.columns, column{name: v, alias: ""})

		case map[string]any:
			// Could be a nested relation or aliased column
			for key, value := range v {
				// Check if it's a nested relation (value is an array)
				if cols, ok := value.([]any); ok {
					nestedRel, err := ParseSelectFromJSON(cols, key)
					if err != nil {
						return rel, err
					}
					nestedRel.parent = &rel
					rel.joins = append(rel.joins, &nestedRel)
				} else {
					// It's an aliased column: {"alias": "column"}
					if colName, ok := value.(string); ok {
						rel.columns = append(rel.columns, column{name: colName, alias: key})
					}
				}
			}

		default:
			return rel, fmt.Errorf("invalid select item type: %T", item)
		}
	}

	// If no columns specified but we have joins, default to all columns
	if len(rel.columns) == 0 && len(rel.joins) > 0 {
		rel.columns = []column{{"*", ""}}
	}

	return rel, nil
}

// BuildReturningFromJSON builds a RETURNING clause from JSON column array.
func (table CacheTable) BuildReturningFromJSON(cols []string) (string, error) {
	if len(cols) == 0 {
		return "", nil
	}

	if len(cols) == 1 && cols[0] == "*" {
		return "RETURNING * ", nil
	}

	query := "RETURNING "
	for i, col := range cols {
		_, err := table.SearchCols(col)
		if err != nil {
			return "", err
		}

		if i > 0 {
			query += ", "
		}
		query += fmt.Sprintf("[%s]", col)
	}

	return query + " ", nil
}
