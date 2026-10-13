-- Removes the topics seeded by the up migration, with their subscriptions
-- and notifications. The topics and subscriptions the up migration deleted
-- can't be restored.
DELETE FROM topics
WHERE name IN (
    'DATASET_PUBLISHED',
    'DATASET_EMBARGOED',
    'DATASET_EMBARGO_RELEASED',
    'DATASET_UNPUBLISHED',
    'DATASET_REVISED',
    'ADDED_TO_DATASET',
    'UPDATE_ORGANIZATION_PERMISSION',
    'ADD_TEAM_USER',
    'DATASET_DELETED',
    'REMOVED_FROM_DATASET',
    'REMOVE_TEAM_USER',
    'CREATE_PACKAGE',
    'DELETE_PACKAGE'
);
