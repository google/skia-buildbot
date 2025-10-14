package api

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/chromeperf/compat"
)

// TODO(b/438183175) Populate remaining fields of GetGroupReportResponse:
// StateId, SelectedKeys, Error, TimerangeMap
// Given a list of anomaly IDs (regressions and possibly improvements),
// fill GetGroupReportResponse Anomalies list using just regressions.
func (api anomaliesApi) getGroupReportByAnomalyIdList(ctx context.Context, anomalyIds *[]string) (*GetGroupReportResponse, error) {
	groupReportResponse := &GetGroupReportResponse{}
	regressions, err := api.regStore.GetByIDs(ctx, *anomalyIds)
	if err != nil {
		return nil, fmt.Errorf("Failed to get regressions by ID: %s", err)
	}
	groupReportResponse.Anomalies = make([]chromeperf.Anomaly, 0)
	for _, reg := range regressions {
		anomalies, err := compat.ConvertRegressionToAnomalies(reg)
		if err != nil {
			sklog.Warningf("Could not convert regression with id %s to anomalies: %s", reg.Id, err)
			continue
		}
		for _, commitNumberMap := range anomalies {
			for _, anomaly := range commitNumberMap {
				groupReportResponse.Anomalies = append(groupReportResponse.Anomalies, anomaly)
			}
		}
	}
	return groupReportResponse, nil
}

// GetGroupReport for regressions that match GetGroupReportRequest.anomalyIDs list
func (api anomaliesApi) getGroupReportByAnomalyId(ctx context.Context, groupReportRequest GetGroupReportRequest) (*GetGroupReportResponse, error) {
	anomalyIds := strings.Split(groupReportRequest.AnomalyIDs, ",")
	return api.getGroupReportByAnomalyIdList(ctx, &anomalyIds)
}

// GetGroupReport for regressions that match GetGroupReportRequest.BugID
func (api anomaliesApi) getGroupReportByBugId(ctx context.Context, groupReportRequest GetGroupReportRequest) (*GetGroupReportResponse, error) {
	id := groupReportRequest.BugID
	anomalyIds, err := api.anomalygroupStore.GetAnomalyIdsByIssueId(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Failed to get anomalyIds from anomalygroup Store by issue ID: %s", err)
	}
	// TODO(b/438183175) query from Culprits, too. Looks like reported_issue_id can be all null, even though we have ongoing bugs.
	return api.getGroupReportByAnomalyIdList(ctx, &anomalyIds)
}
