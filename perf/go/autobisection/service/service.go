package service

import (
	"context"

	"go.skia.org/infra/perf/go/autobisection"
	pb "go.skia.org/infra/perf/go/autobisection/proto/v1"
	"go.skia.org/infra/perf/go/autobisection/sqlautobisectionstore/schema"
	"go.skia.org/infra/perf/go/backend/shared"
	"google.golang.org/grpc"
)

// autobisectionService implements pb.AutobisectionServiceServer
type autobisectionService struct {
	pb.UnimplementedAutobisectionServiceServer
	store autobisection.Store
}

// New returns a new instance of autobisectionService.
func New(store autobisection.Store) *autobisectionService {
	return &autobisectionService{
		store: store,
	}
}

// RegisterGrpc implements backend.BackendService
func (s *autobisectionService) RegisterGrpc(server *grpc.Server) {
	pb.RegisterAutobisectionServiceServer(server, s)
}

// GetAuthorizationPolicy implements backend.BackendService
func (s *autobisectionService) GetAuthorizationPolicy() shared.AuthorizationPolicy {
	// Add proper authorization policy if needed. Following culprit's model for now.
	return shared.AuthorizationPolicy{
		AllowUnauthenticated: true,
	}
}

// GetServiceDescriptor implements backend.BackendService
func (s *autobisectionService) GetServiceDescriptor() grpc.ServiceDesc {
	return pb.AutobisectionService_ServiceDesc
}

// SaveAutobisection saves the result of a autobisection into the store.
func (s *autobisectionService) SaveAutobisection(ctx context.Context, req *pb.SaveAutobisectionRequest) (*pb.SaveAutobisectionResponse, error) {
	autobisectionResult := &schema.AutobisectionSchema{
		JobID:          req.JobId,
		AnomalyGroupID: req.AnomalyGroupId,
		AnomalyId:      req.AnomalyId,
	}

	if err := s.store.Save(ctx, autobisectionResult); err != nil {
		return nil, err
	}

	return &pb.SaveAutobisectionResponse{}, nil
}
