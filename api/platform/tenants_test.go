package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// =============================================================================
// Test Database Setup
// =============================================================================

// setupTenantTestDB creates a test database with all required tables.
func setupTenantTestDB(t *testing.T) *sql.DB {
	t.Helper()

	testDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	// Create templates table
	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS ` + TableTemplates + ` (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			current_version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		testDB.Close()
		t.Fatalf("failed to create templates table: %v", err)
	}

	// Create templates history table
	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS ` + TableTemplatesHistory + ` (
			id INTEGER PRIMARY KEY,
			template_id INTEGER NOT NULL,
			version INTEGER NOT NULL,
			schema BLOB NOT NULL,
			checksum TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(template_id, version)
		)
	`)
	if err != nil {
		testDB.Close()
		t.Fatalf("failed to create templates_history table: %v", err)
	}

	// Create tenants table
	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS ` + TableTenants + ` (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			token TEXT NOT NULL,
			template_id INTEGER NOT NULL,
			template_version INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		testDB.Close()
		t.Fatalf("failed to create tenants table: %v", err)
	}

	// Create migrations table
	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS ` + TableMigrations + ` (
			id INTEGER PRIMARY KEY,
			template_id INTEGER NOT NULL,
			from_version INTEGER NOT NULL,
			to_version INTEGER NOT NULL,
			sql TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			state TEXT,
			total_dbs INTEGER NOT NULL DEFAULT 0,
			completed_dbs INTEGER NOT NULL DEFAULT 0,
			failed_dbs INTEGER NOT NULL DEFAULT 0,
			started_at TEXT,
			completed_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		testDB.Close()
		t.Fatalf("failed to create migrations table: %v", err)
	}

	// Create tenant_migrations table
	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS ` + TableTenantMigrations + ` (
			migration_id INTEGER NOT NULL,
			tenant_id INTEGER NOT NULL,
			status TEXT NOT NULL,
			error TEXT,
			attempts INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (migration_id, tenant_id)
		)
	`)
	if err != nil {
		testDB.Close()
		t.Fatalf("failed to create tenant_migrations table: %v", err)
	}

	return testDB
}

// setTestDB temporarily sets the package-level db for testing.
func setTestDB(t *testing.T, testDB *sql.DB) func() {
	t.Helper()
	dbMu.Lock()
	oldDB := db
	db = testDB
	dbMu.Unlock()

	return func() {
		dbMu.Lock()
		db = oldDB
		dbMu.Unlock()
	}
}

