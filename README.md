# integration-service

Pennsieve's event-integration service. It consumes platform events and turns them into outbound
webhooks and user notifications, serves the user-facing notifications API, and accepts inbound
webhooks. It is deployed as three Lambdas plus a migration image:

| Component | Entry point | What it does |
| --- | --- | --- |
| Event consumer | `cmd/event` → `handler.Handler` | Reads platform events from SQS, delivers dataset webhooks, and records notifications for matching subscriptions. |
| Notifications API | `cmd/notification` → `handler.NotificationHandler` | Topics, subscriptions, notification history, and notification preferences for the signed-in user. |
| Webhook receiver | `cmd/webhook` → `handler.WebhookHandler` | Accepts inbound webhook payloads and stores them. |
| DB migrator | `cmd/dbmigrate` | Runs the `webhooks` and `notifications` schema migrations. |

## Event Processing

pennsieve-api publishes events to the `integration-events` SNS topic, which fans out to an SQS
queue that triggers the event consumer (batches of up to 50). Each message carries:

```json
{
  "organizationId": "45",
  "datasetId": "123",
  "eventCategory": "PUBLISHING",
  "eventType": "COMPLETE_PUBLICATION",
  "eventDetail": { }
}
```

Ids may arrive as strings or numbers. `datasetId` is absent on organization-level events
(`eventCategory: "ORGANIZATION"`), which describe something that happened in a workspace rather
than to one dataset's subscribers.

For each batch the consumer:

1. **Decodes** every record (`internal/event_parser`). A record that can't be decoded is logged
   with `SKIPPED_EVENT_RECORD` and skipped instead of failing the batch; a CloudWatch alarm
   (`terraform/cloudwatch.tf`) fires on a sustained rate of those lines.
2. **Records notifications** (`internal/notification_generator`) for every event whose
   `eventType` names a topic. The event is matched to that topic's
   subscriptions whose context contains the topic's required fields read from the event
   (`internal/notification_matcher`), and one `notifications` row is written per matched
   subscription. Rows are written whatever the topic's and subscription's `enabled` flags are,
   since those gate sending, not history. Email and push delivery are not implemented yet.
3. **Delivers webhooks** (`internal/webhook_mapper`, `internal/webhook_sender`) for events with a
   `datasetId`, POSTing the event (without `eventDetail`) to every webhook registered in the
   organization for that dataset and `eventCategory`, with up to three attempts each.

Webhook and notification failures are logged, not returned, so one failure never makes SQS
redeliver (and re-send) the rest of the batch.

### Notification events

Each notification's title, message and metadata come from its event type's renderer. Event types
without one use the default renderer: the title is the `eventType`, and the message and metadata
are the event body (`organizationId`, `datasetId`, `eventCategory`, `eventType`, `eventDetail`) as
single-line JSON. Event types with their own renderer:

| Event type | Category | Topic context | Notifies |
| --- | --- | --- | --- |
| `DATASET_PUBLISHED_IN_WORKSPACE` | `ORGANIZATION` | `{"organizationId": <int>}`, no dataset scope | Members of the organization who subscribed, whenever any dataset in it is published to Discover. |

`DATASET_PUBLISHED_IN_WORKSPACE` is emitted alongside the dataset-scoped `COMPLETE_PUBLICATION`
changelog event, once Discover confirms the publication (first version or revision) is live. It
is deliberately separate from the `PUBLISHING` events (`REQUEST_*`, `ACCEPT_*`, `COMPLETE_*`,
... for publication, embargo, release, revision and unpublish), which stay dataset-scoped: an
event of this type that arrives with any other category is skipped. Its `eventDetail` is:

| Field | Type | Used for |
| --- | --- | --- |
| `datasetId` | int | Metadata |
| `datasetNodeId` | string | Metadata |
| `datasetName` | string | Title and message (required) |
| `publishedVersion` | int | Message names the version when it is > 1 (required) |
| `doi` | string | Message (required) |
| `ownerUserId` | int | Metadata |
| `ownerName` | string | Message, when present |

The decoded detail is stored as the notification's `metadata`.

## Database

The service owns two Postgres schemas, each migrated independently (see Migrations below).

`webhooks`:

| Table | Purpose |
| --- | --- |
| `messages` | Payloads accepted by the webhook receiver, with a generated `request_id`. |
| `sender_rate_limits` | Per-source-IP request counts backing the receiver's rate limit. |

`notifications`:

| Table | Purpose |
| --- | --- |
| `topics` | Event types users may subscribe to, named after the `eventType` they notify on. `context` is the JSON Schema a subscription's context must satisfy. `enabled` controls whether the topic accepts new subscriptions. |
| `subscriptions` | A user's subscription to a topic, scoped by a JSON `context` (e.g. an organization or dataset id). Unique on `(user_id, topic_id, context)`. `enabled` controls whether notifications are delivered to it. |
| `notifications` | Notifications recorded for a subscription: `title`, `message`, and JSON `metadata`. |
| `preferences` | One row per user: `email_enabled`, `sms_enabled`, `push_enabled`, and `notifications_last_seen` (`NULL` means never viewed). |
| `user_notifications` | Per-user read state (`READ`/`UNREAD`) for a notification. Not yet written by the service. |
| `messages` | Direct messages between users, optionally tied to a notification. Not yet written by the service. |
| `notification_audit` | Audit trail of events against a notification. Not yet written by the service. |

