package clustering2

import (
	"testing"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/ptracestore"
)

const (
	e = vec32.MISSING_DATA_SENTINEL
)

func TestTooMuchMissingData(t *testing.T) {
	testutils.SmallTest(t)
	testCases := []struct {
		value    ptracestore.Trace
		expected bool
		message  string
	}{
		{
			value:    ptracestore.Trace{e, e, 1, 1, 1},
			expected: true,
			message:  "missing one side",
		},
		{
			value:    ptracestore.Trace{1, e, 1, 1, 1},
			expected: false,
			message:  "exactly 50%",
		},
		{
			value:    ptracestore.Trace{1, 1, e, 1, 1},
			expected: true,
			message:  "missing midpoint",
		},
		{
			value:    ptracestore.Trace{e, e, 1, 1},
			expected: true,
			message:  "missing one side - even",
		},
		{
			value:    ptracestore.Trace{e, 1, 1, 1},
			expected: false,
			message:  "exactly 50% - even",
		},
		{
			value:    ptracestore.Trace{e, 1, 1},
			expected: true,
			message:  "Radius = 1",
		},
		{
			value:    ptracestore.Trace{1},
			expected: false,
			message:  "len(tr) < 3",
		},
	}

	for _, tc := range testCases {
		if got, want := tooMuchMissingData(tc.value), tc.expected; got != want {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}
