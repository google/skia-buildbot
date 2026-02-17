package regression

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfiter"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/shortcut"
	types "go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// ProcessState is the state of a RegressionDetectionProcess.
type ProcessState string

const (
	// ProcessRunning means the process is still running.
	ProcessRunning ProcessState = "Running"

	// ProcessSuccess means the process has finished successfully.
	ProcessSuccess ProcessState = "Success"

	// ProcessError means the process has ended on an error.
	ProcessError ProcessState = "Error"
)

// AllProcessState is a list of all ProcessState possible values.
var AllProcessState = []ProcessState{ProcessRunning, ProcessSuccess, ProcessError}

const (
	// The following limits are just to prevent excessively large or long-running
	// regression detections from being triggered.

	// maxK is the largest K used for clustering.
	maxK = 200
)

var (
	timeoutCounter = func(alertName string) metrics2.Counter {
		return metrics2.GetCounter("perf_regression_detection_timeout", map[string]string{"alert": alertName})
	}
)

// ConfirmedRegressionHandler is a callback that is called with ConfirmedRegressions as a RegressionDetectionRequest is being processed.
type ConfirmedRegressionHandler func(context.Context, *RegressionDetectionRequest, []*ConfirmedRegression, string)

// ParamsetProvider is a function that's called to return the current paramset.
type ParamsetProvider func() paramtools.ReadOnlyParamSet

// regressionDetectionProcess handles the processing of a single RegressionDetectionRequest.
type regressionDetectionProcess struct {
	// These members are read-only, should not be modified.
	request                    *RegressionDetectionRequest
	perfGit                    perfgit.Git
	iter                       dfiter.DataFrameIterator
	confirmedRegressionHandler ConfirmedRegressionHandler
	shortcutStore              shortcut.Store
	regressionRefiner          RegressionRefiner
}

// BaseAlertHandling determines how Alerts should be handled by ProcessRegressions.
type BaseAlertHandling int

const (
	// ExpandBaseAlertByGroupBy means that a single Alert should be turned into
	// multiple Alerts based on the GroupBy settings in the Alert.
	ExpandBaseAlertByGroupBy BaseAlertHandling = iota

	// DoNotExpandBaseAlertByGroupBy means that the Alert should not be expanded
	// into multiple Alerts even if it has a non-empty GroupBy value.
	DoNotExpandBaseAlertByGroupBy
)

// Iteration controls how ProcessRegressions deals with errors as it iterates
// across all the DataFrames.
type Iteration int

const (
	// ContinueOnError causes the error to be ignored and iteration continues.
	ContinueOnError Iteration = iota

	// ReturnOnError halts the iteration and returns.
	ReturnOnError
)

// ProcessRegressions detects regressions given the RegressionDetectionRequest.
func ProcessRegressions(ctx context.Context,
	req *RegressionDetectionRequest,
	confirmedRegressionHandler ConfirmedRegressionHandler,
	perfGit perfgit.Git,
	shortcutStore shortcut.Store,
	dfBuilder dataframe.DataFrameBuilder,
	ps paramtools.ReadOnlyParamSet,
	expandBaseRequest BaseAlertHandling,
	iteration Iteration,
	anomalyConfig config.AnomalyConfig,
	dfProvider *dfiter.DfProvider,
	regressionRefiner RegressionRefiner,
) error {
	ctx, span := trace.StartSpan(ctx, "ProcessRegressions")
	defer span.End()

	allRequests := allRequestsFromBaseRequest(req, ps, expandBaseRequest)
	span.AddAttributes(trace.Int64Attribute("num_requests", int64(len(allRequests))))

	metrics2.GetCounter("perf_regression_detection_requests").Inc(1)

	timeoutContext, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	for index, req := range allRequests {
		req.Progress.Message("Requests", fmt.Sprintf("Processing request %d/%d", index, len(allRequests)))
		req.Progress.Message("Stage", "Loading data to analyze")
		// Create a single large dataframe then chop it into 2*radius+1 length sub-dataframes in the iterator.
		req.Progress.Message("Query", req.Query())
		iterErrorCallback := func(msg string) {
			req.Progress.Message("Iteration", msg)
		}

		iter, err := dfiter.NewDataFrameIterator(timeoutContext, req.Progress, dfBuilder, perfGit, iterErrorCallback, req.Query(), req.Domain, req.Alert, anomalyConfig, dfProvider)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				timeoutCounter(req.Alert.DisplayName).Inc(1)
				sklog.Errorf("Failed with timeout. Query: %s, err: %v", req.Query(), err)
			} else if iteration == ContinueOnError {
				// Don't log if we just didn't get enough data.
				if err != dfiter.ErrInsufficientData {
					sklog.Warning(err)
				}
				continue
			}
			return err
		}
		req.Progress.Message("Info", "Data loaded.")

		detectionProcess := &regressionDetectionProcess{
			request:                    req,
			perfGit:                    perfGit,
			confirmedRegressionHandler: confirmedRegressionHandler,
			shortcutStore:              shortcutStore,
			iter:                       iter,
			regressionRefiner:          regressionRefiner,
		}
		if err := detectionProcess.run(timeoutContext); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				timeoutCounter(req.Alert.DisplayName).Inc(1)
				sklog.Errorf("Failed with timeout. Query: %s, err: %v", req.Query(), err)
			}
			return skerr.Wrapf(err, "Failed to run a sub-query: %q", req.Query())
		}
	}
	return nil
}

