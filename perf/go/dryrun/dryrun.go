// dryrun allows testing an Alert and seeing the regression it would find.
package dryrun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
)

const (
	maxCacheAge     = 5 * time.Minute
	cleanupDuration = time.Second
)

// StartDryRunResponse is the JSON response sent from StartHandler.
type StartDryRunResponse struct {
	ID string `json:"id"`
}

// dryRun is the data stored for each dryrun.
type dryRun struct {
	mutex        sync.Mutex
	whenFinished time.Time
	Finished     bool                                          `json:"finished"`    // True if the dry run is complete.
	Message      string                                        `json:"message"`     // Human readable string describing the dry run state.
	Regressions  map[types.CommitNumber]*regression.Regression `json:"regressions"` // All the regressions found so far.
}

// Requests handles HTTP request for doing dryruns.
type Requests struct {
	detector regression.Detector
	perfGit  *perfgit.Git

	mutex    sync.Mutex
	inFlight map[string]*dryRun
}

// New create a new dryrun Request processor.
func New(perfGit *perfgit.Git, detector regression.Detector) *Requests {
	ret := &Requests{
		detector: detector,
		perfGit:  perfGit,
		inFlight: map[string]*dryRun{},
	}
	// Start a go routine to clean up old dry runs.
	go ret.cleaner()
	return ret
}

// cleanerStep does a single step of cleaner().
func (d *Requests) cleanerStep() {
	cutoff := time.Now().Add(-maxCacheAge)
	d.mutex.Lock()
	defer d.mutex.Unlock()
	for id, running := range d.inFlight {
		// First check on each unfinished request and see if it has completed.
		if !running.Finished {
			state, msg, err := d.detector.Status(id)
			if err != nil {
				sklog.Error("Failed to get status if DryRun: %s", id)
				continue
			}
			if state != regression.ProcessRunning {
				running.mutex.Lock()
				running.Finished = true
				running.whenFinished = time.Now()
				if state == regression.ProcessError {
					running.Message = msg
				} else {
					running.Message = "Finished"
				}
				running.mutex.Unlock()
			}
			continue
		}

		if running.whenFinished.Before(cutoff) {
			delete(d.inFlight, id)
		}
	}
	metrics2.GetInt64Metric("dryrun_inflight", nil).Update(int64(len(d.inFlight)))
}

// cleaner removes old dry runs from inFlight.
func (d *Requests) cleaner() {
	for range time.Tick(cleanupDuration) {
		d.cleanerStep()
	}
}

func (d *Requests) StartHandler(w http.ResponseWriter, r *http.Request) {
	// Do not use r.Context() since this kicks off a background process.
	ctx := context.Background()
	w.Header().Set("Content-Type", "application/json")

	var req regression.RegressionDetectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Could not decode POST body.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "dryrun", req)
	if req.Alert.Query == "" {
		httputils.ReportError(w, fmt.Errorf("Query was empty."), "A Query is required.", http.StatusInternalServerError)
		return
	}
	if err := req.Alert.Validate(); err != nil {
		httputils.ReportError(w, err, "Invalid Alert config.", http.StatusInternalServerError)
		return
	}
	id := req.Id()

	d.mutex.Lock()
	defer d.mutex.Unlock()
	// Look for an existing matching dryrun.
	if p, ok := d.inFlight[id]; ok {
		p.mutex.Lock()
		if p.Finished {
			delete(d.inFlight, id)
		}
		p.mutex.Unlock()
	}
	if _, ok := d.inFlight[id]; !ok {
		running := &dryRun{
			Finished:    false,
			Message:     "Starting Dry Run.",
			Regressions: map[types.CommitNumber]*regression.Regression{},
		}
		d.inFlight[id] = running

		// Create a callback that will be passed each found Regression.
		detectorResponseProcessor := func(queryRequest *regression.RegressionDetectionRequest, clusterResponse []*regression.RegressionDetectionResponse, message string) {
			running.mutex.Lock()
			defer running.mutex.Unlock()
			// Loop over clusterResponse, convert each one to a regression, and merge with running.Regressions.
			for _, cr := range clusterResponse {
				c, reg, err := regression.RegressionFromClusterResponse(ctx, cr, req.Alert, d.perfGit)
				if err != nil {
					running.Message = "Failed to convert to Regression, some data may be missing."
					sklog.Errorf("Failed to convert to Regression: %s", err)
					return
				}
				running.Message = fmt.Sprintf("Step: %d/%d\nQuery: %q\nLooking for regressions in query results.\n  Commit: %d\n  Details: %q", queryRequest.Step+1, queryRequest.TotalQueries, queryRequest.Query, c.CommitNumber, message)
				// We might not have found any regressions.
				if reg.Low == nil && reg.High == nil {
					continue
				}
				if origReg, ok := running.Regressions[c.CommitNumber]; !ok {
					running.Regressions[c.CommitNumber] = reg
				} else {
					running.Regressions[c.CommitNumber] = origReg.Merge(reg)
				}
			}
		}

		_, err := d.detector.Add(ctx, detectorResponseProcessor, &req)
		if err != nil {
			httputils.ReportError(w, err, "Failed to start Dry Run process.", http.StatusInternalServerError)
			return
		}
	}
	resp := StartDryRunResponse{
		ID: id,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// RegressionAtCommit is a Regression found for a specific commit.
type RegressionAtCommit struct {
	CID        perfgit.Commit         `json:"cid"`
	Regression *regression.Regression `json:"regression"`
}

// DryRunStatus is the JSON response sent from StatusHandler.
type DryRunStatus struct {
	Finished    bool                  `json:"finished"`
	Message     string                `json:"message"`
	Regressions []*RegressionAtCommit `json:"regressions"`
}

func (d *Requests) StatusHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Grab the running dryrun.
	running, ok := d.inFlight[id]
	if !ok {
		httputils.ReportError(w, fmt.Errorf("Invalid id: %q", id), "Invalid or expired dry run.", http.StatusInternalServerError)
		return
	}

	status := &DryRunStatus{
		Finished:    running.Finished,
		Message:     running.Message,
		Regressions: []*RegressionAtCommit{},
	}

	// Convert the Running.Regressions into a properly formed DryRunStatus response.
	running.mutex.Lock()
	defer running.mutex.Unlock()
	commitNumbers := []types.CommitNumber{}
	for id := range running.Regressions {
		commitNumbers = append(commitNumbers, id)
	}
	sort.Sort(types.CommitNumberSlice(commitNumbers))

	for _, commitNumber := range commitNumbers {
		details, err := d.perfGit.CommitFromCommitNumber(ctx, commitNumber)
		if err != nil {
			sklog.Errorf("Failed to look up commit %d: %s", commitNumber, err)
			continue
		}
		status.Regressions = append(status.Regressions, &RegressionAtCommit{
			CID:        details,
			Regression: running.Regressions[commitNumber],
		})
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}
