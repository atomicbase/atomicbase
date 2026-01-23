# Atomicbase Auth - Design Document

Session-based authentication system inspired by [Lucia Auth](https://lucia-auth.com/) and [The Copenhagen Book](https://thecopenhagenbook.com/).

## Architecture Overview

Auth is **centralized** across two infrastructure databases (see [db_architecture.md](./db_architecture.md)):

- **sessions.db**: Ephemeral session state (validated on every request)
- **tenants.db**: User identity and metadata (permanent)

Atomicbase handles authentication but **not** user → database mapping. Developers define their own relationships.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Auth Architecture                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   sessions.db              tenants.db              tenant databases     │
│   ┌─────────────┐         ┌─────────────┐         ┌─────────────────┐   │
│   │ __sessions  │         │ __users     │         │ user-defined    │   │
│   └─────────────┘         │  - metadata │         │ tables only     │   │
│                           └─────────────┘         └─────────────────┘   │
│                                                                         │
│   Ephemeral               Identity +              No hidden tables      │
│   Redis-swappable         flexible JSON           Full user control     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Why Sessions Over JWTs

From Lucia's experience:

> "We used to support JWT and it was a broken mess. Token rotation requires additional complexity, you need to sync state in the client, and the added security risks require you to just do more."

Key advantages of sessions:

- **Immediate revocation** - Delete from DB, done
- **No sync issues** - Works naturally across multiple tabs
- **Simple implementation** - One DB query per request
- **Fewer security footguns** - No token expiry/refresh complexity

## Database Schema

### sessions.db

```sql
CREATE TABLE __sessions (
    id TEXT PRIMARY KEY,              -- SHA-256 hash of token (for lookup)
    public_id TEXT UNIQUE NOT NULL,   -- random opaque ID (for list/revoke API)
    user_id TEXT NOT NULL,            -- references __users in tenants.db
    expires_at INTEGER NOT NULL,      -- unix epoch seconds
    created_at INTEGER NOT NULL,      -- unix epoch seconds
    user_agent TEXT,
    ip_address TEXT
);

CREATE INDEX __sessions_expires_at ON __sessions(expires_at);
CREATE INDEX __sessions_user_id ON __sessions(user_id);
```

### tenants.db

```sql
CREATE TABLE __users (
    id TEXT PRIMARY KEY,                    -- nanoid or uuid
    email TEXT NOT NULL COLLATE NOCASE,     -- normalized: lowercase, trimmed
    email_verified INTEGER DEFAULT 0,
    password_hash TEXT NOT NULL,            -- argon2id hash
    metadata TEXT,                          -- JSON field for app-specific data
    created_at INTEGER NOT NULL,            -- unix epoch seconds

    UNIQUE(email)
);
```

### Schema Notes

- **expires_at as INTEGER**: Unix epoch seconds. Compare with `unixepoch()` in queries.
- **email COLLATE NOCASE**: Ensures uniqueness is case-insensitive.
- **public_id**: Separate from the hashed token ID. Used for session listing/revocation APIs.
- **metadata**: Flexible JSON field. Store anything: default database, roles, preferences, etc.
- **Cross-database reference**: `__sessions.user_id` references `__users.id` but can't use FK (different DBs).

## The `metadata` Field

The `metadata` column is a JSON field for app-specific user data. Atomicbase doesn't enforce structure - developers decide what to store.

**Common patterns:**

```json
// Database-per-user: store user's database name
{ "database": "user-abc-123" }

// Database-per-org with default
{ "default_org": "acme-corp", "role": "admin" }

// Feature flags
{ "database": "user-123", "beta_features": true }
```

**Why JSON instead of columns:**
- No schema migrations for app-specific fields
- Developers have full flexibility
- Atomicbase stays unopinionated about user → database mapping

## SDK Interface

### Client Creation

```typescript
import { createClient } from '@atomicbase/sdk'

const client = createClient({
  url: 'http://localhost:8080',
  // Optional: default headers, fetch implementation, etc.
})
```

### Tenant Selection (Immutable)

```typescript
// .tenant() returns a NEW client - no mutation
const appDb = client.tenant('my-app-db')
const directoryDb = client.tenant('app-directory')

// Chain directly
const { data } = await client.tenant('my-app-db').from('posts').select()
```

### Auth Methods

```typescript
interface User {
  id: string
  email: string
  email_verified: boolean
  metadata: Record<string, unknown>
  created_at: number
}

interface Session {
  id: string           // public_id
  current: boolean
  created_at: number
  expires_at: number
  user_agent?: string
  ip_address?: string
}

// client.auth methods
client.auth.register(email, password, metadata?)  // Create user + session
client.auth.login(email, password)                // Create session
client.auth.logout()                              // Delete current session
client.auth.logoutAll()                           // Delete all user's sessions
client.auth.getUser()                             // Get current user
client.auth.updateMetadata(metadata)              // Update user's metadata
client.auth.changePassword(current, new)          // Change password
client.auth.listSessions()                        // List user's sessions
client.auth.revokeSession(publicId)               // Delete specific session
```

### Usage Example

```typescript
import { createClient } from '@atomicbase/sdk'
import { eq } from '@atomicbase/sdk/filters'

const client = createClient({ url: 'http://localhost:8080' })

// Register with metadata
const { user, error } = await client.auth.register(
  'user@example.com',
  'password123',
  { database: 'user-abc-123' }  // App decides what to store
)

// Login
const { user, error } = await client.auth.login(
  'user@example.com',
  'password123'
)

// Get user and their database from metadata
const { user } = await client.auth.getUser()
const userDb = client.tenant(user.metadata.database)

// Query user's data
const { data } = await userDb
  .from('posts')
  .select()
  .where(eq('user_id', user.id))

// Logout
await client.auth.logout()
```

## Token Generation

```go
import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base32"
    "encoding/hex"
)

// Generate 15 random bytes = 120 bits of entropy
// Encode as base32 = 24 character token
func generateSessionToken() string {
    bytes := make([]byte, 15)
    rand.Read(bytes)
    return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
}

// Generate public ID for session management API
func generatePublicID() string {
    bytes := make([]byte, 16)
    rand.Read(bytes)
    return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
}

// Hash token before storage
func hashToken(token string) string {
    hash := sha256.Sum256([]byte(token))
    return hex.EncodeToString(hash[:])
}
```

## Email Normalization

```go
func normalizeEmail(email string) string {
    return strings.ToLower(strings.TrimSpace(email))
}
```

Always normalize before:
- Checking if email exists (registration)
- Looking up user (login)
- Storing in database

## API Endpoints

### Important: Cookie-Only Authentication

Session tokens are **never** returned in JSON response bodies. This preserves the security benefit of HttpOnly cookies - JavaScript never sees the credential.

For non-browser clients (mobile apps, CLI tools), a separate API token feature should be implemented (future work).

---

### Register

```
POST /auth/register
Content-Type: application/json

Request:
{
    "email": "user@example.com",
    "password": "securepassword123",
    "metadata": { "database": "user-abc" }   // optional
}

Response (201):
{
    "user": {
        "id": "abc123",
        "email": "user@example.com",
        "email_verified": false,
        "metadata": { "database": "user-abc" },
        "created_at": 1705312200
    }
}

Set-Cookie: __Host-session=JBSWY3DPEHPK3PXP...; Path=/; Secure; HttpOnly; SameSite=Lax; Max-Age=2592000

Errors:
- 400: Invalid email format
- 400: Password too weak (min 8 chars)
- 409: Email already registered
```

### Login

```
POST /auth/login
Content-Type: application/json

Request:
{
    "email": "user@example.com",
    "password": "securepassword123"
}

Response (200):
{
    "user": {
        "id": "abc123",
        "email": "user@example.com",
        "email_verified": false,
        "metadata": { "database": "user-abc" },
        "created_at": 1705312200
    }
}

Set-Cookie: __Host-session=...; Path=/; Secure; HttpOnly; SameSite=Lax; Max-Age=2592000

Errors:
- 401: Invalid email or password (don't reveal which)
```

### Logout

```
POST /auth/logout
Cookie: __Host-session=...

Response (200):
{}

Set-Cookie: __Host-session=; Path=/; Secure; HttpOnly; SameSite=Lax; Max-Age=0

# Always returns 200, even if session was invalid/missing
```

### Logout All Sessions

```
POST /auth/logout-all
Cookie: __Host-session=...

Response (200):
{
    "sessions_revoked": 5
}

Errors:
- 401: Not authenticated
```

### Get Current User

```
GET /auth/me
Cookie: __Host-session=...

Response (200):
{
    "user": {
        "id": "abc123",
        "email": "user@example.com",
        "email_verified": false,
        "metadata": { "database": "user-abc" },
        "created_at": 1705312200
    }
}

Errors:
- 401: Not authenticated
```

### Update Metadata

```
PATCH /auth/me/metadata
Cookie: __Host-session=...
Content-Type: application/json

Request:
{
    "metadata": { "database": "user-abc", "theme": "dark" }
}

Response (200):
{
    "user": {
        "id": "abc123",
        "email": "user@example.com",
        "metadata": { "database": "user-abc", "theme": "dark" },
        ...
    }
}

Errors:
- 401: Not authenticated
```

### Change Password

```
POST /auth/change-password
Cookie: __Host-session=...
Content-Type: application/json

Request:
{
    "current_password": "oldpassword123",
    "new_password": "newpassword456"
}

Response (200):
{}
# Invalidates ALL other sessions for this user

Errors:
- 400: New password too weak
- 401: Not authenticated
- 401: Current password incorrect
```

### List Active Sessions

```
GET /auth/sessions
Cookie: __Host-session=...

Response (200):
{
    "sessions": [
        {
            "id": "ABCDEFGHIJK12345",    // public_id, NOT the hashed token
            "current": true,
            "created_at": 1705312200,
            "expires_at": 1707904200,
            "user_agent": "Mozilla/5.0...",
            "ip_address": "192.168.1.1"
        }
    ]
}
```

### Revoke Specific Session

```
DELETE /auth/sessions/:public_id
Cookie: __Host-session=...

Response (200):
{}

Errors:
- 401: Not authenticated
- 404: Session not found
```

## Cookie Configuration

```
Set-Cookie: __Host-session=<token>; Path=/; Secure; HttpOnly; SameSite=Lax; Max-Age=2592000
```

| Attribute        | Value     | Purpose                                                         |
| ---------------- | --------- | --------------------------------------------------------------- |
| `__Host-` prefix | -         | Prevents cookie injection, requires Secure + Path=/ + no Domain |
| `Path=/`         | `/`       | Available to all paths                                          |
| `Secure`         | -         | HTTPS only                                                      |
| `HttpOnly`       | -         | Not accessible to JavaScript                                    |
| `SameSite=Lax`   | `Lax`     | CSRF protection while allowing top-level navigations            |
| `Max-Age`        | `2592000` | 30 days, must match server expires_at logic                     |

## CSRF Protection

With `SameSite=Lax` cookies, we get baseline CSRF protection. For defense in depth, implement **Origin/Referer validation** on all state-changing endpoints:

```go
func csrfCheck(r *http.Request, allowedOrigins []string) bool {
    if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
        return true
    }

    origin := r.Header.Get("Origin")
    if origin != "" {
        return isAllowedOrigin(origin, allowedOrigins)
    }

    referer := r.Header.Get("Referer")
    if referer != "" {
        return isAllowedOrigin(referer, allowedOrigins)
    }

    return false
}
```

## Session Lifecycle

### Configuration

```
SESSION_LIFETIME       = 30 days
SESSION_REFRESH_WINDOW = 15 days   (refresh when ≤15 days remaining)
MAX_SESSIONS_PER_USER  = 10        (optional limit, 0 = unlimited)
```

### Refresh Logic

Refresh when the session is **close to expiring**, not based on creation time:

```go
func shouldRefreshSession(session *Session) bool {
    now := time.Now().Unix()
    timeRemaining := session.ExpiresAt - now
    return timeRemaining <= REFRESH_WINDOW_SECONDS  // 15 days
}
```

- Days 0-15: No refresh needed
- Days 15-30: Refresh on next request (extends to 30 days from now)

## Password Hashing

Use Argon2id with safe baseline parameters:

```go
var DefaultParams = Argon2Params{
    Memory:      64 * 1024,  // 64 MB
    Iterations:  3,
    Parallelism: 4,
    SaltLength:  16,
    KeyLength:   32,
}
```

## Rate Limiting

### Per-IP Rate Limiting

- **Login**: 10 attempts per 10 minutes per IP
- **Register**: 10 attempts per hour per IP

### Per-Email Rate Limiting

- **Login**: 10 attempts per 10 minutes per email

Apply rate limit checks **before** expensive operations.

## Security Checklist

### Authentication

- [ ] Argon2id for password hashing
- [ ] Constant-time comparison for password verification
- [ ] Rate limiting on login (per-IP and per-email)
- [ ] Rate limiting on register (per-IP)
- [ ] No user enumeration (same error for wrong email vs wrong password)
- [ ] Email normalization before storage and lookup

### Sessions

- [ ] 120-bit random tokens (15 bytes, base32 encoded)
- [ ] SHA-256 hash before storage
- [ ] Separate public_id for session management APIs
- [ ] `__Host-session` cookie with HttpOnly, Secure, SameSite=Lax
- [ ] Refresh based on time-to-expiry
- [ ] Invalidate all sessions on password change

### CSRF

- [ ] Origin/Referer validation on state-changing endpoints

### Input Validation

- [ ] Email format validation
- [ ] Password minimum 8 characters
- [ ] Password maximum 128 characters (DoS prevention)

## Future Work

- [ ] Email verification flow
- [ ] Password reset flow
- [ ] OAuth providers
- [ ] Two-factor authentication
- [ ] API tokens for non-browser clients

## References

- [Lucia Auth](https://lucia-auth.com/) - Session-based auth library
- [The Copenhagen Book](https://thecopenhagenbook.com/) - Auth implementation guide
- [db_architecture.md](./db_architecture.md) - Database separation design
