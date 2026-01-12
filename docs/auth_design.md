# Atomicbase Auth - Design Document

Session-based authentication system inspired by [Lucia Auth](https://lucia-auth.com/) and [The Copenhagen Book](https://thecopenhagenbook.com/).

## Why Sessions Over JWTs

From Lucia's experience:

> "We used to support JWT and it was a broken mess. Token rotation requires additional complexity, you need to sync state in the client, and the added security risks require you to just do more."

Key advantages of sessions:

- **Immediate revocation** - Delete from DB, done
- **No sync issues** - Works naturally across multiple tabs
- **Simple implementation** - One DB query per request
- **Fewer security footguns** - No token expiry/refresh complexity
- **Fast with Turso** - Edge replicas make DB lookups <10ms

## Database Schema

```sql
-- Users table (auto-created by Atomicbase when auth is enabled)
CREATE TABLE _users (
    id TEXT PRIMARY KEY,                    -- nanoid or uuid
    email TEXT NOT NULL COLLATE NOCASE,     -- normalized: lowercase, trimmed
    email_verified INTEGER DEFAULT 0,
    password_hash TEXT NOT NULL,            -- argon2id hash
    created_at INTEGER NOT NULL,            -- unix epoch seconds

    UNIQUE(email)
);

-- Sessions table
CREATE TABLE _sessions (
    id TEXT PRIMARY KEY,                    -- SHA-256 hash of token (for lookup)
    public_id TEXT UNIQUE NOT NULL,         -- random opaque ID (for list/revoke API)
    user_id TEXT NOT NULL REFERENCES _users(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,            -- unix epoch seconds
    created_at INTEGER NOT NULL,            -- unix epoch seconds

    -- Optional security fields (informational only, not for enforcement)
    user_agent TEXT,
    ip_address TEXT
);

-- Indexes
CREATE INDEX _sessions_expires_at ON _sessions(expires_at);
CREATE INDEX _sessions_user_id ON _sessions(user_id);
```

### Schema Notes

- **expires_at as INTEGER**: Unix epoch seconds. Compare with `unixepoch()` in queries. Simpler, faster, and avoids SQLite datetime quirks.
- **email COLLATE NOCASE**: Ensures uniqueness is case-insensitive.
- **public_id**: Separate from the hashed token ID. Used for session listing/revocation APIs. Never reveals the actual session token.
- **No updated_at**: SQLite doesn't auto-update without triggers. Omitted until needed.
- **user_agent/ip_address**: Informational for security UI only. Not used for enforcement (IPs change, mobile networks, etc.).

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
// 16 random bytes = 128 bits, base32 encoded
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
import "strings"

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
    "password": "securepassword123"
}

Response (201):
{
    "user": {
        "id": "abc123",
        "email": "user@example.com",
        "email_verified": false,
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
        "created_at": 1705312200
    }
}

Errors:
- 401: Not authenticated
```

Implementation note: Use a JOIN to maintain "one DB query per request":

```sql
SELECT u.id, u.email, u.email_verified, u.created_at
FROM _sessions s
JOIN _users u ON u.id = s.user_id
WHERE s.id = ? AND s.expires_at > unixepoch()
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

### When to use SameSite=Strict

Use `Strict` instead of `Lax` if:

- Your app doesn't need users to remain logged in when clicking links from external sites
- Higher security is preferred over convenience

Most apps work fine with `Lax`.

## CSRF Protection

With `SameSite=Lax` cookies, we get baseline CSRF protection. However, for defense in depth, implement **Origin/Referer validation** on all state-changing endpoints (POST, PUT, PATCH, DELETE):

