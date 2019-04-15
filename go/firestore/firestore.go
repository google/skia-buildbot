package firestore

/*
   This package provides convenience functions for interacting with Cloud Firestore.
*/

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// Firestore has a timestamp resolution of one microsecond.
	TS_RESOLUTION = time.Microsecond

	// List all apps here as constants.
	APP_TASK_SCHEDULER = "task-scheduler"

	// Base wait time between attempts.
	BACKOFF_WAIT = 5 * time.Second

	// Project ID. At the moment only the skia-firestore project has
	// Firestore enabled.
	FIRESTORE_PROJECT = "skia-firestore"

	// List all instances here as constants.
	INSTANCE_PROD = "prod"
	INSTANCE_TEST = "test"

	// Maximum number of docs in a single transaction.
	MAX_TRANSACTION_DOCS = 500

	// IterDocs won't iterate for longer than this amount of time at once,
	// otherwise we risk server timeouts. Instead, it will stop and resume
	// iteration.
	MAX_ITER_TIME = 50 * time.Second

	opTypeRead  = "read"
	opTypeWrite = "write"

	opCountRows    = "rows"
	opCountQueries = "queries"
)

var (
	// We will retry requests which result in these errors.
	RETRY_ERRORS = []codes.Code{
		codes.Canceled,
		codes.DeadlineExceeded,
		codes.ResourceExhausted,
		codes.Aborted,
		codes.Internal,
		codes.Unavailable,
	}

	// errIterTooLong is a special error used in conjunction with
	// MAX_ITER_TIME and IterDocs to prevent running into server timeouts
	// when iterating a large number of entries.
	errIterTooLong = errors.New("iterated too long")

	opTypes  = []string{opTypeRead, opTypeWrite}
	opCounts = []string{opCountRows, opCountQueries}
)

// DocumentRefSlice is a slice of DocumentRefs, used for sorting.
type DocumentRefSlice []*firestore.DocumentRef

