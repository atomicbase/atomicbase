package database

import (
	"fmt"
	"strings"

	"github.com/joe-ervin05/atomicbase/config"
)

// sanitizeJSONKey validates and escapes a string for use as a JSON object key in SQL.
// Returns an error if the key contains invalid characters.
func sanitizeJSONKey(key string) (string, error) {
	if key == "" {
		return "", nil
	}
	if err := ValidateIdentifier(key); err != nil {
		return "", fmt.Errorf("invalid alias: %w", err)
	}
	// Escape single quotes for SQL string literal safety
	return strings.ReplaceAll(key, "'", "''"), nil
}

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

// findForeignKey searches for a foreign key relationship between two tables.
func (schema SchemaCache) findForeignKey(table, references string) Fk {
	fk, _ := schema.SearchFks(table, references)
	return fk
}

// relationDepth calculates the maximum nesting depth of a Relation tree.
func relationDepth(rel *Relation) int {
	if rel == nil || len(rel.joins) == 0 {
		return 1
	}
	maxChild := 0
	for _, join := range rel.joins {
		if d := relationDepth(join); d > maxChild {
			maxChild = d
		}
	}
	return 1 + maxChild
}

// buildSelect constructs a SELECT query with JSON aggregation for the root relation.
func (schema SchemaCache) buildSelect(rel Relation) (string, string, error) {
	// Check query depth limit
	if depth := relationDepth(&rel); depth > config.Cfg.MaxQueryDepth {
		return "", "", fmt.Errorf("%w: depth %d exceeds limit %d", ErrQueryTooDeep, depth, config.Cfg.MaxQueryDepth)
	}

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
			for _, c := range tbl.Columns {
				if strings.EqualFold(c.Type, ColTypeBlob) {
					continue
				}
				agg += fmt.Sprintf("'%s', [%s], ", c.Name, c.Name)
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

		sel += fmt.Sprintf("[%s].[%s], ", rel.name, col.name)
		if col.alias != "" {
			sanitized, err := sanitizeJSONKey(col.alias)
			if err != nil {
				return "", "", err
			}
			agg += fmt.Sprintf("'%s', [%s], ", sanitized, col.name)
		} else {
			agg += fmt.Sprintf("'%s', [%s], ", col.name, col.name)
		}
	}

	for _, joinTbl := range rel.joins {
		if joinTbl.alias != "" {
			sanitized, err := sanitizeJSONKey(joinTbl.alias)
			if err != nil {
				return "", "", err
			}
			agg += fmt.Sprintf("'%s', json([%s]), ", sanitized, joinTbl.name)
		} else {
			agg += fmt.Sprintf("'%s', json([%s]), ", joinTbl.name, joinTbl.name)
		}
		query, aggs, err := schema.buildSelCurr(*joinTbl, rel.name)
		if err != nil {
			return "", "", err
		}

		fk := schema.findForeignKey(joinTbl.name, rel.name)
		if fk == (Fk{}) {
			return "", "", NoRelationshipErr(rel.name, joinTbl.name)
		}

		sel += fmt.Sprintf("json_group_array(json_object(%s)) FILTER (WHERE [%s].[%s] IS NOT NULL) AS [%s], ", aggs, fk.Table, fk.From, joinTbl.name)

		if joinTbl.inner {
			joins += "INNER "
		} else {
			joins += "LEFT "
		}

		joins += fmt.Sprintf("JOIN (%s) AS [%s] ON [%s].[%s] = [%s].[%s] ", query, joinTbl.name, fk.References, fk.To, fk.Table, fk.From)
	}

	query := "SELECT " + sel[:len(sel)-2] + fmt.Sprintf(" FROM [%s] ", rel.name) + joins

	// When there are joins, we need GROUP BY on root table columns to properly aggregate nested relations
	if len(rel.joins) > 0 {
		var rootGroupBy string
		for _, col := range rel.columns {
			if col.name != "*" {
				rootGroupBy += fmt.Sprintf("[%s].[%s], ", rel.name, col.name)
			} else {
				// Group by all columns of the root table
				for _, c := range tbl.Columns {
					rootGroupBy += fmt.Sprintf("[%s].[%s], ", rel.name, c.Name)
				}
			}
		}
		// Also group by rowid if table has no explicit PK
		if tbl.Pk == "" {
			rootGroupBy += fmt.Sprintf("[%s].[rowid], ", rel.name)
		} else {
			rootGroupBy += fmt.Sprintf("[%s].[%s], ", rel.name, tbl.Pk)
		}
		if rootGroupBy != "" {
			query += "GROUP BY " + rootGroupBy[:len(rootGroupBy)-2] + " "
		}
	}

	return query, agg[:len(agg)-2], nil
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
			for _, c := range tbl.Columns {
				if strings.EqualFold(c.Type, ColTypeBlob) {
					continue
				}
				agg += fmt.Sprintf("'%s', [%s].[%s], ", c.Name, rel.name, c.Name)
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

		sel += fmt.Sprintf("[%s].[%s], ", rel.name, col.name)
		if col.alias != "" {
			sanitized, err := sanitizeJSONKey(col.alias)
			if err != nil {
				return "", "", err
			}
			agg += fmt.Sprintf("'%s', [%s].[%s], ", sanitized, rel.name, col.name)
		} else {
			agg += fmt.Sprintf("'%s', [%s].[%s], ", col.name, rel.name, col.name)
		}
	}

	if !includesFk && fk.Table != "" {
		sel += fmt.Sprintf("[%s].[%s], ", fk.Table, fk.From)
	}

	for _, joinTbl := range rel.joins {
		if joinTbl.alias != "" {
			sanitized, err := sanitizeJSONKey(joinTbl.alias)
			if err != nil {
				return "", "", err
			}
			agg += fmt.Sprintf("'%s', json([%s]), ", sanitized, joinTbl.name)
		} else {
			agg += fmt.Sprintf("'%s', json([%s]), ", joinTbl.name, joinTbl.name)
		}
		query, aggs, err := schema.buildSelCurr(*joinTbl, rel.name)
		if err != nil {
			return "", "", err
		}

		nestedFk := schema.findForeignKey(joinTbl.name, rel.name)
		if nestedFk == (Fk{}) {
			return "", "", NoRelationshipErr(rel.name, joinTbl.name)
		}

		sel += fmt.Sprintf("json_group_array(json_object(%s)) FILTER (WHERE [%s].[%s] IS NOT NULL) AS [%s], ", aggs, nestedFk.Table, nestedFk.From, joinTbl.name)

		if joinTbl.inner {
			joins += "INNER "
		} else {
			joins += "LEFT "
		}

		joins += fmt.Sprintf("JOIN (%s) AS [%s] ON [%s].[%s] = [%s].[%s] ", query, joinTbl.name, nestedFk.References, nestedFk.To, nestedFk.Table, nestedFk.From)
	}

	query := "SELECT " + sel[:len(sel)-2] + fmt.Sprintf(" FROM [%s] ", rel.name) + joins

	return query, agg[:len(agg)-2], nil
}

// parseSelect parses a select parameter string into a Relation tree.
// Syntax: "col1,col2,related_table(col1,col2),other_table(!)"
//   - Parentheses denote related tables (joins) when preceded by a table name
//   - ! marks an inner join
//   - : provides an alias (e.g., "alias:column")
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
			// It's a relation/join
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
