package daos

import (
	"fmt"
	"strings"
)

// Relation represents a table with optional column selections and joins.
type Relation struct {
	name    string
	alias   string
	inner   bool
	columns []column
	joins   []*Relation
	parent  *Relation
}

type column struct {
	name  string
	alias string
}

// findForeignKey searches for a foreign key relationship between two tables using binary search.
func (schema SchemaCache) findForeignKey(table, references string) Fk {
	idx := schema.SearchFks(table, references)
	if idx == -1 {
		return Fk{}
	}
	return schema.Fks[idx]
}

// buildSelect constructs a SELECT query with JSON aggregation for the root relation.
func (schema SchemaCache) buildSelect(rel Relation) (string, string, error) {
	agg := ""
	sel := ""
	joins := ""

	if rel.columns == nil && rel.joins == nil {
		rel.columns = []column{{"*", ""}}
	}

	tbl, err := schema.SearchTbls(rel.name)
	if err != nil {
		return "", "", err
	}

	for _, col := range rel.columns {
		if col.name == "*" {
			sel += "*, "
			for _, col := range tbl.Columns {
				if strings.EqualFold(col.Type, ColTypeBlob) {
					continue
				}
				agg += fmt.Sprintf("'%s', [%s], ", col.Name, col.Name)
			}
			continue
		}

		column, err := tbl.SearchCols(col.name)
		if err != nil {
			return "", "", err
		}

		if strings.EqualFold(column.Type, ColTypeBlob) {
			continue
		}

		if col.name == "count" {
			sel += fmt.Sprintf("COUNT([%s]) as [count], ", tbl.Pk)
			agg += "'count', [count], "
			continue
		}

		sel += fmt.Sprintf("[%s].[%s], ", rel.name, col.name)
		if col.alias != "" {
			agg += fmt.Sprintf("'%s', [%s], ", col.alias, col.name)
		} else {
			agg += fmt.Sprintf("'%s', [%s], ", col.name, col.name)
		}
	}

	for _, tbl := range rel.joins {
		if tbl.alias != "" {
			agg += fmt.Sprintf("'%s', json([%s]), ", tbl.alias, tbl.name)
		} else {
			agg += fmt.Sprintf("'%s', json([%s]), ", tbl.name, tbl.name)
		}
		query, aggs, err := schema.buildSelCurr(*tbl, rel.name)
		if err != nil {
			return "", "", err
		}

		fk := schema.findForeignKey(tbl.name, rel.name)
		if fk == (Fk{}) {
			return "", "", NoRelationshipErr(rel.name, tbl.name)
		}

		sel += fmt.Sprintf("json_group_array(json_object(%s)) FILTER (WHERE [%s].[%s] IS NOT NULL) AS [%s], ", aggs, fk.Table, fk.From, tbl.name)

		if tbl.inner {
			joins += "INNER "
		} else {
			joins += "LEFT "
		}

		joins += fmt.Sprintf("JOIN (%s) AS [%s] ON [%s].[%s] = [%s].[%s] ", query, tbl.name, fk.References, fk.To, fk.Table, fk.From)
	}

	return "SELECT " + sel[:len(sel)-2] + fmt.Sprintf(" FROM [%s] ", rel.name) + joins, agg[:len(agg)-2], nil
}

