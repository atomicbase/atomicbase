package platform

import (
	"context"
	"errors"
	"testing"
)

// =============================================================================
// JobManager Tests
// Criteria A: Core locking mechanism (stable, unlikely to change)
// =============================================================================

func TestJobManager_TryLock(t *testing.T) {
	jm := NewJobManager()

	// First lock should succeed
	if !jm.TryLock(1) {
		t.Error("first TryLock should succeed")
	}

	// Second lock on same template should fail
	if jm.TryLock(1) {
		t.Error("second TryLock on same template should fail")
	}

	// Lock on different template should succeed
	if !jm.TryLock(2) {
		t.Error("TryLock on different template should succeed")
	}
}

func TestJobManager_Unlock(t *testing.T) {
	jm := NewJobManager()

	jm.TryLock(1)
	jm.Unlock(1)

	// After unlock, should be able to lock again
	if !jm.TryLock(1) {
		t.Error("TryLock after Unlock should succeed")
	}
}

func TestJobManager_IsRunning(t *testing.T) {
	jm := NewJobManager()

	if jm.IsRunning(1) {
		t.Error("should not be running before lock")
	}

	jm.TryLock(1)
	if !jm.IsRunning(1) {
		t.Error("should be running after lock")
	}

	jm.Unlock(1)
	if jm.IsRunning(1) {
		t.Error("should not be running after unlock")
	}
}

// =============================================================================
// isRetryableError Tests
// Criteria B: Many edge cases for error classification
// =============================================================================

func TestIsRetryableError_Nil(t *testing.T) {
	if isRetryableError(nil) {
		t.Error("nil error should not be retryable")
	}
}

func TestIsRetryableError_Timeout(t *testing.T) {
	err := errors.New("connection timeout")
	if !isRetryableError(err) {
		t.Error("timeout error should be retryable")
	}
}

func TestIsRetryableError_ConnectionRefused(t *testing.T) {
	err := errors.New("dial tcp: connection refused")
	if !isRetryableError(err) {
		t.Error("connection refused should be retryable")
	}
}

func TestIsRetryableError_ConnectionReset(t *testing.T) {
	err := errors.New("connection reset by peer")
	if !isRetryableError(err) {
		t.Error("connection reset should be retryable")
	}
}

func TestIsRetryableError_IOTimeout(t *testing.T) {
	err := errors.New("i/o timeout")
	if !isRetryableError(err) {
		t.Error("i/o timeout should be retryable")
	}
}

func TestIsRetryableError_NoSuchHost(t *testing.T) {
	err := errors.New("dial tcp: lookup foo: no such host")
	if !isRetryableError(err) {
		t.Error("no such host should be retryable")
	}
}

func TestIsRetryableError_ConstraintViolation(t *testing.T) {
	err := errors.New("UNIQUE constraint failed")
	if isRetryableError(err) {
		t.Error("constraint violation should not be retryable")
	}
}

func TestIsRetryableError_SyntaxError(t *testing.T) {
	err := errors.New("near 'TABL': syntax error")
	if isRetryableError(err) {
		t.Error("syntax error should not be retryable")
	}
}

func TestIsRetryableError_ForeignKeyViolation(t *testing.T) {
	err := errors.New("FOREIGN KEY constraint failed")
	if isRetryableError(err) {
		t.Error("foreign key violation should not be retryable")
	}
}

// =============================================================================
// statusFromResult Tests
// Criteria A: Simple, stable mapping
// =============================================================================

func TestStatusFromResult_Success(t *testing.T) {
	result := MigrationResult{Success: true}
	if statusFromResult(result) != TenantMigrationStatusSuccess {
		t.Error("success result should map to success status")
	}
}

func TestStatusFromResult_Failed(t *testing.T) {
	result := MigrationResult{Success: false, Error: "some error"}
	if statusFromResult(result) != TenantMigrationStatusFailed {
		t.Error("failed result should map to failed status")
	}
}

// =============================================================================
// loadMigrationCache Tests
// Criteria C: Complex context - building migration chain
// =============================================================================

