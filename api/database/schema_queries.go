package database

import (
	"bytes"
	"context"
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

func (dao *Database) updateSchema() error {
	cols, err := schemaCols(dao.Client)
	if err != nil {
		return err
	}
	fks, err := schemaFks(dao.Client)
	if err != nil {
		return err
	}
	ftsTables, err := schemaFTS(dao.Client)
	if err != nil {
		return err
	}

	dao.Schema = SchemaCache{cols, fks, ftsTables}

	return nil
}

func (dao *Database) InvalidateSchema(ctx context.Context) error {

	cols, err := schemaCols(dao.Client)
	if err != nil {
		return err
	}
	fks, err := schemaFks(dao.Client)
	if err != nil {
		return err
	}
	ftsTables, err := schemaFTS(dao.Client)
	if err != nil {
		return err
	}

	dao.Schema = SchemaCache{cols, fks, ftsTables}

	// Update the cached schema if this is the primary database
	if dao.id == 1 {
		updatePrimarySchema(dao.Schema)
	}

	return dao.saveSchema()
}

// GetSchema returns all tables in the schema as JSON.
func (dao *Database) GetSchema() ([]byte, error) {
	return json.Marshal(dao.Schema.Tables)
}

func (dao *Database) GetTableSchema(table string) ([]byte, error) {
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}

	type fKey struct {
		Column     string `json:"column"`
		References string `json:"references"`
	}

	type tableSchema struct {
		Columns     []Col  `json:"columns"`
		PrimaryKey  string `json:"primaryKey"`
		ForeignKeys []fKey `json:"foreignKeys"`
	}

	var buf bytes.Buffer

	tbl, err := dao.Schema.SearchTbls(table)
	if err != nil {
		return nil, err
	}

	var fks []fKey

	for _, key := range dao.Schema.Fks {
		if key.Table == table {
			fks = append(fks, fKey{key.From, fmt.Sprintf("%s.%s", key.References, key.To)})
		}
	}

	err = json.NewEncoder(&buf).Encode(tableSchema{tbl.Columns, tbl.Pk, fks})

	return buf.Bytes(), err
}

func (dao *Database) AlterTable(ctx context.Context, table string, body io.ReadCloser) ([]byte, error) {
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}

	type tblChanges struct {
		NewName       string               `json:"newName"`
		RenameColumns map[string]string    `json:"renameColumns"`
		NewColumns    map[string]NewColumn `json:"newColumns"`
		DropColumns   []string             `json:"dropColumns"`
	}

	tbl, err := dao.Schema.SearchTbls(table)
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
			if err := ValidateColumnName(new); err != nil {
				return nil, err
			}
			_, err = tbl.SearchCols(col)
			if err != nil {
				return nil, err
			}

			query += fmt.Sprintf("ALTER TABLE [%s] RENAME COLUMN [%s] TO [%s]; ", table, col, new)
		}
	}

	if changes.DropColumns != nil {
		for _, col := range changes.DropColumns {
			_, err = tbl.SearchCols(col)
			if err != nil {
				return nil, err
			}

			query += fmt.Sprintf("ALTER TABLE ["+table+"] DROP COLUMN [%s]; ", col)
		}
	}

	if changes.NewColumns != nil {
		for name, col := range changes.NewColumns {
			if err := ValidateColumnName(name); err != nil {
				return nil, err
			}
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
						toTable, err := dao.Schema.SearchTbls(toTbl)
						if err != nil {
							return nil, err
						}
						toCol = col.References[i+1:]
						_, err = toTable.SearchCols(toCol)
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
		if err := ValidateTableName(changes.NewName); err != nil {
			return nil, err
		}
		query += "ALTER TABLE [" + table + "] RENAME TO [" + changes.NewName + "]; "
	}

	_, err = dao.Client.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf(`{"message":"table %s altered"}`, table)), dao.InvalidateSchema(ctx)
}

func (dao *Database) CreateTable(ctx context.Context, table string, body io.ReadCloser) ([]byte, error) {
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}

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
		if err := ValidateColumnName(n); err != nil {
			return nil, err
		}
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
					toTable, err := dao.Schema.SearchTbls(fk.toTbl)
					if err != nil {
						return nil, err
					}
					fk.toCol = col.References[i+1:]
					_, err = toTable.SearchCols(fk.toCol)
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

	_, err = dao.Client.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf(`{"message":"table %s created"}`, table)), dao.InvalidateSchema(ctx)
}

func (dao *Database) DropTable(ctx context.Context, table string) ([]byte, error) {
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}

	_, err := dao.Schema.SearchTbls(table)
	if err != nil {
		return nil, err
	}

	_, err = dao.Client.ExecContext(ctx, fmt.Sprintf("DROP TABLE [%s]", table))
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf(`{"message":"table %s dropped"}`, table)), dao.InvalidateSchema(ctx)
}

func (dao *Database) EditSchema(ctx context.Context, body io.ReadCloser) ([]byte, error) {
	type reqBody struct {
		Query string `json:"query"`
		Args  []any  `json:"args"`
	}

	var bod reqBody

	err := json.NewDecoder(body).Decode(&bod)
	if err != nil {
		return nil, err
	}

	// Validate that the query is a DDL statement (schema modification only)
	if err := ValidateDDLQuery(bod.Query); err != nil {
		return nil, err
	}

	_, err = dao.Client.ExecContext(ctx, bod.Query, bod.Args...)
	if err != nil {
		return nil, err
	}

	return []byte(`{"message":"schema edited"}`), dao.InvalidateSchema(ctx)
}

// mapColType validates and normalizes a column type string.
// Returns empty string if the type is not valid.
func mapColType(str string) string {
	switch strings.ToLower(str) {
	case "text":
		return ColTypeText
	case "integer":
		return ColTypeInteger
	case "real":
		return ColTypeReal
	case "blob":
		return ColTypeBlob
	default:
		return ""
	}
}

// mapOnAction validates and normalizes a foreign key action string.
// Returns empty string if the action is not valid.
func mapOnAction(str string) string {
	switch strings.ToLower(str) {
	case "no action":
		return FkNoAction
	case "restrict":
		return FkRestrict
	case "set null":
		return FkSetNull
	case "set default":
		return FkSetDefault
	case "cascade":
		return FkCascade
	default:
		return ""
	}
}
