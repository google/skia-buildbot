package regression

import (
	"context"
	"fmt"
	"math"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfiter"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/types"
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
	maxK = 100
)

// DetectorResponseProcessor is a callback that is called with RegressionDetectionResponses as a RegressionDetectionRequest is being processed.
type DetectorResponseProcessor func(*RegressionDetectionRequest, []*RegressionDetectionResponse, string)

// ParamsetProvider is a function that's called to return the current paramset.
type ParamsetProvider func() paramtools.ReadOnlyParamSet

// RegressionDetectionRequest is all the info needed to start a clustering run,
// an Alert and the Domain over which to run that Alert.
type RegressionDetectionRequest struct {
	Alert  *alerts.Alert `json:"alert"`
	Domain types.Domain  `json:"domain"`

	// Query is the exact query being run. It may be more specific than the one
	// in the Alert if the Alert has a non-empty GroupBy.
	Query string `json:"query"`

	// Step/TotalQueries is the current percent of all the queries that have been processed.
	Step int `json:"step"`

	// TotalQueries is the number of sub-queries to be processed based on the
	// GroupBy setting in the Alert.
	TotalQueries int `json:"total_queries"`

	// Progress of the detection request.
	Progress progress.Progress `json:"-"`
}

// NewRegressionDetectionRequest returns a new RegressionDetectionRequest.
func NewRegressionDetectionRequest() *RegressionDetectionRequest {
	return &RegressionDetectionRequest{
		Progress: progress.New(),
	}
}

// RegressionDetectionResponse is the response from running a RegressionDetectionRequest.
type RegressionDetectionResponse struct {
	Summary *clustering2.ClusterSummaries `json:"summary"`
	Frame   *dataframe.FrameResponse      `json:"frame"`
}

// regressionDetectionProcess handles the processing of a single RegressionDetectionRequest.
type regressionDetectionProcess struct {
	// These members are read-only, should not be modified.
	request                   *RegressionDetectionRequest
	perfGit                   *perfgit.Git
	iter                      dfiter.DataFrameIterator
	detectorResponseProcessor DetectorResponseProcessor
	shortcutStore             shortcut.Store
}

// ProcessRegressions detects regressions given the RegressionDetectionRequest.
func ProcessRegressions(ctx context.Context,
	req *RegressionDetectionRequest,
	detectorResponseProcessor DetectorResponseProcessor,
	perfGit *perfgit.Git,
	shortcutStore shortcut.Store,
	dfBuilder dataframe.DataFrameBuilder,
	ps paramtools.ReadOnlyParamSet,
) error {
	req.Query = req.Alert.Query
	allRequests := allRequestsFromBaseRequest(req, ps)
	for _, req := range allRequests {
		req.Progress.Message("Stage", "Loading data to analyze")
		// Create a single large dataframe then chop it into 2*radius+1 length sub-dataframes in the iterator.
		sklog.Infof("Building DataFrameIterator for %q", req.Query)
		iter, err := dfiter.NewDataFrameIterator(ctx, req.Progress, dfBuilder, perfGit, nil, req.Query, req.Domain, req.Alert)
		if err != nil {
			sklog.Warningf("Failed to create iterator for query: %q: %s", req.Query, err)
			continue
		}
		detectionProcess := &regressionDetectionProcess{
			request:                   req,
			perfGit:                   perfGit,
			detectorResponseProcessor: detectorResponseProcessor,
			shortcutStore:             shortcutStore,
			iter:                      iter,
		}
		detectionProcess.iter = iter
		if err := detectionProcess.run(ctx); err != nil {
			return skerr.Wrapf(err, "Failed to run a sub-query: %q", req.Query)
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
func allRequestsFromBaseRequest(req *RegressionDetectionRequest, ps paramtools.ReadOnlyParamSet) []*RegressionDetectionRequest {
	ret := []*RegressionDetectionRequest{}

	if req.Alert.GroupBy == "" {
		ret = append(ret, req)
	} else {
		queries, err := req.Alert.QueriesFromParamset(ps)
		if err != nil {
			sklog.Errorf("Failed to build GroupBy combinations: %s", err)
			return ret
		}
		sklog.Infof("Config expanded into %d queries.", len(queries))
		for _, q := range queries {
			reqCopy := *req
			reqCopy.Query = q
			ret = append(ret, &reqCopy)
		}
	}

	return ret
}

// reportError records the reason a RegressionDetectionProcess failed.
func (p *regressionDetectionProcess) reportError(err error, message string) error {
	sklog.Warningf("RegressionDetectionRequest failed: %#v %s: %s", *(p.request), message, err)
	return skerr.Wrapf(err, message)
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
func (p *regressionDetectionProcess) shortcutFromKeys(summary *clustering2.ClusterSummaries) error {
	var err error
	for _, cs := range summary.Clusters {
		if cs.Shortcut, err = p.shortcutStore.InsertShortcut(context.Background(), &shortcut.Shortcut{Keys: cs.Keys}); err != nil {
			return err
		}
	}
	return nil
}

// run does the work in a RegressionDetectionProcess. It does not return until all the
// work is done or the request failed. Should be run as a Go routine.
func (p *regressionDetectionProcess) run(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "regressionDetectionProcess.run")
	defer span.End()

	if p.request.Alert.Algo == "" {
		p.request.Alert.Algo = types.KMeansGrouping
	}
	for p.iter.Next() {
		df, err := p.iter.Value(ctx)
		if err != nil {
			return p.reportError(err, "Failed to get DataFrame from DataFrameIterator.")
		}
		sklog.Infof("Next dataframe: %d traces", len(df.TraceSet))
		before := len(df.TraceSet)
		// Filter out Traces with insufficient data. I.e. we need 50% or more data
		// on either side of the target commit.
		df.FilterOut(tooMuchMissingData)
		after := len(df.TraceSet)
		message := fmt.Sprintf("Filtered Traces: Num Before: %d Num After: %d Delta: %d", before, after, before-after)
		sklog.Info(message)
		if after == 0 {
			continue
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
		sklog.Infof("Clustering with K=%d", k)

		var summary *clustering2.ClusterSummaries
		switch p.request.Alert.Algo {
		case types.KMeansGrouping:
			summary, err = clustering2.CalculateClusterSummaries(df, k, config.MinStdDev, p.detectionProgress, p.request.Alert.Interesting, p.request.Alert.Step)
		case types.StepFitGrouping:
			summary, err = StepFit(df, k, config.MinStdDev, p.detectionProgress, p.request.Alert.Interesting, p.request.Alert.Step)
		default:
			err = skerr.Fmt("Invalid type of clustering: %s", p.request.Alert.Algo)
		}
		if err != nil {
			return p.reportError(err, "Invalid regression detection.")
		}
		if err := p.shortcutFromKeys(summary); err != nil {
			return p.reportError(err, "Failed to write shortcut for keys.")
		}

		df.TraceSet = types.TraceSet{}
		frame, err := dataframe.ResponseFromDataFrame(ctx, df, p.perfGit, false, p.request.Progress)
		if err != nil {
			return p.reportError(err, "Failed to convert DataFrame to FrameResponse.")
		}

		cr := &RegressionDetectionResponse{
			Summary: summary,
			Frame:   frame,
		}
		p.detectorResponseProcessor(p.request, []*RegressionDetectionResponse{cr}, message)
	}
	// We Finish the process, but record Results. The detectorResponseProcessor
	// callback should add the results to Progress if that's required.
	return nil
}
