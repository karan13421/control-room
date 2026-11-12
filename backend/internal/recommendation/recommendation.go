package recommendation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"control_room/internal/store"
	"control_room/internal/weather"
)

const timezoneName = "America/Chicago"

// ErrNoData means trip_hourly has no rows at all; callers should surface a
// clear error rather than a zeroed-out recommendation.
var ErrNoData = errors.New("recommendation: no trip data available")

type AreaRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type WindowRef struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Expected struct {
	Now   float64 `json:"now"`
	Next  float64 `json:"next"`
	Gap   float64 `json:"gap"`
	Ratio float64 `json:"ratio"`
}

type WeatherEvidence struct {
	Available  bool              `json:"available"`
	Condition  weather.Condition `json:"condition,omitempty"`
	Multiplier float64           `json:"multiplier"`
	Note       string            `json:"note,omitempty"`
}

type PastWeek struct {
	Date string `json:"date"`
	Now  int    `json:"now"`
	Next int    `json:"next"`
	Rose bool   `json:"rose"`
}

type RunnerUp struct {
	Name  string  `json:"name"`
	Gap   float64 `json:"gap"`
	Ratio float64 `json:"ratio"`
}

type Evidence struct {
	Weather    WeatherEvidence `json:"weather"`
	PastWeeks  []PastWeek      `json:"past_weeks"`
	WeeksRose  string          `json:"weeks_rose"`
	RunnersUp  []RunnerUp      `json:"runners_up"`
	DataWindow string          `json:"data_window"`
}

type Response struct {
	Action       string    `json:"action"`
	Headline     string    `json:"headline"`
	Area         *AreaRef  `json:"area"`
	Window       WindowRef `json:"window"`
	Expected     Expected  `json:"expected"`
	Evidence     Evidence  `json:"evidence"`
	CouldBeWrong []string  `json:"could_be_wrong"`
}

// Build computes which Chicago community area to send drivers to for the
// hour after now, or "hold" if none show a strong enough demand increase.
func Build(ctx context.Context, db *sqlx.DB, weatherSrc weather.Source, now time.Time) (*Response, error) {
	loc, err := time.LoadLocation(timezoneName)
	if err != nil {
		return nil, fmt.Errorf("recommendation: load location: %w", err)
	}
	now = now.In(loc)
	nowSlot := now.Truncate(time.Hour)
	nextSlot := nowSlot.Add(time.Hour)

	w, err := loadWindow(db, loc)
	if err != nil {
		return nil, err
	}

	var baselines []areaBaseline
	var forecast weather.NextHour
	var weatherErr error

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		baselines, err = computeBaselines(db, loc, w, nowSlot.Weekday(), nextSlot.Weekday(), nowSlot.Hour(), nextSlot.Hour())
		return err
	})
	g.Go(func() error {
		wctx, cancel := context.WithTimeout(gctx, 5*time.Second)
		defer cancel()
		forecast, weatherErr = weatherSrc.NextHour(wctx, now)
		return nil // weather failures must never fail the recommendation
	})
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("recommendation: %w", err)
	}

	weatherEvidence := WeatherEvidence{Multiplier: 1.0}
	multiplier := 1.0
	if weatherErr == nil {
		multiplier = forecast.Multiplier
		weatherEvidence = WeatherEvidence{
			Available:  true,
			Condition:  forecast.Condition,
			Multiplier: forecast.Multiplier,
			Note:       weatherNote(forecast.Condition),
		}
	}

	areaNames, err := areaNameLookup(db)
	if err != nil {
		return nil, err
	}

	results := rankAreas(baselines, multiplier)
	winner, runnersUp := pickWinner(results)

	resp := &Response{
		Window: WindowRef{From: formatClockHour(nowSlot), To: formatClockHour(nextSlot)},
		Evidence: Evidence{
			Weather:    weatherEvidence,
			DataWindow: fmt.Sprintf("%s to %s", w.start.Format("2006-01-02"), w.end.Format("2006-01-02")),
			RunnersUp: lo.Map(runnersUp, func(r areaResult, _ int) RunnerUp {
				return RunnerUp{Name: areaNames[r.AreaID], Gap: round1(r.Gap), Ratio: round2(r.Ratio)}
			}),
		},
	}

	if winner == nil {
		resp.Action = "hold"
		resp.Headline = "No area shows a strong enough demand increase for the next hour."
		resp.CouldBeWrong = couldBeWrong(false, 0, 0, weatherEvidence.Available, 0)
		return resp, nil
	}

	name := areaNames[winner.AreaID]
	resp.Action = "move_drivers"
	resp.Headline = fmt.Sprintf("Send drivers to %s for the next hour.", name)
	resp.Area = &AreaRef{ID: winner.AreaID, Name: name}
	resp.Expected = Expected{
		Now:   round1(winner.Now),
		Next:  round1(winner.Next),
		Gap:   round1(winner.Gap),
		Ratio: round2(winner.Ratio),
	}

	weeks, err := pastWeeks(db, w, loc, winner.AreaID, nowSlot.Weekday(), nowSlot.Hour())
	if err != nil {
		return nil, err
	}
	roseCount := lo.CountBy(weeks, func(pw PastWeek) bool { return pw.Rose })
	resp.Evidence.PastWeeks = weeks
	resp.Evidence.WeeksRose = fmt.Sprintf("%d of %d", roseCount, len(weeks))
	resp.CouldBeWrong = couldBeWrong(true, roseCount, len(weeks), weatherEvidence.Available, winner.Now)

	return resp, nil
}

func areaNameLookup(db *sqlx.DB) (map[int]string, error) {
	areas, err := store.ListCommunityAreas(db)
	if err != nil {
		return nil, fmt.Errorf("recommendation: load community areas: %w", err)
	}
	return lo.SliceToMap(areas, func(a store.CommunityArea) (int, string) { return a.ID, a.Name }), nil
}

func weatherNote(c weather.Condition) string {
	switch c {
	case weather.ConditionSnow:
		return "Snow forecast; ride demand tends to dip slightly."
	case weather.ConditionRain:
		return "Rain forecast; ride demand tends to rise."
	case weather.ConditionCold:
		return "Extreme cold forecast; ride demand tends to dip slightly."
	case weather.ConditionHot:
		return "Hot weather assumption, unverified on this winter-only dataset."
	default:
		return ""
	}
}

func formatClockHour(t time.Time) string { return t.Format("3 PM") }

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }
