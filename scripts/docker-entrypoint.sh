#!/bin/sh
set -e

echo "Starting SAI Storage container initialization..."

# Determine which config template to use based on environment
CONFIG_TEMPLATE="config.yaml.template"

echo "Using config template: $CONFIG_TEMPLATE"

# Check if template exists
if [ ! -f "./$CONFIG_TEMPLATE" ]; then
    echo "Error: Config template ./$CONFIG_TEMPLATE not found!"
    exit 1
fi

# Process template with environment variables
echo "Processing configuration template with environment variables..."
envsubst < "./$CONFIG_TEMPLATE" > "./config.yaml"

echo "Configuration file generated successfully:"
echo "--- Generated config.yaml ---"
cat "./config.yaml"
echo "--- End of config ---"

echo "Environment validation passed."
echo "Starting application with command: $@"

exec "$@"