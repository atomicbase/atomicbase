package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestFindOrCreateUser_CreatesAndNormalizes(t *testing.T) {
	db := setupAuthTestDB(t)

	user, created, err := FindOrCreateUser("  USER@Example.com ", db, context.Background())
	if err != nil {
		t.Fatalf("find/create user: %v", err)
	}
	if !created {
		t.Fatal("expected created=true for first login")
	}
	if user.Email != "user@example.com" {
		t.Fatalf("expected normalized email, got %q", user.Email)
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("expected EmailVerifiedAt to be set for new user")
	}

	var dbEmail string
	var verified sql.NullString
	err = db.QueryRow(`SELECT email, email_verified_at FROM atombase_users WHERE id = ?`, user.ID).Scan(&dbEmail, &verified)
	if err != nil {
		t.Fatalf("query inserted user: %v", err)
	}
	if dbEmail != "user@example.com" || !verified.Valid {
		t.Fatalf("unexpected db row: email=%q verified_valid=%v", dbEmail, verified.Valid)
	}
}

func TestFindOrCreateUser_ExistingUserSetsVerificationIfMissing(t *testing.T) {
	db := setupAuthTestDB(t)

	now := "2026-01-01T00:00:00Z"
	_, err := db.Exec(
		`INSERT INTO atombase_users (id, email, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		"user_1",
		"existing@example.com",
		now,
		now,
	)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	user, created, err := FindOrCreateUser("existing@example.com", db, context.Background())
	if err != nil {
		t.Fatalf("find/create existing user: %v", err)
	}
	if created {
		t.Fatal("expected created=false for existing user")
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("expected existing unverified user to be marked verified")
	}

	var verified sql.NullString
	var lastSignIn sql.NullString
	err = db.QueryRow(`SELECT email_verified_at, last_sign_in_at FROM atombase_users WHERE id = ?`, "user_1").Scan(&verified, &lastSignIn)
	if err != nil {
		t.Fatalf("query updated user: %v", err)
	}
	if !verified.Valid || !lastSignIn.Valid {
		t.Fatalf("expected verification + last_sign_in updates, got verified=%v last_sign_in=%v", verified.Valid, lastSignIn.Valid)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	db := setupAuthTestDB(t)
	_, err := GetUserByID("missing", db, context.Background())
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected ErrInvalidSession for missing user, got %v", err)
	}
}