```go
func csrfCheck(r *http.Request, allowedOrigins []string) bool {
    // Only check state-changing methods
    if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
        return true
    }

    // Try Origin header first
    origin := r.Header.Get("Origin")
    if origin != "" {
        return isAllowedOrigin(origin, allowedOrigins)
    }

    // Fall back to Referer header
    // (Some browsers/privacy extensions strip Origin but keep Referer)
    referer := r.Header.Get("Referer")
    if referer != "" {
        return isAllowedOrigin(referer, allowedOrigins)
    }

    // Neither header present
    // This can happen with:
    // - Direct browser navigation (but those are GET requests, already allowed above)
    // - Non-browser clients (curl, mobile apps, etc.)
    // - Privacy extensions that strip both headers
    //
    // For browser-only APIs: reject (return false)
    // For APIs that need non-browser clients: allow (return true)
    //
    // Since we're cookie-based and SameSite=Lax, non-browser clients
    // would need the cookie anyway, so rejecting is safer.
    return false
}

func isAllowedOrigin(value string, allowedOrigins []string) bool {
    // Parse the origin/referer to extract just the origin portion
    // e.g., "https://example.com/path" -> "https://example.com"
    parsed, err := url.Parse(value)
    if err != nil {
        return false
    }
    origin := parsed.Scheme + "://" + parsed.Host

    for _, allowed := range allowedOrigins {
        if origin == allowed {
            return true
        }
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

### Session Limit Enforcement (MAX_SESSIONS_PER_USER)

When a user logs in and would exceed the session limit:

**Policy: Revoke oldest non-current sessions first**

```go
func enforceSessionLimit(userID string, currentSessionID string, maxSessions int) error {
    if maxSessions <= 0 {
        return nil // No limit
    }

    // Count existing sessions
    count, err := db.Query("SELECT COUNT(*) FROM _sessions WHERE user_id = ?", userID)
    if err != nil {
        return err
    }

    if count < maxSessions {
        return nil // Under limit
    }

    // Delete oldest sessions (excluding current) until under limit
    sessionsToDelete := count - maxSessions + 1
    _, err = db.Exec(`
        DELETE FROM _sessions
        WHERE id IN (
            SELECT id FROM _sessions
            WHERE user_id = ? AND id != ?
            ORDER BY created_at ASC
            LIMIT ?
        )
    `, userID, currentSessionID, sessionsToDelete)

    return err
}
```

This approach:
- Never logs out the user who just logged in
- Removes the oldest sessions first (by `created_at`)
- Is predictable and easy to explain to users
- Avoids "login denied" UX which is confusing

### Refresh Logic

Refresh when the session is **close to expiring**, not based on creation time:

```go
func shouldRefreshSession(session *Session) bool {
    now := time.Now().Unix()
    timeRemaining := session.ExpiresAt - now
    return timeRemaining <= REFRESH_WINDOW_SECONDS  // 15 days in seconds
}
```

This means:

- Days 0-15: No refresh needed
- Days 15-30: Refresh on next request (extends to 30 days from now)
- One refresh per ~15 days per active session (not every request)

### Request Flow

```
┌──────────┐     ┌──────────────┐     ┌─────────────────────────┐
│  Client  │────▶│  Read Cookie │────▶│  Hash Token             │
└──────────┘     └──────────────┘     └───────────┬─────────────┘
                                                   │
                                                   ▼
┌──────────────────────────────────────────────────────────────────┐
│  SELECT s.*, u.* FROM _sessions s                                │
│  JOIN _users u ON u.id = s.user_id                               │
│  WHERE s.id = ? AND s.expires_at > unixepoch()                   │
└──────────────────────────────────────────────────────────────────┘
                                                   │
                        ┌──────────────────────────┴───────────────┐
                        │                                          │
                        ▼                                          ▼
               ┌────────────────┐                         ┌────────────────┐
               │  Not Found or  │                         │     Found      │
               │    Expired     │                         │                │
               └───────┬────────┘                         └───────┬────────┘
                       │                                          │
                       ▼                                          ▼
               ┌────────────────┐              ┌──────────────────────────────┐
               │  401 Response  │              │  time_remaining =            │
               └────────────────┘              │  expires_at - now            │
                                               └──────────────┬───────────────┘
                                                              │
                                       ┌──────────────────────┴──────────────┐
                                       │                                     │
                          time_remaining > 15 days              time_remaining ≤ 15 days
                                       │                                     │
                                       ▼                                     ▼
                              ┌────────────────┐                   ┌─────────────────┐
                              │  No refresh    │                   │  Refresh:       │
                              │  needed        │                   │  UPDATE         │
                              └───────┬────────┘                   │  expires_at     │
                                      │                            │  Set new cookie │
                                      │                            └────────┬────────┘
                                      │                                     │
                                      └──────────────┬──────────────────────┘
                                                     │
                                                     ▼
                                         ┌─────────────────────┐
                                         │  Attach user to ctx │
                                         │  Continue request   │
                                         └─────────────────────┘
