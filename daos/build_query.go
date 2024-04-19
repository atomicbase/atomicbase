package daos

import (
	"fmt"
	"strings"
)

type Table struct {
	name    string
	alias   string
	inner   bool
	columns []column
	joins   []*Table
	parent  *Table
}

type column struct {
	name  string
	alias string
}

func (schema SchemaCache) buildSelect(table Table) (string, string, error) {
	agg := ""
	sel := ""
	joins := ""

	if table.columns == nil && table.joins == nil {
		table.columns = []column{{"*", ""}}
	}

	for _, col := range table.columns {
		if col.name == "*" {
			sel += "*, "
			for name, t := range schema.Tables[table.name] {
				if strings.ToLower(t) == "blob" {
					continue
				}

				agg += fmt.Sprintf("'%s', [%s], ", name, name)
			}

			continue
		}

		if strings.ToLower(schema.Tables[table.name][col.name]) == "blob" {
			continue
		}

		if col.name == "count" {
			sel += fmt.Sprintf("COUNT([%s]) as [count], ", schema.Pks[table.name])
			agg += "'count', [count], "
			continue
		}

		sel += fmt.Sprintf("[%s].[%s], ", table.name, col.name)
		if col.alias != "" {
			agg += fmt.Sprintf("'%s', [%s], ", col.alias, col.name)
		} else {
			agg += fmt.Sprintf("'%s', [%s], ", col.name, col.name)
		}
	}

	for _, tbl := range table.joins {
		if tbl.alias != "" {
			agg += fmt.Sprintf("'%s', json([%s]), ", tbl.alias, tbl.name)
		} else {
			agg += fmt.Sprintf("'%s', json([%s]), ", tbl.name, tbl.name)
		}
		query, aggs, err := schema.buildSelCurr(*tbl, table.name)
		if err != nil {
			return "", "", err
		}
		var fk Fk
		for _, key := range schema.Fks {
			if key.References == table.name && key.Table == tbl.name {
				fk = key
			}
		}

		if fk == (Fk{}) {
			return "", "", err
		}
		sel += fmt.Sprintf("json_group_array(json_object(%s)) FILTER (WHERE [%s].[%s] IS NOT NULL) AS [%s], ", aggs, fk.Table, fk.From, tbl.name)

		if tbl.inner {
			joins += "INNER "
		} else {
			joins += "LEFT "
		}

		joins += fmt.Sprintf("JOIN (%s) AS [%s] ON [%s].[%s] = [%s].[%s] ", query, tbl.name, fk.References, fk.To, fk.Table, fk.From)
	}

	return "SELECT " + sel[:len(sel)-2] + fmt.Sprintf(" FROM [%s] ", table.name) + joins, agg[:len(agg)-2], nil
}

func (schema SchemaCache) buildSelCurr(table Table, joinedOn string) (string, string, error) {
	var sel string
	var joins string
	var agg string
	includesFk := false
	var fk Fk

	if table.columns == nil && table.joins == nil {
		table.columns = []column{{"*", ""}}
	}

	if joinedOn != "" {
		for _, key := range schema.Fks {
			if key.References == joinedOn && key.Table == table.name {
				fk = key
			}
		}
	}

	for _, col := range table.columns {
		if joinedOn != "" && fk.Table == table.name && fk.From == col.name {
			includesFk = true
		}

		if col.name == "*" {
			sel += "*, "
			for name, t := range schema.Tables[table.name] {
				if strings.ToLower(t) == "blob" {
					continue
				}

				agg += fmt.Sprintf("'%s', [%s].[%s], ", name, table.name, name)
			}

			continue
		}

		if strings.ToLower(schema.Tables[table.name][col.name]) == "blob" {
			continue
		}

		if col.name == "count" {
			sel += fmt.Sprintf("COUNT([%s]), ", schema.Pks[table.name])
			agg += "'count', count, "
			continue
		}

		sel += fmt.Sprintf("[%s].[%s], ", table.name, col.name)
		if col.alias != "" {
			agg += fmt.Sprintf("'%s', [%s].[%s], ", col.alias, table.name, col.name)
		} else {
			agg += fmt.Sprintf("'%s', [%s].[%s], ", col.name, table.name, col.name)
		}

	}

	if !includesFk {
		sel += fmt.Sprintf("[%s].[%s], ", fk.Table, fk.From)
	}

	for _, tbl := range table.joins {
		if tbl.alias != "" {
			agg += fmt.Sprintf("'%s', json([%s]), ", tbl.alias, tbl.name)
		} else {
			agg += fmt.Sprintf("'%s', json([%s]), ", tbl.name, tbl.name)
		}
		query, aggs, err := schema.buildSelCurr(*tbl, table.name)
		if err != nil {
			return "", "", err
		}
		var fk Fk
		for _, key := range schema.Fks {
			if key.References == table.name && key.Table == tbl.name {
				fk = key
			}
		}
		if fk == (Fk{}) {
			return "", "", fmt.Errorf("no relationship exists in the schema cache between %s and %s", table.name, tbl.name)
		}

		sel += fmt.Sprintf("json_group_array(json_object(%s)) FILTER (WHERE [%s].[%s] IS NOT NULL) AS [%s], ", aggs, fk.Table, fk.From, tbl.name)

		if tbl.inner {
			joins += "INNER "
		} else {
			joins += "LEFT "
		}

		joins += fmt.Sprintf("JOIN (%s) AS [%s] ON [%s].[%s] = [%s].[%s] ", query, tbl.name, fk.References, fk.To, fk.Table, fk.From)

	}

	return "SELECT " + sel[:len(sel)-2] + fmt.Sprintf(" FROM [%s] ", table.name) + joins, agg[:len(agg)-2], nil
}

func (schema SchemaCache) parseSelect(param string, table string) (Table, error) {
	tbl := Table{table, "", false, nil, nil, nil}
	currTbl := &tbl
	currStr := ""
	alias := ""
	inner := false
	quoted := false
	escaped := false

	for _, v := range param {
		if escaped {
			currStr += string(v)
			escaped = false
			continue
		}
		if v == '\\' {
			escaped = true
			continue
		}
		if quoted && v != '"' {
			currStr += string(v)
			continue
		}
		switch v {
		case '"':
			quoted = !quoted
		case '(':
			err := schema.checkTbl(currStr)
			if err != nil {
				return Table{}, err
			}
			currTbl = &Table{currStr, alias, inner, nil, nil, currTbl}
			currTbl.parent.joins = append(currTbl.parent.joins, currTbl)
			currStr = ""
			alias = ""
			inner = false
		case ')':
			if currStr == "" {
				continue
			}

			err := schema.checkCol(currTbl.name, currStr)
			if err != nil {
				return Table{}, err
			}
			currTbl.columns = append(currTbl.columns, column{currStr, alias})
			currStr = ""
			alias = ""
			currTbl = currTbl.parent
		case ':':
			alias = currStr
			currStr = ""
		case '!':
			inner = true
		case ',':
			if currStr == "" {
				continue
			}

			err := schema.checkCol(currTbl.name, currStr)
			if err != nil {
				return Table{}, err
			}
			currTbl.columns = append(currTbl.columns, column{currStr, alias})
			alias = ""
			currStr = ""
		default:
			currStr += string(v)
		}
	}

	if currStr == "" {
		return tbl, nil
	}

	err := schema.checkCol(currTbl.name, currStr)
	if err != nil {
		return Table{}, err
	}

	currTbl.columns = append(currTbl.columns, column{currStr, alias})

	return tbl, nil
}
