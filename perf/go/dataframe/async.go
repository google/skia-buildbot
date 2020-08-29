package dataframe

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"sync"
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
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/types"
)

type ProcessState string

type RequestType int

const (
	PROCESS_RUNNING ProcessState = "Running"
	PROCESS_SUCCESS ProcessState = "Success"
	PROCESS_ERROR   ProcessState = "Error"

	// Values for FrameRequest.RequestType.
	REQUEST_TIME_RANGE RequestType = 0
	REQUEST_COMPACT    RequestType = 1

	DEFAULT_COMPACT_NUM_COMMITS = 200
)

// AllRequestType is all possible values for a RequestType variable.
var AllRequestType = []RequestType{REQUEST_TIME_RANGE, REQUEST_COMPACT}

const (
	MAX_TRACES_IN_RESPONSE = 350

	// MAX_FINISHED_PROCESS_AGE is the amount of time to keep a finished
	// FrameRequestProcess around before deleting it.
	MAX_FINISHED_PROCESS_AGE = 10 * time.Minute
)

var (
	errorNotFound = errors.New("Process not found.")
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
}

func (f *FrameRequest) Id() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%#v", *f))))
}

// FrameResponse is serialized to JSON as the response to frame requests.
type FrameResponse struct {
	DataFrame *DataFrame `json:"dataframe"`
	Skps      []int      `json:"skps"`
	Msg       string     `json:"msg"`
}

// FrameRequestProcess keeps track of a running Go routine that's
// processing a FrameRequest to build a FrameResponse.
type FrameRequestProcess struct {
	// request is read-only, it should not be modified.
	request *FrameRequest

	// TODO(jcgregorio) Remove ctx from struct.
	ctx context.Context

	perfGit *perfgit.Git

	// dfBuilder builds DataFrame's.
	dfBuilder DataFrameBuilder

	shortcutStore shortcut.Store

	mutex         sync.RWMutex // Protects access to the remaining struct members.
	response      *FrameResponse
	lastUpdate    time.Time    // The last time this process was updated.
	state         ProcessState // The current state of the process.
	message       string       // A longer message if the process fails.
	search        int          // The current search (either Formula or Query) being processed.
	totalSearches int          // The total number of Formulas and Queries in the FrameRequest.
	percent       float32      // The percentage of the searches complete [0.0-1.0].
}

func (fr *RunningFrameRequests) newProcess(ctx context.Context, req *FrameRequest) *FrameRequestProcess {
	numKeys := 0
	if req.Keys != "" {
		numKeys = 1
	}
	ret := &FrameRequestProcess{
		perfGit:       fr.perfGit,
		request:       req,
		lastUpdate:    time.Now(),
		state:         PROCESS_RUNNING,
		totalSearches: len(req.Formulas) + len(req.Queries) + numKeys,
		dfBuilder:     fr.dfBuilder,
		shortcutStore: fr.shortcutStore,
		ctx:           ctx,
	}
	go ret.Run()
	return ret
}

// RunningFrameRequests keeps track of all the FrameRequestProcess's.
//
// Once a FrameRequestProcess is complete the results will be kept in memory
// for MAX_FINISHED_PROCESS_AGE before being deleted.
type RunningFrameRequests struct {
	mutex sync.Mutex

	perfGit *perfgit.Git

	dfBuilder DataFrameBuilder

	shortcutStore shortcut.Store

	// inProcess maps a FrameRequest.Id() of the request to the FrameRequestProcess
	// handling that request.
	inProcess map[string]*FrameRequestProcess
}

func NewRunningFrameRequests(perfGit *perfgit.Git, dfBuilder DataFrameBuilder, shortcutStore shortcut.Store) *RunningFrameRequests {
	fr := &RunningFrameRequests{
		perfGit:       perfGit,
		dfBuilder:     dfBuilder,
		shortcutStore: shortcutStore,

		inProcess: map[string]*FrameRequestProcess{},
	}
	go fr.background()
	return fr
}

// step does a single step in cleaning up old FrameRequestProcess's.
func (fr *RunningFrameRequests) step() {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	now := time.Now()
	for k, v := range fr.inProcess {
		v.mutex.Lock()
		if now.Sub(v.lastUpdate) > MAX_FINISHED_PROCESS_AGE {
			delete(fr.inProcess, k)
		}
		v.mutex.Unlock()
	}
}

// background periodically cleans up old FrameRequestProcess's.
func (fr *RunningFrameRequests) background() {
	fr.step()
	for range time.Tick(time.Minute) {
		fr.step()
	}
}

// Add starts a new running FrameRequestProcess and returns
// the ID of the process to be used in calls to Status() and
// Response().
func (fr *RunningFrameRequests) Add(ctx context.Context, req *FrameRequest) string {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	id := req.Id()
	if _, ok := fr.inProcess[id]; !ok {
		fr.inProcess[id] = fr.newProcess(ctx, req)
	}
	return id
}

