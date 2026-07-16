# Test Plan: Pennsieve `integration-service`

Repo: https://github.com/Pennsieve/integration-service (Go 55% / Terraform 39%)

## 1. System Under Test

Webhook integration notification system with three deployables and one shared Postgres schema:

| Component | Entry point | Trigger | Purpose |
|---|---|---|---|
| Event consumer Lambda | `cmd/event` → `handler.Handler` | SQS (subscribed to SNS, batch size 50) | Consumes platform events, looks up subscribed webhooks in Postgres (cached), sends outbound webhook calls |
| Webhook receiver Lambda | `cmd/webhook` → `handler.WebhookHandler` | API Gateway HTTP API, public, no API Gateway auth | Accepts inbound webhook POST/PUT/PATCH/DELETE, validates a shared secret + rate limit, persists payload to `webhooks.messages` |
| DB migrator | `cmd/dbmigrate` | Run manually/CI on deploy | Applies `internal/dbmigrate/migrations/*.sql` to Postgres |

Data flow: API → ChangelogManager → SNS topic → SQS queue (DLQ after 3 receives) → event Lambda → `event_parser.MapEvents` → `webhook_mapper.MapWebhookMessages` (org-scoped cache, 10 min TTL, backed by `cache.RefreshWebhookCache` querying a per-org Postgres schema) → `webhook_sender.BroadcastMessages` (dedup by URL, 3 retries with jittered backoff, 250ms connect timeout).

Supporting packages: `internal/db` (pool, `Query`, `InsertWebhookMessage`, `RecordSenderRequest`), `internal/cache` (in-memory map + RWMutex), `internal/aws` (SSM param fetch), `internal/utils` (Slack vs. generic JSON body shaping), `internal/models` (shared structs).

## 2. Objectives

- Confirm both Lambda entry points behave correctly under valid, malformed, and adversarial input.
- Confirm the event → cache → webhook delivery pipeline routes messages to the correct subscriber URLs and only those URLs.
- Confirm the public webhook receiver's auth, rate-limiting, and payload-size controls cannot be bypassed.
- Confirm DB migrations apply/roll back cleanly and the cache's join query stays compatible with the schema it depends on.
- Close the coverage gaps identified below before relying on this plan as a release gate.

## 3. Scope

**In scope:** all Go packages under `internal/` and `cmd/`, the two Lambda handlers, DB migrations, and the Terraform-defined infra insofar as it affects testability (IAM scope, concurrency caps, DLQ/alarm wiring).

