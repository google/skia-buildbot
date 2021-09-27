package firestore

import (
	"context"
	"fmt"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"golang.org/x/oauth2"
)

const (
	// At the moment, the only project with Firestore enabled is skia-firestore.
	FIRESTORE_PROJECT = firestore.FIRESTORE_PROJECT

	// Timeouts for various requests.
	GET_SINGLE_TIMEOUT = 10 * time.Second
	GET_MULTI_TIMEOUT  = 60 * time.Second
	PUT_SINGLE_TIMEOUT = 10 * time.Second
	PUT_MULTI_TIMEOUT  = 30 * time.Second

	// We'll perform this many attempts for a given request.
	DEFAULT_ATTEMPTS = 3

	// Load entries in at most 100 goroutines.
	MAX_LOAD_GOROUTINES = 100

	// Maximum documents in a transaction.
	MAX_TRANSACTION_DOCS = firestore.MAX_TRANSACTION_DOCS

	// Firestore key for a Task or Job's Created field.
	KEY_CREATED = "Created"

	// Firestore key for a Task or Job's DbModified field.
	KEY_DB_MODIFIED = "DbModified"

	// Firestore key for a Task or Job's Repo field.
	KEY_REPO = "Repo"

	// Estimated entry density, used for setting the initial size of results
	// collection data structures.
	EST_ENTRY_DENSITY = float64(1000) / float64(time.Hour)

	// Minimum and maximum estimated result set size.
	EST_RESULT_SIZE_MIN = 16
	EST_RESULT_SIZE_MAX = 8192
)

// firestoreDB is a db.DB which uses Cloud Firestore for storage.
type firestoreDB struct {
	client *firestore.Client
}

// NewDB returns a db.DB which uses Cloud Firestore for storage, using the given params.
func NewDBWithParams(ctx context.Context, project, instance string, ts oauth2.TokenSource) (db.DBCloser, error) {
	client, err := firestore.NewClient(ctx, project, firestore.APP_TASK_SCHEDULER, instance, ts)
	if err != nil {
		return nil, err
	}
	return NewDB(ctx, client)
}

// NewDB returns a db.DB which uses the given firestore.Client for storage.
func NewDB(ctx context.Context, client *firestore.Client) (db.DBCloser, error) {
	return &firestoreDB{
		client: client,
	}, nil
}

// See documentation for db.DBCloser interface.
func (d *firestoreDB) Close() error {
	return d.client.Close()
}

// Estimate the size of the result set for the given time chunk based on
// experimentally-determined density of results.
func estResultSize(chunkSize time.Duration) int {
	rv := util.RoundUpToPowerOf2(int32(float64(chunkSize) * EST_ENTRY_DENSITY))
	// Use a minimum of 16 and a maximum of 8192.
	if rv < EST_RESULT_SIZE_MIN {
		rv = EST_RESULT_SIZE_MIN
	} else if rv > EST_RESULT_SIZE_MAX {
		rv = EST_RESULT_SIZE_MAX
	}
	return int(rv)
}

// dateRangeHelper is a helper function for loading entries by date range. It
// breaks the date range into a number of chunks and loads them in separate
// goroutines. In order to get the correct behavior without needing locks, the
// caller must provide two functions:
//
// 1. An initializer for the results. dateRangeHelper calls this function with
//    the number of goroutines it will use to load the results.
// 2. A function to call for each element. The index of the calling goroutine
//    will be provided as the first argument to this function so that the caller
//    can distinguish results from different goroutines, thus avoiding the need
//    for a mutex.
func (d *firestoreDB) dateRangeHelper(ctx context.Context, name string, baseQuery fs.Query, start, end time.Time, init func(int), elem func(int, *fs.DocumentSnapshot) error) error {
	// Adjust start and end times for Firestore resolution.
	start = firestore.FixTimestamp(start)
	end = firestore.FixTimestamp(end.Add(firestore.TS_RESOLUTION - time.Nanosecond))

	// Load tasks in at most N goroutines.
	dateRange := end.Sub(start)
	chunkSize := (dateRange/time.Duration(MAX_LOAD_GOROUTINES) + firestore.TS_RESOLUTION).Truncate(firestore.TS_RESOLUTION)

	// Create the slice of queries to run in parallel.
	queries := []fs.Query{}
	if err := util.IterTimeChunks(start, end, chunkSize, func(start, end time.Time) error {
		q := baseQuery.Where(KEY_CREATED, ">=", start).Where(KEY_CREATED, "<", end).OrderBy(KEY_CREATED, fs.Asc)
		queries = append(queries, q)
		return nil
	}); err != nil {
		// We don't return an error from the above, so we shouldn't hit
		// this case.
		return err
	}

	// Run the given init function.
	init(len(queries))

	// Run the queries.
	return d.client.IterDocsInParallel(ctx, name, fmt.Sprintf("%s - %s", start, end), queries, DEFAULT_ATTEMPTS, GET_MULTI_TIMEOUT, elem)
}
