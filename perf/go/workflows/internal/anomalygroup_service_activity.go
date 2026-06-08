package internal

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	backend "go.skia.org/infra/perf/go/backend/client"
	"golang.org/x/time/rate"
)

type AnomalyGroupServiceActivity struct {
	insecureConn              bool
	legacyPinpointRateLimiter *rate.Limiter
}

func NewAnomalyGroupServiceActivity(insecure bool) *AnomalyGroupServiceActivity {
	return &AnomalyGroupServiceActivity{
		insecureConn: insecure,
		// Protects legacy Pinpoint from overloading with bisection job requests.
		// Set to 1 hour for testing purposes.
		legacyPinpointRateLimiter: rate.NewLimiter(rate.Every(time.Hour), 1),
	}
}

func (agsa *AnomalyGroupServiceActivity) CheckBisectionAllowed(ctx context.Context) (bool, error) {
	if agsa.legacyPinpointRateLimiter == nil {
		return false, skerr.Fmt("Legacy Pinpoint rate limiter is not initialized")
	}
	return agsa.legacyPinpointRateLimiter.Allow(), nil
}

func (agsa *AnomalyGroupServiceActivity) LoadAnomalyGroupByID(ctx context.Context, anomalygroupServiceUrl string, req *pb.LoadAnomalyGroupByIDRequest) (*pb.LoadAnomalyGroupByIDResponse, error) {
	client, err := backend.NewAnomalyGroupServiceClient(anomalygroupServiceUrl, agsa.insecureConn)
	if err != nil {
		return nil, err
	}
	resp, err := client.LoadAnomalyGroupByID(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (agsa *AnomalyGroupServiceActivity) FindTopAnomalies(ctx context.Context, anomalygroupServiceUrl string, req *pb.FindTopAnomaliesRequest) (*pb.FindTopAnomaliesResponse, error) {
	client, err := backend.NewAnomalyGroupServiceClient(anomalygroupServiceUrl, agsa.insecureConn)
	if err != nil {
		return nil, err
	}
	resp, err := client.FindTopAnomalies(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (agsa *AnomalyGroupServiceActivity) UpdateAnomalyGroup(ctx context.Context, anomalygroupServiceUrl string, req *pb.UpdateAnomalyGroupRequest) (*pb.UpdateAnomalyGroupResponse, error) {
	client, err := backend.NewAnomalyGroupServiceClient(anomalygroupServiceUrl, agsa.insecureConn)
	if err != nil {
		return nil, err
	}
	resp, err := client.UpdateAnomalyGroup(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
