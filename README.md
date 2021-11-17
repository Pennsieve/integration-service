# integration-service
Infrastructure for webhook integration notification system.

## Workflow

1. API sends events to the ChangelogManager
2. ChangelogManager puts events on SNS
3. SQS subscribes to SNS and triggers Even_Lambda
4. EventLambda checks with postgres which events should be routed to which API endpoints
5.