```

## SDK Interface

```typescript
interface User {
  id: string;
  email: string;
  email_verified: boolean;
  created_at: number; // unix epoch seconds
}

interface Session {
  id: string; // public_id
  current: boolean;
  created_at: number;
  expires_at: number;
  user_agent?: string;
  ip_address?: string;
}

interface AuthClient {
  // Core methods
  register(
    email: string,
    password: string
  ): Promise<{ user: User; error?: Error }>;
  login(
    email: string,
    password: string
  ): Promise<{ user: User; error?: Error }>;
  logout(): Promise<{ error?: Error }>;
  logoutAll(): Promise<{ sessions_revoked: number; error?: Error }>;

  // User info
  getUser(): Promise<{ user: User | null; error?: Error }>;

  // Password management
  changePassword(
    currentPassword: string,
    newPassword: string
  ): Promise<{ error?: Error }>;

  // Session management
  listSessions(): Promise<{ sessions: Session[]; error?: Error }>;
  revokeSession(sessionId: string): Promise<{ error?: Error }>;

  // Reactive state (for frameworks)
  onAuthStateChange(callback: (user: User | null) => void): () => void;
}
```

### Usage Example

```typescript
const client = createClient("http://localhost:8080");

// Register
const { user, error } = await client.auth.register(
  "user@example.com",
  "password123"
);

// Login
const { user, error } = await client.auth.login(
  "user@example.com",
  "password123"
);

// Check auth state
const { user } = await client.auth.getUser();
if (user) {
  console.log("Logged in as", user.email);
}

// Logout
await client.auth.logout();
```

## Password Hashing

Use Argon2id with safe baseline parameters. These should be tuned based on:

- Server CPU/RAM limits (especially on small instances)
- Desired p95 latency for login/register
- Expected peak concurrency

```go
import "golang.org/x/crypto/argon2"

type Argon2Params struct {
    Memory      uint32
    Iterations  uint32
    Parallelism uint8
    SaltLength  uint32
    KeyLength   uint32
}

// Safe baseline - tune for your infrastructure
var DefaultParams = Argon2Params{
    Memory:      64 * 1024,  // 64 MB
    Iterations:  3,
    Parallelism: 4,
    SaltLength:  16,         // 16 bytes
    KeyLength:   32,         // 32 bytes
}

func HashPassword(password string) (string, error) {
    salt := make([]byte, DefaultParams.SaltLength)
    rand.Read(salt)

    hash := argon2.IDKey(
        []byte(password),
        salt,
        DefaultParams.Iterations,
        DefaultParams.Memory,
        DefaultParams.Parallelism,
        DefaultParams.KeyLength,
    )

    // Encode as: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
    return encodeHash(hash, salt, DefaultParams), nil
}

