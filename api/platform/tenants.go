package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/joe-ervin05/atomicbase/config"
)

// Common errors for tenant operations.
var (
	ErrTenantNotFound = errors.New("tenant not found")
	ErrTenantExists   = errors.New("tenant already exists")
	ErrTenantInSync   = errors.New("tenant is already at current version")
)

// ListTenants returns all tenants.
func ListTenants(ctx context.Context) ([]Tenant, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, name, template_id, template_version, created_at, updated_at
		FROM %s
		ORDER BY name
	`, TableTenants))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.TemplateID, &t.TemplateVersion, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		// Token is omitted in list responses
		tenants = append(tenants, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if tenants == nil {
		tenants = []Tenant{}
	}

	return tenants, nil
}

// GetTenant returns a tenant by name.
func GetTenant(ctx context.Context, name string) (*Tenant, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	row := conn.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, name, token, template_id, template_version, created_at, updated_at
		FROM %s
		WHERE name = ?
	`, TableTenants), name)

	var t Tenant
	var createdAt, updatedAt string

	if err := row.Scan(&t.ID, &t.Name, &t.Token, &t.TemplateID, &t.TemplateVersion, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}

	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &t, nil
}

// CreateTenant creates a new tenant database using the specified template.
// Creates a Turso database, generates a token, and initializes the schema.
func CreateTenant(ctx context.Context, name, templateName string) (*Tenant, error) {
	// Get template to verify it exists and get current version
	template, err := GetTemplate(ctx, templateName)
	if err != nil {
		return nil, err
	}

	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	// Check if tenant already exists
	var exists int
	err = conn.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE name = ?
	`, TableTenants), name).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, ErrTenantExists
	}

	// Create Turso database
	if err := tursoCreateDatabase(ctx, name); err != nil {
		return nil, fmt.Errorf("failed to create turso database: %w", err)
	}

	// Generate database token
	token, err := tursoCreateToken(ctx, name)
	if err != nil {
		// Try to clean up the database
		_ = tursoDeleteDatabase(ctx, name)
		return nil, fmt.Errorf("failed to create database token: %w", err)
	}

	// Initialize database with template schema
	migrationSQL := generateSchemaSQL(template.Schema)
	if err := BatchExecute(ctx, name, token, migrationSQL); err != nil {
		// Try to clean up
		_ = tursoDeleteDatabase(ctx, name)
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	// Insert tenant record
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (name, token, template_id, template_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, TableTenants), name, token, template.ID, template.CurrentVersion, now, now)
	if err != nil {
		// Try to clean up
		_ = tursoDeleteDatabase(ctx, name)
		return nil, err
	}

	tenantID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	createdAt, _ := time.Parse(time.RFC3339, now)

	return &Tenant{
		ID:              int32(tenantID),
		Name:            name,
		Token:           token,
		TemplateID:      template.ID,
		TemplateVersion: template.CurrentVersion,
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}, nil
}

// DeleteTenant deletes a tenant and its Turso database.
func DeleteTenant(ctx context.Context, name string) error {
	conn, err := getDB()
	if err != nil {
		return err
	}

	// Check if tenant exists
	var tenantID int32
	err = conn.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE name = ?
	`, TableTenants), name).Scan(&tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTenantNotFound
		}
		return err
	}

	// Delete Turso database
	if err := tursoDeleteDatabase(ctx, name); err != nil {
		return fmt.Errorf("failed to delete turso database: %w", err)
	}

	// Delete tenant migration records
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE tenant_id = ?
	`, TableTenantMigrations), tenantID)
	if err != nil {
		return err
	}

	// Delete tenant record
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE id = ?
	`, TableTenants), tenantID)
	if err != nil {
		return err
	}

	return nil
}