func (s DocumentRefSlice) Len() int           { return len(s) }
func (s DocumentRefSlice) Less(i, j int) bool { return s[i].Path < s[j].Path }
func (s DocumentRefSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Client is a Cloud Firestore client which enforces separation of app/instance
// data via separate collections and documents. All references to collections
// and documents are automatically prefixed with the app name as the top-level
// collection and instance name as the parent document.
type Client struct {
	*firestore.Client
	ParentDoc *firestore.DocumentRef

	activeOps      map[int64]string
	activeOpsCount metrics2.Int64Metric
	activeOpsId    int64 // Incremented every time we run a transaction.
	activeOpsMtx   sync.RWMutex

	// Counters is a nested map of opType (ie. "read" or "write"), opCount
	// ("rows" or "queries") and document/collection path to a counter which
	// records the number of operations.
	counters     map[string]map[string]map[string]metrics2.Counter
	countersMtx  sync.Mutex
	errorMetrics map[string]metrics2.Counter
	metricTags   map[string]string
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
	metricTags := map[string]string{
		"project":  project,
		"app":      app,
		"instance": instance,
	}
	errorMetrics := make(map[string]metrics2.Counter, len(RETRY_ERRORS))
	for _, code := range RETRY_ERRORS {
		errorMetrics[code.String()] = metrics2.GetCounter("firestore_retryable_errors", metricTags, map[string]string{
			"error": code.String(),
		})
	}
	counters := map[string]map[string]map[string]metrics2.Counter{}
	for _, opType := range opTypes {
		subMap := map[string]map[string]metrics2.Counter{}
		for _, opCount := range opCounts {
			subMap[opCount] = map[string]metrics2.Counter{}
		}
		counters[opType] = subMap
	}
	c := &Client{
		Client:         client,
		ParentDoc:      client.Collection(app).Doc(instance),
		activeOps:      map[int64]string{},
		activeOpsCount: metrics2.GetInt64Metric("firestore_ops_active", metricTags),
		counters:       counters,
		errorMetrics:   errorMetrics,
		metricTags:     metricTags,
	}
	go util.RepeatCtx(time.Minute, ctx, func() {
		c.activeOpsMtx.RLock()
		ids := make([]int64, 0, len(c.activeOps))
		for id := range c.activeOps {
			ids = append(ids, id)
		}
		sort.Sort(util.Int64Slice(ids))
		ops := strings.Builder{}
		for _, id := range ids {
			_, _ = fmt.Fprintf(&ops, "\n%d\t%s", id, c.activeOps[id])
		}
		c.activeOpsMtx.RUnlock()
		sklog.Debugf("Active operations (%d): %s", len(ids), ops.String())
	})
	return c, nil
}

// recordOp adds a transaction to the active transactions map. Returns
// a func which should be deferred until the transaction is finished.
func (c *Client) recordOp(opName, detail string) func() {
	t := metrics2.NewTimer("firestore_ops", c.metricTags, map[string]string{
		"op": opName,
	})
	c.activeOpsMtx.Lock()
	defer c.activeOpsMtx.Unlock()
	id := c.activeOpsId
	c.activeOps[id] = opName + ": " + detail
	c.activeOpsId++
	c.activeOpsCount.Update(int64(len(c.activeOps)))
	return func() {
		c.activeOpsMtx.Lock()
		defer c.activeOpsMtx.Unlock()
		delete(c.activeOps, id)
		c.activeOpsCount.Update(int64(len(c.activeOps)))
		t.Stop()
	}
}

// getCounterHelper returns a read/write row or query metric for the given path.
// The caller should hold c.countersMtx.
func (c *Client) getCounterHelper(op, count, path string) metrics2.Counter {
	counter, ok := c.counters[op][count][path]
	if !ok {
		counter = metrics2.GetCounter("firestore_ops_count", c.metricTags, map[string]string{
			"op":    op,
			"count": count,
			"path":  path,
		})
		c.counters[op][count][path] = counter
	}
	return counter
}

// getCounters returns a read/write row and query metric for the given path.
func (c *Client) getCounters(op, path string) (metrics2.Counter, metrics2.Counter) {
	path = strings.TrimPrefix(path, c.ParentDoc.Path)
	path = strings.TrimPrefix(path, "/")
	path = strings.Split(path, "/")[0]
	c.countersMtx.Lock()
	defer c.countersMtx.Unlock()
	return c.getCounterHelper(op, opCountQueries, path), c.getCounterHelper(op, opCountRows, path)
}

// CountReadRows increments the metric counter for the given path.
func (c *Client) CountReadRows(path string, count int) {
	_, rows := c.getCounters(opTypeRead, path)
	rows.Inc(int64(count))
}

// CountReadQuery increments the metric counter for the given path.
func (c *Client) CountReadQuery(path string) {
	queries, _ := c.getCounters(opTypeRead, path)
	queries.Inc(1)
}

// CountReadQueryAndRows increments the metric counters for the given path.
func (c *Client) CountReadQueryAndRows(path string, rowCount int) {
	queries, rows := c.getCounters(opTypeRead, path)
	queries.Inc(1)
	rows.Inc(int64(rowCount))
}

// CountWriteRows increments the metric counter for the given path.
func (c *Client) CountWriteRows(path string, count int) {
	_, rows := c.getCounters(opTypeWrite, path)
	rows.Inc(int64(count))
}

// CountWriteQuery increments the metric counter for the given path.
func (c *Client) CountWriteQuery(path string) {
	queries, _ := c.getCounters(opTypeWrite, path)
	queries.Inc(1)
}

// CountWriteQueryAndRows increments the metric counters for the given path.
func (c *Client) CountWriteQueryAndRows(path string, rowCount int) {
	queries, rows := c.getCounters(opTypeWrite, path)
	queries.Inc(1)
	rows.Inc(int64(rowCount))
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

// withTimeout runs the given function with the given timeout.
func withTimeout(timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return fn(ctx)
}

// withTimeoutAndRetries runs the given function with the given timeout and a
// maximum of the given number of attempts. The timeout is applied for each
// attempt.
func (c *Client) withTimeoutAndRetries(attempts int, timeout time.Duration, fn func(context.Context) error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = withTimeout(timeout, fn)
		if err == nil {
			return nil
		} else if st, ok := status.FromError(err); ok {
			// Retry if we encountered a whitelisted error code.
			code := st.Code()
			retry := false
			for _, retryCode := range RETRY_ERRORS {
				if code == retryCode {
					retry = true
					c.errorMetrics[code.String()].Inc(1)
					break
				}
			}
			if !retry {
				return err
			}
		} else if err != nil {
			return err
		}
		wait := BACKOFF_WAIT * time.Duration(2^i)
		sklog.Errorf("Encountered Firestore error; retrying in %s: %s;\n", wait, err)
		time.Sleep(wait)
	}
	// Note that we could collect the errors using multierror, but that
	// would break some behavior which relies on pointer equality
	// (eg. err == ErrConcurrentUpdate).
	return err
}

// Get retrieves the given document, using the given timeout and maximum number
// of attempts. Returns (nil, nil) if the document does not exist. Uses the
// given maximum number of attempts and the given per-attempt timeout.
func (c *Client) Get(ref *firestore.DocumentRef, attempts int, timeout time.Duration) (*firestore.DocumentSnapshot, error) {
	defer c.recordOp("Get", ref.Path)()
	var doc *firestore.DocumentSnapshot
	err := c.withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		c.CountReadQueryAndRows(ref.Path, 1)
		got, err := ref.Get(ctx)
		if err == nil {
			doc = got
		}
		return err
	})
	return doc, err
}

