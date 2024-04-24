package daos

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"log"
)

func schemaFks(db *sql.DB) ([]Fk, error) {

	var fks []Fk

	rows, err := db.Query(`
		SELECT m.name as "table", p."table" as "references", p."from", p."to"
		FROM sqlite_master m
		JOIN pragma_foreign_key_list(m.name) p ON m.name != p."table"
		WHERE m.type = 'table'
		ORDER BY "table" ASC, "references" ASC, "from" ASC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var from, to, references, table sql.NullString

		rows.Scan(&table, &references, &from, &to)

		fks = append(fks, Fk{table.String, references.String, from.String, to.String})

	}

	return fks, err
}

func schemaCols(db *sql.DB) ([]Table, error) {

	var tbls []Table

	rows, err := db.Query(`
		SELECT m.name, l.name as col, l.type as colType, l.pk
		FROM sqlite_master m
		JOIN pragma_table_info(m.name) l
		WHERE m.type = 'table'
		ORDER BY m.name ASC, col ASC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var currTbl Table
	firstRow := true

	for rows.Next() {
		var col sql.NullString
		var colType sql.NullString
		var name sql.NullString
		var pk sql.NullBool

		rows.Scan(&name, &col, &colType, &pk)

		if currTbl.Name != name.String {
			if firstRow {
				currTbl.Name = name.String
				firstRow = false
			} else {
				tbls = append(tbls, currTbl)
				currTbl = Table{name.String, "", nil}
			}
		}

		currTbl.Columns = append(currTbl.Columns, Col{col.String, colType.String})

		if pk.Bool {
			currTbl.Pk = col.String
		}
	}

	tbls = append(tbls, currTbl)

	return tbls, rows.Err()

}

func (dao Database) saveSchema() error {
	var client *sql.DB
	var err error

	if dao.id == 1 {
		client = dao.Client
	} else {
		client, err = sql.Open("libsql", "file:atomicdata/primary.db")
		if err != nil {
			log.Fatal(err)
		}

		defer client.Close()
	}

	err = client.Ping()

	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err = enc.Encode(dao.Schema)
	if err != nil {
		return err
	}

	_, err = client.Exec("UPDATE databases SET schema = ? WHERE id = ?", buf.Bytes(), dao.id)

	return err
}

func loadSchema(data []byte) (SchemaCache, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	var schema SchemaCache

	err := dec.Decode(&schema)

	return schema, err

}

func (schema SchemaCache) SearchFks(table string, references string) int {
	left, right := 0, len(schema.Fks)-1

	for left <= right {
		mid := (left + right) / 2
		midValue := schema.Fks[mid]

		if midValue.Table == table && midValue.References == references {
			return mid
		} else if midValue.Table < table {
			left = mid + 1
		} else if midValue.Table == table && midValue.References < references {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	return -1
}

func (schema SchemaCache) SearchTbls(table string) (Table, error) {
	left, right := 0, len(schema.Tables)-1

	for left <= right {
		mid := (left + right) / 2
		midValue := schema.Tables[mid].Name
		if midValue == table {
			return schema.Tables[mid], nil
		} else if midValue < table {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	return Table{}, fmt.Errorf("table %s does not exist. Your schema cache may be stale", table)
}

func (tbl Table) SearchCols(col string) (Col, error) {

	left, right := 0, len(tbl.Columns)-1

	for left <= right {
		mid := (left + right) / 2
		midValue := tbl.Columns[mid].Name
		if midValue == col {
			return tbl.Columns[mid], nil
		} else if midValue < col {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	return Col{}, fmt.Errorf("column %s does not exist on table %s. Your schema cache may be stale", col, tbl.Name)
}
