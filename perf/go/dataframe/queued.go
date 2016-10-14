package dataframe

import (
	"crypto/md5"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"go.skia.org/infra/go/calc"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ptracestore"
)

type ProcessState string

const (
	PROCESS_RUNNING ProcessState = "Running"
	PROCESS_SUCCESS ProcessState = "Success"
	PROCESS_ERROR   ProcessState = "Error"
)

const (
	MAX_TRACES_IN_RESPONSE = 350
)

var (
	errorNotFound = errors.New("Process not found.")
)

// FrameRequest is used to deserialize JSON frame requests in frameHandler().
type FrameRequest struct {
	Begin    int      `json:"begin"`    // Beginning of time range in Unix timestamp seconds.
	End      int      `json:"end"`      // End of time range in Unix timestamp seconds.
	Formulas []string `json:"formulas"` // The Formulae to evaluate.
	Queries  []string `json:"queries"`  // The queries to perform encoded as a URL query.
	Hidden   []string `json:"hidden"`   // The ids of traces to remove from the response.
}

func (f *FrameRequest) Id() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%x", *f))))
}

// FrameResponse is serialized to JSON as the response to frame requests.
type FrameResponse struct {
	DataFrame *DataFrame    `json:"dataframe"`
	Ticks     []interface{} `json:"ticks"`
	Skps      []int         `json:"skps"`
	Msg       string        `json:"msg"`
}

type FrameRequestProcess struct {
	mutex           sync.Mutex
	git             *gitinfo.GitInfo
	request         *FrameRequest
	response        *FrameResponse
	lastUpdate      time.Time
	state           ProcessState
	message         string
	percentComplete float32
}

func newProcess(req *FrameRequest, git *gitinfo.GitInfo) *FrameRequestProcess {
	ret := &FrameRequestProcess{
		git:        git,
		request:    req,
		lastUpdate: time.Now(),
		state:      PROCESS_RUNNING,
	}
	go ret.Run()
	return ret
}

type RunningFrameRequests struct {
	mutex sync.Mutex

	git *gitinfo.GitInfo
	// inProcess maps an md5 hash of the request to the FrameRequestProcess
	// handling that request.
	inProcess map[string]*FrameRequestProcess
}

func NewRunningFrameRequests(git *gitinfo.GitInfo) *RunningFrameRequests {
	// TODO - Start a Go routine to clean up any lingering processes.
	return &RunningFrameRequests{
		git:       git,
		inProcess: map[string]*FrameRequestProcess{},
	}
}

// Returns the id of the request, to be used for later lookups.
func (fr *RunningFrameRequests) Add(req *FrameRequest) string {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	id := req.Id()
	if _, ok := fr.inProcess[id]; !ok {
		fr.inProcess[id] = newProcess(req, fr.git)
	}
	return id
}

func (fr *RunningFrameRequests) Status(id string) (ProcessState, string, float32, error) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	if p, ok := fr.inProcess[id]; !ok {
		return PROCESS_ERROR, "", 0.0, errorNotFound
	} else {
		return p.state, p.message, p.percentComplete, nil
	}
}

func (fr *RunningFrameRequests) Response(id string) (*FrameResponse, error) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	if p, ok := fr.inProcess[id]; !ok {
		return nil, errorNotFound
	} else {
		return p.response, nil
	}
}

func (p *FrameRequestProcess) reportError(err error, message string) {
	p.mutex.Lock()
	p.mutex.Unlock()
	glog.Errorf("FrameRequest failed: %#v %s: %s", *(p.request), message, err)
	p.message = message
	p.state = PROCESS_ERROR
}

func (p *FrameRequestProcess) Run() {
	begin := time.Unix(int64(p.request.Begin), 0)
	end := time.Unix(int64(p.request.End), 0)

	// Results from all the queries and calcs will be accumulated in this dataframe.
	df := NewEmpty()

	// Queries.
	for _, q := range p.request.Queries {
		if q == "" {
			continue
		}
		newDF, err := p.doSearch(q, begin, end)
		if err != nil {
			p.reportError(err, "Failed to complete query.")
			return
		}
		dfAppend(df, newDF)
	}

	// Formulas.
	for _, formula := range p.request.Formulas {
		if formula == "" {
			continue
		}
		newDF, err := p.doCalc(formula, begin, end)
		if err != nil {
			p.reportError(err, "Failed to complete query.")
			return
		}
		dfAppend(df, newDF)
	}

	// Filter out "Hidden" traces.
	for _, key := range p.request.Hidden {
		delete(df.TraceSet, key)
	}

	if len(df.Header) == 0 {
		df = NewHeaderOnly(p.git, begin, end)
	}

	resp, err := ResponseFromDataFrame(df, p.git)
	if err != nil {
		p.reportError(err, "Failed to get ticks or skps.")
		return
	}
	p.response = resp
}

