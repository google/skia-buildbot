package clustering2

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

type ProcessState string

const (
	PROCESS_RUNNING ProcessState = "Running"
	PROCESS_SUCCESS ProcessState = "Success"
	PROCESS_ERROR   ProcessState = "Error"
)

const (
	// MAX_FINISHED_PROCESS_AGE is the amount of time to keep a finished
	// ClusterRequestProcess around before deleting it.
	MAX_FINISHED_PROCESS_AGE = time.Minute

	// The following limits are just to prevent excessively large or long-running
	// clusterings from being triggered.

	// MAX_K is the largest K used for clustering.
	MAX_K = 100

	// MAX_RADIUS  is the maximum number of points on either side of a commit
	// that will be included in clustering. This cannot exceed COMMITS_PER_TILE.
	MAX_RADIUS = 50

	// SPARSE_BLOCK_SEARCH_MULT When searching for commits that have data in a
	// sparse data set, we'll request data in chunks of this many commits per
	// point we are looking for.
	SPARSE_BLOCK_SEARCH_MULT = 100
)

var (
	errorNotFound = errors.New("Process not found.")
)

type ClusterAlgo string

// ClusterAlgo constants.
//
// Update algo-select-sk if this enum is changed.
const (
	KMEANS_ALGO  ClusterAlgo = "kmeans"  // Cluster traces using k-means clustering on their shapes.
	STEPFIT_ALGO ClusterAlgo = "stepfit" // Look at each trace individually and determing if it steps up or down.
	TAIL_ALGO    ClusterAlgo = "tail"    // Whether a trace has a jumping tail (a step in the end)
)

var (
	allClusterAlgos = []ClusterAlgo{KMEANS_ALGO, STEPFIT_ALGO, TAIL_ALGO}
)

func ToClusterAlgo(s string) (ClusterAlgo, error) {
	ret := ClusterAlgo(s)
	for _, c := range allClusterAlgos {
		if c == ret {
			return ret, nil
		}
	}
	return ret, fmt.Errorf("%q is not a valid ClusterAlgo, must be a value in %v", s, allClusterAlgos)
}

// ClusterRequest is all the info needed to start a clustering run.
type ClusterRequest struct {
	Source      string      `json:"source"`
	Offset      int         `json:"offset"`
	Radius      int         `json:"radius"`
	Query       string      `json:"query"`
	K           int         `json:"k"`
	TZ          string      `json:"tz"`
	Algo        ClusterAlgo `json:"algo"`
	Interesting float32     `json:"interesting"`
	Sparse      bool        `json:"sparse"`
}

func (c *ClusterRequest) Id() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%#v", *c))))
}

// ClusterResponse is the response from running clustering over a ClusterRequest.
type ClusterResponse struct {
	Summary *ClusterSummaries        `json:"summary"`
	Frame   *dataframe.FrameResponse `json:"frame"`
}

// ClusterRequestProcess handles the processing of a single ClusterRequest.
type ClusterRequestProcess struct {
	// These members are read-only, should not be modified.
	request   *ClusterRequest
	git       *gitinfo.GitInfo
	cidl      *cid.CommitIDLookup
	dfBuilder dataframe.DataFrameBuilder

	// mutex protects access to the remaining struct members.
	mutex      sync.RWMutex
	response   *ClusterResponse // The response when the clustering is complete.
	lastUpdate time.Time        // The last time this process was updated.
	state      ProcessState     // The current state of the process.
	message    string           // Describes the current state of the process.
	cids       []*cid.CommitID  // The cids to run the clustering over. Calculated in calcCids.
}

func newProcess(req *ClusterRequest, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder) *ClusterRequestProcess {
	return &ClusterRequestProcess{
		request:    req,
		git:        git,
		cidl:       cidl,
		dfBuilder:  dfBuilder,
		lastUpdate: time.Now(),
		state:      PROCESS_RUNNING,
		message:    "Running",
	}
}

func newRunningProcess(ctx context.Context, req *ClusterRequest, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder) *ClusterRequestProcess {
	ret := newProcess(req, git, cidl, dfBuilder)
	go ret.Run(ctx)
	return ret
}

// RunningClusterRequests keeps track of all the ClusterRequestProcess's.
//
// Once a ClusterRequestProcess is complete the results will be kept in memory
// for MAX_FINISHED_PROCESS_AGE before being deleted.
type RunningClusterRequests struct {
	git                *gitinfo.GitInfo
	cidl               *cid.CommitIDLookup
	defaultInteresting float32 // The threshold to control if a cluster is considered interesting.
	dfBuilder          dataframe.DataFrameBuilder

	mutex sync.Mutex
	// inProcess maps a ClusterRequest.Id() of the request to the ClusterRequestProcess
	// handling that request.
	inProcess map[string]*ClusterRequestProcess
}

