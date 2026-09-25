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

## Database

Two Postgres schemas, each migrated independently (see Migrations above): `webhooks` and
`notifications`.

The `notifications` schema (`internal/dbmigrate/migrations/notifications/`) holds:

| Table | Purpose |
| --- | --- |
| `topics` | Event categories a user may subscribe to. `enabled` (boolean, default `true`) controls whether a topic accepts new subscriptions. |
| `subscriptions` | A user's subscription to a topic, optionally scoped by a free-form JSON `context` (e.g. a dataset id). Unique on `(user_id, topic_id, context)`. `enabled` (boolean, default `true`) controls whether new notifications are delivered to it. |
| `notifications` | Individual notification events posted to a subscription. |
| `user_notifications` | Per-user delivery/read state (`READ`/`UNREAD`) for a notification. |
| `preferences` | Per-user notification settings: `email_enabled`, `sms_enabled`, `push_enabled` (booleans, all with defaults), and `notifications_last_seen` (nullable timestamp, no default — `NULL` means "never viewed"). Keyed on `user_id`, one row per user. |
| `messages` | Direct messages between users, optionally tied to a notification. |
| `notification_audit` | Audit trail of events (e.g. delivery attempts) against a notification. |

`preferences` rows are seeded lazily — `CreateSubscription` inserts one on a user's first
subscription — so a user who has never subscribed to anything or set their preferences may have
no row at all. Reads treat a missing row the same as a row with the column defaults (see
`GetNotificationPreferences` in `internal/db/notification_db.go`); writes upsert so they succeed
regardless of whether a row exists yet. `sms_enabled` exists in the schema but is not yet exposed
by the API below.

Topics and subscriptions are switched off by setting `enabled = false`, never by deleting the
row: `notifications` cascade-delete with their subscription (and subscriptions with their topic),
so a delete would also erase the user's in-app and email notification history. A disabled
subscription stops receiving new notifications but keeps everything already posted to it, and
re-subscribing with the same context re-enables it.

Every table that references a user (`subscriptions.user_id`, `preferences.user_id`, etc.)
foreign-keys to `pennsieve.users(id)`, so writes for an unknown user id fail with a foreign-key
violation, which the Go layer maps to `db.ErrUserNotFound`.

See [docs/notifications-last-seen.md](docs/notifications-last-seen.md) for the design rationale
behind the nullable, no-default `notifications_last_seen` column specifically.

## API Endpoints

The notification/subscription routes have their own API Gateway (`notification_service_api` in
`terraform/notification_gateway.tf`), separate from the integration API in `terraform/gateway.tf`,
which now serves only `/webhook`. It is mapped onto the shared API domain under the
`notification` API mapping key, so its Terraform routes omit that prefix (`GET /topics` is served
at `https://<api domain>/notification/topics`).

All of its routes are served by a single Lambda (`internal/handler/notification_handler.go`) that
dispatches on the matched API Gateway route key; there's no per-route Lambda function. Every route
sits behind the shared Pennsieve Lambda REQUEST authorizer, which resolves the caller's bearer
token to a Pennsieve user id. The full request/response schemas are documented in
`terraform/notification-service.yml` (an OpenAPI spec kept for documentation, not wired into any
build).

Paths follow one naming rule: plural for a collection, singular for a single resource.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/notification/topics` | List every topic, including disabled ones (only enabled topics accept new subscriptions). |
| `GET` | `/notification/subscriptions` | List the caller's own subscriptions, enabled and disabled. |
| `POST` | `/notification/topic/{topicId}/subscription` | Subscribe the caller to a topic (upserts on `(user_id, topic_id, context)`, re-enabling a disabled match). `409` if the topic is disabled. |
| `PATCH` | `/notification/subscription/{subscriptionId}` | Enable or disable one of the caller's own subscriptions with `{"enabled": boolean}`. Replaces unsubscribing by deletion; history is kept. |
| `GET` | `/notification/messages` | List every notification posted to any of the caller's subscriptions (including disabled ones), newest first, paginated with `limit`/`offset`. |
| `GET` | `/notification/user/{userId}` | Get the caller's notification preferences: `emailEnabled`, `pushEnabled`, and `notificationsLastSeen`. |
| `POST` | `/notification/user/{userId}` | Fully replace the caller's `emailEnabled`/`pushEnabled` preferences. Never touches `notificationsLastSeen`. |
| `PATCH` | `/notification/user/{userId}` | Partially update just `notificationsLastSeen` (e.g. on opening the notifications UI), leaving `emailEnabled`/`pushEnabled` untouched. |

There is intentionally no `DELETE` route and no per-topic notifications route: subscriptions are
disabled rather than deleted, and clients group `GET /notification/messages` by each
notification's `topic_id` instead.

The `/notification/user/{userId}` routes all require `{userId}` to be the caller's own id —
any other value is rejected with `403`, even if that id doesn't exist, so the route can't be used
to probe which user ids are real.

`GET`/`POST`/`PATCH` on `/notification/user/{userId}` are split by how often each piece changes:
`notificationsLastSeen` is written on every notifications-UI open, while `emailEnabled`/`pushEnabled`
change rarely from a settings page — bundling both into one endpoint would make every last-seen
ping also carry (and risk clobbering) the user's channel settings.

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
