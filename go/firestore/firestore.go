package firestore

/*
   This package provides convenience functions for interacting with Cloud Firestore.
*/

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/firestore"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	// List all apps here as constants.
	APP_TASK_SCHEDULER = "task-scheduler"

	// List all instances here as constants.
	INSTANCE_PROD = "prod"
	INSTANCE_TEST = "test"

	// Maximum number of docs in a single transaction.
	MAX_TRANSACTION_DOCS = 500
)

// Client is a Cloud Firestore client which enforces separation of app/instance
// data via separate collections and documents. All references to collections
// and documents are automatically prefixed with the app name as the top-level
// collection and instance name as the parent document.
type Client struct {
	*firestore.Client
	ParentDoc *firestore.DocumentRef
}

// NewClient returns a Cloud Firestore client which enforces separation of app/
// instance data via separate collections and documents. All references to
// collections and documents are automatically prefixed with the app name as the
// top-level collection and instance name as the parent document.
func NewClient(ctx context.Context, project, app, instance string, ts oauth2.TokenSource) (*Client, error) {
	if project == "" {
		return nil, errors.New("Project name is required.")
	}
	if app == "" {
		return nil, errors.New("App name is required.")
	}
	if instance == "" {
		return nil, errors.New("Instance name is required.")
	}
	client, err := firestore.NewClient(ctx, project, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	return &Client{
		Client:    client,
		ParentDoc: client.Collection(app).Doc(instance),
	}, nil
}

// See documentation for firestore.Client.
func (c *Client) Collection(path string) *firestore.CollectionRef {
	return c.ParentDoc.Collection(path)
}

// See documentation for firestore.Client.
func (c *Client) Collections(ctx context.Context) *firestore.CollectionIterator {
	return c.ParentDoc.Collections(ctx)
}

// See documentation for firestore.Client.
func (c *Client) Doc(path string) *firestore.DocumentRef {
	split := strings.Split(path, "/")
	if len(split) < 2 {
		return nil
	}
	return c.ParentDoc.Collection(split[0]).Doc(strings.Join(split[1:], "/"))
}

// IterDocs is a convenience function which executes the given query and calls
// the given callback function for each document.
func IterDocs(ctx context.Context, q firestore.Query, callback func(*firestore.DocumentSnapshot) error) error {
	it := q.Documents(ctx)
	defer it.Stop()
	for {
		doc, err := it.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return fmt.Errorf("Iteration failed: %s", err)
		}
		if err := callback(doc); err != nil {
			return err
		}
	}
	return nil
}
