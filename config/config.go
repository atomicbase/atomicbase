// Package config provides centralized configuration for the Atomicbase API.
package config

import "os"

// Config holds all application configuration values.
type Config struct {
	Port           string // HTTP server port (e.g., ":8080")
	PrimaryDBPath  string // Path to primary SQLite database file
	DataDir        string // Directory for storing database files
	MaxRequestBody int64  // Maximum request body size in bytes
}

// Cfg is the global configuration instance, loaded at startup.
var Cfg Config

func init() {
	Cfg = Load()
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	return Config{
		Port:           getEnv("PORT", ":8080"),
		PrimaryDBPath:  getEnv("DB_PATH", "atomicdata/primary.db"),
		DataDir:        getEnv("DATA_DIR", "atomicdata"),
		MaxRequestBody: 1 << 20, // 1MB
	}
}

// getEnv returns the environment variable value or a default if not set.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