// NewRunningClusterRequests return a new RunningClusterRequests.
func NewRunningClusterRequests(git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, interesting float32, dfBuilder dataframe.DataFrameBuilder) *RunningClusterRequests {
	fr := &RunningClusterRequests{
		git:                git,
		cidl:               cidl,
		inProcess:          map[string]*ClusterRequestProcess{},
		defaultInteresting: interesting,
		dfBuilder:          dfBuilder,
	}
	go fr.background()
	return fr
}

// step does a single step in cleaning up old ClusterRequestProcess's.
func (fr *RunningClusterRequests) step() {
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

// background periodically cleans up old ClusterRequestProcess's.
func (fr *RunningClusterRequests) background() {
	fr.step()
	for range time.Tick(time.Minute) {
		fr.step()
	}
}

// Add starts a new running ClusterRequestProcess and returns
// the ID of the process to be used in calls to Status() and
// Response().
func (fr *RunningClusterRequests) Add(ctx context.Context, req *ClusterRequest) string {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	if req.Interesting == 0 {
		req.Interesting = fr.defaultInteresting
	}
	id := req.Id()
	if p, ok := fr.inProcess[id]; ok {
		state, _, _ := p.Status()
		if state != PROCESS_RUNNING {
			delete(fr.inProcess, id)
		}
	}
	if _, ok := fr.inProcess[id]; !ok {
		fr.inProcess[id] = newRunningProcess(ctx, req, fr.git, fr.cidl, fr.dfBuilder)
	}
	return id
}

// Status returns the ProcessingState and the message of a
// ClusterRequestProcess of the given 'id'.
func (fr *RunningClusterRequests) Status(id string) (ProcessState, string, error) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	if p, ok := fr.inProcess[id]; !ok {
		return PROCESS_ERROR, "Not Found", errorNotFound
	} else {
		return p.Status()
	}
}

// Response returns the ClusterResponse of the completed ClusterRequestProcess.
func (fr *RunningClusterRequests) Response(id string) (*ClusterResponse, error) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	if p, ok := fr.inProcess[id]; !ok {
		return nil, errorNotFound
	} else {
		return p.Response(), nil
	}
}

// reportError records the reason a ClusterRequestProcess failed.
func (p *ClusterRequestProcess) reportError(err error, message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	sklog.Warningf("ClusterRequest failed: %#v %s: %s", *(p.request), message, err)
	p.message = fmt.Sprintf("%s: %s", message, err)
	p.state = PROCESS_ERROR
	p.lastUpdate = time.Now()
}

// progress records the progress of a ClusterRequestProcess.
func (p *ClusterRequestProcess) progress(step, totalSteps int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.message = fmt.Sprintf("Querying: %d%%", int(float32(100.0)*float32(step)/float32(totalSteps)))
	p.lastUpdate = time.Now()
}

// clusterProgress records the progress of a ClusterRequestProcess.
func (p *ClusterRequestProcess) clusterProgress(totalError float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.message = fmt.Sprintf("Clustering Total Error: %0.2f", totalError)
	p.lastUpdate = time.Now()
}

// Response returns the ClusterResponse of the completed ClusterRequestProcess.
func (p *ClusterRequestProcess) Response() *ClusterResponse {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.response
}

// Status returns the ProcessingState and the message of a
// ClusterRequestProcess of the given 'id'.
func (p *ClusterRequestProcess) Status() (ProcessState, string, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.state, p.message, nil
}

