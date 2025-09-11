#!/bin/sh
set -e

echo "Starting Drop service with migrations..."

# Ensure data directory exists
mkdir -p /data

# Also ensure uploads directory exists  
mkdir -p /uploads

# Run migrations
echo "Running database migrations..."
/app/migrate -action up -db /data/dump.db

echo "Migrations completed successfully"
echo "Starting Drop service..."

# Set config path
export CONFIG_PATH="/app/config/config.yaml"
echo "Config path set to: $CONFIG_PATH"

# Start the main application
exec /app/drop
