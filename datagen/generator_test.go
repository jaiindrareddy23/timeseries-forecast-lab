package main

import (
	"math"
	"testing"
	"time"
)

func TestGenerateLengthAndNoNegatives(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Days = 400
	obs := Generate(cfg)

	if len(obs) != 400 {
		t.Fatalf("expected 400 observations, got %d", len(obs))
	}
	for _, o := range obs {
		if o.Demand < 0 {
			t.Fatalf("found negative demand %v on %v", o.Demand, o.Date)
		}
	}
}

func TestGenerateDatesAreConsecutive(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Days = 100
	obs := Generate(cfg)

	for i := 1; i < len(obs); i++ {
		want := obs[i-1].Date.AddDate(0, 0, 1)
		if !obs[i].Date.Equal(want) {
			t.Fatalf("gap between %v and %v", obs[i-1].Date, obs[i].Date)
		}
	}
}

// TestWeeklySeasonalityPresent checks that average demand on the configured
// high-amplitude days (Saturday) is measurably higher than on low days
// (Tuesday), confirming the weekly pattern survives noise over many weeks.
func TestWeeklySeasonalityPresent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Days = 3 * 365
	cfg.NoiseStd = 5 // low noise to isolate the seasonal signal
	obs := Generate(cfg)

	var satSum, satN, tueSum, tueN float64
	for _, o := range obs {
		switch o.Date.Weekday() {
		case time.Saturday:
			satSum += o.Demand
			satN++
		case time.Tuesday:
			tueSum += o.Demand
			tueN++
		}
	}
	satMean := satSum / satN
	tueMean := tueSum / tueN

	if satMean <= tueMean {
		t.Fatalf("expected Saturday mean (%.2f) > Tuesday mean (%.2f)", satMean, tueMean)
	}
	// weeklyPattern says Saturday is 1.25x-1 and Tuesday 0.80x-1 of WeeklyAmp=60,
	// so the gap should be roughly 60*(1.25-0.80) = 27, within noise/yearly tolerance.
	gap := satMean - tueMean
	if gap < 15 || gap > 40 {
		t.Fatalf("Saturday-Tuesday gap %.2f outside expected range [15,40]", gap)
	}
}

// TestYearlySeasonalityPresent checks the mean demand near the configured
// yearly peak (day ~355) is higher than the mean near the trough (day ~172,
// half a year earlier/later), confirming the cosine yearly component works.
func TestYearlySeasonalityPresent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Days = 3 * 365
	cfg.NoiseStd = 5
	cfg.WeeklyAmp = 0 // isolate yearly component
	obs := Generate(cfg)

	var peakSum, peakN, troughSum, troughN float64
	for _, o := range obs {
		doy := o.Date.YearDay()
		if doy >= 345 && doy <= 365 {
			peakSum += o.Demand
			peakN++
		}
		if doy >= 160 && doy <= 180 {
			troughSum += o.Demand
			troughN++
		}
	}
	peakMean := peakSum / peakN
	troughMean := troughSum / troughN

	if peakMean <= troughMean {
		t.Fatalf("expected peak-season mean (%.2f) > trough-season mean (%.2f)", peakMean, troughMean)
	}
}

func TestDeterministicWithSameSeed(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Days = 50
	a := Generate(cfg)
	b := Generate(cfg)
	for i := range a {
		if math.Abs(a[i].Demand-b[i].Demand) > 1e-9 {
			t.Fatalf("same seed produced different output at index %d: %v vs %v", i, a[i].Demand, b[i].Demand)
		}
	}
}
