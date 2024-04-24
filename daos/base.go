package daos

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

type PrimaryDao struct {
	Database
}

type Database struct {
	Client *sql.DB
	Schema SchemaCache
	id     int32
}

type SchemaCache struct {
	Tables []Table
	Fks    []Fk
}

type Fk struct {
	Table      string
	References string
	From       string
	To         string
}

type Table struct {
	Name    string
	Pk      string
	Columns []Col
}

type Col struct {
	Name string
	Type string
}

func init() {
	err := os.MkdirAll("atomicdata", os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	client, err := sql.Open("libsql", "file:atomicdata/primary.db")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	err = client.Ping()

	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
	schema := SchemaCache{nil, nil}
	gob.NewEncoder(&buf).Encode(schema)

	_, err = client.Exec(`
	CREATE TABLE IF NOT EXISTS databases 
	(
		id INTEGER PRIMARY KEY, 
		name TEXT UNIQUE, 
		token TEXT,
		schema BLOB
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_databases_name ON databases(name);
	INSERT INTO databases (id, schema) values(1, ?) ON CONFLICT (id) DO NOTHING;
	`, buf.Bytes())

	if err != nil {
		log.Fatal(err)
	}

	dao := PrimaryDao{Database: Database{client, schema, 1}}

	err = dao.updateSchema()
	if err != nil {
		log.Fatal(err)
	}

}

func ConnPrimary() (PrimaryDao, error) {
	client, err := sql.Open("libsql", "file:atomicdata/primary.db")
	if err != nil {
		return PrimaryDao{}, err
	}

	err = client.Ping()

	if err != nil {
		log.Fatal(err)
	}

	row := client.QueryRow("SELECT schema from databases WHERE id = 1")
	var sData []byte

	err = row.Scan(&sData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PrimaryDao{}, errors.New("database not found")
		}
		return PrimaryDao{}, err
	}

	schema, err := loadSchema(sData)

	if err != nil {
		log.Fatal(err)
	}

	return PrimaryDao{
		Database: Database{
			client, schema, 1,
		},
	}, err
}

func (dao PrimaryDao) ConnTurso(dbName string) (Database, error) {
	org := os.Getenv("TURSO_ORGANIZATION")

	if org == "" {
		return Database{}, errors.New("TURSO_ORGANIZATION environment variable is not set but is required to access external databases")
	}

	row := dao.Client.QueryRow("SELECT id, token, schema from databases WHERE name = ?", dbName)

	var id sql.NullInt32
	var token sql.NullString
	var sData []byte

	err := row.Scan(&id, &token, &sData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Database{}, errors.New("database not found")
		}
		return Database{}, err
	}

	schema, err := loadSchema(sData)

	if err != nil {
		return Database{}, err
	}

	client, err := sql.Open("libsql", fmt.Sprintf("libsql://%s-%s.turso.io?authToken=%s", dbName, org, token.String))
	if err != nil {
		return Database{}, err
	}

	err = client.Ping()

	if err != nil {
		return Database{}, err
	}

	return Database{
		client, schema, id.Int32,
	}, nil
}

func (dao Database) QueryMap(query string, args ...any) ([]interface{}, error) {
	rows, err := dao.Client.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()

	if err != nil {
		return nil, err
	}

	count := len(columnTypes)
	finalRows := []interface{}{}

	for rows.Next() {

		scanArgs := make([]interface{}, count)

		for i, v := range columnTypes {

			// doesnt use scanType to support more sqlite drivers
			switch v.DatabaseTypeName() {
			case "TEXT":
				scanArgs[i] = new(sql.NullString)
			case "INTEGER":
				scanArgs[i] = new(sql.NullInt64)
			case "REAL":
				scanArgs[i] = new(sql.NullFloat64)
			case "BLOB":
				scanArgs[i] = new(sql.RawBytes)
			default:
				scanArgs[i] = new(sql.NullString)
			}
		}

		err := rows.Scan(scanArgs...)

		if err != nil {
			return nil, err
		}

		masterData := map[string]interface{}{}

		for i, v := range columnTypes {
			if z, ok := (scanArgs[i]).(*sql.NullString); ok {
				masterData[v.Name()] = z.String
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt64); ok {
				masterData[v.Name()] = z.Int64
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullFloat64); ok {
				masterData[v.Name()] = z.Float64
				continue
			}

			masterData[v.Name()] = scanArgs[i]
		}

		finalRows = append(finalRows, masterData)
	}

	return finalRows, nil
}

func (dao Database) QueryJSON(query string, args ...any) ([]byte, error) {
	m, err := dao.QueryMap(query, args...)
	if err != nil {
		return nil, err
	}

	return json.Marshal(&m)
}