// insertTestTemplate inserts a template for testing.
func insertTestTemplate(t *testing.T, testDB *sql.DB, name string, version int) int32 {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	schema := Schema{Tables: []Table{{Name: "users", Pk: []string{"id"}, Columns: map[string]Col{"id": {Name: "id", Type: "INTEGER"}}}}}
	schemaJSON, _ := json.Marshal(schema)

	result, err := testDB.Exec(`
		INSERT INTO `+TableTemplates+` (name, current_version, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, name, version, now, now)
	if err != nil {
		t.Fatalf("failed to insert template: %v", err)
	}

	id, _ := result.LastInsertId()

	// Insert history for all versions up to current
	for v := 1; v <= version; v++ {
		_, err = testDB.Exec(`
			INSERT INTO `+TableTemplatesHistory+` (template_id, version, schema, checksum, created_at)
			VALUES (?, ?, ?, 'test', ?)
		`, id, v, schemaJSON, now)
		if err != nil {
			t.Fatalf("failed to insert template history: %v", err)
		}
	}

	return int32(id)
}

// insertTestTenant inserts a tenant for testing.
func insertTestTenant(t *testing.T, testDB *sql.DB, name string, templateID int32, version int) int32 {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := testDB.Exec(`
		INSERT INTO `+TableTenants+` (name, token, template_id, template_version, created_at, updated_at)
		VALUES (?, 'test-token', ?, ?, ?, ?)
	`, name, templateID, version, now, now)
	if err != nil {
		t.Fatalf("failed to insert tenant: %v", err)
	}

	id, _ := result.LastInsertId()
	return int32(id)
}

// insertTestMigration inserts a migration for testing.
func insertTestMigration(t *testing.T, testDB *sql.DB, templateID int32, fromVersion, toVersion int) int64 {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	sqlJSON, _ := json.Marshal([]string{"ALTER TABLE users ADD COLUMN name TEXT"})

	result, err := testDB.Exec(`
		INSERT INTO `+TableMigrations+` (template_id, from_version, to_version, sql, status, created_at)
		VALUES (?, ?, ?, ?, 'complete', ?)
	`, templateID, fromVersion, toVersion, string(sqlJSON), now)
	if err != nil {
		t.Fatalf("failed to insert migration: %v", err)
	}

	id, _ := result.LastInsertId()
	return id
}

// =============================================================================
// ListTenants Tests
// Criteria B: Various database states
// =============================================================================

func TestListTenants_Empty(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	tenants, err := ListTenants(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tenants) != 0 {
		t.Errorf("expected empty list, got %d tenants", len(tenants))
	}
}

func TestListTenants_Multiple(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 1)
	insertTestTenant(t, testDB, "tenant-a", templateID, 1)
	insertTestTenant(t, testDB, "tenant-b", templateID, 1)
	insertTestTenant(t, testDB, "tenant-c", templateID, 1)

	tenants, err := ListTenants(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tenants) != 3 {
		t.Fatalf("expected 3 tenants, got %d", len(tenants))
	}

	// Should be ordered by name
	if tenants[0].Name != "tenant-a" || tenants[1].Name != "tenant-b" || tenants[2].Name != "tenant-c" {
		t.Errorf("tenants not in expected order: %v", tenants)
	}

	// Token should be omitted in list
	for _, tenant := range tenants {
		if tenant.Token != "" {
			t.Errorf("token should be omitted in list response, got: %s", tenant.Token)
		}
	}
}

// =============================================================================
// GetTenant Tests
// Criteria B: Various scenarios
// =============================================================================

func TestGetTenant_Found(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	insertTestTenant(t, testDB, "my-tenant", templateID, 2)

	tenant, err := GetTenant(context.Background(), "my-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenant.Name != "my-tenant" {
		t.Errorf("name = %s, want my-tenant", tenant.Name)
	}
	if tenant.TemplateID != templateID {
		t.Errorf("templateID = %d, want %d", tenant.TemplateID, templateID)
	}
	if tenant.TemplateVersion != 2 {
		t.Errorf("templateVersion = %d, want 2", tenant.TemplateVersion)
	}
	if tenant.Token != "test-token" {
		t.Errorf("token = %s, want test-token", tenant.Token)
	}
}

func TestGetTenant_NotFound(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	_, err := GetTenant(context.Background(), "nonexistent")
	if err != ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got: %v", err)
	}
}

// =============================================================================
// GetMigrationByVersions Tests
// Criteria A: Core function for chained migrations
// =============================================================================

func TestGetMigrationByVersions_Found(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	insertTestMigration(t, testDB, templateID, 1, 2)

	migration, err := GetMigrationByVersions(context.Background(), templateID, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if migration.FromVersion != 1 {
		t.Errorf("fromVersion = %d, want 1", migration.FromVersion)
	}
	if migration.ToVersion != 2 {
		t.Errorf("toVersion = %d, want 2", migration.ToVersion)
	}
	if len(migration.SQL) != 1 {
		t.Errorf("expected 1 SQL statement, got %d", len(migration.SQL))
	}
}

func TestGetMigrationByVersions_NotFound(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 1)

	_, err := GetMigrationByVersions(context.Background(), templateID, 1, 2)
	if err == nil {
		t.Error("expected error for missing migration")
	}
}

// =============================================================================
// GetTenantsByTemplate Tests
// Criteria C: Complex context - relationship between tenants and templates
// =============================================================================

func TestGetTenantsByTemplate_Found(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	template1 := insertTestTemplate(t, testDB, "app1", 1)
	template2 := insertTestTemplate(t, testDB, "app2", 1)

	insertTestTenant(t, testDB, "tenant-1a", template1, 1)
	insertTestTenant(t, testDB, "tenant-1b", template1, 1)
	insertTestTenant(t, testDB, "tenant-2a", template2, 1)

	tenants, err := GetTenantsByTemplate(context.Background(), template1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tenants) != 2 {
		t.Fatalf("expected 2 tenants for template1, got %d", len(tenants))
	}

	for _, tenant := range tenants {
		if tenant.TemplateID != template1 {
			t.Errorf("tenant %s has wrong templateID: %d", tenant.Name, tenant.TemplateID)
		}
	}
}

func TestGetTenantsByTemplate_Empty(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "unused", 1)

	tenants, err := GetTenantsByTemplate(context.Background(), templateID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tenants) != 0 {
		t.Errorf("expected 0 tenants, got %d", len(tenants))
	}
}

// =============================================================================
// GetPendingTenants Tests
// Criteria C: Complex query with NOT EXISTS subquery
// =============================================================================

func TestGetPendingTenants_AllPending(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)

	insertTestTenant(t, testDB, "tenant-1", templateID, 1)
	insertTestTenant(t, testDB, "tenant-2", templateID, 1)
	insertTestTenant(t, testDB, "tenant-3", templateID, 1)

	pending, err := GetPendingTenants(context.Background(), migrationID, templateID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pending) != 3 {
		t.Errorf("expected 3 pending tenants, got %d", len(pending))
	}
}

func TestGetPendingTenants_SomeAlreadyMigrated(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)

	tenant1 := insertTestTenant(t, testDB, "tenant-1", templateID, 1)
	insertTestTenant(t, testDB, "tenant-2", templateID, 1)
	insertTestTenant(t, testDB, "tenant-3", templateID, 2) // Already at version 2

	// Mark tenant-1 as already processed
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := testDB.Exec(`
		INSERT INTO `+TableTenantMigrations+` (migration_id, tenant_id, status, updated_at)
		VALUES (?, ?, 'success', ?)
	`, migrationID, tenant1, now)
	if err != nil {
		t.Fatalf("failed to insert tenant migration: %v", err)
	}

	pending, err := GetPendingTenants(context.Background(), migrationID, templateID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only tenant-2 should be pending (tenant-1 already processed, tenant-3 at current version)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending tenant, got %d", len(pending))
	}
	if pending[0].Name != "tenant-2" {
		t.Errorf("expected tenant-2, got %s", pending[0].Name)
	}
}

func TestGetPendingTenants_NoPending(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)

	// All tenants already at version 2
	insertTestTenant(t, testDB, "tenant-1", templateID, 2)
	insertTestTenant(t, testDB, "tenant-2", templateID, 2)

	pending, err := GetPendingTenants(context.Background(), migrationID, templateID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("expected 0 pending tenants, got %d", len(pending))
	}
}

// =============================================================================
// GetFailedTenants Tests
// Criteria C: Join query for failed migrations
// =============================================================================

func TestGetFailedTenants_SomeFailed(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)

	tenant1 := insertTestTenant(t, testDB, "tenant-1", templateID, 1)
	tenant2 := insertTestTenant(t, testDB, "tenant-2", templateID, 1)
	tenant3 := insertTestTenant(t, testDB, "tenant-3", templateID, 1)

	now := time.Now().UTC().Format(time.RFC3339)

	// tenant-1: success
	_, _ = testDB.Exec(`INSERT INTO `+TableTenantMigrations+` VALUES (?, ?, 'success', NULL, 1, ?)`, migrationID, tenant1, now)
	// tenant-2: failed
	_, _ = testDB.Exec(`INSERT INTO `+TableTenantMigrations+` VALUES (?, ?, 'failed', 'connection error', 1, ?)`, migrationID, tenant2, now)
	// tenant-3: failed
	_, _ = testDB.Exec(`INSERT INTO `+TableTenantMigrations+` VALUES (?, ?, 'failed', 'timeout', 2, ?)`, migrationID, tenant3, now)

	failed, err := GetFailedTenants(context.Background(), migrationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(failed) != 2 {
		t.Fatalf("expected 2 failed tenants, got %d", len(failed))
	}
}

func TestGetFailedTenants_NoneProcessed(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)

	failed, err := GetFailedTenants(context.Background(), migrationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(failed) != 0 {
		t.Errorf("expected 0 failed tenants, got %d", len(failed))
	}
}

// =============================================================================
// BatchUpdateTenantVersions Tests
// Criteria B: Batch update edge cases
// =============================================================================

func TestBatchUpdateTenantVersions_Multiple(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	id1 := insertTestTenant(t, testDB, "tenant-1", templateID, 1)
	id2 := insertTestTenant(t, testDB, "tenant-2", templateID, 1)
	insertTestTenant(t, testDB, "tenant-3", templateID, 1) // Not updated

	err := BatchUpdateTenantVersions(context.Background(), []int32{id1, id2}, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify updates
	var v1, v2, v3 int
	testDB.QueryRow(`SELECT template_version FROM `+TableTenants+` WHERE id = ?`, id1).Scan(&v1)
	testDB.QueryRow(`SELECT template_version FROM `+TableTenants+` WHERE id = ?`, id2).Scan(&v2)
	testDB.QueryRow(`SELECT template_version FROM `+TableTenants+` WHERE name = 'tenant-3'`).Scan(&v3)

	if v1 != 2 {
		t.Errorf("tenant-1 version = %d, want 2", v1)
	}
	if v2 != 2 {
		t.Errorf("tenant-2 version = %d, want 2", v2)
	}
	if v3 != 1 {
		t.Errorf("tenant-3 version = %d, want 1 (unchanged)", v3)
	}
}

func TestBatchUpdateTenantVersions_Empty(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	err := BatchUpdateTenantVersions(context.Background(), []int32{}, 2)
	if err != nil {
		t.Errorf("empty batch should not error: %v", err)
	}
}

// =============================================================================
// RecordTenantMigration Tests
// Criteria B: Insert and retry scenarios
// =============================================================================

func TestRecordTenantMigration_Success(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)
	tenantID := insertTestTenant(t, testDB, "tenant-1", templateID, 1)

	err := RecordTenantMigration(context.Background(), migrationID, tenantID, "success", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var status string
	var attempts int
	testDB.QueryRow(`SELECT status, attempts FROM `+TableTenantMigrations+` WHERE migration_id = ? AND tenant_id = ?`,
		migrationID, tenantID).Scan(&status, &attempts)

	if status != "success" {
		t.Errorf("status = %s, want success", status)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestRecordTenantMigration_Retry(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)
	tenantID := insertTestTenant(t, testDB, "tenant-1", templateID, 1)

	// First attempt - failed
	err := RecordTenantMigration(context.Background(), migrationID, tenantID, "failed", "connection error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second attempt - success
	err = RecordTenantMigration(context.Background(), migrationID, tenantID, "success", "")
	if err != nil {
		t.Fatalf("unexpected error on retry: %v", err)
	}

	var status string
	var attempts int
	testDB.QueryRow(`SELECT status, attempts FROM `+TableTenantMigrations+` WHERE migration_id = ? AND tenant_id = ?`,
		migrationID, tenantID).Scan(&status, &attempts)

	if status != "success" {
		t.Errorf("status = %s, want success", status)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

// =============================================================================
// generateSchemaSQL Tests
// Criteria A: Core schema generation
// =============================================================================

func TestGenerateSchemaSQL_BasicTable(t *testing.T) {
	schema := Schema{Tables: []Table{
		{
			Name: "users",
			Pk:   []string{"id"},
			Columns: map[string]Col{
				"id":   {Name: "id", Type: "INTEGER"},
				"name": {Name: "name", Type: "TEXT"},
			},
		},
	}}

	sql := generateSchemaSQL(schema)

	if len(sql) < 2 {
		t.Fatalf("expected at least 2 statements, got %d", len(sql))
	}

	// First should be PRAGMA
	if sql[0] != "PRAGMA foreign_keys = ON" {
		t.Errorf("first statement = %s, want PRAGMA foreign_keys = ON", sql[0])
	}

	// Second should be CREATE TABLE
	if sql[1] == "" {
		t.Error("CREATE TABLE statement is empty")
	}
}

func TestGenerateSchemaSQL_WithIndexes(t *testing.T) {
	schema := Schema{Tables: []Table{
		{
			Name: "users",
			Pk:   []string{"id"},
			Columns: map[string]Col{
				"id":    {Name: "id", Type: "INTEGER"},
				"email": {Name: "email", Type: "TEXT"},
			},
			Indexes: []Index{
				{Name: "idx_email", Columns: []string{"email"}, Unique: true},
			},
		},
	}}

	sql := generateSchemaSQL(schema)

	// Should have PRAGMA, CREATE TABLE, and CREATE INDEX
	if len(sql) < 3 {
		t.Fatalf("expected at least 3 statements, got %d", len(sql))
	}

	// Find the index statement
	foundIndex := false
	for _, stmt := range sql {
		if stmt != "" && len(stmt) > 12 && stmt[:12] == "CREATE UNIQU" {
			foundIndex = true
			break
		}
	}
	if !foundIndex {
		t.Error("expected CREATE UNIQUE INDEX statement")
	}
}

func TestGenerateSchemaSQL_WithFTS(t *testing.T) {
	schema := Schema{Tables: []Table{
		{
			Name: "posts",
			Pk:   []string{"id"},
			Columns: map[string]Col{
				"id":      {Name: "id", Type: "INTEGER"},
				"title":   {Name: "title", Type: "TEXT"},
				"content": {Name: "content", Type: "TEXT"},
			},
			FTSColumns: []string{"title", "content"},
		},
	}}

	sql := generateSchemaSQL(schema)

	// Should include FTS setup statements
	foundFTS := false
	for _, stmt := range sql {
		if len(stmt) > 6 && stmt[:6] == "CREATE" && len(stmt) > 20 {
			if stmt[7:20] == "VIRTUAL TABLE" {
				foundFTS = true
				break
			}
		}
	}
	if !foundFTS {
		t.Error("expected FTS virtual table statement")
	}
}

func TestGenerateSchemaSQL_MultipleTables(t *testing.T) {
	schema := Schema{Tables: []Table{
		{
			Name:    "users",
			Pk:      []string{"id"},
			Columns: map[string]Col{"id": {Name: "id", Type: "INTEGER"}},
		},
		{
			Name:    "posts",
			Pk:      []string{"id"},
			Columns: map[string]Col{"id": {Name: "id", Type: "INTEGER"}},
		},
	}}

	sql := generateSchemaSQL(schema)

	// Count CREATE TABLE statements
	createCount := 0
	for _, stmt := range sql {
		if len(stmt) > 12 && stmt[:12] == "CREATE TABLE" {
			createCount++
		}
	}
	if createCount != 2 {
		t.Errorf("expected 2 CREATE TABLE statements, got %d", createCount)
	}
}
