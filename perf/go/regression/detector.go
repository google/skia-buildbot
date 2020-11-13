package regression

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

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
	// maxFinishedProcessAge is the amount of time to keep a finished
	// RegressionDetectionRequestProcess around before deleting it.
	maxFinishedProcessAge = time.Minute

	// The following limits are just to prevent excessively large or long-running
	// regression detections from being triggered.

	// maxK is the largest K used for clustering.
	maxK = 100
)

var (
	errorNotFound = errors.New("Process not found.")
)

// DetectorResponseProcessor is a callback that is called with RegressionDetectionResponses as a RegressionDetectionRequest is being processed.
type DetectorResponseProcessor func(*RegressionDetectionRequest, []*RegressionDetectionResponse, string)

// ParamsetProvider is a function that's called to return the current paramset.
type ParamsetProvider func() paramtools.ParamSet

// Detector does regression detection.
type Detector interface {
	// Add a new RegressionDetectionRequest.
	//
	// DetectorResponseProcessor can have a nil value.
	Add(context.Context, DetectorResponseProcessor, *RegressionDetectionRequest) (string, error)

	// Status of a running request.
	Status(id string) (ProcessState, string, error)

	// Response from a running request.
	Response(id string) (*RegressionDetectionResponse, error)

	// Responses from a running request.
	Responses(id string) ([]*RegressionDetectionResponse, error)
}

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
}

// Id returns a unique identifier for the request.
func (c *RegressionDetectionRequest) Id() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%#v", *c))))
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
	ctx                       context.Context

	// mutex protects access to the remaining struct members.
	mutex      sync.RWMutex
	response   []*RegressionDetectionResponse // The response when the detection is complete.
	lastUpdate time.Time                      // The last time this process was updated.
	state      ProcessState                   // The current state of the process.
	message    string                         // Describes the current state of the process.
}

// detector keeps track of all the RegressionDetectionProcess's.
//
// Once a RegressionDetectionProcess is complete the results will be kept in memory
// for MAX_FINISHED_PROCESS_AGE before being deleted.
type detector struct {
	perfGit            *perfgit.Git
	defaultInteresting float32 // The threshold to control if a regression is considered interesting.
	dfBuilder          dataframe.DataFrameBuilder
	shortcutStore      shortcut.Store

	mutex sync.Mutex
	// inProcess maps a RegressionDetectionRequest.Id() of the request to the RegressionDetectionProcess
	// handling that request.
	inProcess map[string]*regressionDetectionProcess
}

// NewDetector return a new RegressionDetectionRequests.
func NewDetector(perfGit *perfgit.Git, interesting float32, dfBuilder dataframe.DataFrameBuilder, shortcutStore shortcut.Store) Detector {
	fr := &detector{
		perfGit:            perfGit,
		inProcess:          map[string]*regressionDetectionProcess{},
		defaultInteresting: interesting,
		dfBuilder:          dfBuilder,
		shortcutStore:      shortcutStore,
	}
	go fr.background()
	return fr
}

func (d *detector) newRunningProcess(
	ctx context.Context,
	req *RegressionDetectionRequest,
	detectorResponseProcessor DetectorResponseProcessor,
) (*regressionDetectionProcess, error) {
	ret := &regressionDetectionProcess{
		request:                   req,
		perfGit:                   d.perfGit,
		detectorResponseProcessor: detectorResponseProcessor,
		response:                  []*RegressionDetectionResponse{},
		lastUpdate:                time.Now(),
		state:                     ProcessRunning,
		message:                   "Running",
		shortcutStore:             d.shortcutStore,
		ctx:                       ctx,
	}
	// Create a single large dataframe then chop it into 2*radius+1 length sub-dataframes in the iterator.
	iter, err := dfiter.NewDataFrameIterator(ctx, ret.progress, d.dfBuilder, d.perfGit, nil, req.Query, req.Domain, req.Alert)
	if err != nil {
		return nil, fmt.Errorf("Failed to create iterator: %s", err)
	}
	ret.iter = iter
	go ret.run()

	return ret, nil
}

// step does a single step in cleaning up old RegressionDetectionProcess's.
func (d *detector) step() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	now := time.Now()
	for k, v := range d.inProcess {
		v.mutex.Lock()
		if now.Sub(v.lastUpdate) > maxFinishedProcessAge {
			delete(d.inProcess, k)
		}
		v.mutex.Unlock()
	}
}

// background periodically cleans up old RegressionDetectionProcess's.
func (d *detector) background() {
	d.step()
	for range time.Tick(time.Minute) {
		d.step()
	}
}

// Confirm that detector fulfills the Detector interface.
var _ Detector = (*detector)(nil)

