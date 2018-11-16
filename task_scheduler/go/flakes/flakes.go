package flakes

/*
   Find flakily-failed tasks in a time window.
*/

import (
	"go.skia.org/infra/task_scheduler/go/db"
)

// Find flakily-failed tasks in the given slice of tasks.
func FindFlakes(tasks []*db.Task) []*db.Task {
	tasksMap := map[db.TaskKey][]*db.Task{}
	for _, task := range tasks {
		if task.Done() {
			tasksMap[task.TaskKey] = append(tasksMap[task.TaskKey], task)
		}
	}
	flaky := []*db.Task{}
	for _, tasks := range tasksMap {
		// If one or more tasks succeeded and failed, then all failures
		// are flakes.
		success := 0
		failure := 0
		for _, task := range tasks {
			if task.Status == db.TASK_STATUS_SUCCESS {
				success++
			} else if task.Status == db.TASK_STATUS_FAILURE {
				failure++
			} else if task.Status == db.TASK_STATUS_MISHAP {
				// Mishaps are flakes by definition.
				flaky = append(flaky, task)
			}
		}
		if success > 0 && failure > 0 {
			for _, task := range tasks {
				if task.Status == db.TASK_STATUS_FAILURE {
					flaky = append(flaky, task)
				}
			}
		}
	}
	return flaky
}
