package anomalygroup

import (
	"context"

	pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
)

// Data access layer for anomaly group.
type Store interface {
	Create(ctx context.Context,
		subscription_name string,
		subscription_revision string,
		domain_name string,
		benchmark_name string,
		start_commit int64,
		end_commit int64,
		action string) (string, error)

	LoadById(ctx context.Context, group_id string) (*pb.AnomalyGroup, error)

	// Query(ctx context.Context, kvp map[string]interface{}) []service.AnomalyGroup

	// Update(ctx context.Context, group_id string, kvp map[string]interface{}) []string
}
