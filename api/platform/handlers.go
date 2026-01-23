package platform

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/joe-ervin05/atomicbase/config"
	"github.com/joe-ervin05/atomicbase/data"
	"github.com/joe-ervin05/atomicbase/tools"
)

// PrimaryHandler is a handler that operates on the primary database.
type PrimaryHandler func(ctx context.Context, db *data.PrimaryDao, req *http.Request) ([]byte, error)

// RegisterRoutes registers all Platform API routes on the provided ServeMux.
// All routes are prefixed with /platform.
func RegisterRoutes(app *http.ServeMux) {
	// Platform health check
	app.HandleFunc("GET /platform/health", handlePlatformHealth())

	// Tenant (database) management
	app.HandleFunc("GET /platform/tenants", handleListDbs())
	app.HandleFunc("POST /platform/tenants", handleCreateDb())
	app.HandleFunc("POST /platform/tenants/import", handleImportDb())
	app.HandleFunc("DELETE /platform/tenants/{name}", handleDeleteDb())
	app.HandleFunc("POST /platform/tenants/{name}/sync", handleSyncTenant())

	// Schema template management
	app.HandleFunc("GET /platform/templates", handleListTemplates())
	app.HandleFunc("POST /platform/templates", handleCreateOrUpdateTemplate())
	app.HandleFunc("GET /platform/templates/{name}", handleGetTemplate())
	app.HandleFunc("PUT /platform/templates/{name}", handleUpdateTemplate())
	app.HandleFunc("DELETE /platform/templates/{name}", handleDeleteTemplate())
	app.HandleFunc("GET /platform/templates/{name}/tenants", handleListTemplateDBs())
	app.HandleFunc("GET /platform/templates/{name}/history", handleGetTemplateHistory())
	app.HandleFunc("POST /platform/templates/{name}/rollback", handleRollbackTemplate())
	app.HandleFunc("POST /platform/templates/{name}/diff", handleDiffTemplate())
}

// withPrimary wraps handlers that operate on the primary database.
func withPrimary(handler PrimaryHandler) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		req.Body = http.MaxBytesReader(wr, req.Body, config.Cfg.MaxRequestBody)
		defer req.Body.Close()

		dao, err := data.ConnPrimary()
		if err != nil {
			tools.RespErr(wr, err)
			return
		}
		// Note: don't close dao.Client - it's managed by the connection pool

		respData, err := handler(ctx, &dao, req)
		if err != nil {
			tools.RespErr(wr, err)
			return
		}

		if respData != nil {
			wr.Header().Set("Content-Type", "application/json")
		}
		wr.Write(respData)
	}
}

func handlePlatformHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dao, err := data.ConnPrimary()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unhealthy","error":"database connection failed"}`))
			return
		}

		if err := dao.Client.PingContext(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unhealthy","error":"database ping failed"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}
}

func handleCreateDb() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		return CreateDB(ctx, *dao, req.Body)
	})
}

func handleListDbs() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		return ListDBs(ctx, *dao)
	})
}

func handleDeleteDb() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		return DeleteDB(ctx, *dao, req.PathValue("name"))
	})
}

func handleImportDb() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		return ImportDB(ctx, *dao, req.Body)
	})
}

func handleSyncTenant() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		tenantName := req.PathValue("name")
		plan, err := SyncTenant(ctx, *dao, tenantName)
		if err != nil {
			return nil, err
		}

		type response struct {
			Message           string              `json:"message"`
			Changes           []data.SchemaChange `json:"changes,omitempty"`
			RequiresMigration bool                `json:"requiresMigration,omitempty"`
			MigrationSQL      []string            `json:"migrationSql,omitempty"`
		}

		if len(plan.Changes) == 0 {
			return json.Marshal(response{Message: "tenant already up to date"})
		}

		return json.Marshal(response{
			Message:           "tenant synced successfully",
			Changes:           plan.Changes,
			RequiresMigration: plan.RequiresMigration,
			MigrationSQL:      plan.MigrationSQL,
		})
	})
}

// Template handlers

func handleListTemplates() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		return ListTemplatesJSON(ctx, *dao)
	})
}

func handleGetTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		template, err := GetTemplate(ctx, *dao, req.PathValue("name"))
		if err != nil {
			return nil, err
		}
		return json.Marshal(template)
	})
}

func handleDeleteTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		if err := DeleteTemplate(ctx, *dao, req.PathValue("name")); err != nil {
			return nil, err
		}
		return []byte(`{"message":"template deleted"}`), nil
	})
}

func handleListTemplateDBs() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		databases, err := ListDatabasesByTemplate(ctx, *dao, req.PathValue("name"))
		if err != nil {
			return nil, err
		}
		if databases == nil {
			databases = []string{}
		}
		return json.Marshal(databases)
	})
}