Migrations seed the `DATASET_PUBLISHED_IN_WORKSPACE` topic; other topics are maintained by hand.

Topics and subscriptions are switched off with `enabled = false`, never deleted: `notifications`
cascade-delete with their subscription (and subscriptions with their topic), so deleting would
erase the user's history. `preferences` rows are created on a user's first subscription or
preferences write, and reads treat a missing row as the column defaults. Every `user_id` column
foreign-keys to `pennsieve.users(id)`.

The service also reads tables it doesn't own: `pennsieve.users` and `pennsieve.organization_user`,
and each organization schema's `datasets` and webhook tables (`webhooks`,
`webhook_event_subscriptions`, `webhook_event_types`, `dataset_integrations`).

See [docs/notification-status-flags-scope.md](docs/notification-status-flags-scope.md) and
[docs/notifications-last-seen.md](docs/notifications-last-seen.md) for the design behind the
`enabled` flags and `notifications_last_seen`.

## API Endpoints

### Notifications API

Served by its own API Gateway (`terraform/notification_gateway.tf`) under the `notification` API
mapping key, so `GET /topics` in Terraform is `https://<api domain>/notification/topics`. Every
route is handled by one Lambda (`internal/handler/notification_handler.go`) and sits behind the
shared Pennsieve Lambda authorizer, which resolves the bearer token to the caller's user id. The
full request/response schemas are in `terraform/notification-service.yml`, which
`.github/workflows/rdme-openapi.yml` publishes to
[docs.pennsieve.io/reference](https://docs.pennsieve.io/reference) on every merge to `main`.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/notification/topics` | List every topic, including disabled ones, with its context schema. |
| `GET` | `/notification/subscriptions` | List the caller's subscriptions, enabled and disabled. |
| `POST` | `/notification/topic/{topicId}/subscription` | Subscribe the caller to a topic. The body is the subscription context: it must match the topic's context schema, a referenced dataset must exist, and an organization-only context requires the caller to be a member (`403` otherwise). Re-subscribing with the same context returns and re-enables the existing subscription. `409` if the topic is disabled. |
| `PATCH` | `/notification/subscription/{subscriptionId}` | Enable or disable one of the caller's subscriptions with `{"enabled": boolean}`. |
| `GET` | `/notification/messages` | Page through the notifications on all of the caller's subscriptions. Query params: `limit` (default 50, max 200), `offset` (default 0), `orderDirection` (`desc`, the default, or `asc`). Returns `{limit, offset, totalCount, messages}`. |
| `GET` | `/notification/user/{userId}` | Get the caller's `emailEnabled`, `pushEnabled`, and `notificationsLastSeen`. |
| `POST` | `/notification/user/{userId}` | Replace the caller's `emailEnabled` and `pushEnabled`. |
| `PATCH` | `/notification/user/{userId}` | Set only the caller's `notificationsLastSeen`. |

The `/notification/user/{userId}` routes require `{userId}` to be the caller's own id and return
`403` for any other value, whether or not that user exists. There is no `DELETE` route:
subscriptions are disabled rather than deleted, and clients group `GET /notification/messages` by
each notification's `topic_id`.

### Integration API

| Method | Path | Description |
| --- | --- | --- |
| `POST`, `PUT`, `PATCH`, `DELETE` | `/integration/webhook` | Store an inbound JSON payload in `webhooks.messages` and return `202` with its `request_id`. Requires the `X-Pennsieve-Webhook-Secret` header; bodies are capped at 1 MiB and each source IP at 60 requests per minute. |

The route is reachable from the internet but meant for internal senders, so its spec
(`terraform/integration-service.yml`) is not published.

## Migrations

SQL migrations live under `internal/dbmigrate/migrations/`, one folder per schema: `webhooks/` and
`notifications/`. `cmd/dbmigrate` runs both, each against its own schema.

To add a migration:

```bash
./generate-migration-files.sh [webhooks|notifications] <name>
```

This creates a timestamped `<name>.up.sql` / `<name>.down.sql` pair in that schema's folder.
Migrations should not create or drop their own schema (the migrator creates it from
`POSTGRES_SCHEMA`) and should not schema-qualify their own tables, since the connection's
`search_path` is already set to it. After adding one, run `make build-postgres` to rebuild the
pre-seeded image CI tests against, then update the `pennsievedb-integration` image tag in
`docker-compose.test.yml` to match.

## Testing

```bash
make test         # starts pennsievedb + runs dbmigrate in docker, then go test on the host
make test-docker  # runs pennsievedb, dbmigrate, and go test all inside the docker network
make test-ci      # runs go test against the pre-seeded pennsievedb-integration image (Jenkins)
make down         # tears down docker-compose.test.yml resources
```

`make test` reads connection details from `test.env`. `internal/dbmigrate/migrations_test.go`
asserts against the migrated schema, so it needs one of the databases above; every other package
is unit-tested with `go-sqlmock` and runs with plain `go test ./...`.
