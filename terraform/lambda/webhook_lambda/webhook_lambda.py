import os
import logging

logger = logging.getLogger()
logger.setLevel(logging.INFO)

def lambda_handler(event, _context):
    logger.info('## PLATFORM WEBHOOK HANDLER LAMBDA EVENT')
    logger.info(event)

    return event