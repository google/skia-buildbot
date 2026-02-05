package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tpr_client "go.temporal.io/sdk/client"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/anomalygroup"
	ag "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/backend/shared"
	"go.skia.org/infra/perf/go/culprit"
	reg "go.skia.org/infra/perf/go/regression"
	"google.golang.org/grpc"
)

// anomalygroupService implements AnomalyGroupService
type anomalygroupService struct {
	ag.UnimplementedAnomalyGroupServiceServer
	anomalygroupStore anomalygroup.Store
	culpritStore      culprit.Store
	regressionStore   reg.Store
	temporalClient    tpr_client.Client
	newGroupCounter   metrics2.Counter
}

// New returns a new instance of anomalygroupService.
func New(anomalygroupStore anomalygroup.Store, culpritStore culprit.Store, regressionStore reg.Store, temporalClient tpr_client.Client) *anomalygroupService {
	return &anomalygroupService{
		anomalygroupStore: anomalygroupStore,
		culpritStore:      culpritStore,
		regressionStore:   regressionStore,
		temporalClient:    temporalClient,
		newGroupCounter:   metrics2.GetCounter("anomalygroup_created"),
	}
}

// RegisterGrpc implements backend.BackendService
func (s *anomalygroupService) RegisterGrpc(server *grpc.Server) {
	ag.RegisterAnomalyGroupServiceServer(server, s)
}

// GetAuthorizationPolicy implements backend.BackendService
func (s *anomalygroupService) GetAuthorizationPolicy() shared.AuthorizationPolicy {
	return shared.AuthorizationPolicy{
		AllowUnauthenticated: true,
	}
}

// GetServiceDescriptor implements backend.BackendService
func (s *anomalygroupService) GetServiceDescriptor() grpc.ServiceDesc {
	return ag.AnomalyGroupService_ServiceDesc
}

// created a new group given a set of properties.
func (s *anomalygroupService) CreateNewAnomalyGroup(
	ctx context.Context,
	req *ag.CreateNewAnomalyGroupRequest) (*ag.CreateNewAnomalyGroupResponse, error) {

	new_group_id, err := s.anomalygroupStore.Create(
		ctx,
		req.SubscriptionName,
		req.SubscriptionRevision,
		req.Domain,
		req.Benchmark,
		req.StartCommit,
		req.EndCommit,
		req.Action.String())
	if err != nil {
		return nil, fmt.Errorf(
			"error when calling CreateNewAnomalyGroup. Params: %s", req)
	}
	s.newGroupCounter.Inc(1)
	return &ag.CreateNewAnomalyGroupResponse{
		AnomalyGroupId: new_group_id,
	}, nil
}

// Given a group id, return the group.
func (s *anomalygroupService) LoadAnomalyGroupByID(
	ctx context.Context,
	req *ag.LoadAnomalyGroupByIDRequest) (*ag.LoadAnomalyGroupByIDResponse, error) {
	anomaly_group, err := s.anomalygroupStore.LoadById(ctx, req.AnomalyGroupId)
	if err != nil {
		return nil, fmt.Errorf(
			"error when calling LoadAnomalyGroupByID: %s. Params: %s", err.Error(), req)
	}
	return &ag.LoadAnomalyGroupByIDResponse{
		AnomalyGroup: anomaly_group,
	}, nil
}

// Given one of the following value, update the group
//   - bisection id
//   - reported issue id
//   - new anomaly id
//   - new culprit ids
func (s *anomalygroupService) UpdateAnomalyGroup(
	ctx context.Context,
	req *ag.UpdateAnomalyGroupRequest) (*ag.UpdateAnomalyGroupResponse, error) {
	if req.BisectionId != "" {
		if err := s.anomalygroupStore.UpdateBisectID(
			ctx, req.AnomalyGroupId, req.BisectionId); err != nil {
			return nil, skerr.Wrapf(err,
				"error updating the bisection id %s for anomaly group %s",
				req.BisectionId, req.AnomalyGroupId)
		}
	} else if req.IssueId != "" {
		if err := s.anomalygroupStore.UpdateReportedIssueID(
			ctx, req.AnomalyGroupId, req.IssueId); err != nil {
			return nil, skerr.Wrapf(err,
				"error updating the reported issue id %s for anomaly group %s",
				req.IssueId, req.AnomalyGroupId)
		}
	} else if req.AnomalyId != "" {
		anomalies, err := s.regressionStore.GetByIDs(ctx, []string{req.AnomalyId})
		if err != nil {
			return nil, skerr.Wrapf(err, "error getting anomaly %s", req.AnomalyId)
		}
		if len(anomalies) == 0 {
			return nil, skerr.Fmt("anomaly %s not found", req.AnomalyId)
		}
		anomaly := anomalies[0]

		if err := s.anomalygroupStore.AddAnomalyID(
			ctx, req.AnomalyGroupId, req.AnomalyId, int64(anomaly.PrevCommitNumber)+1, int64(anomaly.CommitNumber)); err != nil {
			return nil, skerr.Wrapf(err,
				"error adding the anomaly id %s to anomaly group %s",
				req.AnomalyId, req.AnomalyGroupId)
		}
	} else if len(req.CulpritIds) > 0 {
		if err := s.anomalygroupStore.AddCulpritIDs(
			ctx, req.AnomalyGroupId, req.CulpritIds); err != nil {
			return nil, skerr.Wrapf(err,
				"error adding the culprit ids %s to anomaly group %s",
				req.CulpritIds, req.AnomalyGroupId)
		}
	}
	return &ag.UpdateAnomalyGroupResponse{}, nil
}

