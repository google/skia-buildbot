package regression

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/shortcut2"
	"go.skia.org/infra/perf/go/types"
)

type ProcessState string

const (
	PROCESS_RUNNING ProcessState = "Running"
	PROCESS_SUCCESS ProcessState = "Success"
	PROCESS_ERROR   ProcessState = "Error"
)

type ClusterRequestType int

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
	SPARSE_BLOCK_SEARCH_MULT = 200

	CLUSTERING_REQUEST_TYPE_SINGLE ClusterRequestType = 0 // Do clustering at a single commit.
	CLUSTERING_REQUEST_TYPE_LAST_N ClusterRequestType = 1 // Do clustering over a range of dense commits.
)

var (
	errorNotFound = errors.New("Process not found.")
)

// ClusterRequest is all the info needed to start a clustering run.
type ClusterRequest struct {
	Source      string             `json:"source"`
	Offset      int                `json:"offset"`
	Radius      int                `json:"radius"`
	Query       string             `json:"query"`
	K           int                `json:"k"`
	TZ          string             `json:"tz"`
	Algo        types.ClusterAlgo  `json:"algo"`
	Interesting float32            `json:"interesting"`
	Sparse      bool               `json:"sparse"`
	Type        ClusterRequestType `json:"type"`
	N           int32              `json:"n"`
	End         time.Time          `json:"end"`
}

func (c *ClusterRequest) Id() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%#v", *c))))
}

// ClusterResponse is the response from running clustering over a ClusterRequest.
type ClusterResponse struct {
	Summary *clustering2.ClusterSummaries `json:"summary"`
	Frame   *dataframe.FrameResponse      `json:"frame"`
}

// ClusterRequestProcess handles the processing of a single ClusterRequest.
type ClusterRequestProcess struct {
	// These members are read-only, should not be modified.
	request *ClusterRequest
	git     *gitinfo.GitInfo
	iter    DataFrameIterator

	// mutex protects access to the remaining struct members.
	mutex      sync.RWMutex
	response   []*ClusterResponse // The response when the clustering is complete.
	lastUpdate time.Time          // The last time this process was updated.
	state      ProcessState       // The current state of the process.
	message    string             // Describes the current state of the process.
}

func newProcess(ctx context.Context, req *ClusterRequest, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder) *ClusterRequestProcess {
	ret := &ClusterRequestProcess{
		request:    req,
		git:        git,
		response:   []*ClusterResponse{},
		lastUpdate: time.Now(),
		state:      PROCESS_RUNNING,
		message:    "Running",
	}
	if req.Type == CLUSTERING_REQUEST_TYPE_SINGLE {
		// TODO(jcgregorio) This is awkward and should go away in a future CL.
		ret.iter = NewSingleDataFrameIterator(ret.progress, cidl, git, req, dfBuilder)
	} else {
		// Create a single large dataframe then chop it into 2*radius+1 length sub-dataframes in the iterator.
		iter, err := NewDataFrameIterator(ctx, ret.progress, req, dfBuilder)
		if err != nil {
			sklog.Errorf("Failed to create iterator: %s", err)
			ret.state = PROCESS_ERROR
			ret.message = "Failed to create initial dataframe."
		} else {
			ret.iter = iter
		}
	}
	return ret
}

func newRunningProcess(ctx context.Context, req *ClusterRequest, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder) *ClusterRequestProcess {
	ret := newProcess(ctx, req, git, cidl, dfBuilder)
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

// Responses returns the ClusterResponse's of the completed ClusterRequestProcess.
func (fr *RunningClusterRequests) Responses(id string) ([]*ClusterResponse, error) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	if p, ok := fr.inProcess[id]; !ok {
		return nil, errorNotFound
	} else {
		return p.Responses(), nil
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
	return p.response[0]
}

// Responses returns all the ClusterResponse's of the ClusterRequestProcess.
func (p *ClusterRequestProcess) Responses() []*ClusterResponse {
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

// ShortcutFromKeys stores a new shortcut for each cluster based on its Keys.
func ShortcutFromKeys(summary *clustering2.ClusterSummaries) error {
	var err error
	for _, cs := range summary.Clusters {
		if cs.Shortcut, err = shortcut2.InsertShortcut(&shortcut2.Shortcut{Keys: cs.Keys}); err != nil {
			return err
		}
	}
	return nil
}

// Run does the work in a ClusterRequestProcess. It does not return until all the
// work is done or the request failed. Should be run as a Go routine.
func (p *ClusterRequestProcess) Run(ctx context.Context) {
	if p.request.Algo == "" {
		p.request.Algo = types.KMEANS_ALGO
	}
	for p.iter.Next() {
		df, err := p.iter.Value(ctx)
		if err != nil {
			p.reportError(err, "Failed to get DataFrame from DataFrameIterator.")
			return
		}
		sklog.Infof("Next dataframe: %d traces", len(df.TraceSet))
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

		var summary *clustering2.ClusterSummaries
		switch p.request.Algo {
		case types.KMEANS_ALGO:
			summary, err = clustering2.CalculateClusterSummaries(df, k, config.MIN_STDDEV, p.clusterProgress, p.request.Interesting)
		case types.STEPFIT_ALGO:
			summary, err = StepFit(df, k, config.MIN_STDDEV, p.clusterProgress, p.request.Interesting)
		case types.TAIL_ALGO:
			summary, err = Tail(df, k, config.MIN_STDDEV, p.clusterProgress, p.request.Interesting)

		}
		if err != nil {
			p.reportError(err, "Invalid clustering.")
			return
		}
		if err := ShortcutFromKeys(summary); err != nil {
			p.reportError(err, "Failed to write shortcut for keys.")
			return
		}

		df.TraceSet = types.TraceSet{}
		frame, err := dataframe.ResponseFromDataFrame(ctx, df, p.git, false, p.request.TZ)
		if err != nil {
			p.reportError(err, "Failed to convert DataFrame to FrameResponse.")
			return
		}

		p.mutex.Lock()
		p.state = PROCESS_SUCCESS
		p.message = ""
		p.response = append(p.response, &ClusterResponse{
			Summary: summary,
			Frame:   frame,
		})
		p.mutex.Unlock()
	}
}
