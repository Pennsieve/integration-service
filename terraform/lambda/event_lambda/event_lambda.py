import os
import logging
import boto3
import psycopg2
import psycopg2.extras
import urllib3
import json
from collections import defaultdict
#https://urllib3.readthedocs.io/en/latest/user-guide.html




http = urllib3.PoolManager()


def connect():
    logger = logging.getLogger()
    logger.setLevel(logging.INFO)

    dbname = ssm.get_parameter(Name='/dev/integration-service/integrations-postgres-db', WithDecryption=True)['Parameter']['Value']
    dbuser = ssm.get_parameter(Name='/dev/integration-service/integrations-postgres-user', WithDecryption=True)['Parameter']['Value']
    dbpass = ssm.get_parameter(Name='/dev/integration-service/integrations-postgres-password', WithDecryption=True)['Parameter']['Value']
    dbhost = ssm.get_parameter(Name='/dev/integration-service/integrations-postgres-host', WithDecryption=True)['Parameter']['Value']

    print(dbname+'\n'+dbhost+'\n')

    try:
        conn=psycopg2.connect("dbname='"+dbname+"' user='"+ dbuser+"' password='" + dbpass+"'" + "host='"+dbhost + "'")
    except psycopg2.errors.OperationalError as e:
        print("Connection error.")
        print(e)
        print(type(e))
        print(e.diag.severity)
        print(e.diag.message_primary)
        print(e.diag)
    return conn




def query_builder(organizationId, datasetId, event_category):
    return "(SELECT wh.api_url, wet.event_name, wi.dataset_id \
        FROM \""+organizationId+"\".webhooks AS wh \
        INNER JOIN \""+organizationId+"\".webhook_event_subscriptions as wes ON  wh.id=wes.webhook_id \
        INNER JOIN \""+organizationId+"\".dataset_integrations as wi ON  wh.id=wi.webhook_id \
        INNER JOIN \""+organizationId+"\".webhook_event_types as wet ON wes.webhook_event_type_id=wet.id \
        WHERE wi.dataset_id="+datasetId+" \
        AND wet.event_name=\'"+event_category+ "\' \
        AND wh.is_disabled=False)"


def query(conn, command):
    cur = conn.cursor(cursor_factory = psycopg2.extras.RealDictCursor)
    try:
        cur.execute(command)
    except psycopg2.errors.OperationalError as e:
        print("Query error.")
        print(e.diag.severity)
        print(e.diag.message_primary)
        print(e.diag)
    results = cur.fetchall()
    cur.close()
    data = [dict(row) for row in results]
    return data

sqs_resource = boto3.resource('sqs')
queue = sqs_resource.get_queue_by_name(QueueName=os.environ.get("WEBHOOK_SQS_QUEUE_NAME"))
sqs = boto3.client('sqs')
ssm = boto3.client('ssm')


def lambda_handler(event, _context):
    logger = logging.getLogger()
    logger.setLevel(logging.INFO)
    logger.info('## EVENT : {}'.format(event))

    conn=connect()


    org_commands=defaultdict(list) # (key, value) = (organizationID (datasetId, event_type)
    for record in event['Records']:
        message=json.loads(record['body'].replace('\\n',''))#.replace('null','None')) 
        message=json.loads(message['Message'])
        org_commands[message['organizationId']].append((message['datasetId'], message['eventCategory'], message['eventDetail']))


    for organizationId, dataEvents in org_commands.items():
        command=[]
        webhook_messages=defaultdict(list) # (key, value) = (organizationID (datasetId, event_type)

        print(str(dataEvents))
        for (datasetId, eventCategory, eventDetail) in dataEvents:
            command.append(query_builder(organizationId,datasetId,eventCategory))
            command.append(" UNION ALL ")
            #creating a message for specific dataset and eventCategory
            webhook_messages[(datasetId,eventCategory)].append(eventDetail)
        command.pop()

        #merging all queries for a given organization
        #extracting datasetIds, eventCategory and URLs from PostgreSQL
        webhooks=query(conn, command[0])

        #appending URLs to 
        for w in webhooks:
            if  (str(w['dataset_id']), w['event_name']) in webhook_messages.keys():
                webhook_messages[(str(w['dataset_id']), w['event_name'])].append(w['api_url'])

    #now assigning messages from lambda to particular webhooks

        http = urllib3.PoolManager()
        headers = {"Content-type" : "application/json"}


        for record in webhook_messages.values():
            # Send message to SQS queue
            print("Sending message "+ str(record[0])+ " to " +str(record[1]))
            r = http.request(method='POST', url=record[1],
                         headers=headers,
                         body='{"text": "' + str(record[0])+'"}')
            logger.info("Message sent to " + record[1])
            logger.info(r.data)
            logger.info(r.status)

    return event

