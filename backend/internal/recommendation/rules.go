package recommendation

import (
	"sort"

	"github.com/samber/lo"
)

const (
	GapThreshold   = 10.0
	RatioThreshold = 1.15
)

// qualifies is the one rule that decides whether an area is worth moving
// drivers to: a big enough absolute jump and a big enough relative jump.
func qualifies(gap, ratio float64) bool {
	return gap >= GapThreshold && ratio >= RatioThreshold
}

// areaResult is one area's expected-next-hour demand, after the weather
// multiplier has been applied.
type areaResult struct {
	AreaID int
	Now    float64
	Next   float64
	Gap    float64
	Ratio  float64
}

// rankAreas applies the weather multiplier to every area's next-hour
// baseline and computes its gap/ratio against the current hour. Areas with
// no current demand are dropped since their ratio is undefined.
func rankAreas(baselines []areaBaseline, multiplier float64) []areaResult {
	withDemand := lo.Filter(baselines, func(b areaBaseline, _ int) bool { return b.NowAvg > 0 })

	return lo.Map(withDemand, func(b areaBaseline, _ int) areaResult {
		next := b.NextAvg * multiplier
		return areaResult{
			AreaID: b.AreaID,
			Now:    b.NowAvg,
			Next:   next,
			Gap:    next - b.NowAvg,
			Ratio:  next / b.NowAvg,
		}
	})
}

// pickWinner recommends the qualifying area with the largest gap, plus up to
// two runners-up (preferring other qualifying areas, falling back to the
// closest non-qualifying ones). A nil winner means "hold".
func pickWinner(results []areaResult) (*areaResult, []areaResult) {
	sorted := append([]areaResult(nil), results...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Gap > sorted[j].Gap })

	qualifying := lo.Filter(sorted, func(r areaResult, _ int) bool { return qualifies(r.Gap, r.Ratio) })
	if len(qualifying) == 0 {
		return nil, firstN(sorted, 2)
	}

	winner := qualifying[0]
	runnersUp := firstN(qualifying[1:], 2)
	if len(runnersUp) < 2 {
		runnersUp = firstN(lo.Filter(sorted, func(r areaResult, _ int) bool { return r.AreaID != winner.AreaID }), 2)
	}
	return &winner, runnersUp
}

func firstN(rs []areaResult, n int) []areaResult {
	if len(rs) < n {
		return rs
	}
	return rs[:n]
}