// allRequestsFromBaseRequest returns all possible requests starting from a base
// request.
//
// An Alert with a non-empty GroupBy will be run as a number of requests with
// more refined queries.
//
// An empty slice will be returned on error.
func allRequestsFromBaseRequest(req *RegressionDetectionRequest, ps paramtools.ReadOnlyParamSet, expandBaseRequest BaseAlertHandling) []*RegressionDetectionRequest {
	ret := []*RegressionDetectionRequest{}

	if req.Alert.GroupBy == "" || expandBaseRequest == DoNotExpandBaseAlertByGroupBy {
		ret = append(ret, req)
	} else {
		queries, err := req.Alert.QueriesFromParamset(ps)
		if err != nil {
			sklog.Errorf("Failed to build GroupBy combinations: %s", err)
			return ret
		}
		for _, q := range queries {
			reqCopy := *req
			reqCopy.SetQuery(q)
			ret = append(ret, &reqCopy)
		}
	}

	return ret
}

// reportError records the reason a RegressionDetectionProcess failed.
func (p *regressionDetectionProcess) reportError(err error, message string) error {
	sklog.Warningf("RegressionDetectionRequest failed: %#v %s: %s", *(p.request), message, err)
	p.request.Progress.Message("Warning", fmt.Sprintf("RegressionDetectionRequest failed: %#v %s: %s", *(p.request), message, err))
	return skerr.Wrapf(err, "%s", message)
}

// progress records the progress of a RegressionDetectionProcess.
func (p *regressionDetectionProcess) progress(step, totalSteps int) {
	p.request.Progress.Message("Querying", fmt.Sprintf("%d%%", int(float32(100.0)*float32(step)/float32(totalSteps))))
}

// detectionProgress records the progress of a RegressionDetectionProcess.
func (p *regressionDetectionProcess) detectionProgress(totalError float64) {
	p.request.Progress.Message("Regression Total Error", fmt.Sprintf("%0.2f", totalError))
}

// missing returns true if >50% of the trace is vec32.MISSING_DATA_SENTINEL.
func missing(tr types.Trace) bool {
	count := 0
	for _, x := range tr {
		if x == vec32.MissingDataSentinel {
			count++
		}
	}
	return (100*count)/len(tr) > 50
}

// tooMuchMissingData returns true if a trace has too many
// MISSING_DATA_SENTINEL values.
//
// The criteria is if there is >50% missing data on either side of the target
// commit, which sits at the center of the trace.
func tooMuchMissingData(tr types.Trace) bool {
	if len(tr) < 3 {
		return false
	}
	n := len(tr) / 2
	if tr[n] == vec32.MissingDataSentinel {
		return true
	}
	return missing(tr[:n]) || missing(tr[len(tr)-n:])
}

// shortcutFromKeys stores a new shortcut for each regression based on its Keys.
func (p *regressionDetectionProcess) shortcutFromKeys(ctx context.Context, summary *clustering2.ClusterSummaries) error {
	var err error
	for _, cs := range summary.Clusters {
		if cs.Shortcut, err = p.shortcutStore.InsertShortcut(ctx, &shortcut.Shortcut{Keys: cs.Keys}); err != nil {
			return err
		}
	}
	return nil
}

