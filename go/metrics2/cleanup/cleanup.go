package metrics_cleanup

/*
	Package metrics_cleanup provides helpers for producing metrics on the fly
	and cleaning them up.
*/

import (
	"context"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// DoMetricsWithCleanup encapsulates the process of running a function to
// generate on-the-fly metrics, tracking success or failure of that generation,
// and cleaning up metrics which no longer exist.
func DoMetricsWithCleanup(ctx context.Context, frequency time.Duration, lv metrics2.Liveness, doMetrics func(context.Context, time.Time) ([]metrics2.Int64Metric, error)) {
	oldMetrics := map[metrics2.Int64Metric]struct{}{}
	go util.RepeatCtx(ctx, frequency, func(ctx context.Context) {
		newMetrics, err := doMetrics(ctx, time.Now())
		if err != nil {
			sklog.Errorf("Failed to update metrics: %s", err)
		} else {
			newMetricsMap := make(map[metrics2.Int64Metric]struct{}, len(newMetrics))
			for _, m := range newMetrics {
				newMetricsMap[m] = struct{}{}
			}
			var cleanup []metrics2.Int64Metric
			for m := range oldMetrics {
				if _, ok := newMetricsMap[m]; !ok {
					cleanup = append(cleanup, m)
				}
			}
			if len(cleanup) > 0 {
				failedDelete := []metrics2.Int64Metric{}
				for m := range oldMetrics {
					if err := m.Delete(); err != nil {
						sklog.Warningf("Failed to delete metric: %s", err)
						failedDelete = append(failedDelete, m)
					}
				}
				for _, m := range failedDelete {
					newMetricsMap[m] = struct{}{}
				}
			}
			oldMetrics = newMetricsMap
			lv.Reset()
		}
	})
}
