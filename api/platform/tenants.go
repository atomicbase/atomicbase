// Package platform provides the Platform API for tenant and template management.
package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/joe-ervin05/atomicbase/config"
	"github.com/joe-ervin05/atomicbase/data"
)

// readAPIError reads the error message from a Turso API error response.
func readAPIError(res *http.Response) error {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("turso API error: %s", res.Status)
	}
	if len(body) > 0 {
		return fmt.Errorf("turso API error: %s - %s", res.Status, string(body))
	}
	return fmt.Errorf("turso API error: %s", res.Status)
}

// deleteTursoDB deletes a database from Turso. Used for cleanup on CreateDB failure.
func deleteTursoDB(ctx context.Context, name string) error {
	org := config.Cfg.TursoOrganization
	token := config.Cfg.TursoAPIKey

	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s", org, name), nil)
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(request)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return readAPIError(res)
	}

	return nil
}

// ListDBs returns all tenant databases.
func ListDBs(ctx context.Context, dao data.PrimaryDao) ([]byte, error) {
	// Exclude primary database (id=1, name=NULL) from the list
	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`SELECT 
	json_group_array(json_object('id', id, 'name', name, 'template_id', template_id, 'template_version', template_version)) 
	AS data FROM (SELECT id, name, template_id, template_version from %s WHERE name IS NOT NULL ORDER BY id)`, data.ReservedTableDatabases))

	if row.Err() != nil {
		return nil, row.Err()
	}

	var res []byte

	err := row.Scan(&res)

	return res, err
}

// CreateDB creates a new tenant database via Turso API with a required template.
func CreateDB(ctx context.Context, dao data.PrimaryDao, body io.ReadCloser) ([]byte, error) {
	type reqBody struct {
		Name     string `json:"name"`
		Template string `json:"template"`
		Group    string `json:"group"`
	}

	var bod reqBody
	if err := json.NewDecoder(body).Decode(&bod); err != nil {
		return nil, err
	}

	if bod.Name == "" {
		return nil, errors.New("name is required")
	}
	if bod.Template == "" {
		return nil, errors.New("template is required")
	}
	if bod.Group == "" {
		bod.Group = "default"
	}

	// Get the template first to ensure it exists
	template, err := GetTemplate(ctx, dao, bod.Template)
	if err != nil {
		return nil, err
	}

	org := config.Cfg.TursoOrganization
	if org == "" {
		return nil, errors.New("TURSO_ORGANIZATION is not set but is required for managing turso databases")
	}
	token := config.Cfg.TursoAPIKey
	if token == "" {
		return nil, errors.New("TURSO_API_KEY is not set but is required for managing turso databases")
	}

	// Create database via Turso API
	tursoReq := struct {
		Name  string `json:"name"`
		Group string `json:"group"`
	}{Name: bod.Name, Group: bod.Group}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(tursoReq); err != nil {
		return nil, err
	}

	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases", org), &buf)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, readAPIError(res)
	}

	// Parse response to get hostname
	var createResp struct {
		Database struct {
			Hostname string `json:"Hostname"`
		} `json:"database"`
	}
	if err := json.NewDecoder(res.Body).Decode(&createResp); err != nil {
		return nil, fmt.Errorf("failed to parse Turso response: %w", err)
	}

	// Track if we need to cleanup the Turso database on failure
	var success bool
	defer func() {
		if !success {
			// Best-effort cleanup - ignore errors since we're already returning an error
			_ = deleteTursoDB(ctx, bod.Name)
		}
	}()

	// Create auth token for the new database
	newToken, err := createDBToken(ctx, bod.Name)
	if err != nil {
		return nil, err
	}

	// Connect to the new database
	connStr := fmt.Sprintf("libsql://%s?authToken=%s", createResp.Database.Hostname, newToken)
	dbClient, err := sql.Open("libsql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to new database: %w", err)
	}
	defer dbClient.Close()

	if err := dbClient.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping new database: %w", err)
	}

	targetDB := data.Database{Client: dbClient}

	// Apply template schema to the new database
	_, err = syncSchemaToDatabase(ctx, &targetDB, template.Tables, false)
	if err != nil {
		return nil, fmt.Errorf("failed to apply template schema: %w", err)
	}

	// Insert database record with template association
	_, err = dao.Client.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (name, token, template_id, template_version)
		VALUES (?, ?, (SELECT id FROM %s WHERE name = ?), 1)
	`, data.ReservedTableDatabases, data.ReservedTableTemplates), bod.Name, newToken, bod.Template)
	if err != nil {
		return nil, err
	}

	success = true
	return []byte(fmt.Sprintf(`{"message":"database %s created with template %s"}`, bod.Name, bod.Template)), nil
}

// ImportDB registers an existing Turso database with a template.
// The database schema must match the template schema.
func ImportDB(ctx context.Context, dao data.PrimaryDao, body io.ReadCloser) ([]byte, error) {
	type reqBody struct {
		Database string `json:"database"`
		Template string `json:"template"`
	}

	var bod reqBody
	if err := json.NewDecoder(body).Decode(&bod); err != nil {
		return nil, err
	}

	if bod.Database == "" {
		return nil, errors.New("database is required")
	}
	if bod.Template == "" {
		return nil, errors.New("template is required")
	}

	// Check if database is already registered
	var existingID sql.NullInt32
	err := dao.Client.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT id FROM %s WHERE name = ?", data.ReservedTableDatabases), bod.Database).Scan(&existingID)
	if err == nil {
		return nil, fmt.Errorf("database %s is already registered", bod.Database)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Get the template
	template, err := GetTemplate(ctx, dao, bod.Template)
	if err != nil {
		return nil, err
	}

	org := config.Cfg.TursoOrganization
	if org == "" {
		return nil, errors.New("TURSO_ORGANIZATION is not set but is required for managing turso databases")
	}
	apiToken := config.Cfg.TursoAPIKey
	if apiToken == "" {
		return nil, errors.New("TURSO_API_KEY is not set but is required for managing turso databases")
	}

	// Get database info from Turso API to verify it exists
	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s", org, bod.Database), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+apiToken)

	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, readAPIError(res)
	}

	var dbResp struct {
		Database struct {
			Hostname string `json:"Hostname"`
		} `json:"database"`
	}
	if err := json.NewDecoder(res.Body).Decode(&dbResp); err != nil {
		return nil, err
	}

	// Create auth token for the database
	dbToken, err := createDBToken(ctx, bod.Database)
	if err != nil {
		return nil, err
	}

	// Connect to the database
	connStr := fmt.Sprintf("libsql://%s?authToken=%s", dbResp.Database.Hostname, dbToken)
	dbClient, err := sql.Open("libsql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dbClient.Close()

	if err := dbClient.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Get current schema from the database
	currentTables, err := data.SchemaCols(dbClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get database schema: %w", err)
	}

	// Validate schema matches template
	mismatches := validateSchemaMatchesTemplate(currentTables, template.Tables)
	if len(mismatches) > 0 {
		return nil, fmt.Errorf("schema mismatch: %v", mismatches)
	}

	// Insert database record with template association
	_, err = dao.Client.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (name, token, template_id, template_version)
		VALUES (?, ?, (SELECT id FROM %s WHERE name = ?), 1)
	`, data.ReservedTableDatabases, data.ReservedTableTemplates), bod.Database, dbToken, bod.Template)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf(`{"message":"database %s imported with template %s"}`, bod.Database, bod.Template)), nil
}

