package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/atombasedev/atombase/config"
	"github.com/atombasedev/atombase/primarystore"
	"github.com/atombasedev/atombase/tools"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

// NewAPI builds a Data API module using the shared primary metadata store.
func NewAPI(primaryStore *primarystore.Store) (*API, error) {
	if primaryStore == nil || primaryStore.DB() == nil {
		return nil, errors.New("nil primary store")
	}

	// Schema cache is populated lazily via GetCachedTemplate.
	// No preloading needed - external cache (Redis) persists across restarts.

	return &API{store: primaryStore}, nil
}

// connTurso opens a connection to an external Turso database by name.
func (api *API) connTurso(dbName string) (TenantConnection, error) {
	org := config.Cfg.TursoOrganization

	if org == "" {
		return TenantConnection{}, errors.New("TURSO_ORGANIZATION environment variable is not set but is required to access external databases")
	}

	if api == nil || api.store == nil || api.store.DB() == nil {
		return TenantConnection{}, errors.New("primary store not initialized")
	}

	meta, err := api.store.LookupDatabaseByName(dbName)
	if err != nil {
		return TenantConnection{}, err
	}

	if meta.AuthToken == "" {
		return TenantConnection{}, errors.New("database has no auth token configured")
	}

	// Get cached template (schema + current version).
	schema, currentVersion, err := GetCachedTemplate(api.store.DB(), meta.TemplateID)
	if err != nil {
		return TenantConnection{}, fmt.Errorf("failed to load schema: %w", err)
	}

	client, err := sql.Open("libsql", fmt.Sprintf("libsql://%s-%s.turso.io?authToken=%s", dbName, org, meta.AuthToken))
	if err != nil {
		return TenantConnection{}, err
	}

	err = client.Ping()
	if err != nil {
		client.Close()
		return TenantConnection{}, err
	}

	return TenantConnection{
		Client:          client,
		Schema:          schema,
		Token:           meta.AuthToken,
		Name:            dbName,
		ID:              meta.ID,
		TemplateID:      meta.TemplateID,
		SchemaVersion:   currentVersion,
		DatabaseVersion: meta.DatabaseVersion,
		primaryStore:    api.store,
	}, nil
}

// QueryMap executes a query and returns results as a slice of maps.
func (dao *TenantConnection) QueryMap(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := dao.Client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build column type lookup from schema template (for tenant databases with typeless columns).
	// Falls back to DatabaseTypeName() for primary database or unknown columns.
	schemaTypes := dao.Schema.BuildColumnTypeMap()

	return tools.ScanRowsTyped(rows, func(columnType *sql.ColumnType) string {
		return schemaTypes[columnType.Name()]
	})
}

// QueryJSON executes a query and returns results as JSON bytes.
func (dao *TenantConnection) QueryJSON(ctx context.Context, query string, args ...any) ([]byte, error) {
	m, err := dao.QueryMap(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return json.Marshal(m)
}