func VerifyPassword(password, encodedHash string) bool {
    // Decode and compare using constant-time comparison
}
```

## Rate Limiting

Rate limiting is applied on two dimensions for login attempts:

### Per-IP Rate Limiting

Prevents brute-force attacks from a single source.

- **Login**: 10 attempts per 10 minutes per IP
- **Register**: 10 attempts per hour per IP

### Per-Email Rate Limiting

Prevents targeted credential stuffing against specific accounts, even if the attacker uses multiple IPs (botnets, proxies, etc.).

- **Login**: 10 attempts per 10 minutes per email

### Implementation Notes

```go
func checkLoginRateLimit(ip string, email string) error {
    // Check per-IP limit first (cheaper, no email normalization needed)
    if !rateLimiter.Allow("login:ip:" + ip, 10, 10*time.Minute) {
        return ErrTooManyRequests
    }

    // Check per-email limit
    normalizedEmail := normalizeEmail(email)
    if !rateLimiter.Allow("login:email:" + normalizedEmail, 10, 10*time.Minute) {
        return ErrTooManyRequests
    }

    return nil
}
```

**Important**: Apply rate limit checks **before** doing expensive operations (password hashing, database lookups). Return the same generic error regardless of which limit was hit to avoid leaking information.

## Security Checklist

### Authentication

- [ ] Argon2id for password hashing (tuned for your infra)
- [ ] Constant-time comparison for password verification
- [ ] Rate limiting on login - **both per-IP and per-email**
- [ ] Rate limiting on register (per-IP)
- [ ] No user enumeration (same error for wrong email vs wrong password)
- [ ] Email normalization (lowercase, trim) before storage and lookup

### Sessions

- [ ] 120-bit random tokens (15 bytes, base32 encoded)
- [ ] SHA-256 hash before storage
- [ ] Separate public_id for session management APIs
- [ ] `__Host-session` cookie with HttpOnly, Secure, SameSite=Lax
- [ ] Max-Age matches server expires_at logic
- [ ] Refresh based on time-to-expiry (not created_at)
- [ ] Invalidate all sessions on password change

### CSRF

- [ ] Origin/Referer validation on state-changing endpoints
- [ ] Reject requests without Origin header (for browser-only APIs)

### Input Validation

- [ ] Email format validation
- [ ] Password minimum 8 characters
- [ ] Password maximum 128 characters (DoS prevention on hashing)

### Future (v1+)

- [ ] Email verification flow
- [ ] Password reset flow (requires email sending)
- [ ] OAuth providers
- [ ] Two-factor authentication
- [ ] Account lockout after N failed attempts
- [ ] API tokens for non-browser clients

## Configuration

```go
type AuthConfig struct {
    Enabled              bool          `json:"enabled"`
    SessionLifetime      time.Duration `json:"session_lifetime"`        // default: 30 days
    SessionRefreshWindow time.Duration `json:"session_refresh_window"`  // default: 15 days
    MaxSessionsPerUser   int           `json:"max_sessions_per_user"`   // default: 0 (unlimited)
    PasswordMinLength    int           `json:"password_min_length"`     // default: 8
    PasswordMaxLength    int           `json:"password_max_length"`     // default: 128
    AutoLoginOnRegister  bool          `json:"auto_login_on_register"`  // default: true

    // Rate limiting
    LoginRateLimitPerIP    RateLimit   `json:"login_rate_limit_per_ip"`    // default: 10/10min
    LoginRateLimitPerEmail RateLimit   `json:"login_rate_limit_per_email"` // default: 10/10min
    RegisterRateLimit      RateLimit   `json:"register_rate_limit"`        // default: 10/hour per IP

    // Argon2 params (tune for your infrastructure)
    Argon2Memory         uint32        `json:"argon2_memory"`           // default: 65536 (64MB)
    Argon2Iterations     uint32        `json:"argon2_iterations"`       // default: 3
    Argon2Parallelism    uint8         `json:"argon2_parallelism"`      // default: 4
}
```

## Implementation Effort Estimate

| Component           | Effort                     |
| ------------------- | -------------------------- |
| Schema + migrations | 2 hours                    |
| Password hashing    | 2 hours                    |
| Session management  | 3 hours                    |
| API endpoints       | 4 hours                    |
| CSRF protection     | 1 hour                     |
| Rate limiting       | 2 hours                    |
| SDK methods         | 3 hours                    |
| Testing             | 4 hours                    |
| **Total**           | **~21 hours (2.5-3 days)** |

## References

- [Lucia Auth](https://lucia-auth.com/) - Session-based auth library
- [The Copenhagen Book](https://thecopenhagenbook.com/) - Auth implementation guide
- [Lucia 3.0 Discussion](https://github.com/lucia-auth/lucia/discussions/1361) - Reasoning for sessions over JWTs
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
