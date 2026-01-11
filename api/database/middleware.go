package database

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joe-ervin05/atomicbase/config"
)

// Logger is the global structured logger instance.
var Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

type DbHandler func(ctx context.Context, db *Database, req *http.Request) ([]byte, error)

type DbResponseHandler func(ctx context.Context, db *Database, req *http.Request, w http.ResponseWriter) ([]byte, error)

type PrimaryHandler func(ctx context.Context, db *PrimaryDao, req *http.Request) ([]byte, error)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// generateRequestID creates a random request ID for tracing.
func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// LoggingMiddleware logs all HTTP requests with structured JSON output.
// Logs: method, path, status, duration, client IP, and request ID.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)

		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Get client IP
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = strings.Split(forwarded, ",")[0]
		}

		// Log the request
		Logger.Info("request",
			slog.String("request_id", requestID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", wrapped.status),
			slog.Duration("duration", time.Since(start)),
			slog.String("client_ip", strings.TrimSpace(clientIP)),
			slog.String("user_agent", r.UserAgent()),
		)
	})
}

// rateLimiter tracks request counts per IP address.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string]*clientLimit
	rate     int           // requests per window
	window   time.Duration // time window
}

type clientLimit struct {
	count       int
	windowStart time.Time
}

var limiter = &rateLimiter{
	requests: make(map[string]*clientLimit),
	rate:     config.Cfg.RateLimit,
	window:   time.Minute,
}

// CORSMiddleware handles Cross-Origin Resource Sharing.
// If ATOMICBASE_CORS_ORIGINS is not set, CORS is disabled (no cross-origin access).
// Set to "*" to allow all origins, or comma-separated list of specific origins.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins := config.Cfg.CORSOrigins
		if len(origins) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		allowed := false

		for _, o := range origins {
			if o == "*" || o == origin {
				allowed = true
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

		if !allowed && origin != "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, DB-Name, DB-Token, Prefer")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// TimeoutMiddleware adds a request timeout to prevent long-running requests.
// Default timeout is 30 seconds, configurable via ATOMICBASE_REQUEST_TIMEOUT.
func TimeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeout := time.Duration(config.Cfg.RequestTimeout) * time.Second
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// RateLimitMiddleware limits requests per IP address.
// Set ATOMICBASE_RATE_LIMIT_ENABLED=true to enable rate limiting.
// Use ATOMICBASE_RATE_LIMIT to set requests per minute (default 100).
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !config.Cfg.RateLimitEnabled {
			next.ServeHTTP(w, r)
			return
		}

		// Get client IP (handle X-Forwarded-For for proxies)
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.Split(forwarded, ",")[0]
		}
		ip = strings.TrimSpace(strings.Split(ip, ":")[0])

		limiter.mu.Lock()
		client, exists := limiter.requests[ip]
		now := time.Now()

		if !exists || now.Sub(client.windowStart) > limiter.window {
			limiter.requests[ip] = &clientLimit{count: 1, windowStart: now}
			limiter.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		if client.count >= limiter.rate {
			limiter.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}

		client.count++
		limiter.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware validates the API key from the Authorization header.
// If ATOMICBASE_API_KEY is not set, authentication is disabled.
// Expected header format: Authorization: Bearer <api-key>
// Public endpoints: /health, /openapi.yaml, /docs
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public endpoints that don't require authentication
		switch r.URL.Path {
		case "/health", "/openapi.yaml", "/docs":
			next.ServeHTTP(w, r)
			return
		}

		apiKey := config.Cfg.APIKey
		if apiKey == "" {
			// Authentication disabled
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing Authorization header"})
			return
		}

		// Expect "Bearer <token>" format
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid Authorization header format"})
			return
		}

		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(apiKey)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid API key"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withPrimary(handler PrimaryHandler) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		req.Body = http.MaxBytesReader(wr, req.Body, config.Cfg.MaxRequestBody)
		defer req.Body.Close()

		dao, err := ConnPrimary()
		if err != nil {
			respErr(wr, err)
			return
		}
		// Note: don't close dao.Client - it's managed by the connection pool

		data, err := handler(ctx, &dao, req)
		if err != nil {
			respErr(wr, err)
			return
		}

		if data != nil {
			wr.Header().Set("Content-Type", "application/json")
		}
		wr.Write(data)
	}
}

// withDB wraps handlers that can use either the primary or an external database.
func withDB(handler DbHandler) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		req.Body = http.MaxBytesReader(wr, req.Body, config.Cfg.MaxRequestBody)
		defer req.Body.Close()

		dao, isExternal, err := connDb(req)
		if err != nil {
			respErr(wr, err)
			return
		}
		// Only close external (non-pooled) connections
		if isExternal {
			defer dao.Client.Close()
		}

		data, err := handler(ctx, &dao, req)
		if err != nil {
			respErr(wr, err)
			return
		}

		if data != nil {
			wr.Header().Set("Content-Type", "application/json")
		}
		wr.Write(data)
	}
}

