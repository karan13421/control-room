package config

import "os"

type Config struct {
	Port      string
	DBPath    string
	StaticDir string // if set, NewRouter also serves the built frontend from here
}

func Load() Config {
	return Config{
		Port:      getEnv("PORT", "8080"),
		DBPath:    getEnv("DB_PATH", "data/control_room.db"),
		StaticDir: getEnv("STATIC_DIR", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
