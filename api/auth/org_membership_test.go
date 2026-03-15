package auth

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/atombasedev/atombase/tools"
	_ "github.com/mattn/go-sqlite3"
)

type testOrganizationStore struct {
	db         *sql.DB
	databaseID string
	authToken  string
}

func (s testOrganizationStore) DB() *sql.DB {
	return s.db
}

func (s testOrganizationStore) LookupOrganizationTenant(ctx context.Context, organizationID string) (string, string, error) {
	var resolvedID string
	err := s.db.QueryRowContext(ctx, `SELECT database_id FROM atombase_organizations WHERE id = ?`, organizationID).Scan(&resolvedID)
	if err != nil {
		return "", "", err
	}
	return resolvedID, s.authToken, nil
}

func setupOrganizationMembershipAPI(t *testing.T) (*API, *sql.DB, string) {
	t.Helper()

	db := setupAuthTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE atombase_definitions (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			definition_type TEXT NOT NULL,
			current_version INTEGER NOT NULL
		)`,
		`CREATE TABLE atombase_databases (
			id TEXT PRIMARY KEY,
			definition_id INTEGER NOT NULL,
			definition_version INTEGER NOT NULL,
			auth_token_encrypted BLOB
		)`,
		`CREATE TABLE atombase_organizations (
			id TEXT PRIMARY KEY,
			database_id TEXT NOT NULL,
			name TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			max_members INTEGER,
			created_at TEXT,
			updated_at TEXT
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create org membership primary schema: %v", err)
		}
	}

	tenantPath := filepath.Join(t.TempDir(), "org-tenant.db")
	tenantDB, err := sql.Open("sqlite3", tenantPath)
	if err != nil {
		t.Fatalf("open tenant db: %v", err)
	}
	t.Cleanup(func() { _ = tenantDB.Close() })
	if _, err := tenantDB.Exec(`
		CREATE TABLE atombase_membership (
			user_id TEXT PRIMARY KEY,
			role TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatalf("create tenant membership schema: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO atombase_users (id, email, created_at, updated_at) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		"user-owner", "owner@example.com", now, now,
		"user-admin", "admin@example.com", now, now,
	); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO atombase_definitions (id, name, definition_type, current_version) VALUES (1, 'workspace', 'organization', 1)`); err != nil {
		t.Fatalf("seed definitions: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO atombase_databases (id, definition_id, definition_version) VALUES (?, 1, 1)`, tenantPath); err != nil {
		t.Fatalf("seed databases: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO atombase_organizations (id, database_id, name, owner_id, created_at, updated_at) VALUES (?, ?, 'Acme', 'user-owner', ?, ?)`,
		"org-1", tenantPath, now, now); err != nil {
		t.Fatalf("seed organizations: %v", err)
	}
	if _, err := tenantDB.Exec(`
		INSERT INTO atombase_membership (user_id, role, status, created_at)
		VALUES
			('user-owner', 'owner', 'active', ?),
			('user-admin', 'admin', 'active', ?),
			('user-member', 'member', 'active', ?)
	`, now, now, now); err != nil {
		t.Fatalf("seed tenant membership: %v", err)
	}

	api := NewAPI(testOrganizationStore{db: db, databaseID: tenantPath})

	oldOpen := openOrganizationTenantDB
	openOrganizationTenantDB = func(databaseID, authToken string) (*sql.DB, error) {
		return sql.Open("sqlite3", databaseID)
	}
	t.Cleanup(func() {
		openOrganizationTenantDB = oldOpen
	})

	return api, tenantDB, tenantPath
}

func TestListOrganizationMembers_RequiresActiveMembership(t *testing.T) {
	api, _, _ := setupOrganizationMembershipAPI(t)

	members, err := api.listOrganizationMembers(context.Background(), &Session{Id: "sess-1", UserID: "user-owner"}, "org-1")
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(members))
	}

	_, err = api.listOrganizationMembers(context.Background(), &Session{Id: "sess-2", UserID: "user-outsider"}, "org-1")
	if !errors.Is(err, tools.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for outsider, got %v", err)
	}
}

func TestCreateOrganizationMember_OnlyOwnerCanAssignOwnerRole(t *testing.T) {
	api, tenantDB, _ := setupOrganizationMembershipAPI(t)

	if _, err := api.createOrganizationMember(context.Background(), &Session{Id: "sess-admin", UserID: "user-admin"}, "org-1", createOrganizationMemberRequest{
		UserID: "user-new-owner",
		Role:   "owner",
	}); !errors.Is(err, tools.ErrUnauthorized) {
		t.Fatalf("expected admin owner assignment to fail, got %v", err)
	}

	member, err := api.createOrganizationMember(context.Background(), &Session{Id: "sess-owner", UserID: "user-owner"}, "org-1", createOrganizationMemberRequest{
		UserID: "user-new-owner",
		Role:   "owner",
	})
	if err != nil {
		t.Fatalf("owner create member: %v", err)
	}
	if member.Role != "owner" {
		t.Fatalf("expected owner role, got %s", member.Role)
	}

	var count int
	if err := tenantDB.QueryRow(`SELECT COUNT(*) FROM atombase_membership WHERE user_id = 'user-new-owner' AND role = 'owner'`).Scan(&count); err != nil {
		t.Fatalf("count created member: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected created owner row, got count=%d", count)
	}
}

func TestOrganizationMembership_PreservesLastOwner(t *testing.T) {
	api, tenantDB, _ := setupOrganizationMembershipAPI(t)

	memberRole := "member"
	if _, err := api.updateOrganizationMember(context.Background(), &Session{Id: "sess-owner", UserID: "user-owner"}, "org-1", "user-owner", updateOrganizationMemberRequest{
		Role: &memberRole,
	}); !errors.Is(err, tools.ErrUnauthorized) {
		t.Fatalf("expected last-owner demotion to fail, got %v", err)
	}

	if err := api.deleteOrganizationMember(context.Background(), &Session{Id: "sess-owner", UserID: "user-owner"}, "org-1", "user-owner"); !errors.Is(err, tools.ErrUnauthorized) {
		t.Fatalf("expected last-owner delete to fail, got %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tenantDB.Exec(`INSERT INTO atombase_membership (user_id, role, status, created_at) VALUES ('user-owner-2', 'owner', 'active', ?)`, now); err != nil {
		t.Fatalf("seed second owner: %v", err)
	}

	if err := api.deleteOrganizationMember(context.Background(), &Session{Id: "sess-owner", UserID: "user-owner"}, "org-1", "user-owner-2"); err != nil {
		t.Fatalf("delete second owner: %v", err)
	}

	var remaining int
	if err := tenantDB.QueryRow(`SELECT COUNT(*) FROM atombase_membership WHERE role = 'owner' AND status = 'active'`).Scan(&remaining); err != nil {
		t.Fatalf("count owners: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected 1 remaining owner, got %d", remaining)
	}
}
