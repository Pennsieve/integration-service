# Pennsieve Webhooks — Feature Guide (API-only Setup & Internal Architecture)

> **Status:** Internal reference, code-derived (pennsieve-api + integration-service) and
> cross-checked against the public docs. The Pennsieve App UI for managing webhooks was
> intentionally removed; **the API was retained**. Everything below is done via API calls,
> no UI.
>
> **Sources:** `pennsieve-api` (`WebhooksController`, `DataSetsController`, `WebhookManager`,
> `DatasetManager`, `ChangelogManager`, core-models, db tables); `integration-service` (Go
> event-consumer lambda); `pennsieve-db-migrations` (`webhook_event_types` seed); public docs
> at docs.pennsieve.io (`/docs/pennsieve-webhooks`, `/docs/pennsieve-apps`, `/reference/*webhook*`).
> Event generation (how upload produces events) is covered in §11.

---

## 1. Concept

A **webhook** (a.k.a. a Pennsieve "App" / integration) is a platform integration that:

1. Registers an **external API endpoint** (`apiUrl`) that Pennsieve calls when events occur.
2. Subscribes to a set of **platform event types** (dataset created, files uploaded,
   publication requested/accepted/rejected/withdrawn, etc.).
3. Is **enabled per-dataset** (or on all new datasets, if flagged default). Events only flow
   for datasets where the integration is enabled.

When Pennsieve does something on a dataset it generates a **ChangelogEvent**. Events are
published to SNS → SQS. Two independent consumers read them:

- **Jobs Service** — persists each event into the org's changelog DB table (the dataset
  activity log).
- **integration-service** — looks up which enabled webhooks match the event and **POSTs to
  their `apiUrl`**.

Because SNS fans out, each consumer gets its **own copy** of every event; they do not
contend for the same messages.

### Two authentication directions (don't conflate them)

| Direction | What it is | Credential |
|---|---|---|
| **Inbound** (integration → Pennsieve API) | The integration acting on the platform (reading/writing dataset data) | A dedicated **Integration User** + **API token/secret** minted at webhook-create time, returned **once** in the create response (`tokenSecret`). |
| **Outbound** (Pennsieve → your `apiUrl`) | Pennsieve notifying your endpoint of events | The webhook's `secret` field is stored for this purpose — **but see §7: the current delivery service does NOT send it.** |

---

## 2. Architecture & event flow

```
   EVENT PRODUCERS (emit CREATE_PACKAGE/RESTORE_PACKAGE/… jobs onto the shared jobs SQS queue
   via pennsieve-go-core pkg/changelog → EmitEvents = sqs.SendMessage):

   ┌─────────────────────┐  CREATE_PACKAGE
   │ upload-service-v2   │ ─────────────────┐
   ├─────────────────────┤  RESTORE_PACKAGE │        ┌──────────────┐
   │ packages-service    │ ─────────────────┼──(SQS  │ Jobs Service │
   │  (restore lambda)   │                  │  jobs  │ / job worker │
   ├─────────────────────┤                  │  queue)│              │
   │ ▷ FUTURE PRODUCERS  │  (planned, next  │───────▶└──────┬───────┘
   │   packages-service    3–6 months:      │               │ runs event through
   │   delete/viewer-asset; DELETE_PACKAGE, │               │ ChangelogManager
   │   datasets-service;   viewer-asset,     │               │
   │   others…)            dataset events)  ─┘   ◀───────────┤
   └─────────────────────┘                                  │
                                                             │
   ┌──────────────┐   ChangelogManager.logEvent() (in-proc)  │
   │ pennsieve-api │ ──────────────┐  ◀───────────────────────┘
   └──────────────┘                │ writes changelog row +
        (Scala)                    │ publishes to SNS
                                   ▼
                    ┌─────────────────────────────────┐
                    │ SNS: {env}-integration-events-   │
                    │      sns-topic                    │
                    └───────────────┬───────────────────┘
                         fan-out (each subscriber gets a copy)
              ┌────────────────────┴─────────────────────┐
              ▼                                            ▼
   ┌────────────────────┐                    ┌──────────────────────────────┐
   │ Jobs Service queue │                    │ SQS: {env}-event-integration- │
   │  → changelog DB     │                    │      queue  (DLQ after 3 tries)│
   │  (dataset activity) │                    └───────────────┬───────────────┘
   └────────────────────┘                       batch_size 50, window 2s
                                                               ▼
                                              ┌──────────────────────────────┐
                                              │ integration-service event      │
                                              │ lambda (Go, provided.al2023)   │
                                              │  1. parse SNS-wrapped SQS msg   │
                                              │  2. load org webhook subs (DB,  │
                                              │     10-min cache)               │
                                              │  3. match datasetId + event     │
                                              │  4. POST to each matching apiUrl│
                                              └───────────────┬────────────────┘
                                                              ▼
                                                   Your external endpoint(s)
```