// withDBResponse wraps handlers that need access to the ResponseWriter for setting headers.
func withDBResponse(handler DbResponseHandler) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		req.Body = http.MaxBytesReader(wr, req.Body, config.Cfg.MaxRequestBody)
		defer req.Body.Close()

		dao, isExternal, err := connDb(req)
		if err != nil {
			respErr(wr, err)
			return
		}
		// Only close external (non-pooled) connections
		if isExternal {
			defer dao.Client.Close()
		}

		data, err := handler(ctx, &dao, req, wr)
		if err != nil {
			respErr(wr, err)
			return
		}

		if data != nil {
			wr.Header().Set("Content-Type", "application/json")
		}
		wr.Write(data)
	}
}

func respErr(wr http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"

	// Map known errors to appropriate status codes and safe messages
	switch {
	case errors.Is(err, ErrTableNotFound),
		errors.Is(err, ErrColumnNotFound),
		errors.Is(err, ErrDatabaseNotFound),
		errors.Is(err, ErrNoRelationship),
		errors.Is(err, ErrTemplateNotFound):
		status = http.StatusNotFound
		message = err.Error() // Safe to expose
	case errors.Is(err, ErrTemplateInUse):
		status = http.StatusConflict
		message = err.Error() // Safe to expose
	case errors.Is(err, ErrInvalidOperator),
		errors.Is(err, ErrInvalidColumnType),
		errors.Is(err, ErrMissingWhereClause),
		errors.Is(err, ErrInvalidIdentifier),
		errors.Is(err, ErrEmptyIdentifier),
		errors.Is(err, ErrIdentifierTooLong),
		errors.Is(err, ErrInvalidCharacter),
		errors.Is(err, ErrNotDDLQuery),
		errors.Is(err, ErrQueryTooDeep):
		status = http.StatusBadRequest
		message = err.Error() // Safe to expose
	case errors.Is(err, ErrReservedTable):
		status = http.StatusForbidden
		message = err.Error() // Safe to expose
	case strings.Contains(err.Error(), "UNIQUE constraint failed"):
		status = http.StatusConflict
		message = "record already exists"
	case strings.Contains(err.Error(), "FOREIGN KEY constraint failed"):
		status = http.StatusBadRequest
		message = "foreign key constraint violation"
	case strings.Contains(err.Error(), "NOT NULL constraint failed"):
		status = http.StatusBadRequest
		message = "required field is missing"
	case strings.Contains(err.Error(), "no such table"):
		status = http.StatusNotFound
		message = "table not found"
	case strings.Contains(err.Error(), "no such column"):
		status = http.StatusBadRequest
		message = "column not found"
	default:
		// For unknown errors, log internally but return generic message
		// Avoid exposing SQL syntax errors, connection details, etc.
		status = http.StatusInternalServerError
		message = "internal server error"
	}

	wr.Header().Set("Content-Type", "application/json")
	wr.WriteHeader(status)
	json.NewEncoder(wr).Encode(map[string]string{"error": message})
}

// connDb returns a database connection and a boolean indicating if it's an external (non-pooled) connection.
// External connections should be closed after use; pooled connections should not.
func connDb(req *http.Request) (Database, bool, error) {
	dbName := req.Header.Get("DB-Name")

	dao, err := ConnPrimary()
	if err != nil {
		return Database{}, false, err
	}

	if dbName != "" {
		db, err := dao.ConnTurso(dbName)
		if err != nil {
			return Database{}, false, err
		}
		// Return external connection - should be closed after use
		return db, true, nil
	}

	// Return pooled primary connection - should NOT be closed
	return dao.Database, false, nil
}

func Request(method, url string, headers map[string]string, body any) (*http.Response, error) {
	client := &http.Client{}
	var req *http.Request
	var err error

	if body != nil {
		var buf bytes.Buffer

		err = json.NewEncoder(&buf).Encode(body)
		if err != nil {
			return nil, err
		}

		req, err = http.NewRequest(method, url, &buf)
		if err != nil {
			return nil, err
		}
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, err
		}
	}

	for name, val := range headers {
		req.Header.Add(name, val)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		bod, err := io.ReadAll(res.Body)
		if err != nil {
			return res, err
		}

		if bod == nil {
			return res, errors.New(res.Status)
		}
		return res, errors.New(string(bod))
	}

	return res, nil
}
