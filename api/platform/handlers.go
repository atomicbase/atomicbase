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

	// Schema template management
	app.HandleFunc("GET /platform/templates", handleListTemplates())
	app.HandleFunc("POST /platform/templates", handleCreateTemplate())
	app.HandleFunc("GET /platform/templates/{name}", handleGetTemplate())
	app.HandleFunc("DELETE /platform/templates/{name}", handleDeleteTemplate())
	app.HandleFunc("GET /platform/templates/{name}/tenants", handleListTemplateDBs())
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

// Template handlers

func handleListTemplates() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		return ListTemplatesJSON(ctx, *dao)
	})
}

func handleCreateTemplate() http.HandlerFunc {
	return withPrimary(func(ctx context.Context, dao *data.PrimaryDao, req *http.Request) ([]byte, error) {
		type reqBody struct {
			Name   string       `json:"name"`
			Tables []data.Table `json:"tables"`
		}
		var body reqBody
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		template, err := CreateTemplate(ctx, *dao, body.Name, body.Tables)
		if err != nil {
			return nil, err
		}
		return json.Marshal(template)
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
