package anomalies

import (
	"go.skia.org/infra/perf/go/types"
)

// CommitNumberAnomalyMap is a map of Anomaly, keyed by commit number.
type CommitNumberAnomalyMap map[types.CommitNumber]Anomaly

// AnomalyMap is a map of CommitNumberAnomalyMap, keyed by traceId.
type AnomalyMap map[string]CommitNumberAnomalyMap

// Anomaly defines the object return from Chrome Perf API.
type Anomaly struct {
	Id                  string
	TestPath            string
	BugId               string
	StartRevision       int
	EndRevision         int
	IsImprovement       bool
	Recovered           bool
	State               string
	Statistics          string
	Unit                string
	DegreeOfFreedom     float64
	MedianBeforeAnomaly float64
	MedianAfterAnomaly  float64
	PValue              float64
	SegmentSizeAfter    int
	SegmentSizeBefore   int
	StdDevBeforeAnomaly float64
	TStatistics         float64
}

// Store provides the interface to get anomalies.
type Store interface {
	// GetAnomalies retrieve anomalies for each trace within the begin commit and end commit.
	GetAnomalies(traceNames []string, startCommitPosition int, endCommitPosition int) (AnomalyMap, error)
}
