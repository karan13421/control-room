package main

import (
	"log"

	"control_room/internal/api"
	"control_room/internal/config"
	"control_room/internal/store"
	"control_room/internal/weather"
)

func main() {
	cfg := config.Load()

	db, err := store.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db, store.Migrations()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	r := api.NewRouter(db, weather.NewClient(), cfg.StaticDir)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
