// Command migrate creates/updates the sqlite schema by applying every
// migration under internal/store/migrations, in order.
//
// Usage:
//
//	go run ./cmd/migrate -db data/control_room.db
package main

import (
	"flag"
	"log"

	"control_room/internal/store"
)

func main() {
	dbPath := flag.String("db", "data/control_room.db", "path to the sqlite database file")
	flag.Parse()

	db, err := store.NewSQLite(*dbPath)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db, store.Migrations()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	log.Printf("migrations applied to %s", *dbPath)
}
