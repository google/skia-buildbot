package internal

import (
	"context"

	pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	backend "go.skia.org/infra/perf/go/backend/client"
)

type AnomalyGroupServiceActivity struct {
	insecure_conn bool
}

func (agsa *AnomalyGroupServiceActivity) LoadAnomalyGroupByID(ctx context.Context, anomalygroupServiceUrl string, req *pb.LoadAnomalyGroupByIDRequest) (*pb.LoadAnomalyGroupByIDResponse, error) {
	client, err := backend.NewAnomalyGroupServiceClient(anomalygroupServiceUrl, agsa.insecure_conn)
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
	client, err := backend.NewAnomalyGroupServiceClient(anomalygroupServiceUrl, agsa.insecure_conn)
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
	client, err := backend.NewAnomalyGroupServiceClient(anomalygroupServiceUrl, agsa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := client.UpdateAnomalyGroup(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
