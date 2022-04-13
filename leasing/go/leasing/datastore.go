/*
	Used by the Leasing Server to interact with the cloud datastore.
*/

package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/leasing/go/types"
)

func DatastoreInit(ctx context.Context, project string, ns string) error {
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		return fmt.Errorf("Problem setting up default token source: %s", err)
	}
	return ds.InitWithOpt(project, ns, option.WithTokenSource(ts))
}

func GetRunningDSTasks() *datastore.Iterator {
	q := ds.NewQuery(ds.TASK).EventualConsistency().Filter("Done =", false)
	return ds.DS.Run(context.TODO(), q)
}

func GetAllDSTasks(filterUser string) *datastore.Iterator {
	q := ds.NewQuery(ds.TASK).EventualConsistency()
	if filterUser != "" {
		q = q.Filter("Requester =", filterUser)
	}
	return ds.DS.Run(context.TODO(), q)
}

func GetNewDSKey() *datastore.Key {
	return ds.NewKey(ds.TASK)
}

func GetDSTask(taskID int64) (*datastore.Key, *types.Task, error) {
	key := ds.NewKey(ds.TASK)
	key.ID = taskID

	task := &types.Task{}
	if err := ds.DS.Get(context.TODO(), key, task); err != nil {
		return nil, nil, fmt.Errorf("Error retrieving task from Datastore: %v", err)
	}
	return key, task, nil
}

func PutDSTask(k *datastore.Key, t *types.Task) (*datastore.Key, error) {
	return ds.DS.Put(context.Background(), k, t)
}

func UpdateDSTask(k *datastore.Key, t *types.Task) (*datastore.Key, error) {
	return ds.DS.Put(context.Background(), k, t)
}
