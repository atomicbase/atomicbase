package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/joe-ervin05/atomicbase/config"
)

// SelectResult holds the result of a Select query with optional count.
type SelectResult struct {
	Data  []byte
	Count int64
}

// SelectJSON queries rows using JSON body format.
// POST /query/{table} with Prefer: operation=select
func (dao *Database) SelectJSON(ctx context.Context, relation string, query SelectQuery, includeCount bool) (SelectResult, error) {
	if err := ValidateTableName(relation); err != nil {
		return SelectResult{}, err
	}
	if dao.id == 1 && relation == ReservedTableDatabases {
		return SelectResult{}, ErrReservedTable
	}

	table, err := dao.Schema.SearchTbls(relation)
	if err != nil {
		return SelectResult{}, err
	}

	// Parse select clause
	rel, err := ParseSelectFromJSON(query.Select, relation)
	if err != nil {
		return SelectResult{}, err
	}

	// Build SELECT query
	sqlQuery, agg, err := dao.Schema.buildSelect(rel)
	if err != nil {
		return SelectResult{}, err
	}

	// Build WHERE clause
	where, args, err := table.BuildWhereFromJSON(query.Where, dao.Schema)
	if err != nil {
		return SelectResult{}, err
	}
	baseQuery := sqlQuery + where

	var result SelectResult

	// Get count if requested
	if includeCount {
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s)", baseQuery)
		row := dao.Client.QueryRowContext(ctx, countQuery, args...)
		if err := row.Scan(&result.Count); err != nil {
			return SelectResult{}, err
		}
	}

	// Add ordering
	if query.Order != nil {
		order, err := table.BuildOrderFromJSON(query.Order)
		if err != nil {
			return SelectResult{}, err
		}
		baseQuery += order
	}

	// Handle pagination
	limit := config.Cfg.DefaultLimit
	if query.Limit != nil && *query.Limit >= 0 {
		limit = *query.Limit
	}
	if config.Cfg.MaxQueryLimit > 0 && (limit > config.Cfg.MaxQueryLimit || limit == 0) {
		limit = config.Cfg.MaxQueryLimit
	}

	offset := 0
	if query.Offset != nil && *query.Offset >= 0 {
		offset = *query.Offset
	}

	if limit > 0 {
		baseQuery += fmt.Sprintf("LIMIT %d ", limit)
	}
	if offset > 0 {
		baseQuery += fmt.Sprintf("OFFSET %d ", offset)
	}

	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf("SELECT json_group_array(%s) AS data FROM (%s)", agg, baseQuery), args...)
	if err := row.Scan(&result.Data); err != nil {
		return SelectResult{}, err
	}

	return result, nil
}

// InsertJSON inserts a single row using JSON body format.
// POST /query/{table} (no Prefer header)
func (dao *Database) InsertJSON(ctx context.Context, relation string, req InsertRequest) ([]byte, error) {
	if err := ValidateTableName(relation); err != nil {
		return nil, err
	}
	if dao.id == 1 && relation == ReservedTableDatabases {
		return nil, ErrReservedTable
	}

	table, err := dao.Schema.SearchTbls(relation)
	if err != nil {
		return nil, err
	}

	if len(req.Data) == 0 {
		return nil, errors.New("insert requires at least one column")
	}

	args := make([]any, 0, len(req.Data))
	columns := ""
	values := ""

	for col, val := range req.Data {
		_, err = table.SearchCols(col)
		if err != nil {
			return nil, err
		}

		args = append(args, val)
		columns += fmt.Sprintf("[%s], ", col)
		values += "?, "
	}

	query := fmt.Sprintf("INSERT INTO [%s] (%s) VALUES (%s) ", relation, columns[:len(columns)-2], values[:len(values)-2])

	if len(req.Returning) > 0 {
		retQuery, err := table.BuildReturningFromJSON(req.Returning)
		if err != nil {
			return nil, err
		}
		query += retQuery
		return dao.QueryJSON(ctx, query, args...)
	}

	result, err := dao.Client.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	lastInsertId, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}
	return json.Marshal(map[string]any{"last_insert_id": lastInsertId})
}

// InsertIgnoreJSON inserts a single row, ignoring conflicts.
// POST /query/{table} with Prefer: on-conflict=ignore
func (dao *Database) InsertIgnoreJSON(ctx context.Context, relation string, req InsertRequest) ([]byte, error) {
	if err := ValidateTableName(relation); err != nil {
		return nil, err
	}
	if dao.id == 1 && relation == ReservedTableDatabases {
		return nil, ErrReservedTable
	}

	table, err := dao.Schema.SearchTbls(relation)
	if err != nil {
		return nil, err
	}

	if len(req.Data) == 0 {
		return nil, errors.New("insert requires at least one column")
	}

	args := make([]any, 0, len(req.Data))
	columns := ""
	values := ""

	for col, val := range req.Data {
		_, err = table.SearchCols(col)
		if err != nil {
			return nil, err
		}

		args = append(args, val)
		columns += fmt.Sprintf("[%s], ", col)
		values += "?, "
	}

	query := fmt.Sprintf("INSERT OR IGNORE INTO [%s] (%s) VALUES (%s) ", relation, columns[:len(columns)-2], values[:len(values)-2])

	if len(req.Returning) > 0 {
		retQuery, err := table.BuildReturningFromJSON(req.Returning)
		if err != nil {
			return nil, err
		}
		query += retQuery
		return dao.QueryJSON(ctx, query, args...)
	}

	result, err := dao.Client.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return json.Marshal(map[string]any{"rows_affected": rowsAffected})
}