// detectRegressionsOnDataFrame takes a single DataFrame and processes it, returning a
// RegressionDetectionResponse if regressions are found, or nil if no regressions
// are found. It returns an error if the processing fails. If an error is
// returned then the response will always be nil.
func (p *regressionDetectionProcess) detectRegressionsOnDataFrame(ctx context.Context, df *dataframe.DataFrame) (*RegressionDetectionResponse, error) {
	p.request.Progress.Message("Gathering", fmt.Sprintf("Next dataframe: %d traces", len(df.TraceSet)))
	before := len(df.TraceSet)
	// Filter out Traces with insufficient data. I.e. we need 50% or more data
	// on either side of the target commit.
	df.FilterOut(tooMuchMissingData)
	after := len(df.TraceSet)
	message := fmt.Sprintf("Filtered Traces: Num Before: %d Num After: %d Delta: %d", before, after, before-after)
	p.request.Progress.Message("Filtering", message)
	if after == 0 {
		return nil, nil
	}

	k := p.request.Alert.K
	if k <= 0 || k > maxK {
		n := len(df.TraceSet)
		// We want K to be around 50 when n = 30000, which has been determined via
		// trial and error to be a good value for the Perf data we are working in. We
		// want K to decrease from  there as n gets smaller, but don't want K to go
		// below 10, so we use a simple linear relation:
		//
		//  k = 40/30000 * n + 10
		//
		k = int(math.Floor((40.0/30000.0)*float64(n) + 10))
	}

	var summary *clustering2.ClusterSummaries
	var err error
	switch p.request.Alert.Algo {
	case types.KMeansGrouping:
		p.request.Progress.Message("K", fmt.Sprintf("%d", k))
		summary, err = clustering2.CalculateClusterSummaries(ctx, df, k, config.MinStdDev, p.detectionProgress, p.request.Alert.Interesting, p.request.Alert.Step)
	case types.StepFitGrouping:
		summary, err = StepFit(ctx, df, k, config.MinStdDev, p.detectionProgress, p.request.Alert.Interesting, p.request.Alert.Step)
	default:
		err = skerr.Fmt("Invalid type of clustering: %s", p.request.Alert.Algo)
	}
	if err != nil {
		return nil, p.reportError(err, "Invalid regression detection.")
	}

	df.TraceSet = types.TraceSet{}
	frame, err := frame.ResponseFromDataFrame(ctx, nil, df, p.perfGit, false, p.request.Progress)
	if err != nil {
		return nil, p.reportError(err, "Failed to convert DataFrame to FrameResponse.")
	}

	cr := &RegressionDetectionResponse{
		Summary: summary,
		Frame:   frame,
		Message: message, // Store the per-dataframe message here.
	}
	return cr, nil
}

// refineAndReportRegressions takes all the responses, runs them through the
// post-processor, and then sends the confirmed regressions to the
// confirmedRegressionHandler.
func (p *regressionDetectionProcess) refineAndReportRegressions(ctx context.Context, allResponses []*RegressionDetectionResponse) error {
	// 1. Pass the complete, raw list and the alert config to the regression refiner.
	// The refiner is now responsible for all filtering.
	confirmedRegressions, err := p.regressionRefiner.Process(ctx, p.request.Alert, allResponses)
	if err != nil {
		return p.reportError(err, "Failed during post-processing step.")
	}

	// // 2. Generate shortcuts for all confirmed regressions.
	for _, resp := range confirmedRegressions {
		if err := p.shortcutFromKeys(ctx, resp.Summary); err != nil {
			return p.reportError(err, "Failed to write shortcut for keys during batch processing.")
		}
	}

	// 3. Now, process the final batch of confirmed results.
	if len(confirmedRegressions) > 0 {
		// The summary message can still refer to the original count for clarity.
		summaryMessage := fmt.Sprintf("Batch processing complete for %d dataframes.", len(allResponses))
		p.confirmedRegressionHandler(ctx, p.request, confirmedRegressions, summaryMessage)
		// The refineAndReportRegressions callback should add the results to Progress if that's required.
	}
	return nil
}

// run does the work in a RegressionDetectionProcess. It does not return until all the
// work is done or the request failed. Should be run as a Go routine.
func (p *regressionDetectionProcess) run(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "regressionDetectionProcess.run")
	defer span.End()

	var allResponses []*RegressionDetectionResponse // This will collect all unfiltered results. These are the responses from anomaly detection, but we are not yet confident that we will save these regressions.

	if p.request.Alert.Algo == "" {
		p.request.Alert.Algo = types.KMeansGrouping
	}
	for p.iter.Next() {
		df, err := p.iter.Value(ctx)
		if err != nil {
			return p.reportError(err, "Failed to get DataFrame from DataFrameIterator.")
		}

		resp, err := p.detectRegressionsOnDataFrame(ctx, df)

		if err != nil {
			sklog.Errorf("Failed to detect regressions on DataFrame: %s", err)
			metrics2.GetCounter("perf_regression_detection_errors", map[string]string{"alert": p.request.Alert.DisplayName}).Inc(1)
			continue
		}
		if resp != nil {
			allResponses = append(allResponses, resp)
		}
	}
	return p.refineAndReportRegressions(ctx, allResponses)
}
