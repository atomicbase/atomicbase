package database

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

//go:embed openapi.yaml
var openapiSpec []byte

// RegisterRoutes registers all API routes on the provided ServeMux.
//
// Routes:
//   - GET /health - Health check endpoint
//   - GET /openapi.yaml - OpenAPI 3.0 specification
//   - GET /docs - Swagger UI documentation
//   - POST/PATCH/DELETE /query/{table} - CRUD operations on table rows
//   - GET /schema - List all tables in schema
//   - POST /schema/invalidate - Refresh schema cache
//   - GET/POST/DELETE/PATCH /schema/table/{table} - Table schema operations
//   - GET/POST/DELETE /schema/fts/{table} - FTS (Full-Text Search) operations
//   - GET/POST/PATCH/DELETE /tenants - Tenant (database) management
//   - GET/POST/PUT/DELETE /templates - Schema template management
//   - GET/PUT/DELETE/POST /tenants/{name}/template, /tenants/{name}/sync - Template association
//
// Use Tenant header to target external Turso databases (default: primary).
func RegisterRoutes(app *http.ServeMux) {
	// Health check (no auth required)
	app.HandleFunc("GET /health", handleHealth())

	// OpenAPI documentation (no auth required)
	app.HandleFunc("GET /openapi.yaml", handleOpenAPISpec())
	app.HandleFunc("GET /docs", handleSwaggerUI())

	// Row operations (JSON body API)
	app.HandleFunc("POST /query/{table}", handleQueryRows())
	app.HandleFunc("PATCH /query/{table}", handleUpdateRowsJSON())
	app.HandleFunc("DELETE /query/{table}", handleDeleteRowsJSON())

	// Batch operations
	app.HandleFunc("POST /batch", handleBatch())

	// Schema operations
	app.HandleFunc("GET /schema", handleGetSchema())
	app.HandleFunc("POST /schema/invalidate", handleInvalidateSchema())

	// Table schema operations
	app.HandleFunc("GET /schema/table/{table}", handleGetTableSchema())
	app.HandleFunc("POST /schema/table/{table}", handleCreateTable())
	app.HandleFunc("DELETE /schema/table/{table}", handleDropTable())
	app.HandleFunc("PATCH /schema/table/{table}", handleAlterTable())

	// FTS (Full-Text Search) operations
	app.HandleFunc("GET /schema/fts", handleListFTSIndexes())
	app.HandleFunc("POST /schema/fts/{table}", handleCreateFTSIndex())
	app.HandleFunc("DELETE /schema/fts/{table}", handleDropFTSIndex())

	// Tenant (database) management
	app.HandleFunc("GET /tenants", handleListDbs())
	app.HandleFunc("POST /tenants", handleCreateDb())
	app.HandleFunc("PATCH /tenants", handleRegisterDb())
	app.HandleFunc("PATCH /tenants/all", handleRegisterAll())
	app.HandleFunc("DELETE /tenants/{name}", handleDeleteDb())

	// Schema template management
	app.HandleFunc("GET /templates", handleListTemplates())
	app.HandleFunc("POST /templates", handleCreateTemplate())
	app.HandleFunc("GET /templates/{name}", handleGetTemplate())
	app.HandleFunc("PUT /templates/{name}", handleUpdateTemplate())
	app.HandleFunc("DELETE /templates/{name}", handleDeleteTemplate())
	app.HandleFunc("POST /templates/{name}/sync", handleSyncTemplate())
	app.HandleFunc("GET /templates/{name}/databases", handleListTemplateDBs())

	// Tenant-template association
	app.HandleFunc("GET /tenants/{name}/template", handleGetDBTemplate())
	app.HandleFunc("PUT /tenants/{name}/template", handleSetDBTemplate())
	app.HandleFunc("DELETE /tenants/{name}/template", handleRemoveDBTemplate())
	app.HandleFunc("POST /tenants/{name}/sync", handleSyncDBToTemplate())
}

