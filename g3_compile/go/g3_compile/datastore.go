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
	"go.skia.org/infra/go/ds"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type CompileTask struct {
	Issue    int    `json:"issue"`
	PatchSet int    `json:"patchset"`
	Hash     string `json:"hash"`

	LunchTarget string `json:"lunch_target"`
	MMMATargets string `json:"mmma_targets"`

	Checkout string `json:"checkout"`

	Created   time.Time `json:"created"`
	Completed time.Time `json:"completed"`

	WithPatchSucceeded bool `json:"withpatch_success"`
	NoPatchSucceeded   bool `json:"nopatch_success"`

	WithPatchLog string `json:"withpatch_log"`
	NoPatchLog   string `json:"nopatch_log"`

	IsMasterBranch bool `json:"is_master_branch"`
	Done           bool `json:"done"`
	InfraFailure   bool `json:"infra_failure"`
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

func DatastoreInit(project string, ns string, ts oauth2.TokenSource) error {
	return ds.InitWithOpt(project, ns, option.WithTokenSource(ts))
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
