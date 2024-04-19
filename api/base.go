package api

import (
	"net/http"

	"github.com/joe-ervin05/atomicbase/daos"
)

func Run(app *http.ServeMux) {
	app.HandleFunc("GET /query/{table}", handleSelectRows())    // done
	app.HandleFunc("POST /query/{table}", handleInsertRows())   // done
	app.HandleFunc("PATCH /query/{table}", handleUpdateRows())  // done
	app.HandleFunc("DELETE /query/{table}", handleDeleteRows()) // done

	app.HandleFunc("POST /schema", handleEditSchema())                  // done
	app.HandleFunc("POST /schema/invalidate", handleInvalidateSchema()) // done

	app.HandleFunc("GET /schema/table/{table}", handleGetTableSchema())
	app.HandleFunc("POST /schema/table/{table}", handleCreateTable()) // done
	app.HandleFunc("DELETE /schema/table/{table}", handleDropTable()) // done
	app.HandleFunc("PATCH /schema/table/{table}", handleAlterTable()) // done

	app.HandleFunc("GET /db", handleListDbs())            // done
	app.HandleFunc("POST /db", handleCreateDb())          // done
	app.HandleFunc("PATCH /db", handleRegisterDb())       // done
	app.HandleFunc("DELETE /db/{name}", handleDeleteDb()) // done

}

func handleSelectRows() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {

		return dao.Select(req.PathValue("table"), req.URL.Query())
	})
}

func handleInsertRows() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {
		if req.Header.Get("Prefer") == "resolution=merge-duplicates" {
			return dao.Upsert(req.PathValue("table"), req.URL.Query(), req.Body)
		}

		return dao.Insert(req.PathValue("table"), req.URL.Query(), req.Body)
	})
}

func handleUpdateRows() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {

		return dao.Update(req.PathValue("table"), req.URL.Query(), req.Body)

	})
}

func handleDeleteRows() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {

		return dao.Delete(req.PathValue("table"), req.URL.Query())

	})
}

func handleEditSchema() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {
		return dao.EditSchema(req.Body)
	})
}

func handleInvalidateSchema() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {
		return []byte("schema invalidated"), dao.InvalidateSchema()
	})
}

func handleGetTableSchema() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {
		return dao.GetTableSchema(req.PathValue("table"))
	})
}

func handleCreateTable() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {
		return dao.CreateTable(req.PathValue("table"), req.Body)
	})
}

func handleDropTable() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {
		return dao.DropTable(req.PathValue("table"))
	})
}

func handleAlterTable() http.HandlerFunc {
	return withDB(func(dao daos.Database, req *http.Request) ([]byte, error) {
		return dao.AlterTable(req.PathValue("table"), req.Body)
	})
}

func handleCreateDb() http.HandlerFunc {
	return withPrimary(func(dao daos.PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.CreateDB(req.Body)
	})
}

// func handleRegisterAll() http.HandlerFunc {
// 	return db.WithPrimary(func(dao db.Database, req *http.Request) ([]byte, error) {

// 		err := dao.RegisterAllDbs()
// 		return nil, err

// 	})
// }

func handleRegisterDb() http.HandlerFunc {
	return withPrimary(func(dao daos.PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.RegisterDB(req.Body, req.Header.Get("DB-Token"))
	})
}

func handleListDbs() http.HandlerFunc {
	return withPrimary(func(dao daos.PrimaryDao, req *http.Request) ([]byte, error) {
		return dao.ListDBs()
	})
}

func handleDeleteDb() http.HandlerFunc {
	return withPrimary(func(dao daos.PrimaryDao, req *http.Request) ([]byte, error) {

		return dao.DeleteDB(req.PathValue("name"))
	})
}