// SyncTenant synchronizes a tenant to the template's current version.
// Applies chained migrations if needed (e.g., v1->v2->v3).
// This is synchronous - waits for completion.
func SyncTenant(ctx context.Context, name string) (*SyncTenantResponse, error) {
	// Get tenant
	tenant, err := GetTenant(ctx, name)
	if err != nil {
		return nil, err
	}

	// Get template to find current version
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	var currentVersion int
	err = conn.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT current_version FROM %s WHERE id = ?
	`, TableTemplates), tenant.TemplateID).Scan(&currentVersion)
	if err != nil {
		return nil, err
	}

	// Check if already at current version
	if tenant.TemplateVersion >= currentVersion {
		return nil, ErrTenantInSync
	}

	fromVersion := tenant.TemplateVersion

	// Collect all migration SQL from tenant's version to current
	var allSQL []string
	for v := tenant.TemplateVersion; v < currentVersion; v++ {
		migration, err := GetMigrationByVersions(ctx, tenant.TemplateID, v, v+1)
		if err != nil {
			return nil, fmt.Errorf("migration from v%d to v%d not found: %w", v, v+1, err)
		}
		allSQL = append(allSQL, migration.SQL...)
	}

	// Execute all migrations
	if len(allSQL) > 0 {
		if err := BatchExecute(ctx, tenant.Name, tenant.Token, allSQL); err != nil {
			return nil, fmt.Errorf("migration failed: %w", err)
		}
	}

	// Update tenant version
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET template_version = ?, updated_at = ? WHERE id = ?
	`, TableTenants), currentVersion, now, tenant.ID)
	if err != nil {
		return nil, err
	}

	return &SyncTenantResponse{
		FromVersion: fromVersion,
		ToVersion:   currentVersion,
	}, nil
}

// GetMigrationByVersions returns a migration for a specific version transition.
func GetMigrationByVersions(ctx context.Context, templateID int32, fromVersion, toVersion int) (*Migration, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	row := conn.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, template_id, from_version, to_version, sql, status, state,
			   total_dbs, completed_dbs, failed_dbs, started_at, completed_at, created_at
		FROM %s
		WHERE template_id = ? AND from_version = ? AND to_version = ?
	`, TableMigrations), templateID, fromVersion, toVersion)

	var m Migration
	var sqlJSON string
	var state sql.NullString
	var startedAt, completedAt sql.NullString
	var createdAt string

	if err := row.Scan(
		&m.ID, &m.TemplateID, &m.FromVersion, &m.ToVersion, &sqlJSON,
		&m.Status, &state, &m.TotalDBs, &m.CompletedDBs, &m.FailedDBs,
		&startedAt, &completedAt, &createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("migration not found")
		}
		return nil, err
	}

	if err := json.Unmarshal([]byte(sqlJSON), &m.SQL); err != nil {
		return nil, fmt.Errorf("failed to decode migration SQL: %w", err)
	}

	if state.Valid {
		m.State = &state.String
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339, startedAt.String)
		m.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		m.CompletedAt = &t
	}

	return &m, nil
}

// GetTenantsByTemplate returns all tenants using a specific template.
func GetTenantsByTemplate(ctx context.Context, templateID int32) ([]Tenant, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, name, token, template_id, template_version, created_at, updated_at
		FROM %s
		WHERE template_id = ?
		ORDER BY name
	`, TableTenants), templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.Token, &t.TemplateID, &t.TemplateVersion, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		tenants = append(tenants, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if tenants == nil {
		tenants = []Tenant{}
	}

	return tenants, nil
}

// GetPendingTenants returns tenants that need migration for a given job.
// Pending = template_version < target AND not in tenant_migrations table for this job.
func GetPendingTenants(ctx context.Context, migrationID int64, templateID int32, targetVersion int) ([]Tenant, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
		SELECT t.id, t.name, t.token, t.template_id, t.template_version, t.created_at, t.updated_at
		FROM %s t
		WHERE t.template_id = ?
		  AND t.template_version < ?
		  AND NOT EXISTS (
			SELECT 1 FROM %s tm
			WHERE tm.tenant_id = t.id AND tm.migration_id = ?
		  )
		ORDER BY t.name
	`, TableTenants, TableTenantMigrations), templateID, targetVersion, migrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.Token, &t.TemplateID, &t.TemplateVersion, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		tenants = append(tenants, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if tenants == nil {
		tenants = []Tenant{}
	}

	return tenants, nil
}

// GetFailedTenants returns tenants that failed migration for a given job.
func GetFailedTenants(ctx context.Context, migrationID int64) ([]Tenant, error) {
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
		SELECT t.id, t.name, t.token, t.template_id, t.template_version, t.created_at, t.updated_at
		FROM %s t
		JOIN %s tm ON tm.tenant_id = t.id
		WHERE tm.migration_id = ? AND tm.status = 'failed'
		ORDER BY t.name
	`, TableTenants, TableTenantMigrations), migrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.Token, &t.TemplateID, &t.TemplateVersion, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		tenants = append(tenants, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if tenants == nil {
		tenants = []Tenant{}
	}

	return tenants, nil
}

