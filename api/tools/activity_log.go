package tools

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/joe-ervin05/atomicbase/config"
	_ "github.com/mattn/go-sqlite3"
)

// ActivityLog represents a single activity log record.
type ActivityLog struct {
	Time       time.Time
	Level      slog.Level
	Message    string
	API        string
	Method     string
	Path       string
	Status     int
	DurationMs int64
	ClientIP   string
	Tenant     string
	RequestID  string
	Error      string
}

// ActivityHandler implements slog.Handler for activity logging.
// It batches log entries and writes them to a separate SQLite database.
type ActivityHandler struct {
	db         *sql.DB
	insertStmt *sql.Stmt

	mu        sync.Mutex
	batch     []*ActivityLog
	batchSize int
	debounce  time.Duration
	timer     *time.Timer

	done   chan struct{}
	wg     sync.WaitGroup
	closed bool
}

// ActivityHandlerOptions configures the ActivityHandler.
type ActivityHandlerOptions struct {
	BatchSize int           // Number of logs before auto-flush (default 200)
	Debounce  time.Duration // Time after last log before flush (default 3s)
}

var (
	activityHandler *ActivityHandler
	activityOnce    sync.Once
)

// InitActivityLogger initializes the activity logger if enabled.
func InitActivityLogger() error {
	if !config.Cfg.ActivityLogEnabled {
		return nil
	}

	var initErr error
	activityOnce.Do(func() {
		initErr = initActivityLoggerInternal()
	})
	return initErr
}

func initActivityLoggerInternal() error {
	dir := filepath.Dir(config.Cfg.ActivityLogPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", "file:"+config.Cfg.ActivityLogPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA cache_size = -8000;
		PRAGMA temp_store = MEMORY;
		PRAGMA busy_timeout = 5000;
	`)
	if err != nil {
		db.Close()
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS activity_log (
			id INTEGER PRIMARY KEY,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			level INTEGER NOT NULL,
			message TEXT NOT NULL,
			api TEXT NOT NULL,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			status INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			client_ip TEXT,
			tenant TEXT,
			request_id TEXT,
			error TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_activity_created_at ON activity_log(created_at);
		CREATE INDEX IF NOT EXISTS idx_activity_level ON activity_log(level);
		CREATE INDEX IF NOT EXISTS idx_activity_status ON activity_log(status);
	`)
	if err != nil {
		db.Close()
		return err
	}

	insertStmt, err := db.Prepare(`
		INSERT INTO activity_log (created_at, level, message, api, method, path, status, duration_ms, client_ip, tenant, request_id, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		db.Close()
		return err
	}

	activityHandler = &ActivityHandler{
		db:         db,
		insertStmt: insertStmt,
		batch:      make([]*ActivityLog, 0, 200),
		batchSize:  200,
		debounce:   3 * time.Second,
		done:       make(chan struct{}),
	}

	// Start retention cleanup if configured
	if config.Cfg.ActivityLogRetention > 0 {
		activityHandler.wg.Add(1)
		go activityHandler.retentionCleanup()
	}

	return nil
}

// Enabled reports whether the handler handles records at the given level.
func (h *ActivityHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo
}

// Handle processes a log record.
func (h *ActivityHandler) Handle(_ context.Context, r slog.Record) error {
	if h.closed {
		return nil
	}

	log := &ActivityLog{
		Time:    r.Time,
		Level:   r.Level,
		Message: r.Message,
	}

	// Extract our custom attributes
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "api":
			log.API = a.Value.String()
		case "method":
			log.Method = a.Value.String()
		case "path":
			log.Path = a.Value.String()
		case "status":
			log.Status = int(a.Value.Int64())
		case "duration_ms":
			log.DurationMs = a.Value.Int64()
		case "client_ip":
			log.ClientIP = a.Value.String()
		case "tenant":
			log.Tenant = a.Value.String()
		case "request_id":
			log.RequestID = a.Value.String()
		case "error":
			log.Error = a.Value.String()
		}
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()

	h.batch = append(h.batch, log)

	// Reset debounce timer
	if h.timer != nil {
		h.timer.Stop()
	}
	h.timer = time.AfterFunc(h.debounce, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.flushLocked()
	})

	// Flush if batch threshold reached
	if len(h.batch) >= h.batchSize {
		h.flushLocked()
	}

	return nil
}

// WithAttrs returns a new handler with the given attributes.
func (h *ActivityHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // Activity logs don't use persistent attributes
}

// WithGroup returns a new handler with the given group name.
func (h *ActivityHandler) WithGroup(name string) slog.Handler {
	return h // Activity logs don't use groups
}

// flushLocked writes all batched logs to the database. Must hold h.mu.
func (h *ActivityHandler) flushLocked() {
	if len(h.batch) == 0 {
		return
	}

	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}

	tx, err := h.db.Begin()
	if err != nil {
		Logger.Error("failed to begin activity log transaction", "error", err)
		return
	}

	stmt := tx.Stmt(h.insertStmt)
	for _, log := range h.batch {
		_, err := stmt.Exec(
			log.Time.Format(time.RFC3339),
			log.Level,
			log.Message,
			log.API,
			log.Method,
			log.Path,
			log.Status,
			log.DurationMs,
			log.ClientIP,
			log.Tenant,
			log.RequestID,
			log.Error,
		)
		if err != nil {
			Logger.Error("failed to insert activity log", "error", err)
		}
	}

	if err := tx.Commit(); err != nil {
		Logger.Error("failed to commit activity log transaction", "error", err)
	}

	h.batch = h.batch[:0]
}

// Flush writes all pending logs to the database.
func (h *ActivityHandler) Flush() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.flushLocked()
}

// retentionCleanup periodically removes old log entries.
func (h *ActivityHandler) retentionCleanup() {
	defer h.wg.Done()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	h.runCleanup()

	for {
		select {
		case <-ticker.C:
			h.runCleanup()
		case <-h.done:
			return
		}
	}
}

func (h *ActivityHandler) runCleanup() {
	retention := config.Cfg.ActivityLogRetention
	if retention <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retention).Format(time.RFC3339)
	result, err := h.db.Exec("DELETE FROM activity_log WHERE created_at < ?", cutoff)
	if err != nil {
		Logger.Error("failed to cleanup activity logs", "error", err)
		return
	}

	if rows, _ := result.RowsAffected(); rows > 0 {
		Logger.Info("cleaned up old activity logs", "rows_deleted", rows)
	}
}

// LogActivity logs a request activity entry.
func LogActivity(api, method, path string, status int, durationMs int64, clientIP, tenant, requestID, errMsg string) {
	if activityHandler == nil {
		return
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "request", 0)
	record.AddAttrs(
		slog.String("api", api),
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", status),
		slog.Int64("duration_ms", durationMs),
		slog.String("client_ip", clientIP),
		slog.String("tenant", tenant),
		slog.String("request_id", requestID),
		slog.String("error", errMsg),
	)

	activityHandler.Handle(context.Background(), record)
}

// CloseActivityLogger shuts down the activity logger gracefully.
func CloseActivityLogger() {
	if activityHandler == nil {
		return
	}

	activityHandler.mu.Lock()
	activityHandler.closed = true
	activityHandler.mu.Unlock()

	// Flush remaining logs
	activityHandler.Flush()

	close(activityHandler.done)
	activityHandler.wg.Wait()

	if activityHandler.insertStmt != nil {
		activityHandler.insertStmt.Close()
	}
	if activityHandler.db != nil {
		activityHandler.db.Close()
	}
}
