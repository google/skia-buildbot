package culprit

import (
	"context"

	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
)

// Data Access layer for Culprit
type Store interface {
	// Get fetches stored culprits by ids
	Get(ctx context.Context, ids []string) ([]*pb.Culprit, error)
	// Get anomaly group ids sharing the provided issue id
	GetAnomalyGroupIdsForIssueId(ctx context.Context, issueId string) ([]string, error)
	// Upsert can write a new, or update an existing Culprit
	Upsert(ctx context.Context, anomaly_group_id string, commits []*pb.Commit) ([]string, error)
	// Add IssueId to a culprit id row
	AddIssueId(ctx context.Context, id string, issueId string, groupId string) error
}