// handleBatch handles POST /batch for atomic multi-operation requests.
func handleBatch() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		var batchReq BatchRequest
		if err := json.NewDecoder(req.Body).Decode(&batchReq); err != nil {
			return nil, err
		}
		result, err := dao.Batch(ctx, batchReq)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
}

// handleQueryRows handles POST /query/{table} for SELECT, INSERT, and UPSERT operations.
// Operation determined by Prefer header:
//   - Prefer: operation=select -> SELECT query
//   - Prefer: on-conflict=replace -> UPSERT
//   - Prefer: on-conflict=ignore -> INSERT IGNORE
//   - (no header) -> INSERT
func handleQueryRows() http.HandlerFunc {
	return withDBResponse(func(ctx context.Context, dao *Database, req *http.Request, w http.ResponseWriter) ([]byte, error) {
		prefer := req.Header.Get("Prefer")
		table := req.PathValue("table")

		// SELECT operation
		if strings.Contains(prefer, PreferOperationSelect) {
			var query SelectQuery
			if err := json.NewDecoder(req.Body).Decode(&query); err != nil {
				return nil, err
			}

			includeCount := strings.Contains(prefer, PreferCountExact)
			result, err := dao.SelectJSON(ctx, table, query, includeCount)
			if err != nil {
				return nil, err
			}

			if includeCount {
				w.Header().Set("X-Total-Count", strconv.FormatInt(result.Count, 10))
			}

			return result.Data, nil
		}

		// UPSERT operation
		if strings.Contains(prefer, PreferOnConflictReplace) {
			var upsertReq UpsertRequest
			if err := json.NewDecoder(req.Body).Decode(&upsertReq); err != nil {
				return nil, err
			}
			return dao.UpsertJSON(ctx, table, upsertReq)
		}

		// INSERT IGNORE operation
		if strings.Contains(prefer, PreferOnConflictIgnore) {
			var ignoreReq InsertRequest
			if err := json.NewDecoder(req.Body).Decode(&ignoreReq); err != nil {
				return nil, err
			}
			return dao.InsertIgnoreJSON(ctx, table, ignoreReq)
		}

		// Default: INSERT operation
		var insertReq InsertRequest
		if err := json.NewDecoder(req.Body).Decode(&insertReq); err != nil {
			return nil, err
		}
		return dao.InsertJSON(ctx, table, insertReq)
	})
}

// handleUpdateRowsJSON handles PATCH /query/{table} for UPDATE operations.
func handleUpdateRowsJSON() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		var updateReq UpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&updateReq); err != nil {
			return nil, err
		}
		return dao.UpdateJSON(ctx, req.PathValue("table"), updateReq)
	})
}

// handleDeleteRowsJSON handles DELETE /query/{table} for DELETE operations.
func handleDeleteRowsJSON() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		var deleteReq DeleteRequest
		if err := json.NewDecoder(req.Body).Decode(&deleteReq); err != nil {
			return nil, err
		}
		return dao.DeleteJSON(ctx, req.PathValue("table"), deleteReq)
	})
}

func handleInvalidateSchema() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		return []byte(`{"message":"schema invalidated"}`), dao.InvalidateSchema(ctx)
	})
}

func handleGetSchema() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		return dao.GetSchema()
	})
}

func handleGetTableSchema() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		return dao.GetTableSchema(req.PathValue("table"))
	})
}

func handleCreateTable() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		return dao.CreateTable(ctx, req.PathValue("table"), req.Body)
	})
}

func handleDropTable() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		return dao.DropTable(ctx, req.PathValue("table"))
	})
}

func handleAlterTable() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		return dao.AlterTable(ctx, req.PathValue("table"), req.Body)
	})
}

func handleListFTSIndexes() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		return dao.ListFTSIndexes()
	})
}

func handleCreateFTSIndex() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		return dao.CreateFTSIndex(ctx, req.PathValue("table"), req.Body)
	})
}

func handleDropFTSIndex() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *Database, req *http.Request) ([]byte, error) {
		return dao.DropFTSIndex(ctx, req.PathValue("table"))
	})
}

