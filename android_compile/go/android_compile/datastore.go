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
	Issue    int `json:"issue"`
	PatchSet int `json:"patchset"`

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

func ClaimAndAddCompileTask(taskFromGS *CompileTask) error {
	var err error
	_, err = ds.DS.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		fmt.Println("IN TRANSACTION!!!!!!!!!!!")
		var taskFromDS CompileTask
		// Use the specified task from GS to construct the Key and look in Datastore.
		k := GetDSKey(taskFromGS.LunchTarget, taskFromGS.Issue, taskFromGS.PatchSet)
		if err := tx.Get(k, &taskFromDS); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		if taskFromDS.CompileServerInstance != "" {
			if taskFromDS.CompileServerInstance == serverURL {
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
		// Populate taskFromGS properties before adding to datastore.
		taskFromGS.CompileServerInstance = serverURL
		if _, err := tx.Put(k, taskFromGS); err != nil {
			fmt.Println("XXXXXXXXXXXXXXXXXX")
			fmt.Println(taskFromGS)
			//fmt.Println(type(taskFromGS.Checkout))
			fmt.Println(taskFromGS.CompileServerInstance)
			fmt.Printf("%T\n", taskFromGS.Issue)
			fmt.Printf("%T\n", taskFromGS.PatchSet)
			//fmt.Println(type(taskFromGS.LunchTarget))
			//fmt.Println(type(taskFromGS.MMMATargets))
			return err
		}
		fmt.Println("DONE WITH TRANSACTION!")
		return nil
	})
	return err
}

func GetPendingCompileTasks(runByThisInstance bool) ([]*CompileTask, []*CompileTask, error) {
	waitingTasks := []*CompileTask{}
	runningTasks := []*CompileTask{}

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
			waitingTasks = append(waitingTasks, t)
		} else {
			if runByThisInstance && t.CompileServerInstance == serverURL {
				fmt.Println("OWNED BY THIS INSTANCE!!!")
				fmt.Println(t)
				runningTasks = append(runningTasks, t)
			} else {
				runningTasks = append(runningTasks, t)
			}
		}
	}
	sort.Sort(sortTasks(waitingTasks))
	sort.Sort(sortTasks(runningTasks))

	return waitingTasks, runningTasks, nil
}

func DatastoreInit(project string, ns string, ts oauth2.TokenSource) error {
	return ds.InitWithOpt(project, ns, option.WithTokenSource(ts))
}

func GetDSKey(lunchTarget string, issue, patchset int) *datastore.Key {
	k := ds.NewKey(ds.COMPILE_TASK)
	k.Name = fmt.Sprintf("%s-%d-%d", lunchTarget, issue, patchset)
	return k
}

func PutDSTask(ctx context.Context, t *CompileTask) (*datastore.Key, error) {
	k := GetDSKey(t.LunchTarget, t.Issue, t.PatchSet)
	return ds.DS.Put(ctx, k, t)
}

func UpdateDSTask(ctx context.Context, t *CompileTask) (*datastore.Key, error) {
	k := GetDSKey(t.LunchTarget, t.Issue, t.PatchSet)
	return ds.DS.Put(ctx, k, t)
}
