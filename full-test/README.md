# full-test

Deterministic simulation testing for the Atomicbase Data API.

This runner provisions a fresh, complex schema template using `@atomicbase/template`, pushes it via the Atomicbase CLI, creates a dedicated test database, and then runs a seeded stateful simulation against that database.

## Why this exists

- Reproducible failures via fixed seeds.
- Stateful API validation (not just one-off requests).
- Easy long-running stress mode (`-loop`) for constant testing.

## What gets provisioned

Before simulation starts, `full-test` creates a temporary workspace and uses the workspace packages directly:

- invokes `packages/cli` through `pnpm --filter @atomicbase/cli exec atomicbase ...`
- generates schema files that import `@atomicbase/template`

Then it runs:

- `atomicbase templates push <generated-template-name>`
- `atomicbase databases create <generated-database-name> --template <generated-template-name>`

The generated template is intentionally complex and includes:

- Multiple related tables (`users`, `workspaces`, `projects`, `tags`, `project_tags`, `todos`, `comments`, `attachments`, `audit_events`)
- Composite primary keys
- Foreign keys with mixed actions (`CASCADE`, `SET NULL`, `RESTRICT`)
- Generated columns
- `CHECK`, `UNIQUE`, and collations
- Secondary indexes and FTS tables

Simulation targets the `todos` table in that provisioned database.

## Run

```bash
cd full-test
go run . -api-key "$ATOMICBASE_API_KEY" -token "$ATOMICBASE_API_KEY"
```

## Common options

```bash
go run . -api-key "$ATOMICBASE_API_KEY" -token "$ATOMICBASE_API_KEY" -seed 123 -steps 2000
go run . -loop
go run . -base-url http://localhost:8080 -repo-root /path/to/atomicbase
go run . -keep-resources
go run . -provision=false -database existing-db -table todos
go run . -fail-on-4xx
```

## Env vars

- `ATOMICBASE_BASE_URL`
- `ATOMICBASE_API_KEY`
- `ATOMICBASE_DATABASE`
- `ATOMICBASE_TABLE`
- `ATOMICBASE_TOKEN`
- `ATOMICBASE_ID_COLUMN`
- `ATOMICBASE_TITLE_COLUMN`
- `ATOMICBASE_COMPLETED_COLUMN`
- `SIM_REPO_ROOT`
- `SIM_SEED`
- `SIM_STEPS`
- `SIM_LOOP`
- `SIM_PROVISION`
- `SIM_KEEP_RESOURCES`
- `SIM_TIMEOUT_MS`
- `SIM_FAIL_ON_4XX`

Flags always override env vars.

## Failure behavior

On mismatch or unexpected HTTP failure, the runner exits with a replay command including the exact seed.

By default, created template/database resources are deleted after the run. Use `-keep-resources` to retain them for debugging.