// iterDocsInner is a helper function used by IterDocs which facilitates testing.
func (c *Client) iterDocsInner(query firestore.Query, attempts int, timeout time.Duration, callback func(*firestore.DocumentSnapshot) error, ranTooLong func(time.Time) bool) (int, error) {
	numRestarts := 0
	var lastSeen *firestore.DocumentSnapshot
	for {
		started := time.Now()
		err := c.withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
			q := query
			if lastSeen != nil {
				q = q.StartAfter(lastSeen)
			}
			it := q.Documents(ctx)
			defer it.Stop()
			first := true
			for {
				doc, err := it.Next()
				if err == iterator.Done {
					break
				} else if err != nil {
					return err
				}
				// Query doesn't have a path associated with it, but we'd like to
				// record metrics. Use the path of the parent of the first found doc.
				if first {
					c.CountReadQueryAndRows(doc.Ref.Parent.Path, 1)
					first = false
				} else {
					c.CountReadRows(doc.Ref.Parent.Path, 1)
				}
				if err := callback(doc); err != nil {
					return err
				}
				lastSeen = doc
				if ranTooLong(started) {
					sklog.Debugf("Iterated for longer than %s; pausing to avoid timeouts.", MAX_ITER_TIME)
					return errIterTooLong
				}
			}
			return nil
		})
		if err == nil {
			return numRestarts, nil
		} else if err != errIterTooLong {
			return numRestarts, err
		}
		numRestarts++
		sklog.Debugf("Resuming iteration after %s", lastSeen.Ref.Path)
	}
}

// IterDocs is a convenience function which executes the given query and calls
// the given callback function for each document. Uses the given maximum number
// of attempts and the given per-attempt timeout. IterDocs automatically stops
// iterating after enough time has passed and re-issues the query, continuing
// where it left off. This is to avoid server-side timeouts resulting from
// iterating a large number of results. Note that this behavior may result in
// individual results coming from inconsistent snapshots.
func (c *Client) IterDocs(name, detail string, query firestore.Query, attempts int, timeout time.Duration, callback func(*firestore.DocumentSnapshot) error) error {
	defer c.recordOp(name, detail)()
	_, err := c.iterDocsInner(query, attempts, timeout, callback, func(started time.Time) bool {
		return time.Now().Sub(started) > MAX_ITER_TIME
	})
	return err
}

// IterDocsInParallel is a convenience function which executes the given queries
// in multiple goroutines and calls the given callback function for each
// document. Uses the maximum number of attempts and the given per-attempt
// timeout for each goroutine. Each callback includes the goroutine index.
// IterDocsInParallel automatically stops iterating after enough time has passed
// and re-issues the query, continuing where it left off. This is to avoid
// server-side timeouts resulting from iterating a large number of results. Note
// that this behavior may result in individual results coming from inconsistent
// snapshots.
func (c *Client) IterDocsInParallel(name, detail string, queries []firestore.Query, attempts int, timeout time.Duration, callback func(int, *firestore.DocumentSnapshot) error) error {
	var wg sync.WaitGroup
	errs := make([]error, len(queries))
	for idx, query := range queries {
		wg.Add(1)
		go func(idx int, query firestore.Query) {
			defer wg.Done()
			errs[idx] = c.IterDocs(name, fmt.Sprintf("%s (shard %d)", detail, idx), query, attempts, timeout, func(doc *firestore.DocumentSnapshot) error {
				return callback(idx, doc)
			})
		}(idx, query)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// RunTransaction runs the given function in a transaction. Uses the given
// maximum number of attempts and the given per-attempt timeout.
func (c *Client) RunTransaction(name, detail string, attempts int, timeout time.Duration, fn func(context.Context, *firestore.Transaction) error) error {
	defer c.recordOp(name, detail)()
	return c.withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		return c.Client.RunTransaction(ctx, fn)
	})
}

// See documentation for firestore.DocumentRef.Create(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func (c *Client) Create(ref *firestore.DocumentRef, data interface{}, attempts int, timeout time.Duration) (*firestore.WriteResult, error) {
	defer c.recordOp("Create", ref.Path)()
	var wr *firestore.WriteResult
	err := c.withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		c.CountWriteQueryAndRows(ref.Path, 1)
		var err error
		wr, err = ref.Create(ctx, data)
		return err
	})
	return wr, err
}

