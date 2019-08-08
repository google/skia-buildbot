/*
	Used by the Android Compile Server to interact with the cloud datastore.
*/

package util

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

type AndroidCompileInstance struct {
	MirrorLastSynced     string `json:"mirror_last_synced"`
	MirrorUpdateDuration string `json:"mirror_update_duration"`
	ForceMirrorUpdate    bool   `json:"force_mirror_update"`
	Name                 string `json:"name"`
}

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

type sortTasks []*CompileTask

func (a sortTasks) Len() int      { return len(a) }
func (a sortTasks) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sortTasks) Less(i, j int) bool {
	return a[i].Created.Before(a[j].Created)
}

// ClaimAndAddCompileTask adds the compile task to the datastore and marks it
// as being owned by the specified instance.
// The function throws the following custom errors:
// * ErrThisInstanceOwnsTaskButNotRunning - Thrown when the specified instance
//     owns the task but it is not running yet.
// * ErrThisInstanceRunningTask - Thrown when the specified instance owns the task
// *   and it is currently running.
// * ErrAnotherInstanceRunningTask - Thrown when another instance (not the specified
//     instance) owns the task.
func ClaimAndAddCompileTask(taskFromGS *CompileTask, currentInstance string) error {
	var err error
	_, err = ds.DS.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		var taskFromDS CompileTask
		// Use the task from GS to construct the Key and look in Datastore.
		k := GetTaskDSKey(taskFromGS.LunchTarget, taskFromGS.Issue, taskFromGS.PatchSet)
		if err := tx.Get(k, &taskFromDS); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		if taskFromDS.Done {
			sklog.Infof("%s exists in Datastore and is completed but there was a new request for it. Running it..", k)
		} else if taskFromDS.CompileServerInstance != "" {
			if taskFromDS.CompileServerInstance == currentInstance {
				if taskFromDS.Checkout == "" {
					sklog.Infof("%s has already been picked up by this instance but task is not running.", k)
					return ErrThisInstanceOwnsTaskButNotRunning
				} else {
					sklog.Infof("%s has already been picked up by this instance", k)
					return ErrThisInstanceRunningTask
				}
			} else {
				sklog.Infof("%s has been picked up by %s", k, taskFromDS.CompileServerInstance)
				return ErrAnotherInstanceRunningTask
			}
		}
		// Populate some taskFromGS properties before adding to datastore.
		taskFromGS.CompileServerInstance = currentInstance
		taskFromGS.Created = time.Now()
		if _, err := tx.Put(k, taskFromGS); err != nil {
			return err
		}
		return nil
	})
	return err
}

// AddUnownedCompileTask adds the task to the datastore without an owner instance.
// Task is added to the datastore if it does not already exist in the datastore or
// if it exists but is marked as completed.
func AddUnownedCompileTask(taskFromGS *CompileTask) error {
	var err error
	_, err = ds.DS.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		var taskFromDS CompileTask
		k := GetTaskDSKey(taskFromGS.LunchTarget, taskFromGS.Issue, taskFromGS.PatchSet)
		if err := tx.Get(k, &taskFromDS); err != nil {
			if err == datastore.ErrNoSuchEntity {
				// If task does not exist then add it as a pending task.
				taskFromGS.Created = time.Now()
				if _, err := tx.Put(k, taskFromGS); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if taskFromDS.Done {
			// Task is in Datastore and has completed, but a new request
			// has come in so override the old task.
			taskFromGS.Created = time.Now()
			if _, err := tx.Put(k, taskFromGS); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// GetPendingCompileTasks returns slices of unowned tasks and currently running
// (but not yet completed) tasks.
func GetPendingCompileTasks(ownedByInstance string) ([]*CompileTask, []*CompileTask, error) {
	// Pending tasks that have not been picked up by an instance yet.
	unownedPendingTasks := []*CompileTask{}
	// Pending tasks that have been picked up by an instance.
	ownedPendingTasks := []*CompileTask{}

	q := ds.NewQuery(ds.COMPILE_TASK).EventualConsistency().Filter("Done =", false)
	it := ds.DS.Run(context.TODO(), q)
	for {
		t := &CompileTask{}
		_, err := it.Next(t)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		if t.CompileServerInstance == "" {
			unownedPendingTasks = append(unownedPendingTasks, t)
		} else {
			if ownedByInstance == "" {
				ownedPendingTasks = append(ownedPendingTasks, t)
			} else if t.CompileServerInstance == ownedByInstance {
				ownedPendingTasks = append(ownedPendingTasks, t)
			}
		}
	}
	sort.Sort(sortTasks(unownedPendingTasks))
	sort.Sort(sortTasks(ownedPendingTasks))

	return unownedPendingTasks, ownedPendingTasks, nil
}

func DatastoreInit(project, ns string, ts oauth2.TokenSource) error {
	return ds.InitWithOpt(project, ns, option.WithTokenSource(ts))
}

func UpdateInstanceInDS(ctx context.Context, hostname, mirrorLastSynced string, mirrorUpdateDuration time.Duration, forceMirrorUpdate bool) error {
	k := GetInstanceDSKey(hostname)
	i := AndroidCompileInstance{
		MirrorLastSynced:     mirrorLastSynced,
		MirrorUpdateDuration: mirrorUpdateDuration.String(),
		ForceMirrorUpdate:    forceMirrorUpdate,
		Name:                 hostname,
	}
	_, err := ds.DS.Put(ctx, k, &i)
	return err
}

func GetAllCompileInstances(ctx context.Context) ([]*AndroidCompileInstance, error) {
	var instances []*AndroidCompileInstance
	q := ds.NewQuery(ds.ANDROID_COMPILE_INSTANCES)
	_, err := ds.DS.GetAll(ctx, q, &instances)
	return instances, err
}

func SetForceMirrorUpdateOnAllInstances(ctx context.Context) error {
	var instances []*AndroidCompileInstance
	q := ds.NewQuery(ds.ANDROID_COMPILE_INSTANCES)
	if _, err := ds.DS.GetAll(ctx, q, &instances); err != nil {
		return err
	}
	for _, i := range instances {
		i.ForceMirrorUpdate = true
		if _, err := ds.DS.Put(ctx, GetInstanceDSKey(i.Name), i); err != nil {
			return err
		}
	}
	return nil
}

func GetForceMirrorUpdateBool(ctx context.Context, hostname string) (bool, error) {
	k := GetInstanceDSKey(hostname)
	var i AndroidCompileInstance
	if err := ds.DS.Get(ctx, k, &i); err != nil {
		return false, err
	}
	return i.ForceMirrorUpdate, nil
}

func GetInstanceDSKey(hostname string) *datastore.Key {
	k := ds.NewKey(ds.ANDROID_COMPILE_INSTANCES)
	k.Name = hostname
	return k
}

func GetTaskDSKey(lunchTarget string, issue, patchset int) *datastore.Key {
	k := ds.NewKey(ds.COMPILE_TASK)
	k.Name = fmt.Sprintf("%s-%d-%d", lunchTarget, issue, patchset)
	return k
}

func UpdateTaskInDS(ctx context.Context, t *CompileTask) (*datastore.Key, error) {
	k := GetTaskDSKey(t.LunchTarget, t.Issue, t.PatchSet)
	return ds.DS.Put(ctx, k, t)
}
