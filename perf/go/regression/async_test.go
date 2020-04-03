package regression

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

const (
	e = vec32.MissingDataSentinel
)

func TestTooMuchMissingData(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		value    types.Trace
		expected bool
		message  string
	}{
		{
			value:    types.Trace{e, e, 1, 1, 1},
			expected: true,
			message:  "missing one side",
		},
		{
			value:    types.Trace{1, e, 1, 1, 1},
			expected: false,
			message:  "exactly 50%",
		},
		{
			value:    types.Trace{1, 1, e, 1, 1},
			expected: true,
			message:  "missing midpoint",
		},
		{
			value:    types.Trace{e, e, 1, 1},
			expected: true,
			message:  "missing one side - even",
		},
		{
			value:    types.Trace{e, 1, 1, 1},
			expected: false,
			message:  "exactly 50% - even",
		},
		{
			value:    types.Trace{e, 1, 1},
			expected: true,
			message:  "Radius = 1",
		},
		{
			value:    types.Trace{1},
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
