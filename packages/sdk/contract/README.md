# SDK Contract Tests

These tests validate real API/SDK behavior together (not mocks).

## What is covered

- Database creation through the SDK
- CRUD flows through `client.database(...).from(...)`
- Count behavior (`count`, `withCount`)
- Batch atomicity (rollback when one operation fails)
- Error code contracts (`MISSING_WHERE_CLAUSE`)

## Requirements

1. Atomicbase API is running.
2. API is configured for database creation (Turso environment on the API side).
3. If API auth is enabled, set `ATOMICBASE_API_KEY`.

## Run

From repo root:

```bash
ATOMICBASE_CONTRACT=1 pnpm test:contract:sdk
```

Optional base URL override:

```bash
ATOMICBASE_CONTRACT=1 ATOMICBASE_CONTRACT_BASE_URL=http://localhost:8080 pnpm test:contract:sdk
```

If `ATOMICBASE_CONTRACT` is not set to `1`, tests are skipped intentionally.
