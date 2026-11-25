# datagen

A small, dependency-free Go CLI that generates a synthetic daily demand/sales
series and validates the resulting CSV for data-quality issues. See the full
project README for how this fits into the research pipeline.

## Generative model

```
y(t) = level + trend*t + weekly(dow(t)) + yearly(doy(t)) + noise(t), floored at 0
```

- `level` — base daily demand
- `trend*t` — linear growth per day
- `weekly(·)` — fixed per-day-of-week multiplier (weekends higher, midweek lower)
- `yearly(·)` — cosine wave, period 365.25 days, phased to peak in late December
- `noise(t)` — Gaussian white noise

See the doc comment in `generator.go` for the exact parameters.

## Usage

```bash
go build -o datagen .
./datagen -out ../data/demand.csv -days 1095 -start 2023-01-01 -seed 42 -validate
```

Flags: `-out`, `-days`, `-start`, `-seed`, `-base`, `-trend`, `-weekly-amp`,
`-yearly-amp`, `-noise-std`, `-validate` (runs the data-quality report after
writing the CSV and exits non-zero on failure).

## Tests

```bash
go test ./... -v
```

Covers: output length, non-negativity, date contiguity, determinism given a
fixed seed, and that the weekly/yearly seasonal components are statistically
present in the generated output (mean comparison across day-of-week / time-of-year
buckets, with noise reduced to isolate the signal).
