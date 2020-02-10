package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/ds"
	"google.golang.org/api/option"
)

// Status - A Tree status update.
type Status struct {
	Key      string    `json:"key" datastore:"key"` // Key is the web-safe serialized Datastore key for the Tree Status.
	Date     time.Time `json:"date" datastore:"date"`
	Message  string    `json:"message" datastore:"message"`
	Username string    `json:"username" datastore:"username"`
	// Only specified for backwards compatibility.
	FirstRev int `json:"first_rev" datastore:"first_rev"`
	LastRev  int `json:"last_rev" datastore:"last_rev"`
}

// Call this Status store?

func GetStatuses(num int) ([]*Status, error) {
	statuses := []*Status{}
	q := ds.NewQuery("Status").Order("-date").Limit(num)
	it := ds.DS.Run(context.TODO(), q)
	for {
		s := &Status{}
		// Combine with below
		_, err := it.Next(s)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of statuses: %s", err)
		}
		statuses = append(statuses, s)
	}
	return statuses, nil
}

func DatastoreInit(project string, ns string, local bool) error {
	ts, err := auth.NewDefaultTokenSource(local, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		return fmt.Errorf("Problem setting up default token source: %s", err)
	}
	return ds.InitWithOpt(project, ns, option.WithTokenSource(ts))
}
