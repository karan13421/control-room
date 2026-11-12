package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"control_room/internal/service"
	"control_room/internal/weather"
)

func NewRouter(db *sqlx.DB, weatherSrc weather.Source) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", HealthCheck)

	communityAreas := NewCommunityAreasHandler(service.NewCommunityAreaService(db))
	r.GET("/community-areas", communityAreas.List)

	rec := NewRecommendationHandler(db, weatherSrc)
	r.GET("/api/recommendation", rec.Get)

	return r
}
