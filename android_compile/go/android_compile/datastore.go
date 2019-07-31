/*
	Used by the Android Compile Server to interact with the cloud datastore.
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/sklog"
)

var (
	ErrAnotherInstanceRunningTask        = errors.New("Another instance has picked up this task")
	ErrThisInstanceRunningTask           = errors.New("This instance is already running this task")
	ErrThisInstanceOwnsTaskButNotRunning = errors.New("This instance has picked up this task but it is not running yet")
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

	CompileServerInstance string `json:"compile_server_instance"`
	IsMasterBranch        bool   `json:"is_master_branch"`
	Done                  bool   `json:"done"`
	Error                 string `json:"error"`
	InfraFailure          bool   `json:"infra_failure"`
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

func ClaimCompileTask(issue, patchset int, lunchTarget string) error {
	var t CompileTask
	var err error
	_, err = ds.DS.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		fmt.Println("IN TRANSACTION!!!!!!!!!!!")
		k := GetNewDSKey(lunchTarget, issue, patchset)
		if err := tx.Get(k, &t); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		if t.CompileServerInstance != "" {
			if t.CompileServerInstance == serverURL {
				if t.Checkout == "" {
					sklog.Infof("%s has already been picked up by this instance but task is not running.", k)
					return ErrThisInstanceOwnsTaskButNotRunning
				} else {
					sklog.Infof("%s has already been picked up by this instance", k)
					return ErrThisInstanceRunningTask
				}
			} else {
				sklog.Infof("%s has been picked up by %s", k, t.CompileServerInstance)
				return ErrAnotherInstanceRunningTask
			}
		}
		// This instance is going to
		t.CompileServerInstance = serverURL
		if _, err := tx.Put(k, &t); err != nil {
			return err
		}
		fmt.Println("DONE WITH TRANSACTION!")
		return nil
	})
	return err
}

// TODO(rmistry):
// * Use transactions? - https://cloud.google.com/datastore/docs/concepts/transactions#datastore-datastore-transactional-update-go
// * Put instance here (optionally)?
// WHOLE FUNCTION IS NO LONGER NEEDED!
func GetOwnedCompileTasksAndKeys() ([]*CompileTaskAndKey, error) {
	ownedTasksAndKeys := []*CompileTaskAndKey{}

	it := GetOwnedPendingTasks()
	for {
		t := &CompileTask{}
		datastoreKey, err := it.Next(t)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		taskAndKey := &CompileTaskAndKey{task: t, key: datastoreKey}
		ownedTasksAndKeys = append(ownedTasksAndKeys, taskAndKey)
	}
	sort.Sort(sortTasks(ownedTasksAndKeys))

	return ownedTasksAndKeys, nil
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

func GetOwnedPendingTasks() *datastore.Iterator {
	q := ds.NewQuery(ds.COMPILE_TASK).EventualConsistency().Filter("Done =", false).Filter("CompileServerInstance =", serverURL)
	return ds.DS.Run(context.TODO(), q)
}

func GetAllPendingTasks() *datastore.Iterator {
	q := ds.NewQuery(ds.COMPILE_TASK).EventualConsistency().Filter("Done =", false)
	return ds.DS.Run(context.TODO(), q)
}

func GetNewDSKey(lunchTarget string, issue, patchset int) *datastore.Key {
	k := ds.NewKey(ds.COMPILE_TASK)
	k.Name = fmt.Sprintf("%s-%d-%d", lunchTarget, issue, patchset)
	return k
}

//func FindDSTask()

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