// buildSelCurr constructs a SELECT query for a nested/joined relation.
func (schema SchemaCache) buildSelCurr(rel Relation, joinedOn string) (string, string, error) {
	var sel string
	var joins string
	var agg string
	includesFk := false
	var fk Fk

	if rel.columns == nil && rel.joins == nil {
		rel.columns = []column{{"*", ""}}
	}

	tbl, err := schema.SearchTbls(rel.name)
	if err != nil {
		return "", "", err
	}

	if joinedOn != "" {
		fk = schema.findForeignKey(rel.name, joinedOn)
	}

	for _, col := range rel.columns {
		if joinedOn != "" && fk.Table == rel.name && fk.From == col.name {
			includesFk = true
		}

		if col.name == "*" {
			sel += "*, "
			for _, col := range tbl.Columns {
				if strings.EqualFold(col.Type, ColTypeBlob) {
					continue
				}
				agg += fmt.Sprintf("'%s', [%s].[%s], ", col.Name, rel.name, col.Name)
			}
			continue
		}

		column, err := tbl.SearchCols(col.name)
		if err != nil {
			return "", "", err
		}

		if strings.EqualFold(column.Type, ColTypeBlob) {
			continue
		}

		if col.name == "count" {
			sel += fmt.Sprintf("COUNT([%s]), ", tbl.Pk)
			agg += "'count', count, "
			continue
		}

		sel += fmt.Sprintf("[%s].[%s], ", rel.name, col.name)
		if col.alias != "" {
			agg += fmt.Sprintf("'%s', [%s].[%s], ", col.alias, rel.name, col.name)
		} else {
			agg += fmt.Sprintf("'%s', [%s].[%s], ", col.name, rel.name, col.name)
		}
	}

	if !includesFk {
		sel += fmt.Sprintf("[%s].[%s], ", fk.Table, fk.From)
	}

	for _, tbl := range rel.joins {
		if tbl.alias != "" {
			agg += fmt.Sprintf("'%s', json([%s]), ", tbl.alias, tbl.name)
		} else {
			agg += fmt.Sprintf("'%s', json([%s]), ", tbl.name, tbl.name)
		}
		query, aggs, err := schema.buildSelCurr(*tbl, rel.name)
		if err != nil {
			return "", "", err
		}

		nestedFk := schema.findForeignKey(tbl.name, rel.name)
		if nestedFk == (Fk{}) {
			return "", "", NoRelationshipErr(rel.name, tbl.name)
		}

		sel += fmt.Sprintf("json_group_array(json_object(%s)) FILTER (WHERE [%s].[%s] IS NOT NULL) AS [%s], ", aggs, nestedFk.Table, nestedFk.From, tbl.name)

		if tbl.inner {
			joins += "INNER "
		} else {
			joins += "LEFT "
		}

		joins += fmt.Sprintf("JOIN (%s) AS [%s] ON [%s].[%s] = [%s].[%s] ", query, tbl.name, nestedFk.References, nestedFk.To, nestedFk.Table, nestedFk.From)
	}

	return "SELECT " + sel[:len(sel)-2] + fmt.Sprintf(" FROM [%s] ", rel.name) + joins, agg[:len(agg)-2], nil
}

// parseSelect parses a select parameter string into a Relation tree.
// Syntax: "col1,col2,related_table(col1,col2),other_table(!)"
//   - Parentheses denote related tables (joins)
//   - ! marks an inner join
//   - : provides an alias (e.g., "alias:column" or "alias:table()")
//   - Quotes allow special characters in names
//   - Backslash escapes the next character
func parseSelect(param string, table string) Relation {
	tbl := Relation{table, "", false, nil, nil, nil}
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
			currTbl = &Relation{currStr, alias, inner, nil, nil, currTbl}
			currTbl.parent.joins = append(currTbl.parent.joins, currTbl)
			currStr = ""
			alias = ""
			inner = false
		case ')':
			if currStr != "" {
				currTbl.columns = append(currTbl.columns, column{currStr, alias})
			}
			currTbl = currTbl.parent
			currStr = ""
			alias = ""
		case ':':
			alias = currStr
			currStr = ""
		case '!':
			inner = true
		case ',':
			if currStr == "" {
				continue
			}
			currTbl.columns = append(currTbl.columns, column{currStr, alias})
			alias = ""
			currStr = ""
		default:
			currStr += string(v)
		}
	}

	if currStr == "" {
		return tbl
	}

	currTbl.columns = append(currTbl.columns, column{currStr, alias})

	return tbl
}
