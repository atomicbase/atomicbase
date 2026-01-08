package daos

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestEditSchema(t *testing.T) {
	dao, err := ConnPrimary()
	if err != nil {
		t.Error(err)
	}

	defer dao.Client.Close()

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

	err = json.NewEncoder(&buf).Encode(ch)
	if err != nil {
		t.Error(err)
	}
	body := io.NopCloser(&buf)

	_, err = dao.EditSchema(body)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateTable(t *testing.T) {

	dao, err := ConnPrimary()
	if err != nil {
		t.Error(err)
	}
	defer dao.Client.Close()

	_, err = dao.Client.Exec(`
	DROP TABLE IF EXISTS [test_users];
	DROP TABLE IF EXISTS [test_vehicles];
	DROP TABLE IF EXISTS [test_cars];
	DROP TABLE IF EXISTS [test_motorcycles];
	DROP TABLE IF EXISTS [test_tires];
	`)

	if err != nil {
		t.Error(err)
	}

	type users struct {
		Name     Column `json:"name"`
		Username Column `json:"username"`
		Id       Column `json:"id"`
	}

	name := Column{}
	name.Type = "TEXT"

	username := Column{}
	username.Type = "TEXT"
	username.Unique = true

	id := Column{}
	id.Type = "INTEGER"
	id.PrimaryKey = true

	uTbl := users{name, username, id}

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)

	err = enc.Encode(uTbl)

	if err != nil {
		t.Error(err)
	}

	body := io.NopCloser(&buf)

	_, err = dao.CreateTable("test_users", body)
	if err != nil {
		t.Error(err)
	}

	err = dao.InvalidateSchema()
	if err != nil {
		t.Error(err)
	}

	type vehicles struct {
		Id     Column `json:"id"`
		UserId Column `json:"user_id"`
	}

	id = Column{}
	id.Type = "integer"
	id.PrimaryKey = true

	userId := Column{}
	userId.Type = "Integer"
	userId.References = "test_users.id"

	vTbl := vehicles{id, userId}

	buf.Reset()

	err = enc.Encode(vTbl)
	if err != nil {
		t.Error(err)
	}

	_, err = dao.CreateTable("test_vehicles", body)
	if err != nil {
		t.Error(err)
	}

	err = dao.InvalidateSchema()
	if err != nil {
		t.Error(err)
	}

	type cars struct {
		Id        Column `json:"id"`
		Test      Column `json:"test"`
		Test2     Column `json:"test2"`
		VehicleId Column `json:"vehicle_id"`
	}

	id = Column{}
	id.Type = "integer"
	id.PrimaryKey = true

	test := Column{}
	test.Type = "Text"

	test2 := Column{}
	test2.Type = "Integer"
	test2.NotNull = true

	vehicleId := Column{}
	vehicleId.Type = "Integer"
	vehicleId.References = "test_vehicles.id"

	cTbl := cars{id, test, test2, vehicleId}

	buf.Reset()

	err = enc.Encode(cTbl)
	if err != nil {
		t.Error(err)
	}

	_, err = dao.CreateTable("test_cars", body)
	if err != nil {
		t.Error(err)
	}

	err = dao.InvalidateSchema()
	if err != nil {
		t.Error(err)
	}

	type motorcycles struct {
		Id        Column `json:"id"`
		Brand     Column `json:"brand"`
		VehicleId Column `json:"vehicle_id"`
	}

	id = Column{}
	id.Type = "integer"
	id.PrimaryKey = true

	brand := Column{}
	brand.Type = "text"

	vehicleId = Column{}
	vehicleId.Type = "integer"
	vehicleId.References = "test_vehicles.id"

	mTbl := motorcycles{id, brand, vehicleId}

	buf.Reset()

	err = enc.Encode(mTbl)
	if err != nil {
		t.Error(err)
	}

	_, err = dao.CreateTable("test_motorcycles", body)
	if err != nil {
		t.Error(err)
	}

	err = dao.InvalidateSchema()
	if err != nil {
		t.Error(err)
	}

	type tires struct {
		Id    Column `json:"id"`
		Brand Column `json:"brand"`
		CarId Column `json:"car_id"`
	}

	id = Column{}
	id.Type = "integer"
	id.PrimaryKey = true

	carId := Column{}
	carId.Type = "integer"
	carId.References = "test_cars.id"

	tTbl := tires{id, brand, carId}

	buf.Reset()

	err = enc.Encode(tTbl)
	if err != nil {
		t.Error(err)
	}

	_, err = dao.CreateTable("test_tires", body)
	if err != nil {
		t.Error(err)
	}

	err = dao.InvalidateSchema()
	if err != nil {
		t.Error(err)
	}
}

func TestAlterTable(t *testing.T) {
	TestCreateTable(t)

	dao, err := ConnPrimary()
	if err != nil {
		t.Fatal(err)
	}
	defer dao.Client.Close()

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

	resp, err := dao.AlterTable("test_users", body)
	if err != nil {
		t.Errorf("AlterTable (add column) failed: %v", err)
	}
	if resp == nil {
		t.Error("Expected response from AlterTable")
	}

	err = dao.InvalidateSchema()
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

	_, err = dao.AlterTable("test_users", body)
	if err != nil {
		t.Errorf("AlterTable (rename column) failed: %v", err)
	}

	err = dao.InvalidateSchema()
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

	_, err = dao.AlterTable("test_users", body)
	if err != nil {
		t.Errorf("AlterTable (drop column) failed: %v", err)
	}

	err = dao.InvalidateSchema()
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

	_, err = dao.AlterTable("test_users", body)
	if err != nil {
		t.Errorf("AlterTable (rename table) failed: %v", err)
	}

	err = dao.InvalidateSchema()
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
	dao.AlterTable("test_users_renamed", body)
	dao.InvalidateSchema()
}

func TestDropTable(t *testing.T) {
	dao, err := ConnPrimary()
	if err != nil {
		t.Fatal(err)
	}
	defer dao.Client.Close()

	// Create a test table to drop
	dao.Client.Exec(`
		DROP TABLE IF EXISTS test_drop_me;
		CREATE TABLE test_drop_me (id INTEGER PRIMARY KEY);
	`)
	dao.InvalidateSchema()

	// Verify table exists
	_, err = dao.Schema.SearchTbls("test_drop_me")
	if err != nil {
		t.Error("Expected test_drop_me table to exist before drop")
	}

	// Test dropping the table
	resp, err := dao.DropTable("test_drop_me")
	if err != nil {
		t.Errorf("DropTable failed: %v", err)
	}
	if resp == nil {
		t.Error("Expected response from DropTable")
	}

	// Reload schema since DropTable uses a value receiver
	err = dao.InvalidateSchema()
	if err != nil {
		t.Errorf("InvalidateSchema failed: %v", err)
	}

	// Verify table was dropped
	_, err = dao.Schema.SearchTbls("test_drop_me")
	if err == nil {
		t.Error("Expected test_drop_me table to NOT exist after drop")
	}

	// Test dropping non-existent table (should error)
	_, err = dao.DropTable("nonexistent_table_xyz")
	if err == nil {
		t.Error("Expected error when dropping non-existent table")
	}
}