func handleCreateDb() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.CreateDB(ctx, req.Body)
	})
}

func handleRegisterAll() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		err := dao.RegisterAllDbs(ctx)
		return nil, err
	})
}

func handleRegisterDb() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.RegisterDB(ctx, req.Body, req.Header.Get("DB-Token"))
	})
}

func handleListDbs() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.ListDBs(ctx)
	})
}

func handleDeleteDb() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.DeleteDB(ctx, req.PathValue("name"))
	})
}

func handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check database connectivity using the connection pool
		dao, err := ConnPrimary()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unhealthy","error":"database connection failed"}`))
			return
		}
		// Note: don't close dao.Client - it's managed by the connection pool

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

func handleOpenAPISpec() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(openapiSpec)
	}
}

func handleSwaggerUI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
  <title>Atomicbase API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/openapi.yaml',
      dom_id: '#swagger-ui',
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: 'BaseLayout'
    });
  </script>
</body>
</html>`))
	}
}

// Template handlers

func handleListTemplates() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.ListTemplatesJSON(ctx)
	})
}

func handleCreateTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		type reqBody struct {
			Name   string  `json:"name"`
			Tables []Table `json:"tables"`
		}
		var body reqBody
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		template, err := dao.CreateTemplate(ctx, body.Name, body.Tables)
		if err != nil {
			return nil, err
		}
		return json.Marshal(template)
	})
}

func handleGetTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		template, err := dao.GetTemplate(ctx, req.PathValue("name"))
		if err != nil {
			return nil, err
		}
		return json.Marshal(template)
	})
}

func handleUpdateTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		type reqBody struct {
			Tables []Table `json:"tables"`
		}
		var body reqBody
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		template, err := dao.UpdateTemplate(ctx, req.PathValue("name"), body.Tables)
		if err != nil {
			return nil, err
		}
		return json.Marshal(template)
	})
}

func handleDeleteTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		if err := dao.DeleteTemplate(ctx, req.PathValue("name")); err != nil {
			return nil, err
		}
		return []byte(`{"message":"template deleted"}`), nil
	})
}

func handleSyncTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		dropExtra := req.URL.Query().Get("dropExtra") == "true"
		results, err := dao.SyncTemplate(ctx, req.PathValue("name"), dropExtra)
		if err != nil {
			return nil, err
		}
		return json.Marshal(results)
	})
}

func handleListTemplateDBs() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		databases, err := dao.ListDatabasesByTemplate(ctx, req.PathValue("name"))
		if err != nil {
			return nil, err
		}
		if databases == nil {
			databases = []string{}
		}
		return json.Marshal(databases)
	})
}

func handleGetDBTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		template, err := dao.GetDatabaseTemplate(ctx, req.PathValue("name"))
		if err != nil {
			return nil, err
		}
		if template == nil {
			return []byte("null"), nil
		}
		return json.Marshal(template)
	})
}

func handleSetDBTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		type reqBody struct {
			TemplateName string `json:"templateName"`
		}
		var body reqBody
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		if err := dao.AssociateTemplate(ctx, req.PathValue("name"), body.TemplateName); err != nil {
			return nil, err
		}
		return []byte(`{"message":"template associated"}`), nil
	})
}

func handleRemoveDBTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		if err := dao.DisassociateTemplate(ctx, req.PathValue("name")); err != nil {
			return nil, err
		}
		return []byte(`{"message":"template disassociated"}`), nil
	})
}

func handleSyncDBToTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *PrimaryDao, req *http.Request) ([]byte, error) {
		dropExtra := req.URL.Query().Get("dropExtra") == "true"
		changes, err := dao.SyncDatabaseToTemplate(ctx, req.PathValue("name"), dropExtra)
		if err != nil {
			return nil, err
		}
		if changes == nil {
			changes = []string{}
		}
		return json.Marshal(map[string]any{
			"success": true,
			"changes": changes,
		})
	})
}
