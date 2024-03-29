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

	// Query(ctx context.Context, kvp map[string]interface{}) []service.AnomalyGroup

	// Update the bisection id for an anomaly group.
	// Example use case: if the group's action is BISECT, we will launch
	//   a bisection job. The job id will be saved here.
	UpdateBisectID(ctx context.Context, group_id string, bisection_id string) error

	// Update the reported issue id for an anomaly group.
	// Example use case: if the group's action is REPORT, we will file a
	//	 bug for it. The bug id will be saved here.
	UpdateReportedIssueID(ctx context.Context, group_id string, reported_issue_id string) error

	// Add one anoamly ID to an anomaly group's anomaly list.
	// Example use case: when a new anomaly is detected and associated
	//   with an existing group.
	AddAnomalyID(ctx context.Context, group_id string, anomaly_id string) error

	// Add culprit IDs to an anomaly group's culprit list.
	// Example use case: when an auto bisection job finished with
	// culprit(s) detected.
	AddCulpritIDs(ctx context.Context, group_id string, culprit_ids []string) error
}
