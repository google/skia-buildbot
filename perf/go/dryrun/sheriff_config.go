package dryrun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/regression"
	pb "go.skia.org/infra/perf/go/sheriffconfig/proto/v1"
	"go.skia.org/infra/perf/go/sheriffconfig/service"
	"go.skia.org/infra/perf/go/types"
)

type SheriffConfigDryRunRequest struct {
	Config *pb.SheriffConfig `json:"config"`
	Domain types.Domain      `json:"domain"`
}

// StartSheriffConfigHandler starts a dryrun from a Sheriff Config Protobuf text.
func (d *Requests) StartSheriffConfigHandler(w http.ResponseWriter, r *http.Request) {
	ctx := regression.WithDryRun(context.Background())
	w.Header().Set("Content-Type", "application/json")

	var parsedReq SheriffConfigDryRunRequest
	if err := json.NewDecoder(r.Body).Decode(&parsedReq); err != nil {
		httputils.ReportError(w, err, "Could not decode POST body.", http.StatusInternalServerError)
		return
	}
	auditlog.LogWithUser(r, "", "dryrun-sheriff-config", parsedReq.Config)

	// Create a dummy Request just to get a fresh progress tracker ID and return it to the client.
	req := regression.NewRegressionDetectionRequest()
	req.Domain = parsedReq.Domain
	d.tracker.Add(req.Progress)

	sheriffConfig := parsedReq.Config
	if sheriffConfig == nil || len(sheriffConfig.Subscriptions) == 0 {
		req.Progress.Error("No subscriptions found in the config.")
		if err := req.Progress.JSON(w); err != nil {
			sklog.Errorf("Failed to encode paramset: %s", err)
		}
		return
	}

	go func() {
		defer func() {
			if req.Progress.Status() == progress.Running {
				req.Progress.Finished()
			}
		}()

		allRegressions := map[types.CommitNumber]*regression.Regression{}
		totalAlerts := 0
		alertsProcessed := 0

		var allAlerts []*regression.RegressionDetectionRequest
		for _, subscription := range sheriffConfig.Subscriptions {
			alerts, err := service.SubscriptionToAlerts(subscription)
			if err != nil {
				req.Progress.Error(skerr.Fmt("Error generating alerts: %s", err).Error())
				return
			}
			for _, alert := range alerts {
				newReq := regression.NewRegressionDetectionRequest()
				newReq.Alert = alert
				newReq.Domain = req.Domain
				allAlerts = append(allAlerts, newReq)
			}
		}

		totalAlerts = len(allAlerts)
		if totalAlerts == 0 {
			req.Progress.Error("No alerts generated from the configuration.")
			return
		}

		for _, alertReq := range allAlerts {
			alertsProcessed++

			if alertReq.Alert.Query == "" {
				sklog.Warningf("Generated alert has an empty query, skipping.")
				continue
			}

			if err := alertReq.Alert.Validate(); err != nil {
				sklog.Warningf("Generated alert validation failed: %s", err)
				continue
			}

			req.Progress.Message("Alert Progress", fmt.Sprintf("Processing alert %d of %d", alertsProcessed, totalAlerts))

			detectorResponseProcessor := func(ctx context.Context, queryRequest *regression.RegressionDetectionRequest, allClusterResponses []*regression.ConfirmedRegression, summaryMessage string) error {
				for _, cr := range allClusterResponses {
					c, reg, err := regression.ConfirmedRegressionFromClusterResponse(ctx, cr, alertReq.Alert, d.perfGit)
					if err != nil {
						sklog.Errorf("Failed to convert to Regression: %s", err)
						return err
					}
					req.Progress.Message("Step", fmt.Sprintf("%d/%d", queryRequest.Step+1, queryRequest.TotalQueries))
					req.Progress.Message("Query", fmt.Sprintf("%q", queryRequest.Query()))
					req.Progress.Message("Stage", "Looking for regressions in query results.")
					req.Progress.Message("Commit", fmt.Sprintf("%d", c.CommitNumber))
					req.Progress.Message("Details", cr.Message)

					if origReg, ok := allRegressions[c.CommitNumber]; !ok {
						allRegressions[c.CommitNumber] = reg
					} else {
						allRegressions[c.CommitNumber] = origReg.Merge(reg)
					}
				}
				return nil
			}

			err := regression.ProcessRegressions(ctx, alertReq, detectorResponseProcessor, d.perfGit, d.shortcutStore, d.dfBuilder, d.paramsProvider(), regression.ExpandBaseAlertByGroupBy, regression.ContinueOnError, config.Config.AnomalyConfig, nil, d.regressionRefiner)
			if err != nil {
				req.Progress.Error(err.Error())
				return
			}
		}

		regressions := []*RegressionAtCommit{}
		commitNumbers := []types.CommitNumber{}
		for id := range allRegressions {
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
				Regression: allRegressions[commitNumber],
			})
		}
		req.Progress.Results(regressions)
	}()

	if err := req.Progress.JSON(w); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}
