# Atomicbase

## **Atomicbase is in early stages of development.** APIs may change.

Atomicbase is the backend for effortless multi-tenant architecture. It provides a complete backend solution on top of SQLite and Turso with authentication, file storage, a dashboard, and client SDKs - all packaged as a single lightning-fast Go executable.

## Status

| Component        | Status   |
| ---------------- | -------- |
| Database API     | Beta     |
| Schema Templates | Beta     |
| Authentication   | Planning |
| File Storage     | Planning |
| Dashboard        | Planning |

## Quick Start

```bash
cd api
go build -o atomicbase .
./atomicbase
```

The API runs on `http://localhost:8080` by default.

## REST API

All CRUD operations go through `/query/{table}` with `Prefer` header and `Method` to specify behavior.

**Select:**

```http
POST /query/users
Prefer: operation=select

{"select": ["id", "name"], "where": [{"status": {"eq": "active"}}], "limit": 20}
```

**Insert:**

```http
POST /query/users

{"data": {"name": "Alice", "email": "alice@example.com"}, "returning": ["id"]}
```

**Update:**

```http
PATCH /query/users

{"data": {"status": "inactive"}, "where": [{"id": {"eq": 5}}]}
```

**Delete:**

```http
DELETE /query/users

{"where": [{"id": {"eq": 5}}]}
```

### Filtering

`where` is always an array. Elements are ANDed together.

| Operator                              | Example                              |
| ------------------------------------- | ------------------------------------ |
| `eq`, `neq`, `gt`, `gte`, `lt`, `lte` | `{"age": {"gte": 21}}`               |
| `like`, `glob`                        | `{"name": {"like": "%smith%"}}`      |
| `in`, `between`                       | `{"status": {"in": ["a", "b"]}}`     |
| `is`, `fts`                           | `{"deleted_at": {"is": null}}`       |
| `not`                                 | `{"name": {"not": {"eq": "Admin"}}}` |

**OR conditions:**

```json
{
  "where": [
    { "org_id": { "eq": 5 } },
    { "or": [{ "status": { "eq": "active" } }, { "role": { "eq": "admin" } }] }
  ]
}
```

### Nested Relations

Auto-join via foreign keys:

```json
{ "select": ["id", "name", { "posts": ["title", { "comments": ["body"] }] }] }
```

Returns nested JSON:

```json
[{"id": 1, "name": "Alice", "posts": [{"title": "Hello", "comments": [...]}]}]
```

### Full-Text Search

Create an FTS index, then query with `fts` operator:

```http
POST /schema/fts/articles
{"columns": ["title", "content"]}
```

```http
POST /query/articles
Prefer: operation=select

{"select": ["*"], "where": [{"title": {"fts": "sqlite database"}}]}
```

For complete API documentation, see [docs/database_api_design.md](./docs/database_api_design.md).

## Features

### Database API

- JSON query syntax (filtering, ordering, pagination)
- Nested relation queries with automatic joins
- Full-text search (FTS5)
- Schema management (create, alter, drop tables)
- Multi-database support (local SQLite + Turso)

### Schema Templates

- Define reusable table schemas as templates
- Associate databases with templates
- Sync templates to databases (create missing tables, add missing columns)
- Bulk sync across all tenant databases

### Developer Experience

- API key authentication
- Rate limiting
- CORS configuration
- Request timeout control
- OpenAPI documentation (`GET /openapi.yaml`, `GET /docs`)
- Graceful shutdown

## Configuration

Atomicbase is configured via environment variables (or `.env` file):

| Variable                        | Default                 | Description                         |
| ------------------------------- | ----------------------- | ----------------------------------- |
| `PORT`                          | `:8080`                 | HTTP server port                    |
| `DB_PATH`                       | `atomicdata/primary.db` | Path to primary SQLite database     |
| `DATA_DIR`                      | `atomicdata`            | Directory for database files        |
| `ATOMICBASE_API_KEY`            | (empty)                 | API key for auth (empty = disabled) |
| `ATOMICBASE_RATE_LIMIT_ENABLED` | `false`                 | Enable rate limiting                |
| `ATOMICBASE_RATE_LIMIT`         | `100`                   | Requests per minute per IP          |
| `ATOMICBASE_CORS_ORIGINS`       | (empty)                 | Comma-separated allowed origins     |
| `ATOMICBASE_REQUEST_TIMEOUT`    | `30`                    | Request timeout in seconds          |
| `ATOMICBASE_MAX_QUERY_DEPTH`    | `5`                     | Maximum nested relation depth       |
| `ATOMICBASE_MAX_QUERY_LIMIT`    | `1000`                  | Maximum rows per query              |
| `ATOMICBASE_DEFAULT_LIMIT`      | `100`                   | Default limit when not specified    |

## API Endpoints

| Category  | Endpoint                                          | Methods                  |
| --------- | ------------------------------------------------- | ------------------------ |
| Query     | `/query/{table}`                                  | POST, PATCH, DELETE      |
| Schema    | `/schema`                                         | GET                      |
| Schema    | `/schema/table/{table}`                           | GET, POST, PATCH, DELETE |
| FTS       | `/schema/fts`, `/schema/fts/{table}`              | GET, POST, DELETE        |
| Tenants   | `/tenants`, `/tenants/{name}`                     | GET, POST, PATCH, DELETE |
| Templates | `/templates`, `/templates/{name}`                 | GET, POST, PUT, DELETE   |
| Sync      | `/templates/{name}/sync`, `/tenants/{name}/sync`  | POST                     |
| Health    | `/health`                                         | GET                      |
| Docs      | `/openapi.yaml`, `/docs`                          | GET                      |

**Headers:**

- `Authorization: Bearer <key>` - API authentication
- `Tenant: <name>` - Target tenant database (default: primary)
- `Prefer: operation=select` - SELECT query
- `Prefer: on-conflict=replace` - UPSERT behavior
- `Prefer: on-conflict=ignore` - INSERT IGNORE behavior
- `Prefer: count=exact` - Include total count in `X-Total-Count` header

## Architecture

```
api/
├── main.go          # Entry point, server setup, graceful shutdown
├── config/          # Environment-based configuration
├── database/        # Database API (queries, schema, templates, FTS)
├── auth/            # Authentication (planned)
├── storage/         # File storage (planned)
└── admin/           # Dashboard backend (planned)
```

## Development

```bash
cd api
go test ./...
go build .
```

## Coming Soon

- **Authentication** - User management, sessions, OAuth providers
- **File Storage** - S3-compatible object storage integration
- **Dashboard** - Web UI for database management and monitoring
- **TypeScript SDK** - Type-safe client library

## Contributing

Atomicbase is open source under the Apache-2.0 license.

- [Report issues](https://github.com/joe-ervin05/atomicbase/issues)
- [Contribute code](https://github.com/joe-ervin05/atomicbase)
