/*
	Used by the Android Compile Server to interact with the cloud datastore.
*/

package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/ds"
)

type CompileTask struct {
	Issue    int    `json:"issue"`
	PatchSet int    `json:"patchset"`
	Hash     string `json:"hash"`

	Checkout string `json:"checkout"`

	Created   time.Time `json:"created"`
	Completed time.Time `json:"completed"`

	WithPatchSucceeded bool `json:"withpatch_success"`
	NoPatchSucceeded   bool `json:"nopatch_success"`

	WithPatchLog string `json:"withpatch_log"`
	NoPatchLog   string `json:"nopatch_log"`

	Done         bool `json:"done"`
	InfraFailure bool `json:"infra_failure"`
}

type CompileTaskAndKey struct {
	task *CompileTask
	key  *datastore.Key
}
type sortTasks []*CompileTaskAndKey

func (a sortTasks) Len() int      { return len(a) }
func (a sortTasks) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sortTasks) Less(i, j int) bool {
	return a[i].task.Created.Before(a[j].task.Created)
}

func GetCompileTasksAndKeys() ([]*CompileTaskAndKey, []*CompileTaskAndKey, error) {
	waitingTasksAndKeys := []*CompileTaskAndKey{}
	runningTasksAndKeys := []*CompileTaskAndKey{}

	it := GetPendingTasks()
	for {
		t := &CompileTask{}
		datastoreKey, err := it.Next(t)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		taskAndKey := &CompileTaskAndKey{task: t, key: datastoreKey}
		if t.Checkout == "" {
			waitingTasksAndKeys = append(waitingTasksAndKeys, taskAndKey)
		} else {
			runningTasksAndKeys = append(runningTasksAndKeys, taskAndKey)
		}
	}
	sort.Sort(sortTasks(waitingTasksAndKeys))
	sort.Sort(sortTasks(runningTasksAndKeys))

	return waitingTasksAndKeys, runningTasksAndKeys, nil
}

func GetCompileTasks() ([]*CompileTask, []*CompileTask, error) {
	waitingTasksAndKeys, runningTasksAndKeys, err := GetCompileTasksAndKeys()
	if err != nil {
		return nil, nil, err
	}
	waitingTasks := []*CompileTask{}
	for _, taskAndKey := range waitingTasksAndKeys {
		waitingTasks = append(waitingTasks, taskAndKey.task)
	}
	runningTasks := []*CompileTask{}
	for _, taskAndKey := range runningTasksAndKeys {
		runningTasks = append(runningTasks, taskAndKey.task)
	}
	return waitingTasks, runningTasks, nil
}

func DatastoreInit(project string, ns string) error {
	return ds.Init(project, ns)
}

func GetPendingTasks() *datastore.Iterator {
	q := ds.NewQuery(ds.COMPILE_TASK).EventualConsistency().Filter("Done =", false)
	return ds.DS.Run(context.TODO(), q)
}

func GetNewDSKey() *datastore.Key {
	return ds.NewKey(ds.COMPILE_TASK)
}

func GetDSTask(taskID int64) (*datastore.Key, *CompileTask, error) {
	key := ds.NewKey(ds.COMPILE_TASK)
	key.ID = taskID

	task := &CompileTask{}
	if err := ds.DS.Get(context.TODO(), key, task); err != nil {
		return nil, nil, fmt.Errorf("Error retrieving task from Datastore: %v", err)
	}
	return key, task, nil
}

func PutDSTask(ctx context.Context, k *datastore.Key, t *CompileTask) (*datastore.Key, error) {
	return ds.DS.Put(ctx, k, t)
}

func UpdateDSTask(ctx context.Context, k *datastore.Key, t *CompileTask) (*datastore.Key, error) {
	return ds.DS.Put(ctx, k, t)
}
