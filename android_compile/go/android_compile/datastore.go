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

const (
	REQUEST ds.Kind = "Request"
)

func DatastoreInit(project string, ns string) error {
	return ds.Init(project, ns)
}

func GetRunningDSTasks() *datastore.Iterator {
	q := ds.NewQuery(REQUEST).EventualConsistency().Filter("Done =", false)
	return ds.DS.Run(context.TODO(), q)
}

func GetAllDSTasks(filterUser string) *datastore.Iterator {
	q := ds.NewQuery(REQUEST).EventualConsistency()
	if filterUser != "" {
		q = q.Filter("Requester =", filterUser)
	}
	return ds.DS.Run(context.TODO(), q)
}

func GetNewDSKey() *datastore.Key {
	return ds.NewKey(REQUEST)
}

func GetDSTask(taskID int64) (*datastore.Key, *Request, error) {
	key := ds.NewKey(REQUEST)
	key.ID = taskID

	request := &Request{}
	if err := ds.DS.Get(context.TODO(), key, request); err != nil {
		return nil, nil, fmt.Errorf("Error retrieving task from Datastore: %v", err)
	}
	return key, request, nil
}

func PutDSTask(k *datastore.Key, t *Request) (*datastore.Key, error) {
	return ds.DS.Put(context.Background(), k, t)
}

func UpdateDSTask(k *datastore.Key, t *Request) (*datastore.Key, error) {
	return ds.DS.Put(context.Background(), k, t)
}
