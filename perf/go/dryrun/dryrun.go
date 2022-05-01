// dryrun allows testing an Alert and seeing the regression it would find.
package dryrun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

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

// RegressionAtCommit is a Regression found for a specific commit.
type RegressionAtCommit struct {
	CID        perfgit.Commit         `json:"cid"`
	Regression *regression.Regression `json:"regression"`
}

// Requests handles HTTP request for doing dryruns.
type Requests struct {
	perfGit       *perfgit.Git
	shortcutStore shortcut.Store
	dfBuilder     dataframe.DataFrameBuilder
	tracker       progress.Tracker
	paramsProvier regression.ParamsetProvider
}

// New create a new dryrun Request processor.
func New(perfGit *perfgit.Git, tracker progress.Tracker, shortcutStore shortcut.Store, dfBuilder dataframe.DataFrameBuilder, paramsProvider regression.ParamsetProvider) *Requests {
	ret := &Requests{
		perfGit:       perfGit,
		shortcutStore: shortcutStore,
		dfBuilder:     dfBuilder,
		tracker:       tracker,
		paramsProvier: paramsProvider,
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
	auditlog.LogWithUser(r, "", "dryrun", req)
	d.tracker.Add(req.Progress)

	if req.Alert.Query == "" {
		req.Progress.Error("Query must not be empty.")
		if err := req.Progress.JSON(w); err != nil {
			sklog.Errorf("Failed to encode paramset: %s", err)
		}
		return
	}
	if err := req.Alert.Validate(); err != nil {
		req.Progress.Error(err.Error())
		if err := req.Progress.JSON(w); err != nil {
			sklog.Errorf("Failed to encode paramset: %s", err)
		}
		return
	}

	foundRegressions := map[types.CommitNumber]*regression.Regression{}

	// Create a callback that will be passed each found Regression. It will
	// update the Progress after each new regression is found.
	detectorResponseProcessor := func(queryRequest *regression.RegressionDetectionRequest, clusterResponse []*regression.RegressionDetectionResponse, message string) {
		// Loop over clusterResponse, convert each one to a regression, and merge with running.Regressions.
		for _, cr := range clusterResponse {
			c, reg, err := regression.RegressionFromClusterResponse(ctx, cr, req.Alert, d.perfGit)
			if err != nil {
				sklog.Errorf("Failed to convert to Regression: %s", err)
				return
			}
			req.Progress.Message("Step", fmt.Sprintf("%d/%d", queryRequest.Step+1, queryRequest.TotalQueries))
			req.Progress.Message("Query", fmt.Sprintf("%q", queryRequest.Query()))
			req.Progress.Message("Stage", "Looking for regressions in query results.")
			req.Progress.Message("Commit", fmt.Sprintf("%d", c.CommitNumber))
			req.Progress.Message("Details", message)
			// We might not have found any regressions.
			if reg.Low == nil && reg.High == nil {
				continue
			}
			if origReg, ok := foundRegressions[c.CommitNumber]; !ok {
				foundRegressions[c.CommitNumber] = reg
			} else {
				foundRegressions[c.CommitNumber] = origReg.Merge(reg)
			}

		}

		// Now update the Progress.
		regressions := []*RegressionAtCommit{}

		commitNumbers := []types.CommitNumber{}
		for id := range foundRegressions {
			commitNumbers = append(commitNumbers, id)
		}
		sort.Sort(types.CommitNumberSlice(commitNumbers))

		for _, commitNumber := range commitNumbers {
			details, err := d.perfGit.CommitFromCommitNumber(ctx, commitNumber)
			if err != nil {
				sklog.Errorf("Failed to look up commit %d: %s", commitNumber, err)
				continue
			}
			regressions = append(regressions, &RegressionAtCommit{
				CID:        details,
				Regression: foundRegressions[commitNumber],
			})
		}
		req.Progress.Results(regressions)
	}

	go func() {
		err := regression.ProcessRegressions(ctx, req, detectorResponseProcessor, d.perfGit, d.shortcutStore, d.dfBuilder, d.paramsProvier(), regression.ExpandBaseAlertByGroupBy, regression.ContinueOnError)
		if err != nil {
			req.Progress.Error(err.Error())
		} else {
			req.Progress.Finished()
		}
	}()

	if err := req.Progress.JSON(w); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}
