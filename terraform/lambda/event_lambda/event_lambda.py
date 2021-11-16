import os
import logging
import boto3
import psycopg2
import psycopg2.extras
import urllib3
import json
from collections import defaultdict
from datetime import datetime, timedelta

# https://urllib3.readthedocs.io/en/latest/user-guide.html

http = urllib3.PoolManager()
ssm = boto3.client('ssm')
logger = logging.getLogger()
logger.setLevel(logging.INFO)
logger.info("Lambda CONSTRUCTOR invoked at " + str(datetime.now()))
webhook_cache = {}
env = os.environ.get("ENV")

"""
Generic RDS query wrapper.
"""


def query(command):
    conn = connect()
    cur = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
    try:
        cur.execute(command)
    except psycopg2.errors.OperationalError as e:
        logger.error("Query error.")
        logger.error(e)
        logger.error('Type:' + str(type(e)))
        logger.error('Severity:' + str(e.diag.severity))
        logger.error('Diagnostic message' + str(e.diag.message_primary))
        logger.error('Full diagnostics:' + str(e.diag))
    results = cur.fetchall()
    cur.close()
    conn.close()
    data = [dict(row) for row in results]
    return data


"""
Handles a connection to a RDS
"""


def connect():
    dbname = ssm.get_parameter(Name='/{}/integration-service/integrations-postgres-db'.format(env))['Parameter']['Value']
    dbuser = ssm.get_parameter(Name='/{}/integration-service/integrations-postgres-user'.format(env))['Parameter']['Value']
    dbpass = ssm.get_parameter(Name='/{}/integration-service/integrations-postgres-password'.format(env), WithDecryption=True)[
        'Parameter'][
        'Value']
    dbhost = ssm.get_parameter(Name='/{}/integration-service/integrations-postgres-host'.format(env))['Parameter']['Value']

    try:
        conn = psycopg2.connect(
            "dbname='" + dbname + "' user='" + dbuser + "' password='" + dbpass + "'" + "host='" + dbhost + "'")
        logger.info('Successfully connected to host ' + dbhost + ' database ' + 'dbname')
    except psycopg2.errors.OperationalError as e:
        logger.error("Connection error.")
        logger.error(e)
        logger.error('Type:' + str(type(e)))
        logger.error('Severity:' + str(e.diag.severity))
        logger.error('Diagnostic message' + str(e.diag.message_primary))
        logger.error('Full diagnostics:' + str(e.diag))
    return conn


"""
Creates a query to a RDS to pull webhook records
Refreshes webhook_cache global variable
"""


def refresh_webhook_cache(organization_id):
    command = "SELECT wh.api_url, wet.event_name, wi.dataset_id \
                            FROM \"" + organization_id + "\".webhooks AS wh \
                            INNER JOIN \"" + organization_id + "\".webhook_event_subscriptions as wes ON  wh.id=wes.webhook_id \
                            INNER JOIN \"" + organization_id + "\".dataset_integrations as wi ON  wh.id=wi.webhook_id \
                            INNER JOIN \"" + organization_id + "\".webhook_event_types as wet ON wes.webhook_event_type_id=wet.id"
    webhook_cache[organization_id] = {'lastUpdated': datetime.now(), 'webhooks': query(command)}
    logger.info('REFRESHING CACHE: ' + str(webhook_cache))


"""
Iterate over messages and populate org_commands dict with (datasetId,eventCategory) keys per org.
"""


def map_events(events):
    mapped_events = defaultdict(list)
    force_refresh = False
    for record in events['Records']:
        message = json.loads(record['body'].replace('\\n', ''))  # making sure to replace newlines
        message = json.loads(message['Message'])  # loading a message from lambda
        mapped_events[message['organizationId']].append(
            (message['datasetId'], message['eventCategory'], message))
        if message['eventType'] == 'CREATE_DATASET':
            force_refresh = True

    return mapped_events, force_refresh


"""
Controls webhook launch. 
1. Refreshes cache (connects and pulls data from RDS) for a given organization if >10 minutes from the last refresh.
2. Adds messages from lambda to webhook_messages structure
3. Updates webhook_messages with data from cache.
"""


def map_webhook_messages(mapped_events, force_refresh):
    for organization_id, dataEvents in mapped_events.items():

        # if cache for organization expired or over 10 minutes since last call
        if force_refresh or organization_id not in webhook_cache.keys() or \
                (datetime.now() - webhook_cache[organization_id]['lastUpdated']) > timedelta(minutes=10):
            refresh_webhook_cache(organization_id)

        webhook_messages = defaultdict(lambda: defaultdict(list))
        for (datasetId, eventCategory, eventDetail) in dataEvents:
            # creating a message for specific dataset and eventCategory
            webhook_messages[(int(datasetId), eventCategory)]['message'].append(eventDetail)

        # Iterate over webhooks and if
        for w in webhook_cache[organization_id]['webhooks']:
            if (w['dataset_id'], w['event_name']) in webhook_messages.keys():
                webhook_messages[(w['dataset_id'], w['event_name'])]['url'].append(w['api_url'])

    return webhook_messages


"""
Receives webhook_messages, a dict with (datasetId,eventCategory) as keys and a dict with 'message' and 'url' as values.
"""


def broadcast_messages(webhook_messages):
    headers = {"Content-type": "application/json"}
    for record in webhook_messages.values():
        for url in record['url']:
            for message in record['message']:
                try:
                    response = http.request(method='POST', url=url,
                                            headers=headers,
                                            body='{"text": "' + str(message) + '"}',
                                            timeout=urllib3.Timeout(connect=0.25))
                except urllib3.exceptions.HTTPError as errh:
                    logger.warning("An Http Error occurred:" + repr(errh))
                except urllib3.exceptions.ConnectionError as errc:
                    logger.warning("An Error Connecting to the API occurred:" + repr(errc))
                except urllib3.exceptions.ConnectTimeoutError as errt:
                    logger.info("A Timeout Error occurred:" + repr(errt))


"""
Handles lambda invocation
"""


def lambda_handler(events, _context):
    logger.info("Lambda handler invoked at " + str(datetime.now()) + " with " + str(len(events['Records'])) + " events.")

    # Map Events to Organizations, Datasets, and Event Categories
    mapped_events, force_refresh = map_events(events)

    # Map messages to external API urls
    webhook_messages = map_webhook_messages(mapped_events, force_refresh)

    # Send messages to external APIs
    broadcast_messages(webhook_messages)

    return events
