# Organization-Level Events & HEAL Dataset Publication Notification

Oct 2, 2026 · @Yi Chen

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
| **`DATASET_PUBLISHED_IN_WORKSPACE`** | `DataSetsController` at `COMPLETE_PUBLICATION` | — |

The last three are organization-scoped events describing dataset-level actions. They live in `organization_events` (keyed by `organization_id`); their detail payloads reference dataset fields, but no `dataset_id` column is added to the table.

## HEAL Requirement: `DATASET_PUBLISHED_IN_WORKSPACE`

### Event definition

**Name:** `DATASET_PUBLISHED_IN_WORKSPACE` in `OrganizationEventName`.

**Detail payload:**

```scala
case class DatasetPublishedInWorkspace(
  datasetId:        Int,
  datasetNodeId:    String,
  datasetName:      String,
  publishedVersion: Int,
  doi:              String,
  ownerUserId:      Int,
  ownerName:        String   // denormalized — avoids lookup at render time
) extends OrganizationEventDetail {
  val eventType = DATASET_PUBLISHED_IN_WORKSPACE
}
```

All fields are present to allow notification rendering without a follow-up API call. `ownerName` is denormalized for the same reason as `datasetName`. No `organizationId` on the detail — it is already on the `organization_events` row and the SNS envelope.

### Where and when

**Call site:** `DataSetsController.scala`, at the same point `COMPLETE_PUBLICATION` `ChangelogEvent` is logged — **not** at `ACCEPT_PUBLICATION`.

- `ACCEPT_PUBLICATION` = a publisher approved the request; the dataset is not yet publicly visible on Discover.
- `COMPLETE_PUBLICATION` = Discover has confirmed the dataset is live. This is the moment the HEAL requirement is satisfied and the notification is meaningful.

```scala
// Existing call — unchanged
changelogManager.logEvent(dataset, CompletePublication(...))

// New call — alongside it
organizationEventManager.logEvent(
  subjectUserId = None,
  detail = DatasetPublishedInWorkspace(
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

### Open flag

`COMPLETE_PUBLICATION` fires for revisions as well as first publication. Confirm with the HEAL stakeholder whether the requirement means first publication only (`publishedVersion == 1`) or any publication including revisions. This affects both call-site logic and email copy.

## Wire Format

Reuses the existing `{env}-integration-events-sns-topic` → SQS → integration-service pipeline. No second topic, queue, or IAM wiring needed.

```json
{
  "organizationId": "<int>",
  "actorUserId": "<int>",
  "subjectUserId": "<int, nullable>",
  "eventCategory": "ORGANIZATION",
  "eventType": "<e.g. DATASET_PUBLISHED_IN_WORKSPACE>",
  "eventDetail": { }
}
```

Required fields are the true lowest common denominator (`eventCategory`, `eventType`, `eventDetail`). `organizationId`, `actorUserId`, and `subjectUserId` are category-specific optional fields — not universal — so the topic can serve non-org, non-dataset emitters (e.g. github-service releasing to Discover) without forcing every message to carry fields that have no meaning for them.

**Pre-implementation audit required:** integration-service's `event_parser`/`EventMessage` currently assumes every SNS message on this topic carries a `datasetId`. Organization events carry no `datasetId`. This assumption must be found and removed before the first organization event can flow through without error. (Relevant: the `datasetId`-as-quoted-string parsing bug fixed in integration-service#138 was in this same path.)

## Open Questions

1. **Publishers gate check (cross-service call vs. denormalized flag)** — for `DATASET_CREATED` and workspace-wide `DATASET_OWNERSHIP_TRANSFERRED` subscriptions, integration-service must verify Publishers-team membership at subscription-creation time. The leading option is a synchronous call to a new/extended pennsieve-api endpoint. The alternative — pennsieve-api pushes a denormalized `isPublisher` flag that integration-service caches locally — avoids the cross-service call but introduces staleness risk. **Must be decided before implementation begins.**
2. **`ADD_ORGANIZATION_USER` double-fire risk** — the event is listed as firing from both `OrganizationManager.addUser` and `inviteMember`. `inviteMember` already sends an `addedToOrganization` email. Confirm these code paths are mutually exclusive or add a guard to prevent two `ADD_ORGANIZATION_USER` events from a single invite flow.
3. **`DATASET_PUBLISHED_IN_WORKSPACE`: first publication only vs. all publications** — `COMPLETE_PUBLICATION` fires on revisions too. Confirm with the HEAL stakeholder whether revisions also trigger the notification. Answer determines whether the call site needs a `publishedVersion == 1` guard.
4. **`OrganizationEventDetail` field shapes** — the case classes above are a first pass. Each must be validated against what the relevant manager/controller method actually has in scope at its call site (e.g. does `removeUser` have the user's prior permission level available to include in the detail?).
5. **Category system** — whether `OrganizationEventName` carries a `category` field (as `ChangelogEventName` does) or is a flat enum. Recommendation: flat enum until a second category (`BILLING`, etc.) actually materializes.
6. **Generalizing the workspace-wide visibility gate** — the Publishers-team-membership gate will likely recur for other events (e.g. `DBPermission.Administer`-gated visibility into `REMOVE_ORGANIZATION_USER`). Defer generalization until the second real case arrives; ship Publishers as a one-off.

## Out of Scope

The following are explicitly deferred and do not block this design:

- **Team-grant and org-wide-share dataset notifications** — `ADDED_TO_DATASET`/`REMOVED_FROM_DATASET` cover only direct-to-user grants. Notifying every member of a team when team access changes is a fan-out problem, treated as a separate later design.
- **Retroactive migration** — no backfill of historical org/team membership changes. Like dataset changelog events, coverage begins from the ship date only; the data to reconstruct prior history does not exist.
- **Missing email templates** (`removedFromOrganization`, `removedFromTeam`, permission-changed notice) — adding `logEvent` calls is kept strictly to the event-emission layer. Email template work is deferred to when User Notifications can serve as the delivery mechanism, rather than adding new bespoke templates per action now.
- **Organization settings changes** — name, color theme, subscription/billing status, feature flags, custom ToS versions. Lower-value notification targets with no urgency.
- **Generalized workspace-wide visibility gate mechanism** — deferred until a second concrete case beyond Publishers materializes.
