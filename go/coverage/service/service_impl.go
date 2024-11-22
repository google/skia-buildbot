package service

import (
	"context"
	"time"

	"go.skia.org/infra/go/coverage/coveragestore"
	pb "go.skia.org/infra/go/coverage/proto/v1"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/grpc"
)

const (
	// Those should be configurable for each instance.
	namespace     = "coverage-internal"
	taskQueue     = "coverage.coverage-chrome-public.bisect"
	databaseName  = "coverage"
	successStatus = "OK"
	failStatus    = "FAILED"
)

// coverageService implements coverage.CoverageService, provides a wrapper struct
// for the coverage service implementation.
type coverageService struct {
	pb.UnimplementedCoverageServiceServer
	coverageStore coveragestore.Store
}

// RegisterGrpc registers the grpc service with the server instance.
func (service *coverageService) RegisterGrpc(grpcServer *grpc.Server) {
	sklog.Info("Register Coverage Service")
	pb.RegisterCoverageServiceServer(grpcServer, service)
}

// GetServiceDescriptor returns the service descriptor for the service.
func (service *coverageService) GetServiceDescriptor() grpc.ServiceDesc {
	return pb.CoverageService_ServiceDesc
}

func New(coverageStore coveragestore.Store) *coverageService {
	return &coverageService{
		coverageStore: coverageStore,
	}
}

func checkContextDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	_, status := ctx.Deadline()
	if status {
		return ctx, nil
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	return ctxWithTimeout, cancel
}

// Checks file/builder pair from Database and returns available test suites.
func (s *coverageService) GetAllFiles(ctx context.Context, req *pb.CoverageRequest) (*pb.CoverageAllResponses, error) {
	updatedCtx, cancel := checkContextDeadline(ctx)
	results, err := s.coverageStore.ListAll(updatedCtx, req)
	if cancel != nil {
		defer cancel()
	}

	if err != nil {
		sklog.Errorf("GetAllSuite Failed: %v with error: %v", req, err)
	}
	var allResults pb.CoverageAllResponses
	allResults.Responses = results
	return &allResults, err
}

// Checks file/builder pair from Database and returns available test suites.
func (s *coverageService) GetTestSuite(ctx context.Context, req *pb.CoverageListRequest) (*pb.CoverageListResponse, error) {
	updatedCtx, cancel := checkContextDeadline(ctx)
	test_suites, err := s.coverageStore.List(updatedCtx, req)
	if cancel != nil {
		defer cancel()
	}

	status := successStatus
	if err != nil || test_suites == nil {
		status = failStatus
		sklog.Errorf("GetTestSuite Failed: %v with error: %v", req, err)
	}

	response := pb.CoverageListResponse{TestSuites: test_suites, Status: &status}
	return &response, err
}

// Inserts file/builder pair from Database.
func (s *coverageService) InsertFile(ctx context.Context, req *pb.CoverageChangeRequest) (*pb.CoverageChangeResponse, error) {
	updatedCtx, cancel := checkContextDeadline(ctx)
	err := s.coverageStore.Add(updatedCtx, req)
	if cancel != nil {
		defer cancel()
	}

	status := successStatus
	message := "Add Successful"

	if err != nil {
		status = failStatus
		message = err.Error()
		sklog.Errorf("InsertFile Failed: %s with error: %s", *req.FileName, err)
	}

	response := pb.CoverageChangeResponse{
		Status:  &status,
		Message: &message,
	}
	return &response, err
}

// Deletes file/builder pair from Database.
func (s *coverageService) DeleteFile(ctx context.Context, req *pb.CoverageChangeRequest) (*pb.CoverageChangeResponse, error) {
	updatedCtx, cancel := checkContextDeadline(ctx)
	err := s.coverageStore.Delete(updatedCtx, req)
	if cancel != nil {
		defer cancel()
	}

	status := successStatus
	message := "Delete Successful"

	if err != nil {
		status = failStatus
		message = err.Error()
		sklog.Errorf("Delete Failed: %s with error: %s", *req.FileName, err)
	}

	response := pb.CoverageChangeResponse{
		Status:  &status,
		Message: &message,
	}
	return &response, err
}
