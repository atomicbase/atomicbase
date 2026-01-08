package daos

import (
	"fmt"
	"net/url"
	"strings"
)

// sanitizeAlias validates and sanitizes an alias for use in SQL.
// Returns an error if the alias contains invalid characters.
func sanitizeAlias(alias string) (string, error) {
	if alias == "" {
		return "", nil
	}
	if err := ValidateIdentifier(alias); err != nil {
		return "", fmt.Errorf("invalid alias: %w", err)
	}
	return "[" + alias + "]", nil
}

func (table Table) buildReturning(cols string) (string, error) {
	if cols == "" || cols == "*" {
		return "RETURNING * ", nil
	}

	query := "RETURNING "

	inQuotes := false
	escaped := false
	currStr := ""
	alias := ""

	for _, r := range cols {
		if escaped {
			escaped = false
			currStr += string(r)
			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
		case ':':
			alias = currStr
			currStr = ""
		case ',':
			if inQuotes {
				currStr += ","
				continue
			}

			_, err := table.SearchCols(currStr)
			if err != nil {
				return "", err
			}
			query += fmt.Sprintf("[%s]", currStr)
			if alias != "" {
				sanitized, err := sanitizeAlias(alias)
				if err != nil {
					return "", err
				}
				query += " AS " + sanitized
			}
			query += ", "
			alias = ""
			currStr = ""
		default:
			currStr += string(r)
		}
	}

	if currStr != "" {
		_, err := table.SearchCols(currStr)
		if err != nil {
			return "", err
		}
		query += fmt.Sprintf("[%s]", currStr)
		if alias != "" {
			sanitized, err := sanitizeAlias(alias)
			if err != nil {
				return "", err
			}
			query += " AS " + sanitized
		}
		query += ", "
	}

	return query[:len(query)-2] + " ", nil
}

func (table Table) BuildWhere(params url.Values) (string, []any, error) {
	return table.BuildWhereWithSchema(params, SchemaCache{})
}

// BuildWhereWithSchema builds WHERE clause with schema access for FTS support.
func (table Table) BuildWhereWithSchema(params url.Values, schema SchemaCache) (string, []any, error) {
	query := "WHERE "
	var args []any
	hasWhere := false

	for name, val := range params {
		if name == ParamOrder || name == ParamSelect || name == ParamLimit || name == ParamOffset || name == ParamCount {
			continue
		}

		if name == ParamOr {
			for _, v := range val {
				orList := tokenKeyValList(v)

				for _, or := range orList {
					var tbl string
					var col string

					if len(orList[0]) < 2 {
						tbl = table.Name
						col = or[0][0]
					} else {
						tbl = or[0][0]
						col = or[0][1]
					}

					_, err := table.SearchCols(col)
					if err != nil {
						return "", nil, err
					}

					where, whereArgs, err := buildFilterWithSchema(tbl, col, or[1], schema)
					if err != nil {
						return "", nil, err
					}
					if hasWhere {
						query += "OR "
					} else {
						hasWhere = true
					}

					query += where
					args = append(args, whereArgs...)
				}
			}
			continue
		}

		var tbl string
		var col string
		split := token(name)
		if len(split) < 2 {
			tbl = table.Name
			col = split[0]
		} else {
			tbl = split[0]
			col = split[1]
		}

		_, err := table.SearchCols(col)
		if err != nil {
			return "", nil, err
		}

		for _, v := range val {
			where, whereArgs, err := buildFilterWithSchema(tbl, col, token(v), schema)
			if err != nil {
				return "", nil, err
			}

			if hasWhere {
				query += "AND "
			} else {
				hasWhere = true
			}

			query += where
			args = append(args, whereArgs...)
		}
	}

	if !hasWhere {
		return "", nil, nil
	}

	return query, args, nil
}

func (table Table) BuildOrder(orderBy string) (string, error) {
	if orderBy == "" {
		return "", nil
	}

	query := "ORDER BY "

	orderList := tokenKeyValList(orderBy)

	for _, order := range orderList {
		var tbl string
		var col string
		if len(order[0]) < 2 {
			tbl = table.Name
			col = order[0][0]
		} else {
			tbl = order[0][0]
			col = order[0][1]
		}

		_, err := table.SearchCols(col)
		if err != nil {
			return "", err
		}

		query += fmt.Sprintf("[%s].[%s] ", tbl, col)
		if order[1] != nil {
			switch strings.ToLower(order[1][0]) {
			case OrderAsc:
				query += "ASC"
			case OrderDesc:
				query += "DESC"
			default:
				return "", fmt.Errorf("unknown keyword %s", order[1][0])
			}
		}

		query += ", "
	}

	return query[:len(query)-2] + " ", nil
}

func buildFilter(table string, column string, where []string) (string, []any) {

	query := fmt.Sprintf("[%s].[%s] ", table, column)
	var args []any

	for _, op := range where {
		if mapOperator(op) != "" {
			query += mapOperator(op) + " "
			continue
		}

		query += "? "
		args = append(args, op)
	}

	return query, args
}

// buildFilterWithSchema builds a filter clause with schema awareness for FTS support.
func buildFilterWithSchema(table string, column string, where []string, schema SchemaCache) (string, []any, error) {
	// Check for FTS operator
	if len(where) > 0 && where[0] == OpFts {
		// Verify FTS index exists for this table
		if !schema.HasFTSIndex(table) {
			return "", nil, fmt.Errorf("%w: %s", ErrNoFTSIndex, table)
		}

		// Build FTS subquery: rowid IN (SELECT rowid FROM {table}_fts WHERE {table}_fts MATCH ?)
		ftsTable := table + FTSSuffix
		query := fmt.Sprintf("rowid IN (SELECT rowid FROM [%s] WHERE [%s] MATCH ?) ", ftsTable, ftsTable)

		var args []any
		// Join remaining parts as the search query
		if len(where) > 1 {
			args = append(args, strings.Join(where[1:], " "))
		}

		return query, args, nil
	}

	// Fall back to regular filter
	query, args := buildFilter(table, column, where)
	return query, args, nil
}
