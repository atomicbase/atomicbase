package handlers

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/joe-ervin05/atomicbase/daos"
)

//go:embed openapi.yaml
var openapiSpec []byte

// Run registers all API routes on the provided ServeMux.
//
// Routes:
//   - GET /health - Health check endpoint
//   - GET /openapi.yaml - OpenAPI 3.0 specification
//   - GET /docs - Swagger UI documentation
//   - GET/POST/PATCH/DELETE /query/{table} - CRUD operations on table rows
//   - POST /schema - Execute raw schema SQL
//   - POST /schema/invalidate - Refresh schema cache
//   - GET/POST/DELETE/PATCH /schema/table/{table} - Table schema operations
//   - GET/POST/PATCH/DELETE /db - Database management
//
// Use DB-Name header to target external Turso databases (default: primary).
func Run(app *http.ServeMux) {
	// Health check (no auth required)
	app.HandleFunc("GET /health", handleHealth())

	// OpenAPI documentation (no auth required)
	app.HandleFunc("GET /openapi.yaml", handleOpenAPISpec())
	app.HandleFunc("GET /docs", handleSwaggerUI())

	// Row operations
	app.HandleFunc("GET /query/{table}", handleSelectRows())
	app.HandleFunc("POST /query/{table}", handleInsertRows())
	app.HandleFunc("PATCH /query/{table}", handleUpdateRows())
	app.HandleFunc("DELETE /query/{table}", handleDeleteRows())

	// Schema operations
	app.HandleFunc("POST /schema", handleEditSchema())
	app.HandleFunc("POST /schema/invalidate", handleInvalidateSchema())

	// Table schema operations
	app.HandleFunc("GET /schema/table/{table}", handleGetTableSchema())
	app.HandleFunc("POST /schema/table/{table}", handleCreateTable())
	app.HandleFunc("DELETE /schema/table/{table}", handleDropTable())
	app.HandleFunc("PATCH /schema/table/{table}", handleAlterTable())

	// Database management
	app.HandleFunc("GET /db", handleListDbs())
	app.HandleFunc("POST /db", handleCreateDb())
	app.HandleFunc("PATCH /db", handleRegisterDb())
	app.HandleFunc("PATCH /db/all", handleRegisterAll())
	app.HandleFunc("DELETE /db/{name}", handleDeleteDb())
}

func handleSelectRows() http.HandlerFunc {
	return withDBResponse(func(ctx context.Context, dao *daos.Database, req *http.Request, w http.ResponseWriter) ([]byte, error) {
		params := req.URL.Query()

		// Check for count preferences
		prefer := req.Header.Get("Prefer")
		includeCount := strings.Contains(prefer, "count=exact")
		countOnly := params.Get(daos.ParamCount) == "only"

		// If count is requested, use SelectWithCount
		if includeCount || countOnly {
			result, err := dao.SelectWithCount(ctx, req.PathValue("table"), params, includeCount, countOnly)
			if err != nil {
				return nil, err
			}

			// Set count header
			if includeCount || countOnly {
				w.Header().Set("X-Total-Count", strconv.FormatInt(result.Count, 10))
			}

			// If count only, return just the count as JSON
			if countOnly {
				return json.Marshal(map[string]int64{"count": result.Count})
			}

			return result.Data, nil
		}

		return dao.Select(ctx, req.PathValue("table"), params)
	})
}

func handleInsertRows() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		if req.Header.Get("Prefer") == "resolution=merge-duplicates" {
			return dao.Upsert(ctx, req.PathValue("table"), req.URL.Query(), req.Body)
		}

		return dao.Insert(ctx, req.PathValue("table"), req.URL.Query(), req.Body)
	})
}

func handleUpdateRows() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		return dao.Update(ctx, req.PathValue("table"), req.URL.Query(), req.Body)
	})
}

func handleDeleteRows() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		return dao.Delete(ctx, req.PathValue("table"), req.URL.Query())
	})
}

func handleEditSchema() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		return dao.EditSchema(ctx, req.Body)
	})
}

func handleInvalidateSchema() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		return []byte(`{"message":"schema invalidated"}`), dao.InvalidateSchema(ctx)
	})
}

func handleGetTableSchema() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		return dao.GetTableSchema(req.PathValue("table"))
	})
}

func handleCreateTable() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		return dao.CreateTable(ctx, req.PathValue("table"), req.Body)
	})
}

func handleDropTable() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		return dao.DropTable(ctx, req.PathValue("table"))
	})
}

func handleAlterTable() http.HandlerFunc {
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		return dao.AlterTable(ctx, req.PathValue("table"), req.Body)
	})
}

func handleCreateDb() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *daos.PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.CreateDB(ctx, req.Body)
	})
}

func handleRegisterAll() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *daos.PrimaryDao, req *http.Request) ([]byte, error) {
		err := dao.RegisterAllDbs(ctx)
		return nil, err
	})
}

func handleRegisterDb() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *daos.PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.RegisterDB(ctx, req.Body, req.Header.Get("DB-Token"))
	})
}

func handleListDbs() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *daos.PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.ListDBs(ctx)
	})
}

func handleDeleteDb() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *daos.PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.DeleteDB(ctx, req.PathValue("name"))
	})
}

func handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check database connectivity using the connection pool
		dao, err := daos.ConnPrimary()
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
