import os
import logging
import boto3
import psycopg2
import psycopg2.extras
import urllib3
import json
from collections import defaultdict
#https://urllib3.readthedocs.io/en/latest/user-guide.html




#url = 'https://enqynvfz4l12r.x.pipedream.net'
#myobj = {'somekey': 'somevalue'}

http = urllib3.PoolManager()
#r = http.request('GET', url)

#print(r.data)
#print("RS: " + r.status)

#r = http.request(method='POST', url=url+'/sample/post/request/', body=myobj)
#print("RD: "+r.data)
#print("RS: " +r.status)
callid=0



def message_Slack():
    callid=0
    http = urllib3.PoolManager()

    headers = {"Content-type" : "application/json"}

    r = http.request(method='POST', url='https://hooks.slack.com/services/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',
                     headers=headers,
                     body='{"text":"Hello '+str(callid)+'"}')


    r = http.request(method='POST', url='https://hooks.slack.com/services/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',
                     headers=headers,
                     body='{"text":"Hello '+str(callid)+'"}')
#https://hooks.slack.com/services/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
    print(r.data)
    print(r.status)

#print('RD'+r.data)
#print('RS'+r.status)
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




def query_builder(organizationId, datasetId, event_type):
#    return "(SELECT wh.name, wh.secret, wh.api_url, wh.is_disabled, wet.event_name, wi.dataset_id \
    return "(SELECT wh.api_url, wet.event_name, wi.dataset_id \
        FROM \""+organizationId+"\".webhooks AS wh \
        INNER JOIN \""+organizationId+"\".webhook_event_subscriptions as wes ON  wh.id=wes.webhook_id \
        INNER JOIN \""+organizationId+"\".dataset_integrations as wi ON  wh.id=wi.webhook_id \
        INNER JOIN \""+organizationId+"\".webhook_event_types as wet ON wes.webhook_event_type_id=wet.id \
        WHERE wi.dataset_id="+datasetId+ ")"
#        AND wet.event_name=\'"+event_type+ "\' \
#        AND wh.is_disabled=False)"


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
#    data = [dict(row) for row in cur.fetchall()]
    data = [dict(row) for row in results]
    print(str(data))
    return data

#psycopg.connect("dbname="+dbname['Parameter']['Value']+ " "

#parameters = ssm.describe_parameters()['Parameters']
#logger.info('Descr Params: '+str(parameters))


sqs_resource = boto3.resource('sqs')
queue = sqs_resource.get_queue_by_name(QueueName=os.environ.get("WEBHOOK_SQS_QUEUE_NAME"))
sqs = boto3.client('sqs')
ssm = boto3.client('ssm')


def lambda_handler(event, _context):
    # logger.info('## PLATFORM EVENT HANDLER LAMBDA EVENT: {} events'.format(len(event['Records'])))
#    datasetID='N:dataset:1fa8c657-cce8-4c55-8733-bb1b7f2bcf3d'
#    event_type='METADATA'
    message_Slack()
    logger = logging.getLogger()
    logger.setLevel(logging.INFO)
    logger.info('## EVENT : {}'.format(event))

    print(str(event))
#    records=event['Records']
#    print(str([x['Message']['datasetId'] for x in messages]))
#    print(str([x['Message']['organizationId'] for x in messages]))
#    print(str([x['Message']['event_type'] for x in messages]))

    conn=connect()


    org_commands=defaultdict(list) # (key, value) = (organizationID (datasetId, event_type)
    for record in event['Records']:
        message=json.loads(record['body'].replace('\\n',''))#.replace('null','None')) 
        print(str(message))
        message=json.loads(message['Message'])
#        webhook_messages[]=message['eventDetail']
        print(str(message))
        #query+=str(message)+" UNION ALL "
        #print(query)
#        org_commands[message['organizationId']].append((message['datasetId'], message['eventType'], message['eventDetail']))
        org_commands[message['organizationId']].append((message['datasetId'], message['eventCategory'], message['eventDetail']))
    print('comm:'+ str(org_commands))


    for organizationId, dataEvents in org_commands.items():
        command=[]
        webhook_messages=defaultdict(list) # (key, value) = (organizationID (datasetId, event_type)

        print(str(dataEvents))
        for (datasetId, eventType, eventDetail) in dataEvents:
            command.append(query_builder(organizationId,datasetId,eventType))
            command.append(" UNION ALL ")
            print(command)
            webhook_messages[(datasetId,eventType)].append(eventDetail)
        #message=[x['Message'] for x in messages]
        print("WHM: "+str(webhook_messages))
        command.pop()
        print("Q: "+str(command))
        #print('RECORDS: '+str(query(conn, command[0])))

        #merging all queries for a given organization
        webhooks=query(conn, command[0])
        for w in webhooks:
            print(str(w['dataset_id'])+ '|' + str(w['event_name']) + '|' + str(w['api_url']))
            webhook_messages[(w['dataset_id'], w['event_name'])].append(w['api_url'])
        print("WHM2: "+ str(webhook_messages))

        #now assigning messages from lambda to particular webhooks


    for record in event['Records']:
        # Send message to SQS queue

        print("R:"+str(record))
        print("B:"+str(record['body']))


        response = sqs.send_message(
            QueueUrl=queue.url,
            DelaySeconds=1,
            MessageAttributes={},
            MessageBody=record['body']
        )
    
        logger.info(response['MessageId'])
        logger.info(str(event))






    return event

