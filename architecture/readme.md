# Atomicbase Architecture - Design Document

Single binary architecture. Everything compiles into one executable - API, auth, storage, and dashboard.

## Directory Structure

```
api/
  main.go              # Entry point only - wires everything together

  config/
    config.go          # Configuration loading

  database/            # REST API for database operations
    handlers.go        # /api/* route handlers
    daos.go            # Data access layer
    queries.go         # Query building
    schema.go          # Schema operations
    parse.go           # Request parsing

  auth/                # Auth API
    handlers.go        # /auth/* route handlers
    sessions.go        # Session management
    passwords.go       # Password hashing (Argon2id)

  storage/             # Storage API
    handlers.go        # /storage/* route handlers
    local.go           # Local disk backend
    s3.go              # S3 backend (later)

  admin/               # Dashboard
    embed.go           # //go:embed directive
    handlers.go        # /admin/* route handlers

dashboard/             # Frontend source
  src/
  dist/                # Built assets (embedded by api/admin/embed.go)
```

Each API area is a peer folder. `main.go` imports and wires them together.

## Embedded Assets

Dashboard frontend is embedded into the binary:

```go
// api/admin/embed.go
package admin

import "embed"

//go:embed ../../dashboard/dist/*
var Assets embed.FS
```

## Entry Point

```go
// api/main.go
package main

import (
    "net/http"

    "github.com/joe-ervin05/atomicbase/config"
    "github.com/joe-ervin05/atomicbase/database"
    "github.com/joe-ervin05/atomicbase/auth"
    "github.com/joe-ervin05/atomicbase/storage"
    "github.com/joe-ervin05/atomicbase/admin"
)

func main() {
    cfg := config.Load()

    mux := http.NewServeMux()

    database.RegisterRoutes(mux, cfg)  // /api/*
    auth.RegisterRoutes(mux, cfg)      // /auth/*
    storage.RegisterRoutes(mux, cfg)   // /storage/*
    admin.RegisterRoutes(mux)          // /admin/*

    // Apply middleware
    handler := applyMiddleware(mux, cfg)

    // Start server
    server := &http.Server{
        Addr:    cfg.Port,
        Handler: handler,
    }

    server.ListenAndServe()
}
```

## Route Registration

Each package registers its own routes:

```go
// api/database/handlers.go
package database

func RegisterRoutes(mux *http.ServeMux, cfg *config.Config) {
    mux.HandleFunc("GET /api/{db}/{table}", handleQuery)
    mux.HandleFunc("POST /api/{db}/{table}", handleInsert)
    mux.HandleFunc("PATCH /api/{db}/{table}", handleUpdate)
    mux.HandleFunc("DELETE /api/{db}/{table}", handleDelete)
    // ...
}
```

```go
// api/auth/handlers.go
package auth

func RegisterRoutes(mux *http.ServeMux, cfg *config.Config) {
    mux.HandleFunc("POST /auth/register", handleRegister)
    mux.HandleFunc("POST /auth/login", handleLogin)
    mux.HandleFunc("POST /auth/logout", handleLogout)
    mux.HandleFunc("GET /auth/me", handleMe)
    // ...
}
```

```go
// api/storage/handlers.go
package storage

func RegisterRoutes(mux *http.ServeMux, cfg *config.Config) {
    mux.HandleFunc("PUT /storage/{db}/{path...}", handleUpload)
    mux.HandleFunc("GET /storage/{db}/{path...}", handleDownload)
    mux.HandleFunc("DELETE /storage/{db}/{path...}", handleDelete)
    // ...
}
```

```go
// api/admin/handlers.go
package admin

func RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /admin/", serveDashboard)
    mux.HandleFunc("GET /admin/{path...}", serveDashboard)
}

func serveDashboard(w http.ResponseWriter, r *http.Request) {
    // Serve from embedded Assets, fallback to index.html for SPA
}
```

## Routes

| Path | Package | Purpose |
|------|---------|---------|
| `/api/*` | `database/` | REST API for database operations |
| `/auth/*` | `auth/` | Authentication (register, login, logout, etc.) |
| `/storage/*` | `storage/` | File storage (upload, download, delete) |
| `/admin/*` | `admin/` | Dashboard SPA |

## Build Process

```bash
# 1. Build dashboard frontend
cd dashboard && pnpm build

# 2. Build single binary
cd api && go build -o atomicbase .

# Result: single 'atomicbase' binary with everything embedded
```

## Running

```bash
# Default (port 8080)
./atomicbase

# Custom port
./atomicbase --port 3000

# With environment variables
ATOMICBASE_PORT=3000 ATOMICBASE_DATA_DIR=./data ./atomicbase
```

## Configuration

```go
// api/config/config.go
type Config struct {
    Port    string `env:"PORT" default:":8080"`
    DataDir string `env:"DATA_DIR" default:"./data"`

    // Database
    TursoURL   string `env:"TURSO_URL"`
    TursoToken string `env:"TURSO_TOKEN"`

    // Auth
    SessionTTL     int `env:"SESSION_TTL" default:"2592000"`      // 30 days
    MaxSessions    int `env:"MAX_SESSIONS_PER_USER" default:"10"`

    // Storage
    StorageDriver string `env:"STORAGE_DRIVER" default:"local"`
    StoragePath   string `env:"STORAGE_PATH" default:"./data/storage"`

    // Existing config...
    APIKey           string
    RateLimitEnabled bool
    RateLimit        int
    CORSOrigins      []string
    // ...
}
```

## Data Directory

```
data/
  storage/           # File storage
    {db_name}/
      ...
  atomicbase.db      # Internal metadata (if needed)
```

## Deployment

Single binary means simple deployment:

```bash
# Copy binary to server
scp atomicbase user@server:/usr/local/bin/

# Run with systemd, Docker, or directly
./atomicbase
```

Docker:

```dockerfile
FROM alpine:latest
COPY atomicbase /usr/local/bin/atomicbase
EXPOSE 8080
CMD ["atomicbase"]
```
