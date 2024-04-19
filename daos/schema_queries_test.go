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
		t.Error(err)
	}
	defer dao.Client.Close()
}

func TestDropTable(t *testing.T) {

}
