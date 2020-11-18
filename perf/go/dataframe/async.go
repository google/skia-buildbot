package dataframe

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/calc"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/types"
)

type RequestType int

const (
	// Values for FrameRequest.RequestType.
	REQUEST_TIME_RANGE RequestType = 0
	REQUEST_COMPACT    RequestType = 1

	DEFAULT_COMPACT_NUM_COMMITS = 200
)

// AllRequestType is all possible values for a RequestType variable.
var AllRequestType = []RequestType{REQUEST_TIME_RANGE, REQUEST_COMPACT}

const (
	MAX_TRACES_IN_RESPONSE = 350
)

// FrameRequest is used to deserialize JSON frame requests.
type FrameRequest struct {
	Begin       int         `json:"begin"`       // Beginning of time range in Unix timestamp seconds.
	End         int         `json:"end"`         // End of time range in Unix timestamp seconds.
	Formulas    []string    `json:"formulas"`    // The Formulae to evaluate.
	Queries     []string    `json:"queries"`     // The queries to perform encoded as a URL query.
	Hidden      []string    `json:"hidden"`      // The ids of traces to remove from the response.
	Keys        string      `json:"keys"`        // The id of a list of keys stored via shortcut2.
	TZ          string      `json:"tz"`          // The timezone the request is from. https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/DateTimeFormat/resolvedOptions
	NumCommits  int32       `json:"num_commits"` // If RequestType is REQUEST_COMPACT, then the number of commits to show before End, and Begin is ignored.
	RequestType RequestType `json:"request_type"`

	Progress progress.Progress `json:"-"`
}

// NewFrameRequest returns a new FrameRequest instance.
func NewFrameRequest() *FrameRequest {
	return &FrameRequest{
		Progress: progress.New(),
	}
}

// FrameResponse is serialized to JSON as the response to frame requests.
type FrameResponse struct {
	DataFrame *DataFrame `json:"dataframe"`
	Skps      []int      `json:"skps"`
	Msg       string     `json:"msg"`
}

// frameRequestProcess keeps track of a running Go routine that's
// processing a FrameRequest to build a FrameResponse.
type frameRequestProcess struct {
	// request is read-only, it should not be modified.
	request *FrameRequest

	perfGit *perfgit.Git

	// dfBuilder builds DataFrame's.
	dfBuilder DataFrameBuilder

	shortcutStore shortcut.Store

	search        int     // The current search (either Formula or Query) being processed.
	totalSearches int     // The total number of Formulas and Queries in the FrameRequest.
	percent       float32 // The percentage of the searches complete [0.0-1.0].
}

// StartFrameRequestProcess starts processing a FrameRequest.
func StartFrameRequestProcess(ctx context.Context, req *FrameRequest, perfGit *perfgit.Git, dfBuilder DataFrameBuilder, shortcutStore shortcut.Store) {
	numKeys := 0
	if req.Keys != "" {
		numKeys = 1
	}
	ret := &frameRequestProcess{
		perfGit:       perfGit,
		request:       req,
		totalSearches: len(req.Formulas) + len(req.Queries) + numKeys,
		dfBuilder:     dfBuilder,
		shortcutStore: shortcutStore,
	}
	go ret.Run(ctx)
}

// reportError records the reason a FrameRequestProcess failed.
func (p *frameRequestProcess) reportError(err error, message string) {
	sklog.Errorf("FrameRequest failed: %#v %s: %s", *(p.request), message, err)
	p.request.Progress.Error(message)
}

// progress records the progress of a FrameRequestProcess.
func (p *frameRequestProcess) progress(step, totalSteps int) {
	if p.totalSearches != 0 && totalSteps != 0 {
		p.percent = (float32(p.search) + (float32(step) / float32(totalSteps))) / float32(p.totalSearches)
	} else {
		p.percent = 0
	}
	p.request.Progress.Message("Percent", fmt.Sprintf("%d", int64(p.percent*100.0)))
}

// searchInc records the progress of a FrameRequestProcess as it completes each
// Query or Formula.
func (p *frameRequestProcess) searchInc() {
	p.search += 1
}