// Add starts a new running RegressionDetectionProcess and returns
// the ID of the process to be used in calls to Status() and
// Response().
func (d *detector) Add(ctx context.Context, detectorResponseProcessor DetectorResponseProcessor, req *RegressionDetectionRequest) (string, error) {
	sklog.Info("detector.Add")
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// We don't support GroupBy so just copy the Query over.
	req.Query = req.Alert.Query
	req.TotalQueries = 1
	if req.Alert.Interesting == 0 {
		req.Alert.Interesting = d.defaultInteresting
	}
	id := req.Id()
	if p, ok := d.inProcess[id]; ok {
		state, _, _ := p.status()
		if state != ProcessRunning {
			delete(d.inProcess, id)
		}
	}
	if detectorResponseProcessor == nil {
		detectorResponseProcessor = func(_ *RegressionDetectionRequest, _ []*RegressionDetectionResponse, _ string) {}
	}
	if _, ok := d.inProcess[id]; !ok {
		proc, err := d.newRunningProcess(ctx, req, detectorResponseProcessor)
		if err != nil {
			return "", err
		}
		d.inProcess[id] = proc
	}
	return id, nil
}

// Status returns the ProcessingState and the message of a
// RegressionDetectionProcess of the given 'id'.
func (d *detector) Status(id string) (ProcessState, string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	p, ok := d.inProcess[id]
	if !ok {
		return ProcessError, "Not Found", errorNotFound
	}
	return p.status()
}

// Response returns the RegressionDetectionResponse of the completed RegressionDetectionProcess.
func (d *detector) Response(id string) (*RegressionDetectionResponse, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	p, ok := d.inProcess[id]
	if !ok {
		return nil, errorNotFound
	}
	return p.getResponse(), nil
}

// Responses returns the RegressionDetectionResponse's of the completed RegressionDetectionProcess.
func (d *detector) Responses(id string) ([]*RegressionDetectionResponse, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	p, ok := d.inProcess[id]
	if !ok {
		return nil, errorNotFound
	}
	return p.responses(), nil
}

// reportError records the reason a RegressionDetectionProcess failed.
func (p *regressionDetectionProcess) reportError(err error, message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	sklog.Warningf("RegressionDetectionRequest failed: %#v %s: %s", *(p.request), message, err)
	p.message = fmt.Sprintf("%s: %s", message, err)
	p.state = ProcessError
	p.lastUpdate = time.Now()
}

// progress records the progress of a RegressionDetectionProcess.
func (p *regressionDetectionProcess) progress(step, totalSteps int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.message = fmt.Sprintf("Querying: %d%%", int(float32(100.0)*float32(step)/float32(totalSteps)))
	p.lastUpdate = time.Now()
}

// detectionProgress records the progress of a RegressionDetectionProcess.
func (p *regressionDetectionProcess) detectionProgress(totalError float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.message = fmt.Sprintf("Regression Total Error: %0.2f", totalError)
	p.lastUpdate = time.Now()
}

// getResponse returns the RegressionDetectionResponse of the completed RegressionDetectionProcess.
func (p *regressionDetectionProcess) getResponse() *RegressionDetectionResponse {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.response[0]
}

// responses returns all the RegressionDetectionResponse's of the RegressionDetectionProcess.
func (p *regressionDetectionProcess) responses() []*RegressionDetectionResponse {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.response
}

// status returns the ProcessingState and the message of a
// RegressionDetectionProcess of the given 'id'.
func (p *regressionDetectionProcess) status() (ProcessState, string, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.state, p.message, nil
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
func (p *regressionDetectionProcess) run() {
	if p.request.Alert.Algo == "" {
		p.request.Alert.Algo = types.KMeansGrouping
	}
	for p.iter.Next() {
		df, err := p.iter.Value(p.ctx)
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
		message := fmt.Sprintf("Filtered Traces: Num Before: %d Num After: %d Delta: %d", before, after, before-after)
		sklog.Info(message)

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
			p.reportError(skerr.Fmt("Invalid type of clustering: %s", p.request.Alert.Algo), "Invalid type of clustering.")
		}
		if err != nil {
			p.reportError(err, "Invalid regression detection.")
			return
		}
		if err := p.shortcutFromKeys(summary); err != nil {
			p.reportError(err, "Failed to write shortcut for keys.")
			return
		}

		df.TraceSet = types.TraceSet{}
		frame, err := dataframe.ResponseFromDataFrame(p.ctx, df, p.perfGit, false)
		if err != nil {
			p.reportError(err, "Failed to convert DataFrame to FrameResponse.")
			return
		}

		cr := &RegressionDetectionResponse{
			Summary: summary,
			Frame:   frame,
		}
		p.response = append(p.response, cr)
		p.detectorResponseProcessor(p.request, []*RegressionDetectionResponse{cr}, message)
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.message = ""
	p.state = ProcessSuccess
}
