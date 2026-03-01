package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSplitToken(t *testing.T) {
	tests := []struct {
		name       string
		token      SessionToken
		wantErr    bool
		wantID     string
		wantSecret string
	}{
		{name: "valid token", token: "abc.def", wantID: "abc", wantSecret: "def"},
		{name: "missing dot", token: "abcdef", wantErr: true},
		{name: "empty token", token: "", wantErr: true},
		{name: "multiple dots keeps tail", token: "abc.def.ghi", wantID: "abc", wantSecret: "def.ghi"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, secret, err := splitToken(tc.token)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidSession) {
					t.Fatalf("expected ErrInvalidSession, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if id != tc.wantID || secret != tc.wantSecret {
				t.Fatalf("expected id=%q secret=%q, got id=%q secret=%q", tc.wantID, tc.wantSecret, id, secret)
			}
		})
	}
}

func TestValidateSession(t *testing.T) {
	db := setupAuthTestDB(t)

	userNow := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO atombase_users (id, email, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		"user_1",
		"user@example.com",
		userNow,
		userNow,
	)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	valid := &Session{
		Id:        "sess_1",
		UserID:    "user_1",
		Secret:    "secret_1",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
	}
	if err := SaveSession(valid, db, context.Background()); err != nil {
		t.Fatalf("save session: %v", err)
	}

	tests := []struct {
		name     string
		token    SessionToken
		seed     func(t *testing.T)
		wantErr  bool
		assertOK func(t *testing.T, got *Session)
	}{
		{
			name:  "valid token returns session",
			token: valid.Token(),
			assertOK: func(t *testing.T, got *Session) {
				t.Helper()
				if got.Id != valid.Id || got.UserID != valid.UserID {
					t.Fatalf("unexpected session returned: %+v", got)
				}
			},
		},
		{name: "malformed token rejected", token: "invalidtoken", wantErr: true},
		{name: "wrong secret rejected", token: SessionToken(valid.Id + ".wrong"), wantErr: true},
		{name: "unknown session rejected", token: "missing.secret", wantErr: true},
		{
			name:  "expired session rejected",
			token: "sess_expired.secret_expired",
			seed: func(t *testing.T) {
				t.Helper()
				_, err := db.Exec(
					`INSERT INTO atombase_sessions (id, secret_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
					"sess_expired",
					shaHash("secret_expired"),
					"user_1",
					time.Now().UTC().Add(-1*time.Minute).Format(time.RFC3339),
					time.Now().UTC().Add(-10*time.Minute).Format(time.RFC3339),
				)
				if err != nil {
					t.Fatalf("seed expired session: %v", err)
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.seed != nil {
				tc.seed(t)
			}

			session, err := ValidateSession(tc.token, db, context.Background())
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidSession) {
					t.Fatalf("expected ErrInvalidSession, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.assertOK != nil {
				tc.assertOK(t, session)
			}
		})
	}
}

func TestDeleteUserSessions(t *testing.T) {
	db := setupAuthTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.Exec(`INSERT INTO atombase_users (id, email, created_at, updated_at) VALUES (?, ?, ?, ?)`, "user_1", "u1@example.com", now, now)
	if err != nil {
		t.Fatalf("seed user_1: %v", err)
	}
	_, err = db.Exec(`INSERT INTO atombase_users (id, email, created_at, updated_at) VALUES (?, ?, ?, ?)`, "user_2", "u2@example.com", now, now)
	if err != nil {
		t.Fatalf("seed user_2: %v", err)
	}

	for _, s := range []*Session{
		{Id: "s1", UserID: "user_1", Secret: "sec1", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(10 * time.Minute)},
		{Id: "s2", UserID: "user_1", Secret: "sec2", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(10 * time.Minute)},
		{Id: "s3", UserID: "user_2", Secret: "sec3", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(10 * time.Minute)},
	} {
		if err := SaveSession(s, db, ctx); err != nil {
			t.Fatalf("seed session %s: %v", s.Id, err)
		}
	}

	if err := DeleteUserSessions("user_1", db, ctx); err != nil {
		t.Fatalf("delete user sessions: %v", err)
	}

	var user1Count, user2Count int
	err = db.QueryRow(`SELECT COUNT(*) FROM atombase_sessions WHERE user_id = 'user_1'`).Scan(&user1Count)
	if err != nil {
		t.Fatalf("count user_1 sessions: %v", err)
	}
	err = db.QueryRow(`SELECT COUNT(*) FROM atombase_sessions WHERE user_id = 'user_2'`).Scan(&user2Count)
	if err != nil {
		t.Fatalf("count user_2 sessions: %v", err)
	}

	if user1Count != 0 || user2Count != 1 {
		t.Fatalf("unexpected counts after delete user sessions: user1=%d user2=%d", user1Count, user2Count)
	}
}

func TestCreateSessionAndTokenFormat(t *testing.T) {
	s := CreateSession("user_1")
	if s.UserID != "user_1" {
		t.Fatalf("expected user id to be copied, got %q", s.UserID)
	}
	if !s.ExpiresAt.After(s.CreatedAt) {
		t.Fatal("expected ExpiresAt to be after CreatedAt")
	}
	parts := strings.Split(string(s.Token()), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Fatalf("unexpected session token format: %q", s.Token())
	}
}
