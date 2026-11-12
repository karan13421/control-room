package recommendation

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// window is the calendar range [start, end] (inclusive, local midnights)
// that trip_hourly actually has data for.
type window struct {
	start time.Time
	end   time.Time
}

func loadWindow(db *sqlx.DB, loc *time.Location) (window, error) {
	var minHour, maxHour *string
	if err := db.QueryRow(`SELECT MIN(hour_start), MAX(hour_start) FROM trip_hourly`).Scan(&minHour, &maxHour); err != nil {
		return window{}, fmt.Errorf("recommendation: load window: %w", err)
	}
	if minHour == nil || maxHour == nil {
		return window{}, ErrNoData
	}

	start, err := time.ParseInLocation("2006-01-02", (*minHour)[:10], loc)
	if err != nil {
		return window{}, fmt.Errorf("recommendation: parse window start: %w", err)
	}
	end, err := time.ParseInLocation("2006-01-02", (*maxHour)[:10], loc)
	if err != nil {
		return window{}, fmt.Errorf("recommendation: parse window end: %w", err)
	}

	return window{start: start, end: end}, nil
}

// slotOccurrences returns every real local instant within w that falls on
// weekday at hour. The window can contain a spring-DST gap where that local
// time never happened; time.Date silently normalizes an invalid wall-clock
// hour forward, so comparing the hour we asked for against what we got back
// detects and skips it generically, without hardcoding the date.
func slotOccurrences(w window, weekday time.Weekday, hour int, loc *time.Location) []time.Time {
	var out []time.Time
	for d := w.start; !d.After(w.end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != weekday {
			continue
		}
		t := time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, loc)
		if t.Hour() != hour {
			continue
		}
		out = append(out, t)
	}
	return out
}

// areaBaseline is the zero-filled average trips/hour for one area, for the
// "now" and "next" hour-of-week slots.
type areaBaseline struct {
	AreaID  int
	NowAvg  float64
	NextAvg float64
}

type slotSum struct {
	AreaID  int `db:"area_id"`
	Weekday int `db:"wd"`
	Hour    int `db:"hr"`
	Total   int `db:"total"`
}

func computeBaselines(db *sqlx.DB, loc *time.Location, w window, nowWeekday, nextWeekday time.Weekday, nowHour, nextHour int) ([]areaBaseline, error) {
	nowCount := len(slotOccurrences(w, nowWeekday, nowHour, loc))
	nextCount := len(slotOccurrences(w, nextWeekday, nextHour, loc))
	if nowCount == 0 || nextCount == 0 {
		return nil, fmt.Errorf("recommendation: requested hour never occurred in the data window")
	}

	var sums []slotSum
	query := `
		SELECT area_id,
		       CAST(strftime('%w', hour_start) AS INTEGER) AS wd,
		       CAST(strftime('%H', hour_start) AS INTEGER) AS hr,
		       SUM(trips) AS total
		FROM trip_hourly
		WHERE (strftime('%w', hour_start) = ? AND strftime('%H', hour_start) = ?)
		   OR (strftime('%w', hour_start) = ? AND strftime('%H', hour_start) = ?)
		GROUP BY area_id, wd, hr`
	err := db.Select(&sums, query,
		fmt.Sprintf("%d", int(nowWeekday)), fmt.Sprintf("%02d", nowHour),
		fmt.Sprintf("%d", int(nextWeekday)), fmt.Sprintf("%02d", nextHour),
	)
	if err != nil {
		return nil, fmt.Errorf("recommendation: load slot sums: %w", err)
	}

	totals := map[int]*struct{ now, next int }{}
	for _, s := range sums {
		t, ok := totals[s.AreaID]
		if !ok {
			t = &struct{ now, next int }{}
			totals[s.AreaID] = t
		}
		switch {
		case s.Weekday == int(nowWeekday) && s.Hour == nowHour:
			t.now += s.Total
		case s.Weekday == int(nextWeekday) && s.Hour == nextHour:
			t.next += s.Total
		}
	}

	baselines := make([]areaBaseline, 0, len(totals))
	for areaID, t := range totals {
		baselines = append(baselines, areaBaseline{
			AreaID:  areaID,
			NowAvg:  float64(t.now) / float64(nowCount),
			NextAvg: float64(t.next) / float64(nextCount),
		})
	}
	return baselines, nil
}
