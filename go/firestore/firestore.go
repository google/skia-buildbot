package firestore

/*
   This package provides convenience functions for interacting with Cloud Firestore.
*/

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/cenkalti/backoff"
	"github.com/google/uuid"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils/unittest"
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

	// Length of IDs returned by AlphaNumID.
	ID_LEN = 20

	opTypeRead  = "read"
	opTypeWrite = "write"

	opCountRows    = "rows"
	opCountQueries = "queries"

	alphaNum = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

var (
	// We will retry requests which result in these errors.
	retryErrors = []codes.Code{
		codes.DeadlineExceeded,
		codes.ResourceExhausted,
		codes.Aborted,
		codes.Internal,
		codes.Unavailable,
		codes.Canceled,
	}

	// errIterTooLong is a special error used in conjunction with
	// MAX_ITER_TIME and IterDocs to prevent running into server timeouts
	// when iterating a large number of entries.
	errIterTooLong = errors.New("iterated too long")
)

// FixTimestamp adjusts the given timestamp for storage in Firestore. Firestore
// only supports microsecond precision, and we always want to store UTC.
func FixTimestamp(t time.Time) time.Time {
	return t.UTC().Truncate(TS_RESOLUTION)
}

// AlphaNumID generates a fixed-length alphanumeric document ID using
// crypto/rand. Panics if crypto/rand fails to generate random bytes, eg. it
// cannot read from /dev/urandom.
//
// Motivation: the Go client library for Firestore generates URL-safe base64-
// encoded IDs, whic may not be desirable for all use cases (eg. passing an ID
// which may contain a '-' on the command line would require some escaping).
func AlphaNumID() string {
	bytes := make([]byte, ID_LEN)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read error: %v", err))
	}
	for idx := range bytes {
		bytes[idx] = alphaNum[bytes[idx]%byte(len(alphaNum))]
	}
	return string(bytes)
}

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

	// counters is a cache of Metrics2.Counters to the number of operations and queries for reads
	// and writes. We need to cache it here because multiple calls to the same GetCounter() would
	// give different pointers to the same underlying int, which is undesirable.
	counters     map[counterKey]metrics2.Counter
	countersMtx  sync.Mutex
	errorMetrics map[string]metrics2.Counter
	metricTags   map[string]string
}

