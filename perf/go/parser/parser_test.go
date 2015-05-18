package parser

import (
	"math"
	"testing"

	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
)

func newTestContext() *Context {
	tile := types.NewTile()
	t1 := types.NewPerfTraceN(3)
	t1.Params_["os"] = "Ubuntu12"
	t1.Params_["config"] = "8888"
	t1.Values[1] = 1.234
	tile.Traces["t1"] = t1

	t2 := types.NewPerfTraceN(3)
	t2.Params_["os"] = "Ubuntu12"
	t2.Params_["config"] = "gpu"
	t2.Values[1] = 1.236
	tile.Traces["t2"] = t2

	return NewContext(tile)
}

func TestFilter(t *testing.T) {
	ctx := newTestContext()

	testCases := []struct {
		input  string
		length int
	}{
		{`filter("os=Ubuntu12")`, 2},
		{`filter("")`, 2},
		{`filter("config=8888")`, 1},
		{`filter("config=gpu")`, 1},
		{`filter("config=565")`, 0},
	}
	for _, tc := range testCases {
		traces, err := ctx.Eval(tc.input)
		if err != nil {
			t.Fatalf("Failed to run filter %q: %s", tc.input, err)
		}
		if got, want := len(traces), tc.length; got != want {
			t.Errorf("Wrong traces length %q: Got %v Want %v", tc.input, got, want)
		}
	}
}

func TestEvalNoModifyTile(t *testing.T) {
	ctx := newTestContext()

	traces, err := ctx.Eval(`fill(filter("config=8888"))`)
	if err != nil {
		t.Fatalf("Failed to run filter: %s", err)
	}
	// Make sure we made a deep copy of the traces in the Tile.
	if got, want := ctx.Tile.Traces["t1"].(*types.PerfTrace).Values[0], config.MISSING_DATA_SENTINEL; got != want {
		t.Errorf("Tile incorrectly modified: Got %v Want %v", got, want)
	}
	if got, want := traces[0].Values[0], 1.234; got != want {
		t.Errorf("fill() failed: Got %v Want %v", got, want)
	}
}

