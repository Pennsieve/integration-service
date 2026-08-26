# Splitting `dbmigrate` into Webhook / Notification Code Paths

## Background

`cmd/dbmigrate` is currently the only piece of the webhook/notification split that
isn't already separated. Everything else in the repo — `cmd/webhook`,
`cmd/notification`, `internal/webhook_*`, `internal/notification_*`, and the
`webhooks`/`notifications` Postgres schemas — is already domain-split. Migrations
for both domains live in one flat folder
([internal/dbmigrate/migrations](../internal/dbmigrate/migrations)), loaded via a
single `go:embed`, run by a single binary, and triggered by a single Jenkins
migration job.

### Current state

- **Runner**: `cmd/dbmigrate/main.go`, using the shared library
  `github.com/pennsieve/dbmigrate-go` (wraps `golang-migrate/migrate/v4`).
- **SQL files**: `internal/dbmigrate/migrations/*.up.sql` / `*.down.sql`, embedded
  via `//go:embed migrations/*.sql` in `internal/dbmigrate/config.go`. Three
  migration pairs today, all in one flat directory:
  - `20260630000000_create_webhooks_messages.{up,down}.sql`
  - `20260709000000_create_webhooks_sender_rate_limits.{up,down}.sql`
  - `20260803000000_create_notifications_schema.{up,down}.sql`
- **Schema separation**: fully separate Postgres schemas already exist —
  `webhooks` (`webhooks.messages`, `webhooks.sender_rate_limits`) and
  `notifications` (`.topics`, `.subscriptions`, `.notifications`, `.messages`,
  `.user_notifications`, `.preferences`, `.notification_audit`). No shared
  tables between them.
- **Parameterization today**: `dbmigrate-go`'s `pkg/config` reads
  `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`,
  `POSTGRES_DATABASE`, `POSTGRES_SCHEMA`, `VERBOSE_LOGGING`. Integration-service
  sets `POSTGRES_SCHEMA: "webhooks"` as a default in
  `internal/dbmigrate/config.go`, but this only tells golang-migrate which
  schema to store its version-tracking table in — it does **not** select which
  subset of SQL files to run. There is no existing flag/env var that runs a
  subset of migrations.
- **Invocation**: `cmd/dbmigrate/main.go` loads config → builds the embedded
  `MigrationsSource()` → `dbmigrate.NewLocalMigrator(...)` → `m.Up()`. Built into
  the `integration-service-dbmigrate` image via
  `Dockerfile.cloudwrap-dbmigrate`, run through `pennsieve/go-cloudwrap`
  (env vars injected from SSM per `terraform/ssm.tf`, keyed by
  `dbmigrate_service_name`). Makefile targets `package-dbmigrate` /
  `publish-dbmigrate` build/push a single image — unlike `package-webhook` /
  `package-notification`, which build separate Lambda zips. The Jenkinsfile
  (commit `a610e40`) triggers one external migration job for the whole service
  after `make publish` — not split by domain.

## Approach A: Two separate code paths

**Plan:**

1. Split `internal/dbmigrate/migrations` into two subdirectories,
   `migrations/webhooks/` and `migrations/notifications/`, moving the existing
   files by prefix (`create_webhooks_*` → webhooks,
   `create_notifications_schema` → notifications).
2. Create two config/embed sources, e.g. `internal/dbmigrate/webhooks/config.go`
   and `internal/dbmigrate/notifications/config.go`, each with its own
   `//go:embed` and its own `POSTGRES_SCHEMA` default (`"webhooks"` /
   `"notifications"`).
3. Split `cmd/dbmigrate/main.go` into `cmd/dbmigrate-webhook/main.go` and
   `cmd/dbmigrate-notification/main.go`, mirroring the existing
   `cmd/webhook` / `cmd/notification` split.
4. Add matching `Dockerfile.cloudwrap-dbmigrate-webhook` / `-notification`,
   Makefile targets (`package-dbmigrate-webhook`,
   `package-dbmigrate-notification`, plus `publish-*`), and SSM /
   `dbmigrate_service_name` entries in `terraform/ssm.tf`.
5. Update the Jenkinsfile to trigger two migration jobs (or one parameterized
   Jenkins job called twice) instead of one.

## Approach B: Single parameterized code path

Keep one `cmd/dbmigrate` binary but make it schema-aware: add a
`MIGRATION_TARGET` (or reuse `POSTGRES_SCHEMA`) env var/flag that selects which
embedded migration subdirectory to load. The SQL files still get split into
`migrations/webhooks/` and `migrations/notifications/` subfolders and both get
embedded, but the `iofs` source is chosen at runtime based on the parameter
instead of at compile time via a separate binary. Deployment stays a single
image / Makefile target / Jenkins job, invoked twice with different parameter
values.

## Comparison

| | **A: Two code paths** | **B: One parameterized path** |
|---|---|---|
| **Consistency with repo** | Matches existing `webhook`/`notification` binary split exactly — same mental model as the rest of the codebase | Introduces the only "parameterized by env var" binary in a repo that otherwise splits by domain everywhere else |
| **Blast radius / isolation** | A bad webhook migration can't accidentally touch notifications — no shared binary, no shared config surface | Shared binary means a bug in the runner (not the SQL) affects both domains; wrong parameter value at invocation time could point at the wrong schema |
| **Build/deploy surface** | More files: 2 Dockerfiles, 2 Makefile target pairs, 2 SSM service names, 2 (or duplicated) Jenkins job configs | Minimal new build surface — one image, one Makefile target, one Jenkins job invoked with different params |
| **Testing** | Each binary/config can be unit-tested independently with a narrower embed | One code path to test, but need coverage for the parameter-selection logic itself (e.g. invalid/missing param) |
| **Extensibility** | Adding a third domain (e.g. `event`) means copy-pasting another binary + Dockerfile + Makefile targets | Adding a third domain is just another migrations subfolder + another parameter value — no new build artifacts |
| **Operational safety** | Can't forget the parameter — no "run both" ambiguity, no risk of a typo'd env var silently running the wrong schema's migrations (or none) | Requires the parameter to be passed correctly at every call site (Jenkins job, local dev, CI) — a missing/misspelled value is a real failure mode |
| **Migration ordering** | No shared version-tracking table conflicts possible since each binary only knows its own schema | Must ensure the `iofs` source and `POSTGRES_SCHEMA` are always set together correctly — easy to desync if the parameter changes but the schema default doesn't |

## Recommendation

Given the repo already treats webhooks and notifications as fully separate
deployable units everywhere else, **Approach A (two code paths)** is the more
consistent fit and removes an entire class of "wrong parameter" operational
risk. Approach B saves some boilerplate but reintroduces exactly the coupling
the rest of the repo has deliberately avoided.
