# Local Postgres migrations + tests for integration-service

## Context

`integration-service` already has a working migration runner (`cmd/dbmigrate`, `internal/dbmigrate`, using `github.com/pennsieve/dbmigrate-go`) and a `Dockerfile.cloudwrap-dbmigrate` used to build the migration image for deployment. But there is currently **no way to run migrations or tests locally against Postgres**: no `docker-compose*.yml`, no `Dockerfile.test`, no `build-postgres.sh`, and the `Jenkinsfile` never runs `go test` at all. All existing `_test.go` files use `sqlmock` and never touch a real database.

`github-service` and `collections-service` (sibling Pennsieve repos) both solve this the same way:
1. Build a "seeded" Postgres image = the shared `pennsieve/pennsievedb:<tag>-seed` base image (which already has the core `pennsieve` schema, e.g. `pennsieve.users`) **plus** the service's own migrations baked in, committed as `pennsieve/pennsievedb-<service>:<tag>-seed`.
2. `docker-compose.test.yml` starts that seeded image + a `test` container (`Dockerfile.test`, plain `golang:1.23-alpine`, `go test ./...`) wired together via env vars that match `dbmigrate-go`'s config keys (`POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DATABASE`).
3. `Makefile` targets (`build-postgres`, `test`, `test-ci`, `docker-clean`) wrap the compose files so both local devs and Jenkins invoke the same thing.

`integration-service`'s migrations already assume the same base schema — `20260803000000_create_notifications_schema.up.sql` has FKs to `pennsieve.users` — so it needs exactly the same base seed image (`pennsieve/pennsievedb:V20241120161735-seed`) that both reference repos already use. This plan replicates that pattern, scaled down (no S3/minio needed — `internal/aws/aws.go` only talks to SSM, which stays mocked/unused in tests as today).

## Design

### 1. Seeded-Postgres build pipeline (mirrors `collections-service`)

- **`docker-compose.build-postgres.yml`** (new): `base-pennsievedb` service (`pennsieve/pennsievedb:V20241120161735-seed`, with the same healthcheck used by both reference repos) + `integration-migrations` service (`pennsieve/integration-service-dbmigrate:latest`, built from the existing `Dockerfile.cloudwrap-dbmigrate`, `entrypoint: ["/app/integration-service-dbmigrate"]` to bypass the cloudwrap wrapper), env vars `POSTGRES_HOST/PORT/USER/PASSWORD/POSTGRES_DATABASE` set to match `dbmigrate-go`'s `pkg/config/postgres.go` keys. No `POSTGRES_SCHEMA` override needed — `internal/dbmigrate/config.go`'s `ConfigDefaults()` already defaults to `webhooks`, and the notifications migration creates its own schema explicitly.
- **`build-postgres.sh`** (new, executable): copy of `collections-service/build-postgres.sh` with `REPO="pennsievedb-integration"` and `MIGRATIONS_FOLDER=internal/dbmigrate/migrations` (the real path in this repo). Computes a tag from the latest migration filename timestamp, runs migrations against the base container, commits the result as `pennsieve/pennsievedb-integration:<tag>-seed`.
- **`Makefile`**: add `build-postgres: package-dbmigrate` (target already has `package-dbmigrate`, just needs the new `build-postgres` target and a `docker-clean` / `docker-image-clean` pair), following the same shape as `collections-service/Makefile`.

### 2. Local/CI test runner (mirrors both repos)

- **`Dockerfile.test`** (new): `golang:1.23-alpine` (matches `go.mod`'s `go 1.23.0`), `COPY go.mod go.sum`, `go mod download`, `COPY cmd cmd` / `COPY internal internal`, `CMD ["go", "test", "-v", "./..."]`.
- **`docker-compose.test.yml`** (new): a `test` service (build from `Dockerfile.test`, `depends_on: pennsievedb-integration`, env `POSTGRES_HOST=pennsievedb-integration`, `POSTGRES_USER=postgres`, `POSTGRES_PASSWORD=password`, `POSTGRES_DATABASE=postgres`, and a `/var/run/docker.sock` volume mount so the testcontainers-based migration test in step 3 can start its own container from inside this one — same trick `collections-service` uses) + a `pennsievedb-integration` service pinned to the tag produced by `build-postgres.sh`, with port `5432` exposed so `go test` can also be run directly on the host against the same container.
- **`Makefile`**: add `test: docker-clean` → `docker compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test` then `make clean`; `test-ci` variant for Jenkins; update `docker-clean` to tear down both compose files.

### 3. Migration verification test (new, recommended addition)

- **`internal/dbmigrate/migrations_test.go`** (new), modeled directly on `collections-service/internal/dbmigrate/migrations_test.go`: uses `testcontainers-go` to start a throwaway `pennsieve/pennsievedb:V20241120161735-seed` container, then runs this repo's own `MigrationsSource()` / `ConfigDefaults()` through `dbmigrate.NewLocalMigrator(...).Up()` / `.Down()`, and asserts against the real schema — e.g. insert into `webhooks.messages`, insert a `notifications.topics` row then a `notifications.subscriptions` row referencing a seeded `pennsieve.users` row, confirm FK/constraint behavior (e.g. the `UNIQUE (user_id, topic_id, context)` constraint, `notifications.user_notifications.status` CHECK).
- This is the piece that actually "tests... on a Postgres Docker image" — it needs Docker but **not** the custom `pennsievedb-integration` seed image, so it works before `build-postgres` has ever been run.
- Requires two new `go.mod` dependencies: `github.com/testcontainers/testcontainers-go` (+ `.../wait`). Verification queries can stay on the existing `database/sql` + `github.com/lib/pq` stack already used in `internal/db` — no need to pull in `jackc/pgx/v5` just for this test.

### 4. CI wiring

- **`Jenkinsfile`**: add a `Run Tests` stage, mirroring `github-service/Jenkinsfile`: on non-main branches, first `make build-postgres` (`IMAGE_TAG=...`, `ENVIRONMENT=jenkins`) since this branch's seed image won't exist yet, then `make test-ci`. Wrap the existing stages in a `finally { make clean-ci }`. Leave the existing `Build and Push` / `Run Migrations` / `Deploy` stages for `main` untouched.

### 5. Docs

- **`README.md`**: short "Migrations" + "Testing" section (mirrors `collections-service/README.md`) explaining `make build-postgres`, `make test`, `make docker-clean`, and that `generate-migration-files.sh` (already present) is how new migration files get created.

## Files touched

- New: `docker-compose.build-postgres.yml`, `build-postgres.sh`, `Dockerfile.test`, `docker-compose.test.yml`, `internal/dbmigrate/migrations_test.go`
- Modified: `Makefile`, `Jenkinsfile`, `README.md`, `go.mod`/`go.sum` (testcontainers-go dep)

## Verification

1. `make build-postgres` (requires Docker running) → confirms `pennsieve/pennsievedb-integration:<tag>-seed` image builds, base container healthcheck passes, migrations run cleanly against `pennsieve.users`-backed schema.
2. `make test` → confirms existing `sqlmock`-based suite still passes unchanged inside the container, and the new `internal/dbmigrate/migrations_test.go` passes against a live, ephemeral Postgres container started via testcontainers.
3. `go build ./...` and `go vet ./...` still succeed after the `go.mod` update.
4. Read through updated `Jenkinsfile` to confirm stage ordering still matches the existing main-branch publish/migrate/deploy flow (can't execute Jenkins locally).
