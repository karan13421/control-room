CREATE TABLE IF NOT EXISTS community_areas (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS trip_hourly (
    area_id    INTEGER NOT NULL REFERENCES community_areas(id),
    hour_start TEXT    NOT NULL,           -- 'YYYY-MM-DD HH:00' America/Chicago
    trips      INTEGER NOT NULL,
    fare_sum   REAL    NOT NULL DEFAULT 0,
    PRIMARY KEY (area_id, hour_start)
) WITHOUT ROWID;


