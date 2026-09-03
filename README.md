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

`docker-compose.test.yml` wires up three services: `pennsievedb` (the base seed image), `dbmigrate`
(this repo's own migration image/binary, run once against `pennsievedb` and then exits), and `test`
(`go test ./...`), which waits for `dbmigrate` to complete successfully before starting. All three
run on the same docker network, so `test` reaches the already-migrated database by container name
(`pennsievedb`) — no pre-baked custom image or extra container-orchestration tooling required.

```bash
make test         # local dev loop: starts pennsievedb + runs dbmigrate, then go test on the host
make test-docker  # full CI path: runs pennsievedb + dbmigrate + test all inside the docker network
make down         # tears down docker-compose.test.yml resources
```

`make test` publishes `pennsievedb`'s port via `docker-compose.local.override.yml` and reads
connection details from `test.env`, so `go test` runs directly on the host against the container.
`internal/dbmigrate/migrations_test.go` asserts against the resulting schema (FK/CHECK/UNIQUE
constraints on both the `webhooks` and `notifications` schemas) once migrations have been applied.
