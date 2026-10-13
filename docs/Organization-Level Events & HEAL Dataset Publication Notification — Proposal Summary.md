# Organization-Level Events & HEAL Dataset Publication Notification

Oct 2, 2026 · @Yi Chen · Revised Oct 13, 2026: `DATASET_PUBLISHED_IN_WORKSPACE` renamed to `DATASET_PUBLISHED` and split into one event per publishing outcome, to match the changelog-events spec and integration-service#148.

## Problem & Background

Pennsieve's existing `ChangelogEvent` system covers dataset-scoped actions only — its `dataset_id` column is non-nullable and every consumer assumes exactly one dataset per event. Organization- and team-level administrative actions (adding or removing a user from a workspace, changing a user's role, creating/renaming/deleting a team, adding or removing a user from a team) produce no durable, queryable signal of any kind. Several — removing a user from an org, removing a user from a team — don't even send an email today.

This gap was surfaced while scoping the event vocabulary for the User Notifications feature. A HEAL milestone additionally requires that Pennsieve send an email notification when any dataset in a workspace is published — an organization-scoped event the dataset-changelog system cannot express.

## Architecture Decision

Extending `ChangelogEvent` with a nullable `dataset_id` is not viable. Every consumer — changelog timeline UI, webhook delivery — assumes each event belongs to exactly one dataset; retrofitting a `WHERE dataset_id IS NULL` special case would ripple through all of them. Organization events are a different shape (org-wide, not dataset-scoped) queried by a different audience (org admins, not dataset collaborators).

**Decision: build a parallel capability.** New tables, new model types, and a new manager — mirroring the dataset-changelog pattern exactly but without any coupling to it. Existing dataset-changelog code, tests, and consumers are unaffected.

## Schema

Two new tables per organization schema, mirroring the dataset-changelog pair:

- **`organization_event_types`** `(id, name, created_at)` — lookup table for `OrganizationEventName`, populated lazily via `getOrCreate`.
- **`organization_events`** `(id, organization_id, actor_user_id, subject_user_id, event_type_id, detail, created_at)` — the event log.

The key difference from `changelog_events`: two user references instead of one. `actor_user_id` is who performed the action (an admin); `subject_user_id` is who it was performed on (nullable — not every org event has a single affected user, e.g. team creation). Dataset events only need one `user_id` because the dataset is the subject; in org events the subject is a person, and actor and subject are frequently different individuals who may each want to be notified for different reasons.

## Event Vocabulary

All actions confirmed silent today, traced against actual manager/controller code:

| Event | Fires from | Subject |
| --- | --- | --- |
| `ADD_ORGANIZATION_USER` | `OrganizationManager.addUser` / `inviteMember` | Added user |
| `REMOVE_ORGANIZATION_USER` | `SecureOrganizationManager.removeUser` | Removed user |
| `UPDATE_ORGANIZATION_PERMISSION` | `SecureOrganizationManager.updateUserPermission` | Affected user |
| `ADD_TEAM_USER` | `TeamManager.addUser` | Added user |
| `REMOVE_TEAM_USER` | `TeamManager.removeUser` | Removed user |
| `CREATE_TEAM` | `TeamManager.create` | — |
| `UPDATE_TEAM` | `TeamManager.update` | — |
| `DELETE_TEAM` | `TeamManager.delete` | — |
| `ADDED_TO_DATASET` | `DataSetsController.addUserCollaborator` | Added user |
| `REMOVED_FROM_DATASET` | `DataSetsController.deleteUserCollaborator` | Removed user |
| `DATASET_OWNERSHIP_TRANSFERRED` | `DataSetsController.switchOwner` | New owner |
| `DATASET_CREATED` | `POST /datasets` | — (Publishers-gated) |
| `DATASET_DELETED` | Dataset deletion (call site to be identified) | — |
| **`DATASET_PUBLISHED`** | `PUT /:id/publication/complete`, `(Completed, Publication)` | — |
| `DATASET_EMBARGOED` | same callback, `(Completed, Embargo)` | — |
| `DATASET_EMBARGO_RELEASED` | same callback, `(Completed, Release)` | — |
| `DATASET_UNPUBLISHED` | removal-accept call site | — |
| `DATASET_REVISED` | revision-accept call site | — |

