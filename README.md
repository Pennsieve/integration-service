# integration-service
- Infrastructure for webhook integration notification system.
- Infrastructure for external application invocation

## Webhook Workflow

1. API sends events to the ChangelogManager
2. ChangelogManager puts events on SNS
3. SQS subscribes to SNS and triggers Even_Lambda
4. EventLambda checks with postgres which events should be routed to which API endpoints


## Integration Service Workflow
1. Service lambda receives payload from frontend
2. Payload is validated and authorization to invoke workflow determined
3. Payload attributes persisted 
4. Workflow initiated