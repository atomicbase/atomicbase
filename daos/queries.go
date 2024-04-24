package daos

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
)

type Queries interface {
	Select(string, url.Values) ([]byte, error)
	Update(string, url.Values, io.ReadCloser) ([]byte, error)
	InsertSingle(string, url.Values, io.ReadCloser) ([]byte, error)
	Upsert(string, url.Values, io.ReadCloser) ([]byte, error)
	Delete(string, url.Values) ([]byte, error)
}

func (dao Database) Select(relation string, params url.Values) ([]byte, error) {
	if dao.id == 1 && relation == "databases" {
		return nil, errors.New("cannot query table databases")
	}

	table, err := dao.Schema.SearchTbls(relation)
	if err != nil {
		return nil, err
	}

	var sel string

	if params.Has("select") {
		sel = params["select"][0]
		params.Del("select")
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

	if params["order"] != nil {
		order, err := table.BuildOrder(params["order"][0])
		if err != nil {
			return nil, err
		}

		query += order
	}

	fmt.Println(query)

	row := dao.Client.QueryRow(fmt.Sprintf("SELECT json_group_array(json_object(%s)) AS data FROM (%s)", agg, query), args...)
	if row.Err() != nil {
		return nil, row.Err()
	}

	var res []byte

	row.Scan(&res)

	return res, nil
}

func (dao Database) Update(relation string, params url.Values, body io.ReadCloser) ([]byte, error) {
	if dao.id == 1 && relation == "databases" {
		return nil, errors.New("cannot query table databases")
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

	if params["select"] != nil {
		selQuery, err := table.buildReturning(params["select"][0])
		if err != nil {
			return nil, err
		}

		query += selQuery

		fmt.Println(query)

		return dao.QueryJSON(query, args...)
	}

	_, err = dao.Client.Exec(query, args...)

	return nil, err
}

func (dao Database) Insert(relation string, params url.Values, body io.ReadCloser) ([]byte, error) {
	if dao.id == 1 && relation == "databases" {
		return nil, errors.New("cannot query table databases")
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

	if params.Has("select") {
		selQuery, err := table.buildReturning(params["select"][0])
		if err != nil {
			return nil, err
		}

		query += selQuery

		return dao.QueryJSON(query, args...)
	}

	_, err = dao.Client.Exec(query, args...)

	return []byte("inserted row"), err
}

func (dao Database) Upsert(relation string, params url.Values, body io.ReadCloser) ([]byte, error) {
	if dao.id == 1 && relation == "databases" {
		return nil, errors.New("cannot query table databases")
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

	if params.Has("select") {
		selQuery, err := table.buildReturning(params["select"][0])
		if err != nil {
			return nil, err
		}

		query += selQuery

		return dao.QueryJSON(query, args...)
	}

	fmt.Println(query)

	_, err = dao.Client.Exec(query, args...)

	return []byte("inserted rows"), err
}

func (dao Database) Delete(relation string, params url.Values) ([]byte, error) {
	if dao.id == 1 && relation == "databases" {
		return nil, errors.New("cannot query table databases")
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
		return nil, errors.New("all DELETEs require a where clause")
	}
	query += where

	if params["select"] != nil {
		selQuery, err := table.buildReturning(params["select"][0])
		if err != nil {
			return nil, err
		}

		query += selQuery

		return dao.QueryJSON(query, args...)
	}

	_, err = dao.Client.Exec(query, args...)

	return []byte("deleted"), err
}