// validateSchemaMatchesTemplate checks if a database schema matches a template.
// Returns a list of mismatches, or empty slice if schema matches.
func validateSchemaMatchesTemplate(dbTables map[string]data.Table, templateTables []data.Table) []string {
	var mismatches []string

	for _, templateTable := range templateTables {
		dbTable, exists := dbTables[templateTable.Name]
		if !exists {
			mismatches = append(mismatches, fmt.Sprintf("missing table: %s", templateTable.Name))
			continue
		}

		// Check columns
		for colName, templateCol := range templateTable.Columns {
			dbCol, colExists := dbTable.Columns[colName]
			if !colExists {
				mismatches = append(mismatches, fmt.Sprintf("missing column: %s.%s", templateTable.Name, colName))
				continue
			}

			// Check column type (case-insensitive)
			if !strings.EqualFold(dbCol.Type, templateCol.Type) {
				mismatches = append(mismatches, fmt.Sprintf("column type mismatch: %s.%s (expected %s, got %s)",
					templateTable.Name, colName, templateCol.Type, dbCol.Type))
			}
		}
	}

	return mismatches
}

// DeleteDB deletes a tenant database.
func DeleteDB(ctx context.Context, dao data.PrimaryDao, name string) ([]byte, error) {

	_, err := dao.Client.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE name = ?", data.ReservedTableDatabases), name)
	if err != nil {
		return nil, err
	}

	org := config.Cfg.TursoOrganization
	if org == "" {
		return nil, errors.New("TURSO_ORGANIZATION is not set but is required for managing turso databases")
	}
	token := config.Cfg.TursoAPIKey
	if token == "" {
		return nil, errors.New("TURSO_API_KEY is not set but is required for managing turso databases")
	}

	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s", org, name), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, readAPIError(res)
	}

	return []byte(fmt.Sprintf(`{"message":"database %s deleted"}`, name)), nil
}

