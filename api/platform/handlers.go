package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/joe-ervin05/atomicbase/tools"
)

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

	// Jobs
	mux.HandleFunc("GET /platform/jobs/{id}", handleGetJob)
	mux.HandleFunc("POST /platform/jobs/{id}/retry", handleRetryJob)
}

// =============================================================================
// Template Handlers
// =============================================================================

func handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := ListTemplates(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	respondJSON(w, http.StatusOK, templates)
}

func handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "template name is required", "")
		return
	}

	template, err := GetTemplate(r.Context(), name)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			respondError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", err.Error(),
				"Use GET /platform/templates to list available templates.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	respondJSON(w, http.StatusOK, template)
}

func handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "Check JSON syntax.")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "name is required", "")
		return
	}

	if len(req.Schema.Tables) == 0 {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "schema must have at least one table", "")
		return
	}

	template, err := CreateTemplate(r.Context(), req.Name, req.Schema)
	if err != nil {
		if errors.Is(err, ErrTemplateExists) {
			respondError(w, http.StatusConflict, "TEMPLATE_EXISTS", err.Error(),
				"Choose a different name or delete the existing template first.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	respondJSON(w, http.StatusCreated, template)
}

func handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "template name is required", "")
		return
	}

	err := DeleteTemplate(r.Context(), name)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			respondError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", err.Error(),
				"Use GET /platform/templates to list available templates.")
			return
		}
		if errors.Is(err, ErrTemplateInUse) {
			respondError(w, http.StatusConflict, "TEMPLATE_IN_USE", err.Error(),
				"Delete all tenants using this template first.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleDiffTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "template name is required", "")
		return
	}

	var req DiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "Check JSON syntax.")
		return
	}

	result, err := DiffTemplate(r.Context(), name, req.Schema)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			respondError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", err.Error(),
				"Use GET /platform/templates to list available templates.")
			return
		}
		if errors.Is(err, ErrNoChanges) {
			respondError(w, http.StatusBadRequest, "NO_CHANGES", err.Error(),
				"The provided schema is identical to the current version.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func handleMigrateTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "template name is required", "")
		return
	}

	var req MigrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "Check JSON syntax.")
		return
	}

	ctx := r.Context()

	// Get current template
	template, err := GetTemplate(ctx, name)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			respondError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", err.Error(),
				"Use GET /platform/templates to list available templates.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	// Check for concurrent migration
	jm := GetJobManager()
	if jm.IsRunning(template.ID) {
		respondError(w, http.StatusConflict, "ATOMICBASE_BUSY",
			"another migration is already in progress",
			"Wait for the current migration to complete or check job status.")
		return
	}

	// Diff schemas
	changes := diffSchemas(template.Schema, req.Schema)
	if len(changes) == 0 {
		respondError(w, http.StatusBadRequest, "NO_CHANGES", "no schema changes detected",
			"The provided schema is identical to the current version.")
		return
	}

	// Generate migration plan
	plan, err := GenerateMigrationPlan(template.Schema, req.Schema, changes, req.Merge)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_MIGRATION", err.Error(), "")
		return
	}

	// Validate migration
	// Get first tenant for probe database (if any exist)
	tenants, err := GetTenantsByTemplate(ctx, template.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	// Validate FK references (no DB needed)
	validationResult, err := ValidateMigrationPlan(ctx, req.Schema, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	if !validationResult.Valid {
		respondJSON(w, http.StatusBadRequest, map[string]any{
			"code":   "VALIDATION_FAILED",
			"errors": validationResult.Errors,
		})
		return
	}

	// Create new version in history
	newVersion := template.CurrentVersion + 1
	schemaJSON, _ := json.Marshal(req.Schema)
	hash := sha256.Sum256(schemaJSON)
	checksum := hex.EncodeToString(hash[:])

	conn, err := getDB()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (template_id, version, schema, checksum, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, TableTemplatesHistory), template.ID, newVersion, schemaJSON, checksum, now)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	// Create migration record
	migration, err := CreateMigration(ctx, template.ID, template.CurrentVersion, newVersion, plan.SQL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	// If no tenants, complete immediately
	if len(tenants) == 0 {
		// Update template version directly
		_, err = conn.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s SET current_version = ?, updated_at = ? WHERE id = ?
		`, TableTemplates), newVersion, now, template.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
			return
		}

		state := MigrationStateSuccess
		_ = UpdateMigrationStatus(ctx, migration.ID, MigrationStatusComplete, &state, 0, 0)

		respondJSON(w, http.StatusAccepted, MigrateResponse{JobID: migration.ID})
		return
	}

	// Start background migration job
	RunMigrationJob(ctx, migration.ID)

	respondJSON(w, http.StatusAccepted, MigrateResponse{JobID: migration.ID})
}

func handleRollbackTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "template name is required", "")
		return
	}

	var req RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "Check JSON syntax.")
		return
	}

	if req.Version < 1 {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "version must be at least 1", "")
		return
	}

	ctx := r.Context()

	// Get current template
	template, err := GetTemplate(ctx, name)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			respondError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", err.Error(),
				"Use GET /platform/templates to list available templates.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	if req.Version >= template.CurrentVersion {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST",
			"rollback version must be less than current version",
			fmt.Sprintf("Current version is %d.", template.CurrentVersion))
		return
	}

	// Check for concurrent migration
	jm := GetJobManager()
	if jm.IsRunning(template.ID) {
		respondError(w, http.StatusConflict, "ATOMICBASE_BUSY",
			"another migration is already in progress",
			"Wait for the current migration to complete or check job status.")
		return
	}

	// Get target version schema
	targetVersion, err := GetTemplateVersion(ctx, template.ID, req.Version)
	if err != nil {
		respondError(w, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error(),
			"Use GET /platform/templates/{name}/history to see available versions.")
		return
	}

	// Diff current schema to target schema
	changes := diffSchemas(template.Schema, targetVersion.Schema)

	// Generate migration plan (current -> target)
	plan, err := GenerateMigrationPlan(template.Schema, targetVersion.Schema, changes, nil)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_MIGRATION", err.Error(), "")
		return
	}

	// Create new version in history (rollback creates a NEW version with the old schema)
	newVersion := template.CurrentVersion + 1
	schemaJSON, _ := json.Marshal(targetVersion.Schema)
	hash := sha256.Sum256(schemaJSON)
	checksum := hex.EncodeToString(hash[:])

	conn, err := getDB()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (template_id, version, schema, checksum, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, TableTemplatesHistory), template.ID, newVersion, schemaJSON, checksum, now)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	// Create migration record
	migration, err := CreateMigration(ctx, template.ID, template.CurrentVersion, newVersion, plan.SQL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	// Start background migration job
	RunMigrationJob(ctx, migration.ID)

	respondJSON(w, http.StatusAccepted, RollbackResponse{JobID: migration.ID})
}

func handleGetTemplateHistory(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "template name is required", "")
		return
	}

	history, err := GetTemplateHistory(r.Context(), name)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			respondError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", err.Error(),
				"Use GET /platform/templates to list available templates.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
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
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	respondJSON(w, http.StatusOK, tenants)
}

func handleGetTenant(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "tenant name is required", "")
		return
	}

	tenant, err := GetTenant(r.Context(), name)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			respondError(w, http.StatusNotFound, "TENANT_NOT_FOUND", err.Error(),
				"Use GET /platform/tenants to list available tenants.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	respondJSON(w, http.StatusOK, tenant)
}

func handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "Check JSON syntax.")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "name is required", "")
		return
	}

	if req.Template == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "template is required", "")
		return
	}

	tenant, err := CreateTenant(r.Context(), req.Name, req.Template)
	if err != nil {
		if errors.Is(err, ErrTenantExists) {
			respondError(w, http.StatusConflict, "TENANT_EXISTS", err.Error(),
				"Choose a different name or delete the existing tenant first.")
			return
		}
		if errors.Is(err, ErrTemplateNotFound) {
			respondError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "template not found",
				"Use GET /platform/templates to list available templates.")
			return
		}
		// Check for Turso errors
		if strings.Contains(err.Error(), "turso") || strings.Contains(err.Error(), "TURSO") {
			tools.RespErr(w, err)
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	respondJSON(w, http.StatusCreated, tenant)
}

func handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "tenant name is required", "")
		return
	}

	err := DeleteTenant(r.Context(), name)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			respondError(w, http.StatusNotFound, "TENANT_NOT_FOUND", err.Error(),
				"Use GET /platform/tenants to list available tenants.")
			return
		}
		// Check for Turso errors
		if strings.Contains(err.Error(), "turso") || strings.Contains(err.Error(), "TURSO") {
			tools.RespErr(w, err)
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleSyncTenant(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "tenant name is required", "")
		return
	}

	result, err := SyncTenant(r.Context(), name)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			respondError(w, http.StatusNotFound, "TENANT_NOT_FOUND", err.Error(),
				"Use GET /platform/tenants to list available tenants.")
			return
		}
		if errors.Is(err, ErrTenantInSync) {
			respondError(w, http.StatusBadRequest, "TENANT_IN_SYNC", err.Error(),
				"The tenant is already at the current template version.")
			return
		}
		// Check for Turso errors
		if strings.Contains(err.Error(), "turso") || strings.Contains(err.Error(), "TURSO") {
			tools.RespErr(w, err)
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// =============================================================================
// Job Handlers
// =============================================================================

func handleGetJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "job id is required", "")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid job id", "Job ID must be a number.")
		return
	}

	job, err := GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			respondError(w, http.StatusNotFound, "JOB_NOT_FOUND", err.Error(),
				"The job may have been deleted or never existed.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

func handleRetryJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "job id is required", "")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid job id", "Job ID must be a number.")
		return
	}

	result, err := RetryFailedTenants(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			respondError(w, http.StatusNotFound, "JOB_NOT_FOUND", err.Error(),
				"The job may have been deleted or never existed.")
			return
		}
		if errors.Is(err, ErrMigrationLocked) {
			respondError(w, http.StatusConflict, "ATOMICBASE_BUSY", err.Error(),
				"Wait for the current migration to complete.")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "")
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

func respondError(w http.ResponseWriter, status int, code, message, hint string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]string{
		"code":    code,
		"message": message,
	}
	if hint != "" {
		resp["hint"] = hint
	}
	json.NewEncoder(w).Encode(resp)
}