**Event envelope published to SNS** (`ChangelogManager.formatMessageForSNS`,
`ChangelogManager.scala:190-200`):

```json
{
  "datasetId": "123",
  "organizationId": "45",
  "eventCategory": "PUBLISHING",
  "eventType": "REQUEST_PUBLICATION",
  "eventDetail": { /* event-specific JSON */ }
}
```

- `eventType` = the granular event name (the `ChangelogEventName` enum — see §6).
- `eventCategory` = the **coarse** label webhooks subscribe on: `METADATA`, `FILES`,
  `PUBLISHING`, `PERMISSIONS`, `RECORDS_AND_MODELS`, `CUSTOM` (`ChangelogManager.scala:180-188`).
  Note the remap: internal category `DATASET`→`METADATA`, `PACKAGES`→`FILES`.
- No `userId` in the SNS envelope (though the stored `ChangelogEvent` has one).

> Webhook matching happens on `eventCategory` (see §3-step-2 and §6); the granular `eventType`
> is passed through for the receiver to filter on.

---

## 3. The `/webhooks` API (create / list / get / update / delete)

All routes are under `/webhooks` (mounted in `ScalatraBootstrap.scala:271`), require an
authenticated, org-scoped Pennsieve session or API token, and operate in the caller's
organization.

Base URL (prod): `https://api.pennsieve.io`

### 3.1 Create — `POST /webhooks/`

Request body (`CreateWebhookRequest`, `WebhooksController.scala:38-49`):

| Field | Type | Required | Notes |
|---|---|---|---|
| `apiUrl` | string | ✅ | Your endpoint; trimmed, non-empty, < 256 chars |
| `description` | string | ✅ | non-empty, < 200 chars |
| `secret` | string | ✅ | shared secret for outbound verification; non-empty, < 256 chars |
| `displayName` | string | ✅ | non-empty, < 256 chars; `name` slug derived from it |
| `hasAccess` | boolean | ✅ | if true, integration user gets **Manager** on enabled datasets; else **Viewer** |
| `isPrivate` | boolean | ✅ | if true, only the creator (or superAdmin) can see/enable it |
| `isDefault` | boolean | ✅ | if true, auto-enabled on datasets via `enableDefaultWebhooks` |
| `imageUrl` | string | ❌ | icon URL; empty or < 256 chars |
| `targetEvents` | string[] | ❌ | event names to subscribe to (see §5) |
| `customTargets` | object[] | ❌ | `WebhookTargetDTO` list (see §6) |

**Side effects** (`WebhooksController.scala:81-157`): creates an Integration User, adds it to
the org (`DBPermission.Delete`), mints an API token+secret for it, then inserts the webhook
row + event-type subscriptions.

**Response `201 Created`** — `WebhookDTO` (§4) **with `tokenSecret` populated**:

```json
{
  "id": 42,
  "apiUrl": "https://example.com/pennsieve-hook",
  "displayName": "My Integration",
  "name": "my-integration",
  "eventTargets": ["CREATE_DATASET", "REQUEST_PUBLICATION"],
  "tokenSecret": { "name": "...", "key": "<API_KEY>", "secret": "<API_SECRET>", "lastUsed": null },
  "...": "..."
}
```

> 🔑 **Save `tokenSecret.key` and `tokenSecret.secret` now** — they are returned only on
> create. This is how the integration authenticates *back into* Pennsieve.

```bash
curl -X POST https://api.pennsieve.io/webhooks/ \
  -H "Authorization: Bearer $PENNSIEVE_JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "apiUrl": "https://example.com/pennsieve-hook",
    "description": "Notifies my service of publishing events",
    "secret": "a-long-random-shared-secret",
    "displayName": "My Integration",
    "hasAccess": false,
    "isPrivate": true,
    "isDefault": false,
    "targetEvents": ["PUBLISHING"]
  }'
# NB: targetEvents are CATEGORY strings (see §6). This subscribes to ALL publishing
# stages; filter on the delivered payload's "eventType" (e.g. REQUEST_PUBLICATION) yourself.
```

