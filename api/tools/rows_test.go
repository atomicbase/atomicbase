package tools

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestScanRows(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE items (
			id INTEGER,
			name TEXT,
			price REAL,
			data BLOB
		);
		INSERT INTO items (id, name, price, data) VALUES (1, 'alpha', 9.5, x'0102');
	`)
	if err != nil {
		t.Fatalf("seed db: %v", err)
	}

	rows, err := db.Query(`SELECT id, name, price, data FROM items`)
	if err != nil {
		t.Fatalf("query rows: %v", err)
	}
	defer rows.Close()

	got, err := ScanRows(rows)
	if err != nil {
		t.Fatalf("scan rows: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0]["id"] != int64(1) {
		t.Fatalf("expected id 1, got %#v", got[0]["id"])
	}
	if got[0]["name"] != "alpha" {
		t.Fatalf("expected name alpha, got %#v", got[0]["name"])
	}
	if got[0]["price"] != 9.5 {
		t.Fatalf("expected price 9.5, got %#v", got[0]["price"])
	}
	if blob, ok := got[0]["data"].([]byte); !ok || len(blob) != 2 || blob[0] != 0x01 || blob[1] != 0x02 {
		t.Fatalf("expected blob [0x01 0x02], got %#v", got[0]["data"])
	}
}

func TestScanRowsTyped_UsesResolver(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE metrics (count TEXT);
		INSERT INTO metrics (count) VALUES ('42');
	`)
	if err != nil {
		t.Fatalf("seed db: %v", err)
	}

	rows, err := db.Query(`SELECT count FROM metrics`)
	if err != nil {
		t.Fatalf("query rows: %v", err)
	}
	defer rows.Close()

	got, err := ScanRowsTyped(rows, func(column *sql.ColumnType) string {
		if column.Name() == "count" {
			return "INTEGER"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("scan rows typed: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0]["count"] != int64(42) {
		t.Fatalf("expected coerced integer 42, got %#v", got[0]["count"])
	}
}
