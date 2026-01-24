package data

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Lock retry configuration (matches PocketBase)
var (
	lockRetryIntervals = []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		150 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
		700 * time.Millisecond,
		1000 * time.Millisecond,
	}
	maxLockRetries = 12
)

// isLockError checks if the error is a SQLite lock error.
func isLockError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "table is locked")
}

// execWithRetry executes a function with retry logic for SQLite lock errors.
func execWithRetry(ctx context.Context, fn func() error) error {
	var err error

	for attempt := 0; attempt <= maxLockRetries; attempt++ {
		err = fn()

		if err == nil || !isLockError(err) {
			return err
		}

		// Check context before sleeping
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Get sleep duration (cycle through intervals if we exceed the array)
		sleepIdx := attempt
		if sleepIdx >= len(lockRetryIntervals) {
			sleepIdx = len(lockRetryIntervals) - 1
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lockRetryIntervals[sleepIdx]):
			// Continue to next retry
		}
	}

	return err
}

// ExecContextWithRetry executes a query with lock retry logic.
func ExecContextWithRetry(ctx context.Context, exec Executor, query string, args ...any) (sql.Result, error) {
	var result sql.Result
	var err error

	retryErr := execWithRetry(ctx, func() error {
		result, err = exec.ExecContext(ctx, query, args...)
		return err
	})

	if retryErr != nil && retryErr != err {
		return nil, retryErr // Context error
	}
	return result, err
}
