#!/usr/bin/env bash
set -euo pipefail

REPO="pennsievedb-integration"
MIGRATIONS_FOLDER="internal/dbmigrate/migrations"
COMPOSE_FILE="docker-compose.build-postgres.yml"

# Tag the seeded image with the timestamp of the most recent migration file
# across both schema folders, so a new migration always produces a new tag.
TAG=$(ls -1 "$MIGRATIONS_FOLDER"/*/*.up.sql | xargs -n1 basename | sort | tail -1 | cut -d'_' -f1)
IMAGE="pennsieve/${REPO}:${TAG}-seed"

echo "Building seeded Postgres image ${IMAGE}"

docker compose -f "$COMPOSE_FILE" down -v --remove-orphans || true
docker compose -f "$COMPOSE_FILE" up -d base-pennsievedb

echo "Waiting for base-pennsievedb to be healthy..."
BASE_CONTAINER_ID=$(docker compose -f "$COMPOSE_FILE" ps -q base-pennsievedb)
until [[ "$(docker inspect -f '{{.State.Health.Status}}' "$BASE_CONTAINER_ID")" == "healthy" ]]; do
    sleep 2
done

echo "Running integration-service migrations against base-pennsievedb..."
docker compose -f "$COMPOSE_FILE" run --rm integration-migrations

echo "Committing ${IMAGE}..."
docker commit "$BASE_CONTAINER_ID" "$IMAGE"

docker compose -f "$COMPOSE_FILE" down -v --remove-orphans

echo "Success. Built ${IMAGE}"