// missing returns true if >50% of the trace is vec32.MISSING_DATA_SENTINEL.
func missing(tr types.Trace) bool {
	count := 0
	for _, x := range tr {
		if x == vec32.MISSING_DATA_SENTINEL {
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
	if tr[n] == vec32.MISSING_DATA_SENTINEL {
		return true
	}
	return missing(tr[:n]) || missing(tr[len(tr)-n:])
}

// CidsWithDataInRange is passed to calcCids, and returns all
// the commit ids in [begin, end) that have data.
type CidsWithDataInRange func(begin, end int) ([]*cid.CommitID, error)

// cidsWithData returns the commit ids in the dataframe that have non-missing
// data in at least one trace.
func cidsWithData(df *dataframe.DataFrame) []*cid.CommitID {
	ret := []*cid.CommitID{}
	for i, h := range df.Header {
		for _, tr := range df.TraceSet {
			if tr[i] != vec32.MISSING_DATA_SENTINEL {
				ret = append(ret, &cid.CommitID{
					Source: h.Source,
					Offset: int(h.Offset),
				})
				break
			}
		}
	}
	return ret
}

// calcCids returns a slice of CommitID's that clustering should be run over.
func calcCids(request *ClusterRequest, v vcsinfo.VCS, cidsWithDataInRange CidsWithDataInRange) ([]*cid.CommitID, error) {
	cids := []*cid.CommitID{}
	if request.Sparse {
		// Sparse means data might not be available for every commit, so we need to scan
		// the data and gather up +/- Radius commits from the target commit that actually
		// do have data.

		// Start by checking center point as a quick exit strategy.
		withData, err := cidsWithDataInRange(request.Offset, request.Offset+1)
		if err != nil {
			return nil, err
		}
		if len(withData) == 0 {
			return nil, fmt.Errorf("No data at the target commit id.")
		}
		cids = append(cids, withData...)

		if request.Algo != TAIL_ALGO {
			// Then check from the target forward in time.
			lastCommit := v.LastNIndex(1)
			lastIndex := lastCommit[0].Index
			finalIndex := request.Offset + 1 + SPARSE_BLOCK_SEARCH_MULT*request.Radius
			if finalIndex > lastIndex {
				finalIndex = lastIndex
			}
			withData, err = cidsWithDataInRange(request.Offset+1, finalIndex)
			if err != nil {
				return nil, err
			}
			if len(withData) < request.Radius {
				return nil, fmt.Errorf("Not enough sparse data after the target commit.")
			}
			cids = append(cids, withData[:request.Radius]...)
		}

		// Finally check backward in time.
		backward := request.Radius
		if request.Algo == TAIL_ALGO {
			backward = 2 * request.Radius
		}
		startIndex := request.Offset - SPARSE_BLOCK_SEARCH_MULT*backward
		withData, err = cidsWithDataInRange(startIndex, request.Offset)
		if err != nil {
			return nil, err
		}
		if len(withData) < backward {
			return nil, fmt.Errorf("Not enough sparse data before the target commit.")
		}
		withData = withData[len(withData)-backward:]
		cids = append(withData, cids...)
	} else {
		if request.Radius <= 0 {
			request.Radius = 1
		}
		if request.Algo != TAIL_ALGO && request.Radius > MAX_RADIUS {
			request.Radius = MAX_RADIUS
		}
		from := request.Offset - request.Radius
		to := request.Offset + request.Radius
		if request.Algo == TAIL_ALGO {
			from = request.Offset - 2*request.Radius
			to = request.Offset
		}
		for i := from; i <= to; i++ {
			cids = append(cids, &cid.CommitID{
				Source: request.Source,
				Offset: i,
			})
		}
	}
	return cids, nil
}

// Run does the work in a ClusterRequestProcess. It does not return until all the
// work is done or the request failed. Should be run as a Go routine.
func (p *ClusterRequestProcess) Run(ctx context.Context) {
	if p.request.Algo == "" {
		p.request.Algo = KMEANS_ALGO
	}
	parsedQuery, err := url.ParseQuery(p.request.Query)
	if err != nil {
		p.reportError(err, "Invalid URL query.")
		return
	}
	q, err := query.New(parsedQuery)
	if err != nil {
		p.reportError(err, "Invalid URL query.")
		return
	}

	// cidsWithDataInRange is a closure that we pass to calcCids that returns
	// the CommitID's that are in the given range of offsets that have
	// data in at least one trace that matches the current query.
	cidsWithDataInRange := func(begin, end int) ([]*cid.CommitID, error) {
		c := []*cid.CommitID{}
		for i := begin; i < end; i++ {
			c = append(c, &cid.CommitID{
				Source: p.request.Source,
				Offset: i,
			})
		}
		df, err := p.dfBuilder.NewFromCommitIDsAndQuery(ctx, c, p.cidl, q, nil)
		if err != nil {
			return nil, fmt.Errorf("Failed to load data searching for commit ids: %s", err)
		}
		return cidsWithData(df), nil
	}

	p.cids, err = calcCids(p.request, p.git, cidsWithDataInRange)
	if err != nil {
		p.reportError(err, "Could not calculate the commits to run a cluster over.")
		return
	}
	df, err := p.dfBuilder.NewFromCommitIDsAndQuery(ctx, p.cids, p.cidl, q, p.progress)
	if err != nil {
		p.reportError(err, "Invalid range of commits.")
		return
	}

	before := len(df.TraceSet)
	// Filter out Traces with insufficient data. I.e. we need 50% or more data
	// on either side of the target commit.
	df.FilterOut(tooMuchMissingData)
	after := len(df.TraceSet)
	sklog.Infof("Filtered Traces: %d %d %d", before, after, before-after)

	k := p.request.K
	if k <= 0 || k > MAX_K {
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
	summary, err := CalculateClusterSummaries(df, k, config.MIN_STDDEV, p.clusterProgress, p.request.Interesting, p.request.Algo)
	if err != nil {
		p.reportError(err, "Invalid clustering.")
		return
	}

	df.TraceSet = types.TraceSet{}
	frame, err := dataframe.ResponseFromDataFrame(ctx, df, p.git, false, p.request.TZ)
	if err != nil {
		p.reportError(err, "Failed to convert DataFrame to FrameResponse.")
		return
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.state = PROCESS_SUCCESS
	p.message = ""
	p.response = &ClusterResponse{
		Summary: summary,
		Frame:   frame,
	}
}
