package regression

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
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

func TestProcessRegressions_BadQueryValue_ReturnsNil(t *testing.T) {
	unittest.SmallTest(t)

	alert := alerts.NewConfig() // A known query that will fail to parse.
	alert.Query = "http://[::1]a"
	req := &RegressionDetectionRequest{
		Progress: progress.New(),
		Alert:    alert,
	}

	dfb := &mocks.DataFrameBuilder{}
	err := ProcessRegressions(context.Background(), req, nil, nil, nil, dfb, paramtools.NewReadOnlyParamSet(), ExpandBaseAlertByGroupBy)
	require.NoError(t, err)
	assert.Equal(t, progress.Running, req.Progress.Status())
	var b bytes.Buffer
	err = req.Progress.JSON(&b)
	require.NoError(t, err)
	assert.Contains(t, b.String(), "Failed to find enough data for query")
}

func TestAllRequestsFromBaseRequest_WithValidGroupBy_Success(t *testing.T) {
	unittest.SmallTest(t)

	baseRequest := NewRegressionDetectionRequest()
	alert := alerts.NewConfig()
	alert.GroupBy = "config"
	alert.Query = "arch=x86"
	baseRequest.Alert = alert
	ps := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
		"arch":   []string{"x86", "arm"},
	}
	allRequests := allRequestsFromBaseRequest(baseRequest, ps, ExpandBaseAlertByGroupBy)
	assert.Len(t, allRequests, 2)
	assert.Contains(t, []string{"arch=x86&config=8888", "arch=x86&config=565"}, allRequests[0].Query())
}

func TestAllRequestsFromBaseRequest_WithInvalidGroupBy_NoRequestsReturned(t *testing.T) {
	unittest.SmallTest(t)

	baseRequest := NewRegressionDetectionRequest()
	alert := alerts.NewConfig()
	alert.GroupBy = "SomeUnknownKey"
	alert.Query = "arch=x86"
	baseRequest.Alert = alert
	ps := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
		"arch":   []string{"x86", "arm"},
	}
	allRequests := allRequestsFromBaseRequest(baseRequest, ps, ExpandBaseAlertByGroupBy)
	assert.Empty(t, allRequests)
}

func TestAllRequestsFromBaseRequest_WithoutGroupBy_BaseRequestReturnedUnchanged(t *testing.T) {
	unittest.SmallTest(t)

	baseRequest := NewRegressionDetectionRequest()
	alert := alerts.NewConfig()
	alert.GroupBy = ""
	alert.Query = "arch=x86"
	baseRequest.Alert = alert
	ps := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
		"arch":   []string{"x86", "arm"},
	}
	allRequests := allRequestsFromBaseRequest(baseRequest, ps, ExpandBaseAlertByGroupBy)
	// With no GroupBy a slice with just the baseRequest is returned.
	assert.Len(t, allRequests, 1)
	// Intentionally comparing pointers.
	assert.Same(t, baseRequest, allRequests[0])
}

func TestAllRequestsFromBaseRequest_WithGroupBy_DoNoExpandBaseAlertByGroupBySuppressedGroupBy(t *testing.T) {
	unittest.SmallTest(t)

	baseRequest := NewRegressionDetectionRequest()
	alert := alerts.NewConfig()
	alert.GroupBy = "config"
	alert.Query = "arch=x86"
	baseRequest.Alert = alert
	ps := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
		"arch":   []string{"x86", "arm"},
	}
	allRequests := allRequestsFromBaseRequest(baseRequest, ps, DoNotExpandBaseAlertByGroupBy)
	// With no GroupBy a slice with just the baseRequest is returned.
	assert.Len(t, allRequests, 1)
	// Intentionally comparing pointers.
	assert.Equal(t, baseRequest, allRequests[0])
}

func TestRegressionDetectionRequestQuery_NoAlert_ReturnsEmptyQuery(t *testing.T) {
	unittest.SmallTest(t)
	r := NewRegressionDetectionRequest()
	assert.Equal(t, "", r.Query())
}

func TestRegressionDetectionRequestQuery_Alert_ReturnsTheAlertsQueryValue(t *testing.T) {
	unittest.SmallTest(t)
	r := NewRegressionDetectionRequest()
	r.Alert = alerts.NewConfig()
	r.Alert.Query = "foo"
	assert.Equal(t, r.Alert.Query, r.Query())
}

func TestRegressionDetectionRequestQuery_AlertAndSetQueryCalled_ReturnsTheSetQueryValue(t *testing.T) {
	unittest.SmallTest(t)
	r := NewRegressionDetectionRequest()
	r.Alert = alerts.NewConfig()
	r.Alert.Query = "foo"
	r.SetQuery("bar")
	assert.Equal(t, "bar", r.Query())
}
