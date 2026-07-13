package regression

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dataframe/mocks"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/types"
)

const (
	e = vec32.MissingDataSentinel
)

var (
	defaultAnomalyConfig = config.AnomalyConfig{}
)

func TestTooMuchMissingData(t *testing.T) {
	// Test cases for OriginalStep (isOriginalStep = true)
	testCasesOriginal := []struct {
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
			value:    types.Trace{e, 1, 1},
			expected: true,
			message:  "Radius = 1, missing left",
		},
		{
			value:    types.Trace{1, 1},
			expected: false, // tooMuchMissingData filters traces with len < 2 to prevent panics. Stricter algorithm-specific length checks (e.g., OriginalStep requires len >= 3) are handled by evaluateSimpleRule.
			message:  "len(tr) < 3 for OriginalStep",
		},
		{
			value:    types.Trace{1},
			expected: true, // Filtered because len(tr) < 2 would cause panic in missing()
			message:  "len(tr) < 2 for OriginalStep",
		},
	}

	for _, tc := range testCasesOriginal {
		t.Run("OriginalStep_"+tc.message, func(t *testing.T) {
			if got, want := tooMuchMissingData(tc.value, true), tc.expected; got != want {
				t.Errorf("Got %v Want %v", got, want)
			}
		})
	}

	// Test cases for non-OriginalStep (isOriginalStep = false)
	testCasesNonOriginal := []struct {
		value    types.Trace
		expected bool
		message  string
	}{
		{
			value:    types.Trace{e, e, 1, 1, 1}, // Odd length 5, truncated to 4 [e, e, 1, 1] -> missing one side -> true
			expected: true,
			message:  "odd length, truncated, missing one side",
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
			value:    types.Trace{1, 1},
			expected: false,
			message:  "length 2, both present",
		},
		{
			value:    types.Trace{e, 1},
			expected: true,
			message:  "length 2, missing left",
		},
		{
			value:    types.Trace{1, e},
			expected: true,
			message:  "length 2, missing right",
		},
		{
			value:    types.Trace{1},
			expected: true,
			message:  "len(tr) < 2",
		},
	}

	for _, tc := range testCasesNonOriginal {
		t.Run("NonOriginalStep_"+tc.message, func(t *testing.T) {
			if got, want := tooMuchMissingData(tc.value, false), tc.expected; got != want {
				t.Errorf("Got %v Want %v", got, want)
			}
		})
	}
}

func TestProcessRegressions_BadQueryValue_ReturnsError(t *testing.T) {
	// TODO(b/451967534) Temporary - remove config.Config modifications after Redis is implemented.
	config.Config = &config.InstanceConfig{}
	config.Config.Experiments = config.Experiments{ProgressUseRedisCache: false}

	alert := alerts.NewConfig() // A known query that will fail to parse.
	alert.Query = "http://[::1]a"
	req := &RegressionDetectionRequest{
		Progress: progress.New(),
		Alert:    alert,
	}

	dfb := &mocks.DataFrameBuilder{}
	err := ProcessRegressions(context.Background(), req, nil, nil, nil, dfb, paramtools.NewReadOnlyParamSet(), ExpandBaseAlertByGroupBy, ReturnOnError, defaultAnomalyConfig, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid query")
	assert.Equal(t, progress.Running, req.Progress.Status())
	var b bytes.Buffer
	err = req.Progress.JSON(&b)
	require.NoError(t, err)
}

func TestAllRequestsFromBaseRequest_WithValidGroupBy_Success(t *testing.T) {

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
	r := NewRegressionDetectionRequest()
	assert.Equal(t, "", r.Query())
}

func TestRegressionDetectionRequestQuery_Alert_ReturnsTheAlertsQueryValue(t *testing.T) {
	r := NewRegressionDetectionRequest()
	r.Alert = alerts.NewConfig()
	r.Alert.Query = "foo"
	assert.Equal(t, r.Alert.Query, r.Query())
}

func TestRegressionDetectionRequestQuery_AlertAndSetQueryCalled_ReturnsTheSetQueryValue(t *testing.T) {
	r := NewRegressionDetectionRequest()
	r.Alert = alerts.NewConfig()
	r.Alert.Query = "foo"
	r.SetQuery("bar")
	assert.Equal(t, "bar", r.Query())
}

func TestDetectRegressionsOnDataFrame_EmptyDataFrame_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	df := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{},
	}

	p := &regressionDetectionProcess{
		request: &RegressionDetectionRequest{
			Progress: progress.New(),
			Alert: &alerts.Alert{
				Radius: 5,
			},
		},
	}

	resp, err := p.detectRegressionsOnDataFrame(ctx, df)
	assert.NoError(t, err)
	assert.Nil(t, resp)
}

