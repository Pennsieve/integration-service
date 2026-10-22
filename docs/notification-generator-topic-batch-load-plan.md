# Plan: load all topics once per batch in `notification_generator`

## Background

`notification_generator.generate()` looks up a topic by name lazily, once for
each distinct `event.Type` in a batch. It caches hits and misses in a map that
`Generate()` passes down. The caching works (`TestGenerate_LooksUpTopicOncePerBatch`,
`TestGenerate_MissingTopicIsNotAnError`).

Since the default-renderer change, `rendererFor` falls back to
`defaultRenderer`, so every event type can now trigger a topic lookup. The
topic set is small, static and bounded, so a per-type lazy lookup no longer
matches the problem. A single load per invocation does.

## What the code already has

- **`db.GetTopics(ctx)` already exists**
  ([notification_db.go:41](../internal/db/notification_db.go)). It returns every
  topic, disabled ones too, in one query, and the `GET /topics` handler already
  uses it. So we don't need a new `GetAllTopics`. `Generate` can build the map
  itself.
- The generator calls `db.GetTopicByName` in one place only: the cache-miss
  branch in `generate()`
  ([generator.go:147-160](../internal/notification_generator/generator.go)).
  Outside that, only its own DB tests use it.
- Keeping disabled topics in the map matters. Notifications get recorded even
  when a topic is disabled, and only sending is gated
  (`TestGenerate_RecordsEvenWhenTopicDisabled`, `planDeliveries`). So the load
  must not filter on `enabled`.

## Steps

### 1. Load the topics once at the start of `Generate`

- Call `db.GetTopics(ctx)` and build a `map[string]models.Topic` keyed by `Name`.
- If the load fails, log `ERROR` and return without processing the batch. This
  keeps the "never fails the batch" rule: today a DB error drops just that
  event, and if the topic load fails every event would hit the same error
  anyway. Webhooks still go out afterwards in `handler.go`.
- **Choice: lazy or eager.** `TestGenerate_SkipsWithoutQuerying` says a batch
  of only unrenderable events (wrong category, bad detail) makes no queries. A
  load at the very top breaks that. Two options:
  - *(Recommended)* Load lazily on the first event that renders. Use a small
    closure or `sync.OnceValues` inside `Generate`, so `generate()` gets a map
    that is already loaded and the test's meaning stays the same.
  - Load eagerly at the top and change the test. That's simpler, and it means
    an empty or all-bad batch costs one extra cheap query.
- Skip the load either way when `mapped` is empty.

### 2. Simplify `generate()`

- Change its parameter to `topics map[string]models.Topic`, values not pointers.
- Swap the cache-miss block and the empty
  `case errors.Is(err, db.ErrTopicNotFound):` branch for a plain lookup:

  ```go
  topic, ok := topics[event.Type]
  if !ok {
      return 0, nil
  }
  ```

- Update the doc comment: drop "topics caches topic lookups for the batch; a
  nil entry records a topic that doesn't exist".

### 3. Decide what happens to `GetTopicByName`

- Once the generator stops calling it, only its own tests use it. Either delete
  it along with `TestGetTopicByName` / `TestGetTopicByName_NotFound`, or keep it
  if something planned needs it (for example the Organization-Level Events
  work). I'd lean towards deleting it to keep the DB layer small.
- `ErrTopicNotFound` stays, because `GetTopic` uses it.

### 4. Update the generator tests

In [generator_test.go](../internal/notification_generator/generator_test.go):

- Make `expectTopic` (and the inline `WHERE name = $1` expectations) expect the
  `GetTopics` query, matched on `ORDER BY name` or `FROM notifications.topics`,
  returning a full row set.
- Rename `TestGenerate_LooksUpTopicOncePerBatch` to something like
  `TestGenerate_LoadsTopicsOncePerBatch`. Give it two distinct event types to
  show it still makes one query, which the old version couldn't show.
- `TestGenerate_MissingTopicIsNotAnError`: the load returns topics that don't
  include the event's type. Assert one query and no match query.
- `TestGenerate_DefaultRendererForTypeWithoutOne` and
  `TestGenerate_DatasetScopedPublicationUsesItsOwnTopic`: return the relevant
  topic in the load result.
- New: when the topic load fails, the batch is logged and dropped with no match
  or insert queries, and `Generate` doesn't panic.
- `TestGenerate_SkipsWithoutQuerying`: stays as it is if you take the lazy
  option, otherwise update it.

### 5. Verify

- Run unit tests and vet:

  ```bash
  go test ./internal/notification_generator/... ./internal/db/...
  go vet ./...
  ```

- In Dev, ahead of pennsieve-api#405 (the real `DATASET_PUBLISHED` producer):
  send synthesized ChangelogEvent-shaped messages to the event-consumer SQS
  queue. Cover:
  - one `DATASET_PUBLISHED` for an org that has a subscription
  - one type with a topic and no subscribers
  - one type with no topic
  - one malformed detail

  Then check that CloudWatch shows one topic query per invocation and the
  expected `Recorded N notification(s)`, `SKIPPED` and silent lines, and that
  rows land in `notifications.notifications`. Before we do that, it's worth
  pinning down the exact message envelope `event_parser` expects so the
  synthetic messages parse.

## Risks and notes

- **Stale topics:** this loads once per invocation, not once per container, so
  a topic added mid-flight shows up on the next batch. Don't hoist the map to a
  package-level variable. That would bring back staleness across warm
  invocations.
- **Size:** the topic set is small and bounded, a handful of rows including the
  `context` JSON, so the full load is cheaper than even two by-name lookups.
- **Behaviour change:** a batch whose only renderable events have no topic used
  to make one query per type and now makes exactly one.
