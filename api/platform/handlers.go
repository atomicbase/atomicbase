package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/joe-ervin05/atomicbase/tools"
)

func validateResourceName(name string) (code, message, hint string) {
	if len(name) == 0 {
		return tools.CodeInvalidName, "name cannot be empty",
			"Names must be 1-64 characters, containing only lowercase letters, numbers, and dashes."
	}
	if len(name) > 64 {
		return tools.CodeInvalidName, "name exceeds maximum length of 64 characters",
			"Names must be 1-64 characters, containing only lowercase letters, numbers, and dashes."
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return tools.CodeInvalidName, "name contains invalid characters",
				"Names must contain only lowercase letters, numbers, and dashes."
		}
	}
	return "", "", ""
}

// encodeSchemaForStorage encodes schema for database storage.
func encodeSchemaForStorage(schema Schema) ([]byte, error) {
	return tools.EncodeSchema(schema)
}

// RegisterRoutes registers all platform API routes.
func RegisterRoutes(mux *http.ServeMux) {
	// Templates
	mux.HandleFunc("GET /platform/templates", handleListTemplates)
	mux.HandleFunc("GET /platform/templates/{name}", handleGetTemplate)
	mux.HandleFunc("POST /platform/templates", handleCreateTemplate)
	mux.HandleFunc("DELETE /platform/templates/{name}", handleDeleteTemplate)
	mux.HandleFunc("POST /platform/templates/{name}/diff", handleDiffTemplate)
	mux.HandleFunc("POST /platform/templates/{name}/migrate", handleMigrateTemplate)
	mux.HandleFunc("POST /platform/templates/{name}/rollback", handleRollbackTemplate)
	mux.HandleFunc("GET /platform/templates/{name}/history", handleGetTemplateHistory)

	// Tenants
	mux.HandleFunc("GET /platform/tenants", handleListTenants)
	mux.HandleFunc("GET /platform/tenants/{name}", handleGetTenant)
	mux.HandleFunc("POST /platform/tenants", handleCreateTenant)
	mux.HandleFunc("DELETE /platform/tenants/{name}", handleDeleteTenant)
	mux.HandleFunc("POST /platform/tenants/{name}/sync", handleSyncTenant)

	// Migrations
	mux.HandleFunc("GET /platform/migrations", handleListMigrations)
	mux.HandleFunc("GET /platform/migrations/{id}", handleGetMigration)
	mux.HandleFunc("POST /platform/migrations/{id}/retry", handleRetryMigration)
}

// =============================================================================
// Template Handlers
// =============================================================================

func handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := ListTemplates(r.Context())
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templates)
}

func handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("template name is required"))
		return
	}

	template, err := GetTemplate(r.Context(), name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, template)
}

func handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		tools.RespErr(w, tools.ErrInvalidJSON)
		return
	}

	if req.Name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("name is required"))
		return
	}

	if code, msg, _ := validateResourceName(req.Name); code != "" {
		tools.RespErr(w, tools.InvalidRequestErr(msg))
		return
	}

	if len(req.Schema.Tables) == 0 {
		tools.RespErr(w, tools.InvalidRequestErr("schema must have at least one table"))
		return
	}

	template, err := CreateTemplate(r.Context(), req.Name, req.Schema)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, template)
}

func handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("template name is required"))
		return
	}

	err := DeleteTemplate(r.Context(), name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleDiffTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("template name is required"))
		return
	}

	var req DiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		tools.RespErr(w, tools.ErrInvalidJSON)
		return
	}

	result, err := DiffTemplate(r.Context(), name, req.Schema)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func handleMigrateTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("template name is required"))
		return
	}

	var req MigrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		tools.RespErr(w, tools.ErrInvalidJSON)
		return
	}

	ctx := r.Context()

	// Get current template
	template, err := GetTemplate(ctx, name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	// Check for concurrent migration
	jm := GetJobManager()
	if jm.IsRunning(template.ID) {
		tools.RespErr(w, tools.ErrAtomicbaseBusy)
		return
	}

	// Diff schemas
	changes := diffSchemas(template.Schema, req.Schema)
	if len(changes) == 0 {
		tools.RespErr(w, tools.ErrNoChanges)
		return
	}

	// Generate migration plan
	plan, err := GenerateMigrationPlan(template.Schema, req.Schema, changes, req.Merge)
	if err != nil {
		tools.RespErr(w, tools.InvalidMigrationErr(err.Error()))
		return
	}

	// Validate migration
	// Get first tenant for probe database (if any exist)
	tenants, err := GetTenantsByTemplate(ctx, template.ID)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	// Validate FK references (no DB needed)
	validationResult, err := ValidateMigrationPlan(ctx, req.Schema, nil)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	if !validationResult.Valid {
		// Join validation errors into a single message for standard format
		errMsg := validationResult.Errors[0].Message
		if len(validationResult.Errors) > 1 {
			errMsg = fmt.Sprintf("%d errors: %s", len(validationResult.Errors), errMsg)
		}
		tools.RespErr(w, fmt.Errorf("validation failed: %s", errMsg))
		return
	}

	// Create new version in history
	newVersion := template.CurrentVersion + 1
	schemaJSON, err := encodeSchemaForStorage(req.Schema)
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	hash := sha256.Sum256(schemaJSON)
	checksum := hex.EncodeToString(hash[:])

	conn, err := getDB()
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// If no tenants, use a transaction to atomically create version and update template
	if len(tenants) == 0 {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			tools.RespErr(w, err)
			return
		}
		defer tx.Rollback()

		// Insert history record
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO %s (template_id, version, schema, checksum, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, TableTemplatesHistory), template.ID, newVersion, schemaJSON, checksum, now)
		if err != nil {
			tools.RespErr(w, err)
			return
		}

		// Update template version
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s SET current_version = ?, updated_at = ? WHERE id = ?
		`, TableTemplates), newVersion, now, template.ID)
		if err != nil {
			tools.RespErr(w, err)
			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			tools.RespErr(w, err)
			return
		}

		// Invalidate schema cache so next request loads the new version
		tools.InvalidateTemplate(template.ID)

		// Create migration record (outside transaction, ok if this fails)
		migration, err := CreateMigration(ctx, template.ID, template.CurrentVersion, newVersion, plan.SQL)
		if err != nil {
			// Migration record is optional for tracking, version update already succeeded
			log.Printf("Warning: failed to create migration record: %v", err)
			respondJSON(w, http.StatusAccepted, MigrateResponse{MigrationID: 0})
			return
		}

		state := MigrationStateSuccess
		_ = UpdateMigrationStatus(ctx, migration.ID, MigrationStatusComplete, &state, 0, 0)

		respondJSON(w, http.StatusAccepted, MigrateResponse{MigrationID: migration.ID})
		return
	}

	// With tenants: insert history first, then start background job
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (template_id, version, schema, checksum, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, TableTemplatesHistory), template.ID, newVersion, schemaJSON, checksum, now)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	// Create migration record
	migration, err := CreateMigration(ctx, template.ID, template.CurrentVersion, newVersion, plan.SQL)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	// Start background migration job
	RunMigrationJob(ctx, migration.ID)

	respondJSON(w, http.StatusAccepted, MigrateResponse{MigrationID: migration.ID})
}

func handleRollbackTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("template name is required"))
		return
	}

	var req RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		tools.RespErr(w, tools.ErrInvalidJSON)
		return
	}

	if req.Version < 1 {
		tools.RespErr(w, tools.InvalidRequestErr("version must be at least 1"))
		return
	}

	ctx := r.Context()

	// Get current template
	template, err := GetTemplate(ctx, name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	if req.Version >= template.CurrentVersion {
		tools.RespErr(w, tools.InvalidRequestErr(fmt.Sprintf("rollback version must be less than current version %d", template.CurrentVersion)))
		return
	}

	// Check for concurrent migration
	jm := GetJobManager()
	if jm.IsRunning(template.ID) {
		tools.RespErr(w, tools.ErrAtomicbaseBusy)
		return
	}

	// Get target version schema
	targetVersion, err := GetTemplateVersion(ctx, template.ID, req.Version)
	if err != nil {
		tools.RespErr(w, tools.VersionNotFoundErr(req.Version))
		return
	}

	// Diff current schema to target schema
	changes := diffSchemas(template.Schema, targetVersion.Schema)

	// Generate migration plan (current -> target)
	plan, err := GenerateMigrationPlan(template.Schema, targetVersion.Schema, changes, nil)
	if err != nil {
		tools.RespErr(w, tools.InvalidMigrationErr(err.Error()))
		return
	}

	// Create new version in history (rollback creates a NEW version with the old schema)
	newVersion := template.CurrentVersion + 1
	schemaJSON, err := encodeSchemaForStorage(targetVersion.Schema)
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	hash := sha256.Sum256(schemaJSON)
	checksum := hex.EncodeToString(hash[:])

	conn, err := getDB()
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (template_id, version, schema, checksum, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, TableTemplatesHistory), template.ID, newVersion, schemaJSON, checksum, now)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	// Create migration record
	migration, err := CreateMigration(ctx, template.ID, template.CurrentVersion, newVersion, plan.SQL)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	// Start background migration job
	RunMigrationJob(ctx, migration.ID)

	respondJSON(w, http.StatusAccepted, RollbackResponse{MigrationID: migration.ID})
}

func handleGetTemplateHistory(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("template name is required"))
		return
	}

	history, err := GetTemplateHistory(r.Context(), name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, history)
}

// =============================================================================
// Tenant Handlers
// =============================================================================

func handleListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := ListTenants(r.Context())
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, tenants)
}

func handleGetTenant(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("tenant name is required"))
		return
	}

	tenant, err := GetTenant(r.Context(), name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, tenant)
}

func handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		tools.RespErr(w, tools.ErrInvalidJSON)
		return
	}

	if req.Name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("name is required"))
		return
	}

	if code, msg, _ := validateResourceName(req.Name); code != "" {
		tools.RespErr(w, tools.InvalidRequestErr(msg))
		return
	}

	if req.Template == "" {
		tools.RespErr(w, tools.InvalidRequestErr("template is required"))
		return
	}

	tenant, err := CreateTenant(r.Context(), req.Name, req.Template)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, tenant)
}

func handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("tenant name is required"))
		return
	}

	err := DeleteTenant(r.Context(), name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleSyncTenant(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("tenant name is required"))
		return
	}

	result, err := SyncTenant(r.Context(), name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// =============================================================================
// Migration Handlers
// =============================================================================

func handleListMigrations(w http.ResponseWriter, r *http.Request) {
	// Optional status filter
	status := r.URL.Query().Get("status")

	migrations, err := ListMigrations(r.Context(), status)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, migrations)
}

func handleGetMigration(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		tools.RespErr(w, tools.InvalidRequestErr("migration id is required"))
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		tools.RespErr(w, tools.InvalidRequestErr("invalid migration id"))
		return
	}

	migration, err := GetMigration(r.Context(), id)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, migration)
}

func handleRetryMigration(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		tools.RespErr(w, tools.InvalidRequestErr("migration id is required"))
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		tools.RespErr(w, tools.InvalidRequestErr("invalid migration id"))
		return
	}

	result, err := RetryFailedTenants(r.Context(), id)
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// =============================================================================
// Response Helpers
// =============================================================================

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
