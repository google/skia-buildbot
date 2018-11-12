package firestore

/*
   This package provides convenience functions for interacting with Cloud Firestore.
*/

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
			sklog.Errorf("Encountered Firestore error; retrying: %s", err)
		}
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

// IterDocs is a convenience function which executes the given query and calls
// the given callback function for each document. Uses the given maximum number
// of attempts and the given per-attempt timeout.
func IterDocs(q firestore.Query, attempts int, timeout time.Duration, callback func(*firestore.DocumentSnapshot) error) error {
	return withTimeoutAndRetries(attempts, timeout, func(ctx context.Context) error {
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
	})
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
