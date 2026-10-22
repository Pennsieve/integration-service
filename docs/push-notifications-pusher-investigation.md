# Push Notifications via Pusher — Investigation

Investigated 2026-10-19. Sources: this repo at `2b1f204`, [pennsieve-go-core](https://github.com/Pennsieve/pennsieve-go-core) at `33a287c`, and [upload-service-v2](https://github.com/Pennsieve/upload-service-v2) at `3ce7199`.

Goal: deliver push notifications to a per-user channel named `user-${intId}-notifications`.

## Summary

- **integration-service doesn't send push notifications today.** It stores the `push` channel flag, but nothing delivers on it.
- **pennsieve-go-core has no Pusher client wrapper.** It has only a config model and an interface. Each service builds its own client with `github.com/pusher/pusher-http-go/v5`.
- **The platform is moving off Pusher to AWS AppSync Events** through go-core's `pkg/realtime`. That package has no per-user channel yet.

## integration-service today

- Subscriptions take `notification_channels: {email, push, sms}`, and users have `pushEnabled` preferences (`internal/models/notification.go`).
- Only email has a sender. `internal/notification_generator/generator.go` says that "push and SMS have no sender yet". Push deliveries are only counted in a log line in `logDeliveries`.
- There's no Pusher dependency in `go.mod`. It already depends on `pennsieve-go-core v1.21.0`, which includes `pkg/realtime`.

## What pennsieve-go-core provides for Pusher

| File | Contents |
|---|---|
| `pkg/models/pusher/models.go` | `Config{AppId, Key, Secret, Cluster}` (JSON: `app_id`, `key`, `secret`, `cluster`), plus the upload-specific `UploadMessageItem` |
| `pkg/domain/pusher.go` | `PusherAPI` interface: `Trigger(channel, eventName, data)` and `TriggerBatch(batch)`, used as a seam for mocking in tests |

There's no client constructor, no config loader and no channel-name helper.

## How upload-service-v2 sends Pusher events

1. **Config** (`lambda/upload/handler/handler.go`, `initPusher`): it reads the SSM parameter `/ops/pusher-config` with `WithDecryption: true` and unmarshals it into `pusher.Config`. Then it builds the client:

   ```go
   PusherClient = &pusher.Client{
       AppID:   PusherConfig.AppId,
       Key:     PusherConfig.Key,
       Secret:  PusherConfig.Secret,
       Cluster: PusherConfig.Cluster,
       Secure:  true,
   }
   ```

   If the parameter can't be read, it logs a warning and leaves the client nil, so notifications are just turned off.

2. **Send** (`lambda/upload/handler/notify.go`, `notifyUpload`):

   ```go
   chName := strings.ReplaceAll(datasetNodeId, "N:dataset:", "dataset-")
   if err := s.pusherClient.Trigger(chName, "upload-event", rows); err != nil {
       log.Warn(err)
   }
   ```

   Failures are logged and never fail the upload.

3. **Switching to AppSync:** if `REALTIME_EVENTS_ENDPOINT` is set, it builds `realtime.FromEnv(ctx)` and calls `PublishBatch` on `realtime.Dataset(id)`, skipping Pusher entirely. Batches are split at 500 rows per event to stay under AppSync's 240 KB delivered-message limit.

## Pusher → AppSync migration (go-core `docs/realtime-appsync-design.md`)

- **Why it's moving:**
  - Pusher doesn't sign BAAs (HIPAA business associate agreements).
  - Its channels are public, with no subscription authorization.
  - Payloads carry file names.
  - Pusher is already off in clin.
- **`pkg/realtime` API:**
  - `Publisher` has `Publish` and `PublishBatch`.
  - Channel helpers are `Dataset`, `Run`, `RunScope` and `Application`.
  - `FromEnv` returns an AppSync publisher (SigV4-signed with the Lambda role, which needs `appsync:EventPublish`) when `REALTIME_EVENTS_ENDPOINT` is set. Otherwise it returns `Noop`.
- **Each environment switches over completely** (clin → dev → prod) rather than running both. There's deliberately no Pusher adapter, and `domain.PusherAPI` will be removed once publishers have moved.

## Things to know for `user-${intId}-notifications`

- **`pkg/realtime` has no user namespace.** `Channel.Validate()` only accepts `datasets`, `runs` and `applications`. A per-user channel needs these changes:
  - A new namespace and helper in go-core, for example `/users/<id>`.
  - Authorizer logic in pennsieve-go-api's events authorizer that only lets a user subscribe to their own channel.
  - The namespace added to the Event API Terraform.
- **A public Pusher channel leaks data.** `user-<intId>-notifications` uses a guessable integer ID. Anyone with the public app key could subscribe and read another user's notifications. If Pusher is used in the meantime, use a `private-` channel with an auth endpoint. `private-encrypted-` is Pusher's only HIPAA-acceptable option.

## Recommendation

1. Add a user channel to go-core `pkg/realtime`, with authorizer and Terraform support.
2. In integration-service, build the publisher with `realtime.FromEnv` at cold start. In the generator, add a `sendPush` next to `sendEmails` for deliveries with `Channels.Push`. Log failures and keep going, as upload-service-v2 does.
3. Only if an environment still needs Pusher: build a client from `/ops/pusher-config` behind `domain.PusherAPI`, as `initPusher` does, and send on a private channel.
