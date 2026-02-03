package service

import (
	"context"
	"fmt"
	"slices"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/anomalygroup"
	v1 "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/backend/shared"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/culprit"
	"go.skia.org/infra/perf/go/culprit/notify"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/subscription"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"google.golang.org/grpc"
)

// culpritService implements CulpritService
type culpritService struct {
	pb.UnimplementedCulpritServiceServer
	anomalygroupStore anomalygroup.Store
	culpritStore      culprit.Store
	subscriptionStore subscription.Store
	notifier          notify.CulpritNotifier
	config            *config.InstanceConfig
}

// New returns a new instance of culpritService.
func New(anomalygroupStore anomalygroup.Store, culpritStore culprit.Store, subscriptionStore subscription.Store,
	notifier notify.CulpritNotifier, cfg *config.InstanceConfig) *culpritService {
	return &culpritService{
		anomalygroupStore: anomalygroupStore,
		culpritStore:      culpritStore,
		subscriptionStore: subscriptionStore,
		notifier:          notifier,
		config:            cfg,
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
	sklog.Debugf("[CP] %d culprits loaded by %s.", len(culprits), req.CulpritIds)
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
	// TODO(wenbinzhang): clean up mocks
	//  mock subscription before the sheriff config is ready for production.
	subscription = PrepareSubscription(subscription, anomalygroup, s.config, "Culprit")

	issueIds := make([]string, 0)
	for _, culprit := range culprits {
		sklog.Debugf("[CP] Processing culprit %s.", culprit.Id)
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
	sklog.Debug("Notifying user of anomaly group: %s.", req.AnomalyGroupId)

	var err error
	anomalygroup, err := s.anomalygroupStore.LoadById(ctx, req.AnomalyGroupId)
	if err != nil {
		return nil, err
	}
	subscription, err := s.subscriptionStore.GetSubscription(ctx, anomalygroup.SubsciptionName, anomalygroup.SubscriptionRevision)
	if err != nil {
		return nil, err
	}
	// TODO(wenbinzhang): clean up mocks
	//  mock subscription before the sheriff config is ready for production.
	subscription = PrepareSubscription(subscription, anomalygroup, s.config, "Report")

	issueId, err := s.notifier.NotifyAnomaliesFound(ctx, anomalygroup, subscription, req.Anomaly)
	if err != nil {
		return nil, err
	}
	return &pb.NotifyUserOfAnomalyResponse{IssueId: issueId}, nil
}

// Temporary helper to make up a subscription or certain fields for testing purposes.
func PrepareSubscription(sub *sub_pb.Subscription, ag *v1.AnomalyGroup, config *config.InstanceConfig, suffix string) *sub_pb.Subscription {
	if sub == nil {
		// If no subscription is loaded, use a fake subscirption.
		sklog.Debugf("Cannot load subscription. Using mock. Name: %s, Revision: %s", ag.SubsciptionName, ag.SubscriptionRevision)
		sub = &sub_pb.Subscription{
			Name:         fmt.Sprintf("Mocked Sub For Anomaly - %s", suffix),
			Revision:     fmt.Sprintf("Mocked Revision - %s", suffix),
			BugLabels:    []string{"Mocked Sub Label"},
			Hotlists:     []string{"5141966"},
			BugComponent: "1325852",
			BugPriority:  2,
			BugSeverity:  3,
			BugCcEmails:  []string{"mordeckimarcin@google.com"},
			ContactEmail: "mordeckimarcin@google.com",
		}
	} else if config != nil && !slices.Contains(config.SheriffConfigsToNotify, sub.Name) {
		// If a subscription is loaded, but it is not in the allowlist, update the fields to avoid notifing end users.
		sklog.Debugf("Loaded subscription. Overwriting it. Name: %s, Revision: %s", sub.Name, sub.Revision)
		sub.BugLabels = []string{"Mocked Sub Label - overwrite"}
		sub.Hotlists = []string{"5141966"}
		sub.BugComponent = "1325852"
		sub.BugCcEmails = []string{"mordeckimarcin@google.com"}
		sub.ContactEmail = "mordeckimarcin@google.com"

	}

	return sub
}
