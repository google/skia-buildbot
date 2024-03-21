package service

import (
	"context"

	"go.skia.org/infra/perf/go/backend/shared"
	"go.skia.org/infra/perf/go/culprit"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"google.golang.org/grpc"
)

// culpritService implements CulpritService
type culpritService struct {
	pb.UnimplementedCulpritServiceServer
	store culprit.Store
}

// New returns a new instance of culpritService.
func New(store culprit.Store) *culpritService {
	return &culpritService{
		store: store,
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

func (s *culpritService) PersistCulprit(context context.Context, req *pb.PersistCulpritRequest) (*pb.PersistCulpritResponse, error) {
	ids, err := s.store.Upsert(context, req.AnomalyGroupId, req.Commits)
	if err != nil {
		return nil, err
	} else {
		return &pb.PersistCulpritResponse{CulpritIds: ids}, nil
	}
	// TODO(pasthana): Update anomaly group once anomaly table is available in production
	// notifyReq := &pb.NotifyUserRequest{
	// 	Culprits:       req.Culprits,
	// 	AnomalyGroupId: req.AnomalyGroupId,
	// }
	// notifyResp, err := s.NotifyUser(context, notifyReq)
	// if err != nil {
	// 	return nil, err
	// }
	// response := &pb.PersistCulpritResponse{
	// 	IssueId: notifyResp.IssueId,
	// }
	// return response, nil
}

func (s *culpritService) GetCulprit(context context.Context, req *pb.GetCulpritRequest) (*pb.GetCulpritResponse, error) {
	culprits, err := s.store.Get(context, req.CulpritIds)
	if err != nil {
		return nil, err
	}
	return &pb.GetCulpritResponse{
		Culprits: culprits,
	}, nil
}
