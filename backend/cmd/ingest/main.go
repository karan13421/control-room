// Command ingest loads the community_areas and trip_hourly CSVs into sqlite,
// creating the schema first if needed. community_areas is ingested first
// since trip_hourly.area_id has a foreign key on it.
//
// Usage:
//
//	go run ./cmd/ingest \
//	  -db data/control_room.db \
//	  -community-areas-csv data/community_areas.csv \
//	  -trip-hourly-csv data/trip_hourly.csv
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/jmoiron/sqlx"

	"control_room/internal/store"
)

func main() {
	dbPath := flag.String("db", "data/control_room.db", "path to the sqlite database file")
	communityAreasCSV := flag.String("community-areas-csv", "data/community_areas.csv", "path to the community areas csv file")
	tripHourlyCSV := flag.String("trip-hourly-csv", "data/trip_hourly.csv", "path to the trip hourly csv file")
	flag.Parse()

	db, err := store.NewSQLite(*dbPath)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db, store.Migrations()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	if err := ingestCSV(*communityAreasCSV, "community_areas", func(r io.Reader) (int, error) {
		return ingestCommunityAreas(db, r)
	}); err != nil {
		log.Fatalf("%v", err)
	}

	if err := ingestCSV(*tripHourlyCSV, "trip_hourly", func(r io.Reader) (int, error) {
		return ingestTripHourly(db, r)
	}); err != nil {
		log.Fatalf("%v", err)
	}
}

func ingestCSV(path, label string, ingest func(io.Reader) (int, error)) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s csv: %w", label, err)
	}
	defer f.Close()

	count, err := ingest(f)
	if err != nil {
		return fmt.Errorf("ingest %s: %w", label, err)
	}
	log.Printf("ingested %d %s rows from %s", count, label, path)
	return nil
}

func ingestCommunityAreas(db *sqlx.DB, r io.Reader) (int, error) {
	reader := csv.NewReader(r)

	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read header: %w", err)
	}
	if len(header) < 2 || header[0] != "id" || header[1] != "name" {
		return 0, fmt.Errorf("unexpected csv header %v, want [id name ...]", header)
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO community_areas (id, name) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name`)
	if err != nil {
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read row %d: %w", count+1, err)
		}

		id, err := strconv.Atoi(record[0])
		if err != nil {
			return 0, fmt.Errorf("parse id %q on row %d: %w", record[0], count+1, err)
		}

		if _, err := stmt.Exec(id, record[1]); err != nil {
			return 0, fmt.Errorf("insert row %v: %w", record, err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return count, nil
}

func ingestTripHourly(db *sqlx.DB, r io.Reader) (int, error) {
	reader := csv.NewReader(r)

	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read header: %w", err)
	}
	if len(header) < 3 || header[0] != "area_id" || header[1] != "hour_start" || header[2] != "trips" {
		return 0, fmt.Errorf("unexpected csv header %v, want [area_id hour_start trips ...]", header)
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// fare_sum isn't in the source csv; it defaults to 0 and is populated
	// elsewhere (e.g. a future fare-data ingest) via the same upsert.
	stmt, err := tx.Prepare(`
		INSERT INTO trip_hourly (area_id, hour_start, trips, fare_sum) VALUES (?, ?, ?, 0)
		ON CONFLICT(area_id, hour_start) DO UPDATE SET trips = excluded.trips`)
	if err != nil {
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read row %d: %w", count+1, err)
		}

		areaID, err := strconv.Atoi(record[0])
		if err != nil {
			return 0, fmt.Errorf("parse area_id %q on row %d: %w", record[0], count+1, err)
		}

		trips, err := strconv.Atoi(record[2])
		if err != nil {
			return 0, fmt.Errorf("parse trips %q on row %d: %w", record[2], count+1, err)
		}

		if _, err := stmt.Exec(areaID, record[1], trips); err != nil {
			return 0, fmt.Errorf("insert row %v: %w", record, err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return count, nil
}
