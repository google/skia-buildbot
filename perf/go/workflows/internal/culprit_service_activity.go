package internal

import (
	"context"

	"go.skia.org/infra/go/sklog"
	backend "go.skia.org/infra/perf/go/backend/client"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
)

type CulpritServiceActivity struct {
	insecure_conn bool
}

func (csa *CulpritServiceActivity) PeristCulprit(ctx context.Context, culpritServiceUrl string, req *pb.PersistCulpritRequest) (*pb.PersistCulpritResponse, error) {
	client, err := backend.NewCulpritServiceClient(culpritServiceUrl, csa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := client.PersistCulprit(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (csa *CulpritServiceActivity) NotifyUserOfCulprit(ctx context.Context, culpritServiceUrl string, req *pb.NotifyUserOfCulpritRequest) (*pb.NotifyUserOfCulpritResponse, error) {
	client, err := backend.NewCulpritServiceClient(culpritServiceUrl, csa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := client.NotifyUserOfCulprit(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (csa *CulpritServiceActivity) NotifyUserOfAnomaly(ctx context.Context, culpritServiceUrl string, req *pb.NotifyUserOfAnomalyRequest) (*pb.NotifyUserOfAnomalyResponse, error) {
	sklog.Debugf("[AG] Notify user of anomalies: %s. Group ID: %s", req.Anomaly, req.AnomalyGroupId)
	client, err := backend.NewCulpritServiceClient(culpritServiceUrl, csa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := client.NotifyUserOfAnomaly(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
