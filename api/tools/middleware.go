package tools

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/atomicbase/atomicbase/config"
)

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
// Also logs to activity database if activity logging is enabled.
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

		duration := time.Since(start)

		// Get client IP
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = strings.Split(forwarded, ",")[0]
		}
		clientIP = strings.TrimSpace(clientIP)

		// Log the request to stdout
		Logger.Info("request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", duration,
			"client_ip", clientIP,
			"user_agent", r.UserAgent(),
		)

		// Log to activity database
		LogActivity(
			detectAPIType(r.URL.Path),
			r.Method,
			r.URL.Path,
			wrapped.status,
			duration.Milliseconds(),
			clientIP,
			r.Header.Get("Database"),
			requestID,
			"", // error field
		)
	})
}

// detectAPIType determines whether a request is for the data or platform API.
func detectAPIType(path string) string {
	if strings.HasPrefix(path, "/platform") {
		return "platform"
	}
	if strings.HasPrefix(path, "/data") {
		return "data"
	}
	return "other"
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Database, DB-Token, Prefer")
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
		case "/health", "/openapi.yaml", "/docs", "/platform/health":
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
			json.NewEncoder(w).Encode(map[string]string{"code": "UNAUTHORIZED", "message": "missing Authorization header"})
			return
		}

		// Expect "Bearer <token>" format
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"code": "UNAUTHORIZED", "message": "invalid Authorization header format"})
			return
		}

		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(apiKey)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"code": "UNAUTHORIZED", "message": "invalid API key"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// PanicRecoveryMiddleware recovers from panics and returns a 500 error.
// Logs the panic message and stack trace for debugging.
func PanicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				Logger.Error("panic recovered",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
					"stack", string(stack),
				)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "internal server error",
				})
			}
		}()

		next.ServeHTTP(w, r)
	})
}
