package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"control_room/internal/service"
	"control_room/internal/weather"
)

// NewRouter wires the API. When staticDir is non-empty, it also serves the
// built frontend from there (any path that isn't a file in staticDir falls
// back to index.html, for client-side routing) - this is what lets the
// production image be a single container serving both.
func NewRouter(db *sqlx.DB, weatherSrc weather.Source, staticDir string) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", HealthCheck)

	communityAreas := NewCommunityAreasHandler(service.NewCommunityAreaService(db))
	r.GET("/community-areas", communityAreas.List)

	rec := NewRecommendationHandler(db, weatherSrc)
	r.GET("/api/recommendation", rec.Get)

	if staticDir != "" {
		r.NoRoute(func(c *gin.Context) {
			path := filepath.Join(staticDir, filepath.Clean(c.Request.URL.Path))
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				c.File(path)
				return
			}
			c.File(filepath.Join(staticDir, "index.html"))
		})
	} else {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		})
	}

	return r
}
