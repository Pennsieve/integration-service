#!/usr/bin/env bash

if [[ "$#" -ne 2 ]]; then
    >&2 echo "usage: generate-migration-files.sh [webhooks|notifications] [filename]"
    exit 1
fi

SCHEMA=$1
FILENAME=$2

if [[ "$SCHEMA" != "webhooks" && "$SCHEMA" != "notifications" ]]; then
    >&2 echo "usage: schema must be one of 'webhooks' or 'notifications'"
    exit 1
fi

MIGRATIONS_ROOT=$(find . -path './.git' -prune -o -name migrations -print | head -1)
DIRECTORY="$MIGRATIONS_ROOT/$SCHEMA"

if [[ ! -d "$DIRECTORY" ]]; then
    >&2 echo "error: migrations directory '$DIRECTORY' does not exist"
    exit 1
fi

if [[ "$FILENAME" == *.sql ]]; then
    >&2 echo "usage: migration filename cannot end with '.sql'"
    exit 1
fi

DATE=$(date "+%Y%m%d%H%M%S")
# golang-migrate expects migration files of the form:
# {version}_{title}.up.{extension}
# {version}_{title}.down.{extension}
FULL_UP_FILENAME="$DIRECTORY/${DATE}_$FILENAME.up.sql"
FULL_DOWN_FILENAME="$DIRECTORY/${DATE}_$FILENAME.down.sql"

touch "$FULL_UP_FILENAME" "$FULL_DOWN_FILENAME"

echo "Success. Created migration files $FULL_UP_FILENAME $FULL_DOWN_FILENAME"
