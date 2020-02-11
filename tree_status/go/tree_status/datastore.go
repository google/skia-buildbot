package main

// TODO(rmistry): Need to move this whole package maybe to use firestore.

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/skerr"
)

const (
	STATUS_DS_KIND = "Status"
)

var (
	// DS is the Cloud Datastore client. Valid after DatastoreInit() has been called.
	DS *datastore.Client
)

// Status - A Tree status update.
type Status struct {
	// Key      string    `json:"key" datastore:"key"` // Key is the web-safe serialized Datastore key for the Tree Status.
	Date     time.Time `json:"date" datastore:"date"`
	Message  string    `json:"message" datastore:"message"`
	Username string    `json:"username" datastore:"username"`
	// Only specified for backwards compatibility.
	FirstRev int `json:"first_rev" datastore:"first_rev"`
	LastRev  int `json:"last_rev" datastore:"last_rev"`
}

// Call this Status store?
// Maybe do not need namespace at all???

func AddStatus(message, username string) error {
	s := &Status{
		Date:     time.Now(),
		Message:  message,
		Username: username,
	}
	fmt.Println("==================")
	fmt.Println("%+v", s)
	fmt.Println(s.Date)
	fmt.Println(s.Message)
	fmt.Println(s.Username)
	s.FirstRev = 1
	s.LastRev = 1

	key := &datastore.Key{
		Kind:      STATUS_DS_KIND,
		Namespace: "",
	}
	_, err := DS.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		var err error
		if _, err = tx.Put(key, s); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("Failed to add status: %s", err)
	}
	return nil
}

func GetStatuses(num int) ([]*Status, error) {
	statuses := []*Status{}
	q := datastore.NewQuery("Status").Namespace("").Order("-date").Limit(num)
	it := DS.Run(context.TODO(), q)
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

func DatastoreInit(ts oauth2.TokenSource, project string, ns string, local bool) error {
	var err error
	DS, err = datastore.NewClient(context.Background(), project, option.WithTokenSource(ts))
	if err != nil {
		return skerr.Wrapf(err, "Failed to initialize Cloud Datastore for tree status")
	}
	return err
}
