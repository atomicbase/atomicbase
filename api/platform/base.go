package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/atombasedev/atombase/primarystore"
	"github.com/atombasedev/atombase/tools"
)

// API is the Platform API module with injected dependencies.
type API struct {
	store *primarystore.Store
}

// Table names for internal platform tables.
const (
	TableDefinitions        = "atombase_definitions"
	TableDefinitionsHistory = "atombase_definitions_history"
	TableDatabases          = "atombase_databases"
	TableMigrations         = "atombase_migrations"
	TableMigrationFailures  = "atombase_migration_failures"
	TableAccessPolicies     = "atombase_access_policies"
	TableOrganizations      = "atombase_organizations"
)

// NewAPI builds a Platform API module using the shared primary metadata store.
func NewAPI(primaryStore *primarystore.Store) (*API, error) {
	if primaryStore == nil || primaryStore.DB() == nil {
		return nil, errors.New("nil primary store")
	}
	return &API{store: primaryStore}, nil
}

func (api *API) dbConn() (*sql.DB, error) {
	if api == nil || api.store == nil || api.store.DB() == nil {
		return nil, errors.New("platform database not initialized")
	}
	return api.store.DB(), nil
}

// queryJSON executes a query and returns results as JSON bytes.
func (api *API) queryJSON(ctx context.Context, query string, args ...any) ([]byte, error) {
	conn, err := api.dbConn()
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results, err := tools.ScanRows(rows)
	if err != nil {
		return nil, err
	}

	return json.Marshal(results)
}
