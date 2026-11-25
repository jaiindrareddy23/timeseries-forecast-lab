// Command datagen generates a synthetic daily demand time series and
// (optionally) validates the resulting CSV for schema and data-quality
// issues. See generator.go for the generative model.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	out := flag.String("out", "data.csv", "output CSV path")
	days := flag.Int("days", 3*365, "number of days to generate")
	start := flag.String("start", "2023-01-01", "start date, YYYY-MM-DD")
	seed := flag.Int64("seed", 42, "random seed")
	baseLevel := flag.Float64("base", 500, "base daily demand level")
	trend := flag.Float64("trend", 0.12, "linear trend, demand units/day")
	weeklyAmp := flag.Float64("weekly-amp", 60, "weekly seasonality amplitude")
	yearlyAmp := flag.Float64("yearly-amp", 150, "yearly seasonality amplitude")
	noiseStd := flag.Float64("noise-std", 25, "std-dev of additive Gaussian noise")
	validate := flag.Bool("validate", false, "run data-quality validation on the output CSV after writing it")
	flag.Parse()

	startDate, err := time.Parse("2006-01-02", *start)
	if err != nil {
		log.Fatalf("invalid -start date: %v", err)
	}

	cfg := GenConfig{
		StartDate:      startDate,
		Days:           *days,
		BaseLevel:      *baseLevel,
		Trend:          *trend,
		WeeklyAmp:      *weeklyAmp,
		YearlyAmp:      *yearlyAmp,
		YearlyPhaseDay: 355,
		NoiseStd:       *noiseStd,
		Seed:           *seed,
	}

	obs := Generate(cfg)

	if err := writeCSV(*out, obs); err != nil {
		log.Fatalf("writing CSV: %v", err)
	}
	fmt.Printf("Wrote %d rows to %s (start=%s, seed=%d)\n", len(obs), *out, *start, *seed)

	if *validate {
		report, err := ValidateCSV(*out)
		if err != nil {
			log.Fatalf("validation error: %v", err)
		}
		report.Print()
		if !report.OK() {
			os.Exit(1)
		}
	}
}

func writeCSV(path string, obs []Observation) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"date", "demand"}); err != nil {
		return err
	}
	for _, o := range obs {
		row := []string{
			o.Date.Format("2006-01-02"),
			fmt.Sprintf("%.2f", o.Demand),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
