// Package config provides centralized configuration for the Atomicbase API.
package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration values.
type Config struct {
	Port             string   // HTTP server port (e.g., ":8080")
	PrimaryDBPath    string   // Path to primary SQLite database file
	DataDir          string   // Directory for storing database files
	MaxRequestBody   int64    // Maximum request body size in bytes
	APIKey           string   // API key for authentication (empty disables auth)
	RateLimitEnabled bool     // Whether rate limiting is enabled
	RateLimit        int      // Requests per minute per IP (default 100)
	CORSOrigins      []string // Allowed CORS origins (empty allows none, "*" allows all)
	RequestTimeout   int      // Request timeout in seconds (0 uses default of 30s)
	MaxQueryDepth    int      // Maximum nesting depth for queries (default 5)
	MaxQueryLimit    int      // Maximum rows per query (default 1000, 0 = unlimited)
	DefaultLimit     int      // Default limit when not specified (default 100, 0 = unlimited)

	// Turso configuration (for multi-tenant external databases)
	TursoOrganization    string // Turso organization name
	TursoAPIKey          string // Turso API key for management operations
	TursoTokenExpiration string // Token expiration (e.g., "7d", "30d", "never")
}

// Cfg is the global configuration instance, loaded at startup.
var Cfg Config

func init() {
	// Load .env file before reading config (ignore error if file doesn't exist)
	godotenv.Load()
	Cfg = Load()
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	rateLimitEnabled := strings.ToLower(os.Getenv("ATOMICBASE_RATE_LIMIT_ENABLED")) == "true"

	rateLimit := 100 // default 100 requests per minute
	if val := os.Getenv("ATOMICBASE_RATE_LIMIT"); val != "" {
		if r, err := strconv.Atoi(val); err == nil && r > 0 {
			rateLimit = r
		}
	}

	requestTimeout := 30
	if val := os.Getenv("ATOMICBASE_REQUEST_TIMEOUT"); val != "" {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			requestTimeout = t
		}
	}

	var corsOrigins []string
	if val := os.Getenv("ATOMICBASE_CORS_ORIGINS"); val != "" {
		corsOrigins = strings.Split(val, ",")
		for i := range corsOrigins {
			corsOrigins[i] = strings.TrimSpace(corsOrigins[i])
		}
	}

	maxQueryDepth := 5
	if val := os.Getenv("ATOMICBASE_MAX_QUERY_DEPTH"); val != "" {
		if d, err := strconv.Atoi(val); err == nil && d > 0 {
			maxQueryDepth = d
		}
	}

	maxQueryLimit := 1000
	if val := os.Getenv("ATOMICBASE_MAX_QUERY_LIMIT"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l >= 0 {
			maxQueryLimit = l
		}
	}

	defaultLimit := 100
	if val := os.Getenv("ATOMICBASE_DEFAULT_LIMIT"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l >= 0 {
			defaultLimit = l
		}
	}

	return Config{
		Port:             getEnv("PORT", ":8080"),
		PrimaryDBPath:    getEnv("DB_PATH", "atomicdata/primary.db"),
		DataDir:          getEnv("DATA_DIR", "atomicdata"),
		MaxRequestBody:   1 << 20, // 1MB
		APIKey:           os.Getenv("ATOMICBASE_API_KEY"),
		RateLimitEnabled: rateLimitEnabled,
		RateLimit:        rateLimit,
		CORSOrigins:      corsOrigins,
		RequestTimeout:   requestTimeout,
		MaxQueryDepth:    maxQueryDepth,
		MaxQueryLimit:    maxQueryLimit,
		DefaultLimit:     defaultLimit,

		// Turso configuration
		TursoOrganization:    os.Getenv("TURSO_ORGANIZATION"),
		TursoAPIKey:          os.Getenv("TURSO_API_KEY"),
		TursoTokenExpiration: getEnv("TURSO_TOKEN_EXPIRATION", "7d"),
	}
}

// getEnv returns the environment variable value or a default if not set.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