// Run does the work in a FrameRequestProcess. It does not return until all the
// work is done or the request failed. Should be run as a Go routine.
func (p *frameRequestProcess) Run(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "FrameRequestProcess.Run")
	defer span.End()

	begin := time.Unix(int64(p.request.Begin), 0)
	end := time.Unix(int64(p.request.End), 0)

	// Results from all the queries and calcs will be accumulated in this dataframe.
	df := NewEmpty()

	// Queries.
	for _, q := range p.request.Queries {
		newDF, err := p.doSearch(ctx, q, begin, end)
		if err != nil {
			p.reportError(err, "Failed to complete query.")
			return
		}
		df = Join(df, newDF)
		p.searchInc()
	}

	// Formulas.
	for _, formula := range p.request.Formulas {
		newDF, err := p.doCalc(ctx, formula, begin, end)
		if err != nil {
			p.reportError(err, "Failed to complete query.")
			return
		}
		df = Join(df, newDF)
		p.searchInc()
	}

	// Keys
	if p.request.Keys != "" {
		newDF, err := p.doKeys(ctx, p.request.Keys, begin, end)
		if err != nil {
			p.reportError(err, "Failed to complete query.")
			return
		}
		df = Join(df, newDF)
	}

	// Filter out "Hidden" traces.
	for _, key := range p.request.Hidden {
		delete(df.TraceSet, key)
	}

	if len(df.Header) == 0 {
		var err error
		df, err = NewHeaderOnly(ctx, p.perfGit, begin, end, true)
		if err != nil {
			p.reportError(err, "Failed to load dataframe.")
			return
		}

	}

	resp, err := ResponseFromDataFrame(ctx, df, p.perfGit, true, p.request.Progress)
	if err != nil {
		p.reportError(err, "Failed to get skps.")
		return
	}

	p.request.Progress.FinishedWithResults(resp)
}

// getSkps returns the indices where the SKPs have been updated given
// the ColumnHeaders.
//
// TODO(jcgregorio) Rename this functionality to something more generic.
func getSkps(ctx context.Context, headers []*ColumnHeader, perfGit *perfgit.Git) ([]int, error) {
	if config.Config.GitRepoConfig.FileChangeMarker == "" {
		return []int{}, nil
	}
	begin := types.CommitNumber(headers[0].Offset)
	end := types.CommitNumber(headers[len(headers)-1].Offset)

	commitNumbers, err := perfGit.CommitNumbersWhenFileChangesInCommitNumberRange(ctx, begin, end, config.Config.GitRepoConfig.FileChangeMarker)
	if err != nil {
		return []int{}, skerr.Wrapf(err, "Failed to find skp changes for range: %d-%d", begin, end)
	}
	ret := make([]int, len(commitNumbers))
	for i, n := range commitNumbers {
		ret[i] = int(n)
	}

	return ret, nil
}

// ResponseFromDataFrame fills out the rest of a FrameResponse for the given DataFrame.
//
// If truncate is true then the number of traces returned is limited.
//
// tz is the timezone, and can be the empty string if the default (Eastern) timezone is acceptable.
func ResponseFromDataFrame(ctx context.Context, df *DataFrame, perfGit *perfgit.Git, truncate bool, progress progress.Progress) (*FrameResponse, error) {
	if len(df.Header) == 0 {
		return nil, fmt.Errorf("No commits matched that time range.")
	}

	// Determine where SKP changes occurred.
	skps, err := getSkps(ctx, df.Header, perfGit)
	if err != nil {
		sklog.Errorf("Failed to load skps: %s", err)
	}

	// Truncate the result if it's too large.
	if truncate && len(df.TraceSet) > MAX_TRACES_IN_RESPONSE {
		progress.Message("Message", fmt.Sprintf("Response too large, the number of traces returned has been truncated from %d to %d.", len(df.TraceSet), MAX_TRACES_IN_RESPONSE))
		keys := []string{}
		for k := range df.TraceSet {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		keys = keys[:MAX_TRACES_IN_RESPONSE]
		newTraceSet := types.TraceSet{}
		for _, key := range keys {
			newTraceSet[key] = df.TraceSet[key]
		}
		df.TraceSet = newTraceSet
	}

	return &FrameResponse{
		DataFrame: df,
		Skps:      skps,
	}, nil
}

// doSearch applies the given query and returns a dataframe that matches the
// given time range [begin, end) in a DataFrame.
func (p *frameRequestProcess) doSearch(ctx context.Context, queryStr string, begin, end time.Time) (*DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "FrameRequestProcess.doSearch")
	defer span.End()

	urlValues, err := url.ParseQuery(queryStr)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse query: %s", err)
	}
	q, err := query.New(urlValues)
	if err != nil {
		return nil, fmt.Errorf("Invalid Query: %s", err)
	}
	if p.request.RequestType == REQUEST_TIME_RANGE {
		return p.dfBuilder.NewFromQueryAndRange(ctx, begin, end, q, true, p.progress)
	} else {
		return p.dfBuilder.NewNFromQuery(ctx, end, q, p.request.NumCommits, p.progress)
	}
}

