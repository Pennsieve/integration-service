#!/usr/bin/env bash

if [[ "$#" -ne 1 ]]; then
    >&2 echo "usage: generate-migration-files.sh [filename]"
    exit 1
fi

FILENAME=$1
DIRECTORY=$(find . -path './.git' -prune -o -name migrations -print | head -1)

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