// getCommitTimesForFile returns a slice of Unix timestamps in seconds that are
// the times that the given file changed in git between the given 'begin' and
// 'end' hashes (inclusive).
func getCommitTimesForFile(begin, end string, filename string, git *gitinfo.GitInfo) []int64 {
	ret := []int64{}

	// Now query for all the changes to the skp version over the given range of commits.
	log, err := git.LogFine(begin+"^", end, "--format=format:%ct", "--", filename)
	if err != nil {
		glog.Errorf("Could not get skp log for %s..%s -- %q: %s", begin, end, filename, err)
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
func getSkps(headers []*ColumnHeader, git *gitinfo.GitInfo) ([]int, error) {
	// We have Offsets, which need to be converted to git hashes.
	ci, err := git.ByIndex(int(headers[0].Offset))
	if err != nil {
		return nil, fmt.Errorf("Could not find commit for index %d: %s", headers[0].Offset, err)
	}
	begin := ci.Hash
	ci, err = git.ByIndex(int(headers[len(headers)-1].Offset))
	if err != nil {
		return nil, fmt.Errorf("Could not find commit for index %d: %s", headers[len(headers)-1].Offset, err)
	}
	end := ci.Hash

	// Now query for all the changes to the skp version over the given range of commits.
	ts := getCommitTimesForFile(begin, end, "infra/bots/assets/skp/VERSION", git)
	// Add in the changes to the old skp version over the given range of commits.
	ts = append(ts, getCommitTimesForFile(begin, end, "SKP_VERSION", git)...)

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

func ResponseFromDataFrame(df *DataFrame, git *gitinfo.GitInfo) (*FrameResponse, error) {
	// Calculate the human ticks based on the column headers.
	ts := []int64{}
	for _, c := range df.Header {
		ts = append(ts, c.Timestamp)
	}
	ticks := human.FlotTickMarks(ts)

	// Determine where SKP changes occurred.
	skps, err := getSkps(df.Header, git)
	if err != nil {
		return nil, fmt.Errorf("Failed to load skps: %s", err)
	}

	// Truncate the result if it's too large.
	msg := ""
	if len(df.TraceSet) > MAX_TRACES_IN_RESPONSE {
		msg = fmt.Sprintf("Response too large, the number of traces returned has been truncated from %d to %d.", len(df.TraceSet), MAX_TRACES_IN_RESPONSE)
		keys := []string{}
		for k, _ := range df.TraceSet {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		keys = keys[:MAX_TRACES_IN_RESPONSE]
		newTraceSet := ptracestore.TraceSet{}
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

// doSeach applies the given query and returns a dataframe that matches the
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
	return NewFromQueryAndRange(p.git, ptracestore.Default, begin, end, q)
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
		df, err = NewFromQueryAndRange(p.git, ptracestore.Default, begin, end, q)
		if err != nil {
			return nil, err
		}
		// DataFrames are float32, but calc does its work in float64.
		rows := calc.Rows{}
		for k, v := range df.TraceSet {
			rows[k] = to64(v)
		}
		return rows, nil
	}

	ctx := calc.NewContext(rowsFromQuery)
	rows, err := ctx.Eval(formula)
	if err != nil {
		return nil, fmt.Errorf("Calculation failed: %s", err)
	}

	// Convert the Rows from float64 to float32 for DataFrame.
	ts := ptracestore.TraceSet{}
	for k, v := range rows {
		ts[k] = to32(v)
	}
	df.TraceSet = ts

	// Clear the paramset since we are returning calculated values.
	df.ParamSet = paramtools.ParamSet{}

	return df, nil
}

// dfAppend appends the paramset and traceset of 'b' to 'a'.
//
// Also, if a has to Header then it uses b's Header.
// Assumes that a and b both have the same Header, or that
// 'a' is an empty DataFrame.
func dfAppend(a, b *DataFrame) {
	if len(a.Header) == 0 {
		a.Header = b.Header
	}
	a.ParamSet.AddParamSet(b.ParamSet)
	for k, v := range b.TraceSet {
		a.TraceSet[k] = v
	}
}

func to64(a []float32) []float64 {
	ret := make([]float64, len(a), len(a))
	for i, x := range a {
		if x == ptracestore.MISSING_VALUE {
			ret[i] = ptracestore.MISSING_VALUE
		} else {
			ret[i] = float64(x)
		}
	}
	return ret
}

func to32(a []float64) []float32 {
	ret := make([]float32, len(a), len(a))
	for i, x := range a {
		ret[i] = float32(x)
	}
	return ret
}
