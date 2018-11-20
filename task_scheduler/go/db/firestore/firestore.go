package firestore

import (
	"context"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"golang.org/x/oauth2"
)

const (
	// Timeouts for various requests.
	GET_SINGLE_TIMEOUT = 10 * time.Second
	GET_MULTI_TIMEOUT  = 60 * time.Second
	PUT_SINGLE_TIMEOUT = 10 * time.Second
	PUT_MULTI_TIMEOUT  = 30 * time.Second

	// We'll perform this many attempts for a given request.
	DEFAULT_ATTEMPTS = 3

	// Load entries in at most 100 goroutines.
	MAX_LOAD_GOROUTINES = 100

	// Datastore key for a Task or Job's Created field.
	KEY_CREATED = "Created"

	// Estimated entry density, used for setting the initial size of results
	// collection data structures.
	EST_ENTRY_DENSITY = float64(1000) / float64(time.Hour)

	// Minimum and maximum estimated result set size.
	EST_RESULT_SIZE_MIN = 16
	EST_RESULT_SIZE_MAX = 8192
)

// Fix the given timestamp. Firestore only supports microsecond precision, and
// we always want to store UTC.
func fixTimestamp(t time.Time) time.Time {
	return t.UTC().Truncate(firestore.TS_RESOLUTION)
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
func (d *firestoreDB) dateRangeHelper(coll *fs.CollectionRef, start, end time.Time, init func(int), elem func(int, *fs.DocumentSnapshot) error) error {
	// Adjust start and end times for Firestore resolution.
	start = fixTimestamp(start)
	end = fixTimestamp(end.Add(firestore.TS_RESOLUTION - time.Nanosecond))

	// Load tasks in at most N goroutines.
	dateRange := end.Sub(start)
	chunkSize := (dateRange/time.Duration(MAX_LOAD_GOROUTINES) + firestore.TS_RESOLUTION).Truncate(firestore.TS_RESOLUTION)

	// Create the slice of queries to run in parallel.
	queries := []fs.Query{}
	if err := util.IterTimeChunks(start, end, chunkSize, func(start, end time.Time) error {
		q := coll.Where(KEY_CREATED, ">=", start).Where(KEY_CREATED, "<", end).OrderBy(KEY_CREATED, fs.Asc)
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
	return firestore.IterDocsInParallel(queries, DEFAULT_ATTEMPTS, GET_MULTI_TIMEOUT, elem)
}
