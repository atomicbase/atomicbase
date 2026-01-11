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
	name      string
	alias     string
	aggregate string // count, sum, avg, min, max (empty for regular columns)
}

// findForeignKey searches for a foreign key relationship between two tables using binary search.
func (schema SchemaCache) findForeignKey(table, references string) Fk {
	idx := schema.SearchFks(table, references)
	if idx == -1 {
		return Fk{}
	}
	return schema.Fks[idx]
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
	groupBy := ""
	hasAggregate := false

	if rel.columns == nil && rel.joins == nil {
		rel.columns = []column{{"*", "", ""}}
	}

	tbl, err := schema.SearchTbls(rel.name)
	if err != nil {
		return "", "", err
	}

	// First pass: check if we have any aggregates
	for _, col := range rel.columns {
		if col.aggregate != "" {
			hasAggregate = true
			break
		}
	}

	for _, col := range rel.columns {
		// Handle aggregate functions
		if col.aggregate != "" {
			aggFunc := strings.ToUpper(col.aggregate)
			alias := col.alias
			if alias == "" {
				alias = autoAggAlias(col.aggregate, col.name)
			}
			sanitizedAlias, err := sanitizeJSONKey(alias)
			if err != nil {
				return "", "", err
			}

			if col.name == "*" {
				sel += fmt.Sprintf("%s(*) AS [%s], ", aggFunc, alias)
			} else {
				// Validate column exists (unless it's *)
				if _, err := tbl.SearchCols(col.name); err != nil {
					return "", "", err
				}
				sel += fmt.Sprintf("%s([%s].[%s]) AS [%s], ", aggFunc, rel.name, col.name, alias)
			}
			agg += fmt.Sprintf("'%s', [%s], ", sanitizedAlias, alias)
			continue
		}

		if col.name == "*" {
			sel += "*, "
			for _, c := range tbl.Columns {
				if strings.EqualFold(c.Type, ColTypeBlob) {
					continue
				}
				agg += fmt.Sprintf("'%s', [%s], ", c.Name, c.Name)
				// If we have aggregates, non-aggregate columns need GROUP BY
				if hasAggregate {
					groupBy += fmt.Sprintf("[%s].[%s], ", rel.name, c.Name)
				}
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

		// Non-aggregate columns need GROUP BY when mixed with aggregates
		if hasAggregate {
			groupBy += fmt.Sprintf("[%s].[%s], ", rel.name, col.name)
		}
	}

	for _, tbl := range rel.joins {
		if tbl.alias != "" {
			sanitized, err := sanitizeJSONKey(tbl.alias)
			if err != nil {
				return "", "", err
			}
			agg += fmt.Sprintf("'%s', json([%s]), ", sanitized, tbl.name)
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

	query := "SELECT " + sel[:len(sel)-2] + fmt.Sprintf(" FROM [%s] ", rel.name) + joins

	// Add GROUP BY if we have aggregates mixed with regular columns
	if groupBy != "" {
		query += "GROUP BY " + groupBy[:len(groupBy)-2] + " "
	}

	return query, agg[:len(agg)-2], nil
}

// buildSelCurr constructs a SELECT query for a nested/joined relation.
func (schema SchemaCache) buildSelCurr(rel Relation, joinedOn string) (string, string, error) {
	var sel string
	var joins string
	var agg string
	var groupBy string
	includesFk := false
	hasAggregate := false
	var fk Fk

	if rel.columns == nil && rel.joins == nil {
		rel.columns = []column{{"*", "", ""}}
	}

	tbl, err := schema.SearchTbls(rel.name)
	if err != nil {
		return "", "", err
	}

	if joinedOn != "" {
		fk = schema.findForeignKey(rel.name, joinedOn)
	}

	// First pass: check if we have any aggregates
	for _, col := range rel.columns {
		if col.aggregate != "" {
			hasAggregate = true
			break
		}
	}

	for _, col := range rel.columns {
		if joinedOn != "" && fk.Table == rel.name && fk.From == col.name {
			includesFk = true
		}

		// Handle aggregate functions
		if col.aggregate != "" {
			aggFunc := strings.ToUpper(col.aggregate)
			alias := col.alias
			if alias == "" {
				alias = autoAggAlias(col.aggregate, col.name)
			}
			sanitizedAlias, err := sanitizeJSONKey(alias)
			if err != nil {
				return "", "", err
			}

			if col.name == "*" {
				sel += fmt.Sprintf("%s(*) AS [%s], ", aggFunc, alias)
			} else {
				if _, err := tbl.SearchCols(col.name); err != nil {
					return "", "", err
				}
				sel += fmt.Sprintf("%s([%s].[%s]) AS [%s], ", aggFunc, rel.name, col.name, alias)
			}
			agg += fmt.Sprintf("'%s', [%s], ", sanitizedAlias, alias)
			continue
		}

		if col.name == "*" {
			sel += "*, "
			for _, c := range tbl.Columns {
				if strings.EqualFold(c.Type, ColTypeBlob) {
					continue
				}
				agg += fmt.Sprintf("'%s', [%s].[%s], ", c.Name, rel.name, c.Name)
				if hasAggregate {
					groupBy += fmt.Sprintf("[%s].[%s], ", rel.name, c.Name)
				}
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

		if hasAggregate {
			groupBy += fmt.Sprintf("[%s].[%s], ", rel.name, col.name)
		}
	}

	if !includesFk && fk.Table != "" {
		sel += fmt.Sprintf("[%s].[%s], ", fk.Table, fk.From)
	}

	for _, tbl := range rel.joins {
		if tbl.alias != "" {
			sanitized, err := sanitizeJSONKey(tbl.alias)
			if err != nil {
				return "", "", err
			}
			agg += fmt.Sprintf("'%s', json([%s]), ", sanitized, tbl.name)
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

	query := "SELECT " + sel[:len(sel)-2] + fmt.Sprintf(" FROM [%s] ", rel.name) + joins

	if groupBy != "" {
		query += "GROUP BY " + groupBy[:len(groupBy)-2] + " "
	}

	return query, agg[:len(agg)-2], nil
}

// isAggregateFunc checks if a string is a valid aggregate function name.
func isAggregateFunc(name string) bool {
	switch strings.ToLower(name) {
	case AggCount, AggSum, AggAvg, AggMin, AggMax:
		return true
	}
	return false
}

// parseAggregate parses a string like "sum(price)" or "count(*)" into aggregate and column name.
// Returns aggregate function name, column name, and whether it was an aggregate.
func parseAggregate(s string) (agg string, col string, isAgg bool) {
	parenIdx := strings.Index(s, "(")
	if parenIdx == -1 {
		return "", s, false
	}

	funcName := strings.ToLower(s[:parenIdx])
	if !isAggregateFunc(funcName) {
		return "", s, false
	}

	// Extract column name from within parentheses
	closeIdx := strings.LastIndex(s, ")")
	if closeIdx == -1 || closeIdx <= parenIdx {
		return "", s, false
	}

	colName := strings.TrimSpace(s[parenIdx+1 : closeIdx])
	return funcName, colName, true
}

// parseSelect parses a select parameter string into a Relation tree.
// Syntax: "col1,col2,related_table(col1,col2),other_table(!)"
//   - Parentheses denote related tables (joins) when preceded by a table name
//   - Aggregate functions: count(*), sum(col), avg(col), min(col), max(col)
//   - ! marks an inner join
//   - : provides an alias (e.g., "alias:column" or "alias:sum(price)")
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
	parenDepth := 0      // Track nested parentheses for aggregate functions
	inAggregate := false // Whether we're inside an aggregate function

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
			// Check if this is an aggregate function
			if isAggregateFunc(currStr) {
				inAggregate = true
				parenDepth = 1
				currStr += string(v)
				continue
			}
			// Otherwise it's a relation/join
			currTbl = &Relation{currStr, alias, inner, nil, nil, currTbl}
			currTbl.parent.joins = append(currTbl.parent.joins, currTbl)
			currStr = ""
			alias = ""
			inner = false
		case ')':
			if inAggregate {
				currStr += string(v)
				parenDepth--
				if parenDepth == 0 {
					inAggregate = false
				}
				continue
			}
			if currStr != "" {
				agg, col, isAgg := parseAggregate(currStr)
				currTbl.columns = append(currTbl.columns, column{col, alias, agg})
				if isAgg && alias == "" {
					// Auto-alias aggregates: sum(price) -> sum_price or count(*) -> count
					currTbl.columns[len(currTbl.columns)-1].alias = autoAggAlias(agg, col)
				}
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
			if inAggregate {
				currStr += string(v)
				continue
			}
			if currStr == "" {
				continue
			}
			agg, col, isAgg := parseAggregate(currStr)
			newCol := column{col, alias, agg}
			if isAgg && alias == "" {
				newCol.alias = autoAggAlias(agg, col)
			}
			currTbl.columns = append(currTbl.columns, newCol)
			alias = ""
			currStr = ""
		default:
			currStr += string(v)
		}
	}

	if currStr == "" {
		return tbl
	}

	agg, col, isAgg := parseAggregate(currStr)
	newCol := column{col, alias, agg}
	if isAgg && alias == "" {
		newCol.alias = autoAggAlias(agg, col)
	}
	currTbl.columns = append(currTbl.columns, newCol)

	return tbl
}

// autoAggAlias generates a default alias for an aggregate function.
func autoAggAlias(agg, col string) string {
	if col == "*" {
		return agg
	}
	return agg + "_" + col
}
