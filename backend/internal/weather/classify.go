package weather

type rule struct {
	condition  Condition
	matches    func(tempC, precipMM, snowfallCM float64) bool
	multiplier float64
}

var rules = []rule{
	{
		condition:  ConditionSnow,
		matches:    func(_, _, snowfallCM float64) bool { return snowfallCM >= 0.1 },
		multiplier: 0.96,
	},
	{
		condition:  ConditionRain,
		matches:    func(_, precipMM, _ float64) bool { return precipMM >= 0.2 },
		multiplier: 1.04,
	},
	{
		condition:  ConditionCold,
		matches:    func(tempC, _, _ float64) bool { return tempC <= -15 },
		multiplier: 0.96,
	},
	{
		condition:  ConditionHot,
		matches:    func(tempC, _, _ float64) bool { return tempC >= 30 },
		multiplier: 0.95,
	},
}

// classify applies the rule table in order and falls back to normal/1.0 if
// nothing matches.
func classify(tempC, precipMM, snowfallCM float64) (Condition, float64) {
	for _, r := range rules {
		if r.matches(tempC, precipMM, snowfallCM) {
			return r.condition, r.multiplier
		}
	}
	return ConditionNormal, 1.0
}
