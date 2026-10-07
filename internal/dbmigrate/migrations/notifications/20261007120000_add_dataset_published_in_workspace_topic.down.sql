-- Deleting the topic cascades to its subscriptions and every notification
-- posted to them. To keep that history, disable the topic instead
-- (UPDATE topics SET enabled = false WHERE name = 'DATASET_PUBLISHED_IN_WORKSPACE').
DELETE FROM topics WHERE name = 'DATASET_PUBLISHED_IN_WORKSPACE';
