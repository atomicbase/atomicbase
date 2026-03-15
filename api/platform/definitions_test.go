package platform

import (
	"context"
	"database/sql"
	"testing"

	"github.com/atombasedev/atombase/primarystore"
	_ "github.com/mattn/go-sqlite3"
)

const platformSchema = `
CREATE TABLE atombase_definitions (
	id INTEGER PRIMARY KEY,
	name TEXT UNIQUE NOT NULL,
	definition_type TEXT NOT NULL,
	roles_json TEXT,
	current_version INTEGER DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE atombase_definitions_history (
	id INTEGER PRIMARY KEY,
	definition_id INTEGER NOT NULL,
	version INTEGER NOT NULL,
	schema_json TEXT NOT NULL,
	checksum TEXT NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE(definition_id, version)
);
CREATE TABLE atombase_access_policies (
	definition_id INTEGER NOT NULL,
	version INTEGER NOT NULL,
	table_name TEXT NOT NULL,
	operation TEXT NOT NULL,
	conditions_json TEXT,
	PRIMARY KEY(definition_id, version, table_name, operation)
);
CREATE TABLE atombase_databases (
	id TEXT PRIMARY KEY NOT NULL,
	definition_id INTEGER NOT NULL,
	definition_version INTEGER DEFAULT 1,
	auth_token_encrypted BLOB,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE atombase_users (
	id TEXT PRIMARY KEY NOT NULL,
	database_id TEXT UNIQUE,
	updated_at TEXT
);
CREATE TABLE atombase_organizations (
	id TEXT PRIMARY KEY NOT NULL,
	database_id TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	max_members INTEGER,
	metadata TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`

func setupPlatformAPI(t *testing.T) (*API, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(platformSchema); err != nil {
		t.Fatal(err)
	}
	store, err := primarystore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewAPI(store)
	if err != nil {
		t.Fatal(err)
	}
	return api, db
}

func TestDefinitionCRUDAndHistory(t *testing.T) {
	api, db := setupPlatformAPI(t)
	defer db.Close()

	schema := Schema{Tables: []Table{{Name: "posts", Pk: []string{"id"}, Columns: map[string]Col{
		"id":        {Name: "id", Type: "INTEGER"},
		"author_id": {Name: "author_id", Type: "TEXT"},
	}}}}

	created, err := api.createDefinition(context.Background(), CreateDefinitionRequest{
		Name:   "posts",
		Type:   "organization",
		Roles:  []string{"owner", "member"},
		Schema: schema,
		Access: map[string]OperationPolicy{
			"posts": {
				Select: &Condition{Field: "auth.status", Op: "eq", Value: "member"},
				Update: &Condition{Field: "auth.role", Op: "eq", Value: "owner"},
			},
		},
	})
	if err != nil {
		t.Fatalf("createDefinition failed: %v", err)
	}
	if created.CurrentVersion != 1 {
		t.Fatalf("expected version 1, got %d", created.CurrentVersion)
	}

	history, err := api.getDefinitionHistory(context.Background(), "posts")
	if err != nil {
		t.Fatalf("getDefinitionHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history row, got %d", len(history))
	}

	nextSchema := Schema{Tables: []Table{
		{Name: "posts", Pk: []string{"id"}, Columns: map[string]Col{
			"id":        {Name: "id", Type: "INTEGER"},
			"author_id": {Name: "author_id", Type: "TEXT"},
			"title":     {Name: "title", Type: "TEXT"},
		}},
	}}
	version, err := api.pushDefinition(context.Background(), "posts", PushDefinitionRequest{
		Schema: nextSchema,
		Access: map[string]OperationPolicy{
			"posts": {
				Select: &Condition{Field: "auth.status", Op: "eq", Value: "member"},
			},
		},
	})
	if err != nil {
		t.Fatalf("pushDefinition failed: %v", err)
	}
	if version.Version != 2 {
		t.Fatalf("expected version 2, got %d", version.Version)
	}
}

func TestCreateDatabase_AttachesUserAndOrgMetadata(t *testing.T) {
	api, db := setupPlatformAPI(t)
	defer db.Close()

	_, _ = db.Exec(`INSERT INTO atombase_users (id) VALUES ('user-1')`)
	_, err := api.createDefinition(context.Background(), CreateDefinitionRequest{
		Name: "notes",
		Type: "user",
		Schema: Schema{Tables: []Table{{Name: "notes", Pk: []string{"id"}, Columns: map[string]Col{
			"id": {Name: "id", Type: "INTEGER"},
		}}}},
		Access: map[string]OperationPolicy{"notes": {Select: &Condition{Field: "auth.id", Op: "eq", Value: "auth.id"}}},
	})
	if err != nil {
		t.Fatalf("createDefinition(user) failed: %v", err)
	}
	_, err = api.createDefinition(context.Background(), CreateDefinitionRequest{
		Name: "workspace",
		Type: "organization",
		Schema: Schema{Tables: []Table{{Name: "projects", Pk: []string{"id"}, Columns: map[string]Col{
			"id": {Name: "id", Type: "INTEGER"},
		}}}},
		Access: map[string]OperationPolicy{"projects": {Select: &Condition{Field: "auth.status", Op: "eq", Value: "member"}}},
	})
	if err != nil {
		t.Fatalf("createDefinition(org) failed: %v", err)
	}

	oldCreate := tursoCreateDatabaseFn
	oldDelete := tursoDeleteDatabaseFn
	oldToken := tursoCreateTokenFn
	oldBatch := batchExecuteWithTokenFn
	defer func() {
		tursoCreateDatabaseFn = oldCreate
		tursoDeleteDatabaseFn = oldDelete
		tursoCreateTokenFn = oldToken
		batchExecuteWithTokenFn = oldBatch
	}()
	tursoCreateDatabaseFn = func(ctx context.Context, name string) error { return nil }
	tursoDeleteDatabaseFn = func(ctx context.Context, name string) error { return nil }
	tursoCreateTokenFn = func(ctx context.Context, name string) (string, error) { return "token", nil }
	batchExecuteWithTokenFn = func(ctx context.Context, dbName, token string, statements []string) error { return nil }

	userDB, err := api.createDatabase(context.Background(), CreateDatabaseRequest{
		ID:         "notes-db",
		Definition: "notes",
		UserID:     "user-1",
	})
	if err != nil {
		t.Fatalf("createDatabase(user) failed: %v", err)
	}
	if userDB.DefinitionType != "user" {
		t.Fatalf("expected user definition type, got %s", userDB.DefinitionType)
	}

	orgDB, err := api.createDatabase(context.Background(), CreateDatabaseRequest{
		ID:               "workspace-db",
		Definition:       "workspace",
		OrganizationID:   "org-1",
		OrganizationName: "Acme",
		OwnerID:          "user-1",
	})
	if err != nil {
		t.Fatalf("createDatabase(org) failed: %v", err)
	}
	if orgDB.OrganizationID != "org-1" {
		t.Fatalf("expected org id org-1, got %s", orgDB.OrganizationID)
	}
}
