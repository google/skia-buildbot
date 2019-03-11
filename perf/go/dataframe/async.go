package dataframe

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/calc"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/shortcut2"
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
	DataFrame *DataFrame    `json:"dataframe"`
	Ticks     []interface{} `json:"ticks"`
	Skps      []int         `json:"skps"`
	Msg       string        `json:"msg"`
}

// FrameRequestProcess keeps track of a running Go routine that's
// processing a FrameRequest to build a FrameResponse.
type FrameRequestProcess struct {
	// request is read-only, it should not be modified.
	request *FrameRequest

	// vcs is for Git info. The value of the 'vcs' variable should not be
	//   changed, but vcs is Go routine safe.
	vcs vcsinfo.VCS

	// dfBuilder builds DataFrame's.
	dfBuilder DataFrameBuilder

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
		vcs:           fr.vcs,
		request:       req,
		lastUpdate:    time.Now(),
		state:         PROCESS_RUNNING,
		totalSearches: len(req.Formulas) + len(req.Queries) + numKeys,
		dfBuilder:     fr.dfBuilder,
	}
	go ret.Run(ctx)
	return ret
}

// RunningFrameRequests keeps track of all the FrameRequestProcess's.
//
// Once a FrameRequestProcess is complete the results will be kept in memory
// for MAX_FINISHED_PROCESS_AGE before being deleted.
type RunningFrameRequests struct {
	mutex sync.Mutex

	vcs vcsinfo.VCS

	dfBuilder DataFrameBuilder

	// inProcess maps a FrameRequest.Id() of the request to the FrameRequestProcess
	// handling that request.
	inProcess map[string]*FrameRequestProcess
}