func TestEvalErrors(t *testing.T) {
	ctx := newTestContext()

	testCases := []string{
		// Invalid forms.
		`filter("os=Ubuntu12"`,
		`filter(")`,
		`"config=8888"`,
		`("config=gpu")`,
		`{}`,
		// Forms that fail internal Func validation.
		`filter(2)`,
		`norm("foo")`,
		`norm(filter(""), "foo")`,
		`ave(2)`,
		`avg(2)`,
		`norm(2)`,
		`fill(2)`,
		`norm()`,
		`ave()`,
		`avg()`,
		`fill()`,
	}
	for _, tc := range testCases {
		_, err := ctx.Eval(tc)
		if err == nil {
			t.Fatalf("Expected %q to fail parsing:", tc)
		}
	}
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestNorm(t *testing.T) {
	ctx := newTestContext()
	ctx.Tile.Traces["t1"].(*types.PerfTrace).Values = []float64{2.0, -2.0, 1e100}
	delete(ctx.Tile.Traces, "t2")
	traces, err := ctx.Eval(`norm(filter(""))`)
	if err != nil {
		t.Fatalf("Failed to eval norm() test: %s", err)
	}

	if got, want := traces[0].Values[0], 1.0; !near(got, want) {
		t.Errorf("Distance mismatch: Got %v Want %v Full %#v", got, want, traces[0].Values)
	}
}

func TestAve(t *testing.T) {
	ctx := newTestContext()
	ctx.Tile.Traces["t1"].(*types.PerfTrace).Values = []float64{1.0, -1.0, 1e100, 1e100}
	ctx.Tile.Traces["t2"].(*types.PerfTrace).Values = []float64{1e100, 2.0, -2.0, 1e100}
	traces, err := ctx.Eval(`ave(filter(""))`)
	if err != nil {
		t.Fatalf("Failed to eval ave() test: %s", err)
	}
	if got, want := len(traces), 1; got != want {
		t.Errorf("ave() returned wrong length: Got %v Want %v", got, want)
	}

	for i, want := range []float64{1.0, 0.5, -2.0, 1e100} {
		if got := traces[0].Values[i]; !near(got, want) {
			t.Errorf("Distance mismatch: Got %v Want %v", got, want)
		}
	}
}

func TestAvg(t *testing.T) {
	ctx := newTestContext()
	ctx.Tile.Traces["t1"].(*types.PerfTrace).Values = []float64{1.0, -1.0, 1e100, 1e100}
	ctx.Tile.Traces["t2"].(*types.PerfTrace).Values = []float64{1e100, 2.0, -2.0, 1e100}
	traces, err := ctx.Eval(`avg(filter(""))`)
	if err != nil {
		t.Fatalf("Failed to eval avg() test: %s", err)
	}
	if got, want := len(traces), 1; got != want {
		t.Errorf("avg() returned wrong length: Got %v Want %v", got, want)
	}

	for i, want := range []float64{1.0, 0.5, -2.0, 1e100} {
		if got := traces[0].Values[i]; !near(got, want) {
			t.Errorf("Distance mismatch: Got %v Want %v", got, want)
		}
	}
}

func TestCount(t *testing.T) {
	ctx := newTestContext()
	ctx.Tile.Traces["t1"].(*types.PerfTrace).Values = []float64{1.0, -1.0, 1e100, 1e100}
	ctx.Tile.Traces["t2"].(*types.PerfTrace).Values = []float64{1e100, 2.0, -2.0, 1e100}
	traces, err := ctx.Eval(`count(filter(""))`)
	if err != nil {
		t.Fatalf("Failed to eval count() test: %s", err)
	}
	if got, want := len(traces), 1; got != want {
		t.Errorf("count() returned wrong length: Got %v Want %v", got, want)
	}

	for i, want := range []float64{1.0, 2.0, 1.0, 0.0} {
		if got := traces[0].Values[i]; !near(got, want) {
			t.Errorf("Distance mismatch: Got %v Want %v", got, want)
		}
	}
}

func TestRatio(t *testing.T) {
	ctx := newTestContext()
	ctx.Tile.Traces["t1"].(*types.PerfTrace).Values = []float64{10, 4, 100, 50, 9999, 0}
	ctx.Tile.Traces["t2"].(*types.PerfTrace).Values = []float64{5, 2, 4, 5, 0, 1000}

	traces, err := ctx.Eval(`ratio(
                ave(fill(filter("config=gpu"))),
                ave(fill(filter("config=8888"))))`)
	if err != nil {
		t.Fatalf("Failed to eval ratio() test: %s", err)
	}
	if got, want := len(traces), 1; got != want {
		t.Errorf("ratio() returned wrong length: Got %v Want %v", got, want)
	}
	for i, want := range []float64{0.5, 0.5, 0.04, 0.1, 0, 1e+100} {
		if got := traces[0].Values[i]; got != want {
			t.Errorf("Ratio mismatch: Got %v Want %v", got, want)
		}
	}
}

func TestFill(t *testing.T) {
	ctx := newTestContext()
	ctx.Tile.Traces["t1"].(*types.PerfTrace).Values = []float64{1e100, 1e100, 2, 3, 1e100, 5}
	delete(ctx.Tile.Traces, "t2")
	traces, err := ctx.Eval(`fill(filter("config=8888"))`)
	if err != nil {
		t.Fatalf("Failed to eval fill() test: %s", err)
	}
	if got, want := len(traces), 1; got != want {
		t.Errorf("fill() returned wrong length: Got %v Want %v", got, want)
	}

	for i, want := range []float64{2, 2, 2, 3, 5, 5} {
		if got := traces[0].Values[i]; !near(got, want) {
			t.Errorf("Distance mismatch: Got %v Want %v", got, want)
		}
	}
}

func TestSum(t *testing.T) {
	ctx := newTestContext()
	ctx.Tile.Traces["t1"].(*types.PerfTrace).Values = []float64{1.0, -1.0, 1e100, 1e100}
	ctx.Tile.Traces["t2"].(*types.PerfTrace).Values = []float64{1e100, 2.0, -2.0, 1e100}
	traces, err := ctx.Eval(`sum(filter(""))`)
	if err != nil {
		t.Fatalf("Failed to eval sum() test: %s", err)
	}
	if got, want := len(traces), 1; got != want {
		t.Errorf("sum() returned wrong length: Got %v Want %v", got, want)
	}

	for i, want := range []float64{1.0, 1.0, -2.0, 1e100} {
		if got := traces[0].Values[i]; !near(got, want) {
			t.Errorf("Distance mismatch: Got %v Want %v", got, want)
		}
	}
}

func TestGeo(t *testing.T) {
	ctx := newTestContext()
	ctx.Tile.Traces["t1"].(*types.PerfTrace).Values = []float64{1.0, -1.0, 2.0, 1e100}
	ctx.Tile.Traces["t2"].(*types.PerfTrace).Values = []float64{1e100, 2.0, 8.0, -2.0}
	traces, err := ctx.Eval(`geo(filter(""))`)
	if err != nil {
		t.Fatalf("Failed to eval geo() test: %s", err)
	}
	if got, want := len(traces), 1; got != want {
		t.Errorf("geo() returned wrong length: Got %v Want %v", got, want)
	}

	for i, want := range []float64{1.0, 2.0, 4.0, 1e100} {
		if got := traces[0].Values[i]; !near(got, want) {
			t.Errorf("Distance mismatch: Got %v Want %v", got, want)
		}
	}
}

func TestLog(t *testing.T) {
	ctx := newTestContext()
	ctx.Tile.Traces["t1"].(*types.PerfTrace).Values = []float64{1, 10, 100, -1, 0, 1e100}
	ctx.Tile.Traces["t2"].(*types.PerfTrace).Values = []float64{100}
	traces, err := ctx.Eval(`log(filter(""))`)
	if err != nil {
		t.Fatalf("Failed to eval log() test: %s", err)
	}
	if got, want := len(traces), 2; got != want {
		t.Errorf("log() returned wrong length: Got %v Want %v", got, want)
	}

	for i, want := range []float64{0, 1, 2, 1e100, 1e100, 1e100} {
		if got := traces[0].Values[i]; !near(got, want) {
			t.Errorf("Distance mismatch: Got %v Want %v", got, want)
		}
	}
	if got := traces[1].Values[0]; !near(got, 2) {
		t.Errorf("Got %v Want %v", got, 2)
	}
}
