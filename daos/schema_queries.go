package daos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Column struct {
	Type       string `json:"type"`
	Default    any    `json:"default"`
	PrimaryKey bool   `json:"primaryKey"`
	Unique     bool   `json:"unique"`
	NotNull    bool   `json:"notNull"`
	References string `json:"references"`
	OnDelete   string `json:"onDelete"`
	OnUpdate   string `json:"onUpdate"`
}

type NewColumn struct {
	Type       string `json:"type"`
	Default    any    `json:"default"`
	NotNull    bool   `json:"notNull"`
	References string `json:"references"`
	OnDelete   string `json:"onDelete"`
	OnUpdate   string `json:"onUpdate"`
}

func (dao *Database) InvalidateSchema() error {

	cols, pks, err := schemaCols(dao.Client)
	if err != nil {
		return err
	}
	fks, err := schemaFks(dao.Client)
	if err != nil {
		return err
	}

	dao.Schema = SchemaCache{cols, pks, fks}

	return dao.saveSchema()
}

func (dao Database) GetTableSchema(table string) ([]byte, error) {
	err := dao.Schema.checkTbl(table)
	if err != nil {
		return nil, err
	}

	type fKey struct {
		Column     string `json:"column"`
		References string `json:"references"`
	}

	type tableSchema struct {
		Columns     map[string]string `json:"columns"`
		PrimaryKey  string            `json:"primaryKey"`
		ForeignKeys []fKey            `json:"foreignKeys"`
	}

	var buf bytes.Buffer

	cols := dao.Schema.Tables[table]
	primaryKey := dao.Schema.Pks[table]

	var fks []fKey

	for _, key := range dao.Schema.Fks {
		if key.Table == table {
			fks = append(fks, fKey{key.From, fmt.Sprintf("%s.%s", key.References, key.To)})
		}
	}

	err = json.NewEncoder(&buf).Encode(tableSchema{cols, primaryKey, fks})

	return buf.Bytes(), err
}

func (dao Database) AlterTable(table string, body io.ReadCloser) ([]byte, error) {
	type tblChanges struct {
		NewName       string               `json:"newName"`
		RenameColumns map[string]string    `json:"renameColumns"`
		NewColumns    map[string]NewColumn `json:"newColumns"`
		DropColums    []string             `json:"dropColumns"`
	}

	err := dao.Schema.checkTbl(table)
	if err != nil {
		return nil, err
	}

	query := ""

	var changes tblChanges
	err = json.NewDecoder(body).Decode(&changes)
	if err != nil {
		return nil, err
	}

	if changes.RenameColumns != nil {
		for col, new := range changes.RenameColumns {
			err = dao.Schema.checkCol(table, col)
			if err != nil {
				return nil, err
			}

			query += fmt.Sprintf("ALTER TABLE [%s] RENAME COLUMN [%s] TO [%s]; ", table, col, new)
		}
	}

	if changes.DropColums != nil {
		for _, col := range changes.DropColums {
			err = dao.Schema.checkCol(table, col)
			if err != nil {
				return nil, err
			}

			query += fmt.Sprintf("ALTER TABLE ["+table+"] DROP COLUMN [%s]; ", col)
		}
	}

	if changes.NewColumns != nil {
		for name, col := range changes.NewColumns {
			if mapColType(col.Type) == "" {
				return nil, InvalidTypeErr(name, col.Type)
			}

			query += fmt.Sprintf("ALTER TABLE ["+table+"] ADD COLUMN [%s] %s ", name, mapColType(col.Type))

			if col.NotNull {
				query += "NOT NULL "
			}
			if col.Default != nil {
				switch col.Default.(type) {
				case string:
					query += fmt.Sprintf(`DEFAULT "%s" `, col.Default)
				case float64:
					query += fmt.Sprintf("DEFAULT %g ", col.Default)
				}
			}

			if col.References != "" {
				quoted := false
				toTbl := ""
				toCol := ""
				for i := 0; toTbl == "" && i < len(col.References); i++ {
					if col.References[i] == '\'' {
						quoted = !quoted
					}
					if col.References[i] == '.' && !quoted {
						toTbl = col.References[:i]
						toCol = col.References[i+1:]

						err = dao.Schema.checkCol(toTbl, toCol)
						if err != nil {
							return nil, err
						}
					}
				}

				query += fmt.Sprintf("REFERENCES [%s]([%s]) ", toTbl, toCol)
				if col.OnDelete != "" {
					query += "ON DELETE " + mapOnAction(col.OnDelete) + " "
				}
				if col.OnUpdate != "" {
					query += "ON UPDATE " + mapOnAction(col.OnUpdate) + " "
				}
			}

			query += "; "
		}
	}

	if changes.NewName != "" {
		query += "ALTER TABLE [" + table + "] RENAME TO [" + changes.NewName + "]; "
	}

	fmt.Println(query)

	_, err = dao.Client.Exec(query)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("table %s altered", table)), dao.InvalidateSchema()
}

