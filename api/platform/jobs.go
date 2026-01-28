package platform

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/joe-ervin05/atomicbase/tools"
)

// Job processing constants.
const (
	BatchSize   = 25 // Number of tenants to process concurrently
	MaxRetries  = 5  // Maximum retry attempts for network errors
	BaseBackoff = 100 * time.Millisecond
)

// Common errors for job operations.
var (
	ErrJobNotFound     = errors.New("job not found")
	ErrMigrationLocked = errors.New("migration locked: another migration is already in progress")
	ErrFirstDBFailed   = errors.New("first database migration failed")
)

// JobManager manages background migration jobs.
type JobManager struct {
	mu      sync.Mutex
	running map[int32]bool // templateID -> is running
	wg      sync.WaitGroup
}

// NewJobManager creates a new job manager.
func NewJobManager() *JobManager {
	return &JobManager{
		running: make(map[int32]bool),
	}
}

// Global job manager instance.
var jobManager = NewJobManager()

// GetJobManager returns the global job manager.
func GetJobManager() *JobManager {
	return jobManager
}

// IsRunning checks if a migration is running for a template.
func (jm *JobManager) IsRunning(templateID int32) bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	return jm.running[templateID]
}

// TryLock attempts to acquire the migration lock for a template.
// Returns true if lock acquired, false if already locked.
func (jm *JobManager) TryLock(templateID int32) bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if jm.running[templateID] {
		return false
	}
	jm.running[templateID] = true
	return true
}

// Unlock releases the migration lock for a template.
func (jm *JobManager) Unlock(templateID int32) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	delete(jm.running, templateID)
}

// Wait waits for all running jobs to complete.
func (jm *JobManager) Wait() {
	jm.wg.Wait()
}

// GetJob returns a migration job by ID.
// Job ID is the same as Migration ID.
func GetJob(ctx context.Context, jobID int64) (*Migration, error) {
	migration, err := GetMigration(ctx, jobID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrJobNotFound
		}
		return nil, err
	}
	return migration, nil
}

// MigrationResult tracks the outcome of a single tenant migration.
type MigrationResult struct {
	TenantID int32
	Success  bool
	Error    string
}

// RunMigrationJob executes a migration job in the background.
// This implements the migration flow from the design doc:
// 1. Migrate first DB synchronously (abort if fails)
// 2. Migrate remaining DBs in batches of 25 concurrently
// 3. Update job status and tenant versions
func RunMigrationJob(ctx context.Context, jobID int64) {
	jm := GetJobManager()

	// Get migration details (use request context for initial fetch)
	migration, err := GetMigration(ctx, jobID)
	if err != nil {
		log.Printf("[job %d] failed to get migration: %v", jobID, err)
		return
	}

	// Try to acquire lock
	if !jm.TryLock(migration.TemplateID) {
		log.Printf("[job %d] migration already running for template %d", jobID, migration.TemplateID)
		return
	}

	jm.wg.Add(1)
	go func() {
		defer jm.wg.Done()
		defer jm.Unlock(migration.TemplateID)

		// Use background context for the job - it runs independently of HTTP request
		runMigrationJobInternal(context.Background(), migration)
	}()
}

