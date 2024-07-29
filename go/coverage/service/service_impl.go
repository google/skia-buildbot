package service

import (
	"context"

	"go.skia.org/infra/go/coverage/coveragestore"
	pb "go.skia.org/infra/go/coverage/proto/v1"
	"go.skia.org/infra/go/sklog"
)

type server struct {
	pb.UnimplementedCoverageServiceServer
	store coveragestore.Store
}

const (
	// Those should be configurable for each instance.
	namespace     = "coverage-internal"
	taskQueue     = "coverage.coverage-chrome-public.bisect"
	databaseName  = "coverage"
	successStatus = "OK"
	failStatus    = "FAILED"
)

func New(store coveragestore.Store) *server {
	return &server{
		store: store,
	}
}

// Checks file/builder pair from Database and returns available test suites.
func (s *server) GetTestSuite(ctx context.Context, req *pb.CoverageRequest) (*pb.CoverageListResponse, error) {
	test_suites, err := s.store.List(ctx, req)

	status := successStatus
	if err != nil || test_suites == nil {
		test_suites = req.TestSuiteName
		status = failStatus
		sklog.Errorf("GetTestSuite Failed: %v with error: %v", test_suites, err)
	}

	response := pb.CoverageListResponse{FileName: req.FileName, BuilderName: req.BuilderName,
		TestSuites: test_suites, Status: &status}
	return &response, err
}

// Inserts file/builder pair from Database.
func (s *server) InsertFile(ctx context.Context, req *pb.CoverageRequest) (*pb.CoverageChangeResponse, error) {
	err := s.store.Add(ctx, req)
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
func (s *server) DeleteFile(ctx context.Context, req *pb.CoverageRequest) (*pb.CoverageChangeResponse, error) {
	err := s.store.Delete(ctx, req)
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
