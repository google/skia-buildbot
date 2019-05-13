package main

import (
	"sync"

	"go.skia.org/infra/go/metrics2"
)

var (
	// Metrics regarding number of waiting and running tasks.
	queueLengthMetric   = metrics2.GetCounter("android_compile_waiting_tasks", nil)
	runningLengthMetric = metrics2.GetCounter("android_compile_running_tasks", nil)
	// Mutex to control access to the above metrics.
	lengthMetricsMutex = sync.Mutex{}

	// Metric regarding broken android tree and it's mutex.
	androidTreeBrokenMetric      = metrics2.GetInt64Metric("android_compile_tree_broken", nil)
	androidTreeBrokenMetricMutex = sync.Mutex{}

	// Metric regarding mirror syncs. Does not need a mutex because the tree is
	// only updated after a mutex lock.
	mirrorSyncFailureMetric = metrics2.GetInt64Metric("android_compile_mirror_sync_failure", nil)
)

func resetMetrics() {
	queueLengthMetric.Reset()
	runningLengthMetric.Reset()
}

func updateAndroidTreeBrokenMetric(broken bool) {
	val := 0
	if broken {
		val = 1
	}
	androidTreeBrokenMetricMutex.Lock()
	defer androidTreeBrokenMetricMutex.Unlock()
	androidTreeBrokenMetric.Update(int64(val))
}

func moveToRunningMetric() {
	lengthMetricsMutex.Lock()
	defer lengthMetricsMutex.Unlock()
	queueLengthMetric.Dec(1)
	runningLengthMetric.Inc(1)
}

func decRunningMetric() {
	lengthMetricsMutex.Lock()
	defer lengthMetricsMutex.Unlock()
	runningLengthMetric.Dec(1)
}

func incWaitingMetric() {
	lengthMetricsMutex.Lock()
	defer lengthMetricsMutex.Unlock()
	queueLengthMetric.Inc(1)
}
