/*
	Used by the Android Compile Server to interact with the cloud datastore.
*/

package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
)

// TODO(rmistry): All methods below needed?

const (
	COMPILE_TASK ds.Kind = "CompileTask"
)

func DatastoreInit(project string, ns string) error {
	return ds.Init(project, ns)
}

func GetPendingTasks() *datastore.Iterator {
	q := ds.NewQuery(COMPILE_TASK).EventualConsistency().Filter("Done =", false)
	return ds.DS.Run(context.TODO(), q)
}

func GetNewDSKey() *datastore.Key {
	return ds.NewKey(COMPILE_TASK)
}

func GetDSTask(taskID int64) (*datastore.Key, *CompileTask, error) {
	key := ds.NewKey(COMPILE_TASK)
	key.ID = taskID

	task := &CompileTask{}
	if err := ds.DS.Get(context.TODO(), key, task); err != nil {
		return nil, nil, fmt.Errorf("Error retrieving task from Datastore: %v", err)
	}
	return key, task, nil
}

func PutDSTask(k *datastore.Key, t *CompileTask) (*datastore.Key, error) {
	return ds.DS.Put(context.Background(), k, t)
}

func UpdateDSTask(k *datastore.Key, t *CompileTask) (*datastore.Key, error) {
	return ds.DS.Put(context.Background(), k, t)
}
