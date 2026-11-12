package recommendation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"control_room/internal/store"
	"control_room/internal/weather"
)

func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := store.NewSQLite(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	require.NoError(t, store.RunMigrations(db, store.Migrations()))
	return db
}

func chicagoLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(timezoneName)
	require.NoError(t, err)
	return loc
}

func TestQualifies(t *testing.T) {
	tests := []struct {
		name       string
		gap, ratio float64
		want       bool
	}{
		{"meets both thresholds exactly", 10, 1.15, true},
		{"comfortably qualifies", 50, 2.0, true},
		{"gap too small", 9.99, 1.2, false},
		{"ratio too small", 20, 1.1, false},
		{"meets neither", 0, 1.0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, qualifies(tc.gap, tc.ratio))
		})
	}
}

func TestPickWinner(t *testing.T) {
	t.Run("holds when none qualify", func(t *testing.T) {
		results := []areaResult{
			{AreaID: 1, Gap: 5, Ratio: 1.05},
			{AreaID: 2, Gap: 3, Ratio: 1.02},
		}
		winner, runnersUp := pickWinner(results)
		assert.Nil(t, winner)
		assert.Len(t, runnersUp, 2)
	})

	t.Run("picks the largest gap among qualifying areas, preferring other qualifying runners-up", func(t *testing.T) {
		results := []areaResult{
			{AreaID: 4, Gap: 1, Ratio: 1.0}, // doesn't qualify, shouldn't crowd out qualifying runners-up
			{AreaID: 3, Gap: 15, Ratio: 1.2},
			{AreaID: 1, Gap: 50, Ratio: 2.0},
			{AreaID: 2, Gap: 30, Ratio: 1.5},
		}
		winner, runnersUp := pickWinner(results)
		require.NotNil(t, winner)
		assert.Equal(t, 1, winner.AreaID)
		require.Len(t, runnersUp, 2)
		assert.Equal(t, 2, runnersUp[0].AreaID)
		assert.Equal(t, 3, runnersUp[1].AreaID)
	})

	t.Run("fills remaining runner-up slots with the closest non-qualifying areas", func(t *testing.T) {
		results := []areaResult{
			{AreaID: 3, Gap: 5, Ratio: 1.05}, // doesn't qualify, but is the closest
			{AreaID: 1, Gap: 50, Ratio: 2.0},
			{AreaID: 2, Gap: 12, Ratio: 1.2}, // only other qualifying area
		}
		winner, runnersUp := pickWinner(results)
		require.NotNil(t, winner)
		assert.Equal(t, 1, winner.AreaID)
		require.Len(t, runnersUp, 2)
		assert.Equal(t, 2, runnersUp[0].AreaID)
		assert.Equal(t, 3, runnersUp[1].AreaID)
	})
}

func TestComputeBaselines_ZeroFillsMissingWeeks(t *testing.T) {
	db := newTestDB(t)
	loc := chicagoLoc(t)

	_, err := db.Exec(`INSERT INTO community_areas (id, name) VALUES (1, 'Test Area')`)
	require.NoError(t, err)

	insertTrip(t, db, 1, "2021-06-07 10:00", 10) // week 1
	insertTrip(t, db, 1, "2021-06-14 10:00", 20) // week 2
	// 2021-06-21 10:00 (week 3) intentionally has no row for area 1.

	w := window{
		start: time.Date(2021, 6, 7, 0, 0, 0, 0, loc),
		end:   time.Date(2021, 6, 21, 0, 0, 0, 0, loc),
	}

	baselines, err := computeBaselines(db, loc, w, time.Monday, time.Monday, 10, 11)
	require.NoError(t, err)
	require.Len(t, baselines, 1)

	naiveAvg := (10.0 + 20.0) / 2.0
	zeroFilledAvg := (10.0 + 20.0 + 0.0) / 3.0

	assert.Less(t, baselines[0].NowAvg, naiveAvg)
	assert.InDelta(t, zeroFilledAvg, baselines[0].NowAvg, 0.001)
}

type erroringWeather struct{}

func (erroringWeather) NextHour(ctx context.Context, now time.Time) (weather.NextHour, error) {
	return weather.NextHour{}, errors.New("weather source unavailable")
}

func TestBuild_WeatherErrorStillRecommends(t *testing.T) {
	db := newTestDB(t)
	loc := chicagoLoc(t)

	_, err := db.Exec(`INSERT INTO community_areas (id, name) VALUES (1, 'Area One')`)
	require.NoError(t, err)

	// Two Mondays at 17:00/18:00 is enough to give Build a non-empty window.
	insertTrip(t, db, 1, "2019-01-07 17:00", 10)
	insertTrip(t, db, 1, "2019-01-07 18:00", 10)
	insertTrip(t, db, 1, "2019-01-14 17:00", 10)
	insertTrip(t, db, 1, "2019-01-14 18:00", 10)

	now := time.Date(2019, 1, 14, 17, 20, 0, 0, loc)

	resp, err := Build(context.Background(), db, erroringWeather{}, now)
	require.NoError(t, err)
	assert.False(t, resp.Evidence.Weather.Available)
	assert.Equal(t, 1.0, resp.Evidence.Weather.Multiplier)
}

func insertTrip(t *testing.T, db *sqlx.DB, areaID int, hourStart string, trips int) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO trip_hourly (area_id, hour_start, trips) VALUES (?, ?, ?)`, areaID, hourStart, trips)
	require.NoError(t, err)
}
