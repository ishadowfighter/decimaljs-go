package bench

import (
	"encoding/json"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	decimal "github.com/ishadowfighter/decimaljs-go/src"
)

// The Go side of the benchmark. Each operation is timed individually so a
// latency distribution can be reported rather than only a mean: run with
//
//	go test ./bench/ -run TestMeasure -count 1
//
// which writes bench/results.go.json for bench/run.mjs to merge with the
// JavaScript numbers.

const (
	// Each sample times a batch rather than a single call. The wall clock on
	// Windows advances in steps of roughly a millisecond, so timing a
	// microsecond operation directly reports the clock's resolution rather than
	// the operation; the batch size is chosen per operation so that one sample
	// spans at least minSampleTime. Batching also keeps the compiler from
	// eliminating work whose result is unused.
	samples       = 400
	warmup        = 2000
	minSampleTime = 20 * time.Millisecond
	fileName      = "results.go.json"
)

// sink keeps the benchmarked results reachable so nothing is optimised away.
var sink any

type measurement struct {
	Op         string  `json:"op"`
	Samples    int     `json:"samples"`
	BatchSize  int     `json:"batch_size"`
	MeanNs     float64 `json:"mean_ns"`
	P50Ns      float64 `json:"p50_ns"`
	P99Ns      float64 `json:"p99_ns"`
	MaxNs      float64 `json:"max_ns"`
	OpsPerSec  float64 `json:"ops_per_sec"`
	AllocBytes uint64  `json:"alloc_bytes_per_op"`
}

func TestMeasure(t *testing.T) {
	if os.Getenv("BENCH") == "" {
		t.Skip("set BENCH=1 to measure; this is a benchmark, not a test")
	}

	cfg := decimal.DefaultConfig()
	cfg.Precision = 34 // IEEE 754 decimal128, a realistic working precision
	ctx := decimal.NewContext(cfg)

	mustParse := func(s string) decimal.Decimal {
		d, err := ctx.Parse(s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return d
	}

	a := mustParse("12345.6789012345678901234567890123")
	b := mustParse("9876.54321098765432109876543210987")
	small := mustParse("1.7320508075688772935274463415059")

	ops := []struct {
		name string
		run  func()
	}{
		{"parse", func() { sink, _ = ctx.Parse("12345.6789012345678901234567890123") }},
		{"add", func() { sink = ctx.Add(a, b) }},
		{"sub", func() { sink = ctx.Sub(a, b) }},
		{"mul", func() { sink = ctx.Mul(a, b) }},
		{"div", func() { sink = ctx.Div(a, b) }},
		{"sqrt", func() { sink = ctx.Sqrt(a) }},
		{"ln", func() { sink, _ = ctx.Ln(a) }},
		{"exp", func() { sink = ctx.Exp(small) }},
		{"sin", func() { sink, _ = ctx.Sin(small) }},
		{"toString", func() { sink = ctx.String(a) }},
	}

	results := make([]measurement, 0, len(ops))
	for _, op := range ops {
		for i := 0; i < warmup; i++ {
			op.run()
		}

		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)

		// Grow the batch until a batch is long enough to measure.
		batchSize := 1
		for {
			start := time.Now()
			for j := 0; j < batchSize; j++ {
				op.run()
			}
			if time.Since(start) >= minSampleTime || batchSize >= 1<<20 {
				break
			}
			batchSize *= 2
		}

		times := make([]float64, samples)
		for i := 0; i < samples; i++ {
			start := time.Now()
			for j := 0; j < batchSize; j++ {
				op.run()
			}
			times[i] = float64(time.Since(start).Nanoseconds()) / float64(batchSize)
		}

		runtime.ReadMemStats(&after)
		sort.Float64s(times)

		var total float64
		for _, v := range times {
			total += v
		}
		mean := total / float64(samples)

		results = append(results, measurement{
			Op:         op.name,
			Samples:    samples,
			BatchSize:  batchSize,
			MeanNs:     mean,
			P50Ns:      times[samples/2],
			P99Ns:      times[samples*99/100],
			MaxNs:      times[samples-1],
			OpsPerSec:  1e9 / mean,
			AllocBytes: (after.TotalAlloc - before.TotalAlloc) / uint64(samples*batchSize),
		})
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	out := map[string]any{
		"heap_sys_kb":  mem.Sys / 1024,
		"runtime":      "go " + runtime.Version(),
		"goos":         runtime.GOOS,
		"goarch":       runtime.GOARCH,
		"precision":    cfg.Precision,
		"measurements": results,
	}
	blob, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileName, append(blob, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote bench/%s", fileName)
}
