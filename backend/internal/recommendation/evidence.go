package recommendation

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

const hourKeyLayout = "2006-01-02 15:04"

// pastWeeks shows, for one area, what actually happened at this hour-of-week
// slot in every past week of the data window: the recorded "now" trips and
// the recorded "next" trips (one hour later), zero-filled where no row
// exists.
func pastWeeks(db *sqlx.DB, w window, loc *time.Location, areaID int, nowWeekday time.Weekday, nowHour int) ([]PastWeek, error) {
	occurrences := slotOccurrences(w, nowWeekday, nowHour, loc)
	if len(occurrences) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(occurrences)*2)
	pairs := make([][2]string, len(occurrences))
	for i, t := range occurrences {
		nowKey, nextKey := t.Format(hourKeyLayout), t.Add(time.Hour).Format(hourKeyLayout)
		pairs[i] = [2]string{nowKey, nextKey}
		keys = append(keys, nowKey, nextKey)
	}

	var rows []struct {
		HourStart string `db:"hour_start"`
		Trips     int    `db:"trips"`
	}
	query, args, err := sqlx.In(`SELECT hour_start, trips FROM trip_hourly WHERE area_id = ? AND hour_start IN (?)`, areaID, keys)
	if err != nil {
		return nil, fmt.Errorf("recommendation: build past weeks query: %w", err)
	}
	if err := db.Select(&rows, db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("recommendation: load past weeks: %w", err)
	}

	trips := make(map[string]int, len(rows))
	for _, r := range rows {
		trips[r.HourStart] = r.Trips
	}

	return lo.Map(pairs, func(p [2]string, _ int) PastWeek {
		now, next := trips[p[0]], trips[p[1]]
		return PastWeek{Date: p[0][:10], Now: now, Next: next, Rose: next > now}
	}), nil
}

// couldBeWrong lists the reasons this recommendation might not hold up.
// hasWinner gates the per-area checks, which don't apply to a "hold".
func couldBeWrong(hasWinner bool, roseCount, totalWeeks int, weatherAvailable bool, nowAvg float64) []string {
	reasons := []string{
		"Holidays and one-off events are not modeled.",
		"The historical data covers only Jan-Mar 2019, so seasonal patterns outside that window aren't captured.",
	}

	if !weatherAvailable {
		reasons = append(reasons, "Next-hour weather forecast was unavailable; the demand multiplier was assumed neutral (1.0x).")
	}

	if hasWinner {
		if totalWeeks > 0 && float64(roseCount)/float64(totalWeeks) < 0.8 {
			reasons = append(reasons, fmt.Sprintf("Only %d of %d past weeks actually saw a rise at this slot.", roseCount, totalWeeks))
		}
		if nowAvg < 5 {
			reasons = append(reasons, fmt.Sprintf("Baseline demand in this area is low (~%.1f trips/hr), so the ratio is sensitive to noise.", nowAvg))
		}
	}

	return reasons
}
