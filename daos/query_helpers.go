package daos

import (
	"fmt"
	"net/url"
	"strings"
)

func (schema SchemaCache) buildReturning(relation string, cols string) (string, error) {
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

			err := schema.checkCol(relation, currStr)
			if err != nil {
				return "", err
			}
			query += fmt.Sprintf("[%s]", currStr)
			if alias != "" {
				query += " AS " + alias
			}
			query += ", "
			alias = ""
			currStr = ""
		default:
			currStr += string(r)
		}
	}

	if currStr != "" {
		err := schema.checkCol(relation, currStr)
		if err != nil {
			return "", err
		}
		query += fmt.Sprintf("[%s]", currStr)
		if alias != "" {
			query += " AS " + alias
		}
		query += ", "
	}

	return query[:len(query)-2] + " ", nil
}

func (schema SchemaCache) BuildWhere(relation string, params url.Values) (string, []any, error) {
	query := "WHERE "
	var args []any
	hasWhere := false

	for name, val := range params {
		if name == "order" || name == "select" {
			continue
		}

		if name == "or" {
			for _, v := range val {
				orList := tokenKeyValList(v)

				for _, or := range orList {
					var tbl string
					var col string

					if len(orList[0]) < 2 {
						tbl = relation
						col = or[0][0]
					} else {
						tbl = or[0][0]
						col = or[0][1]
					}

					err := schema.checkCol(tbl, col)
					if err != nil {
						return "", nil, err
					}

					where, whereArgs := buildFilter(tbl, col, or[1])
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
			tbl = relation
			col = split[0]
		} else {
			tbl = split[0]
			col = split[1]
		}

		err := schema.checkCol(tbl, col)
		if err != nil {
			return "", nil, err
		}

		for _, v := range val {
			where, whereArgs := buildFilter(tbl, col, token(v))

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

func (schema SchemaCache) BuildOrder(relation, orderBy string) (string, error) {
	if orderBy == "" {
		return "", nil
	}

	query := "ORDER BY "

	orderList := tokenKeyValList(orderBy)

	for _, order := range orderList {

		var tbl string
		var col string
		if len(order[0]) < 2 {
			tbl = relation
			col = order[0][0]
		} else {
			tbl = order[0][0]
			col = order[0][1]
		}

		err := schema.checkCol(tbl, col)
		if err != nil {
			return "", err
		}

		query += fmt.Sprintf("[%s].[%s] ", tbl, col)
		if order[1] != nil {
			switch strings.ToLower(order[1][0]) {
			case "asc":
				query += "ASC"
			case "desc":
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
