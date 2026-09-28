# Notification Status Flags and Notifications Table

2026-09-24

## Why These Tasks Exist

The current notification system is delete-or-nothing at every layer. The only way for a user to stop receiving notifications for a topic is to delete their subscription — losing their configuration permanently. There is no way for a platform admin to suppress a topic without removing it, which would destroy all subscriber relationships. And there is no user-level global mute: a user who wants a temporary break has no recourse except manually unsubscribing from everything and rebuilding on return.

These two tasks address that gap in tandem. Status flags introduce reversible enable/disable controls at three independent layers without touching subscription or topic records. The NOTIFICATIONS table establishes a durable record of every notification generated, decoupled from whether a delivery actually occurred — making it possible to suppress sends without erasing the notification history users expect to browse.

## Three Status Granularities

Three orthogonal boolean switches, each at a different layer of the notification stack. A notification is only delivered if all three resolve to enabled.

| Layer | Column | Owned by | Values | Affects |
| --- | --- | --- | --- | --- |
| Topic | `topics.status` | Platform / admin | `enabled`, `disabled` | All subscriptions to that topic, platform-wide |
| Subscription | `subscriptions.status` | User (per topic) | `enabled`, `disabled` | One user's subscription to one topic |
| Preference | `notification_preferences.status` | User (global) | `active`, `paused` | All of that user's notifications across all subscriptions |

The send gate evaluates all three in sequence:

```
should_send = topic.enabled AND subscription.enabled AND preference.not_paused
```

Status flags live after notification generation, at the send step. The ChangelogEvent → subscription-match → NOTIFICATION row pipeline is unaffected. Rows are always written; flags only gate dispatch.

## NOTIFICATIONS Table

This table is the source of truth for what the integration-service queue-client has actually dispatched — not what was matched or queued, but what was confirmed as a generated notification event. Rows are written before the send gate is evaluated, which is what makes pause semantics coherent: a paused user still accumulates rows and can browse "My Notifications" in the Pennsieve App.

| Column | Type | Notes |
| --- | --- | --- |
| `notification_id` | Serial, PK | Auto-populated by Postgres |
| `subscription_id` | FK → SUBSCRIPTIONS | Required on insert; user and topic inferred via join |
| `title` | text | Rendered email subject line |
| `message` | text | Rendered email body or push payload |
| `created_at` | timestamptz | Auto-populated by Postgres; when the row was written, not when it was sent |
| `sent_at` | timestamptz, nullable | **Proposed addition.** Null = suppressed or pending; populated when dispatch confirms. Required to distinguish delivered from paused-suppressed. |

`subscription_id` as the sole FK is intentional: user, topic, and delivery channel are all reachable via join, keeping the table normalized. `title` and `message` store post-render content (the human-readable output), not the raw ChangelogEvent — this is the right choice for auditability and for serving the frontend directly.

The gap between `created_at` and `sent_at` is meaningful. It records how long a notification sat suppressed, and is the primary signal for any future delivery reporting or debugging.

## API Endpoint Design

**Endpoint naming:** Rename `GET /notifications/notifications` to `GET /notifications/messages`. The current path is a namespace collision that will confuse every developer who touches it. Confirm with the FE team before changing.

**Query parameters:** Two parameters are proposed; they solve different UX problems and are not mutually exclusive.

| Parameter | Type | Behaviour |
| --- | --- | --- |
| `since` | timestamp | `WHERE created_at > ?` — caller supplies an absolute window |
| `recent` | boolean | Server fetches `notificationsLastSeen` for the user and applies it as the window |

`recent=true` is the better default for the webapp's notification feed — the client should not need to store and send `notificationsLastSeen` itself; that is server state. `since` suits programmatic or power use cases where the caller controls the window. If both parameters are present, `since` takes precedence.

Server-side resolution logic:

```
if since:
    WHERE created_at > since
elif recent:
    fetch user.notificationsLastSeen
    WHERE created_at > notificationsLastSeen
else:
    return all (paginated)
```

**Items not yet scoped — flag for FE input before launch:**

- Pagination (limit/offset or cursor). The notifications feed grows unboundedly; this is required before launch.
- Whether `GET /notifications/messages` updates `notificationsLastSeen`, or whether a separate `POST /notifications/seen` handles that. If the GET updates it, concurrent requests create a race condition.
- Filtering by topic or subscription ID — not in scope for v1, but the FE will likely request it once the UI is built.

## How the Two Tasks Relate

The NOTIFICATIONS table is what makes the status flag semantics viable. Without a durable row written before the send gate is evaluated, "pause notifications" would have to mean "don't generate the notification at all" — which would break the browsing experience. The two tasks share a design dependency even if they are independent in implementation.

The pipeline must be structured so the send gate is evaluated at dispatch time, not at generation time:

```
ChangelogEvent
  → match subscriptions
  → write NOTIFICATION row  ← always happens
  → enqueue send job
      → check topic.status + subscription.status + preference.status
      → send OR skip (set sent_at OR leave null)
```

If the current architecture inlines the send step with notification generation, separating them is a refactor, not just a schema change. This dependency should be scoped and confirmed before either task begins implementation.

## Open Questions and Recommendations

**1. Does pause take a duration/timestamp, or is it a manual toggle?**

Recommendation: manual toggle for v1, with a nullable `paused_until TIMESTAMPTZ` column added to `notification_preferences` now. Adding the column costs nothing and avoids a future migration. For v1, if `paused_until` is null and status is `paused`, the pause is indefinite. If `paused_until` is set, a check at send time (or a background job) can auto-resume. Do not build the scheduler in v1 unless there is an explicit product requirement.

**2. Do topic-level and subscription-level disables need separate audit tables?**

Recommendation: one shared audit table for v1, discriminated by `entity_type`. A single `notification_status_audit` table with columns `(entity_type, entity_id, changed_by, old_status, new_status, changed_at)` handles both. The distinction matters for UI — admins see topic-level changes, users see subscription-level changes — but that is a query filter, not a schema split. Separate tables only make sense if the two have meaningfully different access patterns or retention policies, which is not established.

**3. How does a disabled topic interact with existing subscriptions?**

Recommendation: dormant semantics — subscriptions are unaffected, and topic-level disable is a standalone gate in the send check. The alternative (cascading disable to all subscriptions) creates a silent state problem: if a topic is re-enabled, do subscriptions auto-resume? Which ones? What if a user manually disabled their subscription while the topic was disabled? Dormant semantics are simpler to reason about, audit, and test. Cascading creates hidden coupling and makes rollback ambiguous.

## Schema Summary

All changes are additive. No existing columns are modified.

```sql
-- Topic layer
ALTER TABLE topics
  ADD COLUMN status TEXT NOT NULL DEFAULT 'enabled'
    CHECK (status IN ('enabled', 'disabled'));

-- Subscription layer
ALTER TABLE subscriptions
  ADD COLUMN status TEXT NOT NULL DEFAULT 'enabled'
    CHECK (status IN ('enabled', 'disabled'));

-- Preference layer (user-global)
ALTER TABLE notification_preferences
  ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'paused')),
  ADD COLUMN paused_until TIMESTAMPTZ NULL;

-- NOTIFICATIONS table: proposed addition
ALTER TABLE notifications
  ADD COLUMN sent_at TIMESTAMPTZ NULL;

-- Shared audit log
CREATE TABLE notification_status_audit (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type  TEXT NOT NULL CHECK (entity_type IN ('topic', 'subscription', 'preference')),
  entity_id    UUID NOT NULL,
  changed_by   UUID NOT NULL REFERENCES users(id),
  old_status   TEXT,
  new_status   TEXT NOT NULL,
  changed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```