// handleCreateOrUpdateTemplate handles POST /platform/templates
// Creates a new template or updates an existing one (upsert behavior)
func handleCreateOrUpdateTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		type reqBody struct {
			Name   string       `json:"name"`
			Tables []data.Table `json:"tables"`
		}
		var body reqBody
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}

		// Try to get existing template
		existing, err := GetTemplate(ctx, *dao, body.Name)
		if err != nil && err != tools.ErrTemplateNotFound {
			return nil, err
		}

		if existing.ID != 0 {
			// Template exists, update it
			template, changes, err := UpdateTemplate(ctx, *dao, body.Name, body.Tables)
			if err != nil {
				return nil, err
			}
			type response struct {
				Template data.SchemaTemplate `json:"template"`
				Changes  []data.SchemaChange `json:"changes"`
			}
			return json.Marshal(response{Template: template, Changes: changes})
		}

		// Create new template
		template, err := CreateTemplate(ctx, *dao, body.Name, body.Tables)
		if err != nil {
			return nil, err
		}
		type response struct {
			Template data.SchemaTemplate `json:"template"`
			Changes  []data.SchemaChange `json:"changes"`
		}
		return json.Marshal(response{Template: template, Changes: nil})
	})
}

// handleUpdateTemplate handles PUT /platform/templates/{name}
// Accepts optional resolvedRenames for column/table renames.
// The CLI should call diff first, prompt the user, then call this with resolutions.
func handleUpdateTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		type reqBody struct {
			Tables          []data.Table     `json:"tables"`
			ResolvedRenames []ResolvedRename `json:"resolvedRenames,omitempty"`
		}
		var body reqBody
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}

		// Get current template to generate migration plan
		current, err := GetTemplate(ctx, *dao, req.PathValue("name"))
		if err != nil {
			return nil, err
		}

		// Generate migration plan and apply any resolved renames
		plan := GenerateMigrationPlan(current.Tables, body.Tables)
		if len(body.ResolvedRenames) > 0 {
			plan = ApplyResolvedRenames(plan, body.ResolvedRenames)
		}

		// Apply the update
		template, _, err := UpdateTemplate(ctx, *dao, req.PathValue("name"), body.Tables)
		if err != nil {
			return nil, err
		}

		type response struct {
			Template     data.SchemaTemplate `json:"template"`
			Changes      []data.SchemaChange `json:"changes"`
			MigrationSQL []string            `json:"migrationSql,omitempty"`
		}
		return json.Marshal(response{
			Template:     template,
			Changes:      plan.Changes,
			MigrationSQL: plan.MigrationSQL,
		})
	})
}

// handleGetTemplateHistory handles GET /platform/templates/{name}/history
func handleGetTemplateHistory() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		versions, err := GetTemplateHistory(ctx, *dao, req.PathValue("name"))
		if err != nil {
			return nil, err
		}
		if versions == nil {
			versions = []data.TemplateVersion{}
		}
		return json.Marshal(versions)
	})
}

// handleRollbackTemplate handles POST /platform/templates/{name}/rollback
func handleRollbackTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		type reqBody struct {
			Version int `json:"version"`
		}
		var body reqBody
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}

		template, err := RollbackTemplate(ctx, *dao, req.PathValue("name"), body.Version)
		if err != nil {
			return nil, err
		}
		return json.Marshal(template)
	})
}

// handleDiffTemplate handles POST /platform/templates/{name}/diff
// Accepts JSON body with new tables and optional resolvedRenames.
// Returns the diff with migration SQL without applying.
func handleDiffTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		type reqBody struct {
			Tables          []data.Table     `json:"tables"`
			ResolvedRenames []ResolvedRename `json:"resolvedRenames,omitempty"`
		}
		var body reqBody
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}

		// Get current template
		current, err := GetTemplate(ctx, *dao, req.PathValue("name"))
		if err != nil {
			return nil, err
		}

		// Generate full migration plan
		plan := GenerateMigrationPlan(current.Tables, body.Tables)

		// Apply resolved renames if provided
		if len(body.ResolvedRenames) > 0 {
			plan = ApplyResolvedRenames(plan, body.ResolvedRenames)
		}

		type response struct {
			Changes           []data.SchemaChange `json:"changes"`
			RequiresMigration bool                `json:"requiresMigration"`
			HasAmbiguous      bool                `json:"hasAmbiguous,omitempty"`
			MigrationSQL      []string            `json:"migrationSql,omitempty"`
		}
		return json.Marshal(response{
			Changes:           plan.Changes,
			RequiresMigration: plan.RequiresMigration,
			HasAmbiguous:      plan.HasAmbiguous,
			MigrationSQL:      plan.MigrationSQL,
		})
	})
}
