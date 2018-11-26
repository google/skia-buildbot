package firestore

/*
   This package provides convenience functions for interacting with Cloud Firestore.
*/

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
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

// withTimeout runs the given function with the given timeout.
func withTimeout(timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return fn(ctx)
}

// withTimeoutAndRetries runs the given function with the given timeout and a
// maximum of the given number of attempts. The timeout is applied for each
// attempt.
func withTimeoutAndRetries(attempts int, timeout time.Duration, fn func(context.Context) error) error {
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
func Get(ref *firestore.DocumentRef, attempts int, timeout time.Duration) (*firestore.DocumentSnapshot, error) {
	var doc *firestore.DocumentSnapshot
	err := withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		got, err := ref.Get(ctx)
		if err == nil {
			doc = got
		}
		return err
	})
	return doc, err
}

// iterDocsInner is a helper function used by IterDocs which facilitates testing.
func iterDocsInner(query firestore.Query, attempts int, timeout time.Duration, callback func(*firestore.DocumentSnapshot) error, ranTooLong func(time.Time) bool) (int, error) {
	numRestarts := 0
	var lastSeen *firestore.DocumentSnapshot
	for {
		started := time.Now()
		err := withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
			q := query
			if lastSeen != nil {
				q = q.StartAfter(lastSeen)
			}
			it := q.Documents(ctx)
			defer it.Stop()
			for {
				doc, err := it.Next()
				if err == iterator.Done {
					break
				} else if err != nil {
					return err
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
func IterDocs(query firestore.Query, attempts int, timeout time.Duration, callback func(*firestore.DocumentSnapshot) error) error {
	_, err := iterDocsInner(query, attempts, timeout, callback, func(started time.Time) bool {
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
func IterDocsInParallel(queries []firestore.Query, attempts int, timeout time.Duration, callback func(int, *firestore.DocumentSnapshot) error) error {
	var wg sync.WaitGroup
	errs := make([]error, len(queries))
	for idx, query := range queries {
		wg.Add(1)
		go func(idx int, query firestore.Query) {
			defer wg.Done()
			errs[idx] = IterDocs(query, attempts, timeout, func(doc *firestore.DocumentSnapshot) error {
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
func RunTransaction(client *Client, attempts int, timeout time.Duration, fn func(context.Context, *firestore.Transaction) error) error {
	return withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		return client.RunTransaction(ctx, fn)
	})
}

// See documentation for firestore.DocumentRef.Create(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func Create(ref *firestore.DocumentRef, data interface{}, attempts int, timeout time.Duration) (*firestore.WriteResult, error) {
	var wr *firestore.WriteResult
	err := withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		var err error
		wr, err = ref.Create(ctx, data)
		return err
	})
	return wr, err
}

// See documentation for firestore.DocumentRef.Set(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func Set(ref *firestore.DocumentRef, data interface{}, attempts int, timeout time.Duration, opts ...firestore.SetOption) (*firestore.WriteResult, error) {
	var wr *firestore.WriteResult
	err := withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		var err error
		wr, err = ref.Set(ctx, data, opts...)
		return err
	})
	return wr, err
}

// See documentation for firestore.DocumentRef.Update(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func Update(ref *firestore.DocumentRef, attempts int, timeout time.Duration, updates []firestore.Update, preconds ...firestore.Precondition) (*firestore.WriteResult, error) {
	var wr *firestore.WriteResult
	err := withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		var err error
		wr, err = ref.Update(ctx, updates, preconds...)
		return err
	})
	return wr, err
}

// See documentation for firestore.DocumentRef.Delete(). Uses the given maximum
// number of attempts and the given per-attempt timeout.
func Delete(ref *firestore.DocumentRef, attempts int, timeout time.Duration, preconds ...firestore.Precondition) (*firestore.WriteResult, error) {
	var wr *firestore.WriteResult
	err := withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		var err error
		wr, err = ref.Delete(ctx, preconds...)
		return err
	})
	return wr, err
}

// GetAllDescendantDocuments returns a slice of DocumentRefs for every
// descendent of the given Document. This includes missing documents, ie. those
// which do not exist but have sub-documents.
func GetAllDescendantDocuments(ref *firestore.DocumentRef, attempts int, timeout time.Duration) ([]*firestore.DocumentRef, error) {
	// TODO(borenet): Should we pause and resume like we do in IterDocs?
	colls := map[string]*firestore.CollectionRef{}
	if err := withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
		it := ref.Collections(ctx)
		for {
			coll, err := it.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				return err
			}
			colls[coll.Path] = coll
		}
		return nil
	}); err != nil {
		return nil, err
	}
	docs := map[string]*firestore.DocumentRef{}
	for _, coll := range colls {
		if err := withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
			it := coll.DocumentRefs(ctx)
			for {
				doc, err := it.Next()
				if err == iterator.Done {
					break
				} else if err != nil {
					return err
				}
				docs[doc.Path] = doc
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	rv := make([]*firestore.DocumentRef, 0, len(docs))
	for _, doc := range docs {
		children, err := GetAllDescendantDocuments(doc, attempts, timeout)
		if err != nil {
			return nil, err
		}
		rv = append(rv, children...)
	}
	for _, doc := range docs {
		rv = append(rv, doc)
	}
	sort.Sort(DocumentRefSlice(rv))
	return rv, nil
}

// RecursiveDelete deletes the given document and all of its descendant
// documents and collections. The given maximum number of attempts and the given
// per-attempt timeout apply for each delete operation, as opposed to the whole
// series of operations. This function does nothing to account for documents
// which may be added or modified while it is taking place.
func RecursiveDelete(client *Client, ref *firestore.DocumentRef, attempts int, timeout time.Duration) error {
	docs, err := GetAllDescendantDocuments(ref, attempts, timeout)
	if err != nil {
		return err
	}
	// Also delete the passed-in doc.
	docs = append(docs, ref)
	return util.ChunkIter(len(docs), MAX_TRANSACTION_DOCS, func(start, end int) error {
		return RunTransaction(client, attempts, timeout, func(ctx context.Context, tx *firestore.Transaction) error {
			for _, doc := range docs[start:end] {
				if err := tx.Delete(doc); err != nil {
					return err
				}
			}
			return nil
		})
	})
}
