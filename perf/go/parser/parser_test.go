package parser

import (
	"testing"

	"skia.googlesource.com/buildbot.git/perf/go/types"
)

func newTestContext() *Context {
	tile := types.NewTile()
	t1 := types.NewTrace()
	t1.Params["os"] = "Ubuntu12"
	t1.Params["config"] = "8888"
	t1.Values[1] = 1.234
	tile.Traces["t1"] = t1

	t2 := types.NewTrace()
	t2.Params["os"] = "Ubuntu12"
	t2.Params["config"] = "gpu"
	t2.Values[1] = 1.234
	tile.Traces["t2"] = t2

	return NewContext(tile)
}

func TestEval(t *testing.T) {
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

func TestEvalErrors(t *testing.T) {
	ctx := newTestContext()

	testCases := []string{
		`filter("os=Ubuntu12"`,
		`filter(")`,
		`"config=8888"`,
		`("config=gpu")`,
		`{}`,
	}
	for _, tc := range testCases {
		_, err := ctx.Eval(tc)
		if err == nil {
			t.Fatalf("Expected %q to fail parsing:", tc)
		}
	}
}
