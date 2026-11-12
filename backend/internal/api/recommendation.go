package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"control_room/internal/recommendation"
	"control_room/internal/weather"
)

type RecommendationHandler struct {
	DB      *sqlx.DB
	Weather weather.Source
}

func NewRecommendationHandler(db *sqlx.DB, weatherSrc weather.Source) *RecommendationHandler {
	return &RecommendationHandler{DB: db, Weather: weatherSrc}
}

type recommendationQuery struct {
	At string `form:"at"`
}

func (h *RecommendationHandler) Get(c *gin.Context) {
	var q recommendationQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	now := time.Now()
	if q.At != "" {
		loc, err := time.LoadLocation("America/Chicago")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load timezone"})
			return
		}
		now, err = time.ParseInLocation("2006-01-02T15:04", q.At, loc)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid at parameter, expected format 2006-01-02T15:04"})
			return
		}
	}

	resp, err := recommendation.Build(c.Request.Context(), h.DB, h.Weather, now)
	if err != nil {
		if errors.Is(err, recommendation.ErrNoData) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no trip data available"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build recommendation"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
