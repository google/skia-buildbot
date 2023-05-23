package anomalies

import (
	"context"

	"go.skia.org/infra/perf/go/types"
)

// CommitNumberAnomalyMap is a map of Anomaly, keyed by commit number.
type CommitNumberAnomalyMap map[types.CommitNumber]Anomaly

// AnomalyMap is a map of CommitNumberAnomalyMap, keyed by traceId.
type AnomalyMap map[string]CommitNumberAnomalyMap

// Anomaly defines the object return from Chrome Perf API.
type Anomaly struct {
	Id                  int     `json:"id"`
	TestPath            string  `json:"test_path"`
	BugId               int     `json:"bug_id"`
	StartRevision       int     `json:"start_revision"`
	EndRevision         int     `json:"end_revision"`
	IsImprovement       bool    `json:"is_improvement"`
	Recovered           bool    `json:"recovered"`
	State               string  `json:"state"`
	Statistics          string  `json:"statistic"`
	Unit                string  `json:"units"`
	DegreeOfFreedom     float64 `json:"degrees_of_freedom"`
	MedianBeforeAnomaly float64 `json:"median_before_anomaly"`
	MedianAfterAnomaly  float64 `json:"median_after_anomaly"`
	PValue              float64 `json:"p_value"`
	SegmentSizeAfter    int     `json:"segment_size_after"`
	SegmentSizeBefore   int     `json:"segment_size_before"`
	StdDevBeforeAnomaly float64 `json:"std_dev_before_anomaly"`
	TStatistics         float64 `json:"t_statistic"`
}

// Store provides the interface to get anomalies.
type Store interface {
	// GetAnomalies retrieve anomalies for each trace within the begin commit and end commit.
	GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (AnomalyMap, error)
}
