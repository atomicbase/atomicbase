package daos

import (
	"database/sql"
	"os"
	"testing"
)

func TestSchemaCols(t *testing.T) {

	schema := SchemaCache{}
	client, err := sql.Open("libsql", "file:testSchemaCols.db")
	if err != nil {
		t.Error(err)
	}

	defer client.Close()

	dao := Database{client, schema, 0}
	if err != nil {
		t.Error(err)
		return
	}

	dao.Client.Exec(`
	CREATE TABLE [test_cars] (
		id INTEGER PRIMARY KEY,
		make TEXT,
		model TEXT,
		year INTEGER
	);
	`)

	tbls, err := schemaCols(dao.Client)
	if err != nil {
		t.Error(err)
	}

	if len(tbls) != 1 {
		t.Error("expected 1 table but got", len(tbls))
	}

	if tbls[0].Name != "test_cars" {
		t.Error("expected table named test_cars but got", tbls[0].Name)
	}

	if tbls[0].Pk != "id" {
		t.Error("expected primary key to be id but got", tbls[0].Pk)
	}

	if len(tbls[0].Columns) != 4 {
		t.Error("expected 4 columns but got", tbls[0].Columns)
	}

	os.Remove("testSchemaCols.db")

}
