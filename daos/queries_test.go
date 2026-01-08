package daos

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/url"
	"testing"
)

// setupTestDB creates a fresh test database with proper cleanup.
func setupTestDB(t *testing.T) *Database {
	t.Helper()
	client, err := sql.Open("libsql", "file:test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })

	db := &Database{Client: client, Schema: SchemaCache{}, id: 0}
	return db
}

// setupTestDataForSelect creates the test tables and data needed for select/upsert tests.
func setupTestDataForSelect(t *testing.T, db *Database) {
	t.Helper()

	_, err := db.Client.Exec(`
		DROP TABLE IF EXISTS motorcycles;
		DROP TABLE IF EXISTS tires;
		DROP TABLE IF EXISTS cars;
		DROP TABLE IF EXISTS users;
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
	if err != nil {
		t.Fatal(err)
	}

	db.updateSchema()
}

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

	type userRow struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	}

	type user struct {
		Name        string        `json:"name"`
		Username    string        `json:"username"`
		Cars        []cars        `json:"cars"`
		Motorcycles []motorcycles `json:"motorcycles"`
	}

	type carsInput struct {
		Make   string `json:"make"`
		Model  string `json:"model"`
		Year   int16  `json:"year"`
		UserId int64  `json:"user_id"`
	}

	type tiresInput struct {
		CarId int64  `json:"car_id"`
		Brand string `json:"brand"`
	}

	type motorcyclesInput struct {
		UserId int64  `json:"user_id"`
		Brand  string `json:"brand"`
		Year   int16  `json:"year"`
	}

	db := setupTestDB(t)
	setupTestDataForSelect(t, db)

	// Insert test data
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	enc.Encode([]userRow{{"joe", "joeshmoe"}, {"john", "johndoe"}, {"jimmy", "jimmyp"}})
	body := io.NopCloser(&buf)
	_, err := db.Upsert(context.Background(), "users", nil, body)
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	enc.Encode([]carsInput{{"Nissan", "Maxima", 2018, 1}, {"Toyota", "Camry", 2021, 2}, {"Tuscon", "Hyundai", 2022, 3}, {"Nissan", "Altima", 2012, 1}})
	_, err = db.Upsert(context.Background(), "cars", nil, body)
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	enc.Encode([]tiresInput{{1, "Michelan"}, {2, "Atlas"}, {3, "Tensor"}, {3, "Kirkland"}})
	_, err = db.Upsert(context.Background(), "tires", nil, body)
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	enc.Encode([]motorcyclesInput{{1, "Ninja", 2024}, {1, "Harley Davidson", 2008}, {1, "Toyota", 1992}, {2, "Kia", 2019}})
	_, err = db.Upsert(context.Background(), "motorcycles", nil, body)
	if err != nil {
		t.Fatal(err)
	}

	// Now test select with nested relations
	var val url.Values = map[string][]string{
		"select": {"name,cars(*,tires()),motorcycles(),username"},
		"order":  {"name:asc"},
	}

	resp, err := db.Select(context.Background(), "users", val)
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

	db := setupTestDB(t)
	setupTestDataForSelect(t, db)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	enc.Encode([]user{{"joe", "joeshmoe"}, {"john", "johndoe"}, {"jimmy", "jimmyp"}})
	body := io.NopCloser(&buf)

	_, err := db.Upsert(context.Background(), "users", nil, body)
	if err != nil {
		t.Error(err)
	}

	buf.Reset()

	enc.Encode([]cars{{"Nissan", "Maxima", 2018, 1}, {"Toyota", "Camry", 2021, 2}, {"Tuscon", "Hyundai", 2022, 3}, {"Nissan", "Altima", 2012, 1}})

	_, err = db.Upsert(context.Background(), "cars", nil, body)
	if err != nil {
		t.Error(err)
	}

	buf.Reset()

	enc.Encode([]tires{{1, "Michelan"}, {2, "Atlas"}, {3, "Tensor"}, {3, "Kirkland"}})

	_, err = db.Upsert(context.Background(), "tires", nil, body)
	if err != nil {
		t.Error(err)
	}

	buf.Reset()

	enc.Encode([]motorcycles{{1, "Ninja", 2024}, {1, "Harley Davidson", 2008}, {1, "Toyota", 1992}, {2, "Kia", 2019}})

	_, err = db.Upsert(context.Background(), "motorcycles", nil, body)
	if err != nil {
		t.Error(err)
	}
}

func TestUpdate(t *testing.T) {
	type userRow struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	}

	db := setupTestDB(t)
	setupTestDataForSelect(t, db)

	// Insert test data
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.Encode([]userRow{{"joe", "joeshmoe"}, {"john", "johndoe"}, {"jimmy", "jimmyp"}})
	body := io.NopCloser(&buf)

	_, err := db.Upsert(context.Background(), "users", nil, body)
	if err != nil {
		t.Fatal(err)
	}

	// Test update with WHERE clause
	buf.Reset()
	enc.Encode(map[string]any{"name": "joseph"})
	body = io.NopCloser(&buf)

	params := url.Values{"username": {"eq.joeshmoe"}}
	_, err = db.Update(context.Background(), "users", params, body)
	if err != nil {
		t.Errorf("Update failed: %v", err)
	}

	// Verify the update
	verifyParams := url.Values{"username": {"eq.joeshmoe"}}
	resp, err := db.Select(context.Background(), "users", verifyParams)
	if err != nil {
		t.Errorf("Select after update failed: %v", err)
	}

	var users []map[string]any
	json.Unmarshal(resp, &users)
	if len(users) == 0 || users[0]["name"] != "joseph" {
		t.Errorf("Update did not change name: got %v", users)
	}

	// Test update with RETURNING clause
	buf.Reset()
	enc.Encode(map[string]any{"name": "joe"})
	body = io.NopCloser(&buf)

	params = url.Values{"username": {"eq.joeshmoe"}, "select": {"name,username"}}
	resp, err = db.Update(context.Background(), "users", params, body)
	if err != nil {
		t.Errorf("Update with RETURNING failed: %v", err)
	}
	if resp == nil {
		t.Error("Expected RETURNING to return data")
	}

	// Test update non-existent column (should error)
	buf.Reset()
	enc.Encode(map[string]any{"nonexistent": "value"})
	body = io.NopCloser(&buf)

	_, err = db.Update(context.Background(), "users", url.Values{}, body)
	if err == nil {
		t.Error("Expected error for non-existent column")
	}
}

func TestInsert(t *testing.T) {
	db := setupTestDB(t)

	// Setup test table
	_, err := db.Client.Exec(`
		DROP TABLE IF EXISTS test_insert;
		CREATE TABLE test_insert (
			id INTEGER PRIMARY KEY,
			name TEXT,
			value INTEGER
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.updateSchema()
	t.Cleanup(func() { db.Client.Exec("DROP TABLE IF EXISTS test_insert") })

	// Test basic insert
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.Encode(map[string]any{"name": "test", "value": 42})
	body := io.NopCloser(&buf)

	resp, err := db.Insert(context.Background(), "test_insert", url.Values{}, body)
	if err != nil {
		t.Errorf("Insert failed: %v", err)
	}
	var insertResp map[string]any
	if err := json.Unmarshal(resp, &insertResp); err != nil {
		t.Errorf("Expected JSON response, got %s", resp)
	}
	if _, ok := insertResp["last_insert_id"]; !ok {
		t.Errorf("Expected 'last_insert_id' in response, got %v", insertResp)
	}

	// Test insert with RETURNING clause
	buf.Reset()
	enc.Encode(map[string]any{"name": "test2", "value": 100})
	body = io.NopCloser(&buf)

	params := url.Values{"select": {"name,value"}}
	resp, err = db.Insert(context.Background(), "test_insert", params, body)
	if err != nil {
		t.Errorf("Insert with RETURNING failed: %v", err)
	}
	if resp == nil {
		t.Error("Expected RETURNING to return data")
	}

	// Test insert into non-existent table (should error)
	buf.Reset()
	enc.Encode(map[string]any{"name": "test"})
	body = io.NopCloser(&buf)

	_, err = db.Insert(context.Background(), "nonexistent_table", url.Values{}, body)
	if err == nil {
		t.Error("Expected error for non-existent table")
	}

	// Test insert with invalid column (should error)
	buf.Reset()
	enc.Encode(map[string]any{"invalid_column": "value"})
	body = io.NopCloser(&buf)

	_, err = db.Insert(context.Background(), "test_insert", url.Values{}, body)
	if err == nil {
		t.Error("Expected error for invalid column")
	}
}

func TestDelete(t *testing.T) {
	db := setupTestDB(t)

	// Setup test table with data
	_, err := db.Client.Exec(`
		DROP TABLE IF EXISTS test_delete;
		CREATE TABLE test_delete (
			id INTEGER PRIMARY KEY,
			name TEXT
		);
		INSERT INTO test_delete (name) VALUES ('one'), ('two'), ('three');
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.updateSchema()
	t.Cleanup(func() { db.Client.Exec("DROP TABLE IF EXISTS test_delete") })

	// Test delete without WHERE (should error)
	_, err = db.Delete(context.Background(), "test_delete", url.Values{})
	if err == nil {
		t.Error("Expected error for DELETE without WHERE clause")
	}

	// Test delete with WHERE clause
	params := url.Values{"name": {"eq.one"}}
	resp, err := db.Delete(context.Background(), "test_delete", params)
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}
	var deleteResp map[string]any
	if err := json.Unmarshal(resp, &deleteResp); err != nil {
		t.Errorf("Expected JSON response, got %s", resp)
	}
	if deleteResp["rows_affected"] != float64(1) {
		t.Errorf("Expected 1 row affected, got %v", deleteResp["rows_affected"])
	}

	// Verify deletion
	verifyParams := url.Values{}
	selectResp, _ := db.Select(context.Background(), "test_delete", verifyParams)
	var rows []map[string]any
	json.Unmarshal(selectResp, &rows)
	if len(rows) != 2 {
		t.Errorf("Expected 2 rows after delete, got %d", len(rows))
	}

	// Test delete with RETURNING clause
	params = url.Values{"name": {"eq.two"}, "select": {"name"}}
	resp, err = db.Delete(context.Background(), "test_delete", params)
	if err != nil {
		t.Errorf("Delete with RETURNING failed: %v", err)
	}
	if resp == nil {
		t.Error("Expected RETURNING to return data")
	}
}
