import os
import logging
import boto3

logger = logging.getLogger()
logger.setLevel(logging.INFO)
sqs_resource = boto3.resource('sqs')
queue = sqs_resource.get_queue_by_name(QueueName=os.environ.get("WEBHOOK_SQS_QUEUE_NAME"))
sqs = boto3.client('sqs')

def lambda_handler(event, _context):
    logger.info('## PLATFORM EVENT HANDLER LAMBDA EVENT')

    for record in event['Records']:
        # Send message to SQS queue
        response = sqs.send_message(
            QueueUrl=queue.url,
            DelaySeconds=1,
            MessageAttributes={},
            MessageBody=record['body']
        )

        logger.info(response['MessageId'])

    return event
