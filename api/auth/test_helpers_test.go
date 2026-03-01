package auth

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

const (
	createUsersTable = `
		CREATE TABLE atombase_users (
			id TEXT PRIMARY KEY NOT NULL,
			email TEXT UNIQUE COLLATE NOCASE,
			email_verified_at TEXT,
			last_sign_in_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`

	createSessionsTable = `
		CREATE TABLE atombase_sessions (
			id TEXT PRIMARY KEY NOT NULL,
			secret_hash BLOB NOT NULL,
			user_id TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)
	`

	createMagicLinksTable = `
		CREATE TABLE email_magic_links (
			id TEXT NOT NULL PRIMARY KEY,
			email TEXT NOT NULL UNIQUE COLLATE NOCASE,
			token_hash BLOB NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		)
	`
)

func setupAuthTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	for _, stmt := range []string{createUsersTable, createSessionsTable, createMagicLinksTable} {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			t.Fatalf("create schema: %v", err)
		}
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}
