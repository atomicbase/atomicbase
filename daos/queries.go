package daos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
)

// Select queries rows from a table with optional filtering, ordering, and nested relations.
// Use the "select" param for column selection (e.g., "name,cars(make,model)").
// Use the "order" param for sorting (e.g., "name:asc").
// Other params become WHERE conditions (e.g., "id=eq.1").
func (dao *Database) Select(ctx context.Context, relation string, params url.Values) ([]byte, error) {
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

	var sel string

	if params.Has(ParamSelect) {
		sel = params.Get(ParamSelect)
	} else {
		sel = "*"
	}

	tbls := parseSelect(sel, relation)

	query, agg, err := dao.Schema.buildSelect(tbls)
	if err != nil {
		return nil, err
	}

	where, args, err := table.BuildWhere(params)
	if err != nil {
		return nil, err
	}
	query += where

	if params[ParamOrder] != nil {
		order, err := table.BuildOrder(params[ParamOrder][0])
		if err != nil {
			return nil, err
		}

		query += order
	}

	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf("SELECT json_group_array(json_object(%s)) AS data FROM (%s)", agg, query), args...)
	if row.Err() != nil {
		return nil, row.Err()
	}

	var res []byte

	row.Scan(&res)

	return res, nil
}

// Update modifies rows matching the WHERE conditions from query params.
// Body should be JSON with column:value pairs to update.
// Use "select" param to return modified rows.
func (dao *Database) Update(ctx context.Context, relation string, params url.Values, body io.ReadCloser) ([]byte, error) {
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

	var cols map[string]any
	err = json.NewDecoder(body).Decode(&cols)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("UPDATE [%s] SET ", relation)
	args := make([]any, len(cols))

	colI := 0
	for col, val := range cols {

		_, err = table.SearchCols(col)
		if err != nil {
			return nil, err
		}

		if colI == len(cols)-1 {
			query += fmt.Sprintf("[%s] = ? ", col)
		} else {
			query += fmt.Sprintf("[%s] = ?, ", col)
		}
		args[colI] = val
		colI++
	}

	where, whereArgs, err := table.BuildWhere(params)
	if err != nil {
		return nil, err
	}
	query += where
	args = append(args, whereArgs...)

	if params[ParamSelect] != nil {
		selQuery, err := table.buildReturning(params[ParamSelect][0])
		if err != nil {
			return nil, err
		}

		query += selQuery

		return dao.QueryJSON(ctx, query, args...)
	}

	result, err := dao.Client.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, _ := result.RowsAffected()
	return json.Marshal(map[string]any{"rows_affected": rowsAffected})
}

// Insert adds a single row to the table.
// Body should be JSON with column:value pairs.
// Use "select" param to return the inserted row.
func (dao *Database) Insert(ctx context.Context, relation string, params url.Values, body io.ReadCloser) ([]byte, error) {
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

	var cols map[string]any

	err = json.NewDecoder(body).Decode(&cols)
	if err != nil {
		return nil, err
	}

	args := make([]any, len(cols))

	i := 0
	columns := ""
	values := ""

	for col, val := range cols {
		_, err = table.SearchCols(col)
		if err != nil {
			return nil, err
		}

		args[i] = val
		columns += fmt.Sprintf("[%s], ", col)
		values += "?, "
		i++
	}

	query := fmt.Sprintf("INSERT INTO [%s] (%s) VALUES (%s) ", relation, columns[:len(columns)-2], values[:len(values)-2])

	if params.Has(ParamSelect) {
		selQuery, err := table.buildReturning(params[ParamSelect][0])
		if err != nil {
			return nil, err
		}

		query += selQuery

		return dao.QueryJSON(ctx, query, args...)
	}

	result, err := dao.Client.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	lastInsertId, _ := result.LastInsertId()
	return json.Marshal(map[string]any{"last_insert_id": lastInsertId})
}

// Upsert inserts multiple rows, updating on primary key conflict.
// Body should be a JSON array of objects with column:value pairs.
// Use "select" param to return the upserted rows.
func (dao *Database) Upsert(ctx context.Context, relation string, params url.Values, body io.ReadCloser) ([]byte, error) {
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

	var colSlice []map[string]any

	err = json.NewDecoder(body).Decode(&colSlice)
	if err != nil {
		return nil, err
	}

	if len(colSlice) == 0 {
		return nil, errors.New("upsert requires at least one row")
	}

	if len(colSlice[0]) == 0 {
		return nil, errors.New("upsert rows must have at least one column")
	}

	query := fmt.Sprintf("INSERT INTO [%s] ( ", relation)
	args := make([]any, len(colSlice)*len(colSlice[0]))
	vals := "( "

	colI := 0
	for col := range colSlice[0] {
		_, err := table.SearchCols(col)
		if err != nil {
			return nil, err
		}

		query += fmt.Sprintf("[%s], ", col)
		vals += "?, "

		for i, cols := range colSlice {

			args[i*len(cols)+colI] = cols[col]

		}

		colI++
	}

	vals = vals[:len(vals)-2] + "), "

	query = query[:len(query)-2] + " ) VALUES "

	for i := 0; i < len(colSlice); i++ {
		query += vals

	}

	if table.Pk == "" {
		query = query[:len(query)-2] + " ON CONFLICT(rowid) DO UPDATE SET "
	} else {
		query = query[:len(query)-2] + fmt.Sprintf(" ON CONFLICT([%s]) DO UPDATE SET ", table.Pk)
	}

	for col := range colSlice[0] {
		query += fmt.Sprintf("[%s] = excluded.[%s], ", col, col)
	}

	query = query[:len(query)-2] + " "

	if params.Has(ParamSelect) {
		selQuery, err := table.buildReturning(params[ParamSelect][0])
		if err != nil {
			return nil, err
		}

		query += selQuery

		return dao.QueryJSON(ctx, query, args...)
	}

	result, err := dao.Client.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, _ := result.RowsAffected()
	return json.Marshal(map[string]any{"rows_affected": rowsAffected})
}

// Delete removes rows matching the WHERE conditions from query params.
// A WHERE clause is required (no mass deletes without conditions).
// Use "select" param to return deleted rows.
func (dao *Database) Delete(ctx context.Context, relation string, params url.Values) ([]byte, error) {
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

	where, args, err := table.BuildWhere(params)
	if err != nil {
		return nil, err
	}

	if where == "" {
		return nil, ErrMissingWhereClause
	}
	query += where

	if params[ParamSelect] != nil {
		selQuery, err := table.buildReturning(params[ParamSelect][0])
		if err != nil {
			return nil, err
		}

		query += selQuery

		return dao.QueryJSON(ctx, query, args...)
	}

	result, err := dao.Client.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, _ := result.RowsAffected()
	return json.Marshal(map[string]any{"rows_affected": rowsAffected})
}
