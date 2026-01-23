package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/joe-ervin05/atomicbase/tools"
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
	cols, err := SchemaCols(dao.Client)
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

	dao.Schema = SchemaCache{Tables: cols, Fks: fks, FTSTables: ftsTables}

	return nil
}

func (dao *Database) InvalidateSchema(ctx context.Context) error {
	// Primary database uses introspection
	if dao.ID == 1 || dao.TemplateID == 0 {
		cols, err := SchemaCols(dao.Client)
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

		dao.Schema = SchemaCache{Tables: cols, Fks: fks, FTSTables: ftsTables}

		// Update the cached schema if this is the primary database
		if dao.ID == 1 {
			updatePrimarySchema(dao.Schema)
		}
		return nil
	}

	// Tenant databases: reload from template cache
	// This will fetch the latest version if needed
	schema, err := GetCachedSchema(nil, dao.TemplateID, dao.SchemaVersion)
	if err != nil {
		return err
	}
	dao.Schema = schema
	return nil
}

// GetSchema returns all tables in the schema as JSON.
func (dao *Database) GetSchema() ([]byte, error) {
	return json.Marshal(dao.Schema.Tables)
}

func (dao *Database) GetTableSchema(table string) ([]byte, error) {
	if err := tools.ValidateTableName(table); err != nil {
		return nil, err
	}

	type fKey struct {
		Column     string `json:"column"`
		References string `json:"references"`
	}

	type tableSchema struct {
		Columns     []Col    `json:"columns"`
		PrimaryKey  []string `json:"primaryKey"`
		ForeignKeys []fKey   `json:"foreignKeys"`
	}

	var buf bytes.Buffer

	tbl, err := dao.Schema.SearchTbls(table)
	if err != nil {
		return nil, err
	}

	var fks []fKey

	for _, key := range dao.Schema.Fks[table] {
		fks = append(fks, fKey{key.From, fmt.Sprintf("%s.%s", key.References, key.To)})
	}

	// Convert column map to slice for JSON output
	cols := make([]Col, 0, len(tbl.Columns))
	for _, col := range tbl.Columns {
		cols = append(cols, col)
	}

	err = json.NewEncoder(&buf).Encode(tableSchema{cols, tbl.Pk, fks})

	return buf.Bytes(), err
}
