package firestore

import (
	"context"
	"errors"
	"sync"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sklog"
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

	// Based on average load times, don't load more than this many entries
	// in one iteration, otherwise we risk server timeouts. Instead, stop
	// and resume iteration after this many entries.
	MAX_ITER_ENTRIES = 10000

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

var (
	// errTooManyEntries is a special error used in conjunction with
	// MAX_ITER_ENTRIES to prevent running into server timeouts when loading
	// a large number of entries.
	errTooManyEntries = errors.New("too many entries")
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
	rv := util.RoundUpToPowerOf2(int(float64(chunkSize) * EST_ENTRY_DENSITY))
	// Use a minimum of 16 and a maximum of 8192.
	if rv < EST_RESULT_SIZE_MIN {
		rv = EST_RESULT_SIZE_MIN
	} else if rv > EST_RESULT_SIZE_MAX {
		rv = EST_RESULT_SIZE_MAX
	}
	return rv
}

// dateRangeHelper is a helper function for loading entries by date range. It
// breaks the date range into a number of chunks and loads them in separate
// goroutines. In order to get the correct behavior without needing locks, the
// caller must provide three functions:
//
// 1. An initializer for the results. dateRangeHelper calls this function with
//    the number of goroutines it will use to load the results.
// 2. A function to call for each element. The index of the calling goroutine
//    will be provided as the first argument to this function so that the caller
//    can distinguish results from different goroutines, thus avoiding the need
//    for a mutex.
// 3. A pause function which is called in the case of iteration being stopped
//    due to many entries being loaded. This is to notify the caller that the
//    next set of entries may be duplicated, in the case of overlapping
//    creation timestamps.
// 4. A reset function which is only called in the case of an error which will
//    cause the whole query for a given goroutine to be retried. This function
//    should discard any previously-loaded results for the given goroutine
//    index.
//
// Additionally, it is recommended that the caller use a map to store results,
// because this function may stop and restart iteration if a large number of
// entries has been encountered. This works around server-side timeouts when a
// large number of entries are retrieved. Because the query is performed for
// creation timestamps within a date range, it is possible that some results may
// be duplicated in that case.
func (d *firestoreDB) dateRangeHelper(coll *fs.CollectionRef, start, end time.Time, init func(int), elem func(int, *fs.DocumentSnapshot) error, pause func(int), restart func(int)) error {
	// Adjust start and end times for Firestore resolution.
	start = fixTimestamp(start)
	end = fixTimestamp(end.Add(firestore.TS_RESOLUTION - time.Nanosecond))

	// Load tasks in at most N goroutines.
	dateRange := end.Sub(start)
	chunkSize := (dateRange/time.Duration(MAX_LOAD_GOROUTINES) + firestore.TS_RESOLUTION).Truncate(firestore.TS_RESOLUTION)

	// First, count the number of goroutines we'll use.
	numGoroutines := 0
	if err := util.IterTimeChunks(start, end, chunkSize, func(_, _ time.Time) error {
		numGoroutines++
		return nil
	}); err != nil {
		// We don't return an error from the above, so we shouldn't hit
		// this case.
		return err
	}
	init(numGoroutines)

	// Start all the goroutines.
	var wg sync.WaitGroup
	errs := make([]error, numGoroutines)
	idx := 0
	if err := util.IterTimeChunks(start, end, chunkSize, func(start, end time.Time) error {
		wg.Add(1)
		go func(idx int, start, end time.Time) {
			defer wg.Done()
			errs[idx] = d.dateRangeHelperInner(coll, start, end, func(doc *fs.DocumentSnapshot) error {
				return elem(idx, doc)
			}, func() {
				pause(idx)
			}, func() {
				restart(idx)
			})
		}(idx, start, end)
		idx++
		return nil
	}); err != nil {
		// We don't return an error from the above, so we shouldn't hit
		// this case.
		return err
	}

	// Finish.
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// dateRangeHelperInner is a function used by dateRangeHelper. It is a thin
// wrapper around firestore.IterDocs() which forcibly stops and restarts
// iteration if a large number of entries have been encountered. This works
// around server-side timeouts resulting from loading a large number of results.
// Because iteration is resumed at the last-seen creation timestamp, it is
// recommended that the caller use an ID-keyed map to store results, to avoid
// duplicate results in the case that creation times are duplicated or overlap.
func (d *firestoreDB) dateRangeHelperInner(coll *fs.CollectionRef, start, end time.Time, elem func(*fs.DocumentSnapshot) error, pause func(), restart func()) error {
	for {
		loaded := 0
		var lastDocCreated time.Time
		q := coll.Where(KEY_CREATED, ">=", start).Where(KEY_CREATED, "<", end).OrderBy(KEY_CREATED, fs.Asc)
		err := firestore.IterDocs(q, DEFAULT_ATTEMPTS, GET_MULTI_TIMEOUT, func(doc *fs.DocumentSnapshot) error {
			if err := elem(doc); err != nil {
				return err
			}
			ts, err := doc.DataAt(KEY_CREATED)
			if err != nil {
				return err
			}
			lastDocCreated = ts.(time.Time)
			loaded++
			if loaded%1000 == 0 && loaded > 0 {
				sklog.Infof("  Loaded %d tasks so far", loaded)
			}
			if loaded > MAX_ITER_ENTRIES {
				sklog.Infof("Loaded %d entries; stopping iteration.", loaded)
				return errTooManyEntries
			}
			return nil
		}, restart)
		if err == nil {
			return nil
		} else if err != errTooManyEntries {
			return err
		}
		pause()
		// TODO(borenet): Handle duplicates resulting from equal creation times.
		start = lastDocCreated
		sklog.Infof("Resuming iteration at %s", start)
	}
}
