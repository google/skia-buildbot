package status

import (
	"context"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2"
)

const (
	// App name used for Firestore.
	fsApp = "autoroll"

	// Collection name for AutoRollStatus.
	collectionStatus = "Configs"

	// Firestore-related constants.
	defaultAttempts = 3
	defaultTimeout  = 10 * time.Second
)

// FirestoreDB implements DB using Firestore.
type FirestoreDB struct {
	client *firestore.Client
	coll   *fs.CollectionRef
}

// NewFirestoreDBWithParams returns a FirestoreDB instance using the given params.
func NewFirestoreDBWithParams(ctx context.Context, project, namespace, instance string, ts oauth2.TokenSource) (*FirestoreDB, error) {
	client, err := firestore.NewClient(ctx, project, namespace, instance, ts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return NewFirestoreDB(ctx, client)
}

// NewFirestoreDB returns a FirestoreDB instance using the given Client.
func NewFirestoreDB(ctx context.Context, client *firestore.Client) (*FirestoreDB, error) {
	return &FirestoreDB{
		client: client,
		coll:   client.Collection(collectionStatus),
	}, nil
}

// Close the DB.
func (d *FirestoreDB) Close() error {
	return skerr.Wrap(d.client.Close())
}

// Get implements DB.
func (d *FirestoreDB) Get(ctx context.Context, rollerID string) (*AutoRollStatus, error) {
	ref := d.coll.Doc(rollerID)
	doc, err := d.client.Get(ctx, ref, defaultAttempts, defaultTimeout)
	if err != nil {
		return nil, skerr.Wrapf(err, "retrieving config for %s", rollerID)
	}
	rv := new(AutoRollStatus)
	if err := doc.DataTo(rv); err != nil {
		return nil, skerr.Wrapf(err, "decoding config for %s", rollerID)
	}
	return rv, nil
}

// Set implements DB.
func (d *FirestoreDB) Set(ctx context.Context, rollerID string, st *AutoRollStatus) error {
	ref := d.coll.Doc(rollerID)
	if _, err := ref.Set(ctx, st); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

var _ DB = &FirestoreDB{}
