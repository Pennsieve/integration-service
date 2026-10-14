-- Replaces every topic with the set the workspace Settings / Notifications
-- page offers. Existing subscriptions and topics are deleted outright, which
-- also cascades to the notifications posted to them (and to the messages,
-- user_notifications and notification_audit rows hanging off those). No
-- environment held data worth keeping when this was written.
DELETE FROM subscriptions;
DELETE FROM topics;

-- Restart topic ids so every environment numbers the seeded topics 1-13 in
-- the order below, the same ids the subscribe route takes as {topicId}.
SELECT setval(pg_get_serial_sequence('topics', 'topic_id'), 1, false);

-- Each topic is named after the eventType it notifies on, and its context is
-- the JSON Schema a subscription's context must satisfy. additionalProperties
-- is false throughout, so a subscription can't carry a field that looks like
-- a filter but isn't matched on.
--
--   workspace: every event in the organization. The events carry no datasetId
--              or subjectUserId to filter on.
--   personal:  events about one user (subjectUserId), who must be the
--              subscriber: the notifications service rejects anyone else's id.
--   dataset:   dataset-scoped changelog events, for one dataset.
WITH scope (kind, context) AS (
    VALUES
        ('workspace', '{
            "$schema": "http://json-schema.org",
            "title": "WorkspaceTopicConfiguration",
            "type": "object",
            "properties": {
                "organizationId": {"type": "integer"}
            },
            "required": ["organizationId"],
            "additionalProperties": false
        }'::jsonb),
        ('personal', '{
            "$schema": "http://json-schema.org",
            "title": "PersonalTopicConfiguration",
            "type": "object",
            "properties": {
                "organizationId": {"type": "integer"},
                "subjectUserId": {"type": "integer"}
            },
            "required": ["organizationId", "subjectUserId"],
            "additionalProperties": false
        }'::jsonb),
        ('dataset', '{
            "$schema": "http://json-schema.org",
            "title": "DatasetTopicConfiguration",
            "type": "object",
            "properties": {
                "organizationId": {"type": "integer"},
                "datasetId": {"type": "integer"}
            },
            "required": ["organizationId", "datasetId"],
            "additionalProperties": false
        }'::jsonb)
)
INSERT INTO topics (name, description, context)
SELECT t.name, t.description, scope.context
FROM (
    VALUES
        -- Workspace-wide publishing broadcasts (eventCategory PUBLISHING),
        -- one per successful publishing-workflow outcome.
        (1,  'DATASET_PUBLISHED',             'Dataset Published in Workspace',                 'workspace'),
        (2,  'DATASET_EMBARGOED',             'Dataset Embargoed in Workspace',                 'workspace'),
        (3,  'DATASET_EMBARGO_RELEASED',      'Dataset Embargo Released in Workspace',          'workspace'),
        (4,  'DATASET_UNPUBLISHED',           'Dataset Unpublished in Workspace',               'workspace'),
        (5,  'DATASET_REVISED',               'Dataset Revised in Workspace',                   'workspace')
) AS t (position, name, description, kind)
JOIN scope USING (kind)
ORDER BY t.position;
