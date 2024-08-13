package service

import (
	"context"

	"go.skia.org/infra/perf/go/anomalygroup"
	"go.skia.org/infra/perf/go/backend/shared"
	"go.skia.org/infra/perf/go/culprit"
	"go.skia.org/infra/perf/go/culprit/notify"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/subscription"
	"google.golang.org/grpc"
)

// culpritService implements CulpritService
type culpritService struct {
	pb.UnimplementedCulpritServiceServer
	anomalygroupStore anomalygroup.Store
	culpritStore      culprit.Store
	subscriptionStore subscription.Store
	notifier          notify.CulpritNotifier
}

// New returns a new instance of culpritService.
func New(anomalygroupStore anomalygroup.Store, culpritStore culprit.Store, subscriptionStore subscription.Store,
	notifier notify.CulpritNotifier) *culpritService {
	return &culpritService{
		anomalygroupStore: anomalygroupStore,
		culpritStore:      culpritStore,
		subscriptionStore: subscriptionStore,
		notifier:          notifier,
	}
}

// RegisterGrpc implements backend.BackendService
func (s *culpritService) RegisterGrpc(server *grpc.Server) {
	pb.RegisterCulpritServiceServer(server, s)
}

// GetAuthorizationPolicy implements backend.BackendService
func (s *culpritService) GetAuthorizationPolicy() shared.AuthorizationPolicy {
	// TODO(pasthana): Add proper authorization policy
	return shared.AuthorizationPolicy{
		AllowUnauthenticated: true,
	}
}

// GetServiceDescriptor implements backend.BackendService
func (s *culpritService) GetServiceDescriptor() grpc.ServiceDesc {
	return pb.CulpritService_ServiceDesc
}

func (s *culpritService) PersistCulprit(ctx context.Context, req *pb.PersistCulpritRequest) (*pb.PersistCulpritResponse, error) {
	ids, err := s.culpritStore.Upsert(ctx, req.AnomalyGroupId, req.Commits)
	if err != nil {
		return nil, err
	}
	err = s.anomalygroupStore.AddCulpritIDs(ctx, req.AnomalyGroupId, ids)
	if err != nil {
		return nil, err
	}
	return &pb.PersistCulpritResponse{CulpritIds: ids}, nil
}

func (s *culpritService) GetCulprit(context context.Context, req *pb.GetCulpritRequest) (*pb.GetCulpritResponse, error) {
	culprits, err := s.culpritStore.Get(context, req.CulpritIds)
	if err != nil {
		return nil, err
	}
	return &pb.GetCulpritResponse{
		Culprits: culprits,
	}, nil
}

// File bugs per culprit for the anomaly group (from a bisect)
func (s *culpritService) NotifyUserOfCulprit(ctx context.Context, req *pb.NotifyUserOfCulpritRequest) (*pb.NotifyUserOfCulpritResponse, error) {
	var err error
	culprits, err := s.culpritStore.Get(ctx, req.CulpritIds)
	if err != nil {
		return nil, err
	}
	anomalygroup, err := s.anomalygroupStore.LoadById(ctx, req.AnomalyGroupId)
	if err != nil {
		return nil, err
	}
	subscription, err := s.subscriptionStore.GetSubscription(ctx, anomalygroup.SubsciptionName, anomalygroup.SubscriptionRevision)
	if err != nil {
		return nil, err
	}
	issueIds := make([]string, 0)
	for _, culprit := range culprits {
		issueId, err := s.notifier.NotifyCulpritFound(ctx, culprit, subscription)
		if err != nil {
			return nil, err
		}
		err = s.culpritStore.AddIssueId(ctx, culprit.Id, issueId, req.AnomalyGroupId)
		if err != nil {
			return nil, err
		}
		issueIds = append(issueIds, issueId)
	}
	return &pb.NotifyUserOfCulpritResponse{IssueIds: issueIds}, nil
}

// File a bug to report a list of anomalies.
func (s *culpritService) NotifyUserOfAnomaly(ctx context.Context, req *pb.NotifyUserOfAnomalyRequest) (*pb.NotifyUserOfAnomalyResponse, error) {
	var err error
	anomalygroup, err := s.anomalygroupStore.LoadById(ctx, req.AnomalyGroupId)
	if err != nil {
		return nil, err
	}
	subscription, err := s.subscriptionStore.GetSubscription(ctx, anomalygroup.SubsciptionName, anomalygroup.SubscriptionRevision)
	if err != nil {
		return nil, err
	}
	issueId, err := s.notifier.NotifyAnomaliesFound(ctx, req.Anomaly, subscription)
	return &pb.NotifyUserOfAnomalyResponse{IssueId: issueId}, nil
}