// Give a set of grouping criteria, return the existing groups.
func (s *anomalygroupService) FindExistingGroups(
	ctx context.Context,
	req *ag.FindExistingGroupsRequest) (*ag.FindExistingGroupsResponse, error) {
	test_path_pieces := strings.Split(req.TestPath, "/")
	if len(test_path_pieces) < 5 {
		return nil, fmt.Errorf("invalid fromat of test path: %s", req.TestPath)
	}
	domain_name := test_path_pieces[0]
	benchmark_name := test_path_pieces[2]
	anomaly_groups, err := s.anomalygroupStore.FindExistingGroup(ctx,
		req.SubscriptionName, req.SubscriptionRevision, domain_name,
		benchmark_name, req.StartCommit, req.EndCommit, req.Action.String())
	if err != nil {
		return nil, fmt.Errorf("failed on finding existing groups. Request: %s", req)
	}
	return &ag.FindExistingGroupsResponse{
		AnomalyGroups: anomaly_groups,
	}, nil
}

// Given the group id and a limit value N, return the top N regressions from
// the group's anomalies, sorted by the percentage changed on median.
func (s *anomalygroupService) FindTopAnomalies(
	ctx context.Context,
	req *ag.FindTopAnomaliesRequest) (*ag.FindTopAnomaliesResponse, error) {
	group_id := req.AnomalyGroupId
	anomaly_group, err := s.anomalygroupStore.LoadById(ctx, group_id)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to load anomaly group: %s", group_id)
	}

	anomalies, err := s.regressionStore.GetByIDs(ctx, anomaly_group.AnomalyIds)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to load regressions from group: %s", group_id)
	}

	top_regressions, err := TopAnomaliesMedianCmp(anomalies, req.Limit)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &ag.FindTopAnomaliesResponse{
		Anomalies: top_regressions,
	}, nil
}

func TopAnomaliesMedianCmp(anomalies []*reg.Regression, limit int64) ([]*ag.Anomaly, error) {
	sort.Slice(anomalies, func(i, j int) bool {
		// sort anomalies by the percentage changed from median_before to median_after
		diff_i := (anomalies[i].MedianAfter - anomalies[i].MedianBefore) / anomalies[i].MedianBefore
		diff_j := (anomalies[j].MedianAfter - anomalies[j].MedianBefore) / anomalies[j].MedianBefore
		return diff_i > diff_j
	})

	var count int
	// If the request is 0 or is larger then the total of the group's anomalies, return all the anomalies.
	if limit > 0 && int(limit) < len(anomalies) {
		count = int(limit)
	} else {
		count = len(anomalies)
	}

	top_regressions := []*ag.Anomaly{}
	// loop over the top 'count' regressions.
	for i := 0; i < count; i++ {
		anomaly := anomalies[i]
		paramset := anomaly.Frame.DataFrame.ParamSet

		if !isParamSetValid(paramset) {
			// Debug logs on b/357629365: can we use traceset to replace paramset?
			for key := range anomaly.Frame.DataFrame.TraceSet {
				paramset_from_key, err := query.ParseKey(key)
				if err != nil {
					sklog.Debugf("[AG][InvalidParamset] Failed to parse trace set key: %s", key)
				} else {
					sklog.Debugf("[AG][InvalidParamset] Paramset parsed from trace set: %s", paramset_from_key)
				}
			}

			return nil, skerr.Fmt("invalid paramset %s for chromeperf", paramset)
		}

		// find the last available parameters of the subtest_x series.
		subtest_keys := []string{"subtest_3", "subtest_2", "subtest_1"}
		story := []string{}
		for _, key := range subtest_keys {
			ok := false
			if story, ok = paramset[key]; ok {
				break
			}
		}

		// TODO(wenbinzhang): put the list of parameters in config file.
		paramset_map := map[string]string{
			"bot":         paramset["bot"][0],
			"benchmark":   paramset["benchmark"][0],
			"story":       story[0],
			"measurement": paramset["test"][0],
			"stat":        paramset["stat"][0],
		}

		top_regressions = append(top_regressions, &ag.Anomaly{
			StartCommit:          int64(anomaly.PrevCommitNumber),
			EndCommit:            int64(anomaly.CommitNumber),
			Paramset:             paramset_map,
			ImprovementDirection: paramset["improvement_direction"][0],
			MedianBefore:         anomaly.MedianBefore,
			MedianAfter:          anomaly.MedianAfter,
		})
	}

	return top_regressions, nil
}

// Given the group id, return the issues correlated to the group via the detected culprits.
func (s *anomalygroupService) FindIssuesFromCulprits(ctx context.Context, req *ag.FindIssuesFromCulpritsRequest) (*ag.FindIssuesFromCulpritsResponse, error) {
	groupId := req.AnomalyGroupId
	issueIds := []string{}
	sklog.Debugf("[AG] FindIssuesFromCulprits called for group: %s", groupId)

	anomalyGroup, err := s.anomalygroupStore.LoadById(ctx, groupId)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading group by id: %s", groupId)
	}
	culpritIds := anomalyGroup.CulpritIds
	sklog.Debugf("[AG] FindIssuesFromCulprits loads culpritIds: %s from group: %s", culpritIds, groupId)

	culprits, err := s.culpritStore.Get(ctx, culpritIds)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting culprit by ids: %s", culpritIds)
	}
	for _, culprit := range culprits {
		issueId, ok := culprit.GroupIssueMap[groupId]
		if ok {
			issueIds = append(issueIds, issueId)
		}
	}

	return &ag.FindIssuesFromCulpritsResponse{
		IssueIds: issueIds,
	}, nil
}

func isParamSetValid(paramset paramtools.ReadOnlyParamSet) bool {
	requiredKeys := []string{"bot", "benchmark", "test", "stat", "subtest_1"}
	for _, key := range requiredKeys {
		value, ok := paramset[key]
		if !ok {
			return false
		}
		if len(value) > 1 {
			return false
		}
	}

	return true
}