// runMigrationJobInternal contains the actual job execution logic.
func runMigrationJobInternal(ctx context.Context, migration *Migration) {
	jobID := migration.ID

	// Get all pending tenants first to know total count
	tenants, err := GetPendingTenants(ctx, jobID, migration.TemplateID, migration.ToVersion)
	if err != nil {
		log.Printf("[job %d] failed to get pending tenants: %v", jobID, err)
		markJobFailed(ctx, jobID, 0, 0)
		return
	}

	// Mark job as running with total count
	if err := StartMigration(ctx, jobID, len(tenants)); err != nil {
		log.Printf("[job %d] failed to start migration: %v", jobID, err)
		return
	}

	// No tenants to migrate
	if len(tenants) == 0 {
		markJobSuccess(ctx, jobID, 0, 0)
		updateTemplateVersion(ctx, migration.TemplateID, migration.ToVersion)
		return
	}

	// Pre-load all needed migrations into cache
	migrations, err := loadMigrationCache(ctx, migration.TemplateID, tenants, migration.ToVersion)
	if err != nil {
		log.Printf("[job %d] failed to load migration cache: %v", jobID, err)
		markJobFailed(ctx, jobID, 0, 0)
		return
	}

	// First Database Strategy: migrate first tenant synchronously
	firstTenant := tenants[0]
	tenants = tenants[1:]

	result := migrateTenant(ctx, firstTenant, migrations, migration.ToVersion)
	if err := RecordTenantMigration(ctx, jobID, firstTenant.ID, statusFromResult(result), result.Error); err != nil {
		log.Printf("[job %d] failed to record first tenant migration: %v", jobID, err)
	}

	if !result.Success {
		// First DB failed - abort entire migration
		log.Printf("[job %d] first database migration failed: %s", jobID, result.Error)
		markJobFailed(ctx, jobID, 0, 1)
		return
	}

	// Update first tenant version
	if err := BatchUpdateTenantVersions(ctx, []int32{firstTenant.ID}, migration.ToVersion); err != nil {
		log.Printf("[job %d] failed to update first tenant version: %v", jobID, err)
	}

	// Process remaining tenants in batches
	// Start with 1 completed (first tenant)
	completedCount := 1
	failedCount := 0

	for i := 0; i < len(tenants); i += BatchSize {
		end := i + BatchSize
		if end > len(tenants) {
			end = len(tenants)
		}
		batch := tenants[i:end]

		// Migrate batch concurrently
		results := migrateBatchConcurrent(ctx, batch, migrations, migration.ToVersion)

		// Collect successful tenant IDs
		var successIDs []int32
		for _, r := range results {
			if r.Success {
				successIDs = append(successIDs, r.TenantID)
				completedCount++
			} else {
				failedCount++
			}

			// Record each tenant's result
			if err := RecordTenantMigration(ctx, jobID, r.TenantID, statusFromResult(r), r.Error); err != nil {
				log.Printf("[job %d] failed to record tenant %d migration: %v", jobID, r.TenantID, err)
			}
		}

		// Batch update successful tenants
		if len(successIDs) > 0 {
			if err := BatchUpdateTenantVersions(ctx, successIDs, migration.ToVersion); err != nil {
				log.Printf("[job %d] failed to batch update versions: %v", jobID, err)
			}
		}
	}

	// Determine final state
	if failedCount == 0 {
		markJobSuccess(ctx, jobID, completedCount, failedCount)
	} else if completedCount > 0 {
		markJobPartial(ctx, jobID, completedCount, failedCount)
	} else {
		markJobFailed(ctx, jobID, completedCount, failedCount)
	}

	// Update template version if any succeeded
	if completedCount > 0 {
		updateTemplateVersion(ctx, migration.TemplateID, migration.ToVersion)
	}
}

// loadMigrationCache pre-loads all needed migrations for the job.
func loadMigrationCache(ctx context.Context, templateID int32, tenants []Tenant, targetVersion int) (map[int][]string, error) {
	// Find minimum tenant version
	minVersion := targetVersion
	for _, t := range tenants {
		if t.TemplateVersion < minVersion {
			minVersion = t.TemplateVersion
		}
	}

	// Load all needed migrations
	cache := make(map[int][]string)
	for v := minVersion; v < targetVersion; v++ {
		migration, err := GetMigrationByVersions(ctx, templateID, v, v+1)
		if err != nil {
			return nil, fmt.Errorf("missing migration v%d->v%d: %w", v, v+1, err)
		}
		cache[v] = migration.SQL
	}

	return cache, nil
}

// migrateTenant migrates a single tenant with retry logic.
func migrateTenant(ctx context.Context, tenant Tenant, migrations map[int][]string, targetVersion int) MigrationResult {
	result := MigrationResult{TenantID: tenant.ID}

	// Build chained SQL for this tenant
	var allSQL []string
	for v := tenant.TemplateVersion; v < targetVersion; v++ {
		sql, ok := migrations[v]
		if !ok {
			result.Error = fmt.Sprintf("missing migration v%d->v%d", v, v+1)
			return result
		}
		allSQL = append(allSQL, sql...)
	}

	// Nothing to do if already at target
	if len(allSQL) == 0 {
		result.Success = true
		return result
	}

	// Execute with retry
	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := BaseBackoff * time.Duration(1<<uint(attempt-1))
			time.Sleep(backoff)
		}

		err := BatchExecute(ctx, tenant.Name, tenant.Token, allSQL)
		if err == nil {
			result.Success = true
			return result
		}

		lastErr = err

		// Only retry network errors
		if !isRetryableError(err) {
			break
		}
	}

	result.Error = lastErr.Error()
	return result
}

// migrateBatchConcurrent migrates a batch of tenants concurrently.
func migrateBatchConcurrent(ctx context.Context, tenants []Tenant, migrations map[int][]string, targetVersion int) []MigrationResult {
	results := make([]MigrationResult, len(tenants))
	var wg sync.WaitGroup

	for i, tenant := range tenants {
		wg.Add(1)
		go func(idx int, t Tenant) {
			defer wg.Done()
			results[idx] = migrateTenant(ctx, t, migrations, targetVersion)
		}(i, tenant)
	}

	wg.Wait()
	return results
}