type mockRegressionRefiner struct {
	mock.Mock
}

func (m *mockRegressionRefiner) Process(ctx context.Context, cfg *alerts.Alert, responses []*RegressionDetectionResponse) ([]*ConfirmedRegression, error) {
	args := m.Called(ctx, cfg, responses)
	var r0 []*ConfirmedRegression
	if args.Get(0) != nil {
		r0 = args.Get(0).([]*ConfirmedRegression)
	}
	return r0, args.Error(1)
}

func TestRefineAndReportRegressions_NoResponses_DoesNothing(t *testing.T) {
	oldConfig := config.Config
	config.Config = &config.InstanceConfig{}
	defer func() { config.Config = oldConfig }()

	ctx := context.Background()
	mockRefiner := &mockRegressionRefiner{}

	cfg := &alerts.Alert{Radius: 5}
	p := &regressionDetectionProcess{
		request: &RegressionDetectionRequest{
			Alert:    cfg,
			Progress: progress.New(),
		},
		regressionRefiner: mockRefiner,
	}

	mockRefiner.On("Process", ctx, cfg, ([]*RegressionDetectionResponse)(nil)).Return(([]*ConfirmedRegression)(nil), nil)

	err := p.refineAndReportRegressions(ctx, nil)
	assert.NoError(t, err)
}

func TestRefineAndReportRegressions(t *testing.T) {
	oldConfig := config.Config
	config.Config = &config.InstanceConfig{}
	defer func() { config.Config = oldConfig }()

	ctx := context.Background()
	mockRefiner := &mockRegressionRefiner{}

	cfg := &alerts.Alert{Radius: 5}
	p := &regressionDetectionProcess{
		request: &RegressionDetectionRequest{
			Alert:    cfg,
			Progress: progress.New(),
		},
		regressionRefiner: mockRefiner,
	}

	responses := []*RegressionDetectionResponse{
		{
			Message: "test response",
		},
	}

	confirmedRegressions := []*ConfirmedRegression{
		{
			Summary: &clustering2.ClusterSummaries{},
			Message: "test response confirmed",
		},
	}

	mockRefiner.On("Process", ctx, cfg, responses).Return(confirmedRegressions, nil)

	var handlerCalled bool
	var handledResponses []*ConfirmedRegression

	p.confirmedRegressionHandler = func(ctx context.Context, req *RegressionDetectionRequest, resps []*ConfirmedRegression, message string) error {
		handlerCalled = true
		handledResponses = resps
		return nil
	}

	err := p.refineAndReportRegressions(ctx, responses)
	assert.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.Equal(t, confirmedRegressions, handledResponses)
}

func TestCountErrors(t *testing.T) {
	err1 := errors.New("error one")
	err2 := errors.New("error two")
	err3 := errors.New("error three")
	err4 := errors.New("error four")
	err5 := errors.New("error five")

	// Case 1: Simple standard errors
	assert.Equal(t, 3, CountErrors([]error{err1, err2, err3}))

	// Custom Err simulated with errors.Join
	customErr := errors.Join(err1, err2)

	// Custom Err2 simulated with errors.Join
	customErr2 := errors.Join(customErr, err4)

	// Case 2: Custom interface bundle (simulated)
	assert.Equal(t, 2, CountErrors([]error{customErr}))

	// Case 3: Standard errors.Join multi-error
	stdMulti := errors.Join(err1, err2)
	assert.Equal(t, 2, CountErrors([]error{stdMulti}))

	// Case 4: Hybrid - Custom object (simulated) wrapped inside standard errors.Join
	hybrid := errors.Join(err3, customErr)
	assert.Equal(t, 3, CountErrors([]error{hybrid}))

	// Case 5: Nested standard multi-errors
	nestedStd := errors.Join(err3, stdMulti)
	assert.Equal(t, 3, CountErrors([]error{nestedStd}))

	// Case 6: Hybrid 2 - Custom object (simulated) with nested errors
	hybrid2 := errors.Join(err5, customErr2)
	assert.Equal(t, 4, CountErrors([]error{hybrid2}))

	// Case 7: Count customErr2 (simulated)
	assert.Equal(t, 3, CountErrors([]error{customErr2}))
}

type nilUnwrapErr struct {
	msg string
}

func (e *nilUnwrapErr) Error() string { return e.msg }
func (e *nilUnwrapErr) Unwrap() error { return nil }

func TestCountErrors_NilUnwrap(t *testing.T) {
	// Case 8: Error that implements Unwrap() error but returns nil
	nilErrInstance := &nilUnwrapErr{msg: "defensive nil wrapper"}
	assert.Equal(t, 1, CountErrors([]error{nilErrInstance}))
}
