package clustering2

import (
	"crypto/md5"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/ptracestore"
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
)

var (
	errorNotFound = errors.New("Process not found.")
)

// ClusterRequest is all the info needed to start a k-means clustering run.
type ClusterRequest struct {
	Source string `json:"source"`
	Offset int    `json:"offset"`
	Radius int    `json:"radius"`
	Query  string `json:"query"`
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
	request *ClusterRequest
	git     *gitinfo.GitInfo
	cidl    *cid.CommitIDLookup

	// mutex protects access to the remaining struct members.
	mutex      sync.RWMutex
	response   *ClusterResponse // The response when the clustering is complete.
	lastUpdate time.Time        // The last time this process was updated.
	state      ProcessState     // The current state of the process.
	message    string           // Describes the current state of the process.
}

func newProcess(req *ClusterRequest, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup) *ClusterRequestProcess {
	ret := &ClusterRequestProcess{
		request:    req,
		git:        git,
		cidl:       cidl,
		lastUpdate: time.Now(),
		state:      PROCESS_RUNNING,
		message:    "Running",
	}
	go ret.Run()
	return ret
}

// RunningClusterRequests keeps track of all the ClusterRequestProcess's.
//
// Once a ClusterRequestProcess is complete the results will be kept in memory
// for MAX_FINISHED_PROCESS_AGE before being deleted.
type RunningClusterRequests struct {
	git  *gitinfo.GitInfo
	cidl *cid.CommitIDLookup

	mutex sync.Mutex
	// inProcess maps a ClusterRequest.Id() of the request to the ClusterRequestProcess
	// handling that request.
	inProcess map[string]*ClusterRequestProcess
}

// NewRunningClusterRequests return a new RunningClusterRequests.
func NewRunningClusterRequests(git *gitinfo.GitInfo, cidl *cid.CommitIDLookup) *RunningClusterRequests {
	fr := &RunningClusterRequests{
		git:       git,
		cidl:      cidl,
		inProcess: map[string]*ClusterRequestProcess{},
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
	for _ = range time.Tick(time.Minute) {
		fr.step()
	}
}

// Add starts a new running ClusterRequestProcess and returns
// the ID of the process to be used in calls to Status() and
// Response().
func (fr *RunningClusterRequests) Add(req *ClusterRequest) string {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	id := req.Id()
	if _, ok := fr.inProcess[id]; !ok {
		fr.inProcess[id] = newProcess(req, fr.git, fr.cidl)
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
	glog.Errorf("ClusterRequest failed: %#v %s: %s", *(p.request), message, err)
	p.message = message
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

// Run does the work in a ClusterRequestProcess. It does not return until all the
// work is done or the request failed. Should be run as a Go routine.
func (p *ClusterRequestProcess) Run() {
	cids := []*cid.CommitID{}
	for i := p.request.Offset - p.request.Radius; i <= p.request.Offset+p.request.Radius; i++ {
		cids = append(cids, &cid.CommitID{
			Source: p.request.Source,
			Offset: i,
		})
	}
	parsedQuery, err := url.ParseQuery(p.request.Query)
	if err != nil {
		p.reportError(err, "Invalid URL query.")
		return
	}
	q, err := query.New(parsedQuery)
	if err != nil {
		p.reportError(err, "Invalid Query.")
		return
	}
	df, err := dataframe.NewFromCommitIDsAndQuery(cids, p.cidl, ptracestore.Default, q, p.progress)
	if err != nil {
		p.reportError(err, "Invalid range of commits.")
		return
	}
	n := len(df.TraceSet)
	k := int(math.Floor(math.Sqrt(float64(n))))
	summary, err := CalculateClusterSummaries(df, k, config.MIN_STDDEV, p.clusterProgress)
	if err != nil {
		p.reportError(err, "Invalid clustering.")
		return
	}

	df.TraceSet = ptracestore.TraceSet{}
	frame, err := dataframe.ResponseFromDataFrame(df, p.git, false)
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
