package anomalies

import (
	"context"
	"sort"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/chromeperf"
)

// store implements anomalies.Store.
type store struct {
	ChromePerf chromeperf.AnomalyApiClient
}

// New returns a new anomalies.Store instance .
func New(chromePerf chromeperf.AnomalyApiClient) (*store, error) {
	ret := &store{
		ChromePerf: chromePerf,
	}
	return ret, nil
}

// GetAnomalies implements anomalies.Store
// It calls chrome perf API to fetch the anomalies for the traces within the commit range.
func (as *store) GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (chromeperf.AnomalyMap, error) {
	result := chromeperf.AnomalyMap{}
	// Get anomalies from Chrome Perf
	sort.Strings(traceNames)
	chromePerfAnomalies, err := as.ChromePerf.GetAnomalies(ctx, traceNames, startCommitPosition, endCommitPosition)
	if err != nil {
		sklog.Errorf("Failed to get chrome perf anomalies: %s", err)
	} else {
		for traceName, commitNumberAnomalyMap := range chromePerfAnomalies {
			result[traceName] = commitNumberAnomalyMap
		}
	}

	return result, nil
}

// GetAnomaliesTimeBased implements anomalies.Store
// Retrieves anomalies for each trace within the begin and end times.
func (as *store) GetAnomaliesInTimeRange(ctx context.Context, traceNames []string, startTime time.Time, endTime time.Time) (chromeperf.AnomalyMap, error) {
	ctx, span := trace.StartSpan(ctx, "anomalies.store.GetAnomaliesInTimeRange")
	defer span.End()
	result := chromeperf.AnomalyMap{}
	if len(traceNames) == 0 {
		return result, nil
	}

	sort.Strings(traceNames)

	chromePerfAnomalies, err := as.ChromePerf.GetAnomaliesTimeBased(ctx, traceNames, startTime, endTime)
	if err != nil {
		sklog.Errorf("Failed to get chrome perf anomalies: %s", err)
	} else {
		for traceName, commitNumberAnomalyMap := range chromePerfAnomalies {
			result[traceName] = commitNumberAnomalyMap
		}
	}

	return result, nil
}

// GetAnomaliesAroundRevision implements anomalies.Store
// It fetches anomalies that occured around the specified revision number.
func (as *store) GetAnomaliesAroundRevision(ctx context.Context, revision int) ([]chromeperf.AnomalyForRevision, error) {
	ctx, span := trace.StartSpan(ctx, "anomalies.store.GetAnomaliesAroundRevision")
	defer span.End()
	result, err := as.ChromePerf.GetAnomaliesAroundRevision(ctx, revision)
	if err != nil {
		return nil, err
	}
	return result, nil
}
