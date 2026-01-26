package data

import (
	"context"
)

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
	schema, version, err := GetCachedTemplate(nil, dao.TemplateID)
	if err != nil {
		return err
	}
	dao.Schema = schema
	dao.SchemaVersion = version
	return nil
}
