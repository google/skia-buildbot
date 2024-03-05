package culprit

import (
	"context"

	pb "go.skia.org/infra/perf/go/culprit/proto"
)

// Data Access layer for Culprit
type Store interface {
	// Upsert can write a new, or update an existing Culprit
	Upsert(ctx context.Context, anomaly_group_id string, culprit []*pb.Culprit) error
}
