# notificationsLastSeen

Tracks when a user last viewed their notifications, so the UI can show an
unread badge for anything newer.

The feature spans two repos:

| Piece | Repo | Status |
| --- | --- | --- |
| Storage + write endpoint + read endpoint | integration-service (this repo) | Done |
| `notificationsLastSeen` on `GET /user` | Pennsieve API | **Not done — see [Handoff](#handoff-get-user) below** |

## Storage

`notifications.preferences.notifications_last_seen TIMESTAMPTZ NULL`
(migration `20260917090937_add_notifications_last_seen`).

`notifications.preferences` already keys on `user_id` and already holds
per-user notification state, so this is one additive nullable column rather
than a new table. The column is nullable with **no default**: `NULL` means
"never viewed". Backfilling existing rows with `now()` would have claimed
every user had just read everything, which is the opposite of what an unread
badge should show on first load.

Two consequences worth knowing:

- A user with no `preferences` row at all also reads as "never viewed".
  Rows are seeded lazily — `CreateSubscription` inserts one on a user's first
  subscription — so a user who opens the notifications UI before subscribing
  to anything has no row yet. `GetNotificationsLastSeen` treats
  `sql.ErrNoRows` as `nil` rather than an error for exactly this reason.
- The write is therefore an **upsert**, not an update: opening the
  notifications UI has to be recordable whether or not a row exists. The
  inserted row takes the same channel defaults `CreateSubscription` would
  have given it.

## Endpoints (this repo)

All three live on the notification Lambda, behind the shared Pennsieve
Lambda authorizer, and all enforce that `{userId}` is the caller's own id.
`GET` returns, and `POST` fully replaces, the whole `notifications.preferences`
resource (`emailEnabled`/`pushEnabled` plus `notificationsLastSeen`); this
document covers only the `notificationsLastSeen` slice of it, which is
written separately via `PATCH` because it changes far more often — once per
notifications-UI open — than a user's channel preferences do. See the
README's [API Endpoints](../README.md#api-endpoints) section for the
`emailEnabled`/`pushEnabled` side.

### `PATCH /notification/user/{userId}`

```json
{ "notificationsLastSeen": "2026-09-03T14:22:00.000000Z" }
```

Returns `200` with the stored value:

```json
{ "userId": 42, "notificationsLastSeen": "2026-09-03T14:22:00Z" }
```

| Status | When |
| --- | --- |
| `200` | Persisted |
| `400` | Non-numeric `userId`, bad base64, invalid JSON, or `notificationsLastSeen` missing/null/unparseable |
| `401` | No or invalid bearer token |
| `403` | `{userId}` is not the caller |
| `404` | No such user (the `preferences` → `pennsieve.users` FK rejects it) |

The timestamp is normalized with `.UTC()` before it is stored, so a client
sending an offset (`2026-09-03T10:22:00-04:00`) and one sending the
equivalent `Z` time write the same value. This is a partial update: it never
touches `email_enabled`/`push_enabled`.

Mismatched-user requests return **`403`, not `404`**, even for user ids that
don't exist. The caller is authenticated, just not permitted — and answering
`404` for absent ids would turn the route into a probe for which user ids are
real.

### `GET /notification/user/{userId}`

Returns the full preferences resource, of which `notificationsLastSeen` is
one field:

```json
{ "userId": 42, "emailEnabled": true, "pushEnabled": false, "notificationsLastSeen": "2026-09-03T14:22:00Z" }
```

`notificationsLastSeen` is an explicit JSON `null` when the user has never
viewed notifications — the Go field is a `*time.Time` so that "never" can't
collapse into the zero time (`0001-01-01T00:00:00Z`).

This route exists so the stored value is verifiable from this service. It is
not the endpoint clients should build against for `notificationsLastSeen`
specifically; that is `GET /user`.

<a name="handoff-get-user"></a>
## Handoff: adding `notificationsLastSeen` to `GET /user`

`GET /user` is served by the **Pennsieve API**, not by this service, so that
half of the ticket is not implemented here. What that repo needs:

Join the user query across schemas and select the column:

```sql
SELECT u.id,
       u.node_id,
       u.email,
       ...
       p.notifications_last_seen
FROM pennsieve.users u
LEFT JOIN notifications.preferences p ON p.user_id = u.id
WHERE u.id = $1
```

`LEFT JOIN` is required, not `JOIN`: most users have no `preferences` row
(see [Storage](#storage)), and an inner join would drop them from `GET /user`
entirely.

Serialize as a nullable field named `notificationsLastSeen`, ISO 8601 UTC:

```json
{
  "id": "N:user:...",
  "email": "...",
  "notificationsLastSeen": "2026-09-03T14:22:00.000000Z"
}
```

Emit an explicit `null` rather than omitting the key when the value is
absent, so clients can distinguish "never viewed" from an older API version
that doesn't know the field.

Two prerequisites for that repo:

- Its Postgres role needs `USAGE` on the `notifications` schema and `SELECT`
  on `notifications.preferences`. The API's role may only be granted the
  `pennsieve` schema today — worth checking before the join is deployed.
- This repo's migration must be applied first, or the join will fail on an
  unknown column.

The change is purely additive — no existing field changes name, type, or
nullability — so current consumers of `GET /user` are unaffected.
