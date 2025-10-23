package anomalygroup

import (
	"context"

	pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
)

// Data access layer for anomaly group.
type Store interface {
	// Create a new anomaly group record.
	// Example use case: when a new anomaly is detected and no associted
	//   group can be found.
	Create(ctx context.Context,
		subscription_name string,
		subscription_revision string,
		domain_name string,
		benchmark_name string,
		start_commit int64,
		end_commit int64,
		action string) (string, error)

	// Load the anomaly group by its ID.
	// Example use case: when the orchestrator try to take action on the
	//   group, it will load the group for the info needed.
	LoadById(ctx context.Context, group_id string) (*pb.AnomalyGroup, error)

	// Get all anomaly IDs associated with an issue ID.
	GetAnomalyIdsByIssueId(ctx context.Context, issueId string) ([]string, error)

	// Get all anomaly IDs associated with this group ID.
	GetAnomalyIdsByAnomalyGroupId(ctx context.Context, anomalyGroupId string) ([]string, error)

	// Get all anomaly IDs associated with those groups.
	GetAnomalyIdsByAnomalyGroupIds(ctx context.Context, anomalyGroupIds []string) ([]string, error)

	FindExistingGroup(
		ctx context.Context,
		subscription_name string,
		subscription_revision string,
		domain_name string,
		benchmark_name string,
		start_commit int64,
		end_commit int64,
		action string) ([]*pb.AnomalyGroup, error)

	// Update the bisection id for an anomaly group.
	// Example use case: if the group's action is BISECT, we will launch
	//   a bisection job. The job id will be saved here.
	UpdateBisectID(ctx context.Context, group_id string, bisection_id string) error

	// Update the reported issue id for an anomaly group.
	// Example use case: if the group's action is REPORT, we will file a
	//	 bug for it. The bug id will be saved here.
	UpdateReportedIssueID(ctx context.Context, group_id string, reported_issue_id string) error

	// Add one anoamly ID to an anomaly group's anomaly list.
	// When a new anomaly is added, the group's commit range is narrowed to the
	// intersection of its current range and the new anomaly's range. This ensures
	// that all anomalies within the group share a common revision range, preventing
	// the grouping of anomalies that do not overlap in time.
	AddAnomalyID(ctx context.Context, group_id string, anomaly_id string, anomaly_start_commit int64, anomaly_end_commit int64) error

	// Add culprit IDs to an anomaly group's culprit list.
	// Example use case: when an auto bisection job finished with
	// culprit(s) detected.
	AddCulpritIDs(ctx context.Context, group_id string, culprit_ids []string) error
}
