package anomalies

import (
	"context"
	"time"

	"go.skia.org/infra/perf/go/chromeperf"
)

// Store provides the interface to get anomalies.
type Store interface {
	// GetAnomalies retrieve anomalies for each trace within the begin commit and end commit.
	GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (chromeperf.AnomalyMap, error)

	// GetAnomaliesInTimeRange retrieve anomalies for each trace within the begin and end times.
	GetAnomaliesInTimeRange(ctx context.Context, traceNames []string, startTime time.Time, endTime time.Time) (chromeperf.AnomalyMap, error)

	// GetAnomaliesAroundRevision retrieves traces with anomalies that were generated around a specific commit
	GetAnomaliesAroundRevision(ctx context.Context, revision int) ([]chromeperf.AnomalyForRevision, error)
}
