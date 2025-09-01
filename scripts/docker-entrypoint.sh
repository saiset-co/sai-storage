#!/bin/sh

set -e

echo "Building configuration from template..."
envsubst < "./config.template.yml" > "./config.yml"

echo "Configuration built successfully"
echo "Starting application..."

exec "$@"