func (dao Database) CreateTable(table string, body io.ReadCloser) ([]byte, error) {
	query := fmt.Sprintf("CREATE TABLE [%s] (", table)

	var cols map[string]Column

	err := json.NewDecoder(body).Decode(&cols)
	if err != nil {
		return nil, err
	}

	type fKey struct {
		toTbl string
		toCol string
		col   string
	}

	var fKeys []fKey

	for n, col := range cols {
		if mapColType(col.Type) == "" {
			return nil, InvalidTypeErr(n, col.Type)
		}

		query += fmt.Sprintf("[%s] %s ", n, mapColType(col.Type))
		if col.PrimaryKey {
			query += "PRIMARY KEY "
		}
		if col.Unique {
			query += "UNIQUE "
		}
		if col.NotNull {
			query += "NOT NULL "
		}
		if col.Default != nil {
			switch col.Default.(type) {
			case string:
				query += fmt.Sprintf(`DEFAULT "%s" `, col.Default)
			case float64:
				query += fmt.Sprintf("DEFAULT %g ", col.Default)
			}
		}
		if col.References != "" {
			quoted := false
			fk := fKey{"", "", n}
			for i := 0; fk.toTbl == "" && i < len(col.References); i++ {
				if col.References[i] == '\'' {
					quoted = !quoted
				}
				if col.References[i] == '.' && !quoted {
					fk.toTbl = col.References[:i]
					fk.toCol = col.References[i+1:]

					err = dao.Schema.checkCol(fk.toTbl, fk.toCol)
					if err != nil {
						return nil, err
					}
				}
			}
			fKeys = append(fKeys, fk)
		}

		query += ", "
	}

	for _, val := range fKeys {
		query += fmt.Sprintf("FOREIGN KEY([%s]) REFERENCES [%s]([%s]) ", val.col, val.toTbl, val.toCol)
		if cols[val.col].OnDelete != "" {
			query += "ON DELETE " + mapOnAction(cols[val.col].OnDelete) + " "
		}
		if cols[val.col].OnUpdate != "" {
			query += "ON UPDATE " + mapOnAction(cols[val.col].OnUpdate) + " "
		}
		query += ", "

	}

	query = query[:len(query)-2] + ")"

	_, err = dao.Client.Exec(query)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("table %s created", table)), dao.InvalidateSchema()
}

func (dao Database) DropTable(table string) ([]byte, error) {

	err := dao.Schema.checkTbl(table)
	if err != nil {
		return nil, err
	}

	_, err = dao.Client.Exec("DROP TABLE " + table)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("table %s dropped", table)), dao.InvalidateSchema()
}

func (dao Database) EditSchema(body io.ReadCloser) ([]byte, error) {
	type reqBody struct {
		Query string `json:"query"`
		Args  []any  `json:"args"`
	}

	var bod reqBody

	err := json.NewDecoder(body).Decode(&bod)
	if err != nil {
		return nil, err
	}

	_, err = dao.Client.Exec(bod.Query, bod.Args...)
	if err != nil {
		return nil, err
	}

	return []byte("schema edited"), dao.InvalidateSchema()
}

// map functions guarantee the input is an expected expression
// to limit vulnerabilities and prevent unexpected query affects

func mapColType(str string) string {
	switch strings.ToLower(str) {
	case "text":
		return "TEXT"
	case "integer":
		return "INTEGER"
	case "real":
		return "REAL"
	case "blob":
		return "BLOB"
	default:
		return ""
	}
}

func mapOnAction(str string) string {
	switch strings.ToLower(str) {
	case "no action":
		return "NO ACTION"
	case "restrict":
		return "RESTRICT"
	case "set null":
		return "SET NULL"
	case "set default":
		return "SET DEFAULT"
	case "cascade":
		return "CASCADE"
	default:
		return ""
	}
}
