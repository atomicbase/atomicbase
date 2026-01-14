package database

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"

	"github.com/joe-ervin05/atomicbase/config"
)

func schemaFks(db *sql.DB) (map[string][]Fk, error) {
	fks := make(map[string][]Fk)

	rows, err := db.Query(`
		SELECT m.name as "table", p."table" as "references", p."from", p."to"
		FROM sqlite_master m
		JOIN pragma_foreign_key_list(m.name) p ON m.name != p."table"
		WHERE m.type = 'table';
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

		fk := Fk{table.String, references.String, from.String, to.String}
		fks[table.String] = append(fks[table.String], fk)
	}

	return fks, rows.Err()
}

// schemaFTS discovers FTS5 virtual tables and returns the base table names (without _fts suffix).
func schemaFTS(db *sql.DB) (map[string]bool, error) {
	ftsTables := make(map[string]bool)

	rows, err := db.Query(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND sql LIKE '%fts5%';
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
			ftsTables[name[:len(name)-len(FTSSuffix)]] = true
		}
	}

	return ftsTables, rows.Err()
}

func schemaCols(db *sql.DB) (map[string]Table, error) {
	tbls := make(map[string]Table)

	// First, fetch foreign keys and build a lookup map
	fks, err := schemaFks(db)
	if err != nil {
		return nil, err
	}
	// Map: "table.column" -> "refTable.refColumn"
	fkMap := make(map[string]string)
	for _, fkList := range fks {
		for _, fk := range fkList {
			key := fk.Table + "." + fk.From
			fkMap[key] = fk.References + "." + fk.To
		}
	}

	rows, err := db.Query(`
		SELECT m.name, l.name as col, l.type as colType, l.pk, l."notnull", l.dflt_value
		FROM sqlite_master m
		JOIN pragma_table_info(m.name) l
		WHERE m.type IN ('table', 'view');
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var col sql.NullString
		var colType sql.NullString
		var name sql.NullString
		var pk sql.NullInt64 // SQLite pk is 0 for non-PK, 1+ for PK position in composite keys
		var notNull sql.NullBool
		var dfltValue sql.NullString

		err := rows.Scan(&name, &col, &colType, &pk, &notNull, &dfltValue)
		if err != nil {
			return nil, err
		}

		tbl, exists := tbls[name.String]
		if !exists {
			tbl = Table{Name: name.String, Columns: make(map[string]Col)}
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

		tbl.Columns[col.String] = newCol

		// pk > 0 means this column is part of the primary key
		// For composite keys, pk indicates position (1, 2, etc.)
		// We only track the first column as the "primary key" for simplicity
		if pk.Int64 == 1 {
			tbl.Pk = col.String
		}

		tbls[name.String] = tbl
	}

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
		client, err = sql.Open("sqlite3", "file:"+config.Cfg.PrimaryDBPath)
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

// SearchFks searches for a foreign key from table to references.
// Returns the Fk and true if found, or empty Fk and false if not found.
func (schema SchemaCache) SearchFks(table string, references string) (Fk, bool) {
	fks, exists := schema.Fks[table]
	if !exists {
		return Fk{}, false
	}
	for _, fk := range fks {
		if fk.References == references {
			return fk, true
		}
	}
	return Fk{}, false
}

// SearchTbls searches for a table by name.
// Returns the Table or ErrTableNotFound if not found.
func (schema SchemaCache) SearchTbls(table string) (Table, error) {
	tbl, exists := schema.Tables[table]
	if !exists {
		return Table{}, TableNotFoundErr(table)
	}
	return tbl, nil
}

// SearchCols searches a column by name.
// Returns the Col or ErrColumnNotFound if not found.
func (tbl Table) SearchCols(col string) (Col, error) {
	c, exists := tbl.Columns[col]
	if !exists {
		return Col{}, ColumnNotFoundErr(tbl.Name, col)
	}
	return c, nil
}

// HasFTSIndex checks if a table has an FTS5 index.
func (schema SchemaCache) HasFTSIndex(table string) bool {
	return schema.FTSTables[table]
}