// UpsertJSON inserts multiple rows, updating on conflict.
// POST /query/{table} with Prefer: on-conflict=replace
func (dao *Database) UpsertJSON(ctx context.Context, relation string, req UpsertRequest) ([]byte, error) {
	if err := ValidateTableName(relation); err != nil {
		return nil, err
	}
	if dao.id == 1 && relation == ReservedTableDatabases {
		return nil, ErrReservedTable
	}

	table, err := dao.Schema.SearchTbls(relation)
	if err != nil {
		return nil, err
	}

	if len(req.Data) == 0 {
		return nil, errors.New("upsert requires at least one row")
	}

	if len(req.Data[0]) == 0 {
		return nil, errors.New("upsert rows must have at least one column")
	}

	query := fmt.Sprintf("INSERT INTO [%s] ( ", relation)
	args := make([]any, len(req.Data)*len(req.Data[0]))
	vals := "( "

	colI := 0
	for col := range req.Data[0] {
		_, err := table.SearchCols(col)
		if err != nil {
			return nil, err
		}

		query += fmt.Sprintf("[%s], ", col)
		vals += "?, "

		for i, cols := range req.Data {
			args[i*len(cols)+colI] = cols[col]
		}
		colI++
	}

	vals = vals[:len(vals)-2] + "), "
	query = query[:len(query)-2] + " ) VALUES "

	for i := 0; i < len(req.Data); i++ {
		query += vals
	}

	if table.Pk == "" {
		query = query[:len(query)-2] + " ON CONFLICT(rowid) DO UPDATE SET "
	} else {
		query = query[:len(query)-2] + fmt.Sprintf(" ON CONFLICT([%s]) DO UPDATE SET ", table.Pk)
	}

	for col := range req.Data[0] {
		query += fmt.Sprintf("[%s] = excluded.[%s], ", col, col)
	}

	query = query[:len(query)-2] + " "

	if len(req.Returning) > 0 {
		retQuery, err := table.BuildReturningFromJSON(req.Returning)
		if err != nil {
			return nil, err
		}
		query += retQuery
		return dao.QueryJSON(ctx, query, args...)
	}

	result, err := dao.Client.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return json.Marshal(map[string]any{"rows_affected": rowsAffected})
}

// UpdateJSON modifies rows using JSON body format.
// PATCH /query/{table}
func (dao *Database) UpdateJSON(ctx context.Context, relation string, req UpdateRequest) ([]byte, error) {
	if err := ValidateTableName(relation); err != nil {
		return nil, err
	}
	if dao.id == 1 && relation == ReservedTableDatabases {
		return nil, ErrReservedTable
	}

	table, err := dao.Schema.SearchTbls(relation)
	if err != nil {
		return nil, err
	}

	if len(req.Data) == 0 {
		return nil, errors.New("update requires at least one column")
	}

	query := fmt.Sprintf("UPDATE [%s] SET ", relation)
	var args []any

	first := true
	for col, val := range req.Data {
		_, err = table.SearchCols(col)
		if err != nil {
			return nil, err
		}

		if !first {
			query += ", "
		}
		first = false
		query += fmt.Sprintf("[%s] = ?", col)
		args = append(args, val)
	}
	query += " "

	where, whereArgs, err := table.BuildWhereFromJSON(req.Where, dao.Schema)
	if err != nil {
		return nil, err
	}

	if where == "" {
		return nil, ErrMissingWhereClause
	}
	query += where
	args = append(args, whereArgs...)

	result, err := dao.Client.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return json.Marshal(map[string]any{"rows_affected": rowsAffected})
}

// DeleteJSON removes rows using JSON body format.
// DELETE /query/{table}
func (dao *Database) DeleteJSON(ctx context.Context, relation string, req DeleteRequest) ([]byte, error) {
	if err := ValidateTableName(relation); err != nil {
		return nil, err
	}
	if dao.id == 1 && relation == ReservedTableDatabases {
		return nil, ErrReservedTable
	}

	table, err := dao.Schema.SearchTbls(relation)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("DELETE FROM [%s] ", relation)

	where, args, err := table.BuildWhereFromJSON(req.Where, dao.Schema)
	if err != nil {
		return nil, err
	}

	if where == "" {
		return nil, ErrMissingWhereClause
	}
	query += where

	result, err := dao.Client.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return json.Marshal(map[string]any{"rows_affected": rowsAffected})
}
