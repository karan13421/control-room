package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"control_room/internal/store"
	"control_room/internal/weather"
)

type erroringWeather struct{}

func (erroringWeather) NextHour(ctx context.Context, now time.Time) (weather.NextHour, error) {
	return weather.NextHour{}, errors.New("weather source unavailable")
}

func TestRecommendationHandler_BadAtParam(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, store.RunMigrations(db, store.Migrations()))

	router := NewRouter(db, erroringWeather{}, "")

	req := httptest.NewRequest(http.MethodGet, "/api/recommendation?at=not-a-date", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
