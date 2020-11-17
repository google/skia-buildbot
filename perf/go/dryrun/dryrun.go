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

	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut"
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
	perfGit       *perfgit.Git
	shortcutStore shortcut.Store
	dfBuilder     dataframe.DataFrameBuilder
	tracker       progress.Tracker
}

// New create a new dryrun Request processor.
func New(perfGit *perfgit.Git, tracker progress.Tracker, shortcutStore shortcut.Store, dfBuilder dataframe.DataFrameBuilder) *Requests {
	ret := &Requests{
		perfGit:       perfGit,
		shortcutStore: shortcutStore,
		dfBuilder:     dfBuilder,
		tracker:       tracker,
	}
	return ret
}

// StartHandler starts a dryrun.
func (d *Requests) StartHandler(w http.ResponseWriter, r *http.Request) {
	// Do not use r.Context() since this kicks off a background process.
	ctx := context.Background()
	w.Header().Set("Content-Type", "application/json")

	req := regression.NewRegressionDetectionRequest()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
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

	d.tracker.Add(req.Progress)

	// Look for an existing matching dryrun.
	running := &dryRun{
		Finished:    false,
		Message:     "Starting Dry Run.",
		Regressions: map[types.CommitNumber]*regression.Regression{},
	}

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
			req.Progress.Message("Step", fmt.Sprintf("%d/%d", queryRequest.Step+1, queryRequest.TotalQueries))
			req.Progress.Message("Query", fmt.Sprintf("%q", queryRequest.Query))
			req.Progress.Message("Stage", "Looking for regressions in query results.")
			req.Progress.Message("Commit", fmt.Sprintf("%d", c.CommitNumber))
			req.Progress.Message("Details", message)
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

		// Now update the Progress.
		status := &DryRunStatus{
			Regressions: []*RegressionAtCommit{},
		}
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
		req.Progress.IntermediateResult(status.Regressions)
	}

	if err := regression.NewRunningProcess(ctx, req, detectorResponseProcessor, d.perfGit, d.shortcutStore, d.dfBuilder); err != nil {
		req.Progress.Error()
		req.Progress.Message("Error", "Failed to start Dry Run process.")
	}
	if err := req.Progress.JSON(w); err != nil {
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
	Regressions []*RegressionAtCommit `json:"regressions"`
}
