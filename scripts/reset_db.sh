#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

echo "🛑 Stopping postgres containers..."
docker compose stop postgres postgres-init

echo "🗑️  Removing postgres containers..."
docker compose rm -f postgres postgres-init

# Detect the volume name dynamically
VOLUME_NAME=$(docker volume ls -q | grep "postgres-data" | head -n 1)

if [ -n "$VOLUME_NAME" ]; then
    echo "🧹 Removing volume: $VOLUME_NAME..."
    docker volume rm "$VOLUME_NAME"
else
    echo "⚠️  postgres-data volume not found. It might have already been removed."
fi

echo "🚀 Starting postgres and running initialization..."
docker compose up -d postgres-init

echo "✅ Database reset complete! Migrations (db/migrations/*.up.sql) and seed (db/seed/*.sql) are running in the background."
