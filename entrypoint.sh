#!/bin/sh
# Migrate then (re-)seed on every boot. ingest is an upsert, so running it
# again on a restart is safe - it just re-affirms the same seed data.
set -e
./migrate -db "$DB_PATH"
./ingest -db "$DB_PATH" \
  -community-areas-csv "$SEED_DIR/community_areas.csv" \
  -trip-hourly-csv "$SEED_DIR/trip_hourly.csv"
exec ./server
