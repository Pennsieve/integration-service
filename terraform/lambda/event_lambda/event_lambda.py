import os
import logging
import boto3
import psycopg2
import psycopg2.extras
import urllib3
import json
from collections import defaultdict
from datetime import datetime, timedelta
#import cache
#https://urllib3.readthedocs.io/en/latest/user-guide.html




http = urllib3.PoolManager()
sqs_resource = boto3.resource('sqs')
queue = sqs_resource.get_queue_by_name(QueueName=os.environ.get("WEBHOOK_SQS_QUEUE_NAME"))
sqs = boto3.client('sqs')
ssm = boto3.client('ssm')
logger = logging.getLogger()
logger.setLevel(logging.INFO)
logger.info("Lambda CONSTRUCTOR invoked at " + str(datetime.now()))

webhook_cache={}


"""
Handles a connection to a RDS
"""
def connect():
    logger = logging.getLogger()
    logger.setLevel(logging.INFO)

    dbname = ssm.get_parameter(Name='/dev/integration-service/integrations-postgres-db', WithDecryption=True)['Parameter']['Value']
    dbuser = ssm.get_parameter(Name='/dev/integration-service/integrations-postgres-user', WithDecryption=True)['Parameter']['Value']
    dbpass = ssm.get_parameter(Name='/dev/integration-service/integrations-postgres-password', WithDecryption=True)['Parameter']['Value']
    dbhost = ssm.get_parameter(Name='/dev/integration-service/integrations-postgres-host', WithDecryption=True)['Parameter']['Value']

    try:
        conn=psycopg2.connect("dbname='"+dbname+"' user='"+ dbuser+"' password='" + dbpass+"'" + "host='"+dbhost + "'")
        logger.info('Successfully connected to host '+ dbhost +  ' database ' + 'dbname')
    except psycopg2.errors.OperationalError as e:
        logger.error("Connection error.")
        logger.error(e)
        logger.error('Type:'+str(type(e)))
        logger.error('Severity:'+str(e.diag.severity))
        logger.error('Diagnostic message'+str(e.diag.message_primary))
        logger.error('Full diagnostics:'+str(e.diag))
    return conn



"""
Receives webhook_messages, a dict with (datasetId,eventCategory) as keys and a dict with 'message' and 'url' as values.
"""
def broadcast_messages(webhook_messages):
    http = urllib3.PoolManager()
    headers = {"Content-type" : "application/json"}
    errors=[]

    for record in webhook_messages.values():
#        print(str(record))
        for url in record['url']:
#            print(str(url))
            for message in record['message']:
#                print(str(message))
                logger.info("Sending message "+ str(message) + " to " + url)
                r = http.request(method='POST', url=url,
                     headers=headers,
                     body='{"text": "' + str(message) +'"}')
                logger.info("Message sent to " + url)
                logger.info(r.data)
                logger.info(r.status)


"""
Creates a query to a RDS to pull webhook records
Refreshes webhook_cache global variable
"""
def refresh_webhook_cache(organizationId):
    command = "SELECT wh.api_url, wet.event_name, wi.dataset_id \
                            FROM \""+organizationId+"\".webhooks AS wh \
                            INNER JOIN \""+organizationId+"\".webhook_event_subscriptions as wes ON  wh.id=wes.webhook_id \
                            INNER JOIN \""+organizationId+"\".dataset_integrations as wi ON  wh.id=wi.webhook_id \
                            INNER JOIN \""+organizationId+"\".webhook_event_types as wet ON wes.webhook_event_type_id=wet.id"
    webhook_cache[organizationId]={ 'lastUpdated' : datetime.now(), 'webhooks' : query(command) }
    logger.info('REFRESHING CACHE: '+str(webhook_cache))



"""
Controls webhook launch. 
1. Refreshes cache (connects and pulls data from RDS) for a given organization if >10 minutes from the last refresh.
2. Adds messages from lambda to webhook_messages structure
3. Updates webhook_messages with data from cache.
"""
def invoke_webhooks(org_commands):
    for organizationId, dataEvents in org_commands.items():
        #if cache for organization expired or over 10 minutes since last call
        if organizationId not in webhook_cache.keys() or datetime.now()-webhook_cache[organizationId]['lastUpdated']>timedelta(minutes=10):
            refresh_webhook_cache(organizationId)
        webhook_messages = defaultdict(lambda: defaultdict(list))
        for (datasetId, eventCategory, eventDetail) in dataEvents:
            #creating a message for specific dataset and eventCategory
            webhook_messages[(int(datasetId),eventCategory)]['message'].append(eventDetail)
#        print(str(webhook_messages))
        for w in webhook_cache[organizationId]['webhooks']:
            if  ( w['dataset_id'], w['event_name'] ) in webhook_messages.keys():
                webhook_messages[(w['dataset_id'], w['event_name'])]['url'].append(w['api_url'])
        broadcast_messages(webhook_messages)
    return webhook_messages



"""
Generic RDS query wrapper.
"""
def query(command):
    conn=connect()
    cur = conn.cursor(cursor_factory = psycopg2.extras.RealDictCursor)
    try:
        cur.execute(command)
    except psycopg2.errors.OperationalError as e:
        logger.error("Query error.")
        logger.error(e)
        logger.error('Type:'+str(type(e)))
        logger.error('Severity:'+str(e.diag.severity))
        logger.error('Diagnostic message'+str(e.diag.message_primary))
        logger.error('Full diagnostics:'+str(e.diag))
    results = cur.fetchall()
    cur.close()
    conn.close()
    data = [dict(row) for row in results]
    return data


"""
Handles lambda invocation
"""
def lambda_handler(event, _context):
    logger.info("Lambda handler invoked at " + str(datetime.now()))
    logger.info('## EVENT : {}'.format(event))
    org_commands=defaultdict(list)  #organizationID -> (datasetId, eventCategory, eventDetail)
    for record in event['Records']:
        message=json.loads(record['body'].replace('\\n',''))  #making sure to replace newlines
        message=json.loads(message['Message']) # loading a message from lambda
        org_commands[message['organizationId']].append((message['datasetId'], message['eventCategory'], message['eventDetail']))
#    webhook_messages=defaultdict(list) # (key -> value) = organizationID -> (datasetId, event_category, event_detail)
    webhook_messages = invoke_webhooks(org_commands)

    logger.info('WHM: '+str(webhook_messages))
    return event

