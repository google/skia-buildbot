package api

import (
	"context"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/chromeperf/compat"
	"go.skia.org/infra/perf/go/regression"
	"golang.org/x/sync/errgroup"
)

// Somewhat close to the url length limit, although some space left for the instance address.
// This doesn't have to be precise - when we receive bug reports or see that users require long urls,
// we will implement SID and remove the check that uses this constant.
const needForSidLimit = 1900

// GetGroupReport for regressions that match GetGroupReportRequest.anomalyIDs list
func (api anomaliesApi) getGroupReportByAnomalyId(ctx context.Context, groupReportRequest GetGroupReportRequest) (*GetGroupReportResponse, error) {
	anomalyIds := strings.Split(groupReportRequest.AnomalyIDs, ",")
	return api.getGroupReportByAnomalyIdList(ctx, &anomalyIds)
}

// GetGroupReport for regressions that match GetGroupReportRequest.BugID
func (api anomaliesApi) getGroupReportByBugId(ctx context.Context, groupReportRequest GetGroupReportRequest) (*GetGroupReportResponse, error) {
	id := groupReportRequest.BugID
	errg, errgCtx := errgroup.WithContext(ctx)
	anomalyIdsChan := make(chan []string, 3)

	errg.Go(func() error {
		anomalyGroupIdsFromCulprit, err := api.culpritStore.GetAnomalyGroupIdsForIssueId(errgCtx, id)
		if err != nil {
			return skerr.Fmt("failed to get anomaly group ids by issue id from culprit store: %s", err)
		}
		anomalyIdsRelatedToCulprit, err := api.anomalygroupStore.GetAnomalyIdsByAnomalyGroupIds(errgCtx, anomalyGroupIdsFromCulprit)
		if err != nil {
			return skerr.Fmt("failed to get anomaly ids for groups fetched earlier from culprit store: %s", err)
		}
		anomalyIdsChan <- anomalyIdsRelatedToCulprit
		return nil
	})

	errg.Go(func() error {
		anomalyIdsFromGroupStore, err := api.anomalygroupStore.GetAnomalyIdsByIssueId(errgCtx, id)
		if err != nil {
			return skerr.Fmt("failed to get anomalyIds from anomalygroup store by issue id: %s", err)
		}
		anomalyIdsChan <- anomalyIdsFromGroupStore
		return nil
	})

	errg.Go(func() error {
		bugId, err := strconv.Atoi(id)
		if err != nil {
			return skerr.Fmt("failed to convert bug id %s to int", id)
		}
		anomalyIdsFromRegressionStore, err := api.regStore.GetIdsByManualTriageBugID(errgCtx, bugId)
		if err != nil {
			return skerr.Fmt("failed to get anomalyIds from regression store by triage bug id: %s", err)
		}
		anomalyIdsChan <- anomalyIdsFromRegressionStore
		return nil
	})

	if err := errg.Wait(); err != nil {
		return nil, err
	}

	// We have three sources of anomalyIds, since we need to query Regressions, Culprits,
	// and AnomalyGroup separately.
	// For one anomalygroup there may be many culprits, and therefore many issue_ids.
	// In this case, the expected value in anomalygroups would be null,
	// and culprits table is the source of truth.
	// This is because, when bisecting, we don't populate the anomalygroup's issue_id field at all,
	// which is done by design.
	anomalyIds := <-anomalyIdsChan
	anomalyIds = append(anomalyIds, <-anomalyIdsChan...)
	anomalyIds = append(anomalyIds, <-anomalyIdsChan...)

	// TODO(b/438183175) CREATE INDEX idx_culprits_issue_ids ON Culprits USING GIN (issue_ids);

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
	regressionsWithAllBugs, err := api.regStore.GetBugIdsForRegressions(ctx, regressions)
	if err != nil {
		return nil, skerr.Fmt("failed to add bug ids to %d regressions", len(regressions))
	}

	return api.prepareResponseFromRegressions(ctx, regressionsWithAllBugs)
}

// Given a list of anomaly IDs, fill GetGroupReportResponse Anomalies list.
func (api anomaliesApi) getGroupReportByAnomalyIdList(ctx context.Context, anomalyIds *[]string) (*GetGroupReportResponse, error) {
	regressions, err := api.regStore.GetByIDs(ctx, *anomalyIds)
	if err != nil {
		return nil, skerr.Fmt("failed to get regressions by ID: %s", err)
	}
	regressionsWithAllBugs, err := api.regStore.GetBugIdsForRegressions(ctx, regressions)
	if err != nil {
		return nil, skerr.Fmt("failed to add bug ids to %d regressions", len(regressions))
	}
	return api.prepareResponseFromRegressions(ctx, regressionsWithAllBugs)
}

func (api anomaliesApi) prepareResponseFromRegressions(ctx context.Context, regressions []*regression.Regression) (groupReportResponse *GetGroupReportResponse, err error) {
	groupReportResponse = &GetGroupReportResponse{}
	groupReportResponse.Anomalies = make([]chromeperf.Anomaly, 0)
	for _, reg := range regressions {
		anomalies, err := compat.ConvertRegressionToAnomalies(reg)
		if err != nil {
			sklog.Warningf("could not convert regression with id %s to anomalies: %s", reg.Id, err)
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