func NewRunningFrameRequests(vcs vcsinfo.VCS, dfBuilder DataFrameBuilder) *RunningFrameRequests {
	fr := &RunningFrameRequests{
		vcs:       vcs,
		dfBuilder: dfBuilder,

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
	p.percent = (float32(p.search) + (float32(step) / float32(totalSteps))) / float32(p.totalSearches)
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
func (p *FrameRequestProcess) Run(ctx context.Context) {
	begin := time.Unix(int64(p.request.Begin), 0)
	end := time.Unix(int64(p.request.End), 0)

	// Results from all the queries and calcs will be accumulated in this dataframe.
	df := NewEmpty()

	// Queries.
	for _, q := range p.request.Queries {
		newDF, err := p.doSearch(q, begin, end)
		if err != nil {
			p.reportError(err, "Failed to complete query.")
			return
		}
		df = Join(df, newDF)
		p.searchInc()
	}

	// Formulas.
	for _, formula := range p.request.Formulas {
		newDF, err := p.doCalc(formula, begin, end)
		if err != nil {
			p.reportError(err, "Failed to complete query.")
			return
		}
		df = Join(df, newDF)
		p.searchInc()
	}

	// Keys
	if p.request.Keys != "" {
		newDF, err := p.doKeys(p.request.Keys, begin, end)
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
		df = NewHeaderOnly(p.vcs, begin, end, true)
	}

	resp, err := ResponseFromDataFrame(context.Background(), df, p.vcs, true, p.request.TZ)
	if err != nil {
		p.reportError(err, "Failed to get ticks or skps.")
		return
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.state = PROCESS_SUCCESS
	p.response = resp
}

// getCommitTimesForFile returns a slice of Unix timestamps in seconds that are
// the times that the given file changed in git between the given 'begin' and
// 'end' hashes (inclusive).
func getCommitTimesForFile(ctx context.Context, begin, end string, filename string, vcs vcsinfo.VCS) []int64 {
	ret := []int64{}

	// TODO(jcgregorio): Replace with calls to Gerrit API, only used by the Skia instance of perf.

	// Now query for all the changes to the skp version over the given range of commits.
	log, err := vcs.(*gitinfo.GitInfo).LogFine(ctx, begin+"^", end, "--format=format:%ct", "--", filename)
	if err != nil {
		sklog.Errorf("Could not get skp log for %s..%s -- %q: %s", begin, end, filename, err)
		return ret
	}

	// Parse.
	for _, s := range strings.Split(log, "\n") {
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		ret = append(ret, int64(i))
	}
	return ret
}

// getSkps returns the indices where the SKPs have been updated given
// the ColumnHeaders.
func getSkps(ctx context.Context, headers []*ColumnHeader, vcs vcsinfo.VCS) ([]int, error) {
	// We have Offsets, which need to be converted to git hashes.
	ci, err := vcs.ByIndex(ctx, int(headers[0].Offset))
	if err != nil {
		return nil, fmt.Errorf("Could not find commit for index %d: %s", headers[0].Offset, err)
	}
	begin := ci.Hash
	ci, err = vcs.ByIndex(ctx, int(headers[len(headers)-1].Offset))
	if err != nil {
		return nil, fmt.Errorf("Could not find commit for index %d: %s", headers[len(headers)-1].Offset, err)
	}
	end := ci.Hash

	// Now query for all the changes to the skp version over the given range of commits.
	ts := getCommitTimesForFile(ctx, begin, end, "infra/bots/assets/skp/VERSION", vcs)
	// Add in the changes to the old skp version over the given range of commits.
	ts = append(ts, getCommitTimesForFile(ctx, begin, end, "SKP_VERSION", vcs)...)

	// Sort because they are in reverse order.
	sort.Sort(util.Int64Slice(ts))

	// Now flag all the columns where the skp changes.
	ret := []int{}
	for i, h := range headers {
		if len(ts) == 0 {
			break
		}
		if h.Timestamp >= ts[0] {
			ret = append(ret, i)
			ts = ts[1:]
			if len(ts) == 0 {
				break
			}
			// Coalesce all skp updates for a col into a single index.
			for len(ts) > 0 && h.Timestamp >= ts[0] {
				ts = ts[1:]
			}
		}
	}
	return ret, nil
}

// ResponseFromDataFrame fills out the rest of a FrameResponse for the given DataFrame.
//
// If truncate is true then the number of traces returned is limited.
//
// tz is the timezone, and can be the empty string if the default (Eastern) timezone is acceptable.
func ResponseFromDataFrame(ctx context.Context, df *DataFrame, vcs vcsinfo.VCS, truncate bool, tz string) (*FrameResponse, error) {
	if len(df.Header) == 0 {
		return nil, fmt.Errorf("No commits matched that time range.")
	}
	// Calculate the human ticks based on the column headers.
	ts := []int64{}
	for _, c := range df.Header {
		ts = append(ts, c.Timestamp)
	}
	ticks := human.FlotTickMarks(ts, tz)

	// Determine where SKP changes occurred.
	skps, err := getSkps(ctx, df.Header, vcs)
	if err != nil {
		return nil, fmt.Errorf("Failed to load skps: %s", err)
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
		Ticks:     ticks,
		Skps:      skps,
		Msg:       msg,
	}, nil
}

// doSearch applies the given query and returns a dataframe that matches the
// given time range [begin, end) in a DataFrame.
func (p *FrameRequestProcess) doSearch(queryStr string, begin, end time.Time) (*DataFrame, error) {
	urlValues, err := url.ParseQuery(queryStr)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse query: %s", err)
	}
	q, err := query.New(urlValues)
	if err != nil {
		return nil, fmt.Errorf("Invalid Query: %s", err)
	}
	if p.request.RequestType == REQUEST_TIME_RANGE {
		return p.dfBuilder.NewFromQueryAndRange(begin, end, q, true, p.progress)
	} else {
		return p.dfBuilder.NewNFromQuery(context.Background(), end, q, p.request.NumCommits, p.progress)
	}
}

// doKeys returns a DataFrame that matches the given set of keys given
// the time range [begin, end).
func (p *FrameRequestProcess) doKeys(keyID string, begin, end time.Time) (*DataFrame, error) {
	keys, err := shortcut2.Get(keyID)
	if err != nil {
		return nil, fmt.Errorf("Failed to find that set of keys %q: %s", keyID, err)
	}
	if p.request.RequestType == REQUEST_TIME_RANGE {
		return p.dfBuilder.NewFromKeysAndRange(keys.Keys, begin, end, true, p.progress)
	} else {
		return p.dfBuilder.NewNFromKeys(context.Background(), end, keys.Keys, p.request.NumCommits, p.progress)
	}
}

// doCalc applies the given formula and returns a dataframe that matches the
// given time range [begin, end) in a DataFrame.
func (p *FrameRequestProcess) doCalc(formula string, begin, end time.Time) (*DataFrame, error) {
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
			df, err = p.dfBuilder.NewFromQueryAndRange(begin, end, q, true, p.progress)
		} else {
			df, err = p.dfBuilder.NewNFromQuery(context.Background(), end, q, p.request.NumCommits, p.progress)
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
		keys, err := shortcut2.Get(s)
		if err != nil {
			return nil, err
		}
		if p.request.RequestType == REQUEST_TIME_RANGE {
			df, err = p.dfBuilder.NewFromKeysAndRange(keys.Keys, begin, end, true, p.progress)
		} else {
			df, err = p.dfBuilder.NewNFromKeys(context.Background(), end, keys.Keys, p.request.NumCommits, p.progress)
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

	ctx := calc.NewContext(rowsFromQuery, rowsFromShortcut)
	rows, err := ctx.Eval(formula)
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
