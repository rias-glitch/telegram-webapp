#!/bin/sh
set -e

echo "Applying migrations..."
/bin/migrate_apply -apply || true

echo "Starting app"
/bin/app
