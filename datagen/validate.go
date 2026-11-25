package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"
)

// ValidationReport summarizes the data-quality checks run against a CSV file.
type ValidationReport struct {
	Rows           int
	MissingDates   []string
	NegativeValues []string
	SchemaOK       bool
	DuplicateDates []string
	NonNumericRows []string
}

func (r ValidationReport) OK() bool {
	return r.SchemaOK &&
		len(r.MissingDates) == 0 &&
		len(r.NegativeValues) == 0 &&
		len(r.DuplicateDates) == 0 &&
		len(r.NonNumericRows) == 0
}

func (r ValidationReport) Print() {
	fmt.Println("=== Data quality validation report ===")
	fmt.Printf("Rows read:        %d\n", r.Rows)
	fmt.Printf("Schema OK:        %v\n", r.SchemaOK)
	fmt.Printf("Missing dates:    %d\n", len(r.MissingDates))
	fmt.Printf("Duplicate dates:  %d\n", len(r.DuplicateDates))
	fmt.Printf("Negative values:  %d\n", len(r.NegativeValues))
	fmt.Printf("Non-numeric rows: %d\n", len(r.NonNumericRows))
	if len(r.MissingDates) > 0 {
		fmt.Println("  gap(s) at:", r.MissingDates)
	}
	if len(r.NegativeValues) > 0 {
		fmt.Println("  negative at:", r.NegativeValues)
	}
	if r.OK() {
		fmt.Println("Result: PASS")
	} else {
		fmt.Println("Result: FAIL")
	}
}

// ValidateCSV checks a generated CSV for the schema "date,demand" and for
// missing/duplicate calendar dates, non-numeric values, and negative demand.
func ValidateCSV(path string) (ValidationReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return ValidationReport{}, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return ValidationReport{}, err
	}
	if len(records) == 0 {
		return ValidationReport{SchemaOK: false}, nil
	}

	report := ValidationReport{SchemaOK: len(records[0]) == 2 && records[0][0] == "date" && records[0][1] == "demand"}
	rows := records[1:]
	report.Rows = len(rows)

	seen := map[string]bool{}
	var prevDate time.Time
	for i, row := range rows {
		if len(row) != 2 {
			report.NonNumericRows = append(report.NonNumericRows, fmt.Sprintf("row %d", i+1))
			continue
		}
		d, err := time.Parse("2006-01-02", row[0])
		if err != nil {
			report.NonNumericRows = append(report.NonNumericRows, fmt.Sprintf("row %d (bad date)", i+1))
			continue
		}
		var demand float64
		if _, err := fmt.Sscanf(row[1], "%f", &demand); err != nil {
			report.NonNumericRows = append(report.NonNumericRows, fmt.Sprintf("row %d (bad demand)", i+1))
			continue
		}
		if demand < 0 {
			report.NegativeValues = append(report.NegativeValues, row[0])
		}
		if seen[row[0]] {
			report.DuplicateDates = append(report.DuplicateDates, row[0])
		}
		seen[row[0]] = true

		if i > 0 {
			expected := prevDate.AddDate(0, 0, 1)
			if !d.Equal(expected) {
				// record every missing day between prevDate and d
				for g := expected; g.Before(d); g = g.AddDate(0, 0, 1) {
					report.MissingDates = append(report.MissingDates, g.Format("2006-01-02"))
				}
			}
		}
		prevDate = d
	}
	return report, nil
}
