package roller_cleanup

import (
	"context"
	"fmt"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2"
)

const (
	// Collection name for DB.
	collection = "Cleanup"

	// Firestore-related constants.
	defaultAttempts = 3
	defaultTimeout  = 10 * time.Second

	// App name used for Firestore.
	fsApp = "autoroll"
)

// firestoreDB implements DB using Firestore.
type firestoreDB struct {
	client *firestore.Client
	coll   *fs.CollectionRef
}

// NewDB returns a DB instance backed by the given firestore.Client.
func NewDB(ctx context.Context, client *firestore.Client) (*firestoreDB, error) {
	return &firestoreDB{
		client: client,
		coll:   client.Collection(collection),
	}, nil
}

// NewDB returns a DB instance backed by Firestore, using the given params.
func NewDBWithParams(ctx context.Context, project, instance string, ts oauth2.TokenSource) (DB, error) {
	client, err := firestore.NewClient(ctx, project, fsApp, instance, ts)
	if err != nil {
		return nil, err
	}
	return NewDB(ctx, client)
}

// RequestCleanup implements DB.
func (c *firestoreDB) RequestCleanup(ctx context.Context, req *CleanupRequest) error {
	if err := req.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if _, _, err := c.coll.Add(ctx, req); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// History implements DB.
func (c *firestoreDB) History(ctx context.Context, rollerID string, limit int) ([]*CleanupRequest, error) {
	// TODO(borenet): Implement pagination?
	var rv []*CleanupRequest
	q := c.coll.Query.Where("RollerID", "==", rollerID).OrderBy("Timestamp", fs.Desc)
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := c.client.IterDocs(ctx, "History", fmt.Sprintf("%s-%d", rollerID, limit), q, defaultAttempts, defaultTimeout, func(doc *fs.DocumentSnapshot) error {
		var req CleanupRequest
		if err := doc.DataTo(&req); err != nil {
			return err
		}
		rv = append(rv, &req)
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// Assert that firestoreDB implements DB.
var _ DB = &firestoreDB{}