`ADDED_TO_DATASET` through `DATASET_REVISED` are organization-scoped events describing dataset-level actions. They live in `organization_events` (keyed by `organization_id`); their detail payloads reference dataset fields, but no `dataset_id` column is added to the table.

## HEAL Requirement: `DATASET_PUBLISHED`

### Event definition

**Name:** `DATASET_PUBLISHED` in `OrganizationEventName`, with `eventCategory: "PUBLISHING"`. An earlier draft called it `DATASET_PUBLISHED_IN_WORKSPACE`. The suffix was dropped because workspace scope is already implied by the event being organization-level (subscriptions to it can't carry a `datasetId`), so the name only needs to say what happened.

**One event per publishing outcome.** `DATASET_PUBLISHED` is one of five workspace-wide broadcasts, each mirroring one dataset-scoped success event. No outcome reuses another's event, so subscribers can pick exactly the outcomes they care about:

| Event | Mirrors | Fires from |
| --- | --- | --- |
| `DATASET_PUBLISHED` | `COMPLETE_PUBLICATION` | `PUT /:id/publication/complete`, `(Completed, Publication)` |
| `DATASET_EMBARGOED` | `COMPLETE_EMBARGO` | same callback, `(Completed, Embargo)` |
| `DATASET_EMBARGO_RELEASED` | `RELEASE_EMBARGO` | same callback, `(Completed, Release)` |
| `DATASET_UNPUBLISHED` | `COMPLETE_REMOVAL` | removal-accept call site |
| `DATASET_REVISED` | `COMPLETE_REVISION` | revision-accept call site |

There are no failure siblings: a failed publish concerns the owner and the publisher, who already get the dataset-scoped `FAIL_*` events.

**Detail payload** (all five share these fields, one case class per event):

```scala
case class DatasetPublished(      // DatasetEmbargoed / DatasetEmbargoReleased / DatasetUnpublished / DatasetRevised: same shape
  datasetId:        Int,
  datasetNodeId:    String,
  datasetName:      String,
  publishedVersion: Int,
  doi:              String,
  ownerUserId:      Int,
  ownerName:        String   // denormalized — avoids lookup at render time
) extends OrganizationEventDetail {
  val eventType = DATASET_PUBLISHED
}
```

All fields are present to allow notification rendering without a follow-up API call. `ownerName` is denormalized for the same reason as `datasetName`. No `organizationId` on the detail — it is already on the `organization_events` row and the SNS envelope.

### Where and when

**Call site:** `DataSetsController.scala`, at the same point the `COMPLETE_PUBLICATION` `ChangelogEvent` is logged — **not** at `ACCEPT_PUBLICATION`.

- `ACCEPT_PUBLICATION` = a publisher approved the request; the dataset is not yet publicly visible on Discover.
- `COMPLETE_PUBLICATION` = Discover has confirmed the dataset is live. This is the moment the HEAL requirement is satisfied and the notification is meaningful.

```scala
// Existing call — unchanged
changelogManager.logEvent(dataset, CompletePublication(...))

// New call — alongside it
organizationEventManager.logEvent(
  subjectUserId = None,
  detail = DatasetPublished(
    datasetId        = dataset.id,
    datasetNodeId    = dataset.nodeId,
    datasetName      = dataset.name,
    publishedVersion = publishedVersion,
    doi              = doi,
    ownerUserId      = owner.id,
    ownerName        = s"${owner.firstName} ${owner.lastName}"
  )
)
```

### Subscription gate

No special gate required to subscribe. Publication is already a public act; the dataset is visible on Discover the moment this event fires. Any workspace member who opts in is eligible. This differs from `DATASET_CREATED`, which is gated on Publishers-team membership.

### Revisions (resolved)

An earlier draft reused this one event for revisions and asked whether to guard on `publishedVersion == 1`. Neither is needed now:

- Revisions are their own workflow (`REQUEST_REVISION`/`ACCEPT_REVISION`) and get their own event, `DATASET_REVISED`.
- A later version published through the ordinary publication flow is still `DATASET_PUBLISHED`. That path has no code-level difference between a first publish and a new version.

## Wire Format

Reuses the existing `{env}-integration-events-sns-topic` → SQS → integration-service pipeline. No second topic, queue, or IAM wiring needed.

```json
{
  "organizationId": "<int>",
  "actorUserId": "<int>",
  "subjectUserId": "<int, nullable>",
  "eventCategory": "<ORGANIZATION or PUBLISHING>",
  "eventType": "<e.g. ADD_TEAM_USER or DATASET_PUBLISHED>",
  "eventDetail": { }
}
```

`eventCategory` is `PUBLISHING` for the five publishing broadcasts and `ORGANIZATION` for every other organization event. The broadcasts share `PUBLISHING` with the dataset-scoped publishing events but are told apart by event type. Events about one user must carry `subjectUserId` on the envelope, not only in `eventDetail`: integration-service matches personal subscriptions on it.

Required fields are the true lowest common denominator (`eventCategory`, `eventType`, `eventDetail`). `organizationId`, `actorUserId`, and `subjectUserId` are category-specific optional fields — not universal — so the topic can serve non-org, non-dataset emitters (e.g. github-service releasing to Discover) without forcing every message to carry fields that have no meaning for them.

**Pre-implementation audit required:** integration-service's `event_parser`/`EventMessage` currently assumes every SNS message on this topic carries a `datasetId`. Organization events carry no `datasetId`. This assumption must be found and removed before the first organization event can flow through without error. (Relevant: the `datasetId`-as-quoted-string parsing bug fixed in integration-service#138 was in this same path.) *Addressed in integration-service#148, which also decodes `subjectUserId` and `eventDetail`.*

## Open Questions

1. **Publishers gate check (cross-service call vs. denormalized flag)** — for `DATASET_CREATED` and workspace-wide `DATASET_OWNERSHIP_TRANSFERRED` subscriptions, integration-service must verify Publishers-team membership at subscription-creation time. The leading option is a synchronous call to a new/extended pennsieve-api endpoint. The alternative — pennsieve-api pushes a denormalized `isPublisher` flag that integration-service caches locally — avoids the cross-service call but introduces staleness risk. **Must be decided before implementation begins.**
2. **`ADD_ORGANIZATION_USER` double-fire risk** — the event is listed as firing from both `OrganizationManager.addUser` and `inviteMember`. `inviteMember` already sends an `addedToOrganization` email. Confirm these code paths are mutually exclusive or add a guard to prevent two `ADD_ORGANIZATION_USER` events from a single invite flow.
3. ~~**First publication only vs. all publications**~~ — *Resolved: revisions get their own `DATASET_REVISED` event (see "Revisions" above), so `DATASET_PUBLISHED` needs no version guard.*
4. **`OrganizationEventDetail` field shapes** — the case classes above are a first pass. Each must be validated against what the relevant manager/controller method actually has in scope at its call site (e.g. does `removeUser` have the user's prior permission level available to include in the detail?).
5. **Category system** — *Partly resolved: organization events now use two wire categories, `PUBLISHING` for the five publishing broadcasts and `ORGANIZATION` for everything else.* Category-level subscriptions (one setting covering every topic in a category) are still deferred.
6. **Generalizing the workspace-wide visibility gate** — the Publishers-team-membership gate will likely recur for other events (e.g. `DBPermission.Administer`-gated visibility into `REMOVE_ORGANIZATION_USER`). Defer generalization until the second real case arrives; ship Publishers as a one-off.

## Out of Scope

The following are explicitly deferred and do not block this design:

- **Team-grant and org-wide-share dataset notifications** — `ADDED_TO_DATASET`/`REMOVED_FROM_DATASET` cover only direct-to-user grants. Notifying every member of a team when team access changes is a fan-out problem, treated as a separate later design.
- **Retroactive migration** — no backfill of historical org/team membership changes. Like dataset changelog events, coverage begins from the ship date only; the data to reconstruct prior history does not exist.
- **Missing email templates** (`removedFromOrganization`, `removedFromTeam`, permission-changed notice) — adding `logEvent` calls is kept strictly to the event-emission layer. Email template work is deferred to when User Notifications can serve as the delivery mechanism, rather than adding new bespoke templates per action now.
- **Organization settings changes** — name, color theme, subscription/billing status, feature flags, custom ToS versions. Lower-value notification targets with no urgency.
- **Generalized workspace-wide visibility gate mechanism** — deferred until a second concrete case beyond Publishers materializes.
