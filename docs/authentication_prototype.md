
# Authentication Prototype Plan

The authentication prototype will be very focused and missing many core features. There will be no grants system, only API-key auth for server-side requests. We will include organizations but essentially just as a way to group users and create roles. No real auth functionality for them yet.

Note: Since auth tables are internal only, we can add columns to them without causing any breaking changes to the client-facing APIs. We will try not to rename or remove columns though.

Note: For text ids, we use 22 character base64 strings for 128 bits of entropy.

## Sessions

We will implement the full split-token sessions. No inactivity timeouts or rate limiting yet. We use HMAC-SHA256 for session secrets.

Session model:
```sql
CREATE TABLE sessions (
    id TEXT NOT NULL PRIMARY KEY,
    secret_hash BLOB NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id),
    mfa_verified INTEGER NOT NULL DEFAULT 0,
    last_verified_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    expires_at INTEGER NOT NULL
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
```

## Users

Users model:

```sql
CREATE TABLE users (
    id TEXT NOT NULL PRIMARY KEY,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    email TEXT UNIQUE COLLATE NOCASE,
    email_verified_at INTEGER,
    phone TEXT,
    phone_verified_at INTEGER,
    last_sign_in_at INTEGER,
    password_hash TEXT
);

CREATE UNIQUE INDEX users_phone_idx
ON users(phone)
WHERE phone IS NOT NULL;
```

We create a partial index on phone numbers because many users will never sign up with a phone number so we want to avoid indexing large amounts of null columns. Email is automatically indexed through unique constraint in SQLite.

## Authentication methods

The prototype will only include email magic link authentication for simplicity. All other authentication methods can be added relatively easily later.

Magic link model:

```sql
CREATE TABLE email_magic_links (
    id TEXT NOT NULL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    token_hash BLOB NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    CHECK (expires_at > created_at),
    CHECK (length(token_hash) = 32)
);

CREATE INDEX email_magic_links_token_hash_expires_idx ON email_magic_links(token_hash, expires_at);

CREATE INDEX email_magic_links_expires_at_idx ON email_magic_links(expires_at);
```

## MFA & Passkeys

This prototype will not implement MFA or passkeys. Passkeys will be a post-launch addition if added at all.

## Organizations

Organizations follow templates just like Databases do. For the prototype, organization templates will just define the structure of an organization (its databases, roles, etc). Eventually we will have a grants system for organizations as well.

Organizations model:

```sql
CREATE TABLE organizations (
    id TEXT NOT NULL PRIMARY KEY,
    [name] TEXT UNIQUE NOT NULL,
    [template_id] INTEGER NOT NULL
);

CREATE TABLE organizations_users (
    organization_id TEXT NOT NULL REFERENCES organizations(id),
    user_id TEXT NOT NULL REFERENCES users(id),
    [role] TEXT NOT NULL,
    PRIMARY KEY (organization_id, user_id)
);
```

## Prototype -> MVP Plan

The MVP will add:
- Inactivity timeouts for sessions
- Rate limiting for authentication endpoints (in-memory token bucket)
- Email & password authentication
- OAuth through OIDC only (custom adapters for non-OIDC later)
- MFA through TOTP (phone number will be added later)

Post-MVP:
- Phone number login
- Custom OAuth adapters (for non-OIDC compliant)
- Phone number OTP MFA
- SAML 2.0 + OIDC SSO (similar to WorkOS SSO)
- RLS & RBAC grants for databases and organizations.