func TestLoadMigrationCache(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 3)

	// Insert migrations v1->v2 and v2->v3
	insertTestMigration(t, testDB, templateID, 1, 2)
	insertTestMigration(t, testDB, templateID, 2, 3)

	// Tenants at different versions
	tenants := []Tenant{
		{ID: 1, TemplateVersion: 1},
		{ID: 2, TemplateVersion: 2},
	}

	cache, err := loadMigrationCache(context.Background(), templateID, tenants, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have migrations for v1->v2 and v2->v3
	if _, ok := cache[1]; !ok {
		t.Error("cache should contain v1->v2 migration")
	}
	if _, ok := cache[2]; !ok {
		t.Error("cache should contain v2->v3 migration")
	}
}

func TestLoadMigrationCache_MissingMigration(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 3)

	// Only insert v1->v2, missing v2->v3
	insertTestMigration(t, testDB, templateID, 1, 2)

	tenants := []Tenant{
		{ID: 1, TemplateVersion: 1},
	}

	_, err := loadMigrationCache(context.Background(), templateID, tenants, 3)
	if err == nil {
		t.Error("expected error for missing migration")
	}
}

// =============================================================================
// GetJob Tests
// Criteria B: Error handling scenarios
// =============================================================================

func TestGetJob_Found(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)

	job, err := GetJob(context.Background(), migrationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.ID != migrationID {
		t.Errorf("job.ID = %d, want %d", job.ID, migrationID)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	_, err := GetJob(context.Background(), 99999)
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got: %v", err)
	}
}

// =============================================================================
// RetryFailedTenants Tests
// Criteria C: Complex state management
// =============================================================================

func TestRetryFailedTenants_NoFailed(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)

	resp, err := RetryFailedTenants(context.Background(), migrationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.RetriedCount != 0 {
		t.Errorf("retriedCount = %d, want 0", resp.RetriedCount)
	}
}

func TestRetryFailedTenants_MigrationLocked(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	templateID := insertTestTemplate(t, testDB, "myapp", 2)
	migrationID := insertTestMigration(t, testDB, templateID, 1, 2)

	// Lock the template
	jm := GetJobManager()
	jm.TryLock(templateID)
	defer jm.Unlock(templateID)

	_, err := RetryFailedTenants(context.Background(), migrationID)
	if err != ErrMigrationLocked {
		t.Errorf("expected ErrMigrationLocked, got: %v", err)
	}
}

// =============================================================================
// ResumeRunningJobs Tests
// Criteria A: Startup recovery (critical functionality)
// =============================================================================

func TestResumeRunningJobs_NoRunning(t *testing.T) {
	testDB := setupTenantTestDB(t)
	defer testDB.Close()
	cleanup := setTestDB(t, testDB)
	defer cleanup()

	// No running jobs should not error
	err := ResumeRunningJobs(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// MigrationResult Tests
// =============================================================================

func TestMigrationResult_Fields(t *testing.T) {
	result := MigrationResult{
		TenantID: 42,
		Success:  false,
		Error:    "connection timeout",
	}

	if result.TenantID != 42 {
		t.Errorf("TenantID = %d, want 42", result.TenantID)
	}
	if result.Success {
		t.Error("Success should be false")
	}
	if result.Error != "connection timeout" {
		t.Errorf("Error = %s, want 'connection timeout'", result.Error)
	}
}

// =============================================================================
// Constants Tests
// Criteria A: Verify constants are sensible
// =============================================================================

func TestConstants(t *testing.T) {
	if BatchSize < 1 || BatchSize > 100 {
		t.Errorf("BatchSize = %d, should be between 1 and 100", BatchSize)
	}

	if MaxRetries < 1 || MaxRetries > 10 {
		t.Errorf("MaxRetries = %d, should be between 1 and 10", MaxRetries)
	}

	if BaseBackoff <= 0 {
		t.Errorf("BaseBackoff = %v, should be positive", BaseBackoff)
	}
}
