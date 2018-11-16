package flakes

/*
   Find flakily-failed tasks in a time window.
*/

import (
	"go.skia.org/infra/task_scheduler/go/db"
)

// Find flakily-failed tasks in the given slice of tasks.
func FindFlakes(tasks []*db.Task) []*db.Task {
	tasksMap := make(map[string]*db.Task, len(tasks))
	for _, task := range tasks {
		tasksMap[task.Id] = task
	}
	flaky := []*db.Task{}
	for _, task := range tasks {
		if !task.Done() {
			continue
		}
		// Tasks which failed but whose retries succeeded are flakes.
		if task.Attempt > 0 && task.Status == db.TASK_STATUS_SUCCESS {
			currentTask := task
			for {
				// If the original task is in the slice, add it to the
				// results. If not, assume that it's out of the time
				// window and we don't care about it.
				flakyTask, ok := tasksMap[currentTask.RetryOf]
				if ok {
					// Don't double-count retried mishaps.
					if flakyTask.Status != db.TASK_STATUS_MISHAP {
						flaky = append(flaky, flakyTask)
					}
				} else {
					break
				}
				if currentTask.RetryOf == "" {
					break
				}
				currentTask = flakyTask
			}
		}
		// Mishaps are flakes by definition.
		if task.Status == db.TASK_STATUS_MISHAP {
			flaky = append(flaky, task)
		}
	}
	return flaky
}
