package api

import (
	"context"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/chromeperf/compat"
	"go.skia.org/infra/perf/go/regression"
)

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
		return nil, skerr.Fmt("failed to get anomalyIds from anomalygroup Store by issue ID: %s", err)
	}
	// TODO(b/438183175) query from Culprits, too. Looks like reported_issue_id can be all null, even though we have ongoing bugs.
	return api.getGroupReportByAnomalyIdList(ctx, &anomalyIds)
}

// GetGroupReport for regressions that match GetGroupReportRequest.AnomalyGroupId
func (api anomaliesApi) getGroupReportByAnomalyGroupId(ctx context.Context, groupReportRequest GetGroupReportRequest) (*GetGroupReportResponse, error) {
	id := groupReportRequest.AnomalyGroupID
	anomalyIds, err := api.anomalygroupStore.GetAnomalyIdsByAnomalyGroupId(ctx, id)
	if err != nil {
		return nil, skerr.Fmt("failed to get anomalyIds from anomalygroup Store by anomaly group ID: %s", err)
	}

	return api.getGroupReportByAnomalyIdList(ctx, &anomalyIds)
}

// GetGroupReport for regressions that match GetGroupReportRequest.Revision
func (api anomaliesApi) getGroupReportByRevision(ctx context.Context, groupReportRequest GetGroupReportRequest) (*GetGroupReportResponse, error) {
	rev := groupReportRequest.Revison
	regressions, err := api.regStore.GetByRevision(ctx, rev)
	if err != nil {
		return nil, skerr.Fmt("failed to get anomalyIds from anomalygroup Store by Revision: %s", err)
	}

	return prepareResponseFromRegressions(regressions)
}

// Given a list of anomaly IDs, fill GetGroupReportResponse Anomalies list.
func (api anomaliesApi) getGroupReportByAnomalyIdList(ctx context.Context, anomalyIds *[]string) (*GetGroupReportResponse, error) {
	regressions, err := api.regStore.GetByIDs(ctx, *anomalyIds)
	if err != nil {
		return nil, skerr.Fmt("failed to get regressions by ID: %s", err)
	}
	return prepareResponseFromRegressions(regressions)
}

// TODO(b/438183175) Populate remaining fields of GetGroupReportResponse:
// StateId, SelectedKeys, Error, TimerangeMap
func prepareResponseFromRegressions(regressions []*regression.Regression) (*GetGroupReportResponse, error) {
	groupReportResponse := &GetGroupReportResponse{}
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
