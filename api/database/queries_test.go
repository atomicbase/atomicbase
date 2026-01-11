package database

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

func TestSelectWithCount(t *testing.T) {
	db := setupTestDB(t)

	// Setup test table with data
	_, err := db.Client.Exec(`
		DROP TABLE IF EXISTS test_count;
		CREATE TABLE test_count (
			id INTEGER PRIMARY KEY,
			name TEXT,
			category TEXT,
			price REAL
		);
		INSERT INTO test_count (name, category, price) VALUES
			('Item 1', 'electronics', 100.00),
			('Item 2', 'electronics', 200.00),
			('Item 3', 'books', 25.00),
			('Item 4', 'books', 30.00),
			('Item 5', 'books', 15.00);
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.updateSchema()
	t.Cleanup(func() { db.Client.Exec("DROP TABLE IF EXISTS test_count") })

	// Test SelectWithCount with includeCount=true
	t.Run("include count with data", func(t *testing.T) {
		params := url.Values{"limit": {"2"}}
		result, err := db.SelectWithCount(context.Background(), "test_count", params, true, false)
		if err != nil {
			t.Fatalf("SelectWithCount failed: %v", err)
		}

		// Should return total count of 5 even with limit=2
		if result.Count != 5 {
			t.Errorf("Expected count 5, got %d", result.Count)
		}

		// Should return data
		var rows []map[string]any
		if err := json.Unmarshal(result.Data, &rows); err != nil {
			t.Fatalf("Failed to unmarshal data: %v", err)
		}
		if len(rows) != 2 {
			t.Errorf("Expected 2 rows (due to limit), got %d", len(rows))
		}
	})

	// Test SelectWithCount with countOnly=true
	t.Run("count only", func(t *testing.T) {
		params := url.Values{}
		result, err := db.SelectWithCount(context.Background(), "test_count", params, false, true)
		if err != nil {
			t.Fatalf("SelectWithCount countOnly failed: %v", err)
		}

		if result.Count != 5 {
			t.Errorf("Expected count 5, got %d", result.Count)
		}

		// Data should be nil or empty for countOnly
		if result.Data != nil && len(result.Data) > 0 {
			t.Errorf("Expected no data for countOnly, got %s", result.Data)
		}
	})

	// Test count with WHERE clause
	t.Run("count with filter", func(t *testing.T) {
		params := url.Values{"category": {"eq.books"}}
		result, err := db.SelectWithCount(context.Background(), "test_count", params, false, true)
		if err != nil {
			t.Fatalf("SelectWithCount with filter failed: %v", err)
		}

		if result.Count != 3 {
			t.Errorf("Expected count 3 for books, got %d", result.Count)
		}
	})

	// Test count with offset (count should ignore offset)
	t.Run("count ignores offset", func(t *testing.T) {
		params := url.Values{"limit": {"2"}, "offset": {"2"}}
		result, err := db.SelectWithCount(context.Background(), "test_count", params, true, false)
		if err != nil {
			t.Fatalf("SelectWithCount with offset failed: %v", err)
		}

		// Total count should still be 5
		if result.Count != 5 {
			t.Errorf("Expected total count 5 (ignoring offset), got %d", result.Count)
		}
	})
}

func TestAggregations(t *testing.T) {
	db := setupTestDB(t)

	// Setup test table with data
	_, err := db.Client.Exec(`
		DROP TABLE IF EXISTS test_agg;
		CREATE TABLE test_agg (
			id INTEGER PRIMARY KEY,
			category TEXT,
			price REAL,
			quantity INTEGER
		);
		INSERT INTO test_agg (category, price, quantity) VALUES
			('electronics', 100.00, 5),
			('electronics', 200.00, 3),
			('books', 25.00, 10),
			('books', 30.00, 8),
			('books', 15.00, 20);
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.updateSchema()
	t.Cleanup(func() { db.Client.Exec("DROP TABLE IF EXISTS test_agg") })

	// Test count(*)
	t.Run("count all", func(t *testing.T) {
		params := url.Values{"select": {"count(*)"}}
		resp, err := db.Select(context.Background(), "test_agg", params)
		if err != nil {
			t.Fatalf("Select count(*) failed: %v", err)
		}

		var rows []map[string]any
		if err := json.Unmarshal(resp, &rows); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if len(rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(rows))
		}

		count, ok := rows[0]["count"].(float64)
		if !ok {
			t.Fatalf("Expected count field, got %v", rows[0])
		}
		if count != 5 {
			t.Errorf("Expected count 5, got %v", count)
		}
	})

	// Test sum()
	t.Run("sum", func(t *testing.T) {
		params := url.Values{"select": {"sum(price)"}}
		resp, err := db.Select(context.Background(), "test_agg", params)
		if err != nil {
			t.Fatalf("Select sum() failed: %v", err)
		}

		var rows []map[string]any
		if err := json.Unmarshal(resp, &rows); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		sum, ok := rows[0]["sum_price"].(float64)
		if !ok {
			t.Fatalf("Expected sum_price field, got %v", rows[0])
		}
		if sum != 370.0 {
			t.Errorf("Expected sum 370, got %v", sum)
		}
	})

	// Test avg()
	t.Run("avg", func(t *testing.T) {
		params := url.Values{"select": {"avg(price)"}}
		resp, err := db.Select(context.Background(), "test_agg", params)
		if err != nil {
			t.Fatalf("Select avg() failed: %v", err)
		}

		var rows []map[string]any
		if err := json.Unmarshal(resp, &rows); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		avg, ok := rows[0]["avg_price"].(float64)
		if !ok {
			t.Fatalf("Expected avg_price field, got %v", rows[0])
		}
		if avg != 74.0 {
			t.Errorf("Expected avg 74, got %v", avg)
		}
	})

	// Test min() and max()
	t.Run("min and max", func(t *testing.T) {
		params := url.Values{"select": {"min(price),max(price)"}}
		resp, err := db.Select(context.Background(), "test_agg", params)
		if err != nil {
			t.Fatalf("Select min/max failed: %v", err)
		}

		var rows []map[string]any
		if err := json.Unmarshal(resp, &rows); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		minVal, ok := rows[0]["min_price"].(float64)
		if !ok {
			t.Fatalf("Expected min_price field, got %v", rows[0])
		}
		if minVal != 15.0 {
			t.Errorf("Expected min 15, got %v", minVal)
		}

		maxVal, ok := rows[0]["max_price"].(float64)
		if !ok {
			t.Fatalf("Expected max_price field, got %v", rows[0])
		}
		if maxVal != 200.0 {
			t.Errorf("Expected max 200, got %v", maxVal)
		}
	})

	// Test GROUP BY (aggregate with regular column)
	t.Run("group by", func(t *testing.T) {
		params := url.Values{
			"select": {"category,count(*),sum(price)"},
			"order":  {"category:asc"},
		}
		resp, err := db.Select(context.Background(), "test_agg", params)
		if err != nil {
			t.Fatalf("Select with GROUP BY failed: %v", err)
		}

		var rows []map[string]any
		if err := json.Unmarshal(resp, &rows); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if len(rows) != 2 {
			t.Fatalf("Expected 2 groups, got %d", len(rows))
		}

		// First group: books
		if rows[0]["category"] != "books" {
			t.Errorf("Expected first category 'books', got %v", rows[0]["category"])
		}
		if rows[0]["count"].(float64) != 3 {
			t.Errorf("Expected books count 3, got %v", rows[0]["count"])
		}
		if rows[0]["sum_price"].(float64) != 70.0 {
			t.Errorf("Expected books sum 70, got %v", rows[0]["sum_price"])
		}

		// Second group: electronics
		if rows[1]["category"] != "electronics" {
			t.Errorf("Expected second category 'electronics', got %v", rows[1]["category"])
		}
		if rows[1]["count"].(float64) != 2 {
			t.Errorf("Expected electronics count 2, got %v", rows[1]["count"])
		}
		if rows[1]["sum_price"].(float64) != 300.0 {
			t.Errorf("Expected electronics sum 300, got %v", rows[1]["sum_price"])
		}
	})

	// Test aggregate with alias
	t.Run("aggregate with alias", func(t *testing.T) {
		params := url.Values{"select": {"total:sum(price),items:count(*)"}}
		resp, err := db.Select(context.Background(), "test_agg", params)
		if err != nil {
			t.Fatalf("Select with aliases failed: %v", err)
		}

		var rows []map[string]any
		if err := json.Unmarshal(resp, &rows); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if _, ok := rows[0]["total"]; !ok {
			t.Errorf("Expected 'total' alias, got %v", rows[0])
		}
		if _, ok := rows[0]["items"]; !ok {
			t.Errorf("Expected 'items' alias, got %v", rows[0])
		}
	})

	// Test aggregate with filter
	t.Run("aggregate with filter", func(t *testing.T) {
		params := url.Values{
			"select":   {"count(*),sum(price)"},
			"category": {"eq.electronics"},
		}
		resp, err := db.Select(context.Background(), "test_agg", params)
		if err != nil {
			t.Fatalf("Select aggregate with filter failed: %v", err)
		}

		var rows []map[string]any
		if err := json.Unmarshal(resp, &rows); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if rows[0]["count"].(float64) != 2 {
			t.Errorf("Expected count 2 for electronics, got %v", rows[0]["count"])
		}
		if rows[0]["sum_price"].(float64) != 300.0 {
			t.Errorf("Expected sum 300 for electronics, got %v", rows[0]["sum_price"])
		}
	})
}
