package metrics2

import (
	"runtime"
	"time"
)

const (
	RUNTIME_STATS_FREQUENCY = time.Minute
)

func newRuntimeStat(metric string) *Int64Metric {
	return GetInt64Metric("runtime-metrics", map[string]string{"metric": metric})
}

// RuntimeMetrics periodically reports runtime metrics.
func RuntimeMetrics() {
	heapObjects := newRuntimeStat("heap-objects")
	heapInuse := newRuntimeStat("heap-in-use")
	pauseTotalNs := newRuntimeStat("pause-total-ns")
	numGoroutine := newRuntimeStat("num-goroutine")
	go func() {
		for _ = range time.Tick(RUNTIME_STATS_FREQUENCY) {
			stats := new(runtime.MemStats)
			runtime.ReadMemStats(stats)

			heapObjects.Update(int64(stats.HeapObjects))
			heapInuse.Update(int64(stats.HeapInuse))
			pauseTotalNs.Update(int64(stats.PauseTotalNs))
			numGoroutine.Update(int64(runtime.NumGoroutine()))
		}
	}()

	// App uptime.
	_ = NewLiveness("uptime", nil)
}
