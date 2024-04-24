package daos

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"testing"
)

func TestSelect(t *testing.T) {
	type motorcycles struct {
		UserId int64  `json:"user_id"`
		Brand  string `json:"brand"`
		Year   int64  `json:"year"`
		Id     int64  `json:"id"`
	}

	type tires struct {
		Id    int64  `json:"id"`
		CarId int64  `json:"car_id"`
		Brand string `json:"brand"`
	}

	type cars struct {
		Tires  []tires `json:"tires"`
		Make   string  `json:"make"`
		Model  string  `json:"model"`
		Year   int64   `json:"year"`
		Id     int64   `json:"id"`
		UserId int64   `json:"user_id"`
	}

	type user struct {
		Name        string        `json:"name"`
		Username    string        `json:"username"`
		Cars        []cars        `json:"cars"`
		Motorcycles []motorcycles `json:"motorcycles"`
	}

	TestUpsert(t)

	var val url.Values = map[string][]string{
		"select": {"name,cars(*,tires()),motorcycles(),username"},
		"order":  {"name:asc"},
	}

	schema := SchemaCache{}
	client, err := sql.Open("libsql", "file:test.db")
	if err != nil {
		t.Error(err)
	}

	db := Database{client, schema, 0}

	db.updateSchema()

	resp, err := db.Select("users", val)
	if err != nil {
		t.Error(err)
	}

	var usr []user

	err = json.Unmarshal(resp, &usr)
	if err != nil {
		t.Error(err)
	}

}

func TestUpsert(t *testing.T) {
	type user struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	}

	type cars struct {
		Make   string `json:"make"`
		Model  string `json:"model"`
		Year   int16  `json:"year"`
		UserId int64  `json:"user_id"`
	}

	type tires struct {
		CarId int64  `json:"car_id"`
		Brand string `json:"brand"`
	}

	type motorcycles struct {
		UserId int64  `json:"user_id"`
		Brand  string `json:"brand"`
		Year   int16  `json:"year"`
	}

	schema := SchemaCache{}
	client, err := sql.Open("libsql", "file:test.db")
	if err != nil {
		t.Error(err)
	}

	db := Database{client, schema, 0}

	db.Client.Exec(`
		DROP TABLE IF EXISTS users;
		DROP TABLE IF EXISTS cars;
		DROP TABLE IF EXISTS tires;
		DROP TABLE IF EXISTS motorcycles;
		CREATE TABLE users (
			name TEXT,
			username TEXT UNIQUE
		);
		CREATE TABLE cars (
			id INTEGER PRIMARY KEY,
			make TEXT,
			model TEXT,
			year INTEGER,
			user_id INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(rowid)
		);
		CREATE TABLE tires (
			id INTEGER PRIMARY KEY,
			car_id INTEGER,
			brand TEXT,
			FOREIGN KEY(car_id) REFERENCES cars(id)
		);
		CREATE TABLE motorcycles (
			id INTEGER PRIMARY KEY,
			user_id INTEGER,
			brand TEXT,
			year INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(rowid)
		);
	`)

	db.updateSchema()

	fmt.Println(db.Schema.Tables)

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)

	enc.Encode([]user{{"joe", "joeshmoe"}, {"john", "johndoe"}, {"jimmy", "jimmyp"}})

	if err != nil {
		t.Error(err)
	}
	body := io.NopCloser(&buf)

	_, err = db.Upsert("users", nil, body)
	if err != nil {
		t.Error(err)
	}

	buf.Reset()

	enc.Encode([]cars{{"Nissan", "Maxima", 2018, 1}, {"Toyota", "Camry", 2021, 2}, {"Tuscon", "Hyundai", 2022, 3}, {"Nissan", "Altima", 2012, 1}})

	_, err = db.Upsert("cars", nil, body)
	if err != nil {
		t.Error(err)
	}

	buf.Reset()

	enc.Encode([]tires{{1, "Michelan"}, {2, "Atlas"}, {3, "Tensor"}, {3, "Kirkland"}})

	_, err = db.Upsert("tires", nil, body)
	if err != nil {
		t.Error(err)
	}

	buf.Reset()

	enc.Encode([]motorcycles{{1, "Ninja", 2024}, {1, "Harley Davidson", 2008}, {1, "Toyota", 1992}, {2, "Kia", 2019}})

	_, err = db.Upsert("motorcycles", nil, body)
	if err != nil {
		t.Error(err)
	}
}

func TestUpdate(t *testing.T) {

}

func TestInsert(t *testing.T) {

}

func TestDelete(t *testing.T) {

}
