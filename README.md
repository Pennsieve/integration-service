# integration-service
- Infrastructure for webhook integration notification system.
- Infrastructure for workflow invocation

## Webhook Workflow

1. API sends events to the ChangelogManager
2. ChangelogManager puts events on SNS
3. SQS subscribes to SNS and triggers Even_Lambda
4. EventLambda checks with postgres which events should be routed to which API endpoints

## Migrations

SQL migrations live under `internal/dbmigrate/migrations/`, split into one folder per Postgres
schema: `webhooks/` and `notifications/`. `cmd/dbmigrate` is a single binary/image that runs both
schemas' migrations, one after the other, each against its own schema.

To add a new migration, run:

```bash
./generate-migration-files.sh [webhooks|notifications] <name>
```

This creates a timestamped `<name>.up.sql` / `<name>.down.sql` pair in the given schema's folder.
Migration files should not create/drop their own schema (the migrator creates it automatically
from `POSTGRES_SCHEMA`) and should not schema-qualify table names within their own schema, since
the migrator's connection already has `search_path` set to it.

## Testing

```bash
make build-postgres  # builds a seeded Postgres image with this repo's migrations applied
make test             # runs go test (including the live-Postgres migration tests) against it
make docker-clean     # tears down the docker compose resources from either target above
```

`make build-postgres` only needs to be re-run after migration files change. `internal/dbmigrate/migrations_test.go`
uses `testcontainers-go` to start its own throwaway Postgres container and doesn't require
`build-postgres` to have been run first.
