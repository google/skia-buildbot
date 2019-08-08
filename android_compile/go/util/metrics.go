package util

import (
	"sync"

	"go.skia.org/infra/go/metrics2"
)

var (
	// Metrics regarding number of waiting and running tasks.
	QueueLengthMetric   = metrics2.GetInt64Metric("android_compile_waiting_tasks", nil)
	RunningLengthMetric = metrics2.GetInt64Metric("android_compile_running_tasks", nil)

	// Metric regarding broken android tree and it's mutex.
	androidTreeBrokenMetric      = metrics2.GetInt64Metric("android_compile_tree_broken", nil)
	androidTreeBrokenMetricMutex = sync.Mutex{}

	// Metric regarding mirror syncs. Does not need a mutex because the tree is
	// only updated after a mutex lock.
	MirrorSyncFailureMetric = metrics2.GetInt64Metric("android_compile_mirror_sync_failure", nil)
)

func UpdateAndroidTreeBrokenMetric(broken bool) {
	val := 0
	if broken {
		val = 1
	}
	androidTreeBrokenMetricMutex.Lock()
	defer androidTreeBrokenMetricMutex.Unlock()
	androidTreeBrokenMetric.Update(int64(val))
}
