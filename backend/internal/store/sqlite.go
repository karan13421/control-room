package store

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	_ "modernc.org/sqlite"
)

// NewSQLite opens a connection to the sqlite database at dbPath and applies
// the pragmas needed for correct, concurrent-safe operation from a web server.
func NewSQLite(dbPath string) (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("set pragma %q: %w", pragma, err)
		}
	}

	// sqlite allows only one writer at a time; capping the pool avoids
	// SQLITE_BUSY errors under concurrent requests instead of surfacing them.
	db.SetMaxOpenConns(1)

	return db, nil
}