// doKeys returns a DataFrame that matches the given set of keys given
// the time range [begin, end).
func (p *frameRequestProcess) doKeys(ctx context.Context, keyID string, begin, end time.Time) (*DataFrame, error) {
	keys, err := p.shortcutStore.Get(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("Failed to find that set of keys %q: %s", keyID, err)
	}
	if p.request.RequestType == REQUEST_TIME_RANGE {
		return p.dfBuilder.NewFromKeysAndRange(ctx, keys.Keys, begin, end, true, p.progress)
	} else {
		return p.dfBuilder.NewNFromKeys(ctx, end, keys.Keys, p.request.NumCommits, p.progress)
	}
}

// doCalc applies the given formula and returns a dataframe that matches the
// given time range [begin, end) in a DataFrame.
func (p *frameRequestProcess) doCalc(ctx context.Context, formula string, begin, end time.Time) (*DataFrame, error) {
	// During the calculation 'rowsFromQuery' will be called to load up data, we
	// will capture the dataframe that's created at that time. We only really
	// need df.Headers so it doesn't matter if the calculation has multiple calls
	// to filter(), we can just use the last one returned.
	var df *DataFrame

	rowsFromQuery := func(s string) (calc.Rows, error) {
		urlValues, err := url.ParseQuery(s)
		if err != nil {
			return nil, err
		}
		q, err := query.New(urlValues)
		if err != nil {
			return nil, err
		}
		if p.request.RequestType == REQUEST_TIME_RANGE {
			df, err = p.dfBuilder.NewFromQueryAndRange(ctx, begin, end, q, true, p.progress)
		} else {
			df, err = p.dfBuilder.NewNFromQuery(ctx, end, q, p.request.NumCommits, p.progress)
		}
		if err != nil {
			return nil, err
		}
		// DataFrames are float32, but calc does its work in float64.
		rows := calc.Rows{}
		for k, v := range df.TraceSet {
			rows[k] = vec32.Dup(v)
		}
		return rows, nil
	}

	rowsFromShortcut := func(s string) (calc.Rows, error) {
		keys, err := p.shortcutStore.Get(ctx, s)
		if err != nil {
			return nil, err
		}
		if p.request.RequestType == REQUEST_TIME_RANGE {
			df, err = p.dfBuilder.NewFromKeysAndRange(ctx, keys.Keys, begin, end, true, p.progress)
		} else {
			df, err = p.dfBuilder.NewNFromKeys(ctx, end, keys.Keys, p.request.NumCommits, p.progress)
		}
		if err != nil {
			return nil, err
		}
		// DataFrames are float32, but calc does its work in float64.
		rows := calc.Rows{}
		for k, v := range df.TraceSet {
			rows[k] = vec32.Dup(v)
		}
		return rows, nil
	}

	calcContext := calc.NewContext(rowsFromQuery, rowsFromShortcut)
	rows, err := calcContext.Eval(formula)
	if err != nil {
		return nil, fmt.Errorf("Calculation failed: %s", err)
	}

	// Convert the Rows from float64 to float32 for DataFrame.
	ts := types.TraceSet{}
	for k, v := range rows {
		ts[k] = v
	}
	df.TraceSet = ts

	// Clear the paramset since we are returning calculated values.
	df.ParamSet = paramtools.ParamSet{}

	return df, nil
}
