package daos

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
)

// setupPrimaryDB creates a connection to the primary database with cleanup.
func setupPrimaryDB(t *testing.T) *Database {
	t.Helper()
	dao, err := ConnPrimary()
	if err != nil {
		t.Fatal(err)
	}
	// Note: don't close dao.Client - it's managed by the connection pool
	return &dao.Database
}

// cleanupSchemaTestTables removes the test tables created for schema tests.
func cleanupSchemaTestTables(t *testing.T, dao *Database) {
	t.Helper()
	t.Cleanup(func() {
		dao.Client.Exec(`
			DROP TABLE IF EXISTS [test_tires];
			DROP TABLE IF EXISTS [test_cars];
			DROP TABLE IF EXISTS [test_motorcycles];
			DROP TABLE IF EXISTS [test_vehicles];
			DROP TABLE IF EXISTS [test_users];
			DROP TABLE IF EXISTS [test_users_renamed];
		`)
	})
}

// createSchemaTestTables creates the test tables needed for schema tests.
func createSchemaTestTables(t *testing.T, dao *Database) {
	t.Helper()

	_, err := dao.Client.Exec(`
		DROP TABLE IF EXISTS [test_tires];
		DROP TABLE IF EXISTS [test_cars];
		DROP TABLE IF EXISTS [test_motorcycles];
		DROP TABLE IF EXISTS [test_vehicles];
		DROP TABLE IF EXISTS [test_users];
	`)
	if err != nil {
		t.Fatal(err)
	}

	type users struct {
		Name     Column `json:"name"`
		Username Column `json:"username"`
		Id       Column `json:"id"`
	}

	name := Column{Type: "TEXT"}
	username := Column{Type: "TEXT", Unique: true}
	id := Column{Type: "INTEGER", PrimaryKey: true}
	uTbl := users{name, username, id}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	if err := enc.Encode(uTbl); err != nil {
		t.Fatal(err)
	}
	body := io.NopCloser(&buf)

	if _, err := dao.CreateTable(context.Background(), "test_users", body); err != nil {
		t.Fatal(err)
	}
	if err := dao.InvalidateSchema(context.Background()); err != nil {
		t.Fatal(err)
	}

	type vehicles struct {
		Id     Column `json:"id"`
		UserId Column `json:"user_id"`
	}

	id = Column{Type: "integer", PrimaryKey: true}
	userId := Column{Type: "Integer", References: "test_users.id"}
	vTbl := vehicles{id, userId}

	buf.Reset()
	if err := enc.Encode(vTbl); err != nil {
		t.Fatal(err)
	}

	if _, err := dao.CreateTable(context.Background(), "test_vehicles", body); err != nil {
		t.Fatal(err)
	}
	if err := dao.InvalidateSchema(context.Background()); err != nil {
		t.Fatal(err)
	}

	type cars struct {
		Id        Column `json:"id"`
		Test      Column `json:"test"`
		Test2     Column `json:"test2"`
		VehicleId Column `json:"vehicle_id"`
	}

	id = Column{Type: "integer", PrimaryKey: true}
	test := Column{Type: "Text"}
	test2 := Column{Type: "Integer", NotNull: true}
	vehicleId := Column{Type: "Integer", References: "test_vehicles.id"}
	cTbl := cars{id, test, test2, vehicleId}

	buf.Reset()
	if err := enc.Encode(cTbl); err != nil {
		t.Fatal(err)
	}

	if _, err := dao.CreateTable(context.Background(), "test_cars", body); err != nil {
		t.Fatal(err)
	}
	if err := dao.InvalidateSchema(context.Background()); err != nil {
		t.Fatal(err)
	}

	type motorcycles struct {
		Id        Column `json:"id"`
		Brand     Column `json:"brand"`
		VehicleId Column `json:"vehicle_id"`
	}

	id = Column{Type: "integer", PrimaryKey: true}
	brand := Column{Type: "text"}
	vehicleId = Column{Type: "integer", References: "test_vehicles.id"}
	mTbl := motorcycles{id, brand, vehicleId}

	buf.Reset()
	if err := enc.Encode(mTbl); err != nil {
		t.Fatal(err)
	}

	if _, err := dao.CreateTable(context.Background(), "test_motorcycles", body); err != nil {
		t.Fatal(err)
	}
	if err := dao.InvalidateSchema(context.Background()); err != nil {
		t.Fatal(err)
	}

	type tires struct {
		Id    Column `json:"id"`
		Brand Column `json:"brand"`
		CarId Column `json:"car_id"`
	}

	id = Column{Type: "integer", PrimaryKey: true}
	carId := Column{Type: "integer", References: "test_cars.id"}
	tTbl := tires{id, brand, carId}

	buf.Reset()
	if err := enc.Encode(tTbl); err != nil {
		t.Fatal(err)
	}

	if _, err := dao.CreateTable(context.Background(), "test_tires", body); err != nil {
		t.Fatal(err)
	}
	if err := dao.InvalidateSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestEditSchema(t *testing.T) {
	dao := setupPrimaryDB(t)

	type changes struct {
		Query string `json:"query"`
		Args  []any  `json:"args"`
	}

	ch := changes{`
	CREATE TABLE IF NOT EXISTS [test_edit_schema] (
		id INTEGER PRIMARY KEY
	);
	DROP TABLE IF EXISTS [test_edit_schema];
	`, nil}

	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(ch)
	if err != nil {
		t.Fatal(err)
	}
	body := io.NopCloser(&buf)

	_, err = dao.EditSchema(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateTable(t *testing.T) {
	dao := setupPrimaryDB(t)
	cleanupSchemaTestTables(t, dao)
	createSchemaTestTables(t, dao)

	// Verify all tables were created
	for _, tbl := range []string{"test_users", "test_vehicles", "test_cars", "test_motorcycles", "test_tires"} {
		if _, err := dao.Schema.SearchTbls(tbl); err != nil {
			t.Errorf("Expected table %s to exist after creation", tbl)
		}
	}
}

func TestAlterTable(t *testing.T) {
	dao := setupPrimaryDB(t)
	cleanupSchemaTestTables(t, dao)
	createSchemaTestTables(t, dao)

	// Test adding a new column
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	alterReq := map[string]any{
		"newColumns": map[string]any{
			"email": map[string]any{
				"type": "TEXT",
			},
		},
	}
	enc.Encode(alterReq)
	body := io.NopCloser(&buf)

	resp, err := dao.AlterTable(context.Background(), "test_users", body)
	if err != nil {
		t.Errorf("AlterTable (add column) failed: %v", err)
	}
	if resp == nil {
		t.Error("Expected response from AlterTable")
	}

	err = dao.InvalidateSchema(context.Background())
	if err != nil {
		t.Errorf("InvalidateSchema failed: %v", err)
	}

	// Verify column was added
	tbl, err := dao.Schema.SearchTbls("test_users")
	if err != nil {
		t.Errorf("SearchTbls failed: %v", err)
	}
	_, err = tbl.SearchCols("email")
	if err != nil {
		t.Error("Expected 'email' column to exist after ALTER")
	}

	// Test renaming a column
	buf.Reset()
	renameReq := map[string]any{
		"renameColumns": map[string]string{
			"email": "email_address",
		},
	}
	enc.Encode(renameReq)
	body = io.NopCloser(&buf)

	_, err = dao.AlterTable(context.Background(), "test_users", body)
	if err != nil {
		t.Errorf("AlterTable (rename column) failed: %v", err)
	}

	err = dao.InvalidateSchema(context.Background())
	if err != nil {
		t.Errorf("InvalidateSchema failed: %v", err)
	}

	// Verify column was renamed
	tbl, _ = dao.Schema.SearchTbls("test_users")
	_, err = tbl.SearchCols("email_address")
	if err != nil {
		t.Error("Expected 'email_address' column to exist after rename")
	}

	// Test dropping a column
	buf.Reset()
	dropReq := map[string]any{
		"dropColumns": []string{"email_address"},
	}
	enc.Encode(dropReq)
	body = io.NopCloser(&buf)

	_, err = dao.AlterTable(context.Background(), "test_users", body)
	if err != nil {
		t.Errorf("AlterTable (drop column) failed: %v", err)
	}

	err = dao.InvalidateSchema(context.Background())
	if err != nil {
		t.Errorf("InvalidateSchema failed: %v", err)
	}

	// Verify column was dropped
	tbl, _ = dao.Schema.SearchTbls("test_users")
	_, err = tbl.SearchCols("email_address")
	if err == nil {
		t.Error("Expected 'email_address' column to NOT exist after drop")
	}

	// Test renaming table
	buf.Reset()
	renameTableReq := map[string]any{
		"newName": "test_users_renamed",
	}
	enc.Encode(renameTableReq)
	body = io.NopCloser(&buf)

	_, err = dao.AlterTable(context.Background(), "test_users", body)
	if err != nil {
		t.Errorf("AlterTable (rename table) failed: %v", err)
	}

	err = dao.InvalidateSchema(context.Background())
	if err != nil {
		t.Errorf("InvalidateSchema failed: %v", err)
	}

	// Verify table was renamed
	_, err = dao.Schema.SearchTbls("test_users_renamed")
	if err != nil {
		t.Error("Expected 'test_users_renamed' table to exist after rename")
	}

	// Rename back for cleanup
	buf.Reset()
	enc.Encode(map[string]any{"newName": "test_users"})
	body = io.NopCloser(&buf)
	dao.AlterTable(context.Background(), "test_users_renamed", body)
	dao.InvalidateSchema(context.Background())
}

func TestDropTable(t *testing.T) {
	dao := setupPrimaryDB(t)
	t.Cleanup(func() { dao.Client.Exec("DROP TABLE IF EXISTS test_drop_me") })

	// Create a test table to drop
	_, err := dao.Client.Exec(`
		DROP TABLE IF EXISTS test_drop_me;
		CREATE TABLE test_drop_me (id INTEGER PRIMARY KEY);
	`)
	if err != nil {
		t.Fatal(err)
	}
	dao.InvalidateSchema(context.Background())

	// Verify table exists
	_, err = dao.Schema.SearchTbls("test_drop_me")
	if err != nil {
		t.Error("Expected test_drop_me table to exist before drop")
	}

	// Test dropping the table
	resp, err := dao.DropTable(context.Background(), "test_drop_me")
	if err != nil {
		t.Errorf("DropTable failed: %v", err)
	}
	if resp == nil {
		t.Error("Expected response from DropTable")
	}

	// Reload schema since DropTable uses a value receiver
	err = dao.InvalidateSchema(context.Background())
	if err != nil {
		t.Errorf("InvalidateSchema failed: %v", err)
	}

	// Verify table was dropped
	_, err = dao.Schema.SearchTbls("test_drop_me")
	if err == nil {
		t.Error("Expected test_drop_me table to NOT exist after drop")
	}

	// Test dropping non-existent table (should error)
	_, err = dao.DropTable(context.Background(), "nonexistent_table_xyz")
	if err == nil {
		t.Error("Expected error when dropping non-existent table")
	}
}
