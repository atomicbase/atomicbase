package database

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"

	"github.com/joe-ervin05/atomicbase/config"
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

		err := rows.Scan(&table, &references, &from, &to)
		if err != nil {
			return nil, err
		}

		fks = append(fks, Fk{table.String, references.String, from.String, to.String})

	}

	return fks, err
}

// schemaFTS discovers FTS5 virtual tables and returns the base table names (without _fts suffix).
func schemaFTS(db *sql.DB) ([]string, error) {
	var ftsTables []string

	rows, err := db.Query(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND sql LIKE '%fts5%'
		ORDER BY name ASC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		// Remove _fts suffix to get base table name
		if len(name) > len(FTSSuffix) && name[len(name)-len(FTSSuffix):] == FTSSuffix {
			ftsTables = append(ftsTables, name[:len(name)-len(FTSSuffix)])
		}
	}

	return ftsTables, rows.Err()
}

func schemaCols(db *sql.DB) ([]Table, error) {

	var tbls []Table

	// First, fetch foreign keys and build a lookup map
	fks, err := schemaFks(db)
	if err != nil {
		return nil, err
	}
	// Map: "table.column" -> "refTable.refColumn"
	fkMap := make(map[string]string)
	for _, fk := range fks {
		key := fk.Table + "." + fk.From
		fkMap[key] = fk.References + "." + fk.To
	}

	rows, err := db.Query(`
		SELECT m.name, l.name as col, l.type as colType, l.pk, l."notnull", l.dflt_value
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
		var notNull sql.NullBool
		var dfltValue sql.NullString

		err := rows.Scan(&name, &col, &colType, &pk, &notNull, &dfltValue)
		if err != nil {
			return nil, err
		}

		if currTbl.Name != name.String {
			if firstRow {
				currTbl.Name = name.String
				firstRow = false
			} else {
				tbls = append(tbls, currTbl)
				currTbl = Table{Name: name.String}
			}
		}

		newCol := Col{
			Name:    col.String,
			Type:    colType.String,
			NotNull: notNull.Bool,
		}

		// Parse default value if present
		if dfltValue.Valid && dfltValue.String != "" {
			newCol.Default = parseDefaultValue(dfltValue.String)
		}

		// Check for foreign key reference
		if ref, ok := fkMap[name.String+"."+col.String]; ok {
			newCol.References = ref
		}

		currTbl.Columns = append(currTbl.Columns, newCol)

		if pk.Bool {
			currTbl.Pk = col.String
		}
	}

	tbls = append(tbls, currTbl)

	return tbls, rows.Err()

}

// parseDefaultValue converts SQLite's default value string to appropriate Go type
func parseDefaultValue(val string) any {
	// Remove quotes from string defaults
	if len(val) >= 2 && ((val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"')) {
		return val[1 : len(val)-1]
	}
	// Try to parse as number
	if val == "NULL" || val == "null" {
		return nil
	}
	// Return as-is for expressions like CURRENT_TIMESTAMP
	return val
}

func (dao *Database) saveSchema() error {
	var client *sql.DB
	var err error

	if dao.id == 1 {
		client = dao.Client
	} else {
		client, err = sql.Open("libsql", "file:"+config.Cfg.PrimaryDBPath)
		if err != nil {
			return fmt.Errorf("failed to open primary database for schema save: %w", err)
		}
		defer client.Close()
	}

	err = client.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping database for schema save: %w", err)
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err = enc.Encode(dao.Schema)
	if err != nil {
		return err
	}

	_, err = client.Exec(fmt.Sprintf("UPDATE %s SET schema = ? WHERE id = ?", ReservedTableDatabases), buf.Bytes(), dao.id)
	return err
}

func loadSchema(data []byte) (SchemaCache, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	var schema SchemaCache

	err := dec.Decode(&schema)

	return schema, err

}

// SearchFks performs binary search for a foreign key by table and references.
// Returns the index in schema.Fks or -1 if not found.
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

// SearchTbls performs binary search for a table by name.
// Returns the Table or ErrTableNotFound if not found.
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

	return Table{}, TableNotFoundErr(table)
}

// SearchCols performs binary search for a column by name.
// Returns the Col or ErrColumnNotFound if not found.
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

	return Col{}, ColumnNotFoundErr(tbl.Name, col)
}

// HasFTSIndex checks if a table has an FTS5 index using binary search.
func (schema SchemaCache) HasFTSIndex(table string) bool {
	left, right := 0, len(schema.FTSTables)-1

	for left <= right {
		mid := (left + right) / 2
		midValue := schema.FTSTables[mid]
		if midValue == table {
			return true
		} else if midValue < table {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	return false
}