// Status returns the ProcessingState, the message, and the percent complete of
// a FrameRequestProcess of the given 'id'.
func (fr *RunningFrameRequests) Status(id string) (ProcessState, string, float32, error) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	if p, ok := fr.inProcess[id]; !ok {
		return PROCESS_ERROR, "", 0.0, errorNotFound
	} else {
		return p.Status()
	}
}

// Response returns the FrameResponse of the completed FrameRequestProcess.
func (fr *RunningFrameRequests) Response(id string) (*FrameResponse, error) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	if p, ok := fr.inProcess[id]; !ok {
		return nil, errorNotFound
	} else {
		return p.Response(), nil
	}
}

// reportError records the reason a FrameRequestProcess failed.
func (p *FrameRequestProcess) reportError(err error, message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	sklog.Errorf("FrameRequest failed: %#v %s: %s", *(p.request), message, err)
	p.message = message
	p.state = PROCESS_ERROR
	p.lastUpdate = time.Now()
}

// progress records the progress of a FrameRequestProcess.
func (p *FrameRequestProcess) progress(step, totalSteps int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.totalSearches != 0 && totalSteps != 0 {
		p.percent = (float32(p.search) + (float32(step) / float32(totalSteps))) / float32(p.totalSearches)
	} else {
		p.percent = 0
	}
	p.lastUpdate = time.Now()
}

// searchInc records the progress of a FrameRequestProcess as it completes each
// Query or Formula.
func (p *FrameRequestProcess) searchInc() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.search += 1
}

// Response returns the FrameResponse of the completed FrameRequestProcess.
func (p *FrameRequestProcess) Response() *FrameResponse {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.response
}

// Status returns the ProcessingState, the message, and the percent complete of
// a FrameRequestProcess of the given 'id'.
func (p *FrameRequestProcess) Status() (ProcessState, string, float32, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.state, p.message, p.percent, nil
}

// Run does the work in a FrameRequestProcess. It does not return until all the
// work is done or the request failed. Should be run as a Go routine.
func (p *FrameRequestProcess) Run() {
	ctx, span := trace.StartSpan(p.ctx, "FrameRequestProcess.Run")
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

	resp, err := ResponseFromDataFrame(ctx, df, p.perfGit, true)
	if err != nil {
		p.reportError(err, "Failed to get skps.")
		return
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.state = PROCESS_SUCCESS
	p.response = resp
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
func ResponseFromDataFrame(ctx context.Context, df *DataFrame, perfGit *perfgit.Git, truncate bool) (*FrameResponse, error) {
	if len(df.Header) == 0 {
		return nil, fmt.Errorf("No commits matched that time range.")
	}

	// Determine where SKP changes occurred.
	skps, err := getSkps(ctx, df.Header, perfGit)
	if err != nil {
		sklog.Errorf("Failed to load skps: %s", err)
	}

	// Truncate the result if it's too large.
	msg := ""
	if truncate && len(df.TraceSet) > MAX_TRACES_IN_RESPONSE {
		msg = fmt.Sprintf("Response too large, the number of traces returned has been truncated from %d to %d.", len(df.TraceSet), MAX_TRACES_IN_RESPONSE)
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
		Msg:       msg,
	}, nil
}

// doSearch applies the given query and returns a dataframe that matches the
// given time range [begin, end) in a DataFrame.
func (p *FrameRequestProcess) doSearch(ctx context.Context, queryStr string, begin, end time.Time) (*DataFrame, error) {
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
func (p *FrameRequestProcess) doKeys(ctx context.Context, keyID string, begin, end time.Time) (*DataFrame, error) {
	keys, err := p.shortcutStore.Get(p.ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("Failed to find that set of keys %q: %s", keyID, err)
	}
	if p.request.RequestType == REQUEST_TIME_RANGE {
		return p.dfBuilder.NewFromKeysAndRange(ctx, keys.Keys, begin, end, true, p.progress)
	} else {
		return p.dfBuilder.NewNFromKeys(p.ctx, end, keys.Keys, p.request.NumCommits, p.progress)
	}
}

// doCalc applies the given formula and returns a dataframe that matches the
// given time range [begin, end) in a DataFrame.
func (p *FrameRequestProcess) doCalc(ctx context.Context, formula string, begin, end time.Time) (*DataFrame, error) {
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
			df, err = p.dfBuilder.NewNFromQuery(p.ctx, end, q, p.request.NumCommits, p.progress)
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
		keys, err := p.shortcutStore.Get(p.ctx, s)
		if err != nil {
			return nil, err
		}
		if p.request.RequestType == REQUEST_TIME_RANGE {
			df, err = p.dfBuilder.NewFromKeysAndRange(ctx, keys.Keys, begin, end, true, p.progress)
		} else {
			df, err = p.dfBuilder.NewNFromKeys(p.ctx, end, keys.Keys, p.request.NumCommits, p.progress)
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
