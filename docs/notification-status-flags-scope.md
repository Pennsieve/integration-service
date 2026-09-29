# Notification Status Flags and Notifications Table

2026-09-24, revised 2026-09-29 to match the settled ClickUp ticket
([Notification Status: Topic/Subscription/User-Level Enable-Disable](https://app.clickup.com/t/8664796/868m9jtky))
and what migration `20260925143247_add_topic_and_subscription_enabled` actually
ships. The ticket is the source of truth. If this doc and the ticket disagree,
the ticket wins, and this doc should be fixed.

## Why These Tasks Exist

Before this work, the notification system was delete-or-nothing. The only way
for a user to stop receiving notifications for a topic was to delete their
subscription. Because `notifications` rows cascade off `subscriptions`, and
`subscriptions` rows cascade off `topics`, that delete also erased the user's
notification history. Admins had no way to suppress a topic without destroying
every subscriber relationship.

Status flags replace deletes with reversible enable/disable switches. The
NOTIFICATIONS table keeps a durable record of every notification generated, so
suppressing sends doesn't erase the history users browse.

## Status Flags

| Layer | Column | Owned by | Shipped in this PR | Affects |
| --- | --- | --- | --- | --- |
| Topic | `topics.enabled BOOLEAN NOT NULL DEFAULT true` | Platform / admin | Column only; no API to change it | Every subscription on that topic |
| Subscription | `subscriptions.enabled BOOLEAN NOT NULL DEFAULT true` | User | Column + `PATCH /notification/subscription/{subscriptionId}` | One user's subscription |
| User | See the ticket | User | Not in this PR | All of that user's subscriptions |

These are plain booleans, not `status TEXT CHECK (...)` enums. Existing rows are
backfilled as enabled, because they were all live before the migration.

What the API does today:

- `PATCH /notification/subscription/{subscriptionId}` with `{"enabled": bool}`
  turns one of the caller's own subscriptions on or off. The update is scoped
  to the caller, so someone else's subscription returns 404.
- `POST /notification/topic/{topicId}/subscription` returns 409 when the topic
  is disabled. Subscribing again with a context that matches an existing
  disabled subscription re-enables it instead of creating a duplicate.
- `GET /notification/topics` lists disabled topics too, so clients can still
  label history posted under them. `GET /notification/subscriptions` and
  `GET /notification/messages` include disabled subscriptions, and the history
  already posted to them.

### A disabled topic implicitly disables its subscriptions

Per the ticket: *"If a topic is disabled, all subscriptions on that topic are
implicitly also disabled."*

"Implicitly" means the cascade is derived when the flags are read. It is never
written. Disabling a topic does **not** touch `subscriptions.enabled`. Anything
deciding whether a subscription is effectively on evaluates both flags:

```sql
-- Effective state of a subscription
t.enabled AND s.enabled
-- FROM notifications.subscriptions s
-- JOIN notifications.topics t ON t.topic_id = s.topic_id
```

This has two consequences, and both are intended:

- Re-enabling a topic brings back exactly the subscriptions that were enabled
  before, and nothing else. A user who turned a subscription off stays off,
  whatever happened to the topic in the meantime. Nothing has to remember which
  subscriptions were switched off by the topic and which by their owner.
- No trigger, and no multi-row `UPDATE` inside the topic-disable path, has to
  stay in sync with the subscription flags.

**Requirement for the queue-client notification-generation work:** its send
query must include the `t.enabled AND s.enabled` predicate. Right now the two
columns are fully independent. Only the read-time predicate ties them together,
so a send path that checks `s.enabled` alone would keep delivering on disabled
topics.

This gap can't be reached through the API yet, because there is no endpoint to
disable a topic. Topics can only be disabled with direct SQL.

### New subscriptions on a disabled topic

`CreateSubscription` reads `topics.enabled` inside its own transaction, with
`SELECT ... FOR SHARE`, before the insert or the re-enable upsert. It returns
`ErrTopicDisabled`, which the handler maps to 409. The handler's earlier
`GetTopic` check is only there to fail fast before schema validation. The
foreign-key constraint on its own doesn't catch this, because a disabled topic
still satisfies it. The row lock makes a concurrent topic disable wait until
the subscribe commits, rather than slipping in between the check and the
insert.

### Auditing

Per the ticket, no auditing is needed for v1. There is no
`notification_status_audit` table. The existing `notification_audit` table
tracks notification delivery events, not flag changes.

### User-level pause

The ticket settles this question, and this PR does not implement it. Before
adding a user-level column or a `paused_until` timestamp, check the ticket for
the decided shape. Don't take it from an earlier draft of this doc.

### Rolling back the migration

The down migration drops both `enabled` columns and doesn't preserve their
data. If users have already disabled subscriptions, a rollback loses that
opt-out state. Re-running the up migration then backfills every row as
`true`, which silently turns delivery back on for users who opted out. The
down file has a comment with a snapshot query to run before any rollback in an
environment where the flags have been used.

## NOTIFICATIONS Table

`notifications.notifications` is the durable record of every notification the
queue client generates. It holds what was generated, not only what was
delivered. Rows are written before the send gate is evaluated. That keeps the
history browsable when a send is suppressed.

| Column | Type | Notes |
| --- | --- | --- |
| `notification_id` | `SERIAL` PK | |
| `subscription_id` | FK → `subscriptions`, `ON DELETE CASCADE` | User and topic are reached through a join |
| `title` | `TEXT` | Rendered subject line |
| `message` | `TEXT` | Rendered body |
| `metadata` | `JSONB`, nullable | |
| `created_at` | `TIMESTAMPTZ` | When the row was written, not when it was sent |

`subscription_id` is the only FK on purpose. User, topic and delivery channel
are all reachable with a join, which keeps the table normalized.

Not in this PR: a nullable `sent_at TIMESTAMPTZ` would separate delivered rows
from suppressed ones. It belongs with the queue-client dispatch work. Whether
it's needed there depends on how that work records delivery. The existing
`user_notifications.delivered_at` may already cover it.

## Pipeline Placement

The status flags gate dispatch. They don't gate matching or row generation.

```
ChangelogEvent
  → match subscriptions (topic_id + context containment; no enabled check)
  → write NOTIFICATION row                       ← always happens
  → send gate: topics.enabled AND subscriptions.enabled
      → send, or skip
```

Leaving the flags out of the match query means history keeps accumulating
while a subscription or topic is off. The flags decide only whether anything is
delivered.

## API Endpoint Design (as shipped)

`GET /notification/messages` replaced the old
`GET /notification/{topicId}/notifications`. It returns the caller's
notifications across all their subscriptions, one page at a time:

| Parameter | Default | Notes |
| --- | --- | --- |
| `limit` | 50 | Values from 1 to 200. Anything else falls back to the default |
| `offset` | 0 | |
| `orderDirection` | `desc` | `asc` or `desc` by `created_at`, with `notification_id` as the tiebreaker. Any other value returns 400 |

The response is `{limit, offset, totalCount, messages}`.

`notificationsLastSeen` is read by `GET /notification/user/{userId}` and set by
a separate `PATCH /notification/user/{userId}`. The messages GET never updates
it, which avoids the race you'd get if concurrent GETs wrote it (see
`docs/notifications-last-seen.md`).

Still open for FE input:

- A `since` / `recent=true` window filter based on `notificationsLastSeen`. It
  isn't implemented, and the client can filter by `created_at` in the meantime.
- Filtering by topic or subscription ID. It's out of scope for v1.