// isRetryableError checks if an error is a network error worth retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors (timeout only, Temporary() is deprecated)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for common retryable error messages
	errStr := strings.ToLower(err.Error())
	retryableMessages := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"no such host",
		"i/o timeout",
	}

	for _, msg := range retryableMessages {
		if strings.Contains(errStr, msg) {
			return true
		}
	}

	return false
}

// statusFromResult converts a MigrationResult to status string.
func statusFromResult(r MigrationResult) string {
	if r.Success {
		return TenantMigrationStatusSuccess
	}
	return TenantMigrationStatusFailed
}

// RetryFailedTenants retries all failed tenants from a migration job.
// Returns the count of retried tenants and optionally a new job ID.
func RetryFailedTenants(ctx context.Context, jobID int64) (*RetryJobResponse, error) {
	// Get the original migration
	migration, err := GetMigration(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Check if another migration is running
	jm := GetJobManager()
	if jm.IsRunning(migration.TemplateID) {
		return nil, ErrMigrationLocked
	}

	// Get failed tenants
	failedTenants, err := GetFailedTenants(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if len(failedTenants) == 0 {
		return &RetryJobResponse{
			RetriedCount: 0,
			JobID:        jobID,
		}, nil
	}

	// Clear failed status so they become pending again
	conn, err := getDB()
	if err != nil {
		return nil, err
	}

	for _, tenant := range failedTenants {
		_, err = conn.ExecContext(ctx, fmt.Sprintf(`
			DELETE FROM %s WHERE migration_id = ? AND tenant_id = ?
		`, TableTenantMigrations), jobID, tenant.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to clear failed status: %w", err)
		}
	}

	// Reset job status to running (keep current counts)
	if err := UpdateMigrationStatus(ctx, jobID, MigrationStatusRunning, nil, migration.CompletedDBs, migration.FailedDBs); err != nil {
		return nil, err
	}

	// Start the job again
	RunMigrationJob(ctx, jobID)

	return &RetryJobResponse{
		RetriedCount: len(failedTenants),
		JobID:        jobID,
	}, nil
}

// ResumeRunningJobs resumes any jobs that were interrupted (e.g., by server restart).
// Should be called during server startup.
func ResumeRunningJobs(ctx context.Context) error {
	conn, err := getDB()
	if err != nil {
		return err
	}

	// Find all running migrations
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE status = ?
	`, TableMigrations), MigrationStatusRunning)
	if err != nil {
		return err
	}
	defer rows.Close()

	var jobIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		jobIDs = append(jobIDs, id)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Resume each job
	for _, jobID := range jobIDs {
		log.Printf("[startup] resuming interrupted job %d", jobID)
		RunMigrationJob(ctx, jobID)
	}

	if len(jobIDs) > 0 {
		log.Printf("[startup] resumed %d interrupted jobs", len(jobIDs))
	}

	return nil
}

// =============================================================================
// Helper functions for job status updates
// =============================================================================

func markJobSuccess(ctx context.Context, jobID int64, completed, failed int) {
	state := MigrationStateSuccess
	if err := UpdateMigrationStatus(ctx, jobID, MigrationStatusComplete, &state, completed, failed); err != nil {
		log.Printf("[job %d] failed to mark success: %v", jobID, err)
	}
}

func markJobPartial(ctx context.Context, jobID int64, completed, failed int) {
	state := MigrationStatePartial
	if err := UpdateMigrationStatus(ctx, jobID, MigrationStatusComplete, &state, completed, failed); err != nil {
		log.Printf("[job %d] failed to mark partial: %v", jobID, err)
	}
}

func markJobFailed(ctx context.Context, jobID int64, completed, failed int) {
	state := MigrationStateFailed
	if err := UpdateMigrationStatus(ctx, jobID, MigrationStatusComplete, &state, completed, failed); err != nil {
		log.Printf("[job %d] failed to mark failed: %v", jobID, err)
	}
}

func updateTemplateVersion(ctx context.Context, templateID int32, version int) {
	conn, err := getDB()
	if err != nil {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET current_version = ?, updated_at = ? WHERE id = ?
	`, TableTemplates), version, now, templateID)
	if err != nil {
		log.Printf("[template %d] failed to update version to %d: %v", templateID, version, err)
		return
	}

	// Invalidate schema cache so next request loads the new version
	tools.InvalidateTemplate(templateID)
}