// counterKey is the key to the cache map of metrics counters.
type counterKey struct {
	Operation string
	Count     string
	Path      string
	FileName  string
	FuncName  string
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
	errorMetrics := make(map[string]metrics2.Counter, len(retryErrors))
	for _, code := range retryErrors {
		errorMetrics[code.String()] = metrics2.GetCounter("firestore_retryable_errors", metricTags, map[string]string{
			"error": code.String(),
		})
	}

	c := &Client{
		Client:         client,
		ParentDoc:      client.Collection(app).Doc(instance),
		activeOps:      map[int64]string{},
		activeOpsCount: metrics2.GetInt64Metric("firestore_ops_active", metricTags),
		counters:       map[counterKey]metrics2.Counter{},
		errorMetrics:   errorMetrics,
		metricTags:     metricTags,
	}
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
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

// NewClientForTesting returns a Client and ensures that it will connect to the
// Firestore emulator. The Client's instance name will be randomized to ensure
// concurrent tests don't interfere with each other. It also returns a
// CleanupFunc that closes the Client.
func NewClientForTesting(ctx context.Context, t sktest.TestingT) (*Client, util.CleanupFunc) {
	unittest.RequiresFirestoreEmulator(t)

	project := "test-project"
	app := "NewClientForTesting"
	instance := fmt.Sprintf("test-%s", uuid.New())
	ctx, cancel := context.WithCancel(ctx)
	c, err := NewClient(ctx, project, app, instance, nil)
	if err != nil {
		cancel()
		t.Fatalf("Error creating test firestore.Client: %s", err)
		return nil, nil
	}
	return c, func() {
		if err := c.Close(); err != nil {
			t.Fatalf("Error closing test firestore.Client: %s", err)
		}
		cancel()
	}
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
func (c *Client) getCounterHelper(op, count, path, file, fnName string) metrics2.Counter {
	key := counterKey{
		Operation: op,
		Count:     count,
		Path:      path,
		FileName:  file,
		FuncName:  fnName,
	}
	counter, ok := c.counters[key]
	if !ok {
		counter = metrics2.GetCounter("firestore_ops_count", c.metricTags, map[string]string{
			"op":    op,
			"count": count,
			"path":  path,
			"file":  file,
			"func":  fnName,
		})
		c.counters[key] = counter
	}
	return counter
}

// getCounters returns a read/write row and query metric for the given path.
func (c *Client) getCounters(op, path, file, fnName string) (metrics2.Counter, metrics2.Counter) {
	path = strings.TrimPrefix(path, c.ParentDoc.Path)
	path = strings.TrimPrefix(path, "/")
	path = strings.Split(path, "/")[0]
	c.countersMtx.Lock()
	defer c.countersMtx.Unlock()
	return c.getCounterHelper(op, opCountQueries, path, file, fnName), c.getCounterHelper(op, opCountRows, path, file, fnName)
}

// countReadRows increments the "rows read" metric counter for a path by the given amount.
func (c *Client) countReadRows(path, file, fnName string, count int) {
	_, rows := c.getCounters(opTypeRead, path, file, fnName)
	rows.Inc(int64(count))
}

// countReadQuery increments the "read queries" metric counter for a path by one.
func (c *Client) countReadQuery(path, file, fnName string) {
	queries, _ := c.getCounters(opTypeRead, path, file, fnName)
	queries.Inc(1)
}

// CountReadQueryAndRows increments the metric counters for a path. The "read queries" metric will
// be incremented by one and the "rows read" by the amount given. This should be done when a client
// does some amount of reading from firestore w/o using the helpers in this package (e.g. GetAll).
func (c *Client) CountReadQueryAndRows(path string, rowCount int) {
	file, fnName := callerHelper()
	c.countReadQueryAndRows(path, file, fnName, rowCount)
}

// countReadQueryAndRows is the private version of CountReadQueryAndRows; it takes a file, fnName
// that is passed down from the entry-level API call.
func (c *Client) countReadQueryAndRows(path, file, fnName string, rowCount int) {
	queries, rows := c.getCounters(opTypeRead, path, file, fnName)
	queries.Inc(1)
	rows.Inc(int64(rowCount))
}

// CountWriteQueryAndRows increments the metric counters for a path. The "write queries" metric will
// be incremented by one and the "rows written" by the amount given. This should be done when a
// client does some amount of writing to firestore w/o using the helpers in this package (or uses
// BatchWrite).
func (c *Client) CountWriteQueryAndRows(path string, rowCount int) {
	file, fnName := callerHelper()
	c.countWriteQueryAndRows(path, file, fnName, rowCount)
}

// countWriteQueryAndRows is the private version of CountWriteQueryAndRows; it takes a file, fnName
// that is passed down from the entry-level API call.
func (c *Client) countWriteQueryAndRows(path, file, fnName string, rowCount int) {
	queries, rows := c.getCounters(opTypeWrite, path, file, fnName)
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
func withTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(ctx)
}

// withTimeoutAndRetries runs the given function with the given timeout and a
// maximum of the given number of attempts. The timeout is applied for each
// attempt.
func (c *Client) withTimeoutAndRetries(ctx context.Context, attempts int, timeout time.Duration, fn func(context.Context) error) error {
	var err error
	for i := 0; i < attempts; i++ {
		// Do not retry on e.g. a cancelled context.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err = withTimeout(ctx, timeout, fn)
		unwrapped := skerr.Unwrap(err)
		if err == nil {
			return nil
		} else if ctx.Err() != nil {
			// Do not retry if the passed-in context is expired.
			return ctx.Err()
		} else if st, ok := status.FromError(unwrapped); ok {
			// Retry if we encountered a retryable error code.
			code := st.Code()
			retry := false
			for _, retryCode := range retryErrors {
				if code == retryCode {
					retry = true
					c.errorMetrics[code.String()].Inc(1)
					break
				}
			}
			if !retry {
				return err
			}
		} else if unwrapped != context.DeadlineExceeded {
			// withTimeout uses context.WithDeadline to implement
			// timeouts, therefore we may have received the
			// DeadlineExceeded error from that context, while the
			// passed-in parent context is still valid. We want to
			// retry in that case, otherwise we stop here.
			return err
		}
		wait := BACKOFF_WAIT * time.Duration(2^i)
		sklog.Errorf("Encountered Firestore error; retrying in %s: %s", wait, err)
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
func (c *Client) Get(ctx context.Context, ref *firestore.DocumentRef, attempts int, timeout time.Duration) (*firestore.DocumentSnapshot, error) {
	file, fnName := callerHelper()
	defer c.recordOp("Get", ref.Path)()
	var doc *firestore.DocumentSnapshot
	err := c.withTimeoutAndRetries(ctx, attempts, timeout, func(ctx context.Context) error {
		c.countReadQueryAndRows(ref.Path, file, fnName, 1)
		got, err := ref.Get(ctx)
		if err == nil {
			doc = got
		}
		return err
	})
	return doc, err
}

// iterDocsInner is a helper function used by IterDocs which facilitates testing.
func (c *Client) iterDocsInner(ctx context.Context, query firestore.Query, attempts int, timeout time.Duration, file, fnName string, callback func(*firestore.DocumentSnapshot) error, ranTooLong func(time.Time) bool) (int, error) {
	numRestarts := 0
	var lastSeen *firestore.DocumentSnapshot
	for {
		started := time.Now()
		err := c.withTimeoutAndRetries(ctx, attempts, timeout, func(ctx context.Context) error {
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
					c.countReadQueryAndRows(doc.Ref.Parent.Path, file, fnName, 1)
					first = false
				} else {
					c.countReadRows(doc.Ref.Parent.Path, file, fnName, 1)
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
func (c *Client) IterDocs(ctx context.Context, name, detail string, query firestore.Query, attempts int, timeout time.Duration, callback func(*firestore.DocumentSnapshot) error) error {
	file, fnName := callerHelper()
	return c.iterDocs(ctx, name, detail, file, fnName, query, attempts, timeout, callback)
}

// iterDocs is the private version of IterDocs; it takes a file, fnName that is passed down from
// the entry-level API call.
func (c *Client) iterDocs(ctx context.Context, name, detail, file, fnName string, query firestore.Query, attempts int, timeout time.Duration, callback func(*firestore.DocumentSnapshot) error) error {
	defer c.recordOp(name, detail)()
	_, err := c.iterDocsInner(ctx, query, attempts, timeout, file, fnName, callback, func(started time.Time) bool {
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
func (c *Client) IterDocsInParallel(ctx context.Context, name, detail string, queries []firestore.Query, attempts int, timeout time.Duration, callback func(int, *firestore.DocumentSnapshot) error) error {
	file, fnName := callerHelper()
	var wg sync.WaitGroup
	errs := make([]error, len(queries))
	for idx, query := range queries {
		wg.Add(1)
		go func(idx int, query firestore.Query) {
			defer wg.Done()
			errs[idx] = c.iterDocs(ctx, name, fmt.Sprintf("%s (shard %d)", detail, idx), file, fnName, query, attempts, timeout, func(doc *firestore.DocumentSnapshot) error {
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
func (c *Client) RunTransaction(ctx context.Context, name, detail string, attempts int, timeout time.Duration, fn func(context.Context, *firestore.Transaction) error) error {
	defer c.recordOp(name, detail)()
	return c.withTimeoutAndRetries(ctx, attempts, timeout, func(ctx context.Context) error {
		return c.Client.RunTransaction(ctx, fn)
	})
}

// See documentation for firestore.DocumentRef.Create(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func (c *Client) Create(ctx context.Context, ref *firestore.DocumentRef, data interface{}, attempts int, timeout time.Duration) (*firestore.WriteResult, error) {
	file, fnName := callerHelper()
	defer c.recordOp("Create", ref.Path)()
	var wr *firestore.WriteResult
	err := c.withTimeoutAndRetries(ctx, attempts, timeout, func(ctx context.Context) error {
		c.countWriteQueryAndRows(ref.Path, file, fnName, 1)
		var err error
		wr, err = ref.Create(ctx, data)
		return err
	})
	return wr, err
}

// See documentation for firestore.DocumentRef.Set(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func (c *Client) Set(ctx context.Context, ref *firestore.DocumentRef, data interface{}, attempts int, timeout time.Duration, opts ...firestore.SetOption) (*firestore.WriteResult, error) {
	file, fnName := callerHelper()
	defer c.recordOp("Set", ref.Path)()
	var wr *firestore.WriteResult
	err := c.withTimeoutAndRetries(ctx, attempts, timeout, func(ctx context.Context) error {
		c.countWriteQueryAndRows(ref.Path, file, fnName, 1)
		var err error
		wr, err = ref.Set(ctx, data, opts...)
		return err
	})
	return wr, err
}

// See documentation for firestore.DocumentRef.Update(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func (c *Client) Update(ctx context.Context, ref *firestore.DocumentRef, attempts int, timeout time.Duration, updates []firestore.Update, preconds ...firestore.Precondition) (*firestore.WriteResult, error) {
	file, fnName := callerHelper()
	defer c.recordOp("Update", ref.Path)()
	var wr *firestore.WriteResult
	err := c.withTimeoutAndRetries(ctx, attempts, timeout, func(ctx context.Context) error {
		c.countWriteQueryAndRows(ref.Path, file, fnName, 1)
		var err error
		wr, err = ref.Update(ctx, updates, preconds...)
		return err
	})
	return wr, err
}

// See documentation for firestore.DocumentRef.Delete(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func (c *Client) Delete(ctx context.Context, ref *firestore.DocumentRef, attempts int, timeout time.Duration, preconds ...firestore.Precondition) (*firestore.WriteResult, error) {
	file, fnName := callerHelper()
	return c.delete(ctx, ref, attempts, timeout, file, fnName, preconds...)
}

// delete is the private version of Delete; it takes a file, fnName that is passed down from
// the entry-level API call.
func (c *Client) delete(ctx context.Context, ref *firestore.DocumentRef, attempts int, timeout time.Duration, file, fnName string, preconds ...firestore.Precondition) (*firestore.WriteResult, error) {
	defer c.recordOp("Delete", ref.Path)()
	var wr *firestore.WriteResult
	err := c.withTimeoutAndRetries(ctx, attempts, timeout, func(ctx context.Context) error {
		c.countWriteQueryAndRows(ref.Path, file, fnName, 1)
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
func (c *Client) RecurseDocs(ctx context.Context, name string, ref *firestore.DocumentRef, attempts int, timeout time.Duration, fn func(*firestore.DocumentRef) error) error {
	file, fnName := callerHelper()
	defer c.recordOp(name, ref.Path)()
	return c.recurseDocs(ctx, ref, attempts, timeout, file, fnName, fn)
}

// recurseDocs is a recursive helper function used by RecurseDocs.
func (c *Client) recurseDocs(ctx context.Context, ref *firestore.DocumentRef, attempts int, timeout time.Duration, file, fnName string, fn func(*firestore.DocumentRef) error) error {
	// The Firestore emulator does not correctly handle subcollection queries.
	EnsureNotEmulator()
	// TODO(borenet): Should we pause and resume like we do in IterDocs?
	colls := map[string]*firestore.CollectionRef{}
	if err := c.withTimeoutAndRetries(ctx, attempts, timeout, func(ctx context.Context) error {
		c.countReadQuery(ref.Path, file, fnName)
		it := ref.Collections(ctx)
		for {
			coll, err := it.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				return err
			}
			c.countReadRows(ref.Path, file, fnName, 1)
			colls[coll.Path] = coll
		}
		return nil
	}); err != nil {
		return err
	}
	for _, coll := range colls {
		if err := c.withTimeoutAndRetries(ctx, attempts, timeout, func(ctx context.Context) error {
			c.countReadQuery(ref.Path, file, fnName)
			it := coll.DocumentRefs(ctx)
			for {
				doc, err := it.Next()
				if err == iterator.Done {
					break
				} else if err != nil {
					return err
				}
				c.countReadRows(ref.Path, file, fnName, 1)
				if err := c.recurseDocs(ctx, doc, attempts, timeout, file, fnName, fn); err != nil {
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
// descendant of the given Document. This includes missing documents, ie. those
// which do not exist but have sub-documents. This function does nothing to
// account for documents which may be added or modified while it is running.
func (c *Client) GetAllDescendantDocuments(ctx context.Context, ref *firestore.DocumentRef, attempts int, timeout time.Duration) ([]*firestore.DocumentRef, error) {
	file, fnName := callerHelper()
	defer c.recordOp("GetAllDescendantDocuments", ref.Path)()
	rv := []*firestore.DocumentRef{}
	if err := c.recurseDocs(ctx, ref, attempts, timeout, file, fnName, func(doc *firestore.DocumentRef) error {
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
func (c *Client) RecursiveDelete(ctx context.Context, ref *firestore.DocumentRef, attempts int, timeout time.Duration) error {
	file, fnName := callerHelper()
	defer c.recordOp("RecursiveDelete", ref.Path)()
	return c.recurseDocs(ctx, ref, attempts, timeout, file, fnName, func(ref *firestore.DocumentRef) error {
		_, err := c.delete(ctx, ref, attempts, timeout, file, fnName)
		return err
	})
}

// EnsureNotEmulator will panic if it detects the Firestore Emulator is configured.
// Trying to authenticate to the emulator results in errors like:
// "Failed to initialize Cloud Datastore: dialing: options.WithoutAuthentication
// is incompatible with any option that provides credentials"
func EnsureNotEmulator() {
	if emulators.GetEmulatorHostEnvVar(emulators.Firestore) != "" {
		panic("Firestore Emulator detected. Be sure to unset the following environment variable: " + emulators.GetEmulatorHostEnvVarName(emulators.Firestore))
	}
}

// QuerySnapshotChannel is a helper for firestore.QuerySnapshotIterator which
// passes each QuerySnapshot along the returned channel. QuerySnapshotChannel
// returns the channel immediately but spins up a goroutine which runs
// indefinitely or until an error occurs, in which case the channel is closed
// and an error is logged.
//
// A couple of notes:
//
// 1. QuerySnapshotIterator immediately produces a QuerySnapshot containing all
//    of the current results for the query, then blocks until those results
//    change. QuerySnapshot.Changes contains the changes since the last snapshot
//    (and will therefore be empty on the first snapshot), while its Documents
//    field is an iterator which will obtain all of the updated results.
// 2. If the consumer of the QuerySnapshotChannel is slower than the
//    QuerySnapshotIterator, the most recent snapshot will get stuck waiting to
//    be passed along the channel and thus may be out of date by the time the
//    consumer sees it. If your consumer is slow, consider adding a buffered
//    channel as an intermediate, or a goroutine to collect batches of snapshots
//    to be processed.
func QuerySnapshotChannel(ctx context.Context, q firestore.Query) <-chan *firestore.QuerySnapshot {
	ch := make(chan *firestore.QuerySnapshot)
	go func() {
		iter := q.Snapshots(ctx)
		for {
			qsnap, err := iter.Next()
			if err != nil {
				if ctx.Err() == context.Canceled {
					sklog.Warningf("Context canceled; closing QuerySnapshotChannel: %s", err)
				} else if st, ok := status.FromError(err); ok && st.Code() == codes.Canceled {
					sklog.Warningf("Context canceled; closing QuerySnapshotChannel: %s", err)
				} else {
					sklog.Errorf("QuerySnapshotIterator returned error; closing QuerySnapshotChannel: %s", err)
				}
				iter.Stop()
				close(ch)
				return
			}
			ch <- qsnap
		}
	}()
	return ch
}

// BatchWrite executes breaks a large amount of writes into batches of the given size and commits
// them, using retries that backoff exponentially up to the maxWriteTime. If a single batch fails,
// even with backoff, an error is returned, without attempting any further batches and without
// rolling back the previous successful batches (at present, rollback of previous batches is
// impossible to do correctly in the general case). Callers should also call CountWriteQueryAndRows
// to update counts of the affected collection(s).
func (c *Client) BatchWrite(ctx context.Context, total, batchSize int, maxWriteTime time.Duration, startBatch *firestore.WriteBatch, fn func(b *firestore.WriteBatch, i int) error) error {
	if batchSize > MAX_TRANSACTION_DOCS {
		return skerr.Fmt("Batch size %d exceeds the Firestore maximum of %d", batchSize, MAX_TRANSACTION_DOCS)
	}
	if startBatch == nil && total == 0 {
		// Unless the user has given us a startBatch, having 0 elements to write is a no-op.
		// Otherwise, we want to iterate over our 0 elements and then Commit() the startBatch
		// using the exponential backoff.
		return nil
	}
	b := startBatch
	if b == nil {
		b = c.Batch()
	}
	return util.ChunkIter(total, batchSize, func(start, stop int) error {
		for i := start; i < stop; i++ {
			if err := ctx.Err(); err != nil {
				// context has been cancelled or otherwise ended - stop work.
				return err
			}
			if err := fn(b, i); err != nil {
				return err
			}
		}

		exp := &backoff.ExponentialBackOff{
			InitialInterval:     time.Second,
			RandomizationFactor: 0.5,
			Multiplier:          2,
			MaxInterval:         maxWriteTime / 4,
			MaxElapsedTime:      maxWriteTime,
			Clock:               backoff.SystemClock,
		}

		o := func() error {
			// If the context is bad, retrying won't help, so just end.
			if err := ctx.Err(); err != nil {
				return backoff.Permanent(err)
			}
			_, err := b.Commit(ctx)
			return err
		}

		if err := backoff.Retry(o, exp); err != nil {
			// We really hope this doesn't happen, as it may leave the data in a partially
			// broken state.
			return skerr.Wrapf(err, "committing batch [%d, %d], even with exponential retry", start, stop)
		}
		// Go on to the next batch, if needed.
		if stop < total {
			b = c.Batch()
		}
		return nil
	})
}

// callerHelper returns the file and function name of the caller's caller. This should only be used
// in functions that are in the public API (so that the returned function/file are outside of this
// package).
func callerHelper() (string, string) {
	file, fnName := "", ""
	if pc, f, _, ok := runtime.Caller(2); ok {
		file = f
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			fnName = fn.Name()
		}
	}
	return file, fnName
}
