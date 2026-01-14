package database

import (
	"fmt"
	"strings"
)

// BuildWhereFromJSON builds a WHERE clause from JSON filter array.
// Each element in the array is ANDed together.
// Example input: [{"id": {"eq": 5}}, {"or": [{"status": {"eq": "active"}}, {"role": {"eq": "admin"}}]}]
func (table Table) BuildWhereFromJSON(where []map[string]any, schema SchemaCache) (string, []any, error) {
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
func (table Table) buildOrClause(conditions []any, schema SchemaCache) (string, []any, error) {
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

// buildFilterClause builds a single filter clause for a column.
func (table Table) buildFilterClause(column string, filter map[string]any, schema SchemaCache) (string, []any, error) {
	// Validate column exists
	_, err := table.SearchCols(column)
	if err != nil {
		return "", nil, err
	}

	var args []any

	// Check for NOT wrapper
	if notFilter, ok := filter["not"]; ok {
		notMap, ok := notFilter.(map[string]any)
		if !ok {
			return "", nil, fmt.Errorf("not value must be an object")
		}
		return table.buildNotFilterClause(column, notMap, schema)
	}

	// Handle each operator
	for op, val := range filter {
		switch op {
		case OpEq:
			return fmt.Sprintf("[%s].[%s] = ? ", table.Name, column), []any{val}, nil
		case OpNeq:
			return fmt.Sprintf("[%s].[%s] != ? ", table.Name, column), []any{val}, nil
		case OpGt:
			return fmt.Sprintf("[%s].[%s] > ? ", table.Name, column), []any{val}, nil
		case OpGte:
			return fmt.Sprintf("[%s].[%s] >= ? ", table.Name, column), []any{val}, nil
		case OpLt:
			return fmt.Sprintf("[%s].[%s] < ? ", table.Name, column), []any{val}, nil
		case OpLte:
			return fmt.Sprintf("[%s].[%s] <= ? ", table.Name, column), []any{val}, nil
		case OpLike:
			return fmt.Sprintf("[%s].[%s] LIKE ? ", table.Name, column), []any{val}, nil
		case OpGlob:
			return fmt.Sprintf("[%s].[%s] GLOB ? ", table.Name, column), []any{val}, nil
		case OpIs:
			// IS NULL, IS TRUE, IS FALSE
			if val == nil {
				return fmt.Sprintf("[%s].[%s] IS NULL ", table.Name, column), nil, nil
			}
			return fmt.Sprintf("[%s].[%s] IS %v ", table.Name, column, val), nil, nil
		case OpIn:
			arr, ok := val.([]any)
			if !ok {
				return "", nil, fmt.Errorf("in value must be an array")
			}
			placeholders := make([]string, len(arr))
			for i := range arr {
				placeholders[i] = "?"
			}
			return fmt.Sprintf("[%s].[%s] IN (%s) ", table.Name, column, strings.Join(placeholders, ", ")), arr, nil
		case OpBetween:
			arr, ok := val.([]any)
			if !ok || len(arr) != 2 {
				return "", nil, fmt.Errorf("between value must be an array of exactly 2 elements")
			}
			return fmt.Sprintf("[%s].[%s] BETWEEN ? AND ? ", table.Name, column), arr, nil
		case OpFts:
			// Full-text search
			if !schema.HasFTSIndex(table.Name) {
				return "", nil, fmt.Errorf("%w: %s", ErrNoFTSIndex, table.Name)
			}
			ftsTable := table.Name + FTSSuffix
			query := fmt.Sprintf("rowid IN (SELECT rowid FROM [%s] WHERE [%s] MATCH ?) ", ftsTable, ftsTable)
			return query, []any{val}, nil
		default:
			return "", nil, fmt.Errorf("%w: %s", ErrInvalidOperator, op)
		}
	}

	return "", args, nil
}

// buildNotFilterClause builds a NOT filter clause.
func (table Table) buildNotFilterClause(column string, filter map[string]any, schema SchemaCache) (string, []any, error) {
	for op, val := range filter {
		switch op {
		case OpEq:
			return fmt.Sprintf("[%s].[%s] != ? ", table.Name, column), []any{val}, nil
		case OpIn:
			arr, ok := val.([]any)
			if !ok {
				return "", nil, fmt.Errorf("in value must be an array")
			}
			placeholders := make([]string, len(arr))
			for i := range arr {
				placeholders[i] = "?"
			}
			return fmt.Sprintf("[%s].[%s] NOT IN (%s) ", table.Name, column, strings.Join(placeholders, ", ")), arr, nil
		case OpIs:
			if val == nil {
				return fmt.Sprintf("[%s].[%s] IS NOT NULL ", table.Name, column), nil, nil
			}
			return fmt.Sprintf("[%s].[%s] IS NOT %v ", table.Name, column, val), nil, nil
		case OpLike:
			return fmt.Sprintf("[%s].[%s] NOT LIKE ? ", table.Name, column), []any{val}, nil
		case OpGlob:
			return fmt.Sprintf("[%s].[%s] NOT GLOB ? ", table.Name, column), []any{val}, nil
		default:
			return "", nil, fmt.Errorf("%w: not.%s", ErrInvalidOperator, op)
		}
	}
	return "", nil, nil
}

// BuildOrderFromJSON builds an ORDER BY clause from JSON order map.
// Example input: {"created_at": "desc", "name": "asc"}
func (table Table) BuildOrderFromJSON(order map[string]string) (string, error) {
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
func (table Table) BuildReturningFromJSON(cols []string) (string, error) {
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
