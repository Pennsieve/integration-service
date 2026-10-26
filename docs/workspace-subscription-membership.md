# Workspace Subscriptions and Organization Membership

Oct 19, 2026

This note answers two questions about the organization-membership check on
workspace-wide subscriptions (e.g. `DATASET_PUBLISHED`):

1. Does the workspace authorizer change anything about existing enabled
   subscriptions, or does it only gate new subscribe calls?
2. Should matching/delivery do its own live membership check, or should a
   cleanup job disable subscriptions when a user is removed from an
   organization (triggered by `REMOVE_ORGANIZATION_USER`, once that event
   exists; see the Organization-Level Events proposal)?

## 1. Does the authorizer affect subscriptions that already exist?

No. It only checks new subscribe calls.

- The membership check, `validateOrganizationMembership` in
  `internal/handler/subscription_validation.go`, runs only from
  `handleSubscribe` (`POST /notification/topic/{topicId}/subscription`).
  Nothing else calls `db.IsOrganizationMember`.
- `db.MatchSubscriptions` (`internal/db/notification_db.go`) matches on
  `topic_id` and the context only. `planDeliveries`
  (`internal/notification_generator/generator.go`) filters on
  `topic.Enabled && s.Enabled`. Neither looks at membership, so a subscription
  created while the user was a member keeps matching after they leave.
- **The PATCH route had a hole (now closed).**
  `PATCH /notification/subscription/{subscriptionId}` calls
  `db.SetSubscriptionEnabled`, which used to check only that the caller owns
  the subscription, so a removed user could turn a disabled subscription back
  on. It now also requires current `organization_user` membership when
  enabling a subscription whose context has `organizationId` (403
  otherwise). Disabling still needs no membership.

## 2. A live check at delivery, or a cleanup job?

Recommendation: put the live check in matching, and treat a cleanup job as an
optional extra.

### Disabling isn't enough

`generate` records notifications against every matched subscription, enabled
or not (`db.CreateNotifications` doesn't filter). `GET /notification/messages`
returns notifications on disabled subscriptions too, by design. So if a
cleanup job sets `enabled = false`, the removed user still gets every new
workspace notification in their in-app feed; only the email stops.

A cleanup job would have to delete the subscription, or the read path would
have to filter. Either one cuts against the "disable, don't delete, keep
history" model.

### The live check is cheap here

`IsOrganizationMember` already reads `pennsieve.organization_user` from the
same database. The check is one more condition in the `MatchSubscriptions`
query, not a call to another service:

```sql
AND (NOT s.context ? 'organizationId'
     OR EXISTS (SELECT 1 FROM pennsieve.organization_user ou
                WHERE ou.organization_id = (s.context->>'organizationId')::int
                  AND ou.user_id = s.user_id))
```

This keeps rows and history and needs no new state. If the user rejoins, their
subscriptions start working again, the same way the derived
`t.enabled AND s.enabled` cascade works (see
`docs/notification-status-flags-scope.md`).

### A job driven by events can miss users

`REMOVE_ORGANIZATION_USER` doesn't exist yet. Coverage starts on the day it
ships, with no backfill, so anyone removed before then is never cleaned up. A
lost or failed SQS message has the same effect. The membership table is the
source of truth, so check it at the moment it matters.

## Suggested changes

1. Add the membership condition to `MatchSubscriptions`. Then a removed user
   gets neither in-app notifications nor email.
2. ~~Run the same membership check in `handleUpdateSubscription` when
   `enabled` is being set to `true`.~~ Done, in `db.SetSubscriptionEnabled`.
3. Later, and optional: once `REMOVE_ORGANIZATION_USER` exists, add a job that
   disables those subscriptions so the user's subscription list is accurate.
   Tidy, but not needed for correctness.

## Related gap: dataset-scoped subscriptions

`validateDatasetReference` only checks that the dataset exists, not that the
caller can see it. That probably needs the same kind of check before any
dataset-scoped topic carries private content.
