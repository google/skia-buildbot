package firestore

import (
	"context"
	"time"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/task_scheduler/go/db"
	"golang.org/x/oauth2"
)

const (
	GET_SINGLE_TIMEOUT = 5 * time.Second
	GET_MULTI_TIMEOUT  = 60 * time.Second
	PUT_SINGLE_TIMEOUT = 10 * time.Second
	PUT_MULTI_TIMEOUT  = 30 * time.Second

	DEFAULT_ATTEMPTS = 3
)

// Fix the given timestamp. Firestore only supports microsecond precision, and
// we always want to store UTC.
func fixTimestamp(t time.Time) time.Time {
	return t.UTC().Truncate(time.Microsecond)
}

// firestoreDB is a db.DB which uses Cloud Firestore for storage.
type firestoreDB struct {
	client    *firestore.Client
	parentDoc string

	// ModifiedTasks and ModifiedJobs are embedded in order to implement
	// db.TaskReader and db.JobReader.
	db.ModifiedTasks
	db.ModifiedJobs
}

// NewDB returns a db.DB which uses Cloud Firestore for storage. The parentDoc
// parameter is optional and indicates the path of a parent document to which
// all collections within the DB will belong. If it is not supplied, then the
// collections will be at the top level.
func NewDB(ctx context.Context, project, instance string, ts oauth2.TokenSource) (db.DBCloser, error) {
	client, err := firestore.NewClient(ctx, project, firestore.APP_TASK_SCHEDULER, instance, ts)
	if err != nil {
		return nil, err
	}
	return &firestoreDB{
		client: client,
	}, nil
}

// See documentation for db.DBCloser interface.
func (d *firestoreDB) Close() error {
	return d.client.Close()
}
