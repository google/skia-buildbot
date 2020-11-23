package regression

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/dataframe/mocks"
	"go.skia.org/infra/perf/go/progress"
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

func TestProcessRegressions_BadQueryValue_Fails(t *testing.T) {
	unittest.SmallTest(t)

	req := &RegressionDetectionRequest{
		Query:    "http://[::1]a", // A known query that will fail to parse.
		Progress: progress.New(),
	}

	dfb := &mocks.DataFrameBuilder{}
	err := ProcessRegressions(context.Background(), req, nil, nil, nil, dfb)
	require.Error(t, err)
	assert.Equal(t, progress.Running, req.Progress.Status())
	var b bytes.Buffer
	err = req.Progress.JSON(&b)
	require.NoError(t, err)
	assert.Equal(t, "{\"status\":\"Running\",\"messages\":[{\"key\":\"Stage\",\"value\":\"Loading data to analyze\"}],\"url\":\"\"}\n", b.String())
}
