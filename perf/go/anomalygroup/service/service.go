package service

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/perf/go/anomalygroup"
	pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/backend/shared"
	"google.golang.org/grpc"
)

// anomalygroupService implements AnomalyGroupService
type anomalygroupService struct {
	pb.UnimplementedAnomalyGroupServiceServer
	store anomalygroup.Store
}

// New returns a new instance of anomalygroupService.
func New(store anomalygroup.Store) *anomalygroupService {
	return &anomalygroupService{
		store: store,
	}
}

// RegisterGrpc implements backend.BackendService
func (s *anomalygroupService) RegisterGrpc(server *grpc.Server) {
	pb.RegisterAnomalyGroupServiceServer(server, s)
}

// GetAuthorizationPolicy implements backend.BackendService
func (s *anomalygroupService) GetAuthorizationPolicy() shared.AuthorizationPolicy {
	return shared.AuthorizationPolicy{
		AllowUnauthenticated: true,
	}
}

// GetServiceDescriptor implements backend.BackendService
func (s *anomalygroupService) GetServiceDescriptor() grpc.ServiceDesc {
	return pb.AnomalyGroupService_ServiceDesc
}

func (s *anomalygroupService) CreateNewAnomalyGroup(
	ctx context.Context,
	req *pb.CreateAnomalyGroupRequest) (*pb.CreateAnomalyGroupResponse, error) {

	new_group_id, err := s.store.Create(
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
	return &pb.CreateAnomalyGroupResponse{
		AnomalyGroupId: new_group_id,
	}, nil
}

func (s *anomalygroupService) LoadAnomalyGroupByID(
	ctx context.Context,
	req *pb.ReadAnomalyGroupRequest) (*pb.ReadAnomalyGroupResponse, error) {
	anomaly_group, err := s.store.LoadById(ctx, req.AnomalyGroupId)
	if err != nil {
		return nil, fmt.Errorf(
			"error when calling LoadAnomalyGroupByID. Params: %s", req)
	}
	return &pb.ReadAnomalyGroupResponse{
		AnomalyGroup: anomaly_group,
	}, nil
}

func (s *anomalygroupService) UpdateAnomalyGroup(
	ctx context.Context,
	req *pb.UpdateAnomalyGroupRequest) (*pb.UpdateAnomalyGroupResponse, error) {
	if req.BisectionId != "" {
		if err := s.store.UpdateBisectID(
			ctx, req.AnomalyGroupId, req.BisectionId); err != nil {
			return nil, fmt.Errorf(
				"error updating the bisection id %s for anomaly group %s",
				req.BisectionId, req.AnomalyGroupId)
		}
	} else if req.IssueId != "" {
		if err := s.store.UpdateReportedIssueID(
			ctx, req.AnomalyGroupId, req.IssueId); err != nil {
			return nil, fmt.Errorf(
				"error updating the reported issue id %s for anomaly group %s",
				req.IssueId, req.AnomalyGroupId)
		}
	} else if req.AnomalyId != "" {
		if err := s.store.AddAnomalyID(
			ctx, req.AnomalyGroupId, req.AnomalyId); err != nil {
			return nil, fmt.Errorf(
				"error adding the anomaly id %s to anomaly group %s",
				req.AnomalyId, req.AnomalyGroupId)
		}
	} else if len(req.CulpritIds) > 0 {
		if err := s.store.AddCulpritIDs(
			ctx, req.AnomalyGroupId, req.CulpritIds); err != nil {
			return nil, fmt.Errorf(
				"error adding the culprit ids %s to anomaly group %s",
				req.CulpritIds, req.AnomalyGroupId)
		}
	}
	return &pb.UpdateAnomalyGroupResponse{}, nil
}

func (s *anomalygroupService) FindExistingGroups(
	ctx context.Context,
	req *pb.FindExistingGroupsRequest) (*pb.FindExistingGroupsResponse, error) {
	test_path_pieces := strings.Split(req.TestPath, "/")
	if len(test_path_pieces) < 5 {
		return nil, fmt.Errorf("invalid fromat of test path: %s", req.TestPath)
	}
	domain_name := test_path_pieces[0]
	benchmark_name := test_path_pieces[2]
	anomaly_groups, err := s.store.FindExistingGroup(ctx,
		req.SubscriptionName, req.SubscriptionRevision, domain_name,
		benchmark_name, req.StartCommit, req.EndCommit, req.Action)
	if err != nil {
		return nil, fmt.Errorf("failed on finding existing groups. Request: %s", req)
	}
	return &pb.FindExistingGroupsResponse{
		AnomalyGroups: anomaly_groups,
	}, nil
}