// BatchUpdateTenantVersions updates template_version for multiple tenants in one query.
func BatchUpdateTenantVersions(ctx context.Context, tenantIDs []int32, version int) error {
	if len(tenantIDs) == 0 {
		return nil
	}

	conn, err := getDB()
	if err != nil {
		return err
	}

	// Build placeholder string
	placeholders := make([]byte, 0, len(tenantIDs)*2)
	args := make([]any, 0, len(tenantIDs)+2)
	args = append(args, version, time.Now().UTC().Format(time.RFC3339))

	for i, id := range tenantIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, id)
	}

	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET template_version = ?, updated_at = ? WHERE id IN (%s)
	`, TableTenants, string(placeholders)), args...)

	return err
}

// RecordTenantMigration records the outcome of a tenant migration.
func RecordTenantMigration(ctx context.Context, migrationID int64, tenantID int32, status, errMsg string) error {
	conn, err := getDB()
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Use INSERT OR REPLACE to handle retries
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (migration_id, tenant_id, status, error, attempts, updated_at)
		VALUES (?, ?, ?, ?, 1, ?)
		ON CONFLICT(migration_id, tenant_id) DO UPDATE SET
			status = excluded.status,
			error = excluded.error,
			attempts = attempts + 1,
			updated_at = excluded.updated_at
	`, TableTenantMigrations), migrationID, tenantID, status, errMsg, now)

	return err
}

// generateSchemaSQL generates the SQL statements to create a schema from scratch.
func generateSchemaSQL(schema Schema) []string {
	var sql []string

	// Enable foreign keys
	sql = append(sql, "PRAGMA foreign_keys = ON")

	// Create tables
	for _, table := range schema.Tables {
		sql = append(sql, generateCreateTableSQL(table))

		// Create indexes
		for _, idx := range table.Indexes {
			sql = append(sql, generateCreateIndexSQL(table.Name, idx))
		}

		// Create FTS if configured
		if len(table.FTSColumns) > 0 {
			sql = append(sql, generateFTSSQL(table.Name, table.FTSColumns, table.Pk)...)
		}
	}

	return sql
}

// =============================================================================
// Turso API Integration
// =============================================================================

// tursoCreateDatabase creates a new database in Turso.
func tursoCreateDatabase(ctx context.Context, name string) error {
	org := config.Cfg.TursoOrganization
	apiKey := config.Cfg.TursoAPIKey
	if org == "" || apiKey == "" {
		return fmt.Errorf("TURSO_ORGANIZATION and TURSO_API_KEY must be set")
	}

	body, err := json.Marshal(map[string]string{
		"name":  name,
		"group": "default",
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases", org)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		var errBody bytes.Buffer
		errBody.ReadFrom(resp.Body)
		return fmt.Errorf("turso API error: %s - %s", resp.Status, errBody.String())
	}

	return nil
}

// tursoDeleteDatabase deletes a database from Turso.
func tursoDeleteDatabase(ctx context.Context, name string) error {
	org := config.Cfg.TursoOrganization
	apiKey := config.Cfg.TursoAPIKey
	if org == "" || apiKey == "" {
		return fmt.Errorf("TURSO_ORGANIZATION and TURSO_API_KEY must be set")
	}

	url := fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s", org, name)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 200 = deleted, 404 = already gone (both OK)
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		var errBody bytes.Buffer
		errBody.ReadFrom(resp.Body)
		return fmt.Errorf("turso API error: %s - %s", resp.Status, errBody.String())
	}

	return nil
}

// tursoCreateToken creates a database access token.
func tursoCreateToken(ctx context.Context, dbName string) (string, error) {
	org := config.Cfg.TursoOrganization
	apiKey := config.Cfg.TursoAPIKey
	if org == "" || apiKey == "" {
		return "", fmt.Errorf("TURSO_ORGANIZATION and TURSO_API_KEY must be set")
	}

	// Build request body with expiration
	expiration := config.Cfg.TursoTokenExpiration
	body, err := json.Marshal(map[string]any{
		"expiration": expiration,
		"read_only":  false,
	})
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s/auth/tokens", org, dbName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errBody bytes.Buffer
		errBody.ReadFrom(resp.Body)
		return "", fmt.Errorf("turso API error: %s - %s", resp.Status, errBody.String())
	}

	var tokenResp struct {
		JWT string `json:"jwt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	return tokenResp.JWT, nil
}
