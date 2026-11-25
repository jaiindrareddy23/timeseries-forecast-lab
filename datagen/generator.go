// Package main implements the synthetic demand/sales generator used by the
// Python research pipeline in ../analysis.
//
// Generative model (additive decomposition, one observation per calendar day):
//
//	y(t) = level + trend*t + weekly(dow(t)) + yearly(doy(t)) + noise(t)
//
//	level      - base daily demand
//	trend*t    - linear growth/decline over the series
//	weekly(·)  - fixed per-day-of-week multiplier on a seasonal amplitude
//	             (weekends higher than weekdays, mimicking retail demand)
//	yearly(·)  - sin wave with period 365.25 days, phase-shifted so demand
//	             peaks in northern-hemisphere winter (e.g. holiday season)
//	noise(t)   - Gaussian white noise, mean 0, configurable std-dev
//
// The series is floored at 0 (demand cannot be negative) which is the only
// nonlinearity in the model.
package main

import (
	"math"
	"math/rand"
	"time"
)

// weeklyPattern holds a relative multiplier for each day of week, Sunday=0.
// Values are centered around 1.0; retail-style demand dips midweek and
// rises on the weekend.
var weeklyPattern = [7]float64{
	1.15, // Sunday
	0.85, // Monday
	0.80, // Tuesday
	0.85, // Wednesday
	0.95, // Thursday
	1.15, // Friday
	1.25, // Saturday
}

// GenConfig controls synthetic series generation.
type GenConfig struct {
	StartDate      time.Time
	Days           int
	BaseLevel      float64 // average daily demand before seasonality
	Trend          float64 // additive units of demand gained per day
	WeeklyAmp      float64 // amplitude of weekly seasonality (units of demand)
	YearlyAmp      float64 // amplitude of yearly seasonality (units of demand)
	YearlyPhaseDay float64 // day-of-year offset where the yearly cycle peaks
	NoiseStd       float64 // std-dev of additive Gaussian noise
	Seed           int64
}

// DefaultConfig returns sensible defaults for a 3-year daily retail-style series.
func DefaultConfig() GenConfig {
	start, _ := time.Parse("2006-01-02", "2023-01-01")
	return GenConfig{
		StartDate:      start,
		Days:           3 * 365,
		BaseLevel:      500,
		Trend:          0.12,
		WeeklyAmp:      60,
		YearlyAmp:      150,
		YearlyPhaseDay: 355, // peaks near late December
		NoiseStd:       25,
		Seed:           42,
	}
}

// Observation is one row of the generated series.
type Observation struct {
	Date   time.Time
	Demand float64
}

// Generate produces Days daily observations starting at StartDate according
// to the additive model documented above.
func Generate(cfg GenConfig) []Observation {
	rng := rand.New(rand.NewSource(cfg.Seed))
	out := make([]Observation, cfg.Days)

	for t := 0; t < cfg.Days; t++ {
		date := cfg.StartDate.AddDate(0, 0, t)
		dow := int(date.Weekday())
		doy := float64(date.YearDay())

		trendComponent := cfg.Trend * float64(t)
		weeklyComponent := cfg.WeeklyAmp * (weeklyPattern[dow] - 1.0)
		yearlyComponent := cfg.YearlyAmp * math.Cos(2*math.Pi*(doy-cfg.YearlyPhaseDay)/365.25)
		noise := rng.NormFloat64() * cfg.NoiseStd

		demand := cfg.BaseLevel + trendComponent + weeklyComponent + yearlyComponent + noise
		if demand < 0 {
			demand = 0
		}
		out[t] = Observation{Date: date, Demand: math.Round(demand*100) / 100}
	}
	return out
}
