/*
	Used by the Leasing Server to interact with the cloud datastore.
*/

package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/perf/go/ds"
)

const (
	TASK ds.Kind = "Task"
)

func DatastoreInit(project string, ns string) error {
	return ds.Init(project, ns)
}

func GetRunningDSTasks() *datastore.Iterator {
	q := ds.NewQuery(TASK).EventualConsistency().Filter("Done =", false)
	return ds.DS.Run(context.TODO(), q)
}

func GetAllDSTasks(filterUser string) *datastore.Iterator {
	q := ds.NewQuery(TASK).EventualConsistency()
	if filterUser != "" {
		q = q.Filter("Requester =", filterUser)
	}
	return ds.DS.Run(context.TODO(), q)
}

func GetNewDSKey() *datastore.Key {
	return ds.NewKey(TASK)
}

func GetDSTask(taskID int64) (*datastore.Key, *Task, error) {
	key := ds.NewKey(TASK)
	key.ID = taskID

	task := &Task{}
	if err := ds.DS.Get(context.TODO(), key, task); err != nil {
		return nil, nil, fmt.Errorf("Error retrieving task from Datastore: %v", err)
	}
	return key, task, nil
}

func PutDSTask(k *datastore.Key, t *Task) (*datastore.Key, error) {
	return ds.DS.Put(context.Background(), k, t)
}

func UpdateDSTask(k *datastore.Key, t *Task) (*datastore.Key, error) {
	return ds.DS.Put(context.Background(), k, t)
}
