package api

import (
	"context"
	"net/http"

	"github.com/joe-ervin05/atomicbase/daos"
)

// Run registers all API routes on the provided ServeMux.
//
// Routes:
//   - GET/POST/PATCH/DELETE /query/{table} - CRUD operations on table rows
//   - POST /schema - Execute raw schema SQL
//   - POST /schema/invalidate - Refresh schema cache
//   - GET/POST/DELETE/PATCH /schema/table/{table} - Table schema operations
//   - GET/POST/PATCH/DELETE /db - Database management
//
// Use DB-Name header to target external Turso databases (default: primary).
func Run(app *http.ServeMux) {
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
	return withDB(func(ctx context.Context, dao *daos.Database, req *http.Request) ([]byte, error) {
		return dao.Select(ctx, req.PathValue("table"), req.URL.Query())
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
