package weather

import (
	"context"
	"time"
)

type Condition string

const (
	ConditionSnow   Condition = "snow"
	ConditionRain   Condition = "rain"
	ConditionCold   Condition = "cold"
	ConditionHot    Condition = "hot"
	ConditionNormal Condition = "normal"
)

type NextHour struct {
	Hour       time.Time
	TempC      float64
	PrecipMM   float64
	SnowfallCM float64
	Condition  Condition
	Multiplier float64
}

// Source is what the recommendation code depends on, so tests can fake it
// instead of hitting a real weather provider.
type Source interface {
	NextHour(ctx context.Context, now time.Time) (NextHour, error)
}