**Out of scope:** the upstream services that publish to SNS (ChangelogManager) and whatever service owns the `webhooks` / `webhook_event_subscriptions` / `dataset_integrations` / `webhook_event_types` tables (not defined in this repo's migrations — treated as an external dependency/contract).

## 4. Current Automated Coverage (baseline)

Existing `_test.go` files are solid at the unit level:

| Package | File | Covers |
|---|---|---|
| `handler` | `webhook_handler_test.go` (368 lines) | method allow-list, shared-secret auth (missing/wrong/case-insensitive), payload size cap, base64 decode + invalid base64, invalid JSON, empty body, rate limit exceeded, DB insert failure, success across all 4 allowed methods, UUID format/uniqueness, response helpers |
| `event_parser` | `event_parser_test.go` | grouping by org, `CREATE_DATASET` force-refresh flag, malformed envelope rejection |
| `webhook_mapper` | `webhook_mapper_test.go` | lookup key construction, cache-hit matching, no-match-yields-empty-URLs |
| `webhook_sender` | `webhook_sender_test.go` | success/no-retry, non-2xx exhausts retries, transport error exhausts retries |
| `utils` | `utils_test.go` | default JSON body vs. Slack-wrapped body |
| `cache` | `cache_test.go` | get/set round-trip, org-ID SQL-injection-pattern rejection, valid org-ID pattern |

**No test files exist for:** `internal/handler/handler.go` (the event-Lambda entrypoint), `internal/db` (directly — only exercised indirectly through `sqlmock` in handler tests), `internal/aws`, `internal/dbmigrate`, or `cmd/*`.

## 5. Test Levels

### 5.1 Unit tests (extend existing suite)

New cases to add, by package:

**`internal/handler` (`handler.go`, currently untested)**
- End-to-end `Handler()` call with a fake `db.SetPoolForTest` pool and a pre-seeded `cache.Set` entry, verifying `webhook_sender.BroadcastMessages` actually hits an `httptest.Server` with the expected body.
- `event_parser.MapEvents` failure propagates as an error (bad `Records` shape) without panicking or partially mutating cache.
- `db.EnsureDB` failure short-circuits before touching the parser/mapper/sender.
- Multiple orgs in one batch route to their respective cached webhook sets independently.

**`event_parser`**
- Empty `Records` array (valid envelope, zero messages) → empty map, no error.
- `record.body.Message` present but not valid JSON for `EventMessage` (type mismatch, e.g. `datasetId` as a string) → error.
- Batch at the SQS `batch_size` boundary (50 records) for a basic throughput/regression check.
- Multiple `CREATE_DATASET` events plus other types in the same batch still sets `forceRefresh=true` exactly once semantically (idempotent).

**`cache`**
- TTL expiry: entry older than 10 minutes triggers `RefreshWebhookCache` even when `forceRefresh=false` (currently only tested via `webhook_mapper`'s happy path with a fresh timestamp; add the stale-timestamp branch explicitly in `cache` package tests, or as a `webhook_mapper` test with `Updated: time.Now().Add(-11*time.Minute)`).
- `RefreshWebhookCache` with a DB error (via `sqlmock` returning an error from `db.Query`) leaves the previous cache entry untouched and logs rather than panics.
- Concurrent `Get`/`Set`/`RefreshWebhookCache` under `go test -race` to validate the `RWMutex` usage.

**`webhook_mapper`**
- `forceRefresh=true` path actually calls `cache.RefreshWebhookCache` (needs a `db.SetPoolForTest` + `sqlmock` expectation) rather than only exercising the cache-hit shortcut.
- Same `datasetId:eventCategory` key across two orgs' events stays correctly scoped per org (no cross-org URL leakage) — the current key format (`fmt.Sprintf("%d:%s", ...)`) has no org component, so this should be verified explicitly.

**`webhook_sender`**
- `BroadcastMessages` de-dups identical URLs across multiple messages (`urlSet`) — assert the fake server receives exactly N calls, not N×duplicates.
- Backoff timing: attempt 2 sleep is in `[2s, 3s)`, attempt 3 in `[4s, 5s)` (jitter bounds), without asserting exact wall-clock (use short overrides or table-driven check on the formula, not real sleeps, to keep tests fast).
- Context cancellation mid-retry loop stops further attempts.
- Slack URL body shape is actually round-tripped through `BroadcastMessages` → `httptest.Server` (currently only unit-tested at the `utils` level, not through the sender).

**`internal/db`**
- Direct tests (not only via handler) for `InsertWebhookMessage` and `RecordSenderRequest` using `sqlmock`, asserting exact SQL text and argument binding.
- `RecordSenderRequest` window-boundary behavior: request at exactly `window_start + window` resets vs. one second before does not (needs either `sqlmock` row-level assertions or an integration test against real Postgres, since the CASE logic runs server-side).
- `Query()` scan-error path (wrong column count/type from a mocked row).

**`internal/aws`**
- `GetSSMParam` before `InitAWS` returns the "uninitialized SSM client" error.
- `InitAWS` failure (bad config) is cached and surfaces on subsequent `GetSSMParam` calls without retrying.

**`internal/dbmigrate`**
- `MigrationsSource()` embeds and lists both migration files.
- `ConfigDefaults()` returns expected defaults (schema name, etc.).

### 5.2 Integration tests (new)

Run against ephemeral, real dependencies rather than mocks:

- **Postgres via `testcontainers-go`:** apply `up` migrations, run `InsertWebhookMessage` / `RecordSenderRequest` against the real schema, then apply `down` and confirm clean teardown (schema + tables gone).
- **Cache join query against a real schema:** since `webhooks`, `webhook_event_subscriptions`, `dataset_integrations`, and `webhook_event_types` aren't created by this repo, hand-build a minimal fixture schema matching the expected columns/joins in `cache.webhookQuery` and confirm `RefreshWebhookCache` returns the right `WebhookRecord`s. This is the highest-value gap to close, since a column rename or type change on the owning side would otherwise only be caught in production.
- **LocalStack for SSM:** exercise `aws.GetSSMParam` / `db.initDB` / `ensureWebhookSecret` against LocalStack's SSM to catch parameter-name or decryption-flag mistakes without needing real AWS credentials.
- **Full pipeline, mocked network boundary only:** synthetic SNS→SQS event JSON → `handler.Handler` → real Postgres (testcontainers) → real HTTP calls to an `httptest` webhook receiver, verifying payload shape end-to-end.

### 5.3 Contract tests

- `EventMessage` JSON tags (`organizationId`, `datasetId`, `eventCategory`, `eventType`) against a sample real ChangelogManager/SNS payload, if one can be obtained, to catch drift between the producer and this consumer.
- `WebhookResponse` JSON shape (`request_id`, `received_at`, `code`, `message`) as the receiver's public contract — add a golden-file/snapshot test so accidental field renames are caught.

### 5.4 Security tests

- Shared-secret bypass attempts: empty header, header present but empty string, timing-safe comparison behavior (already uses `subtle.ConstantTimeCompare` — add a test asserting comparison time doesn't leak length via early return, at least at the code-review level since Go timing tests are inherently noisy).
- SQL injection via `orgID` (already covered) — extend with additional payloads (`org%s`, format-string-looking values, unicode homoglyphs).
- Oversized payload exactly at `maxPayloadBytes` boundary (`== 1MiB` accepted, `== 1MiB+1` rejected).
- Deeply nested/pathological JSON within the byte-size cap (JSON "billion laughs"-style nesting) — currently **no protection beyond byte count**; confirm `encoding/json` unmarshal cost/behavior is acceptable or add a depth guard.
- Non-UTF8 / invalid-Unicode bytes in the body.
- Confirm the API Gateway route truly has no auth (`authorization_type = "NONE"`) and that this is intentional/compensated for entirely by the shared secret — flag if the secret were ever to leak, there's no secondary control (e.g., IP allowlist, WAF).
- IAM policy review for both Lambda roles (`webhook_receiver_lambda_role`, event consumer role): confirm scoped to `parameter/{env}/{service}/*` for SSM, no wildcard resource beyond what's necessary, no overly broad `rds-db:connect`.

### 5.5 Rate limiting & concurrency tests

- Sequential requests from one IP: 61st request in the 60s window returns 429 (currently unit-tested with a mocked count only — add an integration test against real Postgres to validate the atomic upsert under concurrency).
- Concurrent requests from the same IP (goroutines hitting `RecordSenderRequest` against real Postgres) to confirm the single atomic UPSERT prevents race-past-the-limit.
- Window reset: a request just after the 60s window closes gets `request_count` reset to 1, not incremented.
- Lambda-level `reserved_concurrent_executions = 5` on the webhook receiver: load test (e.g., `k6`/`hey`) with >5 concurrent requests against a deployed dev endpoint to confirm throttling behavior degrades gracefully (429/5xx, not crashes) and doesn't starve the SQS-driven event consumer's DB connections.

### 5.6 Resilience / failure-mode tests

- Downstream webhook endpoint slow (>250ms to connect) → connect-timeout triggers retry, not a hang (validates the custom dialer-timeout-vs-client-timeout design called out in code comments).
- Downstream webhook endpoint returns 3xx/4xx/5xx → correct retry/no-retry semantics per status class (currently only 500 and connection-refused are tested; add 400, 403, 429, 301).
- SQS partial-batch failure: a malformed record among valid ones currently returns a hard error for the whole batch (`event_parser.MapEvents` returns on first error) — confirm this is intended (whole batch retried/DLQ'd after 3 receives) vs. the TODO comment's stated desire for per-record skip-and-log; write a test documenting current behavior either way so a future fix doesn't silently regress test expectations.
- DB unavailable at Lambda cold start (`EnsureDB` failure) for both handlers returns the correct error/500 without leaking connection strings/credentials in logs.
- SSM parameter fetch failure (webhook secret or DB creds) fails closed (no default/empty secret accepted).

### 5.7 Infrastructure (Terraform) tests

- `terraform validate` / `terraform plan` in CI for all `.tf` files.
- Static analysis (`tflint`, `checkov`, or `tfsec`) for the public-facing `gateway.tf` (no-auth route) and IAM policies.
- Confirm `aws_cloudwatch_metric_alarm` on the DLQ (`event_integration_dlq_cloudwatch_metric_alarm`) actually fires by seeding a message into the DLQ in a dev environment and checking VictorOps/Slack notification.
- Confirm `redrive_policy` (`maxReceiveCount = 3`) matches the assumption baked into the resilience tests above.
- Confirm reserved concurrency (5) and VPC/security-group config match what's assumed in the load tests.

### 5.8 CI / process gap

**Finding:** `Jenkinsfile` only runs `make publish` (build + package + deploy) on `main` — there is no `go test ./...`, `go vet`, or lint stage anywhere in CI. Recommend adding a test/vet/lint stage that gates the build step before any deploy job runs, plus `go test -race ./...` given the cache package's shared mutable state.

## 6. Test Environments

| Environment | Purpose | Tooling |
|---|---|---|
| Local/CI (no external deps) | Unit tests | `go test`, `testify`, `DATA-DOG/go-sqlmock`, `net/http/httptest` |
| Local/CI (ephemeral deps) | Integration tests | `testcontainers-go` (Postgres), LocalStack (SSM/SNS/SQS) |
| Dev AWS (`dev-vpc-use1`) | E2E, load, infra, alarm tests | Deployed via existing Terraform/Jenkins pipeline; `k6`/`hey` for load; manual `curl`/Postman smoke tests against the public webhook endpoint with a dev shared secret |

## 7. Entry / Exit Criteria

- **Entry:** feature branch builds (`make compile`, `go vet ./...`) cleanly.
- **Exit for merge:** all unit + new integration tests pass; `go test -race ./...` clean; no critical/high findings from the security test set above.
- **Exit for release:** dev-environment E2E and load tests pass; Terraform plan reviewed with no unintended IAM/network changes; DLQ alarm confirmed live.

## 8. Key Risks / Gaps Summary

1. No CI test gate at all today (`Jenkinsfile` skips straight to build/deploy).
2. Event-Lambda entrypoint (`handler.Handler`) has zero direct test coverage.
3. `internal/db` and `internal/aws` are only exercised indirectly through handler-level `sqlmock` tests, not directly.
4. `cache.webhookQuery`'s 4-table join is untested against any real or fixture schema — cross-repo schema drift risk.
5. DB migrations (`up`/`down`) have never been run in an automated test.
6. Payload validation stops at byte size; no defense/test against pathological JSON structure within that limit.
7. No automated end-to-end test connects SNS → SQS → event Lambda → outbound webhook delivery; coverage is unit-level and mocked throughout.