// createDBToken creates a new auth token for a Turso database.
// If TURSO_TOKEN_EXPIRATION is set (e.g., "7d", "30d", "never"), it will be used.
func createDBToken(ctx context.Context, dbName string) (string, error) {
	type jwtBody struct {
		Jwt string `json:"jwt"`
	}

	org := config.Cfg.TursoOrganization
	if org == "" {
		return "", errors.New("TURSO_ORGANIZATION is not set but is required for managing turso databases")
	}
	token := config.Cfg.TursoAPIKey
	if token == "" {
		return "", errors.New("TURSO_API_KEY is not set but is required for managing turso databases")
	}

	// Build URL with optional expiration parameter
	url := fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s/auth/tokens", org, dbName)
	if expiration := config.Cfg.TursoTokenExpiration; expiration != "" {
		url += "?expiration=" + expiration
	}

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", readAPIError(res)
	}

	var jwtBod jwtBody
	if err := json.NewDecoder(res.Body).Decode(&jwtBod); err != nil {
		return "", err
	}

	return jwtBod.Jwt, nil
}

// SyncTenant migrates a tenant database to the latest template version.
// Returns the migration plan that was applied.
func SyncTenant(ctx context.Context, dao data.PrimaryDao, tenantName string) (MigrationPlan, error) {
	// Get tenant database info
	var dbID int32
	var templateID int32
	var currentVersion int
	var token string

	err := dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT d.id, d.template_id, COALESCE(d.template_version, 1), d.token
		FROM %s d
		WHERE d.name = ?
	`, data.ReservedTableDatabases), tenantName).Scan(&dbID, &templateID, &currentVersion, &token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MigrationPlan{}, fmt.Errorf("tenant %s not found", tenantName)
		}
		return MigrationPlan{}, err
	}

	if templateID == 0 {
		return MigrationPlan{}, fmt.Errorf("tenant %s has no associated template", tenantName)
	}

	// Get template current version
	var targetVersion int
	err = dao.Client.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COALESCE(current_version, 1) FROM %s WHERE id = ?
	`, data.ReservedTableTemplates), templateID).Scan(&targetVersion)
	if err != nil {
		return MigrationPlan{}, fmt.Errorf("failed to get template version: %w", err)
	}

	if currentVersion >= targetVersion {
		// Already up to date
		return MigrationPlan{}, nil
	}

	// Get the schemas for current and target versions
	oldTables, err := getVersionTables(ctx, dao, templateID, currentVersion)
	if err != nil {
		return MigrationPlan{}, fmt.Errorf("failed to get current schema: %w", err)
	}

	newTables, err := getVersionTables(ctx, dao, templateID, targetVersion)
	if err != nil {
		return MigrationPlan{}, fmt.Errorf("failed to get target schema: %w", err)
	}

	// Generate migration plan
	plan := GenerateMigrationPlan(oldTables, newTables)

	if len(plan.MigrationSQL) == 0 {
		// No actual changes needed, just update version
		_, err = dao.Client.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s SET template_version = ? WHERE id = ?
		`, data.ReservedTableDatabases), targetVersion, dbID)
		return plan, err
	}

	// Connect to tenant database
	org := config.Cfg.TursoOrganization
	dbURL := fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s", org, tenantName)

	// Get database hostname
	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, "GET", dbURL, nil)
	if err != nil {
		return plan, err
	}
	request.Header.Set("Authorization", "Bearer "+config.Cfg.TursoAPIKey)

	res, err := client.Do(request)
	if err != nil {
		return plan, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return plan, readAPIError(res)
	}

	var dbResp struct {
		Database struct {
			Hostname string `json:"Hostname"`
		} `json:"database"`
	}
	if err := json.NewDecoder(res.Body).Decode(&dbResp); err != nil {
		return plan, err
	}

	// Connect to tenant database
	connStr := fmt.Sprintf("libsql://%s?authToken=%s", dbResp.Database.Hostname, token)
	tenantDB, err := sql.Open("libsql", connStr)
	if err != nil {
		return plan, fmt.Errorf("failed to connect to tenant database: %w", err)
	}
	defer tenantDB.Close()

	// Apply migration
	if err := MigrateDatabase(ctx, tenantDB, plan); err != nil {
		return plan, fmt.Errorf("migration failed: %w", err)
	}

	// Update template_version in primary database
	_, err = dao.Client.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET template_version = ? WHERE id = ?
	`, data.ReservedTableDatabases), targetVersion, dbID)
	if err != nil {
		return plan, fmt.Errorf("migration succeeded but failed to update version: %w", err)
	}

	return plan, nil
}
