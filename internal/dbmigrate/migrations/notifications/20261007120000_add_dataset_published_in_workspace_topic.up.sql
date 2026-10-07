-- DATASET_PUBLISHED_IN_WORKSPACE is an organization-level event: pennsieve-api
-- emits it (eventCategory ORGANIZATION) once Discover confirms a dataset is
-- live, alongside the dataset-scoped COMPLETE_PUBLICATION changelog event.
-- Subscriptions are scoped to an organization only, never a dataset:
-- additionalProperties is false so a datasetId can't be added to a
-- subscription's context, where it would look like a filter but match every
-- dataset in the organization anyway (the event's envelope carries no
-- datasetId to match on).
INSERT INTO topics (name, description, context)
VALUES (
    'DATASET_PUBLISHED_IN_WORKSPACE',
    'A dataset in the workspace was published to Pennsieve Discover',
    '{
        "$schema": "http://json-schema.org",
        "title": "DatasetPublishedInWorkspaceTopicConfiguration",
        "type": "object",
        "properties": {
            "organizationId": {"type": "integer"}
        },
        "required": ["organizationId"],
        "additionalProperties": false
    }'::jsonb
)
ON CONFLICT (name) DO NOTHING;