**Create response body (full `WebhookDTO`)** — field names are the circe-derived JSON keys
(`WebhookDTO.scala`, `deriveEncoder`):

```json
{
  "id": 42,
  "apiUrl": "https://example.com/pennsieve-hook",
  "imageUrl": "https://example.com/icon.png",
  "description": "Notifies my service of publishing events",
  "name": "my-integration",
  "displayName": "My Integration",
  "isPrivate": true,
  "isDefault": false,
  "isDisabled": false,
  "hasAccess": false,
  "eventTargets": ["PUBLISHING"],
  "tokenSecret": {
    "name": "Integration-user",
    "key": "<PENNSIEVE_API_KEY>",
    "secret": "<PENNSIEVE_API_SECRET>",
    "lastUsed": null
  },
  "customTargets": null,
  "createdBy": 1234,
  "createdAt": "2026-07-16T12:00:00Z"
}
```
`tokenSecret` is populated **only** in this create response (secret #1 in §7.1). On all other
endpoints it is `null`.

### 3.2 List — `GET /webhooks/`  (operation id `getIntegrations`)

- **Request:** no params, no body. `Authorization: Bearer <jwt>`.
- **Returns:** `200 OK`, a JSON **array** of `WebhookDTO`. Visible = public webhooks
  (`isPrivate=false`) plus the caller's own. `tokenSecret` is `null` on every element;
  `eventTargets` is populated. (`WebhooksController.scala:159-182`.)

```json
[
  { "id": 42, "apiUrl": "https://...", "displayName": "My Integration",
    "name": "my-integration", "isPrivate": true, "isDefault": false, "isDisabled": false,
    "hasAccess": false, "eventTargets": ["PUBLISHING"], "tokenSecret": null,
    "customTargets": null, "createdBy": 1234, "createdAt": "2026-07-16T12:00:00Z",
    "description": "...", "imageUrl": null }
]
```

### 3.3 Get one — `GET /webhooks/{id}`

- **Request:** path param `id: Int` (int32); no body.
- **Returns:** `200 OK`, a single `WebhookDTO` (same shape as above, `tokenSecret: null`).
- **Errors:** `404` if not found; `403`-style `InvalidAction` if private and caller is not the
  creator. (`WebhooksController.scala:184-205`.)

### 3.4 Update — `PUT /webhooks/{id}`

- **Request:** path param `id: Int`. Body `UpdateWebhookRequest` — **all fields optional**
  (`WebhooksController.scala:51-63`):

```json
{
  "apiUrl": "https://example.com/new-hook",
  "imageUrl": "https://example.com/icon.png",
  "description": "Updated description",
  "secret": "new-shared-secret",
  "displayName": "My Integration (v2)",
  "targetEvents": ["PUBLISHING", "FILES"],
  "customTargets": null,
  "hasAccess": true,
  "isPrivate": false,
  "isDefault": false,
  "isDisabled": false
}
```

- **Permission:** `DBPermission.Administer` (superAdmin, webhook creator, or org Administer).
- **`targetEvents` semantics:** `null`/omitted → leave subscriptions unchanged; `[]` → remove
  all; `[...]` → replace with exactly this set (unknown names → predicate error).
- **Returns:** `200 OK`, the updated `WebhookDTO` (`tokenSecret: null`). `404` if id not found.
- **Caveats in current code:** `hasAccess` is accepted but **not applied** in the merge;
  changing `displayName` regenerates the `name` slug.

### 3.5 Delete — `DELETE /webhooks/{id}`

- **Request:** path param `id: Int`; no body.
- **Returns:** `200 OK`, body is a bare integer (affected row count, typically `1`).
- **Permission:** `Administer`. Side effects: deletes the webhook, revokes the integration
  user's API token, removes it as a collaborator from all datasets, and removes it from the
  org. (`WebhooksController.scala:249-317`.)

### 3.6 Field reference (request bodies)

`CreateWebhookRequest` (`WebhooksController.scala:38-49`) — see the table in §3.1.
`UpdateWebhookRequest` (`WebhooksController.scala:51-63`) — same fields, all `Option`, plus
`isDisabled`. `customTargets` element = `WebhookTargetDTO` (§6):
`{ "target": "PACKAGE|PACKAGES|RECORD|RECORDS|DATASET", "filter": { "packageFilter": { "fileType": ["CSV", ...] } } }`.

---

## 4. `WebhookDTO` (response shape) — field reference

`WebhookDTO.scala` (JSON keys via circe `deriveEncoder`, so identical to field names):

| Field | Type | Notes |
|---|---|---|
| `id` | int | |
| `apiUrl` | string | |
| `imageUrl` | string \| null | |
| `description` | string | |
| `name` | string | slug derived from `displayName` |
| `displayName` | string | |
| `isPrivate` | bool | |
| `isDefault` | bool | |
| `isDisabled` | bool | |
| `hasAccess` | bool | |
| `eventTargets` | string[] \| null | subscribed **category** names |
| `tokenSecret` | object \| null | `{name, key, secret, lastUsed}` — **create only** |
| `customTargets` | object[] \| null | `WebhookTargetDTO[]` |
| `createdBy` | int | user id |
| `createdAt` | string (ISO-8601) | |

`tokenSecret` = `APITokenSecretDTO` (`APITokenSecretDTO.scala`): `name: string`, `key: string`
(the API token/key), `secret: string` (`TokenSecret.plaintext`, cleartext, once), `lastUsed:
string|null`.

---

## 5. Enabling a webhook on a dataset

Registering a webhook does **not** make it fire. You must enable it per-dataset. These routes
live on **`DataSetsController`** (`/datasets/*`), keyed by dataset **node id** (`N:dataset:...`).

| Action | Route | Permission |
|---|---|---|
| List enabled integrations | `GET /datasets/{datasetNodeId}/webhook` | `ViewWebhooks` |
| **Enable** | `PUT /datasets/{datasetNodeId}/webhook/{webhookId}` | `ManageWebhooks` |
| **Disable** | `DELETE /datasets/{datasetNodeId}/webhook/{webhookId}` | `ManageWebhooks` |

**Enable** (`DataSetsController.scala:5182-5218` → `DatasetManager.enableWebhook`,
`1652-1673`) inserts a `dataset_integrations` row and adds the integration user as a dataset
collaborator — **Manager if `hasAccess=true`, else Viewer**.

**Default webhooks:** `isDefault=true` webhooks are auto-enabled on datasets via
`enableDefaultWebhooks` (`DatasetManager.scala:1695+`), so they attach without an explicit
`PUT`.

```bash
# enable webhook 42 on a dataset
curl -X PUT "https://api.pennsieve.io/datasets/N:dataset:abc.../webhook/42" \
  -H "Authorization: Bearer $PENNSIEVE_JWT"

# verify
curl "https://api.pennsieve.io/datasets/N:dataset:abc.../webhook" \
  -H "Authorization: Bearer $PENNSIEVE_JWT"
```

### Manually trigger a custom event
`POST /datasets/{datasetNodeId}/event` with body `{"eventType": "...", "message": "..."}`
(permission `TriggerCustomEvents`; blocked in demo/sandbox org). Logs a `CUSTOM_EVENT` to the
changelog and publishes to SNS. (`DataSetsController.scala:5105-5151`.)

---

## 6. Event types you can subscribe to (`targetEvents`)

> **You subscribe at the CATEGORY level, not the individual-event level.** `targetEvents`
> values must be the coarse **category** strings below, which are the only rows seeded into
> the `webhook_event_types` lookup table. Subscribing to a granular name like
> `REQUEST_PUBLICATION` will be **rejected** with a predicate error. When a webhook fires,
> the payload includes the granular `eventType` so your endpoint can filter further itself.

### Subscribable categories (the `webhook_event_types` seed)

Seeded by `pennsieve-db-migrations`
(`organization/V20210616170959__add_webhook_event_types.sql` +
`V20211218094309__add_custom_event_type.sql`). These are the **only** valid `targetEvents`
values:

| `targetEvents` value | Fires on… | Emitted by API? |
|---|---|---|
| `METADATA` | dataset metadata/lifecycle changes (name, description, license, tags, README, banner, collections, contributors, external pubs, ignore files, **status**) | ✅ |
| `FILES` | package/file changes (create, rename, move, delete, restore) | ✅ |
| `RECORDS_AND_MODELS` | model & record create/update/delete (incl. properties) | ✅ |
| `PERMISSIONS` | permission & owner changes | ✅ |
| `PUBLISHING` | all publishing/embargo/removal/revision workflow stages | ✅ |
| `CUSTOM` | custom events triggered via `POST /datasets/{id}/event` | ✅ |
| `STATUS` | *(nothing — see §8.2)* | ❌ **never emitted** |

### Granular `eventType` values delivered in the payload

pennsieve-api maps each granular `ChangelogEventName` to one of the categories above
(`ChangelogManager.eventCategory`, `ChangelogManager.scala:179-188`; category enum in
`ChangelogEventName.scala`). The webhook subscribes to the **category**; the **`eventType`**
field of the delivered payload carries the specific name so you can filter:

- **→ `METADATA`** (internal category `DATASET`): `CREATE_DATASET`, `UPDATE_METADATA`,
  `UPDATE_NAME`, `UPDATE_DESCRIPTION`, `UPDATE_LICENSE`, `ADD_TAG`, `REMOVE_TAG`,
  `UPDATE_README`, `UPDATE_BANNER_IMAGE`, `ADD_COLLECTION`, `REMOVE_COLLECTION`,
  `ADD_CONTRIBUTOR`, `REMOVE_CONTRIBUTOR`, `ADD_EXTERNAL_PUBLICATION`,
  `REMOVE_EXTERNAL_PUBLICATION`, `UPDATE_IGNORE_FILES`, `UPDATE_STATUS`
- **→ `FILES`** (internal category `PACKAGES`): `CREATE_PACKAGE`, `RENAME_PACKAGE`,
  `MOVE_PACKAGE`, `DELETE_PACKAGE`, `RESTORE_PACKAGE`
- **→ `RECORDS_AND_MODELS`**: `CREATE_MODEL`, `UPDATE_MODEL`, `DELETE_MODEL`,
  `CREATE_MODEL_PROPERTY`, `UPDATE_MODEL_PROPERTY`, `DELETE_MODEL_PROPERTY`, `CREATE_RECORD`,
  `UPDATE_RECORD`, `DELETE_RECORD`
- **→ `PERMISSIONS`**: `UPDATE_PERMISSION`, `UPDATE_OWNER`
- **→ `PUBLISHING`**: `REQUEST_PUBLICATION`, `ACCEPT_PUBLICATION`, `REJECT_PUBLICATION`,
  `CANCEL_PUBLICATION`, `REQUEST_EMBARGO`, `ACCEPT_EMBARGO`, `REJECT_EMBARGO`,
  `CANCEL_EMBARGO`, `RELEASE_EMBARGO`, `REQUEST_REMOVAL`, `ACCEPT_REMOVAL`, `REJECT_REMOVAL`,
  `CANCEL_REMOVAL`, `REQUEST_REVISION`, `ACCEPT_REVISION`, `REJECT_REVISION`,
  `CANCEL_REVISION`, `UPDATE_CHANGELOG`
- **→ `CUSTOM`**: `CUSTOM_EVENT`

> Note the category remap: internal `DATASET`→`METADATA` and `PACKAGES`→`FILES`. So "files
> uploaded" style events (`CREATE_PACKAGE`) arrive under category **`FILES`**, and dataset
> status changes (`UPDATE_STATUS`) arrive under **`METADATA`**, not `STATUS`.

### `customTargets` (optional, advanced)
`WebhookTargetDTO` = `{ target: IntegrationTarget, filter?: {packageFilter?: {fileType: [...]}} }`.
`IntegrationTarget` ∈ `PACKAGE`, `PACKAGES`, `RECORD`, `RECORDS`, `DATASET`. Used to scope
which platform objects the integration targets.

---

## 7. What your endpoint receives (delivery contract)

Delivered by the **integration-service** event lambda (Go). Per matching event:

- **Method:** `POST` to your `apiUrl`.
- **Headers:** `Content-Type: application/json` **only**.
- **Body (general endpoints):** the event envelope, e.g.
  ```json
  {"organizationId":"45","datasetId":123,"eventCategory":"PUBLISHING","eventType":"REQUEST_PUBLICATION"}
  ```
  (Note: the delivered body carries `organizationId/datasetId/eventCategory/eventType`; it
  does **not** currently include `eventDetail`.)
- **Body (Slack URLs, prefix `https://hooks.slack.com/`):** `{"text":"<envelope JSON as string>"}`.
- **One POST per (url, event)** — not batched.
- **Timeout:** 250ms **connect** timeout only (read time unbounded).
- **Retries:** up to 3, backoff `2s * attempt + jitter`, sequential.

> ⚠️ **No outbound authentication is sent.** No `secret`, HMAC signature, or bearer token is
> attached to the outbound POST — despite the `secret` field being collected at create time.
> A receiver **cannot currently verify** a call genuinely came from Pennsieve from headers
> alone. See §8 and the secret model below.

### 7.1 The three "secrets" — do not conflate them

The word "secret" appears in three unrelated places. This trips people up, so explicitly:

| # | Name | Where it comes from | Direction | Used by integration-service? | Sent to your `apiUrl`? |
|---|---|---|---|---|---|
| 1 | **`tokenSecret`** (`{name, key, secret}`) | Returned **once** by `POST /webhooks/`. It is a Pennsieve **API token + secret for the auto-created Integration User** (`tokenManager.create`, `WebhooksController.scala:122-129`; `secret` = `TokenSecret.plaintext`). | **Inbound** — how the integration calls *back into* the Pennsieve API (Cognito-backed). | **No.** Never read. | **No.** |
| 2 | **`webhook.secret`** | The `secret` field you pass in `CreateWebhookRequest`; stored on the `webhooks` row. Intended as a shared secret for the receiver to verify outbound calls. | **Outbound** (intended). | **No** — the cache query (`cache.go:31-35`) doesn't even `SELECT` it, and `webhook_sender.go` never references it. | **No.** Collected and stored but **entirely unused** today. |
| 3 | **`X-Pennsieve-Webhook-Secret`** | A separate pre-shared secret in SSM (`/{env}/integration-service/webhook-shared-secret`), validated by integration-service's **inbound receiver lambda** (`webhook_handler.go:38,79-110`). | Inbound **to** the receiver test-sink lambda. | Yes — but only by the *receiver* lambda, which is a test sink, not the delivery path. | N/A |

**Answering the specific questions:**
- *Are `tokenSecret` key/name/secret inspected or validated in the Integration Service?* **No.** Integration-service never sees them; they are Pennsieve-API credentials for the Integration User.
- *Are they passed to the external webhook URL?* **No.** The outbound POST (`webhook_sender.go:39-45`) carries only `Content-Type: application/json`.
- *So how would a receiver verify authenticity today?* It can't from the request alone. The only integrity signal is source-IP/network, or the receiver could out-of-band require callers to use a secret path/query in `apiUrl`. Adding an HMAC of the body keyed on `webhook.secret` (or a bearer header) would be the natural fix and is a strong candidate for the 3–6 month expansion.

---

## 8. Known gaps & caveats (important)

These are real behaviors of the current code, worth knowing before relying on the feature:

1. **Category-level subscription matching — VERIFIED CORRECT (not broken).** An earlier
   draft of this doc worried the matcher compared incompatible vocabularies. That was wrong.
   Confirmed against the seed: `webhook_event_types` is seeded with **coarse categories**
   (`METADATA, STATUS, RECORDS_AND_MODELS, FILES, PERMISSIONS, PUBLISHING, CUSTOM`), and
   pennsieve-api's SNS `eventCategory` uses the **same** coarse vocabulary
   (`ChangelogManager.scala:179-188`). integration-service matches
   `datasetId:eventCategory` (message) against `datasetId:event_name` (subscription)
   (`webhook_mapper.go:43,61`) — **same vocabulary, matching works.** The subtlety to
   communicate to users is that subscription is *category-grained*, so a webhook subscribed
   to `PUBLISHING` receives **every** publishing stage and must filter on the payload's
   `eventType` itself.

2. **`STATUS` is subscribable but never fires.** `webhook_event_types` seeds a `STATUS` row
   (and the public docs list a "Status" category), but pennsieve-api's `eventCategory` remap
   produces no `STATUS` — dataset status changes emit `UPDATE_STATUS`, which maps to
   `METADATA` (`ChangelogManager.scala:181`). A subscriber selecting `STATUS` will receive
   nothing; they should subscribe to `METADATA` and filter for `eventType == "UPDATE_STATUS"`.

3. **No outbound auth/signature** (§7) — receivers can't verify authenticity from the request.
   The `secret` collected at create time is stored but never sent by the current delivery
   service.

4. **`eventDetail` is dropped in delivery.** pennsieve-api's SNS envelope includes
   `eventDetail` (the event-specific JSON), but integration-service's `EventMessage` model
   only carries `organizationId/datasetId/eventCategory/eventType` — so the detail payload is
   **not forwarded** to your endpoint.

5. **Delivery failures are swallowed** — exhausted retries are logged only; the SQS batch
   still succeeds, so **no DLQ redelivery** for delivery failures (only parse failures DLQ).

6. **No delivery statistics persisted.** Despite a `webhook_statistics` table existing in
   pennsieve-api's schema, integration-service does **not** write it. Success/failure is only
   in CloudWatch logs.

7. **Whole-batch failure on any malformed record** during SNS/SQS parse (a per-record
   skip-and-log is a TODO).

8. **Sequential delivery** — retries `time.Sleep` block the whole batch (parallelism is a
   TODO).

9. **`update` quirks** — `hasAccess` is accepted but not applied on `PUT /webhooks/{id}`;
   changing `displayName` regenerates the `name` slug.

---

## 9. Data model (reference)

All webhook tables live in the **per-organization Postgres schema** (`"{orgSchemaId}".*`),
not the shared `pennsieve` schema.

| Table | Key columns |
|---|---|
| `webhooks` | `id`, `api_url`, `image_url?`, `description`, `secret`, `name`, `display_name`, `is_private`, `is_default`, `is_disabled`, `has_access`, `integration_user_id`, `webhook_targets` (JSON), `created_by`, `created_at` |
| `webhook_event_subscriptions` | `id`, `webhook_id`, `webhook_event_type_id` |
| `webhook_event_types` | `id`, `event_name` |
| `webhook_statistics` | `webhook_id` (PK), `successes`, `failures`, `date` |
| `dataset_integrations` | `id`, `webhook_id`, `dataset_id`, `enabled_by`, `enabled_on` |

integration-service reads `webhooks ⨝ webhook_event_subscriptions ⨝ dataset_integrations ⨝
webhook_event_types` for the caller's org schema (`cache.go:31-35`), 10-min in-lambda cache,
force-refreshed when a `CREATE_DATASET` event appears in a batch.

---

## 10. End-to-end setup checklist (API only)

1. **Create** the webhook: `POST /webhooks/` with `apiUrl`, `secret`, `displayName`,
   `description`, `hasAccess`, `isPrivate`, `isDefault`, and `targetEvents` — the **category**
   strings from §6 (`PUBLISHING`, `FILES`, `METADATA`, …), not granular event names.
   **Save `tokenSecret.key`/`.secret`** from the response.
2. **Enable** on each dataset: `PUT /datasets/{datasetNodeId}/webhook/{webhookId}`
   (or set `isDefault=true` to auto-enable on new datasets).
3. **Verify** enablement: `GET /datasets/{datasetNodeId}/webhook`.
4. **Test** delivery: perform a subscribed action (or `POST /datasets/{id}/event`) and watch
   your endpoint. If nothing arrives: confirm you subscribed to the **category** (not a
   granular name), and note that `STATUS` never fires (§8.2). Filter on the payload's
   `eventType` for the specific action.
5. **Manage:** `GET /webhooks/`, `GET /webhooks/{id}`, `PUT /webhooks/{id}` (Administer),
   `DELETE /webhooks/{id}` (Administer).
6. **Disable/clean up:** `DELETE /datasets/{id}/webhook/{webhookId}`, then
   `DELETE /webhooks/{id}`.

---

## 11. Where events come from (event generation)

Not all events originate synchronously inside pennsieve-api. There are **two paths** by which
a platform event reaches `ChangelogManager` (and therefore the integration-events SNS topic):

**Path A — in-process API actions.** When a `pennsieve-api` request mutates a dataset
(rename, permission change, publication request, `POST /datasets/{id}/event`, …), the
controller calls `ChangelogManager.logEvent(...)` directly, which writes the changelog row
**and** publishes to the integration-events SNS topic in the same call.

**Path B — async jobs from other services (e.g. Upload Service).** Services that do work
outside the API enqueue a **changelog job** onto the shared **jobs SQS queue**; a downstream
job worker (Jobs Service / pennsieve-api) consumes it and runs it through `ChangelogManager`
— which is what then publishes to SNS. The Upload Service is the canonical example:

- **upload-service-v2** (Go; lambdas + a Fargate move task) does **not** publish to the
  integration-events SNS topic and has zero coupling to it.
- On file import (`lambda/upload/handler/store.go:333-363`) it builds a
  `changelog.Event{EventType: CreatePackage, …}` and calls `EmitEvents`, which does an
  **SQS `SendMessage` to the shared jobs queue** (`JOBS_QUEUE_ID`), not an SNS publish
  (`pennsieve-go-core/pkg/changelog/client.go:19-38`).
- Literal event type emitted: **`CREATE_PACKAGE`**
  (`pennsieve-go-core/pkg/changelog/models.go`). Message envelope key
  `DatasetChangelogEventJob` → `{organizationId, datasetId, userId, events[], traceId, id}`,
  where each event has `eventType`, `eventDetail` (`PackageCreateEvent{id,name,nodeId,parent}`),
  `timestamp`. **Note: no `eventCategory` at this layer** — the category is assigned later by
  `ChangelogManager.eventCategory` when it publishes to SNS (`CREATE_PACKAGE` → `PACKAGES` →
  category `FILES`).

**Implication for "files uploaded" webhooks:** to receive upload events, subscribe to the
**`FILES`** category. The chain is:

```
upload-service-v2  ──(CREATE_PACKAGE job, SQS jobs queue)──▶  Jobs Service / ChangelogManager
   ──(SNS integration-events, eventCategory=FILES, eventType=CREATE_PACKAGE)──▶  integration-service  ──▶  your webhook
```

Two *other* SNS topics owned by upload-service-v2 are **unrelated** to webhooks and listed
only to avoid confusion:
- `{env}-upload-service-v2-imported-file-*` — internal trigger for the Fargate object-move.
- `{env}-upload-service-v2-file-finalized-*` — `eventType="FileFinalized"` fan-out consumed
  by scan-service (HIPAA compliance-tier filtering) and future metadata/AI services.

> The `datasetId` in delivered webhook payloads is the **integer** dataset id (from the
> changelog job / SNS envelope), whereas the enable-on-dataset API routes (§5) use the
> dataset **node id** (`N:dataset:...`). Receivers correlating the two need to map between
> them.

### 11.1 Producer inventory (as of this writing)

All Go producers use the same mechanism: build a `changelog.Event`, call
`changelog.Client.EmitEvents`, which does `sqs.SendMessage` to the shared jobs queue
(`JOBS_QUEUE_ID`); pennsieve-go-core `pkg/changelog/client.go`.

| Producer | Emits today | Event type(s) | Where |
|---|---|---|---|
| **pennsieve-api** (Scala) | ✅ synchronous | all in-process dataset actions (§6) | `ChangelogManager.logEvent`; publishes to SNS directly |
| **upload-service-v2** (Go) | ✅ via jobs queue | `CREATE_PACKAGE` | `lambda/upload/handler/store.go:333-363` |
| **packages-service** (Go) | ✅ via jobs queue | `RESTORE_PACKAGE` | restore lambda; `api/store/restore/changelog.go:63`, emitted from `lambda/restore/handler/handler.go:125` (best-effort — logs a warning on failure, does not fail the restore) |
| **datasets-service** (Go) | ❌ none | — | pure read-only service; no `changelog` import, no SNS/SQS emit, no state mutation. Dormant SNS scaffold exists only for a (commented-out) manifest-worker trigger, which is not a changelog event. |

> The `eventDetail` shapes differ per event type: `PackageCreateEvent{id,name,nodeId,parent}`
> (upload), `PackageRestoreEvent{id,name,originalName,nodeId,parent}` (restore). Recall from §8
> that integration-service currently **drops** `eventDetail` before delivery, so receivers get
> only `eventType`/`eventCategory`/`datasetId`/`organizationId` regardless.

### 11.2 Future producers (roadmap — next 3–6 months)

This capability is expected to expand significantly. Anticipated additions (candidates
identified during this investigation, not yet implemented):

- **packages-service — delete path:** a `DELETE_PACKAGE` event (no delete handler emits today;
  `changelog.DeletePackage` already exists in go-core but is unused here).
- **packages-service — viewer assets:** events for viewer-asset create/update/delete (see the
  separate viewer-assets/highlights work); would likely need new `ChangelogEventName` values.
- **datasets-service:** currently emits nothing; a natural future producer if/when it gains
  mutating operations.
- **Other services** adopting the same `pkg/changelog` → jobs-queue pattern.

Adding a producer is uniform: import `pkg/changelog`, set `JOBS_QUEUE_ID`, grant
`sqs:SendMessage` to the jobs queue, and `EmitEvents`. New *event types* additionally require:
(a) a `ChangelogEventName` enum value + category mapping in pennsieve-api
(`ChangelogEventName.scala`, `ChangelogManager.eventCategory`), and (b) — only if a **new
category** is introduced — a row in the `webhook_event_types` seed
(`pennsieve-db-migrations`) so it becomes subscribable. Reusing an existing category (e.g. a
new `FILES` event) needs no migration.