// See documentation for firestore.DocumentRef.Set(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func (c *Client) Set(ref *firestore.DocumentRef, data interface{}, attempts int, timeout time.Duration, opts ...firestore.SetOption) (*firestore.WriteResult, error) {
	defer c.recordOp("Set", ref.Path)()
	var wr *firestore.WriteResult
	err := c.withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		c.CountWriteQueryAndRows(ref.Path, 1)
		var err error
		wr, err = ref.Set(ctx, data, opts...)
		return err
	})
	return wr, err
}

// See documentation for firestore.DocumentRef.Update(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func (c *Client) Update(ref *firestore.DocumentRef, attempts int, timeout time.Duration, updates []firestore.Update, preconds ...firestore.Precondition) (*firestore.WriteResult, error) {
	defer c.recordOp("Update", ref.Path)()
	var wr *firestore.WriteResult
	err := c.withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		c.CountWriteQueryAndRows(ref.Path, 1)
		var err error
		wr, err = ref.Update(ctx, updates, preconds...)
		return err
	})
	return wr, err
}

// See documentation for firestore.DocumentRef.Delete(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func (c *Client) Delete(ref *firestore.DocumentRef, attempts int, timeout time.Duration, preconds ...firestore.Precondition) (*firestore.WriteResult, error) {
	defer c.recordOp("Delete", ref.Path)()
	var wr *firestore.WriteResult
	err := c.withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		c.CountWriteQueryAndRows(ref.Path, 1)
		var err error
		wr, err = ref.Delete(ctx, preconds...)
		return err
	})
	return wr, err
}

// RecurseDocs runs the given func for every descendent of the given document.
// This includes missing documents, ie. those which do not exist but have sub-
// documents. The func is run for leaf documents before their parents. This
// function does nothing to account for documents which may be added or modified
// while it is running.
func (c *Client) RecurseDocs(name string, ref *firestore.DocumentRef, attempts int, timeout time.Duration, fn func(*firestore.DocumentRef) error) error {
	defer c.recordOp(name, ref.Path)()
	return c.recurseDocs(ref, attempts, timeout, fn)
}

// recurseDocs is a recursive helper function used by RecurseDocs.
func (c *Client) recurseDocs(ref *firestore.DocumentRef, attempts int, timeout time.Duration, fn func(*firestore.DocumentRef) error) error {
	// TODO(borenet): Should we pause and resume like we do in IterDocs?
	colls := map[string]*firestore.CollectionRef{}
	if err := c.withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		c.CountReadQuery(ref.Path)
		it := ref.Collections(ctx)
		for {
			coll, err := it.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				return err
			}
			c.CountReadRows(ref.Path, 1)
			colls[coll.Path] = coll
		}
		return nil
	}); err != nil {
		return err
	}
	for _, coll := range colls {
		if err := c.withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
			c.CountReadQuery(ref.Path)
			it := coll.DocumentRefs(ctx)
			for {
				doc, err := it.Next()
				if err == iterator.Done {
					break
				} else if err != nil {
					return err
				}
				c.CountReadRows(ref.Path, 1)
				if err := c.recurseDocs(doc, attempts, timeout, fn); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return fn(ref)
}

// GetAllDescendantDocuments returns a slice of DocumentRefs for every
// descendent of the given Document. This includes missing documents, ie. those
// which do not exist but have sub-documents. This function does nothing to
// account for documents which may be added or modified while it is running.
func (c *Client) GetAllDescendantDocuments(ref *firestore.DocumentRef, attempts int, timeout time.Duration) ([]*firestore.DocumentRef, error) {
	rv := []*firestore.DocumentRef{}
	if err := c.RecurseDocs("GetAllDescendantDocuments", ref, attempts, timeout, func(doc *firestore.DocumentRef) error {
		// Don't include the passed-in doc.
		if doc.Path != ref.Path {
			rv = append(rv, doc)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Sort(DocumentRefSlice(rv))
	return rv, nil
}

// RecursiveDelete deletes the given document and all of its descendant
// documents and collections. The given maximum number of attempts and the given
// per-attempt timeout apply for each delete operation, as opposed to the whole
// series of operations. This function does nothing to account for documents
// which may be added or modified while it is running.
func (c *Client) RecursiveDelete(ref *firestore.DocumentRef, attempts int, timeout time.Duration) error {
	return c.RecurseDocs("RecursiveDelete", ref, attempts, timeout, func(ref *firestore.DocumentRef) error {
		c.CountWriteQueryAndRows(ref.Path, 1)
		_, err := c.Delete(ref, attempts, timeout)
		return err
	})
}
